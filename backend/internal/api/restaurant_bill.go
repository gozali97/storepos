package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/pricing"
	"github.com/storepos/storepos/internal/sales"
)

func (s *Server) orderBill(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("order_id tidak valid"))
		return
	}
	var out map[string]any
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		var typ, status, discountKind string
		var discountValue int64
		var version int
		var fee int64
		err := tx.QueryRow(ctx, `SELECT branch_id,order_type,status,version,delivery_fee_cents,discount_kind,discount_value FROM restaurant_orders WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, id).Scan(&branch, &typ, &status, &version, &fee, &discountKind, &discountValue)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("pesanan")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		if status != "open" {
			return errs.Conflict("tagihan sementara hanya untuk order open")
		}
		var tax *pricing.TaxRule
		var name, mode string
		var bps int64
		err = tx.QueryRow(ctx, `SELECT name,rate_bps,mode FROM tax_rules WHERE business_id=$1 AND active`, tc.BusinessID).Scan(&name, &bps, &mode)
		if err == nil {
			tax = &pricing.TaxRule{Name: name, RateBps: bps, Mode: pricing.TaxExclusive}
			if mode == "inclusive" {
				tax.Mode = pricing.TaxInclusive
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var sc *pricing.ServiceChargeRule
		var rate int64
		var inTax bool
		err = tx.QueryRow(ctx, `SELECT rate_bps,include_in_tax_base FROM restaurant_service_charges WHERE business_id=$1 AND branch_id=$2 AND order_type=$3`, tc.BusinessID, branch, typ).Scan(&rate, &inTax)
		if err == nil {
			sc = &pricing.ServiceChargeRule{RateBps: rate, IncludeInTaxBase: inTax}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id::text,qty_milli,unit_price_cents+modifier_delta_cents,kitchen_status,product_name,variant_name,base_unit,note,modifiers,variant_id,discount_kind,discount_value FROM restaurant_order_lines WHERE business_id=$1 AND order_id=$2 AND kitchen_status<>'CANCELLED' ORDER BY created_at,id`, tc.BusinessID, id)
		if err != nil {
			return err
		}
		var lines []pricing.Line
		type billLine struct {
			ID        string          `json:"line_id"`
			Name      string          `json:"product_name"`
			Variant   string          `json:"variant_name"`
			Unit      string          `json:"unit"`
			Note      string          `json:"note"`
			Modifiers json.RawMessage `json:"modifiers"`
			Qty       string          `json:"qty_milli"`
			Price     string          `json:"effective_unit_price_cents"`
			Total     string          `json:"line_total_cents"`
			Tax       string          `json:"tax_cents"`
			SC        string          `json:"service_charge_cents"`
		}
		details := []billLine{}
		variants := []idkit.ID{}
		notSent := 0
		for rows.Next() {
			var line string
			var qty, price int64
			var ks string
			var detail billLine
			var variant idkit.ID
			var dk string
			var dv int64
			if err := rows.Scan(&line, &qty, &price, &ks, &detail.Name, &detail.Variant, &detail.Unit, &detail.Note, &detail.Modifiers, &variant, &dk, &dv); err != nil {
				rows.Close()
				return err
			}
			if ks == "NOT_SENT" {
				notSent++
			}
			detail.ID = line
			detail.Qty = strconv.FormatInt(qty, 10)
			detail.Price = strconv.FormatInt(price, 10)
			details = append(details, detail)
			variants = append(variants, variant)
			d, err := storedDiscount(dk, dv)
			if err != nil {
				rows.Close()
				return errs.Validation(err.Error())
			}
			lines = append(lines, pricing.Line{ID: line, Qty: money.Milli(qty), UnitPriceCents: money.Cents(price), Tax: tax, ServiceCharge: sc, LineDiscount: d, DiscountEligible: true})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(lines) == 0 {
			return errs.Conflict("pesanan tidak memiliki item aktif")
		}
		for i, variant := range variants {
			var product idkit.ID
			if err := tx.QueryRow(ctx, `SELECT product_id FROM variants WHERE business_id=$1 AND id=$2`, tc.BusinessID, variant).Scan(&product); err != nil {
				return err
			}
			t, err := sales.EffectiveTax(ctx, tx, tc.BusinessID, product, tax)
			if err != nil {
				return err
			}
			lines[i].Tax = t
		}
		txDiscount, err := storedDiscount(discountKind, discountValue)
		if err != nil {
			return errs.Validation(err.Error())
		}
		total, err := pricing.Compute(pricing.Input{Lines: lines, TransactionDiscount: txDiscount})
		if err != nil {
			return err
		}
		var discountBase int64
		for i, l := range total.Lines {
			if lines[i].LineDiscount.Kind == pricing.DiscountAmount && lines[i].LineDiscount.Value > int64(l.GrossCents) {
				return errs.Validation("diskon baris melebihi nilai item")
			}
			discountBase += int64(l.NetBeforeTxCents)
		}
		if txDiscount.Kind == pricing.DiscountAmount && txDiscount.Value > discountBase {
			return errs.Validation("diskon transaksi melebihi subtotal")
		}
		for i, l := range total.Lines {
			details[i].Total = money.FormatCents(l.LineTotalCents)
			details[i].Tax = money.FormatCents(l.TaxCents)
			details[i].SC = money.FormatCents(l.ServiceChargeCents)
		}
		if int64(total.TotalCents) > math.MaxInt64-fee {
			return errs.Validation("total/ongkir melampaui kapasitas nominal")
		}
		out = map[string]any{"order_id": id.String(), "version": version, "label": "BELUM LUNAS", "subtotal_gross_cents": money.FormatCents(total.SubtotalGrossCents), "tax_cents": money.FormatCents(total.TaxCents), "service_charge_cents": money.FormatCents(total.ServiceChargeCents), "delivery_fee_cents": strconv.FormatInt(fee, 10), "total_cents": money.FormatCents(total.TotalCents + money.Cents(fee)), "line_discount_cents": money.FormatCents(total.TotalLineDiscountCents), "transaction_discount_cents": money.FormatCents(total.TransactionDiscountCents), "not_sent_count": notSent, "payment_blocked": notSent > 0, "lines": details}
		return nil
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, out)
}

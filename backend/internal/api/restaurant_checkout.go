package apiserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/idempotency"
	"github.com/storepos/storepos/internal/inventory"
	"github.com/storepos/storepos/internal/org"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/pricing"
	"github.com/storepos/storepos/internal/sales"
)

func (s *Server) checkoutRestaurant(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	order, version, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key wajib maksimal 128 karakter"))
		return
	}
	var req struct {
		Approval *approvalReq `json:"approval"`
		Register string       `json:"register_id"`
		Payments []struct {
			Method string `json:"method"`
			Amount string `json:"amount_cents"`
		} `json:"payments"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	register, err := idkit.Parse(req.Register)
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("register_id tidak valid"))
		return
	}
	payments := []sales.PaymentIn{}
	for _, p := range req.Payments {
		a, err := money.ParseCents(p.Amount)
		if err != nil || a <= 0 {
			httpx.WriteError(w, r, errs.Validation("nominal pembayaran tidak valid"))
			return
		}
		payments = append(payments, sales.PaymentIn{Method: p.Method, AmountCents: int64(a)})
	}
	approved, err := s.approver(r.Context(), tc, req.Approval)
	if err != nil {
		httpx.WriteError(w, r, mapShiftError(err))
		return
	}
	raw, err := json.Marshal(struct {
		Version int
		Request any
	}{version, req})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hash := sha256.Sum256(raw)
	var result *sales.Result
	var replay *idempotency.Stored
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		err := tx.QueryRow(ctx, `SELECT branch_id FROM restaurant_orders WHERE business_id=$1 AND id=$2`, tc.BusinessID, order).Scan(&branch)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("pesanan")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		st, err := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /restaurant/orders/"+order.String()+"/checkout", hash[:])
		replay = st
		if err != nil || st != nil {
			return err
		}
		// Match finalization lock ordering: business, then shift, then order/balances.
		if _, err := tx.Exec(ctx, `SELECT id FROM businesses WHERE id=$1 FOR UPDATE`, tc.BusinessID); err != nil {
			return err
		}
		var shift idkit.ID
		err = tx.QueryRow(ctx, `SELECT id FROM shifts WHERE business_id=$1 AND branch_id=$2 AND register_id=$3 AND status='open' FOR SHARE`, tc.BusinessID, branch, register).Scan(&shift)
		if errors.Is(err, pgx.ErrNoRows) {
			return sales.ErrShiftNotOpen
		}
		if err != nil {
			return err
		}
		if _, err := lockRestaurantOrder(ctx, tx, tc, order, version); err != nil {
			return err
		}
		var typ, note, discountKind, discountReason string
		var discountValue int64
		var fee int64
		if err := tx.QueryRow(ctx, `SELECT order_type,note,delivery_fee_cents,discount_kind,discount_value,discount_reason FROM restaurant_orders WHERE business_id=$1 AND id=$2`, tc.BusinessID, order).Scan(&typ, &note, &fee, &discountKind, &discountValue, &discountReason); err != nil {
			return err
		}
		var sc *pricing.ServiceChargeRule
		var rate int64
		var taxbase bool
		err = tx.QueryRow(ctx, `SELECT rate_bps,include_in_tax_base FROM restaurant_service_charges WHERE business_id=$1 AND branch_id=$2 AND order_type=$3`, tc.BusinessID, branch, typ).Scan(&rate, &taxbase)
		if err == nil {
			sc = &pricing.ServiceChargeRule{RateBps: rate, IncludeInTaxBase: taxbase}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT l.id,l.variant_id,l.qty_milli,l.unit_price_cents,l.product_name,l.variant_name,l.base_unit,l.note,l.modifiers,l.modifier_delta_cents,l.kitchen_status,
 r.qty_milli,r.status,l.discount_kind,l.discount_value
 FROM restaurant_order_lines l LEFT JOIN restaurant_stock_reservations r ON r.business_id=l.business_id AND r.line_id=l.id
 WHERE l.business_id=$1 AND l.order_id=$2 AND l.kitchen_status<>'CANCELLED' ORDER BY l.variant_id,l.id`, tc.BusinessID, order)
		if err != nil {
			return err
		}
		type item struct {
			id, variant       idkit.ID
			qty               int64
			snapshot          sales.OrderSnapshot
			note, status      string
			reserved          *int64
			reservationStatus *string
			discountKind      string
			discountValue     int64
		}
		items := []item{}
		for rows.Next() {
			var i item
			if err := rows.Scan(&i.id, &i.variant, &i.qty, &i.snapshot.Price, &i.snapshot.Product, &i.snapshot.Variant, &i.snapshot.Unit, &i.note, &i.snapshot.Modifiers, &i.snapshot.Delta, &i.status, &i.reserved, &i.reservationStatus, &i.discountKind, &i.discountValue); err != nil {
				rows.Close()
				return err
			}
			items = append(items, i)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(items) == 0 {
			return errs.Validation("pesanan tidak memiliki item aktif")
		}
		// All reservations are consumed under inventory locks before shared Finalize deducts SALE.
		sort.Slice(items, func(i, j int) bool { return items[i].variant.String() < items[j].variant.String() })
		input := sales.Input{BusinessID: tc.BusinessID, BranchID: branch, RegisterID: register, UserID: tc.UserID, Note: note, Payments: payments, ServiceCharge: sc, DeliveryFeeCents: fee}
		policy, err := org.LoadPolicy(ctx, tx, tc.BusinessID)
		if err != nil {
			return err
		}
		input.TxDiscount, err = storedDiscount(discountKind, discountValue)
		if err != nil {
			return errs.Validation(err.Error())
		}
		input.DiscountReason = discountReason
		input.DiscountApprovedBy = approved
		input.DiscountLimitBps = policy.CashierDiscountBps
		input.UnlimitedDiscount = authz.Check(tc.Role, authz.PermDiscountOverride) == authz.Allowed
		for _, i := range items {
			if i.status == "NOT_SENT" {
				return errs.Conflict("kirim atau hapus seluruh item NOT_SENT sebelum bayar")
			}
			if i.snapshot.Price < 0 || i.snapshot.Delta < 0 || i.snapshot.Price > math.MaxInt64-i.snapshot.Delta {
				return errs.Validation("harga snapshot tidak valid")
			}
			if i.reserved != nil {
				if i.reservationStatus == nil || *i.reservationStatus != "active" || *i.reserved != i.qty {
					return errs.Conflict("reservasi item tidak konsisten")
				}
				if _, err := inventory.Lock(ctx, tx, tc.BusinessID, branch, i.variant); err != nil {
					return err
				}
				tag, err := tx.Exec(ctx, `UPDATE inventory_balances SET reserved_milli=reserved_milli-$4 WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3 AND reserved_milli >= $4`, tc.BusinessID, branch, i.variant, *i.reserved)
				if err != nil {
					return err
				}
				if tag.RowsAffected() != 1 {
					return errs.Conflict("saldo reservasi tidak konsisten")
				}
				if _, err := tx.Exec(ctx, `UPDATE restaurant_stock_reservations SET status='consumed' WHERE business_id=$1 AND line_id=$2 AND status='active'`, tc.BusinessID, i.id); err != nil {
					return err
				}
				i.snapshot.Track = true
			}
			d, err := storedDiscount(i.discountKind, i.discountValue)
			if err != nil {
				return errs.Validation(err.Error())
			}
			snap := i.snapshot
			input.Lines = append(input.Lines, sales.LineIn{VariantID: i.variant, QtyMilli: i.qty, Note: i.note, OrderSnapshot: &snap, Discount: d})
		}
		result, err = sales.Finalize(ctx, tx, input)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE sales SET restaurant_order_id=$3,order_type=$4,service_charge_rate_bps=$5,service_charge_in_tax_base=$6 WHERE business_id=$1 AND id=$2`, tc.BusinessID, result.SaleID, order, typ, rate, taxbase); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_orders SET status='completed',payment_status='paid',sale_id=$3,closed_at=clock_timestamp(),version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, order, result.SaleID); err != nil {
			return err
		}
		if err := audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.order_paid", EntityType: "restaurant_order", EntityID: order.String(), After: map[string]string{"sale_id": result.SaleID.String()}}); err != nil {
			return err
		}
		body, err := json.Marshal(saleResponse(result))
		if err != nil {
			return err
		}
		return idempotency.Complete(ctx, tx, tc.BusinessID, key, http.StatusCreated, body)
	})
	if err != nil {
		httpx.WriteError(w, r, mapSalesError(err))
		return
	}
	if replay != nil {
		w.Header().Set("Idempotent-Replay", "true")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(replay.Status)
		_, _ = w.Write(replay.Body)
		return
	}
	httpx.JSON(w, http.StatusCreated, saleResponse(result))
}

package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) saleReadRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/sales", s.guard(authz.PermSaleReturn, false, s.searchSales))
	mux.Handle("GET /api/v1/sales/{id}", s.guard(authz.PermSaleReturn, false, s.getSale))
}

type saleHeader struct {
	Note         string          `json:"note"`
	Snapshot     json.RawMessage `json:"header_snapshot"`
	ID           string          `json:"sale_id"`
	Branch       string          `json:"branch_id"`
	Receipt      string          `json:"receipt_number"`
	Status       string          `json:"status"`
	ReturnStatus string          `json:"return_status"`
	Total        string          `json:"total_cents"`
	Change       string          `json:"change_cents"`
	Rounding     string          `json:"cash_rounding_cents"`
	Tax          string          `json:"tax_cents"`
	DeliveryFee  string          `json:"delivery_fee_cents"`
	At           time.Time       `json:"completed_at"`
	Customer     *string         `json:"customer_name"`
}

func (s *Server) searchSales(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return
	}
	receipt := strings.TrimSpace(q.Get("receipt_number"))
	if len(receipt) > 100 {
		httpx.WriteError(w, r, errs.Validation("nomor struk maksimal 100 byte"))
		return
	}
	var from, until *time.Time
	if q.Get("from") != "" || q.Get("to") != "" {
		f, ok := s.reportFilter(w, r)
		if !ok {
			return
		}
		from = &f.From
		until = &f.Until
	} else if receipt == "" {
		httpx.WriteError(w, r, errs.Validation("nomor struk atau periode from/to wajib"))
		return
	}
	var customer *idkit.ID
	if raw := q.Get("customer_id"); raw != "" {
		cid, err := idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("customer_id tidak valid"))
			return
		}
		var exists bool
		if err := s.pool.Underlying().QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM customers WHERE business_id=$1 AND id=$2)`, tc.BusinessID, cid).Scan(&exists); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if !exists {
			httpx.WriteError(w, r, errs.NotFound("pelanggan"))
			return
		}
		customer = &cid
	}
	var cursor idkit.ID
	var before *time.Time
	if raw := q.Get("before"); raw != "" {
		id, err := idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return
		}
		cursor = id
		var at time.Time
		err = s.pool.Underlying().QueryRow(r.Context(), `SELECT completed_at FROM sales WHERE business_id=$1 AND branch_id=$2 AND id=$3 AND ($4='' OR receipt_number=$4) AND ($5::timestamptz IS NULL OR completed_at >= $5) AND ($6::timestamptz IS NULL OR completed_at < $6) AND ($7::uuid IS NULL OR customer_id=$7)`, tc.BusinessID, branch, id, receipt, from, until, customer).Scan(&at)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, errs.Validation("cursor di luar cakupan/filter"))
			return
		}
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		before = &at
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT s.id::text,s.branch_id::text,s.receipt_number,s.status,s.return_status,s.total_cents::text,s.change_cents::text,s.cash_rounding_cents::text,s.tax_cents::text,s.completed_at,c.name,s.note
 FROM sales s LEFT JOIN customers c ON c.business_id=s.business_id AND c.id=s.customer_id
 WHERE s.business_id=$1 AND s.branch_id=$2 AND ($3='' OR s.receipt_number=$3) AND ($4::timestamptz IS NULL OR s.completed_at >= $4) AND ($5::timestamptz IS NULL OR s.completed_at < $5)
 AND ($6::timestamptz IS NULL OR (s.completed_at,s.id)<($6,$7::uuid)) AND ($8::uuid IS NULL OR s.customer_id=$8) ORDER BY s.completed_at DESC,s.id DESC LIMIT 101`, tc.BusinessID, branch, receipt, from, until, before, cursor, customer)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	out := []saleHeader{}
	for rows.Next() {
		var x saleHeader
		if err := rows.Scan(&x.ID, &x.Branch, &x.Receipt, &x.Status, &x.ReturnStatus, &x.Total, &x.Change, &x.Rounding, &x.Tax, &x.At, &x.Customer, &x.Note); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].ID
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"sales": out, "next_cursor": next})
}
func (s *Server) getSale(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("sale_id tidak valid"))
		return
	}
	var header saleHeader
	var lines []saleLineDetail
	var payments []paymentDetail
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		// SHARE prevents concurrent void/return from producing inconsistent remaining quantities.
		var branch idkit.ID
		err := tx.QueryRow(ctx, `SELECT s.id::text,s.branch_id,s.receipt_number,s.status,s.return_status,s.total_cents::text,s.change_cents::text,s.cash_rounding_cents::text,s.tax_cents::text,s.delivery_fee_cents::text,s.completed_at,c.name,s.receipt_header_snapshot,s.note
  FROM sales s LEFT JOIN customers c ON c.business_id=s.business_id AND c.id=s.customer_id WHERE s.business_id=$1 AND s.id=$2 FOR SHARE OF s`, tc.BusinessID, id).Scan(&header.ID, &branch, &header.Receipt, &header.Status, &header.ReturnStatus, &header.Total, &header.Change, &header.Rounding, &header.Tax, &header.DeliveryFee, &header.At, &header.Customer, &header.Snapshot, &header.Note)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("transaksi")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		header.Branch = branch.String()
		cost := authz.Check(tc.Role, authz.PermProductViewCost) == authz.Allowed
		rows, err := tx.Query(ctx, `SELECT l.id::text,l.variant_id::text,l.sku,l.product_name,l.variant_name,l.base_unit,l.sale_unit,l.factor_milli::text,l.qty_milli::text,l.base_qty_milli::text,l.unit_price_cents::text,l.line_total_cents::text,l.allow_fractional,
  COALESCE((SELECT SUM(r.qty_milli) FROM sale_return_lines r WHERE r.business_id=l.business_id AND r.sale_line_id=l.id),0)::text,
  (l.qty_milli-COALESCE((SELECT SUM(r.qty_milli) FROM sale_return_lines r WHERE r.business_id=l.business_id AND r.sale_line_id=l.id),0))::text,
  CASE WHEN $3 THEN l.cost_cents::text END,l.note,l.tax_rule_name,l.tax_rate_bps,l.tax_mode,l.gross_cents::text,l.line_discount_cents::text,l.alloc_discount_cents::text,l.net_cents::text,l.tax_cents::text,l.service_charge_cents::text,COALESCE((SELECT SUM(r.amount_cents) FROM sale_return_lines r WHERE r.business_id=l.business_id AND r.sale_line_id=l.id),0)::text,l.modifiers
  FROM sale_lines l WHERE l.business_id=$1 AND l.sale_id=$2 ORDER BY line_no`, tc.BusinessID, id, cost)
		if err != nil {
			return err
		}
		lines = []saleLineDetail{}
		for rows.Next() {
			var l saleLineDetail
			if err := rows.Scan(&l.ID, &l.Variant, &l.SKU, &l.Product, &l.VariantName, &l.BaseUnit, &l.SaleUnit, &l.Factor, &l.Qty, &l.BaseQty, &l.Price, &l.Total, &l.Fractional, &l.Returned, &l.Remaining, &l.Cost, &l.Note, &l.TaxName, &l.TaxRate, &l.TaxMode, &l.Gross, &l.LineDiscount, &l.AllocatedDiscount, &l.Net, &l.Tax, &l.SC, &l.Refunded, &l.Modifiers); err != nil {
				rows.Close()
				return err
			}
			lines = append(lines, l)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		rows, err = tx.Query(ctx, `SELECT method,amount_cents::text,tendered_cents::text,change_cents::text,status,manual_refund_required,verification FROM payments WHERE business_id=$1 AND sale_id=$2 ORDER BY created_at,id`, tc.BusinessID, id)
		if err != nil {
			return err
		}
		payments = []paymentDetail{}
		for rows.Next() {
			var p paymentDetail
			if err := rows.Scan(&p.Method, &p.Amount, &p.Tendered, &p.Change, &p.Status, &p.Manual, &p.Verification); err != nil {
				rows.Close()
				return err
			}
			payments = append(payments, p)
		}
		rows.Close()
		return rows.Err()
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"sale": header, "lines": lines, "payments": payments})
}

type saleLineDetail struct {
	Modifiers         json.RawMessage `json:"modifiers"`
	TaxName           *string         `json:"tax_rule_name"`
	TaxRate           *int            `json:"tax_rate_bps"`
	TaxMode           *string         `json:"tax_mode"`
	Gross             string          `json:"gross_cents"`
	LineDiscount      string          `json:"line_discount_cents"`
	AllocatedDiscount string          `json:"alloc_discount_cents"`
	Net               string          `json:"net_cents"`
	Tax               string          `json:"tax_cents"`
	SC                string          `json:"service_charge_cents"`
	Refunded          string          `json:"refunded_cents"`
	Note              string          `json:"note"`
	ID                string          `json:"sale_line_id"`
	Variant           string          `json:"variant_id"`
	SKU               string          `json:"sku"`
	Product           string          `json:"product_name"`
	VariantName       string          `json:"variant_name"`
	BaseUnit          string          `json:"base_unit"`
	SaleUnit          *string         `json:"sale_unit"`
	Factor            string          `json:"factor_milli"`
	Qty               string          `json:"qty_milli"`
	BaseQty           string          `json:"base_qty_milli"`
	Price             string          `json:"unit_price_cents"`
	Total             string          `json:"line_total_cents"`
	Fractional        bool            `json:"allow_fractional"`
	Returned          string          `json:"returned_qty_milli"`
	Remaining         string          `json:"remaining_qty_milli"`
	Cost              *string         `json:"cost_cents"`
}
type paymentDetail struct {
	Verification string `json:"verification"`
	Method       string `json:"method"`
	Amount       string `json:"amount_cents"`
	Tendered     string `json:"tendered_cents"`
	Change       string `json:"change_cents"`
	Status       string `json:"status"`
	Manual       bool   `json:"manual_refund_required"`
}

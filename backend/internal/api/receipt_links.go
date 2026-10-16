package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/auth"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) receiptRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/sales/{id}/receipt-links", s.guard(authz.PermSaleCreate, true, s.createReceiptLink))
	mux.Handle("DELETE /api/v1/sales/{id}/receipt-links", s.guard(authz.PermBusinessManage, true, s.revokeReceiptLinks))
	mux.HandleFunc("POST /api/v1/public/receipt", s.publicReceipt)
	mux.Handle("POST /api/v1/sales/{id}/reprints", s.guard(authz.PermSaleCreate, true, s.reprintReceipt))
}

// Token goes in POST body, not URL path/query: access logs must never capture bearer links.
func (s *Server) createReceiptLink(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	sale, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("sale_id tidak valid"))
		return
	}
	token, err := auth.NewReceiptToken()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	id := idkit.New()
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		err := tx.QueryRow(ctx, `SELECT branch_id FROM sales WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, sale).Scan(&branch)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("transaksi")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO receipt_links(id,business_id,sale_id,token_hash,created_by) VALUES($1,$2,$3,$4,$5)`, id, tc.BusinessID, sale, auth.HashToken(token), tc.UserID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "receipt_link.created", EntityType: "receipt_link", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusCreated, map[string]string{"token": token, "receipt_path": "/receipt#" + token})
}
func (s *Server) revokeReceiptLinks(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	sale, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("sale_id tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		err := tx.QueryRow(ctx, `SELECT branch_id FROM sales WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, sale).Scan(&branch)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("transaksi")
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE receipt_links SET revoked_at=COALESCE(revoked_at,now()) WHERE business_id=$1 AND sale_id=$2`, tc.BusinessID, sale); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "receipt_link.revoked", EntityType: "sale", EntityID: sale.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
func (s *Server) publicReceipt(w http.ResponseWriter, r *http.Request) {
	var allowed bool
	err := s.pool.Underlying().QueryRow(r.Context(), `INSERT INTO public_receipt_limits(key_hash,window_start,requests)
	VALUES($1,date_trunc('minute',clock_timestamp()),1)
	ON CONFLICT(key_hash) DO UPDATE SET
	requests=CASE WHEN public_receipt_limits.window_start=date_trunc('minute',clock_timestamp()) THEN LEAST(public_receipt_limits.requests+1,61) ELSE 1 END,
	window_start=date_trunc('minute',clock_timestamp()) RETURNING requests<=60`, auth.HashToken("receipt-peer:"+ClientIP(r))).Scan(&allowed)
	if err != nil {
		httpx.WriteError(w, r, errs.Internal("pemeriksaan batas akses struk gagal").Wrap(err))
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", "60")
		httpx.WriteError(w, r, errs.New(errs.CodeRateLimited, "terlalu banyak permintaan struk", http.StatusTooManyRequests))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	var req struct {
		Token string `json:"token"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !strings.HasPrefix(req.Token, auth.PrefixReceipt) || len(req.Token) != len(auth.PrefixReceipt)+43 {
		httpx.WriteError(w, r, errs.NotFound("struk"))
		return
	}
	var data map[string]any
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var biz, sale idkit.ID
		err := tx.QueryRow(ctx, `SELECT business_id,sale_id FROM receipt_links WHERE token_hash=$1 AND revoked_at IS NULL FOR SHARE`, auth.HashToken(req.Token)).Scan(&biz, &sale)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("struk")
		}
		if err != nil {
			return err
		}
		var receipt, status, total, change, tax, rounding, deliveryFee string
		var at time.Time
		var customer *string
		var header json.RawMessage
		var gross, discount, sc string
		err = tx.QueryRow(ctx, `SELECT s.receipt_number,s.status,s.total_cents::text,s.change_cents::text,s.tax_cents::text,s.cash_rounding_cents::text,s.completed_at,c.name,s.receipt_header_snapshot,s.subtotal_gross_cents::text,s.delivery_fee_cents::text,(s.line_discount_cents+s.tx_discount_cents)::text,s.service_charge_cents::text FROM sales s LEFT JOIN customers c ON c.business_id=s.business_id AND c.id=s.customer_id WHERE s.business_id=$1 AND s.id=$2`, biz, sale).Scan(&receipt, &status, &total, &change, &tax, &rounding, &at, &customer, &header, &gross, &deliveryFee, &discount, &sc)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT product_name,variant_name,COALESCE(sale_unit,base_unit),qty_milli::text,unit_price_cents::text,line_total_cents::text,tax_rule_name,tax_rate_bps,tax_mode,(line_discount_cents+alloc_discount_cents)::text,tax_cents::text,service_charge_cents::text,modifiers FROM sale_lines WHERE business_id=$1 AND sale_id=$2 ORDER BY line_no`, biz, sale)
		if err != nil {
			return err
		}
		defer rows.Close()
		type line struct {
			Modifiers json.RawMessage `json:"modifiers"`
			TaxName   *string         `json:"tax_rule_name"`
			TaxRate   *int            `json:"tax_rate_bps"`
			TaxMode   *string         `json:"tax_mode"`
			Discount  string          `json:"discount_cents"`
			Tax       string          `json:"tax_cents"`
			SC        string          `json:"service_charge_cents"`
			Name      string          `json:"product_name"`
			Variant   string          `json:"variant_name"`
			Unit      string          `json:"unit"`
			Qty       string          `json:"qty_milli"`
			Price     string          `json:"unit_price_cents"`
			Total     string          `json:"line_total_cents"`
		}
		lines := []line{}
		for rows.Next() {
			var l line
			if err := rows.Scan(&l.Name, &l.Variant, &l.Unit, &l.Qty, &l.Price, &l.Total, &l.TaxName, &l.TaxRate, &l.TaxMode, &l.Discount, &l.Tax, &l.SC, &l.Modifiers); err != nil {
				return err
			}
			lines = append(lines, l)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		data = map[string]any{"receipt_number": receipt, "status": status, "total_cents": total, "change_cents": change, "tax_cents": tax, "cash_rounding_cents": rounding, "delivery_fee_cents": deliveryFee, "completed_at": at, "customer_name": customer, "lines": lines, "header_snapshot": header, "header_snapshot_missing": header == nil, "subtotal_gross_cents": gross, "discount_cents": discount, "service_charge_cents": sc}
		return nil
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, data)
}

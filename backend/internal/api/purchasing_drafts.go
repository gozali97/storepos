package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) getGoodsReceipt(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("receipt_id tidak valid"))
		return
	}
	type line struct {
		ID      string  `json:"line_id"`
		Unit    string  `json:"unit_id"`
		Variant string  `json:"variant_id"`
		Qty     string  `json:"qty_milli"`
		Price   string  `json:"purchase_price_cents"`
		Name    *string `json:"unit_name"`
		Factor  *string `json:"factor_milli"`
		BaseQty *string `json:"base_qty_milli"`
		Cost    *string `json:"base_cost_cents"`
	}
	var branch idkit.ID
	var status, date, invoice string
	var version int
	var supplier *string
	var posted *time.Time
	var lines []line
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		err := tx.QueryRow(ctx, `SELECT branch_id,status,version,received_date::text,invoice_number,supplier_id::text,posted_at FROM goods_receipts WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, id).Scan(&branch, &status, &version, &date, &invoice, &supplier, &posted)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("penerimaan")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		rows, err := tx.Query(ctx, `SELECT l.id::text,l.unit_id::text,u.variant_id::text,l.qty_milli::text,l.purchase_price_cents::text,l.unit_name,l.factor_milli::text,l.base_qty_milli::text,l.base_cost_cents::text FROM goods_receipt_lines l JOIN purchase_units u ON u.business_id=l.business_id AND u.id=l.unit_id WHERE l.business_id=$1 AND l.receipt_id=$2 ORDER BY l.id`, tc.BusinessID, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		lines = []line{}
		for rows.Next() {
			var l line
			if err := rows.Scan(&l.ID, &l.Unit, &l.Variant, &l.Qty, &l.Price, &l.Name, &l.Factor, &l.BaseQty, &l.Cost); err != nil {
				return err
			}
			lines = append(lines, l)
		}
		return rows.Err()
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"receipt_id": id.String(), "branch_id": branch.String(), "status": status, "version": version, "received_date": date, "invoice_number": invoice, "supplier_id": supplier, "posted_at": posted, "lines": lines})
}
func (s *Server) editGoodsReceipt(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	v, e := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if err != nil || e != nil || v <= 0 {
		httpx.WriteError(w, r, errs.Validation("id/If-Match tidak valid"))
		return
	}
	var req struct {
		Date     string  `json:"received_date"`
		Invoice  string  `json:"invoice_number"`
		Supplier *string `json:"supplier_id"`
		Lines    []struct {
			Unit  string `json:"unit_id"`
			Qty   string `json:"qty_milli"`
			Price string `json:"purchase_price_cents"`
		} `json:"lines"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := time.Parse("2006-01-02", req.Date); err != nil || len(req.Invoice) > 200 || len(req.Lines) == 0 || len(req.Lines) > 500 {
		httpx.WriteError(w, r, errs.Validation("tanggal/faktur/baris tidak valid"))
		return
	}
	type input struct {
		unit       idkit.ID
		qty, price int64
	}
	items := []input{}
	for _, l := range req.Lines {
		u, e1 := idkit.Parse(l.Unit)
		q, e2 := money.ParseMilli(l.Qty)
		p, e3 := money.ParseCents(l.Price)
		if e1 != nil || e2 != nil || e3 != nil || q <= 0 || p < 0 {
			httpx.WriteError(w, r, errs.Validation("baris tidak valid"))
			return
		}
		items = append(items, input{u, int64(q), int64(p)})
	}
	var supplier *idkit.ID
	if req.Supplier != nil {
		u, err := idkit.Parse(*req.Supplier)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("supplier_id tidak valid"))
			return
		}
		supplier = &u
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		// Match supplier deletion's lock order: supplier first, then receipt.
		if supplier != nil {
			var active bool
			err := tx.QueryRow(ctx, `SELECT NOT archived FROM suppliers WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, *supplier).Scan(&active)
			if errors.Is(err, pgx.ErrNoRows) {
				return errs.NotFound("supplier")
			}
			if err != nil {
				return err
			}
			if !active {
				return errs.Conflict("supplier diarsipkan")
			}
		}
		var branch idkit.ID
		var status string
		var current int
		err := tx.QueryRow(ctx, `SELECT branch_id,status,version FROM goods_receipts WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&branch, &status, &current)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("penerimaan")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		if status != "draft" || current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "penerimaan berubah atau sudah posted", http.StatusConflict)
		}
		for _, l := range items {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM purchase_units WHERE business_id=$1 AND id=$2 AND NOT archived)`, tc.BusinessID, l.unit).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return errs.Validation("satuan di luar tenant atau nonaktif")
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM goods_receipt_lines WHERE business_id=$1 AND receipt_id=$2`, tc.BusinessID, id); err != nil {
			return err
		}
		for _, l := range items {
			if _, err := tx.Exec(ctx, `INSERT INTO goods_receipt_lines(id,business_id,receipt_id,unit_id,qty_milli,purchase_price_cents) VALUES($1,$2,$3,$4,$5,$6)`, idkit.New(), tc.BusinessID, id, l.unit, l.qty, l.price); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE goods_receipts SET received_date=$3,invoice_number=$4,supplier_id=$5,version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Date, req.Invoice, supplier); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "goods_receipt.edited", EntityType: "goods_receipt", EntityID: id.String(), After: map[string]int{"version": v + 1, "line_count": len(items)}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"receipt_id": id.String(), "status": "draft", "version": v + 1})
}

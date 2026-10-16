package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) minimumRoutes(mux *http.ServeMux) {
	mux.Handle("PUT /api/v1/stock/minimums/{variant}", s.guard(authz.PermProductManage, true, s.setMinimum))
	mux.Handle("GET /api/v1/stock/reorder", s.guard(authz.PermProductView, false, s.reorderList))
}
func (s *Server) setMinimum(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		BranchID string `json:"branch_id"`
		Minimum  string `json:"minimum_qty_milli"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.BranchID)
	if !ok {
		return
	}
	vid, e1 := idkit.Parse(r.PathValue("variant"))
	qty, e2 := money.ParseMilli(req.Minimum)
	version, e3 := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if e1 != nil || e2 != nil || qty < 0 || e3 != nil || version < 0 {
		httpx.WriteError(w, r, errs.Validation("varian/minimum tidak valid; If-Match 0 untuk buat atau versi positif untuk edit"))
		return
	}
	var next int
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		// Lock parent variant to serialize creation as well as versioned updates.
		var track bool
		err := tx.QueryRow(ctx, `SELECT p.track_stock FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id WHERE v.business_id=$1 AND v.id=$2 AND v.status<>'archived' AND p.status<>'archived' FOR UPDATE OF v`, tc.BusinessID, vid).Scan(&track)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("varian")
		}
		if err != nil {
			return err
		}
		if !track {
			return errs.Validation("produk tidak melacak stok")
		}
		var active bool
		err = tx.QueryRow(ctx, `SELECT status='active' FROM branches WHERE business_id=$1 AND id=$2`, tc.BusinessID, branch).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("cabang")
		}
		if err != nil {
			return err
		}
		if !active {
			return errs.Conflict("cabang nonaktif")
		}
		var current int
		err = tx.QueryRow(ctx, `SELECT version FROM stock_minimums WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, tc.BusinessID, branch, vid).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			current = 0
		} else if err != nil {
			return err
		}
		if current != version {
			return errs.New(errs.CodeOptimisticLockFailure, "stok minimum diubah pengguna lain", http.StatusConflict)
		}
		next = current + 1
		_, err = tx.Exec(ctx, `INSERT INTO stock_minimums(business_id,branch_id,variant_id,minimum_qty_milli,updated_by) VALUES($1,$2,$3,$4,$5)
  ON CONFLICT(business_id,branch_id,variant_id) DO UPDATE SET minimum_qty_milli=EXCLUDED.minimum_qty_milli,version=stock_minimums.version+1,updated_by=EXCLUDED.updated_by,updated_at=now()`, tc.BusinessID, branch, vid, int64(qty), tc.UserID)
		if err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "stock.minimum_changed", EntityType: "variant", EntityID: vid.String(), After: map[string]any{"minimum_qty_milli": req.Minimum, "version": next}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"variant_id": vid.String(), "branch_id": branch.String(), "minimum_qty_milli": req.Minimum, "version": next})
}

type reorderItem struct {
	Reserved     string  `json:"reserved_milli"`
	Available    string  `json:"available_milli"`
	VariantID    string  `json:"variant_id"`
	SKU          string  `json:"sku"`
	Name         string  `json:"product_name"`
	Unit         string  `json:"base_unit"`
	Balance      string  `json:"qty_milli"`
	Minimum      string  `json:"minimum_qty_milli"`
	Version      int     `json:"version"`
	SupplierID   *string `json:"last_supplier_id"`
	SupplierName *string `json:"last_supplier_name"`
}

func (s *Server) reorderList(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return
	}
	var cursor idkit.ID
	if q.Get("after") != "" {
		var err error
		cursor, err = idkit.Parse(q.Get("after"))
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return
		}
	}
	var pharmacy bool
	if err := s.pool.Underlying().QueryRow(r.Context(), `SELECT business_type='pharmacy' FROM businesses WHERE id=$1`, tc.BusinessID).Scan(&pharmacy); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if pharmacy {
		s.pharmacyReorder(w, r, tc, branch, cursor)
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT v.id::text,v.sku,p.name,v.base_unit,COALESCE(b.qty_milli,0)::text,m.minimum_qty_milli::text,m.version,last_supplier.id::text,last_supplier.name,COALESCE(b.reserved_milli,0)::text,(COALESCE(b.qty_milli,0)-COALESCE(b.reserved_milli,0))::text
 FROM stock_minimums m JOIN variants v ON v.business_id=m.business_id AND v.id=m.variant_id
 JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id
 LEFT JOIN inventory_balances b ON b.business_id=m.business_id AND b.branch_id=m.branch_id AND b.variant_id=m.variant_id
 LEFT JOIN LATERAL (
 SELECT s.id,s.name FROM goods_receipt_lines l JOIN goods_receipts g ON g.business_id=l.business_id AND g.id=l.receipt_id
 JOIN suppliers s ON s.business_id=g.business_id AND s.id=g.supplier_id
 WHERE l.business_id=m.business_id AND l.variant_id=m.variant_id AND g.branch_id=m.branch_id AND g.status='posted'
 ORDER BY g.posted_at DESC,g.id DESC LIMIT 1
 ) last_supplier ON true
 WHERE m.business_id=$1 AND m.branch_id=$2 AND v.id>$3 AND v.status='active' AND p.status='active' AND p.track_stock
 AND COALESCE(b.qty_milli,0)-COALESCE(b.reserved_milli,0)<=m.minimum_qty_milli ORDER BY v.id LIMIT 101`, tc.BusinessID, branch, cursor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	out := []reorderItem{}
	for rows.Next() {
		var x reorderItem
		if err := rows.Scan(&x.VariantID, &x.SKU, &x.Name, &x.Unit, &x.Balance, &x.Minimum, &x.Version, &x.SupplierID, &x.SupplierName, &x.Reserved, &x.Available); err != nil {
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
		next = out[99].VariantID
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out, "next_cursor": next})
}

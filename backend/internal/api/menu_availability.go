package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) availabilityRoutes(mux *http.ServeMux) {
	mux.Handle("PUT /api/v1/restaurant/menu/{id}/availability", s.restaurantGuard(true, s.setMenuAvailability))
	mux.Handle("GET /api/v1/restaurant/menu/availability", s.orderGuard(s.listMenuAvailability))
}
func (s *Server) setMenuAvailability(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, e1 := idkit.Parse(r.PathValue("id"))
	v, e2 := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if e1 != nil || e2 != nil || v < 0 {
		httpx.WriteError(w, r, errs.Validation("variant_id/If-Match wajib valid (0 baru)"))
		return
	}
	var req struct {
		Branch  string `json:"branch_id"`
		SoldOut *bool  `json:"sold_out"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.Branch)
	if !ok {
		return
	}
	if req.SoldOut == nil {
		httpx.WriteError(w, r, errs.Validation("sold_out wajib boolean"))
		return
	}
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var active bool
		err := tx.QueryRow(ctx, `SELECT v.status='active' AND p.status='active' FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id WHERE v.business_id=$1 AND v.id=$2 FOR UPDATE OF v`, tc.BusinessID, id).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("menu")
		}
		if err != nil {
			return err
		}
		if !active {
			return errs.Conflict("menu nonaktif")
		}
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
		current := 0
		err = tx.QueryRow(ctx, `SELECT version FROM restaurant_menu_availability WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, tc.BusinessID, branch, id).Scan(&current)
		if !errors.Is(err, pgx.ErrNoRows) && err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "ketersediaan berubah", http.StatusConflict)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO restaurant_menu_availability(business_id,branch_id,variant_id,sold_out,updated_by) VALUES($1,$2,$3,$4,$5) ON CONFLICT(business_id,branch_id,variant_id) DO UPDATE SET sold_out=EXCLUDED.sold_out,version=restaurant_menu_availability.version+1,updated_by=EXCLUDED.updated_by,updated_at=now()`, tc.BusinessID, branch, id, *req.SoldOut, tc.UserID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.availability_changed", EntityType: "variant", EntityID: id.String(), After: map[string]bool{"sold_out": *req.SoldOut}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"variant_id": id.String(), "sold_out": *req.SoldOut, "version": v + 1})
}
func (s *Server) listMenuAvailability(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return
	}
	var cursor idkit.ID
	if q.Get("after") != "" {
		id, err := idkit.Parse(q.Get("after"))
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return
		}
		cursor = id
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT v.id::text,p.name,v.name,COALESCE(a.sold_out,false),COALESCE(a.version,0),p.track_stock,COALESCE(b.qty_milli,0)::text,COALESCE(b.reserved_milli,0)::text,(COALESCE(b.qty_milli,0)-COALESCE(b.reserved_milli,0))::text FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id LEFT JOIN restaurant_menu_availability a ON a.business_id=v.business_id AND a.branch_id=$2 AND a.variant_id=v.id LEFT JOIN inventory_balances b ON b.business_id=v.business_id AND b.branch_id=$2 AND b.variant_id=v.id WHERE v.business_id=$1 AND v.status='active' AND p.status='active' AND v.id>$3 ORDER BY v.id LIMIT 101`, tc.BusinessID, branch, cursor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type item struct {
		ID        string `json:"variant_id"`
		Name      string `json:"product_name"`
		Variant   string `json:"variant_name"`
		SoldOut   bool   `json:"sold_out"`
		Version   int    `json:"version"`
		Track     bool   `json:"track_stock"`
		OnHand    string `json:"qty_milli"`
		Reserved  string `json:"reserved_milli"`
		Available string `json:"available_milli"`
	}
	out := []item{}
	for rows.Next() {
		var x item
		if err := rows.Scan(&x.ID, &x.Name, &x.Variant, &x.SoldOut, &x.Version, &x.Track, &x.OnHand, &x.Reserved, &x.Available); err != nil {
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
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out, "next_cursor": next})
}

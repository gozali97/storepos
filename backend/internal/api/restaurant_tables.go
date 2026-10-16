package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) restaurantGuard(write bool, h http.HandlerFunc) http.Handler {
	check := func(tc tenant.Context) error {
		if write {
			_, err := authz.Require(tc, authz.PermTableManage, idkit.ID{})
			return err
		}
		return authz.RequireRead(tc, authz.PermTableManage, idkit.ID{})
	}
	return CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(RequireModule("restaurant")(RequirePermission(check)(h)))))
}
func (s *Server) restaurantRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/restaurant/areas/{id}/restore", s.restaurantGuard(true, s.restoreArea))
	mux.Handle("POST /api/v1/restaurant/tables/{id}/restore", s.restaurantGuard(true, s.restoreTable))
	mux.Handle("DELETE /api/v1/restaurant/areas/{id}", s.restaurantGuard(true, s.archiveArea))
	mux.Handle("DELETE /api/v1/restaurant/tables/{id}", s.restaurantGuard(true, s.archiveTable))
	mux.Handle("GET /api/v1/restaurant/areas", s.restaurantGuard(false, s.listAreas))
	mux.Handle("PUT /api/v1/restaurant/areas/{id}", s.restaurantGuard(true, s.editArea))
	mux.Handle("PUT /api/v1/restaurant/tables/{id}", s.restaurantGuard(true, s.editTable))
	mux.Handle("POST /api/v1/restaurant/areas", s.restaurantGuard(true, s.createArea))
	mux.Handle("POST /api/v1/restaurant/tables", s.restaurantGuard(true, s.createTable))
	mux.Handle("GET /api/v1/restaurant/tables", s.restaurantGuard(false, s.listTables))
}
func restaurantError(err error) error {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23505" {
		return errs.Conflict("nama area/meja sudah dipakai di cabang")
	}
	return err
}
func (s *Server) createArea(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		Branch string `json:"branch_id"`
		Name   string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.Branch)
	if !ok {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 100 {
		httpx.WriteError(w, r, errs.Validation("nama area wajib maksimal 100 karakter"))
		return
	}
	id := idkit.New()
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var active bool
		err := tx.QueryRow(ctx, `SELECT status='active' FROM branches WHERE business_id=$1 AND id=$2`, tc.BusinessID, branch).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("cabang")
		}
		if err != nil {
			return err
		}
		if !active {
			return errs.Conflict("cabang nonaktif")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO restaurant_areas(id,business_id,branch_id,name) VALUES($1,$2,$3,$4)`, id, tc.BusinessID, branch, req.Name); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.area_created", EntityType: "area", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, restaurantError(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"area_id": id.String(), "version": 1})
}
func (s *Server) createTable(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		Branch   string `json:"branch_id"`
		Area     string `json:"area_id"`
		Name     string `json:"name"`
		Capacity int    `json:"capacity"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.Branch)
	if !ok {
		return
	}
	area, err := idkit.Parse(req.Area)
	req.Name = strings.TrimSpace(req.Name)
	if err != nil || req.Name == "" || utf8.RuneCountInString(req.Name) > 100 || req.Capacity < 1 || req.Capacity > 1000 {
		httpx.WriteError(w, r, errs.Validation("area/nama/kapasitas tidak valid"))
		return
	}
	id := idkit.New()
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var valid bool
		err := tx.QueryRow(ctx, `SELECT NOT a.archived AND b.status='active' FROM restaurant_areas a JOIN branches b ON b.business_id=a.business_id AND b.id=a.branch_id WHERE a.business_id=$1 AND a.branch_id=$2 AND a.id=$3 FOR SHARE OF a`, tc.BusinessID, branch, area).Scan(&valid)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("area")
		}
		if err != nil {
			return err
		}
		if !valid {
			return errs.Validation("area/cabang tidak aktif atau di luar cakupan")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO restaurant_tables(id,business_id,branch_id,area_id,name,capacity) VALUES($1,$2,$3,$4,$5,$6)`, id, tc.BusinessID, branch, area, req.Name, req.Capacity); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.table_created", EntityType: "table", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, restaurantError(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"table_id": id.String(), "version": 1})
}
func (s *Server) listTables(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return
	}
	var area *idkit.ID
	if q.Get("area_id") != "" {
		id, err := idkit.Parse(q.Get("area_id"))
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("area_id tidak valid"))
			return
		}
		area = &id
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
	archived, ok := includeArchivedRestaurant(w, r)
	if !ok {
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT t.id::text,t.area_id::text,a.name,t.name,t.capacity,t.version,t.archived,
 CASE WHEN EXISTS(SELECT 1 FROM restaurant_orders o WHERE o.business_id=t.business_id AND o.table_id=t.id AND o.status='open') THEN 'OCCUPIED'
 WHEN EXISTS(SELECT 1 FROM restaurant_orders o WHERE o.business_id=t.business_id AND o.table_id=t.id AND o.closed_at>t.cleaned_at) THEN 'NEEDS_CLEANING' ELSE 'AVAILABLE' END FROM restaurant_tables t JOIN restaurant_areas a ON a.business_id=t.business_id AND a.branch_id=t.branch_id AND a.id=t.area_id WHERE t.business_id=$1 AND t.branch_id=$2 AND ($5 OR (NOT t.archived AND NOT a.archived)) AND ($3::uuid IS NULL OR t.area_id=$3) AND t.id>$4 ORDER BY t.id LIMIT 101`, tc.BusinessID, branch, area, cursor, archived)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type table struct {
		ID       string `json:"table_id"`
		Area     string `json:"area_id"`
		AreaName string `json:"area_name"`
		Name     string `json:"name"`
		Capacity int    `json:"capacity"`
		Version  int    `json:"version"`
		Status   string `json:"status"`
		Archived bool   `json:"archived"`
	}
	out := []table{}
	for rows.Next() {
		var t table
		if err := rows.Scan(&t.ID, &t.Area, &t.AreaName, &t.Name, &t.Capacity, &t.Version, &t.Archived, &t.Status); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, t)
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
	httpx.JSON(w, http.StatusOK, map[string]any{"tables": out, "next_cursor": next})
}

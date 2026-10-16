package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) kitchenReadGuard(h http.HandlerFunc) http.Handler {
	return CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(RequireModule("restaurant")(RequirePermission(func(tc tenant.Context) error { return authz.RequireRead(tc, authz.PermKitchenUpdate, idkit.ID{}) })(h)))))
}
func (s *Server) stationRoutes(mux *http.ServeMux) {
	mux.Handle("DELETE /api/v1/restaurant/category-routes/{id}", s.restaurantGuard(true, s.deleteCategoryRoute))
	mux.Handle("PUT /api/v1/restaurant/stations/{id}", s.restaurantGuard(true, s.editStation))
	mux.Handle("GET /api/v1/restaurant/category-routes", s.restaurantGuard(false, s.listCategoryRoutes))
	mux.Handle("POST /api/v1/restaurant/stations", s.restaurantGuard(true, s.createStation))
	mux.Handle("GET /api/v1/restaurant/stations", s.kitchenReadGuard(s.listStations))
	mux.Handle("PUT /api/v1/restaurant/category-routes/{id}", s.restaurantGuard(true, s.setCategoryRoute))
}
func (s *Server) createStation(w http.ResponseWriter, r *http.Request) {
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
		httpx.WriteError(w, r, errs.Validation("nama stasiun wajib maksimal 100 karakter"))
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
		if _, err := tx.Exec(ctx, `INSERT INTO kitchen_stations(id,business_id,branch_id,name) VALUES($1,$2,$3,$4)`, id, tc.BusinessID, branch, req.Name); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "kitchen.station_created", EntityType: "station", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, restaurantError(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"station_id": id.String(), "version": 1})
}
func (s *Server) setCategoryRoute(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	category, e1 := idkit.Parse(r.PathValue("id"))
	v, e2 := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if e1 != nil || e2 != nil || v < 0 {
		httpx.WriteError(w, r, errs.Validation("category_id/If-Match tidak valid; 0 untuk mapping baru"))
		return
	}
	var req struct {
		Branch  string `json:"branch_id"`
		Station string `json:"station_id"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.Branch)
	if !ok {
		return
	}
	station, err := idkit.Parse(req.Station)
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("station_id tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var active bool
		err := tx.QueryRow(ctx, `SELECT status='active' FROM categories WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, category).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("kategori")
		}
		if err != nil {
			return err
		}
		if !active {
			return errs.Conflict("kategori diarsipkan")
		}
		err = tx.QueryRow(ctx, `SELECT NOT archived FROM kitchen_stations WHERE business_id=$1 AND branch_id=$2 AND id=$3 FOR SHARE`, tc.BusinessID, branch, station).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("stasiun cabang")
		}
		if err != nil {
			return err
		}
		if !active {
			return errs.Conflict("stasiun diarsipkan")
		}
		current := 0
		var old *string
		err = tx.QueryRow(ctx, `SELECT version,station_id::text FROM kitchen_category_routes WHERE business_id=$1 AND branch_id=$2 AND category_id=$3`, tc.BusinessID, branch, category).Scan(&current, &old)
		if !errors.Is(err, pgx.ErrNoRows) && err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "mapping kategori berubah", http.StatusConflict)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO kitchen_category_routes(business_id,branch_id,category_id,station_id) VALUES($1,$2,$3,$4) ON CONFLICT(business_id,branch_id,category_id) DO UPDATE SET station_id=EXCLUDED.station_id,version=kitchen_category_routes.version+1`, tc.BusinessID, branch, category, station); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "kitchen.category_routed", EntityType: "category", EntityID: category.String(), Before: map[string]any{"station_id": old}, After: map[string]string{"station_id": station.String()}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"category_id": category.String(), "station_id": station.String(), "version": v + 1})
}
func (s *Server) listStations(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	branch, ok := branchOf(w, r, tc, r.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	var cursor idkit.ID
	if raw := r.URL.Query().Get("after"); raw != "" {
		id, err := idkit.Parse(raw)
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
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,name,version,archived FROM kitchen_stations WHERE business_id=$1 AND branch_id=$2 AND ($4 OR NOT archived) AND id>$3 ORDER BY id LIMIT 101`, tc.BusinessID, branch, cursor, archived)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type station struct {
		ID       string `json:"station_id"`
		Name     string `json:"name"`
		Version  int    `json:"version"`
		Archived bool   `json:"archived"`
	}
	out := []station{}
	for rows.Next() {
		var x station
		if err := rows.Scan(&x.ID, &x.Name, &x.Version, &x.Archived); err != nil {
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
	httpx.JSON(w, http.StatusOK, map[string]any{"stations": out, "next_cursor": next})
}

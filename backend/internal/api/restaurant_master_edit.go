package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) listAreas(w http.ResponseWriter, r *http.Request) {
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
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,name,version,archived FROM restaurant_areas WHERE business_id=$1 AND branch_id=$2 AND ($4 OR NOT archived) AND id>$3 ORDER BY id LIMIT 101`, tc.BusinessID, branch, cursor, archived)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type area struct {
		ID       string `json:"area_id"`
		Name     string `json:"name"`
		Version  int    `json:"version"`
		Archived bool   `json:"archived"`
	}
	out := []area{}
	for rows.Next() {
		var a area
		if err := rows.Scan(&a.ID, &a.Name, &a.Version, &a.Archived); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, a)
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
	httpx.JSON(w, http.StatusOK, map[string]any{"areas": out, "next_cursor": next})
}
func (s *Server) editArea(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 100 {
		httpx.WriteError(w, r, errs.Validation("nama area wajib maksimal 100 karakter"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		var old string
		var current int
		var archived bool
		err := tx.QueryRow(ctx, `SELECT branch_id,name,version,archived FROM restaurant_areas WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&branch, &old, &current, &archived)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("area")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		if current != v || archived {
			return errs.New(errs.CodeOptimisticLockFailure, "area berubah atau diarsipkan", http.StatusConflict)
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_areas SET name=$3,version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Name); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.area_updated", EntityType: "area", EntityID: id.String(), Before: map[string]string{"name": old}, After: map[string]string{"name": req.Name}})
	})
	if err != nil {
		httpx.WriteError(w, r, restaurantError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"area_id": id.String(), "version": v + 1})
}
func (s *Server) editTable(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Name     string `json:"name"`
		Capacity int    `json:"capacity"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 100 || req.Capacity < 1 || req.Capacity > 1000 {
		httpx.WriteError(w, r, errs.Validation("nama/kapasitas meja tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		var old string
		var capacity, current int
		var archived bool
		err := tx.QueryRow(ctx, `SELECT branch_id,name,capacity,version,archived FROM restaurant_tables WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&branch, &old, &capacity, &current, &archived)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("meja")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		if current != v || archived {
			return errs.New(errs.CodeOptimisticLockFailure, "meja berubah atau diarsipkan", http.StatusConflict)
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_tables SET name=$3,capacity=$4,version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Name, req.Capacity); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.table_updated", EntityType: "table", EntityID: id.String(), Before: map[string]any{"name": old, "capacity": capacity}, After: map[string]any{"name": req.Name, "capacity": req.Capacity}})
	})
	if err != nil {
		httpx.WriteError(w, r, restaurantError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"table_id": id.String(), "version": v + 1})
}

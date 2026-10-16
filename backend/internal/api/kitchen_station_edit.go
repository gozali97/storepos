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

func (s *Server) editStation(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Name     string `json:"name"`
		Archived bool   `json:"archived"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 100 {
		httpx.WriteError(w, r, errs.Validation("nama stasiun wajib maksimal 100 karakter"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		var current int
		var old string
		var archived bool
		err := tx.QueryRow(ctx, `SELECT branch_id,version,name,archived FROM kitchen_stations WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&branch, &current, &old, &archived)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("stasiun")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "stasiun berubah", http.StatusConflict)
		}
		if req.Archived {
			var mapped, busy bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM kitchen_category_routes WHERE business_id=$1 AND station_id=$2),EXISTS(SELECT 1 FROM kitchen_tickets t JOIN kitchen_ticket_items i ON i.business_id=t.business_id AND i.ticket_id=t.id JOIN restaurant_order_lines l ON l.business_id=i.business_id AND l.id=i.line_id WHERE t.business_id=$1 AND t.station_id=$2 AND l.kitchen_status IN ('SENT','IN_PROGRESS','READY'))`, tc.BusinessID, id).Scan(&mapped, &busy); err != nil {
				return err
			}
			if mapped || busy {
				return errs.Conflict("pindahkan routing kategori dan selesaikan tiket aktif sebelum arsip stasiun")
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE kitchen_stations SET name=$3,archived=$4,version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Name, req.Archived); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "kitchen.station_updated", EntityType: "station", EntityID: id.String(), Before: map[string]any{"name": old, "archived": archived}, After: map[string]any{"name": req.Name, "archived": req.Archived}})
	})
	if err != nil {
		httpx.WriteError(w, r, restaurantError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"station_id": id.String(), "version": v + 1, "archived": req.Archived})
}
func (s *Server) listCategoryRoutes(w http.ResponseWriter, r *http.Request) {
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
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT r.category_id::text,c.name,r.station_id::text,s.name,r.version FROM kitchen_category_routes r JOIN categories c ON c.business_id=r.business_id AND c.id=r.category_id JOIN kitchen_stations s ON s.business_id=r.business_id AND s.branch_id=r.branch_id AND s.id=r.station_id WHERE r.business_id=$1 AND r.branch_id=$2 AND r.category_id>$3 ORDER BY r.category_id LIMIT 101`, tc.BusinessID, branch, cursor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type route struct {
		Category    string `json:"category_id"`
		Name        string `json:"category_name"`
		Station     string `json:"station_id"`
		StationName string `json:"station_name"`
		Version     int    `json:"version"`
	}
	out := []route{}
	for rows.Next() {
		var x route
		if err := rows.Scan(&x.Category, &x.Name, &x.Station, &x.StationName, &x.Version); err != nil {
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
		next = out[99].Category
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"routes": out, "next_cursor": next})
}

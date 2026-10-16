package apiserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) restoreArea(w http.ResponseWriter, r *http.Request) {
	s.restoreRestaurantMaster(w, r, true)
}
func (s *Server) restoreTable(w http.ResponseWriter, r *http.Request) {
	s.restoreRestaurantMaster(w, r, false)
}
func (s *Server) restoreRestaurantMaster(w http.ResponseWriter, r *http.Request, isArea bool) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	table, entity := "restaurant_tables", "table"
	if isArea {
		table, entity = "restaurant_areas", "area"
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		var area *idkit.ID
		var current int
		var archived bool
		if isArea {
			err = tx.QueryRow(ctx, `SELECT branch_id,version,archived FROM restaurant_areas WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&branch, &current, &archived)
		} else {
			// Area lock first matches create/archive lock order.
			var a idkit.ID
			err = tx.QueryRow(ctx, `SELECT area_id FROM restaurant_tables WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&a)
			if err == nil {
				var active bool
				err = tx.QueryRow(ctx, `SELECT NOT archived FROM restaurant_areas WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, a).Scan(&active)
				if err == nil && !active {
					return errs.Conflict("reaktifkan area dahulu")
				}
				area = &a
			}
			if err == nil {
				err = tx.QueryRow(ctx, `SELECT branch_id,version,archived FROM restaurant_tables WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&branch, &current, &archived)
			}
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound(entity)
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		if current != v || !archived {
			return errs.New(errs.CodeOptimisticLockFailure, "master berubah atau sudah aktif", http.StatusConflict)
		}
		var active bool
		err = tx.QueryRow(ctx, `SELECT status='active' FROM branches WHERE business_id=$1 AND id=$2`, tc.BusinessID, branch).Scan(&active)
		if err != nil {
			return err
		}
		if !active {
			return errs.Conflict("cabang nonaktif")
		}
		if _, err := tx.Exec(ctx, `UPDATE `+table+` SET archived=false,version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant." + entity + "_restored", EntityType: entity, EntityID: id.String(), After: map[string]any{"area_id": area, "version": v + 1}})
	})
	if err != nil {
		httpx.WriteError(w, r, restaurantError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id.String(), "archived": false, "version": v + 1})
}

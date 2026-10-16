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

func (s *Server) archiveArea(w http.ResponseWriter, r *http.Request) {
	s.archiveRestaurantMaster(w, r, true)
}
func (s *Server) archiveTable(w http.ResponseWriter, r *http.Request) {
	s.archiveRestaurantMaster(w, r, false)
}
func (s *Server) archiveRestaurantMaster(w http.ResponseWriter, r *http.Request, area bool) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	table, entity := "restaurant_tables", "table"
	if area {
		table, entity = "restaurant_areas", "area"
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		var current int
		var archived bool
		err := tx.QueryRow(ctx, `SELECT branch_id,version,archived FROM `+table+` WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&branch, &current, &archived)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound(entity)
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		if current != v || archived {
			return errs.New(errs.CodeOptimisticLockFailure, "master berubah atau sudah diarsipkan", http.StatusConflict)
		}
		var blocked bool
		if area {
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM restaurant_tables WHERE business_id=$1 AND area_id=$2 AND NOT archived)`, tc.BusinessID, id).Scan(&blocked)
		} else {
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM restaurant_orders WHERE business_id=$1 AND table_id=$2 AND status='open')`, tc.BusinessID, id).Scan(&blocked)
		}
		if err != nil {
			return err
		}
		if blocked {
			return errs.Conflict("arsipkan meja area dahulu; meja dengan pesanan terbuka tidak dapat diarsipkan")
		}
		if _, err := tx.Exec(ctx, `UPDATE `+table+` SET archived=true,version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant." + entity + "_archived", EntityType: entity, EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id.String(), "archived": true, "version": v + 1})
}

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

func (s *Server) deleteCategoryRoute(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, e1 := idkit.Parse(r.PathValue("id"))
	v, e2 := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if e1 != nil || e2 != nil || v <= 0 {
		httpx.WriteError(w, r, errs.Validation("kategori/If-Match tidak valid"))
		return
	}
	branch, ok := branchOf(w, r, tc, r.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var category idkit.ID
		err := tx.QueryRow(ctx, `SELECT id FROM categories WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&category)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("kategori")
		}
		if err != nil {
			return err
		}
		var current int
		var station idkit.ID
		err = tx.QueryRow(ctx, `SELECT version,station_id FROM kitchen_category_routes WHERE business_id=$1 AND branch_id=$2 AND category_id=$3 FOR UPDATE`, tc.BusinessID, branch, id).Scan(&current, &station)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("routing")
		}
		if err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "routing berubah", http.StatusConflict)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM kitchen_category_routes WHERE business_id=$1 AND branch_id=$2 AND category_id=$3`, tc.BusinessID, branch, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "kitchen.category_unrouted", EntityType: "category", EntityID: id.String(), Before: map[string]string{"station_id": station.String()}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/org"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) businessTypeRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/settings/business-type", s.guard(authz.PermBusinessManage, false, s.getBusinessType))
	mux.Handle("PUT /api/v1/settings/business-type", s.guard(authz.PermBusinessManage, true, s.putBusinessType))
}
func (s *Server) getBusinessType(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var kind string
	var modules []string
	var version int
	var first *time.Time
	err := s.pool.Underlying().QueryRow(r.Context(), `SELECT business_type,enabled_modules,org_version,first_sale_completed_at FROM businesses WHERE id=$1`, tc.BusinessID).Scan(&kind, &modules, &version, &first)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"business_type": kind, "enabled_modules": modules, "version": version, "locked": first != nil, "first_sale_completed_at": first})
}
func (s *Server) putBusinessType(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	v, err := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if err != nil || v <= 0 {
		httpx.WriteError(w, r, errs.Validation("If-Match wajib versi positif"))
		return
	}
	var req struct {
		Type string `json:"business_type"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	typ := org.BusinessType(req.Type)
	if !typ.Valid() {
		httpx.WriteError(w, r, errs.Validation("business_type tidak dikenal"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current int
		var old string
		var first *time.Time
		if err := tx.QueryRow(ctx, `SELECT org_version,business_type,first_sale_completed_at FROM businesses WHERE id=$1 FOR UPDATE`, tc.BusinessID).Scan(&current, &old, &first); err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "pengaturan berubah", http.StatusConflict)
		}
		if first != nil {
			return org.ErrTypeLocked
		}
		if err := org.NewRepo(tx).UpdateBusinessType(ctx, tc.BusinessID, typ); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE businesses SET org_version=org_version+1 WHERE id=$1`, tc.BusinessID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "business.type_changed", EntityType: "business", EntityID: tc.BusinessID.String(), Before: map[string]string{"business_type": old}, After: map[string]any{"business_type": req.Type, "enabled_modules": org.ModulesFor(typ)}})
	})
	if errors.Is(err, org.ErrTypeLocked) {
		httpx.WriteError(w, r, errs.Conflict(err.Error()))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"business_type": req.Type, "enabled_modules": org.ModulesFor(typ), "version": v + 1})
}

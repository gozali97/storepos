package apiserver

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/org"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) policyRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/settings/operational-policy", s.guard(authz.PermBusinessManage, false, s.getPolicy))
	mux.Handle("PUT /api/v1/settings/operational-policy", s.guard(authz.PermBusinessManage, true, s.putPolicy))
}
func (s *Server) getPolicy(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var p org.Policy
	var version int
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT policy_version FROM businesses WHERE id=$1 FOR SHARE`, tc.BusinessID).Scan(&version); err != nil {
			return err
		}
		var err error
		p, err = org.LoadPolicy(ctx, tx, tc.BusinessID)
		return err
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"policy": p, "version": version})
}
func (s *Server) putPolicy(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	req := org.DefaultPolicy()
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := req.Validate(); err != nil {
		httpx.WriteError(w, r, errs.Validation(err.Error()))
		return
	}
	v, err := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if err != nil || v <= 0 {
		httpx.WriteError(w, r, errs.Validation("If-Match wajib versi positif"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current int
		if err := tx.QueryRow(ctx, `SELECT policy_version FROM businesses WHERE id=$1 FOR UPDATE`, tc.BusinessID).Scan(&current); err != nil {
			return err
		}
		if v != current {
			return errs.New(errs.CodeOptimisticLockFailure, "pengaturan diubah pengguna lain", http.StatusConflict)
		}
		old, err := org.LoadPolicy(ctx, tx, tc.BusinessID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE businesses SET settings=jsonb_set(settings,'{operational_policy}',$2::jsonb,true),policy_version=policy_version+1,updated_at=now() WHERE id=$1`, tc.BusinessID, req); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "business.policy_changed", EntityType: "business", EntityID: tc.BusinessID.String(), Before: old, After: req})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"policy": req, "version": v + 1})
}

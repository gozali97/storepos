package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/catalog"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) combinationRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/catalog/variant-combinations", s.guard(authz.PermProductManage, true, s.generateCombinations))
	mux.Handle("PATCH /api/v1/variants/{id}/status", s.guard(authz.PermProductManage, true, s.variantStatus))
}
func (s *Server) generateCombinations(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prefix     string              `json:"sku_prefix"`
		Attributes []catalog.Attribute `json:"attributes"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := catalog.Generate(req.Prefix, req.Attributes)
	if err != nil {
		httpx.WriteError(w, r, errs.Validation(err.Error()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"variants": out})
}
func (s *Server) variantStatus(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, e1 := idkit.Parse(r.PathValue("id"))
	v, e2 := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if e1 != nil || e2 != nil || v <= 0 {
		httpx.WriteError(w, r, errs.Validation("id/If-Match tidak valid"))
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Status != "active" && req.Status != "inactive" {
		httpx.WriteError(w, r, errs.Validation("status harus active/inactive"))
		return
	}
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current int
		var old string
		err := tx.QueryRow(ctx, `SELECT version,status FROM variants WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&current, &old)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("varian")
		}
		if err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "varian diubah pengguna lain", http.StatusConflict)
		}
		if old == "archived" {
			return errs.Conflict("varian diarsipkan tidak dapat diaktifkan melalui endpoint ini")
		}
		if _, err := tx.Exec(ctx, `UPDATE variants SET status=$3,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Status); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "variant.status_changed", EntityType: "variant", EntityID: id.String(), Before: map[string]string{"status": old}, After: map[string]string{"status": req.Status}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id.String(), "status": req.Status, "version": v + 1})
}

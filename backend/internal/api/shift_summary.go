package apiserver

import (
	"github.com/storepos/storepos/internal/platform/authz"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/sales"
	"net/http"
)

func (s *Server) shiftSummary(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("shift_id tidak valid"))
		return
	}
	all := authz.Check(tc.Role, authz.PermShiftViewAll) == authz.Allowed
	result, err := sales.Summary(r.Context(), s.pool.Underlying(), tc.BusinessID, id, tc.UserID, all, tc.HasBranch)
	if err != nil {
		httpx.WriteError(w, r, mapShiftError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

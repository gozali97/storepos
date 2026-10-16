package apiserver

import (
	"github.com/storepos/storepos/internal/platform/authz"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/tenant"
	"net/http"
	"strconv"
)

func includeArchivedRestaurant(w http.ResponseWriter, r *http.Request) (bool, bool) {
	raw := r.URL.Query().Get("include_archived")
	if raw == "" {
		return false, true
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("include_archived harus boolean"))
		return false, false
	}
	tc := tenant.MustFromContext(r.Context())
	if enabled && authz.Check(tc.Role, authz.PermTableManage) != authz.Allowed {
		httpx.WriteError(w, r, errs.PermissionDenied("master arsip hanya untuk pengelola restoran"))
		return false, false
	}
	return enabled, true
}

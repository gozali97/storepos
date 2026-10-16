package apiserver

import (
	"net/http"

	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

// guard memasang rantai penuh PRD §16.5: CSRF → auth → tenant → izin.
// write=true memakai authz.Require (ReadOnly ditolak); false memakai RequireRead.
func (s *Server) guard(perm authz.Permission, write bool, h http.HandlerFunc) http.Handler {
	check := func(tc tenant.Context) error {
		if write {
			_, err := authz.Require(tc, perm, idkit.ID{})
			return err
		}
		return authz.RequireRead(tc, perm, idkit.ID{})
	}
	return CSRFMiddleware(s.secure)(
		RequireAuth(s)(
			RequireTenant(s)(
				RequirePermission(check)(h))))
}

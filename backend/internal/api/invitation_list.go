package apiserver

import (
	"github.com/storepos/storepos/internal/platform/authz"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
	"net/http"
	"time"
)

func (s *Server) invitationListRoutes(mux *http.ServeMux) {
	h := http.HandlerFunc(s.listInvitations)
	mux.Handle("GET /api/v1/invitations", CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(RequirePermission(func(tc tenant.Context) error {
		if authz.Check(tc.Role, authz.PermUserManage) == authz.Allowed || authz.Check(tc.Role, authz.PermUserManageLimited) == authz.Allowed {
			return nil
		}
		return errs.PermissionDenied("")
	})(h)))))
}
func (s *Server) listInvitations(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var cursor idkit.ID
	if raw := r.URL.Query().Get("after"); raw != "" {
		id, err := idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return
		}
		cursor = id
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,contact,role,ARRAY(SELECT b::text FROM unnest(branch_ids) b),expires_at,accepted_at,revoked_at FROM invitations WHERE business_id=$1 AND id>$2 AND ($3 OR invited_by=$4) ORDER BY id LIMIT 101`, tc.BusinessID, cursor, tc.Role == tenant.RoleOwner, tc.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type invitation struct {
		ID       string     `json:"invitation_id"`
		Contact  string     `json:"contact"`
		Role     string     `json:"role"`
		Branches []string   `json:"branch_ids"`
		Expires  time.Time  `json:"expires_at"`
		Accepted *time.Time `json:"accepted_at"`
		Revoked  *time.Time `json:"revoked_at"`
	}
	out := []invitation{}
	for rows.Next() {
		var x invitation
		if err := rows.Scan(&x.ID, &x.Contact, &x.Role, &x.Branches, &x.Expires, &x.Accepted, &x.Revoked); err != nil {
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
		next = out[99].ID
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"invitations": out, "next_cursor": next})
}

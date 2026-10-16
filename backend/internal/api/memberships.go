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

func (s *Server) membershipRoutes(mux *http.ServeMux) {
	protect := func(h http.HandlerFunc) http.Handler {
		return CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(RequirePermission(func(tc tenant.Context) error {
			if tc.Role != tenant.RoleOwner && tc.Role != tenant.RoleManager {
				return errs.PermissionDenied("")
			}
			return nil
		})(h))))
	}
	mux.Handle("PUT /api/v1/memberships/{id}", protect(s.updateMembership))
	mux.Handle("GET /api/v1/memberships", protect(s.listMemberships))
}
func validMemberRole(role string) bool {
	switch tenant.Role(role) {
	case tenant.RoleOwner, tenant.RoleManager, tenant.RoleCashier, tenant.RoleWarehouse, tenant.RoleKitchen, tenant.RolePharmacist:
		return true
	}
	return false
}
func (s *Server) updateMembership(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, e1 := idkit.Parse(r.PathValue("id"))
	version, e2 := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if e1 != nil || e2 != nil || version <= 0 {
		httpx.WriteError(w, r, errs.Validation("id/If-Match wajib valid"))
		return
	}
	var req struct {
		Role     string   `json:"role"`
		Status   string   `json:"status"`
		Branches []string `json:"branch_ids"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !validMemberRole(req.Role) || (req.Status != "active" && req.Status != "disabled") || len(req.Branches) > 1000 {
		httpx.WriteError(w, r, errs.Validation("peran/status/cabang tidak valid"))
		return
	}
	branches := []idkit.ID{}
	seen := map[idkit.ID]bool{}
	for _, raw := range req.Branches {
		b, err := idkit.Parse(raw)
		if err != nil || seen[b] {
			httpx.WriteError(w, r, errs.Validation("branch_ids invalid atau duplikat"))
			return
		}
		seen[b] = true
		branches = append(branches, b)
	}
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		// Tenant parent lock serializes competing owner demotions.
		if _, err := tx.Exec(ctx, `SELECT id FROM businesses WHERE id=$1 FOR UPDATE`, tc.BusinessID); err != nil {
			return err
		}
		// Recheck actor under the same lock; middleware state may predate a concurrent demotion.
		var actorRole, actorStatus string
		if err := tx.QueryRow(ctx, `SELECT role,status FROM memberships WHERE business_id=$1 AND id=$2`, tc.BusinessID, tc.MembershipID).Scan(&actorRole, &actorStatus); err != nil {
			return err
		}
		if actorStatus != "active" || (actorRole != "owner" && actorRole != "manager") {
			return errs.PermissionDenied("")
		}
		var oldRole, oldStatus string
		var current int
		var user idkit.ID
		err := tx.QueryRow(ctx, `SELECT role,status,version,user_id FROM memberships WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&oldRole, &oldStatus, &current, &user)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("keanggotaan")
		}
		if err != nil {
			return err
		}
		if current != version {
			return errs.New(errs.CodeOptimisticLockFailure, "keanggotaan diubah pengguna lain", http.StatusConflict)
		}
		if actorRole == "manager" {
			rows, err := tx.Query(ctx, `SELECT branch_id FROM membership_branches WHERE business_id=$1 AND membership_id=$2`, tc.BusinessID, tc.MembershipID)
			if err != nil {
				return err
			}
			tc.BranchIDs = nil
			for rows.Next() {
				var b idkit.ID
				if err := rows.Scan(&b); err != nil {
					rows.Close()
					return err
				}
				tc.BranchIDs = append(tc.BranchIDs, b)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
		}
		if actorRole == "manager" && (oldRole == "owner" || oldRole == "manager" || req.Role == "owner" || req.Role == "manager") {
			return errs.PermissionDenied("Manajer tidak boleh mengubah Pemilik/Manajer")
		}
		if oldRole == "owner" && oldStatus == "active" && (req.Role != "owner" || req.Status != "active") {
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM memberships m JOIN users u ON u.id=m.user_id WHERE m.business_id=$1 AND m.role='owner' AND m.status='active' AND u.status='active'`, tc.BusinessID).Scan(&n); err != nil {
				return err
			}
			if n <= 1 {
				return errs.Conflict("Pemilik aktif terakhir tidak dapat dinonaktifkan atau diturunkan perannya")
			}
		}
		for _, b := range branches {
			if actorRole == "manager" && !tc.HasBranch(b) {
				return errs.PermissionDenied("Manajer hanya boleh menugaskan cabang dalam cakupannya")
			}
			var active bool
			err := tx.QueryRow(ctx, `SELECT status='active' FROM branches WHERE business_id=$1 AND id=$2`, tc.BusinessID, b).Scan(&active)
			if errors.Is(err, pgx.ErrNoRows) || err == nil && !active {
				return errs.Validation("cabang tidak aktif atau bukan milik bisnis")
			}
			if err != nil {
				return err
			}
		}
		if actorRole == "manager" {
			rows, err := tx.Query(ctx, `SELECT branch_id FROM membership_branches WHERE business_id=$1 AND membership_id=$2`, tc.BusinessID, id)
			if err != nil {
				return err
			}
			allowed := true
			for rows.Next() {
				var b idkit.ID
				if err := rows.Scan(&b); err != nil {
					rows.Close()
					return err
				}
				if !tc.HasBranch(b) {
					allowed = false
				}
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			if !allowed {
				return errs.PermissionDenied("anggota memiliki cabang di luar cakupan Manajer")
			}
		}
		oldBranches := []string{}
		rows, err := tx.Query(ctx, `SELECT branch_id::text FROM membership_branches WHERE business_id=$1 AND membership_id=$2 ORDER BY branch_id`, tc.BusinessID, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var b string
			if err := rows.Scan(&b); err != nil {
				rows.Close()
				return err
			}
			oldBranches = append(oldBranches, b)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE memberships SET role=$3,status=$4,version=version+1,updated_at=now(),pin_hash=CASE WHEN role<>$3 OR $4='disabled' THEN NULL ELSE pin_hash END WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Role, req.Status); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM membership_branches WHERE business_id=$1 AND membership_id=$2`, tc.BusinessID, id); err != nil {
			return err
		}
		for _, b := range branches {
			if _, err := tx.Exec(ctx, `INSERT INTO membership_branches(membership_id,business_id,branch_id) VALUES($1,$2,$3)`, id, tc.BusinessID, b); err != nil {
				return err
			}
		}
		if req.Status == "disabled" {
			// Revoke sessions scoped to this business; multi-business sessions are still denied by tenant resolution.
			if _, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE user_id=$1 AND active_business_id=$2 AND revoked_at IS NULL`, user, tc.BusinessID); err != nil {
				return err
			}
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "membership.changed", EntityType: "membership", EntityID: id.String(), Before: map[string]any{"role": oldRole, "status": oldStatus, "branch_ids": oldBranches}, After: map[string]any{"role": req.Role, "status": req.Status, "branch_ids": req.Branches}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"membership_id": id.String(), "version": version + 1})
}
func (s *Server) listMemberships(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var cursor idkit.ID
	if raw := r.URL.Query().Get("after"); raw != "" {
		var err error
		cursor, err = idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor invalid"))
			return
		}
	}
	branches := []string{}
	for _, b := range tc.BranchIDs {
		branches = append(branches, b.String())
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT m.id::text,m.user_id::text,m.role,m.status,m.version,
 COALESCE((SELECT array_agg(b.branch_id::text ORDER BY b.branch_id) FROM membership_branches b WHERE b.business_id=m.business_id AND b.membership_id=m.id),ARRAY[]::text[])
 FROM memberships m WHERE m.business_id=$1 AND m.id>$2 AND ($3 OR (m.role NOT IN ('owner','manager') AND NOT EXISTS(SELECT 1 FROM membership_branches b WHERE b.business_id=m.business_id AND b.membership_id=m.id AND NOT(b.branch_id=ANY($4::uuid[]))))) ORDER BY m.id LIMIT 101`, tc.BusinessID, cursor, tc.Role == tenant.RoleOwner, branches)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type member struct {
		ID       string   `json:"membership_id"`
		User     string   `json:"user_id"`
		Role     string   `json:"role"`
		Status   string   `json:"status"`
		Version  int      `json:"version"`
		Branches []string `json:"branch_ids"`
	}
	out := []member{}
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.ID, &m.User, &m.Role, &m.Status, &m.Version, &m.Branches); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, m)
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
	httpx.JSON(w, http.StatusOK, map[string]any{"memberships": out, "next_cursor": next})
}

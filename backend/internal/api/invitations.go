package apiserver

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/identity"
	"github.com/storepos/storepos/internal/platform/auth"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) invitationRoutes(mux *http.ServeMux) {
	manage := func(h http.HandlerFunc) http.Handler {
		return CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(RequirePermission(func(tc tenant.Context) error {
			if tc.Role != tenant.RoleOwner && tc.Role != tenant.RoleManager {
				return errs.PermissionDenied("")
			}
			return nil
		})(h))))
	}
	mux.Handle("POST /api/v1/invitations", manage(s.createInvitation))
	mux.Handle("DELETE /api/v1/invitations/{id}", manage(s.revokeInvitation))
	mux.Handle("POST /api/v1/invitations/accept", CSRFMiddleware(s.secure)(RequireAuth(s)(http.HandlerFunc(s.acceptInvitation))))
}
func (s *Server) createInvitation(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		Email    string   `json:"email"`
		Role     string   `json:"role"`
		Branches []string `json:"branch_ids"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	email := identity.NormalizeEmail(req.Email)
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || len(email) > 254 || !validMemberRole(req.Role) || len(req.Branches) > 1000 {
		httpx.WriteError(w, r, errs.Validation("email/peran/cabang tidak valid"))
		return
	}
	branches := []string{}
	seen := map[string]bool{}
	for _, raw := range req.Branches {
		id, err := idkit.Parse(raw)
		if err != nil || seen[id.String()] {
			httpx.WriteError(w, r, errs.Validation("cabang tidak valid/duplikat"))
			return
		}
		seen[id.String()] = true
		branches = append(branches, id.String())
	}
	token, err := auth.NewInvitationToken()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	id := idkit.New()
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT id FROM businesses WHERE id=$1 FOR UPDATE`, tc.BusinessID); err != nil {
			return err
		}
		var role, status string
		if err := tx.QueryRow(ctx, `SELECT role,status FROM memberships WHERE business_id=$1 AND id=$2`, tc.BusinessID, tc.MembershipID).Scan(&role, &status); err != nil {
			return err
		}
		if status != "active" || (role != "owner" && role != "manager") {
			return errs.PermissionDenied("")
		}
		if role == "manager" && (req.Role == "owner" || req.Role == "manager") {
			return errs.PermissionDenied("Manajer tidak boleh mengundang Pemilik/Manajer")
		}
		for _, b := range branches {
			var active bool
			err := tx.QueryRow(ctx, `SELECT status='active' FROM branches WHERE business_id=$1 AND id=$2`, tc.BusinessID, b).Scan(&active)
			if errors.Is(err, pgx.ErrNoRows) || err == nil && !active {
				return errs.Validation("cabang tidak aktif atau bukan milik bisnis")
			}
			if err != nil {
				return err
			}
			if role == "manager" {
				var assigned bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM membership_branches WHERE business_id=$1 AND membership_id=$2 AND branch_id=$3)`, tc.BusinessID, tc.MembershipID, b).Scan(&assigned); err != nil {
					return err
				}
				if !assigned {
					return errs.PermissionDenied("cabang di luar cakupan Manajer")
				}
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO invitations(id,business_id,contact,role,branch_ids,invited_by,token_hash,expires_at) VALUES($1,$2,$3,$4,$5::uuid[],$6,$7,$8)`, id, tc.BusinessID, email, req.Role, branches, tc.UserID, auth.HashToken(token), expires); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "invitation.created", EntityType: "invitation", EntityID: id.String(), After: map[string]any{"role": req.Role, "branch_ids": branches}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Token returned once to the authorized creator; never logged or written into audit.
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusCreated, map[string]any{"invitation_id": id.String(), "token": token, "expires_at": expires, "delivery": "manual"})
}
func (s *Server) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	session := MustSession(r.Context())
	user, err := idkit.Parse(session.UserID)
	if err != nil {
		httpx.WriteError(w, r, errs.Unauthenticated(""))
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(req.Token) < 32 || len(req.Token) > 128 {
		httpx.WriteError(w, r, errs.Validation("token undangan tidak valid"))
		return
	}
	var membership, biz idkit.ID
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var id, inviter idkit.ID
		var email, role string
		var branches []string
		var expires time.Time
		var accepted, revoked *time.Time
		// Discover tenant, then follow the same tenant-before-membership lock order as management.
		err := tx.QueryRow(ctx, `SELECT business_id FROM invitations WHERE token_hash=$1`, auth.HashToken(req.Token)).Scan(&biz)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("undangan")
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT id FROM businesses WHERE id=$1 FOR UPDATE`, biz); err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `SELECT id,contact,role,ARRAY(SELECT b::text FROM unnest(branch_ids) b),invited_by,expires_at,accepted_at,revoked_at FROM invitations WHERE token_hash=$1 FOR UPDATE`, auth.HashToken(req.Token)).Scan(&id, &email, &role, &branches, &inviter, &expires, &accepted, &revoked)
		if err != nil {
			return err
		}
		if accepted != nil || revoked != nil || !expires.After(time.Now()) {
			return errs.Conflict("undangan sudah dipakai, dicabut, atau kedaluwarsa")
		}
		var accountEmail *string
		var accountStatus string
		var verified *time.Time
		if err := tx.QueryRow(ctx, `SELECT email,status,email_verified_at FROM users WHERE id=$1 FOR SHARE`, user).Scan(&accountEmail, &accountStatus, &verified); err != nil {
			return err
		}
		if verified == nil {
			return errs.PermissionDenied("verifikasi email sebelum menerima undangan")
		}
		if accountStatus != "active" || accountEmail == nil || identity.NormalizeEmail(*accountEmail) != identity.NormalizeEmail(email) {
			return errs.PermissionDenied("undangan bukan untuk akun ini")
		}
		var inviterRole, inviterStatus string
		err = tx.QueryRow(ctx, `SELECT m.role,m.status FROM memberships m JOIN users u ON u.id=m.user_id WHERE m.business_id=$1 AND m.user_id=$2 AND u.status='active'`, biz, inviter).Scan(&inviterRole, &inviterStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.PermissionDenied("pengundang sudah tidak berwenang")
		}
		if err != nil {
			return err
		}
		if inviterStatus != "active" || (inviterRole != "owner" && inviterRole != "manager") || !validMemberRole(role) || (inviterRole == "manager" && (role == "owner" || role == "manager")) {
			return errs.PermissionDenied("pengundang sudah tidak berwenang")
		}
		for _, b := range branches {
			var valid bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM branches WHERE business_id=$1 AND id=$2 AND status='active')`, biz, b).Scan(&valid); err != nil {
				return err
			}
			if !valid {
				return errs.Conflict("cabang undangan sudah tidak aktif")
			}
			if inviterRole == "manager" {
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM membership_branches b JOIN memberships m ON m.business_id=b.business_id AND m.id=b.membership_id WHERE b.business_id=$1 AND m.user_id=$2 AND b.branch_id=$3)`, biz, inviter, b).Scan(&valid); err != nil {
					return err
				}
				if !valid {
					return errs.PermissionDenied("cabang pengundang berubah")
				}
			}
		}
		membership = idkit.New()
		tag, err := tx.Exec(ctx, `INSERT INTO memberships(id,user_id,business_id,role) VALUES($1,$2,$3,$4) ON CONFLICT(user_id,business_id) DO NOTHING`, membership, user, biz, role)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return errs.Conflict("akun sudah memiliki keanggotaan; ubah lewat pengelolaan anggota")
		}
		for _, b := range branches {
			if _, err := tx.Exec(ctx, `INSERT INTO membership_branches(membership_id,business_id,branch_id) VALUES($1,$2,$3)`, membership, biz, b); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE invitations SET accepted_at=now() WHERE id=$1`, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: biz, ActorUserID: user, Action: "invitation.accepted", EntityType: "invitation", EntityID: id.String(), After: map[string]any{"membership_id": membership.String(), "role": role, "branch_ids": branches}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"membership_id": membership.String(), "business_id": biz.String()})
}
func (s *Server) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("invitation_id tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT id FROM businesses WHERE id=$1 FOR UPDATE`, tc.BusinessID); err != nil {
			return err
		}
		var role, status string
		if err := tx.QueryRow(ctx, `SELECT role,status FROM memberships WHERE business_id=$1 AND id=$2`, tc.BusinessID, tc.MembershipID).Scan(&role, &status); err != nil {
			return err
		}
		if status != "active" || (role != "owner" && role != "manager") {
			return errs.PermissionDenied("")
		}
		var inviter idkit.ID
		var accepted *time.Time
		err := tx.QueryRow(ctx, `SELECT invited_by,accepted_at FROM invitations WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&inviter, &accepted)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("undangan")
		}
		if err != nil {
			return err
		}
		if role == "manager" && inviter != tc.UserID {
			return errs.PermissionDenied("Manajer hanya dapat mencabut undangannya sendiri")
		}
		if accepted != nil {
			return errs.Conflict("undangan sudah diterima; kelola keanggotaan")
		}
		if _, err := tx.Exec(ctx, `UPDATE invitations SET revoked_at=COALESCE(revoked_at,now()) WHERE id=$1`, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "invitation.revoked", EntityType: "invitation", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

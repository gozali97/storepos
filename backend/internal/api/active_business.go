package apiserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/identity"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
)

func (s *Server) activeBusiness(w http.ResponseWriter, r *http.Request) {
	info := MustSession(r.Context())
	user, e1 := idkit.Parse(info.UserID)
	session, e2 := idkit.Parse(info.SessionID)
	if e1 != nil || e2 != nil {
		httpx.WriteError(w, r, errs.Unauthenticated(""))
		return
	}
	var req struct {
		Business string `json:"business_id"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	biz, err := idkit.Parse(req.Business)
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("business_id tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM memberships WHERE business_id=$1 AND user_id=$2 FOR SHARE`, biz, user).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.PermissionDenied("")
		}
		if err != nil {
			return err
		}
		if status != "active" {
			return errs.PermissionDenied("")
		}
		if err := identity.NewSessionRepo(tx).SetActiveBusiness(ctx, session, biz); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: biz, ActorUserID: user, Action: "session.business_selected", EntityType: "session", EntityID: session.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]string{"business_id": biz.String()})
}

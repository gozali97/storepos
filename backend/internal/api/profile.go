package apiserver

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
)

func (s *Server) profileRoutes(mux *http.ServeMux) {
	protect := func(h http.HandlerFunc) http.Handler { return CSRFMiddleware(s.secure)(RequireAuth(s)(h)) }
	mux.Handle("GET /api/v1/me/profile", protect(s.getProfile))
	mux.Handle("PUT /api/v1/me/profile", protect(s.putProfile))
}
func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	uid, err := idkit.Parse(MustSession(r.Context()).UserID)
	if err != nil {
		httpx.WriteError(w, r, errs.Unauthenticated(""))
		return
	}
	var name string
	var version int
	if err := s.pool.Underlying().QueryRow(r.Context(), `SELECT display_name,profile_version FROM users WHERE id=$1`, uid).Scan(&name, &version); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"display_name": name, "version": version})
}
func (s *Server) putProfile(w http.ResponseWriter, r *http.Request) {
	uid, err := idkit.Parse(MustSession(r.Context()).UserID)
	if err != nil {
		httpx.WriteError(w, r, errs.Unauthenticated(""))
		return
	}
	var req struct {
		Name string `json:"display_name"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || !utf8.ValidString(req.Name) || utf8.RuneCountInString(req.Name) > 200 {
		httpx.WriteError(w, r, errs.Validation("nama tampilan wajib, maksimal 200 karakter"))
		return
	}
	for _, c := range req.Name {
		if unicode.IsControl(c) {
			httpx.WriteError(w, r, errs.Validation("nama tampilan tidak boleh mengandung karakter kontrol"))
			return
		}
	}
	v, err := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if err != nil || v <= 0 {
		httpx.WriteError(w, r, errs.Validation("If-Match versi positif wajib"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET display_name=$3,profile_version=profile_version+1,updated_at=now() WHERE id=$1 AND profile_version=$2`, uid, v, req.Name)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return errs.New(errs.CodeOptimisticLockFailure, "profil berubah; muat ulang", http.StatusConflict)
		}
		// Never replicate the name into append-only audit history.
		return audit.Write(ctx, tx, audit.Entry{ActorUserID: uid, Action: "user.profile_changed", EntityType: "user", EntityID: uid.String(), After: map[string]int{"version": v + 1}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"display_name": req.Name, "version": v + 1})
}

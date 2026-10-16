package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/identity"
	"github.com/storepos/storepos/internal/platform/auth"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
)

func (s *Server) passwordRoutes(mux *http.ServeMux) {
	mux.Handle("PUT /api/v1/auth/password", CSRFMiddleware(s.secure)(RequireAuth(s)(http.HandlerFunc(s.changePassword))))
}

// changePassword mengganti kata sandi akun sendiri (NFR-011). Kata sandi lama wajib benar,
// yang baru lolos kebijakan, dan semua sesi lain dicabut; sesi saat ini tetap aktif.
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	info := MustSession(r.Context())
	uid, e1 := idkit.Parse(info.UserID)
	sid, e2 := idkit.Parse(info.SessionID)
	if e1 != nil || e2 != nil {
		httpx.WriteError(w, r, errs.Unauthenticated(""))
		return
	}
	var req struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Current == "" {
		httpx.WriteError(w, r, errs.Validation("kata sandi saat ini wajib diisi"))
		return
	}
	if err := auth.VerifyPolicy(req.New); err != nil {
		httpx.WriteError(w, r, errs.Validation(err.Error()))
		return
	}
	if req.New == req.Current {
		httpx.WriteError(w, r, errs.Validation("kata sandi baru harus berbeda dari yang lama"))
		return
	}
	// Batas percobaan per akun sama dengan login agar kata sandi lama tidak bisa ditebak lewat endpoint ini.
	key := "pwd:" + uid.String()
	if ok, retry := s.acctLimit.Allow(key); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		httpx.WriteError(w, r, errs.New(errs.CodeRateLimited, "terlalu banyak percobaan, coba lagi nanti", http.StatusTooManyRequests))
		return
	}
	user, err := s.users.ByID(r.Context(), uid)
	if err != nil {
		httpx.WriteError(w, r, errs.Internal("gagal membaca akun").Wrap(err))
		return
	}
	if ok, err := auth.Verify(req.Current, user.PasswordHash); err != nil || !ok {
		httpx.WriteError(w, r, errs.New(errs.CodeValidation, "kata sandi saat ini salah", http.StatusUnprocessableEntity))
		return
	}
	hash, err := auth.Hash(req.New)
	if err != nil {
		httpx.WriteError(w, r, errs.Internal("gagal memproses kata sandi").Wrap(err))
		return
	}
	var revoked int64
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		// Verifikasi mahal dilakukan di luar transaksi; lock ini mencegah hash lama menimpa perubahan bersamaan.
		var currentHash, status string
		if err := tx.QueryRow(ctx, `SELECT password_hash,status FROM users WHERE id=$1 FOR UPDATE`, uid).Scan(&currentHash, &status); err != nil {
			return err
		}
		if status != "active" {
			return errs.Unauthenticated("akun tidak aktif")
		}
		if currentHash != user.PasswordHash {
			return errs.Conflict("kata sandi berubah di sesi lain; muat ulang dan verifikasi kembali")
		}
		if err := identity.NewUserRepo(tx).UpdatePasswordHash(ctx, uid, hash); err != nil {
			return err
		}
		n, err := identity.NewSessionRepo(tx).RevokeAllForUserExcept(ctx, uid, sid)
		if err != nil {
			return err
		}
		revoked = n
		// Audit tidak memuat kata sandi, hash, maupun email.
		return audit.Write(ctx, tx, audit.Entry{ActorUserID: uid, Action: "auth.password_changed", EntityType: "user", EntityID: uid.String(), After: map[string]int64{"other_sessions_revoked": n}})
	})
	if err != nil {
		if _, ok := errs.As(err); ok {
			httpx.WriteError(w, r, err)
			return
		}
		if errors.Is(err, identity.ErrUserNotFound) {
			httpx.WriteError(w, r, errs.Unauthenticated(""))
			return
		}
		httpx.WriteError(w, r, errs.Internal("gagal menyimpan kata sandi").Wrap(err))
		return
	}
	s.acctLimit.Reset(key)
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok", "other_sessions_revoked": revoked})
}

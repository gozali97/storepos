// Package apiserver merangkai HTTP API v1: rute, middleware, dan handler.
//
// Urutan middleware mengikuti PRD §16.5:
//
//	RequestID → Recover → AccessLog → CORS(dev) → CSRF → Auth → Tenant → Module → Izin
//
// Setiap lapisan hanya bertanggung jawab atas satu hal, sehingga mudah diuji
// secara terpisah dan alasan penolakan selalu dapat dijelaskan ke pengguna.
package apiserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/storepos/storepos/internal/audit"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/tenant"
)

// Nama cookie. Cookie sesi HttpOnly; cookie CSRF harus dapat dibaca JavaScript
// (pola double-submit, PRD §16.5).
const (
	CookieSession = "storepos_session"
	CookieCSRF    = "storepos_csrf"

	HeaderCSRF       = "X-CSRF-Token"
	HeaderBusinessID = "X-Business-ID"
)

// ctxKey adalah tipe kunci context privat paket ini.
type ctxKey int

const (
	ctxSession ctxKey = iota
)

// SessionInfo adalah identitas yang sudah diverifikasi dari cookie sesi.
type SessionInfo struct {
	SessionID string
	UserID    string
	CSRFToken string
	UserAgent string
	IP        string
	ActiveBiz string
}

// WithSession menyimpan SessionInfo ke context.
func WithSession(ctx context.Context, s SessionInfo) context.Context {
	return context.WithValue(ctx, ctxSession, s)
}

// SessionFrom mengambil SessionInfo dari context.
func SessionFrom(ctx context.Context) (SessionInfo, bool) {
	v, ok := ctx.Value(ctxSession).(SessionInfo)
	return v, ok
}

// MustSession mengembalikan SessionInfo atau panic.
// Hanya dipakai pada rute yang pasti dilindungi middleware autentikasi.
func MustSession(ctx context.Context) SessionInfo {
	s, ok := SessionFrom(ctx)
	if !ok {
		panic("apiserver: session tidak ada di context; rute tidak dilindungi middleware")
	}
	return s
}

// SessionVerifier adalah kontrak minimal yang dibutuhkan middleware autentikasi.
// Implementasi nyata ada di modul identity; interface ini menjaga paket HTTP
// tidak bergantung langsung ke penyimpanan.
type SessionVerifier interface {
	// Verify mengembalikan identitas sesi untuk token mentah.
	Verify(ctx context.Context, rawToken string) (SessionInfo, error)
	// Touch memperbarui jejak terakhir sesi (boleh no-op).
	Touch(ctx context.Context, sessionID string)
}

// RequireAuth menolak request tanpa sesi valid.
func RequireAuth(v SessionVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(CookieSession)
			if err != nil || c.Value == "" {
				httpx.WriteError(w, r, errs.Unauthenticated("sesi tidak ditemukan"))
				return
			}

			info, err := v.Verify(r.Context(), c.Value)
			if err != nil {
				// Detail kegagalan tidak dibocorkan ke klien; cukup 401.
				httpx.Logger(r.Context()).Debug("sesi ditolak", "err", err)
				clearSessionCookies(w)
				httpx.WriteError(w, r, errs.Unauthenticated("sesi tidak valid atau sudah berakhir"))
				return
			}
			v.Touch(r.Context(), info.SessionID)

			ctx := WithSession(audit.WithRequest(r.Context(), r.RemoteAddr, r.UserAgent()), info)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CSRFMiddleware menerapkan pola double-submit cookie untuk metode yang mengubah
// data (PRD §16.5). Harus dipasang SEBELUM RequireAuth agar tidak ada kerja
// database untuk request yang tidak punya token CSRF.
func CSRFMiddleware(cookieSecure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// Endpoint login tidak punya sesi, jadi tidak dapat memakai pola ini.
			// Lihat auth.go: login memakai pemeriksaan Origin/Referer.
			cookie, err := r.Cookie(CookieCSRF)
			if err != nil || cookie.Value == "" {
				httpx.WriteError(w, r, errs.New(errs.CodePermissionDenied,
					"token CSRF tidak ditemukan", http.StatusForbidden))
				return
			}
			header := r.Header.Get(HeaderCSRF)
			if header == "" || !constantTimeEqual(header, cookie.Value) {
				httpx.WriteError(w, r, errs.New(errs.CodePermissionDenied,
					"token CSRF tidak cocok", http.StatusForbidden))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// TenantResolver menyusun tenant.Context dari sesi + header X-Business-ID.
type TenantResolver interface {
	// Resolve mengembalikan konteks tenant untuk (user, bisnis) atau error bertipe AppError.
	Resolve(ctx context.Context, userID, businessID string) (tenant.Context, error)
}

// HeaderBusinessID adalah nama header yang dipakai frontend untuk memilih bisnis aktif.
const HeaderBusinessIDName = HeaderBusinessID

// RequireTenant memasang tenant.Context. Sumber businessID: header X-Business-ID,
// lalu sesi (active_business_id) sebagai fallback, sehingga pengguna dengan satu
// bisnis tidak perlu mengirim header.
func RequireTenant(res TenantResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, ok := SessionFrom(r.Context())
			if !ok {
				httpx.WriteError(w, r, errs.Unauthenticated(""))
				return
			}

			businessID := strings.TrimSpace(r.Header.Get(HeaderBusinessID))
			if businessID == "" {
				businessID = sess.ActiveBiz
			}
			if businessID == "" {
				httpx.WriteError(w, r, errs.Validation("pilih bisnis aktif terlebih dahulu",
					errs.FieldError{Field: HeaderBusinessID, Code: "required",
						Message: "header X-Business-ID atau bisnis aktif pada sesi wajib diisi"}))
				return
			}

			tc, err := res.Resolve(r.Context(), sess.UserID, businessID)
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}

			next.ServeHTTP(w, r.WithContext(tenant.WithContext(r.Context(), tc)))
		})
	}
}

// RequireModule menolak request bila modul tidak aktif untuk bisnis tersebut
// (PRD CORE-002 → 403 MODULE_DISABLED).
func RequireModule(modules ...string) func(http.Handler) http.Handler {
	want := make(map[string]bool, len(modules))
	for _, m := range modules {
		want[m] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tc, err := tenant.FromContext(r.Context())
			if err != nil {
				httpx.WriteError(w, r, errs.Internal("konteks tenant tidak tersedia").Wrap(err))
				return
			}
			if !enabledFor(want, tc) {
				httpx.WriteError(w, r, errs.ModuleDisabled(strings.Join(modules, ",")))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission menolak request bila peran tidak punya izin atas tindakan.
func RequirePermission(check func(tenant.Context) error) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tc, err := tenant.FromContext(r.Context())
			if err != nil {
				httpx.WriteError(w, r, errs.Internal("konteks tenant tidak tersedia").Wrap(err))
				return
			}
			if err := check(tc); err != nil {
				httpx.WriteError(w, r, errs.PermissionDenied(""))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ---------------------------------------------------------------------------
// Utilitas
// ---------------------------------------------------------------------------

func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// constantTimeEqual membandingkan dua string tanpa membocorkan waktu.
func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// ClientIP mengambil alamat IP klien dari koneksi langsung.
//
// Header X-Forwarded-For SENGAJA tidak dipercaya: nilai itu dapat dipalsukan
// klien dan dipakai untuk menghindari rate limit. Bila aplikasi berada di
// belakang proxy tepercaya, proxy harus menetapkan header khusus dan alamat
// proxy harus dikonfigurasi eksplisit di sini.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func clearSessionCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieSession,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// enabledFor melaporkan apakah salah satu modul yang diminta aktif.
// Daftar modul aktif dibawa di tenant.Context oleh resolver.
func enabledFor(want map[string]bool, tc tenant.Context) bool {
	for _, m := range tc.EnabledModules {
		if want[m] {
			return true
		}
	}
	return false
}

// ErrNoSession dikembalikan verifier ketika token tidak cocok.
var ErrNoSession = errors.New("sesi tidak valid")

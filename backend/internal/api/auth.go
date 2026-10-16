package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/identity"
	"github.com/storepos/storepos/internal/org"
	"github.com/storepos/storepos/internal/platform/auth"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/ratelimit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

// Server menampung dependensi handler API v1 dan memenuhi SessionVerifier
// serta TenantResolver untuk middleware.
type Server struct {
	users    *identity.UserRepo
	sessions *identity.SessionRepo
	org      *org.Repo
	pool     *database.Pool

	ttl    time.Duration
	secure bool

	// NFR-011: 10 percobaan/menit per IP dan per akun.
	ipLimit   *ratelimit.Limiter
	acctLimit *ratelimit.Limiter

	// Hash palsu untuk menyamakan waktu respons saat akun tidak ada.
	dummyHash string
}

// NewServer membuat Server. ttl = umur absolut sesi; secure = flag Secure pada cookie.
func NewServer(pool *database.Pool, ttl time.Duration, secure bool) (*Server, error) {
	dummy, err := auth.Hash("kata-sandi-palsu-untuk-pengatur-waktu")
	if err != nil {
		return nil, err
	}
	db := pool.Underlying()
	return &Server{
		users:     identity.NewUserRepo(db),
		sessions:  identity.NewSessionRepo(db),
		org:       org.NewRepo(db),
		pool:      pool,
		ttl:       ttl,
		secure:    secure,
		ipLimit:   ratelimit.New(10, time.Minute),
		acctLimit: ratelimit.New(10, time.Minute),
		dummyHash: dummy,
	}, nil
}

// Routes mendaftarkan rute autentikasi dan profil.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/leads", s.createLead)
	// shortcut: login tanpa token CSRF (belum ada sesi); mengandalkan cookie
	// SameSite=Lax + body JSON. Tambah pemeriksaan Origin bila ada klien lintas situs.
	mux.HandleFunc("POST /api/v1/auth/login", s.login)

	authed := func(h http.HandlerFunc) http.Handler {
		return CSRFMiddleware(s.secure)(RequireAuth(s)(h))
	}
	mux.Handle("POST /api/v1/auth/logout", authed(s.logout))
	mux.Handle("GET /api/v1/me", authed(s.me))
	mux.Handle("PUT /api/v1/session/business", authed(s.activeBusiness))
	mux.Handle("GET /api/v1/workspace", authed(func(w http.ResponseWriter, r *http.Request) {
		RequireTenant(s)(http.HandlerFunc(s.workspace)).ServeHTTP(w, r)
	}))
	s.catalogRoutes(mux)
	s.salesRoutes(mux)
	s.shiftRoutes(mux)
	s.taxRoutes(mux)
	s.draftRoutes(mux)
	s.stockRoutes(mux)
	s.purchasingRoutes(mux)
	s.supplierRoutes(mux)
	s.countRoutes(mux)
	s.reconciliationRoutes(mux)
	s.minimumRoutes(mux)
	s.combinationRoutes(mux)
	s.unitRoutes(mux)
	s.customerRoutes(mux)
	s.expenseRoutes(mux)
	s.reportRoutes(mux)
	s.exportRoutes(mux)
	s.auditRoutes(mux)
	s.membershipRoutes(mux)
	s.invitationRoutes(mux)
	s.organizationRoutes(mux)
	s.policyRoutes(mux)
	s.saleReadRoutes(mux)
	s.receiptRoutes(mux)
	s.profileRoutes(mux)
	s.restaurantRoutes(mux)
	s.orderRoutes(mux)
	s.orderLineRoutes(mux)
	s.discountOrderRoutes(mux)
	s.modifierRoutes(mux)
	s.stationRoutes(mux)
	s.ticketRoutes(mux)
	s.cancellationReportRoutes(mux)
	s.operationRoutes(mux)
	s.agingRoutes(mux)
	s.availabilityRoutes(mux)
	s.eventRoutes(mux)
	s.contactPrivacyRoutes(mux)
	s.businessTypeRoutes(mux)
	s.adminDetailRoutes(mux)
	s.invitationListRoutes(mux)
	s.batch2Routes(mux)
	s.scopedTaxRoutes(mux)
	s.batch4Routes(mux)
	s.catalogManagementRoutes(mux)
	s.importRoutes(mux)
	s.pharmacyRoutes(mux)
	s.pharmacyCountRoutes(mux)
	s.passwordRoutes(mux)
	s.onboardingRoutes(mux)
	s.paymentMethodRoutes(mux)
	s.prescriptionRoutes(mux)
}

// ---------------------------------------------------------------------------
// SessionVerifier & TenantResolver
// ---------------------------------------------------------------------------

// Verify memeriksa token sesi dan status akun pada setiap request, sehingga
// penonaktifan pengguna berlaku seketika (CORE-004).
func (s *Server) Verify(ctx context.Context, raw string) (SessionInfo, error) {
	sess, err := s.sessions.ByToken(ctx, raw)
	if err != nil {
		return SessionInfo{}, err
	}
	u, err := s.users.ByID(ctx, sess.UserID)
	if err != nil {
		return SessionInfo{}, err
	}
	if u.Status != identity.UserActive {
		return SessionInfo{}, identity.ErrUserDisabled
	}
	// Tulis last_seen paling sering sekali per menit.
	if time.Since(sess.LastSeenAt) > time.Minute {
		_ = s.sessions.Touch(ctx, sess.ID)
	}
	info := SessionInfo{
		SessionID: sess.ID.String(),
		UserID:    sess.UserID.String(),
		UserAgent: sess.UserAgent,
		IP:        sess.IP,
	}
	if sess.ActiveBusinessID != nil {
		info.ActiveBiz = sess.ActiveBusinessID.String()
	}
	return info, nil
}

// Touch tidak melakukan apa pun; pembaruan last_seen ada di Verify.
func (s *Server) Touch(context.Context, string) {}

// Resolve membangun tenant.Context. Non-anggota dan anggota nonaktif sama-sama
// mendapat 403 agar keberadaan bisnis lain tidak bisa ditebak.
func (s *Server) Resolve(ctx context.Context, userID, businessID string) (tenant.Context, error) {
	uid, err := idkit.Parse(userID)
	if err != nil {
		return tenant.Context{}, errs.Unauthenticated("")
	}
	bid, err := idkit.Parse(businessID)
	if err != nil {
		return tenant.Context{}, errs.Validation("X-Business-ID tidak valid")
	}

	m, err := s.org.MembershipFor(ctx, uid, bid)
	if errors.Is(err, org.ErrNotMember) {
		return tenant.Context{}, errs.PermissionDenied("")
	}
	if err != nil {
		return tenant.Context{}, errs.Internal("gagal membaca keanggotaan").Wrap(err)
	}
	if m.Status != "active" {
		return tenant.Context{}, errs.PermissionDenied("")
	}

	b, err := s.org.BusinessByID(ctx, bid)
	if err != nil {
		return tenant.Context{}, errs.Internal("gagal membaca bisnis").Wrap(err)
	}
	branches, err := s.org.BranchIDsForMembership(ctx, m.ID)
	if err != nil {
		return tenant.Context{}, errs.Internal("gagal membaca cabang").Wrap(err)
	}

	return tenant.Context{
		UserID:         uid,
		BusinessID:     bid,
		MembershipID:   m.ID,
		Role:           m.Role,
		BranchIDs:      branches,
		EnabledModules: b.EnabledModules,
	}, nil
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userDTO struct {
	ID    string `json:"id"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

type membershipDTO struct {
	BusinessID   string `json:"business_id"`
	BusinessName string `json:"business_name"`
	BusinessType string `json:"business_type"`
	Role         string `json:"role"`
}

type sessionResponse struct {
	User        userDTO         `json:"user"`
	Memberships []membershipDTO `json:"memberships"`
}

var errBadLogin = errs.Unauthenticated("email atau kata sandi salah")

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(audit.WithRequest(r.Context(), r.RemoteAddr, r.UserAgent()))
	failed := func(actor idkit.ID, reason string) bool {
		err := audit.Write(r.Context(), s.pool.Underlying(), audit.Entry{ActorUserID: actor, Action: "auth.login_failed", EntityType: "authentication", Reason: reason})
		if err != nil {
			httpx.WriteError(w, r, errs.Internal("gagal mencatat audit autentikasi").Wrap(err))
			return false
		}
		return true
	}
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	email := identity.NormalizeEmail(req.Email)
	if email == "" || req.Password == "" {
		httpx.WriteError(w, r, errs.Validation("email dan kata sandi wajib diisi"))
		return
	}

	ip := ClientIP(r)
	acctKey := "acct:" + email
	for _, key := range []string{"ip:" + ip, acctKey} {
		lim := s.ipLimit
		if key == acctKey {
			lim = s.acctLimit
		}
		if ok, retry := lim.Allow(key); !ok {
			if !failed(idkit.ID{}, "rate_limited") {
				return
			}
			w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			httpx.WriteError(w, r, errs.New(errs.CodeRateLimited,
				"terlalu banyak percobaan, coba lagi nanti", http.StatusTooManyRequests))
			return
		}
	}

	u, err := s.users.ByEmail(r.Context(), email)
	switch {
	case errors.Is(err, identity.ErrUserNotFound):
		// Hitung hash palsu agar waktu respons tidak membedakan akun ada/tidak.
		_, _ = auth.Verify(req.Password, s.dummyHash)
		if !failed(idkit.ID{}, "invalid_credentials") {
			return
		}
		httpx.WriteError(w, r, errBadLogin)
		return
	case err != nil:
		httpx.WriteError(w, r, errs.Internal("gagal membaca akun").Wrap(err))
		return
	}

	ok, err := auth.Verify(req.Password, u.PasswordHash)
	if err != nil || !ok || u.Status != identity.UserActive {
		if !failed(u.ID, "invalid_credentials") {
			return
		}
		httpx.WriteError(w, r, errBadLogin)
		return
	}
	s.acctLimit.Reset(acctKey)

	csrf, err := auth.NewCSRFToken()
	if err != nil {
		httpx.WriteError(w, r, errs.Internal("gagal membuat token CSRF").Wrap(err))
		return
	}
	var token string
	var sess *identity.Session
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var err error
		token, sess, err = identity.NewSessionRepo(tx).Create(ctx, u.ID, s.ttl, r.UserAgent(), ip)
		if err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{ActorUserID: u.ID, Action: "auth.login", EntityType: "session", EntityID: sess.ID.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, errs.Internal("gagal membuat sesi/audit").Wrap(err))
		return
	}
	s.setCookies(w, token, csrf, sess.ExpiresAt)

	s.writeSession(w, r, u, http.StatusOK)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	info := MustSession(r.Context())
	if id, err := idkit.Parse(info.SessionID); err == nil {
		uid, err := idkit.Parse(info.UserID)
		if err != nil {
			httpx.WriteError(w, r, errs.Unauthenticated(""))
			return
		}
		if err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
			if err := identity.NewSessionRepo(tx).Revoke(ctx, id); err != nil {
				return err
			}
			return audit.Write(ctx, tx, audit.Entry{ActorUserID: uid, Action: "auth.logout", EntityType: "session", EntityID: id.String()})
		}); err != nil {
			httpx.WriteError(w, r, errs.Internal("gagal mencabut sesi").Wrap(err))
			return
		}
	}
	clearSessionCookies(w)
	http.SetCookie(w, &http.Cookie{Name: CookieCSRF, Value: "", Path: "/", MaxAge: -1, Secure: s.secure})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	info := MustSession(r.Context())
	uid, err := idkit.Parse(info.UserID)
	if err != nil {
		httpx.WriteError(w, r, errs.Unauthenticated(""))
		return
	}
	u, err := s.users.ByID(r.Context(), uid)
	if err != nil {
		httpx.WriteError(w, r, errs.Internal("gagal membaca akun").Wrap(err))
		return
	}
	s.writeSession(w, r, u, http.StatusOK)
}

func (s *Server) writeSession(w http.ResponseWriter, r *http.Request, u *identity.User, status int) {
	ms, err := s.org.MembershipsForUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, errs.Internal("gagal membaca keanggotaan").Wrap(err))
		return
	}
	resp := sessionResponse{
		User:        userDTO{ID: u.ID.String(), Email: u.Email, Phone: u.Phone},
		Memberships: make([]membershipDTO, 0, len(ms)),
	}
	for _, m := range ms {
		resp.Memberships = append(resp.Memberships, membershipDTO{
			BusinessID:   m.BusinessID.String(),
			BusinessName: m.BusinessName,
			BusinessType: string(m.BusinessType),
			Role:         string(m.Role),
		})
	}
	httpx.JSON(w, status, resp)
}

func (s *Server) setCookies(w http.ResponseWriter, session, csrf string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieSession, Value: session, Path: "/", Expires: expires,
		HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode,
	})
	// Cookie CSRF sengaja tidak HttpOnly: frontend membacanya dan mengirim ulang di header.
	http.SetCookie(w, &http.Cookie{
		Name: CookieCSRF, Value: csrf, Path: "/", Expires: expires,
		Secure: s.secure, SameSite: http.SameSiteLaxMode,
	})
}

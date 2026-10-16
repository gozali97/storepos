package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/storepos/storepos/internal/org"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/sales"
)

func (s *Server) shiftRoutes(mux *http.ServeMux) {
	s.shiftHistoryRoutes(mux)
	mux.Handle("GET /api/v1/shifts/open", s.guard(authz.PermShiftManageOwn, false, s.getOpenShift))
	mux.Handle("POST /api/v1/shifts/cash", s.guard(authz.PermCashInOut, true, s.cashMove))
	mux.Handle("POST /api/v1/shifts/close", s.guard(authz.PermShiftManageOwn, true, s.closeShift))
	mux.Handle("GET /api/v1/shifts/{id}/summary", s.guard(authz.PermShiftManageOwn, false, s.shiftSummary))
	// PIN atasan hanya untuk Pemilik/Manajer (CORE-006).
	mux.Handle("PUT /api/v1/memberships/me/pin", CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(
		RequirePermission(func(tc tenant.Context) error {
			if tc.Role != tenant.RoleOwner && tc.Role != tenant.RoleManager {
				return errors.New("hanya Pemilik/Manajer yang memiliki PIN atasan")
			}
			return nil
		})(http.HandlerFunc(s.setPIN))))))
}

func (s *Server) getOpenShift(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	query := r.URL.Query()
	branch, ok := branchOf(w, r, tc, query.Get("branch_id"))
	if !ok {
		return
	}
	register, err := idkit.Parse(query.Get("register_id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("register_id tidak valid"))
		return
	}
	var shiftID, openedBy idkit.ID
	var opening string
	var openedAt time.Time
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT id,opened_by,opening_cash_cents::text,opened_at
		FROM shifts WHERE business_id=$1 AND branch_id=$2 AND register_id=$3 AND status='open'`, tc.BusinessID, branch, register).Scan(&shiftID, &openedBy, &opening, &openedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, http.StatusOK, map[string]any{"shift": nil})
		return
	}
	if err != nil {
		httpx.WriteError(w, r, errs.Internal("gagal membaca shift terminal").Wrap(err))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"shift": map[string]any{
		"shift_id": shiftID.String(), "opening_cash_cents": opening, "opened_at": openedAt,
		"opened_by_current_user": openedBy == tc.UserID,
	}})
}

type approvalReq struct {
	Email string `json:"email"`
	PIN   string `json:"pin"`
}

// approver memverifikasi PIN atasan di luar transaksi bisnis, agar penghitung gagal tetap tersimpan.
func (s *Server) approver(ctx context.Context, tc tenant.Context, a *approvalReq) (*idkit.ID, error) {
	if a == nil {
		return nil, nil
	}
	return s.approverRoles(ctx, tc, a, "owner", "manager")
}

// pharmacistApprover memverifikasi apoteker lewat email + PIN (APT-009).
func (s *Server) pharmacistApprover(ctx context.Context, tc tenant.Context, a *approvalReq) (idkit.ID, error) {
	id, err := s.approverRoles(ctx, tc, a, "pharmacist")
	if err != nil {
		return idkit.ID{}, err
	}
	return *id, nil
}

func (s *Server) approverRoles(ctx context.Context, tc tenant.Context, a *approvalReq, roles ...string) (*idkit.ID, error) {
	var id idkit.ID
	var verificationErr error
	// Transaksi terpisah mengunci penghitung PIN; penolakan tetap di-commit agar lockout tidak hilang.
	err := s.pool.InTx(ctx, func(ctx context.Context, tx database.Tx) error {
		id, verificationErr = org.NewRepo(tx).VerifyRoles(ctx, tc.BusinessID, tc.UserID, a.Email, a.PIN, roles...)
		if errors.Is(verificationErr, org.ErrApprovalDenied) || errors.Is(verificationErr, org.ErrPINLocked) {
			return nil
		}
		return verificationErr
	})
	if err != nil {
		return nil, err
	}
	if verificationErr != nil {
		return nil, verificationErr
	}
	return &id, nil
}

func mapShiftError(err error) error {
	switch {
	case errors.Is(err, org.ErrPINLocked):
		return errs.New(errs.CodeTooManyRequests, err.Error(), http.StatusTooManyRequests)
	case errors.Is(err, org.ErrApprovalDenied), errors.Is(err, sales.ErrApprovalRequired):
		return errs.New(errs.CodeApprovalRequired, err.Error(), http.StatusForbidden)
	case errors.Is(err, sales.ErrShiftNotOpen):
		return errs.New(errs.CodeShiftNotOpen, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, sales.ErrShiftStillOpen):
		return errs.Conflict(err.Error())
	case errors.Is(err, sales.ErrNotFound):
		return errs.NotFound("shift")
	case errors.Is(err, sales.ErrNotShiftOwner):
		return errs.PermissionDenied(err.Error())
	case errors.Is(err, sales.ErrReasonRequired), errors.Is(err, sales.ErrInvalidAmount):
		return errs.Validation(err.Error())
	}
	return errs.Internal("operasi shift gagal").Wrap(err)
}

func (s *Server) cashMove(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		B           string       `json:"branch_id"`
		R           string       `json:"register_id"`
		Kind        string       `json:"kind"`
		AmountCents string       `json:"amount_cents"`
		Reason      string       `json:"reason"`
		Approval    *approvalReq `json:"approval"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.B)
	if !ok {
		return
	}
	reg, e1 := idkit.Parse(req.R)
	amount, e2 := money.ParseCents(req.AmountCents)
	if e1 != nil || e2 != nil {
		httpx.WriteError(w, r, errs.Validation("register_id atau amount_cents tidak valid"))
		return
	}
	appr, err := s.approver(r.Context(), tc, req.Approval)
	if err != nil {
		httpx.WriteError(w, r, mapShiftError(err))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		return sales.CashMove(ctx, tx, tc.BusinessID, branch, reg, tc.UserID, req.Kind, int64(amount), req.Reason, appr)
	})
	if err != nil {
		httpx.WriteError(w, r, mapShiftError(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (s *Server) closeShift(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		B        string       `json:"branch_id"`
		R        string       `json:"register_id"`
		Counted  string       `json:"counted_cash_cents"`
		Note     string       `json:"note"`
		Force    bool         `json:"force"`
		Approval *approvalReq `json:"approval"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.B)
	if !ok {
		return
	}
	reg, e1 := idkit.Parse(req.R)
	counted, e2 := money.ParseCents(req.Counted)
	if e1 != nil || e2 != nil {
		httpx.WriteError(w, r, errs.Validation("register_id atau counted_cash_cents tidak valid"))
		return
	}
	// Tutup paksa shift orang lain hanya untuk peran yang boleh melihat semua shift (E7).
	if req.Force && authz.Check(tc.Role, authz.PermShiftViewAll) != authz.Allowed {
		httpx.WriteError(w, r, errs.PermissionDenied(""))
		return
	}
	appr, err := s.approver(r.Context(), tc, req.Approval)
	if err != nil {
		httpx.WriteError(w, r, mapShiftError(err))
		return
	}
	var res *sales.CloseResult
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var e error
		res, e = sales.CloseShift(ctx, tx, tc.BusinessID, branch, reg, tc.UserID, req.Force, int64(counted), req.Note, appr)
		return e
	})
	if err != nil {
		httpx.WriteError(w, r, mapShiftError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{
		"shift_id":            res.ShiftID.String(),
		"expected_cash_cents": strconv.FormatInt(res.ExpectedCents, 10),
		"counted_cash_cents":  strconv.FormatInt(res.CountedCents, 10),
		"variance_cents":      strconv.FormatInt(res.VarianceCents, 10),
	})
}

func (s *Server) setPIN(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		PIN string `json:"pin"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := s.org.SetPIN(r.Context(), tc.MembershipID, req.PIN); err != nil {
		httpx.WriteError(w, r, errs.Validation("PIN harus 6 digit angka"))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

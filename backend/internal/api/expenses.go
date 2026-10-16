package apiserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/idempotency"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/sales"
)

func (s *Server) expenseRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/expenses", s.guard(authz.PermExpenseManage, true, s.createExpense))
	mux.Handle("GET /api/v1/expenses", s.guard(authz.PermExpenseManage, false, s.listExpenses))
}
func (s *Server) createExpense(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		BranchID   string       `json:"branch_id"`
		Date       string       `json:"expense_date"`
		Category   string       `json:"category"`
		Amount     string       `json:"amount_cents"`
		Method     string       `json:"method"`
		Note       string       `json:"note"`
		RegisterID string       `json:"register_id"`
		Approval   *approvalReq `json:"approval"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.BranchID)
	if !ok {
		return
	}
	amount, err := money.ParseCents(req.Amount)
	_, dateErr := time.Parse("2006-01-02", req.Date)
	if err != nil || amount <= 0 || dateErr != nil || strings.TrimSpace(req.Category) == "" || utf8.RuneCountInString(req.Category) > 100 || utf8.RuneCountInString(req.Note) > 200 || (req.Method != "shift_cash" && req.Method != "outside_cash") {
		httpx.WriteError(w, r, errs.Validation("tanggal/kategori/nominal/metode/catatan pengeluaran tidak valid"))
		return
	}
	var reg idkit.ID
	if req.Method == "shift_cash" {
		reg, err = idkit.Parse(req.RegisterID)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("register_id wajib untuk kas shift"))
			return
		}
	}
	if tc.Role == tenant.RoleCashier && req.Method != "shift_cash" {
		httpx.WriteError(w, r, errs.PermissionDenied("kasir hanya boleh mencatat pengeluaran melalui kas shift"))
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key wajib diisi, maksimal 128 karakter"))
		return
	}
	approved, err := s.approver(r.Context(), tc, req.Approval)
	if err != nil {
		httpx.WriteError(w, r, mapShiftError(err))
		return
	}
	if authz.Check(tc.Role, authz.PermExpenseManage) == authz.AllowedWithO && approved == nil {
		httpx.WriteError(w, r, mapShiftError(sales.ErrApprovalRequired))
		return
	}
	raw, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hash := sha256.Sum256(raw)
	id := idkit.New()
	var replay *idempotency.Stored
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		st, e := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /api/v1/expenses", hash[:])
		replay = st
		if e != nil || st != nil {
			return e
		}
		var active bool
		e = tx.QueryRow(ctx, `SELECT status='active' FROM branches WHERE business_id=$1 AND id=$2`, tc.BusinessID, branch).Scan(&active)
		if errors.Is(e, pgx.ErrNoRows) {
			return errs.NotFound("cabang")
		}
		if e != nil {
			return e
		}
		if !active {
			return errs.Conflict("cabang nonaktif")
		}
		var shift *idkit.ID
		if req.Method == "shift_cash" {
			var sid idkit.ID
			e = tx.QueryRow(ctx, `SELECT id FROM shifts WHERE business_id=$1 AND branch_id=$2 AND register_id=$3 AND status='open' FOR SHARE`, tc.BusinessID, branch, reg).Scan(&sid)
			if errors.Is(e, pgx.ErrNoRows) {
				return sales.ErrShiftNotOpen
			}
			if e != nil {
				return e
			}
			shift = &sid
			if e := sales.CashMove(ctx, tx, tc.BusinessID, branch, reg, tc.UserID, "out", int64(amount), strings.TrimSpace(req.Category)+": "+req.Note, approved); e != nil {
				return e
			}
		}
		if _, e := tx.Exec(ctx, `INSERT INTO expenses(id,business_id,branch_id,expense_date,category,amount_cents,method,note,shift_id,created_by,approved_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, id, tc.BusinessID, branch, req.Date, strings.TrimSpace(req.Category), int64(amount), req.Method, req.Note, shift, tc.UserID, approved); e != nil {
			return e
		}
		if e := audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, ApprovedBy: approved, Action: "expense.created", EntityType: "expense", EntityID: id.String(), After: map[string]string{"method": req.Method, "amount_cents": req.Amount}}); e != nil {
			return e
		}
		body, e := json.Marshal(map[string]string{"expense_id": id.String()})
		if e != nil {
			return e
		}
		return idempotency.Complete(ctx, tx, tc.BusinessID, key, http.StatusCreated, body)
	})
	if err != nil {
		if _, ok := errs.As(err); ok {
			httpx.WriteError(w, r, err)
		} else if errors.Is(err, idempotency.ErrConflict) {
			httpx.WriteError(w, r, mapSalesError(err))
		} else {
			httpx.WriteError(w, r, mapShiftError(err))
		}
		return
	}
	if replay != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Idempotent-Replay", "true")
		w.WriteHeader(replay.Status)
		_, _ = w.Write(replay.Body)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"expense_id": id.String()})
}
func (s *Server) listExpenses(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return
	}
	from, e1 := time.Parse("2006-01-02", q.Get("from"))
	to, e2 := time.Parse("2006-01-02", q.Get("to"))
	if e1 != nil || e2 != nil || from.After(to) {
		httpx.WriteError(w, r, errs.Validation("periode from/to wajib valid YYYY-MM-DD"))
		return
	}
	var cursor idkit.ID
	if q.Get("after") != "" {
		var err error
		cursor, err = idkit.Parse(q.Get("after"))
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return
		}
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,expense_date::text,category,amount_cents::text,method,note,shift_id::text FROM expenses WHERE business_id=$1 AND branch_id=$2 AND expense_date BETWEEN $3 AND $4 AND id>$5 ORDER BY id LIMIT 101`, tc.BusinessID, branch, from, to, cursor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type expense struct {
		ID       string  `json:"expense_id"`
		Date     string  `json:"expense_date"`
		Category string  `json:"category"`
		Amount   string  `json:"amount_cents"`
		Method   string  `json:"method"`
		Note     string  `json:"note"`
		Shift    *string `json:"shift_id"`
	}
	out := []expense{}
	for rows.Next() {
		var x expense
		if err := rows.Scan(&x.ID, &x.Date, &x.Category, &x.Amount, &x.Method, &x.Note, &x.Shift); err != nil {
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
	httpx.JSON(w, http.StatusOK, map[string]any{"expenses": out, "next_cursor": next})
}

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
	"github.com/storepos/storepos/internal/inventory"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/sales"
)

func (s *Server) stockRoutes(mux *http.ServeMux) {
	mux.Handle("PUT /api/v1/stock/variants/{id}/opening-cost", s.guard(authz.PermBusinessManage, true, s.openingCost))
	mux.Handle("POST /api/v1/stock/adjustments", s.guard(authz.PermStockAdjust, true, s.adjustStock))
	mux.Handle("GET /api/v1/stock/card", s.guard(authz.PermProductView, false, s.stockCard))
}

func validStockReason(reason, note string) bool {
	if utf8.RuneCountInString(note) > 200 {
		return false
	}
	switch reason {
	case "damaged", "lost", "expired", "own_use", "correction":
		return true
	case "other":
		return strings.TrimSpace(note) != ""
	}
	return false
}

func (s *Server) adjustStock(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		BranchID  string       `json:"branch_id"`
		VariantID string       `json:"variant_id"`
		Qty       string       `json:"qty_milli"`
		Reason    string       `json:"reason"`
		Note      string       `json:"note"`
		Approval  *approvalReq `json:"approval"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.BranchID)
	if !ok {
		return
	}
	variant, e1 := idkit.Parse(req.VariantID)
	qty, e2 := money.ParseMilli(req.Qty)
	if e1 != nil || e2 != nil || qty == 0 || !validStockReason(req.Reason, req.Note) {
		httpx.WriteError(w, r, errs.Validation("varian/qty/alasan tidak valid; other wajib catatan maksimal 200 karakter"))
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
	if authz.Check(tc.Role, authz.PermStockAdjust) == authz.AllowedWithO && approved == nil {
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
	var after int64
	var replay *idempotency.Stored
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		st, e := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /api/v1/stock/adjustments", hash[:])
		replay = st
		if e != nil || st != nil {
			return e
		}
		var valid bool
		e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id
  JOIN branches b ON b.business_id=v.business_id AND b.id=$3
  WHERE v.business_id=$1 AND v.id=$2 AND p.track_stock AND p.status<>'archived' AND v.status<>'archived' AND b.status='active')`, tc.BusinessID, variant, branch).Scan(&valid)
		if e != nil {
			return e
		}
		if !valid {
			return errs.Validation("varian/cabang tidak valid atau produk tidak melacak stok")
		}
		kind := inventory.Kind("ADJUSTMENT_IN")
		if qty < 0 {
			kind = "ADJUSTMENT_OUT"
		}
		after, e = inventory.Apply(ctx, tx, inventory.Movement{BusinessID: tc.BusinessID, BranchID: branch, VariantID: variant, Kind: kind, QtyMilli: int64(qty), RefType: "adjustment", RefID: id, UserID: tc.UserID, Reason: req.Reason, Note: req.Note, ApprovedBy: approved})
		if e != nil {
			return e
		}
		if e := audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, ApprovedBy: approved, Action: "stock.adjusted", EntityType: "adjustment", EntityID: id.String(), After: map[string]string{"qty_milli": req.Qty, "balance_after_milli": money.FormatMilli(money.Milli(after))}, Reason: req.Reason + ": " + req.Note}); e != nil {
			return e
		}
		body, e := json.Marshal(map[string]string{"adjustment_id": id.String(), "balance_after_milli": money.FormatMilli(money.Milli(after))})
		if e != nil {
			return e
		}
		return idempotency.Complete(ctx, tx, tc.BusinessID, key, http.StatusCreated, body)
	})
	if err != nil {
		if errors.Is(err, inventory.ErrNegative) {
			httpx.WriteError(w, r, errs.New(errs.CodeStockInsufficient, "penyesuaian membuat stok negatif", http.StatusConflict))
		} else if errors.Is(err, idempotency.ErrConflict) {
			httpx.WriteError(w, r, mapSalesError(err))
		} else {
			httpx.WriteError(w, r, err)
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
	httpx.JSON(w, http.StatusCreated, map[string]string{"adjustment_id": id.String(), "balance_after_milli": money.FormatMilli(money.Milli(after))})
}

type stockCardRow struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Qty       string    `json:"qty_milli"`
	Balance   string    `json:"balance_after_milli"`
	RefType   string    `json:"ref_type"`
	RefID     string    `json:"ref_id"`
	Actor     string    `json:"created_by"`
	Reason    string    `json:"reason"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Server) stockCard(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return
	}
	variant, err := idkit.Parse(q.Get("variant_id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("variant_id wajib diisi"))
		return
	}
	var valid bool
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM variants WHERE business_id=$1 AND id=$2)`, tc.BusinessID, variant).Scan(&valid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !valid {
		httpx.WriteError(w, r, errs.NotFound("varian"))
		return
	}
	var cursor idkit.ID
	if q.Get("before") != "" {
		cursor, err = idkit.Parse(q.Get("before"))
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return
		}
	}
	// Cursor references a real movement timestamp, not UUID time (commit order can differ).
	var beforeTime *time.Time
	if !cursor.IsZero() {
		var t time.Time
		err = s.pool.Underlying().QueryRow(r.Context(), `SELECT created_at FROM stock_movements WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3 AND id=$4`, tc.BusinessID, branch, variant, cursor).Scan(&t)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, errs.Validation("cursor tidak ditemukan"))
			return
		}
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		beforeTime = &t
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,kind,qty_milli::text,balance_after_milli::text,ref_type,ref_id::text,created_by::text,COALESCE(reason,''),COALESCE(note,''),created_at
 FROM stock_movements WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3 AND ($4::timestamptz IS NULL OR (created_at,id)<($4,$5::uuid)) ORDER BY created_at DESC,id DESC LIMIT 101`, tc.BusinessID, branch, variant, beforeTime, cursor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	out := []stockCardRow{}
	for rows.Next() {
		var x stockCardRow
		if err := rows.Scan(&x.ID, &x.Kind, &x.Qty, &x.Balance, &x.RefType, &x.RefID, &x.Actor, &x.Reason, &x.Note, &x.CreatedAt); err != nil {
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
	httpx.JSON(w, http.StatusOK, map[string]any{"movements": out, "next_cursor": next})
}

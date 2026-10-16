package apiserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/idempotency"
	"github.com/storepos/storepos/internal/pharmacy"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) pharmacyCountRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/pharmacy/batches/{id}/count", s.pharmacyGuard(authz.PermBatchManage, true, s.countBatch))
}

// countBatch menyamakan saldo satu batch/status dengan hasil hitung fisik (APT-013).
// Selisih dicatat sebagai COUNT_ADJUSTMENT append-only dengan alasan dan audit.
func (s *Server) countBatch(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	batch, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("batch_id invalid"))
		return
	}
	var req struct {
		Branch  string `json:"branch_id"`
		Status  string `json:"status"`
		Counted string `json:"counted_milli"`
		Reason  string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.Branch)
	if !ok {
		return
	}
	counted, err := money.ParseMilli(req.Counted)
	if err != nil || counted < 0 || counted%1000 != 0 || (req.Status != "SELLABLE" && req.Status != "QUARANTINE") ||
		strings.TrimSpace(req.Reason) == "" || utf8.RuneCountInString(req.Reason) > 200 {
		httpx.WriteError(w, r, errs.Validation("status/hasil hitung (kelipatan satuan dasar)/alasan tidak valid"))
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key wajib diisi (maks 128 karakter)"))
		return
	}
	raw, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hash := sha256.Sum256(append([]byte(batch.String()), raw...))
	ref := idkit.New()
	var replay *idempotency.Stored
	var delta int64
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		st, e := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /pharmacy/batches/count", hash[:])
		if e != nil || st != nil {
			replay = st
			return e
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pharmacy_batches WHERE business_id=$1 AND id=$2)`, tc.BusinessID, batch).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return errs.NotFound("batch")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO pharmacy_batch_balances(business_id,branch_id,batch_id,status,qty_milli) VALUES($1,$2,$3,$4,0) ON CONFLICT DO NOTHING`, tc.BusinessID, branch, batch, req.Status); err != nil {
			return err
		}
		var current int64
		if err := tx.QueryRow(ctx, `SELECT qty_milli FROM pharmacy_batch_balances WHERE business_id=$1 AND branch_id=$2 AND batch_id=$3 AND status=$4 FOR UPDATE`, tc.BusinessID, branch, batch, req.Status).Scan(&current); err != nil {
			return err
		}
		delta = int64(counted) - current
		if delta != 0 {
			if err := pharmacy.Move(ctx, tx, tc.BusinessID, branch, batch, tc.UserID, ref, req.Status, "COUNT_ADJUSTMENT", req.Reason, delta); err != nil {
				return err
			}
		}
		if err := audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "pharmacy.batch_counted", EntityType: "batch", EntityID: batch.String(), Reason: req.Reason}); err != nil {
			return err
		}
		return idempotency.Complete(ctx, tx, tc.BusinessID, key, http.StatusOK, []byte(`{"status":"ok"}`))
	})
	if err != nil {
		if errors.Is(err, pharmacy.ErrBatch) {
			err = errs.New(errs.CodeStockInsufficient, err.Error(), http.StatusConflict)
		}
		s.pharmacyError(w, r, err)
		return
	}
	if replay != nil {
		w.Header().Set("Idempotent-Replay", "true")
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

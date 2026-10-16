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
	"github.com/storepos/storepos/internal/inventory"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
)

// Opening cost is a one-time initialization of unknown current cost, not a historical rewrite.
func (s *Server) openingCost(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	vid, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("variant_id tidak valid"))
		return
	}
	var req struct {
		BranchID string `json:"branch_id"`
		Cost     string `json:"average_cost_cents"`
		Reason   string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.BranchID)
	if !ok {
		return
	}
	cost, err := money.ParseCents(req.Cost)
	req.Reason = strings.TrimSpace(req.Reason)
	if err != nil || cost < 0 || req.Reason == "" || utf8.RuneCountInString(req.Reason) > 200 {
		httpx.WriteError(w, r, errs.Validation("HPP tidak negatif dan alasan maksimal 200 karakter wajib"))
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key wajib maksimal 128 karakter"))
		return
	}
	raw, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hash := sha256.Sum256(raw)
	body, err := json.Marshal(map[string]string{"variant_id": vid.String(), "branch_id": branch.String(), "average_cost_cents": money.FormatCents(cost)})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var replay *idempotency.Stored
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		st, e := idempotency.Begin(ctx, tx, tc.BusinessID, key, "PUT /api/v1/stock/variants/"+vid.String()+"/opening-cost", hash[:])
		replay = st
		if e != nil || st != nil {
			return e
		}
		var valid bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id JOIN branches b ON b.business_id=v.business_id AND b.id=$3 WHERE v.business_id=$1 AND v.id=$2 AND p.track_stock AND b.status='active' AND v.status<>'archived' AND p.status<>'archived')`, tc.BusinessID, vid, branch).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return errs.Validation("varian/cabang tidak aktif atau tidak melacak stok")
		}
		balance, err := inventory.Lock(ctx, tx, tc.BusinessID, branch, vid)
		if err != nil {
			return err
		}
		if balance <= 0 {
			return errs.Conflict("HPP awal memerlukan saldo positif")
		}
		var old *int64
		if err := tx.QueryRow(ctx, `SELECT average_cost_cents FROM inventory_balances WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, tc.BusinessID, branch, vid).Scan(&old); err != nil {
			return err
		}
		if old != nil {
			return errs.Conflict("HPP sudah diketahui; gunakan penerimaan berbiaya untuk pembaruan moving average")
		}
		if _, err := tx.Exec(ctx, `UPDATE inventory_balances SET average_cost_cents=$4,updated_at=now() WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, tc.BusinessID, branch, vid, int64(cost)); err != nil {
			return err
		}
		if err := audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "stock.opening_cost_set", EntityType: "variant", EntityID: vid.String(), Before: map[string]any{"average_cost_cents": nil}, After: map[string]string{"average_cost_cents": money.FormatCents(cost)}, Reason: req.Reason}); err != nil {
			return err
		}
		return idempotency.Complete(ctx, tx, tc.BusinessID, key, http.StatusOK, body)
	})
	if err != nil {
		if errors.Is(err, idempotency.ErrConflict) {
			httpx.WriteError(w, r, mapSalesError(err))
		} else {
			httpx.WriteError(w, r, err)
		}
		return
	}
	if replay != nil {
		body = replay.Body
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

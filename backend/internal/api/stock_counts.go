package apiserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/idempotency"
	"github.com/storepos/storepos/internal/inventory"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) countRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/stock/counts", s.guard(authz.PermStockCountInput, true, s.startCount))
	mux.Handle("GET /api/v1/stock/counts/{id}", s.guard(authz.PermStockCountInput, false, s.getCount))
	mux.Handle("PUT /api/v1/stock/counts/{id}/lines/{variant}", s.guard(authz.PermStockCountInput, true, s.recordCount))
	mux.Handle("POST /api/v1/stock/counts/{id}/post", s.guard(authz.PermStockCountPost, true, s.postCount))
}
func countError(err error) error {
	switch {
	case errors.Is(err, inventory.ErrCountNotFound):
		return errs.NotFound("sesi opname")
	case errors.Is(err, inventory.ErrCountConflict):
		return errs.New(errs.CodeOptimisticLockFailure, err.Error(), http.StatusConflict)
	case errors.Is(err, inventory.ErrCountInvalid):
		return errs.Validation(err.Error())
	case errors.Is(err, inventory.ErrNegative):
		return errs.New(errs.CodeStockInsufficient, "posting opname membuat saldo negatif; hitung ulang item", http.StatusConflict)
	}
	return err
}
func countVersion(r *http.Request) (idkit.ID, int, error) {
	id, e1 := idkit.Parse(r.PathValue("id"))
	v, e2 := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if e1 != nil || e2 != nil || v <= 0 {
		return id, 0, errs.Validation("id dan If-Match versi positif wajib valid")
	}
	return id, v, nil
}
func (s *Server) startCount(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		BranchID string `json:"branch_id"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.BranchID)
	if !ok {
		return
	}
	var id idkit.ID
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var err error
		id, err = inventory.StartCount(ctx, tx, tc.BusinessID, branch, tc.UserID)
		return err
	})
	if err != nil {
		httpx.WriteError(w, r, countError(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"count_id": id.String(), "version": 1, "status": "draft"})
}
func (s *Server) recordCount(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	variant, err := idkit.Parse(r.PathValue("variant"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("variant_id tidak valid"))
		return
	}
	var req struct {
		Qty string `json:"counted_qty_milli"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	qty, err := money.ParseMilli(req.Qty)
	if err != nil || qty < 0 {
		httpx.WriteError(w, r, errs.Validation("jumlah fisik harus integer milli-unit >= 0"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		return inventory.RecordCount(ctx, tx, tc.BusinessID, id, variant, tc.UserID, v, int64(qty), tc.HasBranch)
	})
	if err != nil {
		httpx.WriteError(w, r, countError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"count_id": id.String(), "version": v + 1})
}
func (s *Server) postCount(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key maksimal 128 karakter"))
		return
	}
	// shortcut: optional key keeps existing posting clients working; require after frontend migration.
	hash := sha256.Sum256([]byte(id.String() + ":" + strconv.Itoa(v)))
	body, err := json.Marshal(map[string]any{"count_id": id.String(), "version": v + 1, "status": "posted"})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var replay *idempotency.Stored
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		err := tx.QueryRow(ctx, `SELECT branch_id FROM stock_counts WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&branch)
		if errors.Is(err, pgx.ErrNoRows) {
			return inventory.ErrCountNotFound
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		if key != "" {
			st, e := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /api/v1/stock/counts/"+id.String()+"/post", hash[:])
			replay = st
			if e != nil || st != nil {
				return e
			}
		}
		if err := inventory.PostCount(ctx, tx, tc.BusinessID, id, tc.UserID, v, tc.HasBranch); err != nil {
			return err
		}
		if key != "" {
			return idempotency.Complete(ctx, tx, tc.BusinessID, key, http.StatusOK, body)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, idempotency.ErrConflict) {
			httpx.WriteError(w, r, mapSalesError(err))
		} else {
			httpx.WriteError(w, r, countError(err))
		}
		return
	}
	if replay != nil {
		w.Header().Set("Idempotent-Replay", "true")
		body = replay.Body
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
func (s *Server) getCount(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("count_id tidak valid"))
		return
	}
	// Header and lines must represent the same version while counters are being edited.
	var out map[string]any
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		var status string
		var version int
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stock_counts WHERE business_id=$1 AND id=$2)`, tc.BusinessID, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return inventory.ErrCountNotFound
		}
		if err := tx.QueryRow(ctx, `SELECT branch_id,status,version FROM stock_counts WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, id).Scan(&branch, &status, &version); err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return inventory.ErrCountNotFound
		}
		rows, err := tx.Query(ctx, `SELECT variant_id::text,snapshot_qty_milli::text,system_qty_at_count::text,counted_qty_milli::text,counted_at FROM stock_count_lines WHERE business_id=$1 AND count_id=$2 ORDER BY variant_id`, tc.BusinessID, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		type line struct {
			Variant  string     `json:"variant_id"`
			Snapshot string     `json:"snapshot_qty_milli"`
			System   *string    `json:"system_qty_at_count"`
			Counted  *string    `json:"counted_qty_milli"`
			At       *time.Time `json:"counted_at"`
		}
		lines := []line{}
		for rows.Next() {
			var l line
			if err := rows.Scan(&l.Variant, &l.Snapshot, &l.System, &l.Counted, &l.At); err != nil {
				return err
			}
			lines = append(lines, l)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		out = map[string]any{"count_id": id.String(), "branch_id": branch.String(), "status": status, "version": version, "lines": lines}
		return nil
	})
	if err != nil {
		httpx.WriteError(w, r, countError(err))
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

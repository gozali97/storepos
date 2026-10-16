package apiserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/storepos/storepos/internal/idempotency"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/sales"
)

func (s *Server) voidSale(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("sale_id tidak valid"))
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key wajib diisi, maksimal 128 karakter"))
		return
	}
	var req struct {
		Reason   string       `json:"reason"`
		Approval *approvalReq `json:"approval"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Reversal always requires O, including for roles with direct void permission.
	appr, err := s.approver(r.Context(), tc, req.Approval)
	if err != nil {
		httpx.WriteError(w, r, mapShiftError(err))
		return
	}
	if appr == nil {
		httpx.WriteError(w, r, mapShiftError(sales.ErrApprovalRequired))
		return
	}
	payload, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hash := sha256.Sum256(payload)
	var replay *idempotency.Stored
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		st, err := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /api/v1/sales/"+id.String()+"/void", hash[:])
		replay = st
		if err != nil || st != nil {
			return err
		}
		if err := sales.Void(ctx, tx, tc.BusinessID, id, tc.UserID, *appr, req.Reason, tc.HasBranch); err != nil {
			return err
		}
		return idempotency.Complete(ctx, tx, tc.BusinessID, key, http.StatusOK, []byte(`{"status":"voided"}`))
	})
	if err != nil {
		switch {
		case errors.Is(err, sales.ErrVoidClosedShift), errors.Is(err, sales.ErrVoidState):
			httpx.WriteError(w, r, errs.New(errs.CodeInvalidStateTransit, err.Error(), http.StatusConflict))
		case errors.Is(err, idempotency.ErrConflict):
			httpx.WriteError(w, r, mapSalesError(err))
		default:
			httpx.WriteError(w, r, mapShiftError(err))
		}
		return
	}
	if replay != nil {
		w.Header().Set("Idempotent-Replay", "true")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(replay.Status)
		_, _ = w.Write(replay.Body)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "voided"})
}

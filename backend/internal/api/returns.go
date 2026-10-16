package apiserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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

func (s *Server) returnSale(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	sale, err := idkit.Parse(r.PathValue("id"))
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
		RegisterID string       `json:"register_id"`
		Reason     string       `json:"reason"`
		Approval   *approvalReq `json:"approval"`
		Lines      []struct {
			LineID    string `json:"sale_line_id"`
			Qty       string `json:"qty_milli"`
			Condition string `json:"condition"`
		} `json:"lines"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	in := sales.ReturnInput{BusinessID: tc.BusinessID, SaleID: sale, UserID: tc.UserID, Reason: req.Reason, HasBranch: tc.HasBranch}
	if req.RegisterID != "" {
		in.RegisterID, err = idkit.Parse(req.RegisterID)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("register_id tidak valid"))
			return
		}
	}
	for _, l := range req.Lines {
		id, e1 := idkit.Parse(l.LineID)
		qty, e2 := money.ParseMilli(l.Qty)
		if e1 != nil || e2 != nil || qty <= 0 {
			httpx.WriteError(w, r, errs.Validation("baris retur tidak valid"))
			return
		}
		in.Lines = append(in.Lines, sales.ReturnLine{LineID: id, Qty: int64(qty), Condition: l.Condition})
	}
	in.ApprovedBy, err = s.approver(r.Context(), tc, req.Approval)
	if err != nil {
		httpx.WriteError(w, r, mapShiftError(err))
		return
	}
	if authz.Check(tc.Role, authz.PermSaleReturn) == authz.AllowedWithO && in.ApprovedBy == nil {
		httpx.WriteError(w, r, mapShiftError(sales.ErrApprovalRequired))
		return
	}
	raw, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hash := sha256.Sum256(raw)
	var res *sales.ReturnResult
	var replay *idempotency.Stored
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		st, e := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /api/v1/sales/"+sale.String()+"/returns", hash[:])
		replay = st
		if e != nil || st != nil {
			return e
		}
		res, e = sales.Return(ctx, tx, in)
		if e != nil {
			return e
		}
		body, e := json.Marshal(res)
		if e != nil {
			return e
		}
		return idempotency.Complete(ctx, tx, tc.BusinessID, key, http.StatusCreated, body)
	})
	if err != nil {
		switch {
		case errors.Is(err, sales.ErrReturnWindow):
			httpx.WriteError(w, r, errs.Validation(err.Error()))
		case errors.Is(err, sales.ErrReturnComponents):
			httpx.WriteError(w, r, errs.Conflict(err.Error()))
		case errors.Is(err, sales.ErrReturnQuantity), errors.Is(err, sales.ErrReturnMethod):
			httpx.WriteError(w, r, errs.Validation(err.Error()))
		case errors.Is(err, sales.ErrReturnState):
			httpx.WriteError(w, r, errs.New(errs.CodeInvalidStateTransit, err.Error(), http.StatusConflict))
		case errors.Is(err, idempotency.ErrConflict):
			httpx.WriteError(w, r, mapSalesError(err))
		default:
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
	httpx.JSON(w, http.StatusCreated, res)
}

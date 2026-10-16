package apiserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strconv"
	"strings"

	"github.com/storepos/storepos/internal/idempotency"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/purchasing"
)

func (s *Server) purchasingRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/goods-receipts", s.guard(authz.PermGoodsReceipt, false, s.listGoodsReceipts))
	mux.Handle("GET /api/v1/goods-receipts/{id}", s.guard(authz.PermGoodsReceipt, false, s.getGoodsReceipt))
	mux.Handle("PUT /api/v1/goods-receipts/{id}", s.guard(authz.PermGoodsReceipt, true, s.editGoodsReceipt))
	mux.Handle("POST /api/v1/purchase-units", s.guard(authz.PermProductManage, true, s.createPurchaseUnit))
	mux.Handle("POST /api/v1/goods-receipts", s.guard(authz.PermGoodsReceipt, true, s.createGoodsReceipt))
	mux.Handle("POST /api/v1/goods-receipts/{id}/post", s.guard(authz.PermGoodsReceipt, true, s.postGoodsReceipt))
}
func purchaseError(err error) error {
	switch {
	case errors.Is(err, purchasing.ErrNotFound):
		return errs.NotFound("penerimaan")
	case errors.Is(err, purchasing.ErrConflict):
		return errs.New(errs.CodeOptimisticLockFailure, err.Error(), http.StatusConflict)
	case errors.Is(err, purchasing.ErrInvalid):
		return errs.Validation(err.Error())
	case errors.Is(err, purchasing.ErrUnknownCost):
		return errs.Conflict(err.Error())
	}
	return err
}
func (s *Server) createPurchaseUnit(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		VariantID string `json:"variant_id"`
		Name      string `json:"name"`
		Factor    string `json:"factor_milli"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	vid, e1 := idkit.Parse(req.VariantID)
	factor, e2 := money.ParseMilli(req.Factor)
	if e1 != nil || e2 != nil || factor <= 0 || strings.TrimSpace(req.Name) == "" {
		httpx.WriteError(w, r, errs.Validation("varian/nama/faktor tidak valid"))
		return
	}
	id := idkit.New()
	tag, err := s.pool.Underlying().Exec(r.Context(), `INSERT INTO purchase_units(id,business_id,variant_id,name,factor_milli)
 SELECT $1,$2,id,$4,$5 FROM variants WHERE business_id=$2 AND id=$3 ON CONFLICT DO NOTHING`, id, tc.BusinessID, vid, strings.TrimSpace(req.Name), int64(factor))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if tag.RowsAffected() != 1 {
		httpx.WriteError(w, r, errs.Conflict("varian tidak ditemukan atau nama satuan sudah dipakai"))
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"unit_id": id.String()})
}
func (s *Server) createGoodsReceipt(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		BranchID   string `json:"branch_id"`
		SupplierID string `json:"supplier_id"`
		Invoice    string `json:"invoice_number"`
		Date       string `json:"received_date"`
		Lines      []struct {
			UnitID string `json:"unit_id"`
			Qty    string `json:"qty_milli"`
			Price  string `json:"purchase_price_cents"`
		} `json:"lines"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.BranchID)
	if !ok {
		return
	}
	lines := []purchasing.Line{}
	for _, l := range req.Lines {
		id, e1 := idkit.Parse(l.UnitID)
		qty, e2 := money.ParseMilli(l.Qty)
		price, e3 := money.ParseCents(l.Price)
		if e1 != nil || e2 != nil || e3 != nil {
			httpx.WriteError(w, r, errs.Validation("baris penerimaan tidak valid"))
			return
		}
		lines = append(lines, purchasing.Line{UnitID: id, QtyMilli: int64(qty), PriceCents: int64(price)})
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key maksimal 128 karakter"))
		return
	}
	raw, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hash := sha256.Sum256(raw)
	var replay *idempotency.Stored
	var response []byte
	var id idkit.ID
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		// shortcut: key optional for legacy clients; all built-in receipt forms send it.
		if key != "" {
			stored, err := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /api/v1/goods-receipts", hash[:])
			replay = stored
			if err != nil || stored != nil {
				return err
			}
		}
		var supplier *idkit.ID
		if req.SupplierID != "" {
			sid, err := idkit.Parse(req.SupplierID)
			if err != nil {
				return errs.Validation("supplier_id tidak valid")
			}
			var archived bool
			err = tx.QueryRow(ctx, `SELECT archived FROM suppliers WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, sid).Scan(&archived)
			if errors.Is(err, pgx.ErrNoRows) {
				return errs.NotFound("supplier")
			}
			if err != nil {
				return err
			}
			if archived {
				return errs.Conflict("supplier diarsipkan tidak dapat dipakai di penerimaan baru")
			}
			supplier = &sid
		}
		var err error
		id, err = purchasing.Create(ctx, tx, tc.BusinessID, branch, tc.UserID, req.Invoice, req.Date, lines)
		if err != nil {
			return err
		}
		if supplier != nil {
			if _, err = tx.Exec(ctx, `UPDATE goods_receipts SET supplier_id=$3 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, *supplier); err != nil {
				return err
			}
		}
		response, err = json.Marshal(map[string]any{"receipt_id": id.String(), "status": "draft", "version": 1})
		if err != nil {
			return err
		}
		if key != "" {
			return idempotency.Complete(ctx, tx, tc.BusinessID, key, http.StatusCreated, response)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, idempotency.ErrConflict) {
			httpx.WriteError(w, r, mapSalesError(err))
		} else {
			httpx.WriteError(w, r, purchaseError(err))
		}
		return
	}
	if replay != nil {
		response = replay.Body
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(response)
}
func (s *Server) postGoodsReceipt(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, e1 := idkit.Parse(r.PathValue("id"))
	v, e2 := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if e1 != nil || e2 != nil || v <= 0 {
		httpx.WriteError(w, r, errs.Validation("id dan If-Match wajib valid"))
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key maksimal 128 karakter"))
		return
	}
	// shortcut: key optional for existing clients; require it after frontend posting client migration.
	hash := sha256.Sum256([]byte(id.String() + ":" + strconv.Itoa(v)))
	body, err := json.Marshal(map[string]any{"receipt_id": id.String(), "status": "posted", "version": v + 1})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var replay *idempotency.Stored
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		// Recheck branch access even on replay: an old response must not bypass current assignment.
		var branch idkit.ID
		err := tx.QueryRow(ctx, `SELECT branch_id FROM goods_receipts WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&branch)
		if errors.Is(err, pgx.ErrNoRows) {
			return purchasing.ErrNotFound
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		if key != "" {
			st, e := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /api/v1/goods-receipts/"+id.String()+"/post", hash[:])
			replay = st
			if e != nil || st != nil {
				return e
			}
		}
		if err := purchasing.Post(ctx, tx, tc.BusinessID, id, tc.UserID, v, tc.HasBranch); err != nil {
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
			httpx.WriteError(w, r, purchaseError(err))
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

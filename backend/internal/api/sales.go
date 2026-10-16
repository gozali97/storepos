package apiserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/storepos/storepos/internal/idempotency"
	"github.com/storepos/storepos/internal/inventory"
	"github.com/storepos/storepos/internal/org"
	"github.com/storepos/storepos/internal/pharmacy"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/sales"
)

func (s *Server) salesRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/sales/{id}/returns", s.guard(authz.PermSaleReturn, true, s.returnSale))
	mux.Handle("POST /api/v1/sales/{id}/void", s.guard(authz.PermSaleVoid, true, s.voidSale))
	mux.Handle("POST /api/v1/shifts/open", s.guard(authz.PermShiftManageOwn, true, s.openShift))
	mux.Handle("POST /api/v1/stock/receipts", s.guard(authz.PermGoodsReceipt, true, s.receiveStock))
	mux.Handle("POST /api/v1/sales/finalize", s.guard(authz.PermSaleCreate, true, s.finalizeSale))
}

// branchOf memvalidasi branch_id dan penugasan cabang pengguna (CORE-005).
func branchOf(w http.ResponseWriter, r *http.Request, tc tenant.Context, raw string) (idkit.ID, bool) {
	id, err := idkit.Parse(raw)
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("branch_id tidak valid"))
		return id, false
	}
	if !tc.HasBranch(id) {
		httpx.WriteError(w, r, errs.PermissionDenied(""))
		return id, false
	}
	return id, true
}

func (s *Server) openShift(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		BranchID     string `json:"branch_id"`
		RegisterID   string `json:"register_id"`
		OpeningCents string `json:"opening_cash_cents"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.BranchID)
	if !ok {
		return
	}
	reg, err := idkit.Parse(req.RegisterID)
	opening, err2 := money.ParseCents(req.OpeningCents)
	if err != nil || err2 != nil || opening < 0 {
		httpx.WriteError(w, r, errs.Validation("register_id atau opening_cash_cents tidak valid"))
		return
	}
	var id idkit.ID
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var e error
		id, e = sales.OpenShift(ctx, tx, tc.BusinessID, branch, reg, tc.UserID, int64(opening))
		return e
	})
	switch {
	case errors.Is(err, sales.ErrShiftAlreadyOpen):
		httpx.WriteError(w, r, errs.Conflict(err.Error()))
	case errors.Is(err, sales.ErrNotFound):
		httpx.WriteError(w, r, errs.NotFound("terminal"))
	case err != nil:
		httpx.WriteError(w, r, errs.Internal("gagal membuka shift").Wrap(err))
	default:
		httpx.JSON(w, http.StatusCreated, map[string]string{"shift_id": id.String()})
	}
}

func (s *Server) receiveStock(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		BranchID string `json:"branch_id"`
		Lines    []struct {
			VariantID string `json:"variant_id"`
			QtyMilli  string `json:"qty_milli"`
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
	lines := make([]inventory.ReceiptLine, 0, len(req.Lines))
	for _, l := range req.Lines {
		vid, e1 := idkit.Parse(l.VariantID)
		q, e2 := money.ParseMilli(l.QtyMilli)
		if e1 != nil || e2 != nil || q <= 0 {
			httpx.WriteError(w, r, errs.Validation("baris penerimaan tidak valid (variant_id, qty_milli > 0)"))
			return
		}
		lines = append(lines, inventory.ReceiptLine{VariantID: vid, QtyMilli: int64(q)})
	}
	var ref idkit.ID
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var e error
		ref, e = inventory.Receive(ctx, tx, tc.BusinessID, branch, tc.UserID, lines)
		return e
	})
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("penerimaan gagal: varian tidak valid atau qty salah").Wrap(err))
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"receipt_id": ref.String()})
}

type finalizeReq struct {
	Note         string `json:"note"`
	CustomerID   string `json:"customer_id,omitempty"`
	DraftID      string `json:"draft_id"`
	DraftVersion int    `json:"draft_version"`
	BranchID     string `json:"branch_id"`
	RegisterID   string `json:"register_id"`
	Lines        []struct {
		Note        string      `json:"note"`
		VariantID   string      `json:"variant_id"`
		UnitID      string      `json:"unit_id,omitempty"`
		QtyMilli    string      `json:"qty_milli"`
		Discount    discountReq `json:"discount"`
		BatchID     string      `json:"batch_id,omitempty"`
		BatchReason string      `json:"batch_reason,omitempty"`
	} `json:"lines"`
	PrescriptionRef string       `json:"prescription_reference,omitempty"`
	Pharmacist      *approvalReq `json:"pharmacist_verification,omitempty"`
	Discount        discountReq  `json:"discount"`
	DiscountReason  string       `json:"discount_reason"`
	Approval        *approvalReq `json:"approval"`
	Payments        []struct {
		Method      string `json:"method"`
		AmountCents string `json:"amount_cents"`
	} `json:"payments"`
}

func (s *Server) finalizeSale(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("header Idempotency-Key wajib diisi (maks 128 karakter)"))
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("body terlalu besar atau tidak terbaca"))
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	var req finalizeReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.BranchID)
	if !ok {
		return
	}
	reg, err := idkit.Parse(req.RegisterID)
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("register_id tidak valid"))
		return
	}
	in := sales.Input{BusinessID: tc.BusinessID, BranchID: branch, RegisterID: reg, UserID: tc.UserID, Note: req.Note}
	for _, l := range req.Lines {
		vid, e1 := idkit.Parse(l.VariantID)
		q, e2 := money.ParseMilli(l.QtyMilli)
		if e1 != nil || e2 != nil {
			httpx.WriteError(w, r, errs.Validation("baris tidak valid (variant_id, qty_milli)"))
			return
		}
		d, err := l.Discount.parse()
		if err != nil {
			httpx.WriteError(w, r, errs.Validation(err.Error()))
			return
		}
		var unit *idkit.ID
		if l.UnitID != "" {
			id, e := idkit.Parse(l.UnitID)
			if e != nil {
				httpx.WriteError(w, r, errs.Validation("unit_id tidak valid"))
				return
			}
			unit = &id
		}
		var batch *idkit.ID
		if l.BatchID != "" {
			id, e := idkit.Parse(l.BatchID)
			if e != nil {
				httpx.WriteError(w, r, errs.Validation("batch_id tidak valid"))
				return
			}
			batch = &id
		}
		in.Lines = append(in.Lines, sales.LineIn{VariantID: vid, UnitID: unit, QtyMilli: int64(q), Discount: d, Note: l.Note, BatchID: batch, BatchReason: l.BatchReason})
	}
	for _, p := range req.Payments {
		a, e := money.ParseCents(p.AmountCents)
		if e != nil {
			httpx.WriteError(w, r, errs.Validation("amount_cents harus bilangan sen (string)"))
			return
		}
		in.Payments = append(in.Payments, sales.PaymentIn{Method: p.Method, AmountCents: int64(a)})
	}

	in.TxDiscount, err = req.Discount.parse()
	if err != nil {
		httpx.WriteError(w, r, errs.Validation(err.Error()))
		return
	}
	in.DiscountReason = req.DiscountReason
	policy, err := org.LoadPolicy(r.Context(), s.pool.Underlying(), tc.BusinessID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	in.DiscountLimitBps = policy.CashierDiscountBps
	in.UnlimitedDiscount = authz.Check(tc.Role, authz.PermDiscountOverride) == authz.Allowed

	// Verifikasi sebelum mengambil koneksi transaksi: pool tidak boleh habis oleh transaksi yang menunggu verifikasi PIN.
	in.DiscountApprovedBy, err = s.approver(r.Context(), tc, req.Approval)
	if err != nil {
		httpx.WriteError(w, r, mapShiftError(err))
		return
	}
	// Apoteker yang sedang login otomatis terverifikasi; selain itu butuh login/PIN apoteker (APT-009).
	in.PrescriptionRef = req.PrescriptionRef
	in.CanOverrideBatch = tc.Role == tenant.RolePharmacist
	if tc.Role == tenant.RolePharmacist {
		in.PharmacistVerified, in.VerifiedBy = true, &tc.UserID
	} else if req.Pharmacist != nil {
		id, err := s.pharmacistApprover(r.Context(), tc, req.Pharmacist)
		if err != nil {
			httpx.WriteError(w, r, mapShiftError(err))
			return
		}
		in.PharmacistVerified, in.VerifiedBy, in.CanOverrideBatch = true, &id, true
	}
	sum := sha256.Sum256(raw)
	var replay *idempotency.Stored
	var res *sales.Result
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		replay, res = nil, nil
		st, e := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /api/v1/sales/finalize", sum[:])
		if e != nil || st != nil {
			replay = st
			return e
		}
		if req.DraftID != "" {
			if e = s.checkDraftFinalization(ctx, tx, tc, req, branch, reg); e != nil {
				return e
			}
		}
		if res, e = sales.Finalize(ctx, tx, in); e != nil {
			return e
		}
		if req.CustomerID != "" {
			cid, err := idkit.Parse(req.CustomerID)
			if err != nil {
				return errs.Validation("customer_id tidak valid")
			}
			var anonymized bool
			err = tx.QueryRow(ctx, `SELECT anonymized FROM customers WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, cid).Scan(&anonymized)
			if errors.Is(err, pgx.ErrNoRows) {
				return errs.NotFound("pelanggan")
			}
			if err != nil {
				return err
			}
			if anonymized {
				return errs.Conflict("pelanggan sudah dianonimkan")
			}
			if _, err := tx.Exec(ctx, `UPDATE sales SET customer_id=$3 WHERE business_id=$1 AND id=$2`, tc.BusinessID, res.SaleID, cid); err != nil {
				return err
			}
		}
		if req.DraftID != "" {
			if _, e = tx.Exec(ctx, `UPDATE sale_drafts SET status='completed',completed_sale_id=$3,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, req.DraftID, res.SaleID); e != nil {
				return e
			}
		}
		body, _ := json.Marshal(saleResponse(res))
		return idempotency.Complete(ctx, tx, tc.BusinessID, key, http.StatusCreated, body)
	})
	if err != nil {
		httpx.WriteError(w, r, mapSalesError(err))
		return
	}
	if replay != nil {
		w.Header().Set("Idempotent-Replay", "true")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(replay.Status)
		_, _ = w.Write(replay.Body)
		return
	}
	httpx.JSON(w, http.StatusCreated, saleResponse(res))
}

func saleResponse(r *sales.Result) map[string]string {
	return map[string]string{
		"sale_id": r.SaleID.String(), "receipt_number": r.ReceiptNumber,
		"total_cents": strconv.FormatInt(r.TotalCents, 10), "change_cents": strconv.FormatInt(r.ChangeCents, 10),
		"cash_rounding_cents": strconv.FormatInt(r.CashRoundingCents, 10),
		"tax_cents":           strconv.FormatInt(r.TaxCents, 10),
		"delivery_fee_cents":  strconv.FormatInt(r.DeliveryFeeCents, 10),
	}
}

func mapSalesError(err error) error {
	if _, ok := errs.As(err); ok {
		return err
	}
	var ie *sales.InsufficientError
	var pe *sales.PaymentError
	switch {
	case errors.As(err, &ie):
		d := make([]errs.FieldError, 0, len(ie.Items))
		for _, it := range ie.Items {
			d = append(d, errs.FieldError{Field: it.VariantID, Code: "insufficient",
				Message: it.ProductName + " (" + it.SKU + "): diminta " + strconv.FormatInt(it.Requested, 10) +
					", tersedia " + strconv.FormatInt(it.Available, 10) + " (milli-unit)"})
		}
		return errs.New(errs.CodeStockInsufficient, "stok tidak mencukupi", http.StatusConflict).WithDetails(d...)
	case errors.Is(err, sales.ErrApprovalRequired), errors.Is(err, org.ErrApprovalDenied), errors.Is(err, org.ErrPINLocked):
		return mapShiftError(err)
	case errors.Is(err, sales.ErrPrescriptionRequired):
		return errs.New(errs.CodePrescriptionRequired, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, sales.ErrBatchOverride):
		return errs.Validation(err.Error())
	case errors.Is(err, pharmacy.ErrExpired):
		return errs.New(errs.CodeBatchExpired, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, sales.ErrInvalidDeliveryFee), errors.Is(err, sales.ErrInvalidNote), errors.Is(err, sales.ErrReasonRequired), errors.Is(err, sales.ErrInvalidDiscount), errors.Is(err, sales.ErrDiscountExceeds):
		return errs.Validation(err.Error())
	case errors.Is(err, sales.ErrShiftNotOpen):
		return errs.New(errs.CodeShiftNotOpen, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, idempotency.ErrConflict):
		return errs.New(errs.CodeIdempotencyConflict, err.Error(), http.StatusConflict)
	case errors.As(err, &pe), errors.Is(err, sales.ErrNoLines), errors.Is(err, sales.ErrInvalidQty),
		errors.Is(err, sales.ErrVariantUnavailable), errors.Is(err, sales.ErrZeroTotal):
		return errs.Validation(err.Error())
	case errors.Is(err, sales.ErrNotFound):
		return errs.NotFound("cabang atau terminal")
	}
	return errs.Internal("gagal menyelesaikan transaksi").Wrap(err)
}

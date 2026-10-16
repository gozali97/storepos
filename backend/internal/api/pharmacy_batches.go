package apiserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
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

func (s *Server) pharmacyGuard(perm authz.Permission, write bool, h http.HandlerFunc) http.Handler {
	check := func(tc tenant.Context) error {
		if write {
			grant, err := authz.Require(tc, perm, idkit.ID{})
			if err != nil {
				return err
			}
			if grant != authz.Allowed {
				return errs.PermissionDenied("")
			}
			return nil
		}
		return authz.RequireRead(tc, perm, idkit.ID{})
	}
	return CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(RequireModule("pharmacy")(RequirePermission(check)(h)))))
}
func (s *Server) pharmacyRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/reports/stock/batches", s.pharmacyGuard(authz.PermProductView, false, s.reportBatchStock))
	mux.Handle("GET /api/v1/pharmacy/batches/{id}/trace", s.pharmacyGuard(authz.PermBatchManage, false, s.batchTrace))
	mux.Handle("GET /api/v1/pharmacy/products", s.pharmacyGuard(authz.PermProductView, false, s.drugList))
	mux.Handle("GET /api/v1/pharmacy/batch-receipts", s.pharmacyGuard(authz.PermBatchManage, false, s.batchReceiptHistory))
	mux.Handle("PUT /api/v1/pharmacy/products/{id}", s.pharmacyGuard(authz.PermProductManage, true, s.saveDrug))
	mux.Handle("GET /api/v1/pharmacy/products/{id}", s.pharmacyGuard(authz.PermProductView, false, s.getDrug))
	mux.Handle("POST /api/v1/pharmacy/batch-receipts", s.pharmacyGuard(authz.PermBatchManage, true, s.receiveBatch))
	mux.Handle("POST /api/v1/pharmacy/batches/{id}/release", s.pharmacyGuard(authz.PermQuarantineRelease, true, s.batchAction))
	mux.Handle("POST /api/v1/pharmacy/batches/{id}/dispose", s.pharmacyGuard(authz.PermBatchManage, true, s.batchAction))
	mux.Handle("GET /api/v1/pharmacy/batches", s.pharmacyGuard(authz.PermProductView, false, s.listBatches))
	mux.Handle("GET /api/v1/pharmacy/fefo-preview", s.pharmacyGuard(authz.PermSaleCreate, false, s.fefoPreview))
	mux.Handle("GET /api/v1/pharmacy/batches/{id}/movements", s.pharmacyGuard(authz.PermBatchManage, false, s.batchMovements))
}

type drugDTO struct {
	Ingredient     string `json:"active_ingredient"`
	Form           string `json:"dosage_form"`
	Strength       string `json:"strength"`
	Manufacturer   string `json:"manufacturer"`
	Registration   string `json:"registration_number"`
	Classification string `json:"classification"`
	Prescription   bool   `json:"prescription_required"`
	Restricted     bool   `json:"special_restrictions"`
}

func (s *Server) saveDrug(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req drugDTO
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for _, x := range []string{req.Ingredient, req.Form, req.Strength, req.Manufacturer, req.Registration, req.Classification} {
		if utf8.RuneCountInString(x) > 500 {
			httpx.WriteError(w, r, errs.Validation("field obat maksimal 500 karakter"))
			return
		}
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if err := lockCatalogVersion(ctx, tx, tc.BusinessID, id, "products", v); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO pharmacy_drugs(business_id,product_id,active_ingredient,dosage_form,strength,manufacturer,registration_number,classification,prescription_required,special_restrictions) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(business_id,product_id) DO UPDATE SET active_ingredient=EXCLUDED.active_ingredient,dosage_form=EXCLUDED.dosage_form,strength=EXCLUDED.strength,manufacturer=EXCLUDED.manufacturer,registration_number=EXCLUDED.registration_number,classification=EXCLUDED.classification,prescription_required=EXCLUDED.prescription_required,special_restrictions=EXCLUDED.special_restrictions,version=pharmacy_drugs.version+1`, tc.BusinessID, id, req.Ingredient, req.Form, req.Strength, req.Manufacturer, req.Registration, req.Classification, req.Prescription, req.Restricted); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE products SET version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "pharmacy.drug_changed", EntityType: "product", EntityID: id.String(), After: map[string]bool{"prescription_required": req.Prescription, "special_restrictions": req.Restricted}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"product_id": id.String(), "version": v + 1})
}
func (s *Server) getDrug(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("id invalid"))
		return
	}
	var x drugDTO
	var v int
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT active_ingredient,dosage_form,strength,manufacturer,registration_number,classification,prescription_required,special_restrictions,version FROM pharmacy_drugs WHERE business_id=$1 AND product_id=$2`, tc.BusinessID, id).Scan(&x.Ingredient, &x.Form, &x.Strength, &x.Manufacturer, &x.Registration, &x.Classification, &x.Prescription, &x.Restricted, &v)
	if readError(w, r, err, "obat") {
		return
	}
	httpx.JSON(w, 200, map[string]any{"drug": x, "drug_version": v, "product_id": id.String()})
}
func (s *Server) receiveBatch(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		Branch   string `json:"branch_id"`
		Variant  string `json:"variant_id"`
		Batch    string `json:"batch_number"`
		Expiry   string `json:"expiry_date"`
		Supplier string `json:"supplier_id"`
		Invoice  string `json:"invoice_number"`
		Qty      string `json:"qty_milli"`
		Cost     string `json:"unit_cost_cents"`
		Status   string `json:"status"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.Branch)
	if !ok {
		return
	}
	variant, e1 := idkit.Parse(req.Variant)
	supplier, e2 := idkit.Parse(req.Supplier)
	ed, e3 := pharmacy.Expiry(req.Expiry)
	qty, e4 := money.ParseMilli(req.Qty)
	cost, e5 := money.ParseCents(req.Cost)
	req.Batch = strings.TrimSpace(req.Batch)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || qty <= 0 || qty%1000 != 0 || cost < 0 || req.Batch == "" || len(req.Batch) > 100 || len(req.Invoice) > 200 || (req.Status != "SELLABLE" && req.Status != "QUARANTINE") {
		httpx.WriteError(w, r, errs.Validation("batch/ED/supplier/qty dasar/HPP/status tidak valid"))
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key wajib"))
		return
	}
	raw, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hash := sha256.Sum256(raw)
	id := idkit.New()
	var batch idkit.ID
	var replay *idempotency.Stored
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		st, e := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /pharmacy/batch-receipts", hash[:])
		replay = st
		if e != nil || st != nil {
			return e
		}
		today, err := pharmacy.BranchDate(ctx, tx, tc.BusinessID, branch)
		if err != nil {
			return err
		}
		if !ed.After(today) && req.Status != "QUARANTINE" {
			return pharmacy.ErrExpired
		}
		var valid bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM suppliers WHERE business_id=$1 AND id=$2 AND NOT archived),EXISTS(SELECT 1 FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id WHERE v.business_id=$1 AND v.id=$3 AND v.status='active' AND p.status='active' AND p.track_stock AND NOT v.allow_fractional)`, tc.BusinessID, supplier, variant).Scan(&valid, new(bool)); err != nil {
			return err
		}
		if !valid {
			return errs.Validation("supplier harus aktif pada tenant")
		}
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id WHERE v.business_id=$1 AND v.id=$2 AND v.status='active' AND p.status='active' AND p.track_stock AND NOT v.allow_fractional)`, tc.BusinessID, variant).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return errs.Validation("varian apotek pelacak stok aktif nonpecahan wajib")
		}
		proposed := idkit.New()
		err = tx.QueryRow(ctx, `INSERT INTO pharmacy_batches(id,business_id,variant_id,batch_number,expiry_date) VALUES($1,$2,$3,$4,$5) ON CONFLICT(business_id,variant_id,batch_number,expiry_date) DO UPDATE SET batch_number=EXCLUDED.batch_number RETURNING id`, proposed, tc.BusinessID, variant, req.Batch, ed.Format("2006-01-02")).Scan(&batch)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO pharmacy_batch_receipts(id,business_id,branch_id,batch_id,supplier_id,invoice_number,qty_milli,unit_cost_cents,status,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id, tc.BusinessID, branch, batch, supplier, req.Invoice, int64(qty), int64(cost), req.Status, tc.UserID); err != nil {
			return err
		}
		if err := pharmacy.Move(ctx, tx, tc.BusinessID, branch, batch, tc.UserID, id, req.Status, "RECEIPT", "", int64(qty)); err != nil {
			return err
		}
		if err := audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "pharmacy.batch_received", EntityType: "batch_receipt", EntityID: id.String(), After: map[string]string{"batch_id": batch.String(), "status": req.Status}}); err != nil {
			return err
		}
		body, e := json.Marshal(map[string]string{"receipt_id": id.String(), "batch_id": batch.String()})
		if e != nil {
			return e
		}
		return idempotency.Complete(ctx, tx, tc.BusinessID, key, 201, body)
	})
	if err != nil {
		s.pharmacyError(w, r, err)
		return
	}
	if replay != nil {
		w.Header().Set("Idempotent-Replay", "true")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write(replay.Body)
		return
	}
	httpx.JSON(w, 201, map[string]string{"receipt_id": id.String(), "batch_id": batch.String()})
}
func (s *Server) pharmacyError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, pharmacy.ErrBatch) {
		httpx.WriteError(w, r, errs.New(errs.CodeStockInsufficient, err.Error(), 409))
	} else if errors.Is(err, pharmacy.ErrExpired) {
		httpx.WriteError(w, r, errs.New(errs.CodeBatchExpired, err.Error(), 422))
	} else if errors.Is(err, idempotency.ErrConflict) {
		httpx.WriteError(w, r, mapSalesError(err))
	} else {
		httpx.WriteError(w, r, err)
	}
}
func (s *Server) batchAction(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	batch, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("batch_id invalid"))
		return
	}
	release := strings.HasSuffix(r.URL.Path, "/release")
	if release && tc.Role != tenant.RolePharmacist {
		httpx.WriteError(w, r, errs.PermissionDenied("pelepasan karantina hanya Apoteker"))
		return
	}
	var req struct {
		Branch string `json:"branch_id"`
		Qty    string `json:"qty_milli"`
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.Branch)
	if !ok {
		return
	}
	qty, err := money.ParseMilli(req.Qty)
	if err != nil || qty <= 0 || qty%1000 != 0 || strings.TrimSpace(req.Reason) == "" || utf8.RuneCountInString(req.Reason) > 200 || (!release && req.Status != "SELLABLE" && req.Status != "QUARANTINE") {
		httpx.WriteError(w, r, errs.Validation("qty/status/alasan invalid"))
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("key wajib"))
		return
	}
	raw, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hash := sha256.Sum256(raw)
	ref := idkit.New()
	var replay *idempotency.Stored
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		st, e := idempotency.Begin(ctx, tx, tc.BusinessID, key, r.Pattern+batch.String(), hash[:])
		replay = st
		if e != nil || st != nil {
			return e
		}
		var ed string
		err := tx.QueryRow(ctx, `SELECT expiry_date::text FROM pharmacy_batches WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, batch).Scan(&ed)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("batch")
		}
		if err != nil {
			return err
		}
		if release {
			today, err := pharmacy.BranchDate(ctx, tx, tc.BusinessID, branch)
			if err != nil {
				return err
			}
			expiry, err := pharmacy.Expiry(ed)
			if err != nil {
				return err
			}
			if !expiry.After(today) {
				return pharmacy.ErrExpired
			}
			if err := pharmacy.Move(ctx, tx, tc.BusinessID, branch, batch, tc.UserID, ref, "QUARANTINE", "QUARANTINE_RELEASE_OUT", req.Reason, -int64(qty)); err != nil {
				return err
			}
			if err := pharmacy.Move(ctx, tx, tc.BusinessID, branch, batch, tc.UserID, ref, "SELLABLE", "QUARANTINE_RELEASE_IN", req.Reason, int64(qty)); err != nil {
				return err
			}
		} else {
			if err := pharmacy.Move(ctx, tx, tc.BusinessID, branch, batch, tc.UserID, ref, req.Status, "DISPOSE", req.Reason, -int64(qty)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO pharmacy_batch_balances(business_id,branch_id,batch_id,status,qty_milli) VALUES($1,$2,$3,'DISPOSED',0) ON CONFLICT DO NOTHING`, tc.BusinessID, branch, batch); err != nil {
				return err
			}
		}
		if err := audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "pharmacy.batch_action", EntityType: "batch", EntityID: batch.String(), Reason: req.Reason}); err != nil {
			return err
		}
		return idempotency.Complete(ctx, tx, tc.BusinessID, key, 200, []byte(`{"status":"ok"}`))
	})
	if err != nil {
		s.pharmacyError(w, r, err)
		return
	}
	if replay != nil {
		w.Header().Set("Idempotent-Replay", "true")
	}
	httpx.JSON(w, 200, map[string]string{"status": "ok"})
}

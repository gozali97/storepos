package apiserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/catalog"
	"github.com/storepos/storepos/internal/idempotency"
	"github.com/storepos/storepos/internal/inventory"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) importRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/catalog/import/template", s.guard(authz.PermProductManage, false, s.importTemplate))
	mux.Handle("POST /api/v1/catalog/import/preview", s.guard(authz.PermProductManage, true, s.importCSV))
	mux.Handle("POST /api/v1/catalog/import/commit", s.guard(authz.PermProductManage, true, s.importCSV))
}
func (s *Server) importTemplate(w http.ResponseWriter, r *http.Request) {
	var b bytes.Buffer
	cw := csv.NewWriter(&b)
	if err := cw.Write(catalog.CSVHeader); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="products-template.csv"`)
	_, _ = w.Write(b.Bytes())
}
func (s *Server) importCSV(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	branch, ok := branchOf(w, r, tc, r.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 5<<20))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("CSV maksimal 5 MiB"))
		return
	}
	rows, issues, err := catalog.ParseImportCSV(bytes.NewReader(raw))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation(err.Error()))
		return
	}
	preview := strings.HasSuffix(r.URL.Path, "/preview")
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if !preview && (key == "" || len(key) > 128) {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key wajib maksimal 128 karakter"))
		return
	}
	var replay *idempotency.Stored
	var body []byte
	sum := sha256.New()
	sum.Write([]byte(branch.String() + ":"))
	sum.Write(raw)
	hash := sum.Sum(nil)
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if !preview {
			st, e := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /catalog/import/commit", hash)
			replay = st
			if e != nil || st != nil {
				return e
			}
		}
		var valid bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM branches b JOIN businesses t ON t.id=b.business_id WHERE b.business_id=$1 AND b.id=$2 AND b.status='active' AND t.business_type='retail')`, tc.BusinessID, branch).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return errs.Validation("impor ini hanya untuk cabang retail aktif; apotek memerlukan batch/ED")
		}
		conflicts, err := catalog.ValidateImportConflicts(ctx, tx, tc.BusinessID, rows)
		if err != nil {
			return err
		}
		all := append(append([]catalog.ImportIssue{}, issues...), conflicts...)
		if preview {
			body, err = json.Marshal(map[string]any{"row_count": len(rows), "valid": len(all) == 0, "errors": all})
			return err
		}
		if len(all) > 0 {
			details := []errs.FieldError{}
			for _, i := range all {
				details = append(details, errs.FieldError{Field: i.Field, Code: "csv_invalid", Message: "Baris " + strconv.Itoa(i.Row) + ": " + i.Message})
			}
			return errs.Validation("CSV ditolak; tidak ada data disimpan", details...)
		}
		// One CSV row = one new product/default variant; never overwrite existing inventory.
		for _, x := range rows {
			p := &catalog.Product{Name: x.Product, TrackStock: x.Track}
			nv := catalog.NewVariant{Name: x.Variant, SKU: x.SKU, BaseUnit: x.Unit, PriceCents: x.Price, AllowFractional: x.Fractional}
			if x.Barcode != "" {
				nv.Barcodes = []string{x.Barcode}
			}
			if err := catalog.NewRepo(tx).CreateProduct(ctx, tc.BusinessID, p, []catalog.NewVariant{nv}); err != nil {
				if errors.Is(err, catalog.ErrSKUTaken) {
					return errs.Conflict("SKU berubah sejak pratinjau; unggah ulang")
				}
				var bc *catalog.BarcodeTakenError
				if errors.As(err, &bc) {
					return errs.Conflict(bc.Error())
				}
				return err
			}
			variant := p.Variants[0].ID
			if x.OpeningQty > 0 {
				if _, err := inventory.Apply(ctx, tx, inventory.Movement{BusinessID: tc.BusinessID, BranchID: branch, VariantID: variant, Kind: "OPENING_BALANCE", QtyMilli: x.OpeningQty, RefType: "csv_import", RefID: p.ID, UserID: tc.UserID, Reason: "opening_balance"}); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE inventory_balances SET average_cost_cents=$4 WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, tc.BusinessID, branch, variant, x.OpeningCost); err != nil {
					return err
				}
			}
		}
		if err := audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "catalog.csv_imported", EntityType: "catalog", EntityID: branch.String(), After: map[string]int{"rows": len(rows)}}); err != nil {
			return err
		}
		body, err = json.Marshal(map[string]int{"imported_rows": len(rows)})
		if err != nil {
			return err
		}
		return idempotency.Complete(ctx, tx, tc.BusinessID, key, http.StatusCreated, body)
	})
	if err != nil {
		if errors.Is(err, idempotency.ErrConflict) {
			httpx.WriteError(w, r, mapSalesError(err))
		} else {
			httpx.WriteError(w, r, err)
		}
		return
	}
	code := http.StatusCreated
	if preview {
		code = 200
	}
	if replay != nil {
		body = replay.Body
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_, _ = w.Write(body)
}

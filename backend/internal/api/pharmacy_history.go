package apiserver

import (
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/idkit"
	"net/http"
	"time"

	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) drugList(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	cursor, ok := pageCursor(w, r)
	if !ok {
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT p.id::text,p.name,d.classification,d.prescription_required,d.special_restrictions,p.version FROM pharmacy_drugs d JOIN products p ON p.business_id=d.business_id AND p.id=d.product_id WHERE d.business_id=$1 AND p.id>$2 ORDER BY p.id LIMIT 101`, tc.BusinessID, cursor)
	if readError(w, r, err, "obat") {
		return
	}
	defer rows.Close()
	type drug struct {
		ID             string `json:"product_id"`
		Name           string `json:"name"`
		Classification string `json:"classification"`
		Prescription   bool   `json:"prescription_required"`
		Restricted     bool   `json:"special_restrictions"`
		Version        int    `json:"product_version"`
	}
	out := []drug{}
	for rows.Next() {
		var x drug
		if readError(w, r, rows.Scan(&x.ID, &x.Name, &x.Classification, &x.Prescription, &x.Restricted, &x.Version), "obat") {
			return
		}
		out = append(out, x)
	}
	if readError(w, r, rows.Err(), "obat") {
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].ID
	}
	httpx.JSON(w, 200, map[string]any{"drugs": out, "next_cursor": next})
}
func (s *Server) batchReceiptHistory(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	branch, ok := branchOf(w, r, tc, r.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	cursor, ok := pageCursor(w, r)
	if !ok {
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT r.id::text,r.batch_id::text,b.batch_number,b.expiry_date::text,r.supplier_id::text,r.invoice_number,r.qty_milli::text,r.unit_cost_cents::text,r.status,r.created_at FROM pharmacy_batch_receipts r JOIN pharmacy_batches b ON b.business_id=r.business_id AND b.id=r.batch_id WHERE r.business_id=$1 AND r.branch_id=$2 AND r.id>$3 ORDER BY r.id LIMIT 101`, tc.BusinessID, branch, cursor)
	if readError(w, r, err, "penerimaan batch") {
		return
	}
	defer rows.Close()
	type receipt struct {
		ID       string    `json:"receipt_id"`
		Batch    string    `json:"batch_id"`
		Number   string    `json:"batch_number"`
		Expiry   string    `json:"expiry_date"`
		Supplier string    `json:"supplier_id"`
		Invoice  string    `json:"invoice_number"`
		Qty      string    `json:"qty_milli"`
		Cost     string    `json:"unit_cost_cents"`
		Status   string    `json:"status"`
		At       time.Time `json:"created_at"`
	}
	out := []receipt{}
	for rows.Next() {
		var x receipt
		if readError(w, r, rows.Scan(&x.ID, &x.Batch, &x.Number, &x.Expiry, &x.Supplier, &x.Invoice, &x.Qty, &x.Cost, &x.Status, &x.At), "penerimaan batch") {
			return
		}
		out = append(out, x)
	}
	if readError(w, r, rows.Err(), "penerimaan batch") {
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].ID
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, 200, map[string]any{"receipts": out, "next_cursor": next})
}

// batchTrace menelusuri satu batch: penerimaan, penjualan (snapshot alokasi) dan pergerakan (APT-011).
func (s *Server) batchTrace(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	branch, ok := branchOf(w, r, tc, r.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	batch, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("batch invalid"))
		return
	}
	var out struct {
		Receipts []map[string]any `json:"receipts"`
		Sales    []map[string]any `json:"sales"`
	}
	q := func(sql string, dst *[]map[string]any, cols ...string) bool {
		rows, err := s.pool.Underlying().Query(r.Context(), sql, tc.BusinessID, branch, batch)
		if err != nil {
			httpx.WriteError(w, r, err)
			return false
		}
		defer rows.Close()
		*dst = []map[string]any{}
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				httpx.WriteError(w, r, err)
				return false
			}
			m := map[string]any{}
			for i, c := range cols {
				m[c] = vals[i]
			}
			*dst = append(*dst, m)
		}
		if err := rows.Err(); err != nil {
			httpx.WriteError(w, r, err)
			return false
		}
		return true
	}
	if !q(`SELECT id::text,supplier_id::text,invoice_number,qty_milli::text,status,created_at FROM pharmacy_batch_receipts WHERE business_id=$1 AND branch_id=$2 AND batch_id=$3 ORDER BY created_at,id LIMIT 200`, &out.Receipts, "receipt_id", "supplier_id", "invoice_number", "qty_milli", "status", "created_at") {
		return
	}
	// Tanpa nama/kontak pelanggan; hanya nomor struk, status dan jumlah (privasi).
	if !q(`SELECT s.id::text,s.receipt_number,s.status,s.return_status,x.qty_milli::text,x.returned_milli::text,x.override_reason,s.completed_at FROM sale_line_batches x JOIN sales s ON s.business_id=x.business_id AND s.id=x.sale_id WHERE x.business_id=$1 AND x.branch_id=$2 AND x.batch_id=$3 ORDER BY s.completed_at,s.id LIMIT 200`, &out.Sales, "sale_id", "receipt_number", "status", "return_status", "qty_milli", "returned_milli", "override_reason", "completed_at") {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, 200, out)
}

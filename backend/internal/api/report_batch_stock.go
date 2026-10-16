package apiserver

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"strconv"
	"time"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/authz"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/reporting"
)

// reportBatchStock: laporan stok per batch apotek (CORE-039, APT-004/007). JSON atau CSV teraudit.
// shortcut: sinkron dengan batas 100.000 baris; belum lewat antrean ekspor latar seperti laporan lain.
func (s *Server) reportBatchStock(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	f, ok := s.reportFilter(w, r)
	if !ok {
		return
	}
	if f.UserID != nil {
		httpx.WriteError(w, r, errs.Validation("laporan stok batch tidak mendukung filter kasir"))
		return
	}
	q := r.URL.Query()
	filter := q.Get("filter")
	if filter == "" {
		filter = "all"
	}
	if filter != "all" && filter != "near_expiry" && filter != "expired" && filter != "quarantine" {
		httpx.WriteError(w, r, errs.Validation("filter harus all/near_expiry/expired/quarantine"))
		return
	}
	days := 90
	if q.Get("days") != "" {
		v, err := strconv.Atoi(q.Get("days"))
		if err != nil || v < 0 || v > 3650 {
			httpx.WriteError(w, r, errs.Validation("days 0..3650"))
			return
		}
		days = v
	}
	canViewCost := authz.Check(tc.Role, authz.PermProductViewCost) == authz.Allowed
	out, err := reporting.BatchStock(r.Context(), s.pool.Underlying(), f, canViewCost, filter, days)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(out) > 100000 {
		httpx.WriteError(w, r, errs.Conflict("laporan melebihi 100.000 baris; persempit filter"))
		return
	}
	label := "Estimasi — rata-rata tertimbang harga beli penerimaan batch, bukan HPP inti"
	if q.Get("format") != "csv" {
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, http.StatusOK, map[string]any{"timezone": f.Timezone, "balance_basis": "current", "generated_at": time.Now().UTC(), "expiry_warning_days": days, "cost_visible": canViewCost, "valuation_label": label, "rows": out})
		return
	}
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	header := []string{"batch_id", "variant_id", "sku", "product_name", "batch_number", "expiry_date", "status", "stored_status", "days_to_expiry", "qty_milli", "period_movement_milli", "period_movement_count", "last_supplier_name"}
	if canViewCost {
		header = append(header, "average_cost_cents", "estimated_value_cents", "valuation_label")
	}
	if err := cw.Write(header); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	nullable := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	for _, x := range out {
		row := []string{x.BatchID, x.VariantID, csvText(x.SKU), csvText(x.ProductName), csvText(x.BatchNumber), x.ExpiryDate, x.Status, x.StoredStatus, strconv.Itoa(x.DaysToExpiry), x.Balance, x.Movement, strconv.FormatInt(x.MovementCount, 10), csvText(nullable(x.LastSupplier))}
		if canViewCost {
			row = append(row, nullable(x.AverageCost), nullable(x.EstimatedValue), label)
		}
		if err := cw.Write(row); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := audit.Write(r.Context(), s.pool.Underlying(), audit.Entry{BusinessID: tc.BusinessID, BranchID: &f.BranchID, ActorUserID: tc.UserID, Action: "report.exported", EntityType: "batch_stock_report", EntityID: f.BranchID.String(), After: map[string]any{"from": q.Get("from"), "to": q.Get("to"), "filter": filter, "rows": len(out), "cost_visible": canViewCost}}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="batch-stock.csv"`)
	_, _ = w.Write(buf.Bytes())
}

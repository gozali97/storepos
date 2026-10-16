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

func (s *Server) reportStock(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	f, ok := s.reportFilter(w, r)
	if !ok {
		return
	}
	if f.UserID != nil {
		httpx.WriteError(w, r, errs.Validation("laporan saldo stok tidak mendukung filter kasir; gunakan kartu stok untuk pelaku pergerakan"))
		return
	}
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}
	if filter != "all" && filter != "below_minimum" && filter != "negative" {
		httpx.WriteError(w, r, errs.Validation("filter harus all/below_minimum/negative"))
		return
	}
	canViewCost := authz.Check(tc.Role, authz.PermProductViewCost) == authz.Allowed
	out, err := reporting.Stock(r.Context(), s.pool.Underlying(), f, canViewCost, filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(out) > 100000 {
		httpx.WriteError(w, r, errs.Conflict("laporan melebihi 100.000 baris; ekspor latar belum tersedia"))
		return
	}
	label := "Estimasi — akurat jika harga beli dicatat pada setiap penerimaan barang"
	if r.URL.Query().Get("format") != "csv" {
		httpx.JSON(w, http.StatusOK, map[string]any{"timezone": f.Timezone, "balance_basis": "current", "generated_at": time.Now().UTC(), "cost_visible": canViewCost, "valuation_label": label, "rows": out})
		return
	}
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	header := []string{"variant_id", "sku", "product_name", "base_unit", "qty_milli", "minimum_qty_milli", "below_minimum", "negative", "period_movement_milli", "period_movement_count"}
	if canViewCost {
		header = append(header, "average_cost_cents", "estimated_value_cents", "cost_unknown", "valuation_label")
	}
	header = append(header, "reserved_milli", "available_milli")
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
		row := []string{x.VariantID, csvText(x.SKU), csvText(x.ProductName), csvText(x.Unit), x.Balance, nullable(x.Minimum), strconv.FormatBool(x.BelowMinimum), strconv.FormatBool(x.Negative), x.Movement, strconv.FormatInt(x.MovementCount, 10)}
		if canViewCost {
			row = append(row, nullable(x.AverageCost), nullable(x.EstimatedValue), strconv.FormatBool(x.CostUnknown), label)
		}
		row = append(row, x.Reserved, x.Available)
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
	if err := audit.Write(r.Context(), s.pool.Underlying(), audit.Entry{BusinessID: tc.BusinessID, BranchID: &f.BranchID, ActorUserID: tc.UserID, Action: "report.exported", EntityType: "stock_report", EntityID: f.BranchID.String(), After: map[string]any{"from": r.URL.Query().Get("from"), "to": r.URL.Query().Get("to"), "filter": filter, "rows": len(out), "cost_visible": canViewCost}}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="stock.csv"`)
	_, _ = w.Write(buf.Bytes())
}

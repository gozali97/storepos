package apiserver

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"strconv"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/authz"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/reporting"
)

func (s *Server) reportProducts(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	f, ok := s.reportFilter(w, r)
	if !ok {
		return
	}
	group := r.URL.Query().Get("group_by")
	if group != "" && group != "product" && group != "category" {
		httpx.WriteError(w, r, errs.Validation("group_by harus product/category"))
		return
	}
	profit := authz.Check(tc.Role, authz.PermReportProfit) == authz.Allowed
	rows, err := reporting.Products(r.Context(), s.pool.Underlying(), f, profit)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(rows) > 100000 {
		httpx.WriteError(w, r, errs.Conflict("laporan melebihi 100.000 baris; ekspor latar belum tersedia"))
		return
	}
	missing := map[string]bool{}
	for _, p := range rows {
		if p.MissingCost {
			missing[p.ProductID] = true
		}
	}
	label := "Estimasi — akurat jika harga beli dicatat pada setiap penerimaan barang"
	categories, err := reporting.Categories(rows)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if r.URL.Query().Get("format") != "csv" {
		var data any = rows
		if group == "category" {
			data = categories
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"rows": data, "profit_visible": profit, "valuation_label": label, "products_without_cost": len(missing)})
		return
	}
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	header := []string{"product_id", "category_id", "product_name", "sold_base_qty_milli", "returned_base_qty_milli", "net_sales_cents", "incomplete_revenue"}
	if profit {
		header = append(header, "sold_cost_cents", "restock_cost_cents", "estimated_gross_profit_cents", "missing_cost", "valuation_label")
	}
	if group == "category" {
		header = []string{"category_id", "product_count", "net_sales_cents"}
		if profit {
			header = append(header, "sold_cost_cents", "restock_cost_cents", "estimated_gross_profit_cents", "products_without_cost", "valuation_label")
		}
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
	if group == "category" {
		for _, c := range categories {
			row := []string{nullable(c.CategoryID), strconv.Itoa(c.ProductCount), nullable(c.Net)}
			if profit {
				row = append(row, nullable(c.Cost), nullable(c.RestockCost), nullable(c.Profit), strconv.Itoa(c.ProductsWithoutCost), label)
			}
			if err := cw.Write(row); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
		}
	} else {
		for _, p := range rows {
			row := []string{p.ProductID, nullable(p.CategoryID), csvText(p.Name), p.Sold, p.Returned, nullable(p.Net), strconv.FormatBool(p.IncompleteRevenue)}
			if profit {
				row = append(row, nullable(p.Cost), nullable(p.RestockCost), nullable(p.Profit), strconv.FormatBool(p.MissingCost), label)
			}
			if err := cw.Write(row); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := audit.Write(r.Context(), s.pool.Underlying(), audit.Entry{BusinessID: tc.BusinessID, BranchID: &f.BranchID, ActorUserID: tc.UserID, Action: "report.exported", EntityType: "products_report", EntityID: f.BranchID.String(), After: map[string]any{"rows": len(rows), "profit_visible": profit, "group_by": group}}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="products.csv"`)
	_, _ = w.Write(buf.Bytes())
}

package apiserver

import (
	"bytes"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/reporting"
)

func (s *Server) reportFilter(w http.ResponseWriter, r *http.Request) (reporting.PaymentFilter, bool) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	var f reporting.PaymentFilter
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return f, false
	}
	var tz string
	err := s.pool.Underlying().QueryRow(r.Context(), `SELECT timezone FROM branches WHERE business_id=$1 AND id=$2`, tc.BusinessID, branch).Scan(&tz)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, errs.NotFound("cabang"))
		return f, false
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return f, false
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		httpx.WriteError(w, r, errs.Internal("zona waktu cabang tidak valid"))
		return f, false
	}
	from, e1 := time.ParseInLocation("2006-01-02", q.Get("from"), loc)
	to, e2 := time.ParseInLocation("2006-01-02", q.Get("to"), loc)
	if e1 != nil || e2 != nil || from.After(to) || to.Sub(from) > 366*24*time.Hour {
		httpx.WriteError(w, r, errs.Validation("periode wajib YYYY-MM-DD, maksimal 366 hari"))
		return f, false
	}
	f = reporting.PaymentFilter{BusinessID: tc.BusinessID, BranchID: branch, From: from, Until: to.AddDate(0, 0, 1), Timezone: tz}
	if q.Get("user_id") != "" {
		id, err := idkit.Parse(q.Get("user_id"))
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("user_id tidak valid"))
			return f, false
		}
		f.UserID = &id
	}
	format := q.Get("format")
	if format != "" && format != "json" && format != "csv" {
		httpx.WriteError(w, r, errs.Validation("format harus json/csv"))
		return f, false
	}
	return f, true
}
func (s *Server) reportSales(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	f, ok := s.reportFilter(w, r)
	if !ok {
		return
	}
	rows, err := reporting.Sales(r.Context(), s.pool.Underlying(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(rows) > 100000 {
		httpx.WriteError(w, r, errs.Conflict("laporan melebihi 100.000 baris; ekspor latar belum tersedia"))
		return
	}
	if r.URL.Query().Get("format") != "csv" {
		httpx.JSON(w, http.StatusOK, map[string]any{"timezone": f.Timezone, "rows": rows})
		return
	}
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	header := []string{"date", "hour", "user_id", "gross_cents", "discount_cents", "returns_cents", "net_cents", "tax_cents", "service_charge_cents", "delivery_fee_cents", "net_bill_cents", "cash_rounding_cents", "sale_count", "average_sale_cents", "incomplete_return_count"}
	if err := cw.Write(header); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	nullable := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	for _, x := range rows {
		if err := cw.Write([]string{x.Date, strconv.Itoa(x.Hour), x.UserID, x.Gross, x.Discount, nullable(x.Returns), nullable(x.Net), nullable(x.Tax), nullable(x.ServiceCharge), nullable(x.DeliveryFee), nullable(x.BillNet), nullable(x.Rounding), strconv.FormatInt(x.SaleCount, 10), x.Average, strconv.FormatInt(x.IncompleteReturns, 10)}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := audit.Write(r.Context(), s.pool.Underlying(), audit.Entry{BusinessID: tc.BusinessID, BranchID: &f.BranchID, ActorUserID: tc.UserID, Action: "report.exported", EntityType: "sales_report", EntityID: f.BranchID.String(), After: map[string]any{"from": r.URL.Query().Get("from"), "to": r.URL.Query().Get("to"), "rows": len(rows)}}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="sales.csv"`)
	_, _ = w.Write(buf.Bytes())
}

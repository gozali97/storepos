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

func (s *Server) reportRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/reports/payments", s.guard(authz.PermReportSales, false, s.reportPayments))
	mux.Handle("GET /api/v1/reports/sales", s.guard(authz.PermReportSales, false, s.reportSales))
	mux.Handle("GET /api/v1/reports/corrections", s.guard(authz.PermReportSales, false, s.reportCorrections))
	mux.Handle("GET /api/v1/reports/stock", s.guard(authz.PermProductView, false, s.reportStock))
	mux.Handle("GET /api/v1/reports/products", s.guard(authz.PermReportSales, false, s.reportProducts))
}
func (s *Server) reportPayments(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	f, ok := s.reportFilter(w, r)
	if !ok {
		return
	}
	branch, tz, format := f.BranchID, f.Timezone, q.Get("format")
	out, err := reporting.Payments(r.Context(), s.pool.Underlying(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(out) > 100000 {
		httpx.WriteError(w, r, errs.Conflict("laporan melebihi 100.000 baris; persempit periode. Ekspor latar belum tersedia"))
		return
	}
	if format != "csv" {
		httpx.JSON(w, http.StatusOK, map[string]any{"timezone": tz, "rows": out})
		return
	}
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	if err := cw.Write([]string{"date", "hour", "user_id", "method", "verification", "received_cents", "refunded_cents", "net_cents"}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for _, p := range out {
		if err := cw.Write([]string{p.Date, strconv.Itoa(p.Hour), p.UserID, p.Method, p.Verification, p.Received, p.Refunded, p.Net}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := audit.Write(r.Context(), s.pool.Underlying(), audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "report.exported", EntityType: "payments_report", EntityID: branch.String(), After: map[string]any{"from": q.Get("from"), "to": q.Get("to"), "rows": len(out)}}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="payments.csv"`)
	_, _ = w.Write(buf.Bytes())
}

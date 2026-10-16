package apiserver

import (
	"net/http"
	"time"

	"github.com/storepos/storepos/internal/platform/authz"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/reporting"
)

func (s *Server) reconciliationRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/reconciliation/findings", s.guard(authz.PermAuditView, false, s.reconciliationFindings))
}
func (s *Server) reconciliationFindings(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	date := r.URL.Query().Get("date")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		httpx.WriteError(w, r, errs.Validation("date wajib YYYY-MM-DD (UTC)"))
		return
	}
	var started, completed *time.Time
	var tenantCount int64
	if err := s.pool.Underlying().QueryRow(r.Context(), `SELECT
 (SELECT started_at FROM reconciliation_runs WHERE run_date=$1),
 (SELECT completed_at FROM reconciliation_runs WHERE run_date=$1),
 (SELECT count(*) FROM reconciliation_findings WHERE run_date=$1 AND business_id=$2)`, date, tc.BusinessID).Scan(&started, &completed, &tenantCount); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT business_id::text,kind,entity_key,expected_value::text,actual_value::text FROM reconciliation_findings WHERE business_id=$1 AND run_date=$2 ORDER BY kind,entity_key LIMIT 1001`, tc.BusinessID, date)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	out := []reporting.Finding{}
	for rows.Next() {
		var f reporting.Finding
		if err := rows.Scan(&f.BusinessID, &f.Kind, &f.EntityKey, &f.Expected, &f.Actual); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Explicit truncation avoids treating an incomplete list as a clean reconciliation.
	truncated := len(out) > 1000
	if truncated {
		out = out[:1000]
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"date": date, "findings": out, "truncated": truncated, "run_started_at": started, "run_completed_at": completed, "tenant_finding_count": tenantCount, "date_basis": "UTC"})
}

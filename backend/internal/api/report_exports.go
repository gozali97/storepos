package apiserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/platform/authz"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/reporting"
)

func (s *Server) exportRoutes(mux *http.ServeMux) {
	for _, kind := range []string{"payments", "sales", "corrections", "stock", "products"} {
		perm := authz.PermReportSales
		if kind == "stock" {
			perm = authz.PermProductView
		}
		mux.Handle("POST /api/v1/reports/"+kind+"/exports", s.guard(perm, false, s.createExport))
	}
	// ReadOnly report grants can export their readable reports; creation itself changes no business data.
	mux.Handle("GET /api/v1/report-exports/{id}", CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(http.HandlerFunc(s.getExport)))))
}
func (s *Server) createExport(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	f, ok := s.reportFilter(w, r)
	if !ok {
		return
	}
	var kind string
	// Route pattern is server-controlled, not a client-supplied filename.
	switch r.Pattern {
	case "POST /api/v1/reports/payments/exports":
		kind = "payments"
	case "POST /api/v1/reports/sales/exports":
		kind = "sales"
	case "POST /api/v1/reports/corrections/exports":
		kind = "corrections"
	case "POST /api/v1/reports/stock/exports":
		kind = "stock"
	case "POST /api/v1/reports/products/exports":
		kind = "products"
	}
	group := r.URL.Query().Get("group_by")
	if group != "" && group != "product" && group != "category" {
		httpx.WriteError(w, r, errs.Validation("group_by tidak valid"))
		return
	}
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}
	if filter != "all" && filter != "below_minimum" && filter != "negative" {
		httpx.WriteError(w, r, errs.Validation("filter stok tidak valid"))
		return
	}
	if kind == "stock" && f.UserID != nil {
		httpx.WriteError(w, r, errs.Validation("laporan stok tidak mendukung filter kasir"))
		return
	}
	cost := kind == "stock" && authz.Check(tc.Role, authz.PermProductViewCost) == authz.Allowed
	profit := kind == "products" && authz.Check(tc.Role, authz.PermReportProfit) == authz.Allowed
	raw, err := json.Marshal(reporting.ExportParameters{Filter: f, Group: group, StockFilter: filter})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	id := idkit.New()
	_, err = s.pool.Underlying().Exec(r.Context(), `INSERT INTO report_exports(id,business_id,branch_id,requested_by,report_kind,parameters,cost_visible,profit_visible) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, tc.BusinessID, f.BranchID, tc.UserID, kind, raw, cost, profit)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]string{"export_id": id.String(), "status": "pending"})
}
func (s *Server) getExport(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("export_id tidak valid"))
		return
	}
	var branch, user idkit.ID
	var kind, status string
	var cost, profit bool
	var count *int64
	var code *string
	var expires time.Time
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT branch_id,requested_by,report_kind,status,cost_visible,profit_visible,row_count,error_code,expires_at FROM report_exports WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&branch, &user, &kind, &status, &cost, &profit, &count, &code, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, errs.NotFound("ekspor"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if user != tc.UserID {
		httpx.WriteError(w, r, errs.NotFound("ekspor"))
		return
	}
	if err := reporting.ExportAccess(r.Context(), s.pool.Underlying(), tc.BusinessID, branch, user, kind, cost, profit); err != nil {
		if errors.Is(err, reporting.ErrExportPermission) {
			httpx.WriteError(w, r, errs.PermissionDenied("izin ekspor sudah dicabut"))
		} else {
			httpx.WriteError(w, r, err)
		}
		return
	}
	if time.Now().After(expires) || status == "expired" {
		httpx.WriteError(w, r, errs.New(errs.CodeNotFound, "hasil ekspor kedaluwarsa", http.StatusGone))
		return
	}
	if r.URL.Query().Get("download") != "true" {
		httpx.JSON(w, http.StatusOK, map[string]any{"export_id": id.String(), "status": status, "row_count": count, "error_code": code, "expires_at": expires})
		return
	}
	if status != "completed" {
		httpx.WriteError(w, r, errs.Conflict("hasil ekspor belum tersedia"))
		return
	}
	var artifact []byte
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT artifact FROM report_exports WHERE business_id=$1 AND id=$2 AND status='completed' AND expires_at>now()`, tc.BusinessID, id).Scan(&artifact)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, errs.NotFound("hasil ekspor"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="report.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(artifact)
}

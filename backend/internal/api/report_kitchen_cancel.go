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
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/reporting"
)

func (s *Server) cancellationReportRoutes(mux *http.ServeMux) {
	h := http.HandlerFunc(s.reportKitchenCancellations)
	mux.Handle("GET /api/v1/restaurant/reports/cancellations", CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(RequireModule("restaurant")(RequirePermission(func(tc tenant.Context) error { return authz.RequireRead(tc, authz.PermReportSales, idkit.ID{}) })(h))))))
}
func (s *Server) reportKitchenCancellations(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	f, ok := s.reportFilter(w, r)
	if !ok {
		return
	}
	out, err := reporting.KitchenCancellations(r.Context(), s.pool.Underlying(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(out) > 100000 {
		httpx.WriteError(w, r, errs.Conflict("laporan melebihi 100.000 baris; ekspor latar pembatalan belum tersedia"))
		return
	}
	label := "Nilai menu berdasarkan harga jual snapshot, bukan nilai kerugian/HPP"
	if r.URL.Query().Get("format") != "csv" {
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, http.StatusOK, map[string]any{"rows": out, "timezone": f.Timezone, "value_label": label})
		return
	}
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	if err := cw.Write([]string{"line_id", "order_id", "order_type", "product_name", "variant_name", "qty_milli", "base_unit", "modifiers", "cancelled_from_status", "potential_waste", "reason", "actor_user_id", "approved_by_user_id", "cancelled_at", "local_date", "menu_value_cents", "value_label"}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for _, x := range out {
		approver := ""
		if x.Approver != nil {
			approver = *x.Approver
		}
		if err := cw.Write([]string{x.LineID, x.OrderID, x.OrderType, csvText(x.ProductName), csvText(x.VariantName), x.Qty, csvText(x.Unit), csvText(string(x.ModifierSnapshot)), x.CancelledFrom, strconv.FormatBool(x.PotentialWaste), csvText(x.Reason), x.Actor, approver, x.At.Format(time.RFC3339Nano), x.LocalDate, x.MenuValue, label}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := audit.Write(r.Context(), s.pool.Underlying(), audit.Entry{BusinessID: tc.BusinessID, BranchID: &f.BranchID, ActorUserID: tc.UserID, Action: "report.exported", EntityType: "kitchen_cancellations_report", EntityID: f.BranchID.String(), After: map[string]any{"rows": len(out), "from": r.URL.Query().Get("from"), "to": r.URL.Query().Get("to")}}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="kitchen-cancellations.csv"`)
	_, _ = w.Write(buf.Bytes())
}

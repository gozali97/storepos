package apiserver

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"strings"
	"time"

	"github.com/storepos/storepos/internal/audit"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/reporting"
)

// CSV cells are text data, never spreadsheet formulas supplied in a reason/receipt.
func csvText(s string) string {
	trimmed := strings.TrimLeft(s, " \t\r\n")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + s
	}
	if strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") || strings.HasPrefix(s, "\n") {
		return "'" + s
	}
	return s
}

func (s *Server) reportCorrections(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	f, ok := s.reportFilter(w, r)
	if !ok {
		return
	}
	out, err := reporting.Corrections(r.Context(), s.pool.Underlying(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(out) > 100000 {
		httpx.WriteError(w, r, errs.Conflict("laporan melebihi 100.000 baris; ekspor latar belum tersedia"))
		return
	}
	if r.URL.Query().Get("format") != "csv" {
		httpx.JSON(w, http.StatusOK, map[string]any{"timezone": f.Timezone, "rows": out})
		return
	}
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	if err := cw.Write([]string{"kind", "document_id", "sale_id", "receipt_number", "occurred_at", "local_date", "actor_user_id", "approved_by_user_id", "reason", "line_discount_cents", "transaction_discount_cents", "amount_cents"}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for _, x := range out {
		approver := ""
		if x.Approver != nil {
			approver = *x.Approver
		}
		if err := cw.Write([]string{x.Kind, x.DocumentID, x.SaleID, csvText(x.Receipt), x.At.Format(time.RFC3339Nano), x.LocalDate, x.Actor, approver, csvText(x.Reason), x.LineDiscount, x.TransactionDiscount, x.Amount}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := audit.Write(r.Context(), s.pool.Underlying(), audit.Entry{BusinessID: tc.BusinessID, BranchID: &f.BranchID, ActorUserID: tc.UserID, Action: "report.exported", EntityType: "corrections_report", EntityID: f.BranchID.String(), After: map[string]any{"from": r.URL.Query().Get("from"), "to": r.URL.Query().Get("to"), "rows": len(out)}}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="corrections.csv"`)
	_, _ = w.Write(buf.Bytes())
}

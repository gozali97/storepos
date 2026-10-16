package apiserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/org"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) paymentMethodRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/settings/payment-methods", s.guard(authz.PermProductView, false, s.getPaymentMethods))
	mux.Handle("PUT /api/v1/settings/payment-methods", s.guard(authz.PermBusinessManage, true, s.putPaymentMethods))
}
func (s *Server) getPaymentMethods(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	items, version, err := org.LoadPaymentMethods(r.Context(), s.pool.Underlying(), tc.BusinessID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, 200, map[string]any{"methods": items, "version": version})
}
func (s *Server) putPaymentMethods(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	version, err := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if err != nil || version < 1 {
		httpx.WriteError(w, r, errs.Validation("If-Match wajib versi positif"))
		return
	}
	var req struct {
		Methods []org.PaymentMethod `json:"methods"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for i := range req.Methods {
		req.Methods[i].Name = strings.TrimSpace(req.Methods[i].Name)
	}
	if err := org.ValidatePaymentMethods(req.Methods); err != nil {
		httpx.WriteError(w, r, errs.Validation(err.Error()))
		return
	}
	raw, err := json.Marshal(req.Methods)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var current int
		if err := tx.QueryRow(ctx, `SELECT payment_methods_version FROM businesses WHERE id=$1 FOR UPDATE`, tc.BusinessID).Scan(&current); err != nil {
			return err
		}
		if current != version {
			return errs.Conflict("metode pembayaran berubah; muat ulang pengaturan")
		}
		if _, err := tx.Exec(ctx, `UPDATE businesses SET settings=jsonb_set(settings,'{payment_methods}',$2::jsonb,true),payment_methods_version=payment_methods_version+1 WHERE id=$1`, tc.BusinessID, raw); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, ActorUserID: tc.UserID, Action: "payment_methods.updated", EntityType: "business", EntityID: tc.BusinessID.String(), After: req.Methods})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"methods": req.Methods, "version": version + 1})
}

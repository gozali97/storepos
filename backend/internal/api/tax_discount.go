package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/pricing"
	"github.com/storepos/storepos/internal/sales"
)

// Value nominal dalam sen, persen dalam basis points; selalu string integer.
type discountReq struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

func (d discountReq) parse() (pricing.Discount, error) {
	if (d.Kind == "" || d.Kind == "none") && (d.Value == "" || d.Value == "0") {
		return pricing.Discount{}, nil
	}
	v, err := strconv.ParseInt(d.Value, 10, 64)
	if err != nil || v <= 0 {
		return pricing.Discount{}, sales.ErrInvalidDiscount
	}
	switch d.Kind {
	case "amount":
		return pricing.Discount{Kind: pricing.DiscountAmount, Value: v}, nil
	case "percent":
		if v <= 10000 {
			return pricing.Discount{Kind: pricing.DiscountPercent, Value: v}, nil
		}
	}
	return pricing.Discount{}, sales.ErrInvalidDiscount
}

func (s *Server) taxRoutes(mux *http.ServeMux) {
	mux.Handle("PUT /api/v1/settings/tax", s.guard(authz.PermBusinessManage, true, s.setTax))
}

func (s *Server) setTax(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		Name    string `json:"name"`
		RateBps int    `json:"rate_bps"`
		Mode    string `json:"mode"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Kunci tenant mencegah dua Pemilik mengganti aturan aktif bersamaan.
	err := s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT id FROM businesses WHERE id=$1 FOR UPDATE`, tc.BusinessID); err != nil {
			return err
		}
		return sales.SetTaxRule(ctx, tx, tc.BusinessID, tc.UserID, req.Name, req.RateBps, req.Mode)
	})
	if errors.Is(err, sales.ErrInvalidTax) {
		httpx.WriteError(w, r, errs.Validation(err.Error()))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, errs.Internal("gagal mengatur pajak").Wrap(err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

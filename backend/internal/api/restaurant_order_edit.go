package apiserver

import (
	"context"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) editOrderHeader(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Note    string `json:"note"`
		Channel string `json:"channel"`
		Fee     string `json:"delivery_fee_cents"`
		Name    string `json:"contact_name"`
		Phone   string `json:"contact_phone"`
		Queue   string `json:"queue_label"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Fee == "" {
		req.Fee = "0"
	}
	fee, err := money.ParseCents(req.Fee)
	if err != nil || fee < 0 || fee > 10000000000 || utf8.RuneCountInString(req.Note) > 200 || utf8.RuneCountInString(req.Channel) > 100 || utf8.RuneCountInString(req.Name) > 100 || len(req.Phone) > 32 || utf8.RuneCountInString(req.Queue) > 40 {
		httpx.WriteError(w, r, errs.Validation("catatan/kanal/ongkir/kontak/antrean tidak valid"))
		return
	}
	for _, c := range req.Phone {
		if !(c >= '0' && c <= '9') && c != '+' && c != ' ' && c != '-' && c != '(' && c != ')' {
			httpx.WriteError(w, r, errs.Validation("nomor telepon tidak valid"))
			return
		}
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		branch, err := lockRestaurantOrder(ctx, tx, tc, id, v)
		if err != nil {
			return err
		}
		var typ string
		if err := tx.QueryRow(ctx, `SELECT order_type FROM restaurant_orders WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&typ); err != nil {
			return err
		}
		if typ != "DELIVERY_MANUAL" && (fee != 0 || req.Phone != "" || req.Channel != "") {
			return errs.Validation("kanal/ongkir/telepon hanya untuk delivery manual")
		}
		if typ == "DINE_IN" && (req.Name != "" || req.Queue != "") {
			return errs.Validation("nama/antrean hanya untuk takeaway atau delivery")
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_orders SET note=$3,channel=$4,delivery_fee_cents=$5,contact_name=$6,contact_phone=$7,queue_label=$8,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Note, strings.TrimSpace(req.Channel), int64(fee), strings.TrimSpace(req.Name), strings.TrimSpace(req.Phone), strings.TrimSpace(req.Queue)); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.order_header_updated", EntityType: "restaurant_order", EntityID: id.String(), After: map[string]int{"version": v + 1}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"order_id": id.String(), "version": v + 1})
}

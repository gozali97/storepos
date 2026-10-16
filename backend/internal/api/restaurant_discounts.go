package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/pricing"
)

func storedDiscount(kind string, value int64) (pricing.Discount, error) {
	return (discountReq{Kind: kind, Value: strconv.FormatInt(value, 10)}).parse()
}
func (s *Server) discountOrderRoutes(mux *http.ServeMux) {
	mux.Handle("PUT /api/v1/restaurant/orders/{id}/discount", s.orderGuard(s.setOrderDiscount))
	mux.Handle("PUT /api/v1/restaurant/orders/{id}/lines/{line}/discount", s.orderGuard(s.setOrderDiscount))
	mux.Handle("GET /api/v1/restaurant/orders/{id}/discounts", s.orderGuard(s.getOrderDiscounts))
}
func (s *Server) setOrderDiscount(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if authz.Check(tc.Role, authz.PermDiscountWithinLimit) != authz.Allowed {
		httpx.WriteError(w, r, errs.PermissionDenied(""))
		return
	}
	var req struct {
		Discount discountReq `json:"discount"`
		Reason   string      `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	d, err := req.Discount.parse()
	if err != nil || utf8.RuneCountInString(req.Reason) > 200 {
		httpx.WriteError(w, r, errs.Validation("diskon/alasan tidak valid"))
		return
	}
	kind := req.Discount.Kind
	if d.Kind == pricing.DiscountNone {
		kind = "none"
	}
	var line *idkit.ID
	if raw := r.PathValue("line"); raw != "" {
		x, err := idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("line_id tidak valid"))
			return
		}
		line = &x
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		branch, err := lockRestaurantOrder(ctx, tx, tc, id, v)
		if err != nil {
			return err
		}
		if line != nil {
			var status string
			err := tx.QueryRow(ctx, `SELECT kitchen_status FROM restaurant_order_lines WHERE business_id=$1 AND order_id=$2 AND id=$3`, tc.BusinessID, id, *line).Scan(&status)
			if errors.Is(err, pgx.ErrNoRows) {
				return errs.NotFound("item")
			}
			if err != nil {
				return err
			}
			if status == "CANCELLED" {
				return errs.Conflict("item dibatalkan")
			}
			if _, err := tx.Exec(ctx, `UPDATE restaurant_order_lines SET discount_kind=$4,discount_value=$5 WHERE business_id=$1 AND order_id=$2 AND id=$3`, tc.BusinessID, id, *line, kind, d.Value); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(ctx, `UPDATE restaurant_orders SET discount_kind=$3,discount_value=$4 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, kind, d.Value); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_orders SET discount_reason=$3,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Reason); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.discount_drafted", EntityType: "restaurant_order", EntityID: id.String(), After: map[string]any{"kind": kind, "value": d.Value, "line_id": line}, Reason: req.Reason})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"order_id": id.String(), "version": v + 1})
}
func (s *Server) getOrderDiscounts(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("id tidak valid"))
		return
	}
	var out map[string]any
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		var kind, reason string
		var value int64
		var version int
		err := tx.QueryRow(ctx, `SELECT branch_id,discount_kind,discount_value,discount_reason,version FROM restaurant_orders WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, id).Scan(&branch, &kind, &value, &reason, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("pesanan")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		rows, err := tx.Query(ctx, `SELECT id::text,discount_kind,discount_value::text FROM restaurant_order_lines WHERE business_id=$1 AND order_id=$2 AND kitchen_status<>'CANCELLED' ORDER BY id`, tc.BusinessID, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		type item struct {
			ID    string `json:"line_id"`
			Kind  string `json:"kind"`
			Value string `json:"value"`
		}
		items := []item{}
		for rows.Next() {
			var i item
			if err := rows.Scan(&i.ID, &i.Kind, &i.Value); err != nil {
				return err
			}
			items = append(items, i)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		out = map[string]any{"discount": discountReq{Kind: kind, Value: strconv.FormatInt(value, 10)}, "reason": reason, "version": version, "lines": items, "approval_checked_at_checkout": true}
		return nil
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, 200, out)
}

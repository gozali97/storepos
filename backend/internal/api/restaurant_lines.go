package apiserver

import (
	"context"
	"errors"
	"net/http"
	"unicode/utf8"

	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/restaurant"
	"math"
)

// Edit operations are explicit so new lines never replace existing kitchen/history IDs.
func (s *Server) orderLineRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/restaurant/orders/{id}/lines", s.orderGuard(s.addOrderLine))
	mux.Handle("PUT /api/v1/restaurant/orders/{id}/lines/{line}", s.orderGuard(s.editOrderLine))
	mux.Handle("DELETE /api/v1/restaurant/orders/{id}/lines/{line}", s.orderGuard(s.deleteOrderLine))
	mux.Handle("GET /api/v1/restaurant/orders/{id}/lines", s.orderGuard(s.getOrderLines))
}
func lockRestaurantOrder(ctx context.Context, tx database.Tx, tc tenant.Context, id idkit.ID, version int) (idkit.ID, error) {
	var branch idkit.ID
	var status string
	var current int
	err := tx.QueryRow(ctx, `SELECT branch_id,status,version FROM restaurant_orders WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&branch, &status, &current)
	if errors.Is(err, pgx.ErrNoRows) {
		return branch, errs.NotFound("pesanan")
	}
	if err != nil {
		return branch, err
	}
	if !tc.HasBranch(branch) {
		return branch, errs.PermissionDenied("")
	}
	if status != "open" {
		return branch, errs.Conflict("pesanan tidak terbuka")
	}
	if current != version {
		return branch, errs.New(errs.CodeOptimisticLockFailure, "pesanan diubah pengguna lain; muat ulang", http.StatusConflict)
	}
	return branch, nil
}
func (s *Server) addOrderLine(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	order, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Modifiers []string `json:"modifier_option_ids"`
		Variant   string   `json:"variant_id"`
		Qty       string   `json:"qty_milli"`
		Note      string   `json:"note"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	variant, e1 := idkit.Parse(req.Variant)
	qty, e2 := money.ParseMilli(req.Qty)
	if e1 != nil || e2 != nil || qty <= 0 || utf8.RuneCountInString(req.Note) > 140 {
		httpx.WriteError(w, r, errs.Validation("varian/qty/catatan maksimal 140 karakter tidak valid"))
		return
	}
	selected := []idkit.ID{}
	for _, raw := range req.Modifiers {
		id, err := idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("modifier_option_id tidak valid"))
			return
		}
		selected = append(selected, id)
	}
	id := idkit.New()
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		branch, err := lockRestaurantOrder(ctx, tx, tc, order, v)
		if err != nil {
			return err
		}
		var name, varname, unit string
		var price int64
		var fractional bool
		var product idkit.ID
		var category *idkit.ID
		err = tx.QueryRow(ctx, `SELECT p.name,v.name,v.base_unit,v.price_cents,v.allow_fractional,p.id,p.category_id FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id WHERE v.business_id=$1 AND v.id=$2 AND v.status='active' AND p.status='active'`, tc.BusinessID, variant).Scan(&name, &varname, &unit, &price, &fractional, &product, &category)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.Validation("menu tidak aktif atau bukan milik tenant")
		}
		if err != nil {
			return err
		}
		soldOut, err := restaurant.SoldOut(ctx, tx, tc.BusinessID, branch, variant)
		if err != nil {
			return err
		}
		if soldOut {
			return errs.Conflict("menu ditandai Habis")
		}
		if !fractional && qty%1000 != 0 {
			return errs.Validation("menu tidak mengizinkan pecahan")
		}
		mods, delta, err := restaurant.SelectModifiers(ctx, tx, tc.BusinessID, product, selected)
		if errors.Is(err, restaurant.ErrModifierSelection) {
			return errs.Validation(err.Error())
		}
		if err != nil {
			return err
		}
		if price > math.MaxInt64-delta {
			return errs.Validation("harga modifier melampaui batas")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO restaurant_order_lines(id,business_id,order_id,variant_id,qty_milli,unit_price_cents,product_name,variant_name,base_unit,note,modifiers,modifier_delta_cents,category_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, id, tc.BusinessID, order, variant, int64(qty), price, name, varname, unit, req.Note, mods, delta, category); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_orders SET version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, order); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.line_added", EntityType: "order_line", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"line_id": id.String(), "version": v + 1})
}
func (s *Server) editOrderLine(w http.ResponseWriter, r *http.Request) {
	s.changeOrderLine(w, r, false)
}
func (s *Server) deleteOrderLine(w http.ResponseWriter, r *http.Request) {
	s.changeOrderLine(w, r, true)
}
func (s *Server) changeOrderLine(w http.ResponseWriter, r *http.Request, remove bool) {
	tc := tenant.MustFromContext(r.Context())
	order, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	line, err := idkit.Parse(r.PathValue("line"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("line_id tidak valid"))
		return
	}
	var qty money.Milli
	var req struct {
		Qty  string `json:"qty_milli"`
		Note string `json:"note"`
	}
	if !remove {
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		qty, err = money.ParseMilli(req.Qty)
		if err != nil || qty <= 0 || utf8.RuneCountInString(req.Note) > 140 {
			httpx.WriteError(w, r, errs.Validation("qty/catatan tidak valid"))
			return
		}
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		branch, err := lockRestaurantOrder(ctx, tx, tc, order, v)
		if err != nil {
			return err
		}
		var status string
		var fractional bool
		var variant idkit.ID
		var oldQty int64
		err = tx.QueryRow(ctx, `SELECT l.kitchen_status,v.allow_fractional,l.variant_id,l.qty_milli FROM restaurant_order_lines l JOIN variants v ON v.business_id=l.business_id AND v.id=l.variant_id WHERE l.business_id=$1 AND l.order_id=$2 AND l.id=$3 FOR UPDATE OF l`, tc.BusinessID, order, line).Scan(&status, &fractional, &variant, &oldQty)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("baris pesanan")
		}
		if err != nil {
			return err
		}
		if status != "NOT_SENT" {
			return errs.Conflict("item sudah dikirim; gunakan pembatalan berotorisasi")
		}
		action := "restaurant.line_updated"
		if remove {
			if _, err := tx.Exec(ctx, `DELETE FROM restaurant_order_lines WHERE business_id=$1 AND order_id=$2 AND id=$3`, tc.BusinessID, order, line); err != nil {
				return err
			}
			action = "restaurant.line_removed"
		} else {
			if int64(qty) > oldQty {
				soldOut, err := restaurant.SoldOut(ctx, tx, tc.BusinessID, branch, variant)
				if err != nil {
					return err
				}
				if soldOut {
					return errs.Conflict("menu ditandai Habis; jumlah tidak dapat ditambah")
				}
			}
			if !fractional && qty%1000 != 0 {
				return errs.Validation("menu tidak mengizinkan pecahan")
			}
			if _, err := tx.Exec(ctx, `UPDATE restaurant_order_lines SET qty_milli=$4,note=$5 WHERE business_id=$1 AND order_id=$2 AND id=$3`, tc.BusinessID, order, line, int64(qty), req.Note); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_orders SET version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, order); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: action, EntityType: "order_line", EntityID: line.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"order_id": order.String(), "version": v + 1})
}
func (s *Server) getOrderLines(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("order_id tidak valid"))
		return
	}
	var out map[string]any
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		var version int
		err := tx.QueryRow(ctx, `SELECT branch_id,version FROM restaurant_orders WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, id).Scan(&branch, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("pesanan")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		rows, err := tx.Query(ctx, `SELECT id::text,variant_id::text,qty_milli::text,unit_price_cents::text,product_name,variant_name,base_unit,note,kitchen_status,modifiers,modifier_delta_cents::text,(unit_price_cents+modifier_delta_cents)::text FROM restaurant_order_lines WHERE business_id=$1 AND order_id=$2 ORDER BY created_at,id`, tc.BusinessID, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		type line struct {
			ID             string          `json:"line_id"`
			Variant        string          `json:"variant_id"`
			Qty            string          `json:"qty_milli"`
			Price          string          `json:"unit_price_cents"`
			Name           string          `json:"product_name"`
			VariantName    string          `json:"variant_name"`
			Unit           string          `json:"base_unit"`
			Note           string          `json:"note"`
			Status         string          `json:"kitchen_status"`
			Modifiers      json.RawMessage `json:"modifiers"`
			Delta          string          `json:"modifier_delta_cents"`
			EffectivePrice string          `json:"effective_unit_price_cents"`
		}
		lines := []line{}
		for rows.Next() {
			var l line
			if err := rows.Scan(&l.ID, &l.Variant, &l.Qty, &l.Price, &l.Name, &l.VariantName, &l.Unit, &l.Note, &l.Status, &l.Modifiers, &l.Delta, &l.EffectivePrice); err != nil {
				return err
			}
			lines = append(lines, l)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		out = map[string]any{"order_id": id.String(), "version": version, "lines": lines}
		return nil
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, out)
}

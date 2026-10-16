package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) kitchenOperateGuard(h http.HandlerFunc) http.Handler {
	check := func(tc tenant.Context) error {
		if authz.Check(tc.Role, authz.PermKitchenUpdate) == authz.Allowed || authz.Check(tc.Role, authz.PermSaleCreate) == authz.Allowed {
			return nil
		}
		return errs.PermissionDenied("")
	}
	return CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(RequireModule("restaurant")(RequirePermission(check)(h)))))
}
func (s *Server) ticketRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/restaurant/kitchen/tickets/{id}", s.kitchenOperateGuard(s.kitchenTicketDetail))
	mux.Handle("GET /api/v1/restaurant/kitchen/tickets", s.kitchenOperateGuard(s.listKitchenTickets))
	mux.Handle("PATCH /api/v1/restaurant/orders/{id}/lines/{line}/kitchen-status", s.kitchenOperateGuard(s.kitchenStatus))
}
func (s *Server) kitchenStatus(w http.ResponseWriter, r *http.Request) {
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
	var req struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ranks := map[string]int{"SENT": 0, "IN_PROGRESS": 1, "READY": 2, "SERVED": 3}
	next, ok := ranks[req.Status]
	if !ok || utf8.RuneCountInString(req.Reason) > 200 {
		httpx.WriteError(w, r, errs.Validation("status/alasan tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		branch, err := lockRestaurantOrder(ctx, tx, tc, order, v)
		if err != nil {
			return err
		}
		var old string
		err = tx.QueryRow(ctx, `SELECT kitchen_status FROM restaurant_order_lines WHERE business_id=$1 AND order_id=$2 AND id=$3 FOR UPDATE`, tc.BusinessID, order, line).Scan(&old)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("baris pesanan")
		}
		if err != nil {
			return err
		}
		previous, valid := ranks[old]
		if !valid || next-previous != 1 && next-previous != -1 {
			return errs.New(errs.CodeInvalidStateTransit, "status hanya boleh maju/mundur satu langkah", http.StatusConflict)
		}
		serving := next == 3 || previous == 3
		if serving {
			if authz.Check(tc.Role, authz.PermSaleCreate) != authz.Allowed {
				return errs.PermissionDenied("SERVED hanya oleh kasir/pelayan berizin")
			}
		} else if authz.Check(tc.Role, authz.PermKitchenUpdate) != authz.Allowed {
			return errs.PermissionDenied("persiapan item hanya oleh peran dapur berizin")
		}
		if next < previous && strings.TrimSpace(req.Reason) == "" {
			return errs.Validation("mundur status wajib alasan")
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_order_lines SET kitchen_status=$4 WHERE business_id=$1 AND order_id=$2 AND id=$3`, tc.BusinessID, order, line, req.Status); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_orders SET version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, order); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "kitchen.item_status_changed", EntityType: "order_line", EntityID: line.String(), Before: map[string]string{"status": old}, After: map[string]string{"status": req.Status}, Reason: req.Reason})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"line_id": line.String(), "status": req.Status, "order_version": v + 1})
}
func (s *Server) listKitchenTickets(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return
	}
	station, err := idkit.Parse(q.Get("station_id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("station_id wajib"))
		return
	}
	var cursor idkit.ID
	if q.Get("after") != "" {
		id, err := idkit.Parse(q.Get("after"))
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return
		}
		cursor = id
	}
	var after *time.Time
	if !cursor.IsZero() {
		var at time.Time
		err := s.pool.Underlying().QueryRow(r.Context(), `SELECT created_at FROM kitchen_tickets WHERE business_id=$1 AND branch_id=$2 AND station_id=$3 AND id=$4`, tc.BusinessID, branch, station, cursor).Scan(&at)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, errs.Validation("cursor di luar stasiun/cabang"))
			return
		}
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		after = &at
	}
	// A single SQL statement presents ticket snapshots and current item statuses consistently.
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT t.id::text,t.order_id::text,t.order_type,t.table_name,t.created_at,o.version,
 (SELECT jsonb_agg(jsonb_build_object('line_id',i.line_id::text,'snapshot',i.snapshot,'status',l.kitchen_status,'cancel_reason',l.cancel_reason,'cancelled_at',l.cancelled_at) ORDER BY i.line_id)
 FROM kitchen_ticket_items i JOIN restaurant_order_lines l ON l.business_id=i.business_id AND l.id=i.line_id WHERE i.business_id=t.business_id AND i.ticket_id=t.id)
 FROM kitchen_tickets t JOIN restaurant_orders o ON o.business_id=t.business_id AND o.id=t.order_id
 WHERE t.business_id=$1 AND t.branch_id=$2 AND t.station_id=$3 AND o.status='open'
 AND EXISTS(SELECT 1 FROM kitchen_ticket_items i JOIN restaurant_order_lines l ON l.business_id=i.business_id AND l.id=i.line_id WHERE i.business_id=t.business_id AND i.ticket_id=t.id AND (l.kitchen_status IN ('SENT','IN_PROGRESS','READY') OR (l.kitchen_status='CANCELLED' AND l.cancelled_at>now()-interval '1 day')))
 AND ($4::timestamptz IS NULL OR (t.created_at,t.id)>($4,$5::uuid)) ORDER BY t.created_at,t.id LIMIT 101`, tc.BusinessID, branch, station, after, cursor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type ticket struct {
		ID      string          `json:"ticket_id"`
		Order   string          `json:"order_id"`
		Type    string          `json:"order_type"`
		Table   *string         `json:"table_name"`
		At      time.Time       `json:"created_at"`
		Version int             `json:"order_version"`
		Items   json.RawMessage `json:"items"`
	}
	out := []ticket{}
	for rows.Next() {
		var t ticket
		if err := rows.Scan(&t.ID, &t.Order, &t.Type, &t.Table, &t.At, &t.Version, &t.Items); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next := ""
	if len(out) > 100 {
		out = out[:100]
		next = out[99].ID
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"tickets": out, "next_cursor": next, "server_time": time.Now().UTC()})
}

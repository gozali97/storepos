package apiserver

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/storepos/storepos/internal/platform/authz"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) eventRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/restaurant/events", s.kitchenOperateGuard(s.restaurantEvents))
	mux.Handle("GET /api/v1/restaurant/order-status", s.kitchenOperateGuard(s.restaurantOrderStatus))
}

// Invalidations contain no receipt/customer payload. Clients reload their authorized APIs.
// shortcut: DB polling every two seconds; use a transactional event feed when polling load warrants it.
func (s *Server) restaurantEvents(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	branch, ok := branchOf(w, r, tc, r.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	cookie, err := r.Cookie(CookieSession)
	if err != nil {
		httpx.WriteError(w, r, errs.Unauthenticated(""))
		return
	}
	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Now().Add(55 * time.Second)); err != nil {
		httpx.WriteError(w, r, errs.Internal("stream tidak didukung server"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	if _, err := fmt.Fprint(w, "retry: 2000\n\n"); err != nil {
		return
	}
	if err := controller.Flush(); err != nil {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lifetime := time.NewTimer(50 * time.Second)
	defer lifetime.Stop()
	var previous [32]byte
	for {
		session, err := s.Verify(r.Context(), cookie.Value)
		if err != nil {
			return
		}
		current, err := s.Resolve(r.Context(), session.UserID, tc.BusinessID.String())
		if err != nil || !current.HasBranch(branch) || (r.Header.Get(HeaderBusinessID) == "" && session.ActiveBiz != tc.BusinessID.String()) {
			return
		}
		module := false
		for _, m := range current.EnabledModules {
			if m == "restaurant" {
				module = true
			}
		}
		if !module || (authz.Check(current.Role, authz.PermKitchenUpdate) != authz.Allowed && authz.Check(current.Role, authz.PermSaleCreate) != authz.Allowed) {
			return
		}
		var state []byte
		// Tables include derived availability; orders include item transitions via version increments.
		err = s.pool.Underlying().QueryRow(r.Context(), `SELECT jsonb_build_object(
   'orders',COALESCE((SELECT jsonb_agg(jsonb_build_array(id,version,status) ORDER BY id) FROM restaurant_orders WHERE business_id=$1 AND branch_id=$2 AND status='open'),'[]'::jsonb),
   'tables',COALESCE((SELECT jsonb_agg(jsonb_build_array(t.id,t.version,
    EXISTS(SELECT 1 FROM restaurant_orders o WHERE o.business_id=t.business_id AND o.table_id=t.id AND o.status='open'),
    EXISTS(SELECT 1 FROM restaurant_orders o WHERE o.business_id=t.business_id AND o.table_id=t.id AND o.closed_at>t.cleaned_at)) ORDER BY t.id)
    FROM restaurant_tables t WHERE t.business_id=$1 AND t.branch_id=$2 AND NOT t.archived),'[]'::jsonb))`, tc.BusinessID, branch).Scan(&state)
		if err != nil {
			httpx.Logger(r.Context()).Error("restaurant stream query failed", "err", err)
			return
		}
		hash := sha256.Sum256(state)
		if hash != previous {
			if _, err := fmt.Fprintf(w, "event: refresh\ndata: {\"branch_id\":\"%s\"}\n\n", branch.String()); err != nil {
				return
			}
			previous = hash
		} else {
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
		}
		if err := controller.Flush(); err != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-lifetime.C:
			return
		case <-ticker.C:
		}
	}
}
func (s *Server) restaurantOrderStatus(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	branch, ok := branchOf(w, r, tc, r.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT o.id::text,o.version,o.order_type,o.table_id::text,
 COALESCE(jsonb_object_agg(s.kitchen_status,s.n) FILTER(WHERE s.kitchen_status IS NOT NULL),'{}'::jsonb),
 COALESCE(SUM(s.n) FILTER(WHERE s.kitchen_status='READY'),0)
 FROM restaurant_orders o LEFT JOIN LATERAL(SELECT kitchen_status,count(*) AS n FROM restaurant_order_lines WHERE business_id=o.business_id AND order_id=o.id GROUP BY kitchen_status) s ON true
 WHERE o.business_id=$1 AND o.branch_id=$2 AND o.status='open' GROUP BY o.id ORDER BY o.created_at,o.id LIMIT 1001`, tc.BusinessID, branch)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type status struct {
		ID      string          `json:"order_id"`
		Version int             `json:"version"`
		Type    string          `json:"order_type"`
		Table   *string         `json:"table_id"`
		Counts  json.RawMessage `json:"item_status_counts"`
		Ready   int64           `json:"ready_count"`
	}
	out := []status{}
	for rows.Next() {
		var x status
		if err := rows.Scan(&x.ID, &x.Version, &x.Type, &x.Table, &x.Counts, &x.Ready); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	truncated := len(out) > 1000
	if truncated {
		out = out[:1000]
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Poll-Interval", strconv.Itoa(5))
	httpx.JSON(w, http.StatusOK, map[string]any{"orders": out, "truncated": truncated, "server_time": time.Now().UTC()})
}

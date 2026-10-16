package apiserver

import (
	"net/http"
	"strconv"
	"time"

	"github.com/storepos/storepos/internal/platform/authz"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) agingRoutes(mux *http.ServeMux) {
	h := http.HandlerFunc(s.agedOrders)
	mux.Handle("GET /api/v1/restaurant/reports/aged-orders", CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(RequireModule("restaurant")(RequirePermission(func(tc tenant.Context) error { return authz.RequireRead(tc, authz.PermReportSales, idkit.ID{}) })(h))))))
}
func (s *Server) agedOrders(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return
	}
	hours := 4
	if q.Get("hours") != "" {
		v, err := strconv.Atoi(q.Get("hours"))
		if err != nil || v < 1 || v > 720 {
			httpx.WriteError(w, r, errs.Validation("hours harus 1..720"))
			return
		}
		hours = v
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
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT o.id::text,o.order_type,t.name,o.created_at,o.version,EXTRACT(EPOCH FROM clock_timestamp()-o.created_at)::bigint FROM restaurant_orders o LEFT JOIN restaurant_tables t ON t.business_id=o.business_id AND t.id=o.table_id WHERE o.business_id=$1 AND o.branch_id=$2 AND o.status='open' AND o.created_at<now()-make_interval(hours=>$3) AND o.id>$4 ORDER BY o.id LIMIT 101`, tc.BusinessID, branch, hours, cursor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type aged struct {
		ID      string    `json:"order_id"`
		Type    string    `json:"order_type"`
		Table   *string   `json:"table_name"`
		Created time.Time `json:"created_at"`
		Version int       `json:"version"`
		Age     int64     `json:"age_seconds"`
	}
	out := []aged{}
	for rows.Next() {
		var x aged
		if err := rows.Scan(&x.ID, &x.Type, &x.Table, &x.Created, &x.Version, &x.Age); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, x)
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
	httpx.JSON(w, http.StatusOK, map[string]any{"orders": out, "threshold_hours": hours, "next_cursor": next})
}

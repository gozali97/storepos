package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) kitchenTicketDetail(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("ticket_id tidak valid"))
		return
	}
	var out map[string]any
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch, order idkit.ID
		var station *string
		var typ string
		var table *string
		var at time.Time
		err := tx.QueryRow(ctx, `SELECT t.branch_id,t.order_id,t.station_name_snapshot,t.order_type,t.table_name,t.created_at FROM kitchen_tickets t JOIN kitchen_stations s ON s.business_id=t.business_id AND s.branch_id=t.branch_id AND s.id=t.station_id WHERE t.business_id=$1 AND t.id=$2`, tc.BusinessID, id).Scan(&branch, &order, &station, &typ, &table, &at)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("tiket")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		var status string
		var version int
		if err := tx.QueryRow(ctx, `SELECT status,version FROM restaurant_orders WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, order).Scan(&status, &version); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT i.line_id::text,i.snapshot,l.kitchen_status,l.cancel_reason,l.cancelled_at FROM kitchen_ticket_items i JOIN restaurant_order_lines l ON l.business_id=i.business_id AND l.id=i.line_id WHERE i.business_id=$1 AND i.ticket_id=$2 ORDER BY i.line_id`, tc.BusinessID, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		type item struct {
			Line      string          `json:"line_id"`
			Snapshot  json.RawMessage `json:"snapshot"`
			Status    string          `json:"status"`
			Reason    *string         `json:"cancel_reason"`
			Cancelled *time.Time      `json:"cancelled_at"`
		}
		items := []item{}
		for rows.Next() {
			var x item
			if err := rows.Scan(&x.Line, &x.Snapshot, &x.Status, &x.Reason, &x.Cancelled); err != nil {
				return err
			}
			items = append(items, x)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		out = map[string]any{"ticket_id": id.String(), "order_id": order.String(), "station_name": station, "station_snapshot_missing": station == nil, "order_type": typ, "table_name": table, "created_at": at, "server_time": time.Now().UTC(), "order_status": status, "order_version": version, "label": "TIKET DAPUR — BUKAN STRUK PEMBAYARAN", "items": items}
		return nil
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, out)
}

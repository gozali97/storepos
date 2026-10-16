package apiserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/idempotency"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/restaurant"
)

func (s *Server) sendKitchen(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	order, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key wajib maksimal 128 karakter"))
		return
	}
	hash := sha256.Sum256([]byte(order.String() + ":" + r.Header.Get("If-Match")))
	var replay *idempotency.Stored
	var body []byte
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		err := tx.QueryRow(ctx, `SELECT branch_id FROM restaurant_orders WHERE business_id=$1 AND id=$2`, tc.BusinessID, order).Scan(&branch)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("pesanan")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		st, err := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /restaurant/orders/"+order.String()+"/send", hash[:])
		replay = st
		if err != nil || st != nil {
			return err
		}
		if _, err := lockRestaurantOrder(ctx, tx, tc, order, v); err != nil {
			return err
		}
		var typ string
		var table *string
		if err := tx.QueryRow(ctx, `SELECT o.order_type,t.name FROM restaurant_orders o LEFT JOIN restaurant_tables t ON t.business_id=o.business_id AND t.branch_id=o.branch_id AND t.id=o.table_id WHERE o.business_id=$1 AND o.id=$2`, tc.BusinessID, order).Scan(&typ, &table); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT l.id,l.variant_id,l.category_id,l.qty_milli,p.track_stock,jsonb_build_object('product_name',l.product_name,'variant_name',l.variant_name,'qty_milli',l.qty_milli::text,'base_unit',l.base_unit,'note',l.note,'modifiers',l.modifiers)
 FROM restaurant_order_lines l JOIN variants v ON v.business_id=l.business_id AND v.id=l.variant_id JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id
 WHERE l.business_id=$1 AND l.order_id=$2 AND l.kitchen_status='NOT_SENT' ORDER BY l.id FOR UPDATE OF l`, tc.BusinessID, order)
		if err != nil {
			return err
		}
		type item struct {
			id, variant idkit.ID
			category    *idkit.ID
			qty         int64
			track       bool
			snapshot    []byte
		}
		var items []item
		for rows.Next() {
			var i item
			if err := rows.Scan(&i.id, &i.variant, &i.category, &i.qty, &i.track, &i.snapshot); err != nil {
				rows.Close()
				return err
			}
			items = append(items, i)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(items) == 0 {
			return errs.Conflict("tidak ada item NOT_SENT untuk dikirim")
		}
		sort.Slice(items, func(i, j int) bool { return items[i].variant.String() < items[j].variant.String() })
		for _, i := range items {
			soldOut, err := restaurant.SoldOut(ctx, tx, tc.BusinessID, branch, i.variant)
			if err != nil {
				return err
			}
			if soldOut {
				return errs.Conflict("menu ditandai Habis; kirim ditolak")
			}
			if i.track {
				if err := restaurant.Reserve(ctx, tx, tc.BusinessID, branch, i.id, i.variant, i.qty); err != nil {
					if errors.Is(err, restaurant.ErrReservedStock) {
						return errs.New(errs.CodeStockInsufficient, err.Error(), http.StatusConflict)
					}
					return err
				}
			}
		}
		tickets := map[idkit.ID]idkit.ID{}
		ticketIDs := []string{}
		for _, i := range items {
			if i.category == nil {
				return errs.Validation("menu tanpa kategori belum memiliki routing dapur")
			}
			var station idkit.ID
			var stationName string
			err := tx.QueryRow(ctx, `SELECT s.id,s.name FROM kitchen_category_routes r JOIN kitchen_stations s ON s.business_id=r.business_id AND s.branch_id=r.branch_id AND s.id=r.station_id WHERE r.business_id=$1 AND r.branch_id=$2 AND r.category_id=$3 AND NOT s.archived FOR SHARE OF r,s`, tc.BusinessID, branch, *i.category).Scan(&station, &stationName)
			if errors.Is(err, pgx.ErrNoRows) {
				return errs.Validation("kategori menu belum memiliki stasiun aktif")
			}
			if err != nil {
				return err
			}
			ticket, exists := tickets[station]
			if !exists {
				ticket = idkit.New()
				tickets[station] = ticket
				ticketIDs = append(ticketIDs, ticket.String())
				if _, err := tx.Exec(ctx, `INSERT INTO kitchen_tickets(id,business_id,branch_id,order_id,station_id,order_type,table_name,created_by,station_name_snapshot) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, ticket, tc.BusinessID, branch, order, station, typ, table, tc.UserID, stationName); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx, `INSERT INTO kitchen_ticket_items(business_id,ticket_id,line_id,snapshot) VALUES($1,$2,$3,$4)`, tc.BusinessID, ticket, i.id, i.snapshot); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE restaurant_order_lines SET kitchen_status='SENT' WHERE business_id=$1 AND id=$2`, tc.BusinessID, i.id); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_orders SET version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, order); err != nil {
			return err
		}
		if err := audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "kitchen.items_sent", EntityType: "restaurant_order", EntityID: order.String(), After: map[string]any{"ticket_ids": ticketIDs, "items": len(items)}}); err != nil {
			return err
		}
		body, err = json.Marshal(map[string]any{"order_id": order.String(), "version": v + 1, "ticket_ids": ticketIDs})
		if err != nil {
			return err
		}
		return idempotency.Complete(ctx, tx, tc.BusinessID, key, http.StatusCreated, body)
	})
	if err != nil {
		if errors.Is(err, idempotency.ErrConflict) {
			httpx.WriteError(w, r, mapSalesError(err))
		} else {
			httpx.WriteError(w, r, err)
		}
		return
	}
	if replay != nil {
		body = replay.Body
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(body)
}

package apiserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/idempotency"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) orderGuard(h http.HandlerFunc) http.Handler {
	return CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(RequireModule("restaurant")(RequirePermission(func(tc tenant.Context) error {
		_, err := authz.Require(tc, authz.PermSaleCreate, idkit.ID{})
		return err
	})(h)))))
}
func (s *Server) orderRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/restaurant/orders/{id}/checkout", s.orderGuard(s.checkoutRestaurant))
	mux.Handle("PUT /api/v1/restaurant/orders/{id}", s.orderGuard(s.editOrderHeader))
	mux.Handle("POST /api/v1/restaurant/orders/{id}/lines/{line}/cancel", s.orderGuard(s.cancelKitchenLine))
	mux.Handle("POST /api/v1/restaurant/orders/{id}/send", s.orderGuard(s.sendKitchen))
	mux.Handle("POST /api/v1/restaurant/orders", s.orderGuard(s.openOrder))
	mux.Handle("GET /api/v1/restaurant/orders/{id}", s.orderGuard(s.getOrder))
	mux.Handle("GET /api/v1/restaurant/orders", s.orderGuard(s.listOrders))
}
func (s *Server) openOrder(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	var req struct {
		Branch       string  `json:"branch_id"`
		Table        *string `json:"table_id"`
		Type         string  `json:"order_type"`
		Note         string  `json:"note"`
		Channel      string  `json:"channel"`
		Fee          string  `json:"delivery_fee_cents"`
		ContactName  string  `json:"contact_name"`
		ContactPhone string  `json:"contact_phone"`
		Queue        string  `json:"queue_label"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.Branch)
	if !ok {
		return
	}
	if req.Type != "DINE_IN" && req.Type != "TAKEAWAY" && req.Type != "DELIVERY_MANUAL" || utf8.RuneCountInString(req.Note) > 200 || utf8.RuneCountInString(req.Channel) > 100 {
		httpx.WriteError(w, r, errs.Validation("jenis/catatan/kanal pesanan tidak valid"))
		return
	}
	if req.Fee == "" {
		req.Fee = "0"
	}
	fee, err := money.ParseCents(req.Fee)
	if err != nil || fee < 0 || fee > 10000000000 || !validOrderContact(req.Type, req.ContactName, req.ContactPhone, req.Queue) || req.Type != "DELIVERY_MANUAL" && fee != 0 {
		httpx.WriteError(w, r, errs.Validation("ongkir hanya untuk delivery dan tidak negatif"))
		return
	}
	var table *idkit.ID
	if req.Table != nil {
		id, err := idkit.Parse(*req.Table)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("table_id tidak valid"))
			return
		}
		table = &id
	}
	if (req.Type == "DINE_IN") != (table != nil) {
		httpx.WriteError(w, r, errs.Validation("dine-in wajib meja; jenis lain tidak memakai meja"))
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) > 128 {
		httpx.WriteError(w, r, errs.Validation("Idempotency-Key maksimal 128 karakter"))
		return
	}
	raw, err := json.Marshal(req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hash := sha256.Sum256(raw)
	var replay *idempotency.Stored
	var response []byte
	code := http.StatusCreated
	id := idkit.New()
	version := 1
	resumed := false
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		resumed = false
		version = 1
		var active bool
		err := tx.QueryRow(ctx, `SELECT status='active' FROM branches WHERE business_id=$1 AND id=$2`, tc.BusinessID, branch).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("cabang")
		}
		if err != nil {
			return err
		}
		if !active {
			return errs.Conflict("cabang nonaktif")
		}
		// Recheck current branch permission/activity before replay. Key remains optional for legacy clients.
		if key != "" {
			stored, err := idempotency.Begin(ctx, tx, tc.BusinessID, key, "POST /restaurant/orders", hash[:])
			replay = stored
			if err != nil || stored != nil {
				return err
			}
		}
		finish := func() error {
			code = http.StatusCreated
			if resumed {
				code = http.StatusOK
			}
			var err error
			response, err = json.Marshal(map[string]any{"order_id": id.String(), "version": version, "resumed": resumed, "detail_path": "/api/v1/restaurant/orders/" + id.String()})
			if err != nil {
				return err
			}
			if key != "" {
				return idempotency.Complete(ctx, tx, tc.BusinessID, key, code, response)
			}
			return nil
		}
		if table != nil {
			err := tx.QueryRow(ctx, `SELECT NOT t.archived AND NOT a.archived FROM restaurant_tables t JOIN restaurant_areas a ON a.business_id=t.business_id AND a.branch_id=t.branch_id AND a.id=t.area_id WHERE t.business_id=$1 AND t.branch_id=$2 AND t.id=$3 FOR UPDATE OF t`, tc.BusinessID, branch, *table).Scan(&active)
			if errors.Is(err, pgx.ErrNoRows) {
				return errs.NotFound("meja")
			}
			if err != nil {
				return err
			}
			if !active {
				return errs.Conflict("meja/area diarsipkan")
			}
			var dirty bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM restaurant_orders o JOIN restaurant_tables t ON t.business_id=o.business_id AND t.id=o.table_id WHERE o.business_id=$1 AND o.table_id=$2 AND o.closed_at>t.cleaned_at)`, tc.BusinessID, *table).Scan(&dirty); err != nil {
				return err
			}
			if dirty {
				return errs.Conflict("meja perlu dibersihkan sebelum pesanan baru")
			}
			err = tx.QueryRow(ctx, `SELECT id,version FROM restaurant_orders WHERE business_id=$1 AND table_id=$2 AND status='open'`, tc.BusinessID, *table).Scan(&id, &version)
			if err == nil {
				resumed = true
				return finish()
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO restaurant_orders(id,business_id,branch_id,table_id,order_type,note,channel,delivery_fee_cents,created_by,contact_name,contact_phone,queue_label) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, id, tc.BusinessID, branch, table, req.Type, req.Note, strings.TrimSpace(req.Channel), int64(fee), tc.UserID, strings.TrimSpace(req.ContactName), strings.TrimSpace(req.ContactPhone), strings.TrimSpace(req.Queue)); err != nil {
			return err
		}
		if err := audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.order_opened", EntityType: "restaurant_order", EntityID: id.String(), After: map[string]string{"order_type": req.Type}}); err != nil {
			return err
		}
		return finish()
	})
	if err != nil {
		if errors.Is(err, idempotency.ErrConflict) {
			err = mapSalesError(err)
		}
		httpx.WriteError(w, r, err)
		return
	}
	if replay != nil {
		response = replay.Body
		code = replay.Status
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_, _ = w.Write(response)
}

type orderHeader struct {
	ContactName  string    `json:"contact_name"`
	ContactPhone string    `json:"contact_phone"`
	Queue        string    `json:"queue_label"`
	ID           string    `json:"order_id"`
	Branch       string    `json:"branch_id"`
	Table        *string   `json:"table_id"`
	Type         string    `json:"order_type"`
	Status       string    `json:"status"`
	Payment      string    `json:"payment_status"`
	Version      int       `json:"version"`
	Note         string    `json:"note"`
	Channel      string    `json:"channel"`
	Fee          string    `json:"delivery_fee_cents"`
	At           time.Time `json:"created_at"`
}

func (s *Server) getOrder(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("order_id tidak valid"))
		return
	}
	var h orderHeader
	var branch idkit.ID
	err = s.pool.Underlying().QueryRow(r.Context(), `SELECT id::text,branch_id,table_id::text,order_type,status,payment_status,version,note,channel,delivery_fee_cents::text,created_at,contact_name,contact_phone,queue_label FROM restaurant_orders WHERE business_id=$1 AND id=$2`, tc.BusinessID, id).Scan(&h.ID, &branch, &h.Table, &h.Type, &h.Status, &h.Payment, &h.Version, &h.Note, &h.Channel, &h.Fee, &h.At, &h.ContactName, &h.ContactPhone, &h.Queue)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, errs.NotFound("pesanan"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !tc.HasBranch(branch) {
		httpx.WriteError(w, r, errs.PermissionDenied(""))
		return
	}
	h.Branch = branch.String()
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, h)
}
func (s *Server) listOrders(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	q := r.URL.Query()
	branch, ok := branchOf(w, r, tc, q.Get("branch_id"))
	if !ok {
		return
	}
	var cursor idkit.ID
	if raw := q.Get("after"); raw != "" {
		id, err := idkit.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.Validation("cursor tidak valid"))
			return
		}
		cursor = id
	}
	status := q.Get("status")
	if status == "" {
		status = "open"
	}
	if status != "open" && status != "completed" && status != "cancelled" && status != "all" {
		httpx.WriteError(w, r, errs.Validation("status harus open/completed/cancelled/all"))
		return
	}
	typ := q.Get("order_type")
	if typ != "" && !validOrderType(typ) {
		httpx.WriteError(w, r, errs.Validation("order_type tidak valid"))
		return
	}
	var from, until *time.Time
	if q.Get("from") != "" || q.Get("to") != "" {
		f, ok := s.reportFilter(w, r)
		if !ok {
			return
		}
		from = &f.From
		until = &f.Until
	} else if status != "open" {
		httpx.WriteError(w, r, errs.Validation("riwayat pesanan wajib periode from/to"))
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT id::text,branch_id::text,table_id::text,order_type,status,payment_status,version,note,channel,delivery_fee_cents::text,created_at FROM restaurant_orders WHERE business_id=$1 AND branch_id=$2 AND ($4='all' OR status=$4) AND ($5='' OR order_type=$5) AND ($6::timestamptz IS NULL OR created_at >= $6) AND ($7::timestamptz IS NULL OR created_at < $7) AND id>$3 ORDER BY id LIMIT 101`, tc.BusinessID, branch, cursor, status, typ, from, until)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	out := []orderHeader{}
	for rows.Next() {
		var h orderHeader
		if err := rows.Scan(&h.ID, &h.Branch, &h.Table, &h.Type, &h.Status, &h.Payment, &h.Version, &h.Note, &h.Channel, &h.Fee, &h.At); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, h)
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
	httpx.JSON(w, http.StatusOK, map[string]any{"orders": out, "next_cursor": next})
}

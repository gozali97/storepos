package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) operationRoutes(mux *http.ServeMux) {
	mux.Handle("PUT /api/v1/restaurant/service-charge", s.restaurantGuard(true, s.setServiceCharge))
	mux.Handle("GET /api/v1/restaurant/service-charge", s.restaurantGuard(false, s.getServiceCharge))
	mux.Handle("POST /api/v1/restaurant/orders/{id}/cancel", s.orderGuard(s.cancelOrder))
	mux.Handle("POST /api/v1/restaurant/tables/{id}/clean", s.orderGuard(s.cleanTable))
	mux.Handle("GET /api/v1/restaurant/orders/{id}/bill", s.orderGuard(s.orderBill))
}
func validOrderType(t string) bool {
	return t == "DINE_IN" || t == "TAKEAWAY" || t == "DELIVERY_MANUAL"
}
func (s *Server) setServiceCharge(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	v, err := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), `"`))
	if err != nil || v < 0 {
		httpx.WriteError(w, r, errs.Validation("If-Match wajib (0 baru)"))
		return
	}
	var req struct {
		Branch  string `json:"branch_id"`
		Type    string `json:"order_type"`
		Rate    int    `json:"rate_bps"`
		TaxBase bool   `json:"include_in_tax_base"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	branch, ok := branchOf(w, r, tc, req.Branch)
	if !ok {
		return
	}
	if !validOrderType(req.Type) || req.Rate < 0 || req.Rate > 10000 {
		httpx.WriteError(w, r, errs.Validation("jenis/tarif tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var active bool
		err := tx.QueryRow(ctx, `SELECT status='active' FROM branches WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, branch).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("cabang")
		}
		if err != nil {
			return err
		}
		if !active {
			return errs.Conflict("cabang nonaktif")
		}
		var current int
		err = tx.QueryRow(ctx, `SELECT version FROM restaurant_service_charges WHERE business_id=$1 AND branch_id=$2 AND order_type=$3`, tc.BusinessID, branch, req.Type).Scan(&current)
		if !errors.Is(err, pgx.ErrNoRows) && err != nil {
			return err
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "service charge berubah", http.StatusConflict)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO restaurant_service_charges(business_id,branch_id,order_type,rate_bps,include_in_tax_base) VALUES($1,$2,$3,$4,$5) ON CONFLICT(business_id,branch_id,order_type) DO UPDATE SET rate_bps=EXCLUDED.rate_bps,include_in_tax_base=EXCLUDED.include_in_tax_base,version=restaurant_service_charges.version+1`, tc.BusinessID, branch, req.Type, req.Rate, req.TaxBase); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.service_charge_changed", EntityType: "branch", EntityID: branch.String(), After: map[string]any{"order_type": req.Type, "rate_bps": req.Rate, "include_in_tax_base": req.TaxBase}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"version": v + 1})
}
func (s *Server) getServiceCharge(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	branch, ok := branchOf(w, r, tc, r.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	rows, err := s.pool.Underlying().Query(r.Context(), `SELECT order_type,rate_bps,include_in_tax_base,version FROM restaurant_service_charges WHERE business_id=$1 AND branch_id=$2 ORDER BY order_type`, tc.BusinessID, branch)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	type rule struct {
		Type    string `json:"order_type"`
		Rate    int    `json:"rate_bps"`
		TaxBase bool   `json:"include_in_tax_base"`
		Version int    `json:"version"`
	}
	out := []rule{}
	for rows.Next() {
		var x rule
		if err := rows.Scan(&x.Type, &x.Rate, &x.TaxBase, &x.Version); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"rules": out})
}
func (s *Server) cancelOrder(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if utf8.RuneCountInString(req.Reason) > 200 {
		httpx.WriteError(w, r, errs.Validation("alasan maksimal 200 karakter"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		branch, err := lockRestaurantOrder(ctx, tx, tc, id, v)
		if err != nil {
			return err
		}
		var sent bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM restaurant_order_lines WHERE business_id=$1 AND order_id=$2 AND kitchen_status NOT IN ('NOT_SENT','CANCELLED'))`, tc.BusinessID, id).Scan(&sent); err != nil {
			return err
		}
		if sent {
			return errs.Conflict("batalkan item yang sudah dikirim dengan persetujuan dahulu")
		}
		var history bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM restaurant_order_lines WHERE business_id=$1 AND order_id=$2 AND cancelled_from_status IN ('SENT','IN_PROGRESS','READY','SERVED'))`, tc.BusinessID, id).Scan(&history); err != nil {
			return err
		}
		if history && req.Reason == "" {
			return errs.Validation("pesanan pernah dikirim; alasan pembatalan wajib")
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_order_lines SET kitchen_status='CANCELLED',cancelled_at=clock_timestamp(),cancelled_by=$3,cancelled_from_status=kitchen_status,cancel_reason=$4 WHERE business_id=$1 AND order_id=$2 AND kitchen_status='NOT_SENT'`, tc.BusinessID, id, tc.UserID, req.Reason); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_orders SET status='cancelled',closed_at=clock_timestamp(),cancel_reason=$3,cancelled_by=$4,version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id, req.Reason, tc.UserID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.order_cancelled", EntityType: "restaurant_order", EntityID: id.String(), Reason: req.Reason})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"order_id": id.String(), "status": "cancelled", "version": v + 1})
}
func (s *Server) cleanTable(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		var current int
		err := tx.QueryRow(ctx, `SELECT branch_id,version FROM restaurant_tables WHERE business_id=$1 AND id=$2 AND NOT archived FOR UPDATE`, tc.BusinessID, id).Scan(&branch, &current)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("meja")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "meja berubah", http.StatusConflict)
		}
		var open bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM restaurant_orders WHERE business_id=$1 AND table_id=$2 AND status='open')`, tc.BusinessID, id).Scan(&open); err != nil {
			return err
		}
		if open {
			return errs.Conflict("meja masih memiliki pesanan terbuka")
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_tables SET cleaned_at=clock_timestamp(),version=version+1 WHERE business_id=$1 AND id=$2`, tc.BusinessID, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.table_cleaned", EntityType: "table", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"table_id": id.String(), "version": v + 1, "status": "AVAILABLE"})
}

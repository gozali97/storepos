package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/restaurant"
	"github.com/storepos/storepos/internal/sales"
)

func (s *Server) cancelKitchenLine(w http.ResponseWriter, r *http.Request) {
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
		Reason   string       `json:"reason"`
		Approval *approvalReq `json:"approval"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" || utf8.RuneCountInString(req.Reason) > 200 {
		httpx.WriteError(w, r, errs.Validation("alasan wajib maksimal 200 karakter"))
		return
	}
	approved, err := s.approver(r.Context(), tc, req.Approval)
	if err != nil {
		httpx.WriteError(w, r, mapShiftError(err))
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
			return errs.NotFound("item")
		}
		if err != nil {
			return err
		}
		if old == "CANCELLED" {
			return errs.Conflict("item sudah dibatalkan")
		}
		grant := authz.Check(tc.Role, authz.PermCancelItemAfterSend)
		// Cashiers have sale.create but conditional cancellation; explicit permission remains required otherwise.
		if old != "NOT_SENT" {
			if grant == authz.Denied && tc.Role != tenant.RoleCashier && tc.Role != tenant.RolePharmacist {
				return errs.PermissionDenied("")
			}
			needsApproval := old != "SENT" || grant != authz.Allowed
			if needsApproval && approved == nil {
				return sales.ErrApprovalRequired
			}
		}
		if err := restaurant.Release(ctx, tx, tc.BusinessID, line); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_order_lines SET kitchen_status='CANCELLED',cancelled_at=clock_timestamp(),cancelled_by=$4,cancel_approved_by=$5,cancel_reason=$6,cancelled_from_status=kitchen_status WHERE business_id=$1 AND order_id=$2 AND id=$3`, tc.BusinessID, order, line, tc.UserID, approved, req.Reason); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_orders SET version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, order); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, ApprovedBy: approved, Action: "restaurant.item_cancelled", EntityType: "order_line", EntityID: line.String(), Before: map[string]string{"status": old}, After: map[string]string{"status": "CANCELLED"}, Reason: req.Reason})
	})
	if err != nil {
		if errors.Is(err, sales.ErrApprovalRequired) {
			httpx.WriteError(w, r, mapShiftError(err))
		} else {
			httpx.WriteError(w, r, err)
		}
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"line_id": line.String(), "status": "CANCELLED", "order_version": v + 1})
}

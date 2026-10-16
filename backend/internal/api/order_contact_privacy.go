package apiserver

import (
	"context"
	"errors"
	"net/http"
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

func validOrderContact(typ, name, phone, queue string) bool {
	if utf8.RuneCountInString(name) > 100 || len(phone) > 32 || utf8.RuneCountInString(queue) > 40 {
		return false
	}
	if typ == "DINE_IN" && (name != "" || phone != "" || queue != "") {
		return false
	}
	if typ != "DELIVERY_MANUAL" && phone != "" {
		return false
	}
	for _, c := range phone {
		if !(c >= '0' && c <= '9') && c != '+' && c != ' ' && c != '-' && c != '(' && c != ')' {
			return false
		}
	}
	return true
}
func (s *Server) contactPrivacyRoutes(mux *http.ServeMux) {
	h := http.HandlerFunc(s.eraseOrderContact)
	mux.Handle("DELETE /api/v1/restaurant/orders/{id}/contact", CSRFMiddleware(s.secure)(RequireAuth(s)(RequireTenant(s)(RequireModule("restaurant")(RequirePermission(func(tc tenant.Context) error {
		_, err := authz.Require(tc, authz.PermBusinessManage, idkit.ID{})
		return err
	})(h))))))
}
func (s *Server) eraseOrderContact(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, v, err := countVersion(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		var current int
		err := tx.QueryRow(ctx, `SELECT branch_id,version FROM restaurant_orders WHERE business_id=$1 AND id=$2 FOR UPDATE`, tc.BusinessID, id).Scan(&branch, &current)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("pesanan")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		if current != v {
			return errs.New(errs.CodeOptimisticLockFailure, "pesanan berubah", http.StatusConflict)
		}
		if _, err := tx.Exec(ctx, `UPDATE restaurant_orders SET contact_name='',contact_phone='',queue_label='',version=version+1,updated_at=now() WHERE business_id=$1 AND id=$2`, tc.BusinessID, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "restaurant.contact_erased", EntityType: "restaurant_order", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"order_id": id.String(), "version": v + 1})
}

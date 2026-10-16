package apiserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

// Reprint records a requested copy, not proof that a physical printer succeeded.
// The JSON detail payload is supplied by the existing snapshot reader, never by the client.
func (s *Server) reprintReceipt(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("sale_id tidak valid"))
		return
	}
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch, reg idkit.ID
		err := tx.QueryRow(ctx, `SELECT branch_id,register_id FROM sales WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, id).Scan(&branch, &reg)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("transaksi")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, RegisterID: &reg, ActorUserID: tc.UserID, Action: "receipt.reprint_requested", EntityType: "sale", EntityID: id.String(), After: map[string]string{"label": "SALINAN"}})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	// Existing authenticated detail endpoint supplies line/payment snapshots.
	httpx.JSON(w, http.StatusCreated, map[string]string{"label": "SALINAN", "sale_id": id.String(), "detail_path": "/api/v1/sales/" + id.String()})
}

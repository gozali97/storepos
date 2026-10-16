package apiserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/httpx"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func (s *Server) prescriptionRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/pharmacy/sales/{id}/prescription", s.pharmacyGuard(authz.PermPrescriptionVerify, false, s.readPrescriptionReference))
}

// Referensi resep tidak ada di detail sales/struk umum. Pembacaan harus berhasil diaudit sebelum data dikirim.
func (s *Server) readPrescriptionReference(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	if tc.Role != tenant.RolePharmacist {
		httpx.WriteError(w, r, errs.PermissionDenied("referensi resep hanya untuk Apoteker"))
		return
	}
	id, err := idkit.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, errs.Validation("sale_id tidak valid"))
		return
	}
	var reference, verifiedBy *string
	err = s.pool.InTx(r.Context(), func(ctx context.Context, tx database.Tx) error {
		var branch idkit.ID
		err := tx.QueryRow(ctx, `SELECT branch_id,prescription_reference,pharmacist_verified_by::text FROM sales WHERE business_id=$1 AND id=$2 FOR SHARE`, tc.BusinessID, id).Scan(&branch, &reference, &verifiedBy)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("penjualan")
		}
		if err != nil {
			return err
		}
		if !tc.HasBranch(branch) {
			return errs.PermissionDenied("")
		}
		return audit.Write(ctx, tx, audit.Entry{BusinessID: tc.BusinessID, BranchID: &branch, ActorUserID: tc.UserID, Action: "pharmacy.prescription_reference_read", EntityType: "sale", EntityID: id.String()})
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Referrer-Policy", "no-referrer")
	httpx.JSON(w, 200, map[string]any{"sale_id": id.String(), "prescription_reference": reference, "verified_by": verifiedBy})
}

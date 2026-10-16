package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

// Keranjang pembayaran harus sama dengan versi draft yang disimpan; pembayaran tetap dari request.
func (s *Server) checkDraftFinalization(ctx context.Context, tx database.Tx, tc tenant.Context, req finalizeReq, branch, reg idkit.ID) error {
	id, err := idkit.Parse(req.DraftID)
	if err != nil || req.DraftVersion <= 0 {
		return errs.Validation("draft_id dan draft_version tidak valid")
	}
	if err := lockDraftTerminal(ctx, tx, tc, branch, reg); err != nil {
		return err
	}
	var status string
	var version int
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT status,version,payload FROM sale_drafts WHERE business_id=$1 AND branch_id=$2 AND register_id=$3 AND id=$4 FOR UPDATE`, tc.BusinessID, branch, reg, id).Scan(&status, &version, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.NotFound("draft")
	}
	if err != nil {
		return err
	}
	if version != req.DraftVersion {
		return errs.New(errs.CodeOptimisticLockFailure, "draft diubah pengguna lain", http.StatusConflict)
	}
	if status != "active" {
		return errs.Conflict("hanya draft aktif yang dapat difinalisasi")
	}
	var p draftPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	if p.Note != req.Note || len(p.Lines) != len(req.Lines) || p.Discount != req.Discount || p.DiscountReason != req.DiscountReason {
		return errs.Conflict("keranjang pembayaran berbeda dari draft tersimpan")
	}
	for i, l := range p.Lines {
		r := req.Lines[i]
		if l.Note != r.Note || l.UnitID != r.UnitID || l.VariantID != r.VariantID || l.QtyMilli != r.QtyMilli || l.Discount != r.Discount {
			return errs.Conflict("baris pembayaran berbeda dari draft tersimpan")
		}
	}
	return nil
}

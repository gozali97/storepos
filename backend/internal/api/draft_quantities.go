package apiserver

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/platform/database"
	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/money"
	"github.com/storepos/storepos/internal/purchasing"
)

func validateDraftVariants(ctx context.Context, db database.Getter, biz idkit.ID, p draftPayload) error {
	for _, l := range p.Lines {
		id, err := idkit.Parse(l.VariantID)
		if err != nil {
			return errs.Validation("variant_id tidak valid")
		}
		qty, err := money.ParseMilli(l.QtyMilli)
		if err != nil {
			return errs.Validation("qty_milli tidak valid")
		}
		var fractional bool
		err = db.QueryRow(ctx, `SELECT v.allow_fractional FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id WHERE v.business_id=$1 AND v.id=$2 AND v.status='active' AND p.status='active'`, biz, id).Scan(&fractional)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.Validation("varian tidak ditemukan atau nonaktif")
		}
		if err != nil {
			return err
		}
		if l.UnitID != "" {
			uid, err := idkit.Parse(l.UnitID)
			if err != nil {
				return errs.Validation("unit_id tidak valid")
			}
			var factor int64
			var price *int64
			err = db.QueryRow(ctx, `SELECT factor_milli,sale_price_cents FROM purchase_units WHERE business_id=$1 AND variant_id=$2 AND id=$3 AND NOT archived`, biz, id, uid).Scan(&factor, &price)
			if errors.Is(err, pgx.ErrNoRows) || err == nil && price == nil {
				return errs.Validation("satuan jual tidak tersedia")
			}
			if err != nil {
				return err
			}
			if _, err := purchasing.Convert(int64(qty), factor); err != nil {
				return errs.Validation("konversi kuantitas kehilangan presisi")
			}
		}
		if !fractional && qty%1000 != 0 {
			return errs.Validation("varian tidak mengizinkan kuantitas pecahan")
		}
	}
	return nil
}

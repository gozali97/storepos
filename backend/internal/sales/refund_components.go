package sales

import (
	"context"
	"errors"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

var ErrReturnComponents = errors.New("rincian refund historis belum lengkap; rekonsiliasi manual diperlukan")

// Net is the residual so net + tax + SC always equals the actual refunded amount.
// Inclusive tax is already included in line_total_cents and must never be added again.
func refundComponents(ctx context.Context, db database.Getter, biz, line idkit.ID, sold, returned, qty, amount int64) (net, tax, sc int64, err error) {
	var taxTotal, scTotal, priorTax, priorSC int64
	var missing bool
	err = db.QueryRow(ctx, `SELECT l.tax_cents,l.service_charge_cents,
 COALESCE((SELECT SUM(r.tax_cents) FROM sale_return_lines r WHERE r.business_id=l.business_id AND r.sale_line_id=l.id),0),
 COALESCE((SELECT SUM(r.service_charge_cents) FROM sale_return_lines r WHERE r.business_id=l.business_id AND r.sale_line_id=l.id),0),
 EXISTS(SELECT 1 FROM sale_return_lines r WHERE r.business_id=l.business_id AND r.sale_line_id=l.id AND (r.net_cents IS NULL OR r.tax_cents IS NULL OR r.service_charge_cents IS NULL))
 FROM sale_lines l WHERE l.business_id=$1 AND l.id=$2`, biz, line).Scan(&taxTotal, &scTotal, &priorTax, &priorSC, &missing)
	if err != nil {
		return
	}
	if missing {
		err = ErrReturnComponents
		return
	}
	tax, err = RefundSlice(taxTotal, sold, returned, priorTax, qty)
	if err != nil {
		return
	}
	sc, err = RefundSlice(scTotal, sold, returned, priorSC, qty)
	if err != nil {
		return
	}
	if tax+sc > amount {
		err = ErrReturnComponents
		return
	}
	net = amount - tax - sc
	return
}

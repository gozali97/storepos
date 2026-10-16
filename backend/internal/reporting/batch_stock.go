package reporting

import (
	"context"

	"github.com/storepos/storepos/internal/platform/database"
)

type BatchStockRow struct {
	BatchID        string  `json:"batch_id"`
	VariantID      string  `json:"variant_id"`
	SKU            string  `json:"sku"`
	ProductName    string  `json:"product_name"`
	BatchNumber    string  `json:"batch_number"`
	ExpiryDate     string  `json:"expiry_date"`
	StoredStatus   string  `json:"stored_status"`
	Status         string  `json:"status"` // EXPIRED diturunkan dari tanggal lokal cabang, bukan disimpan
	DaysToExpiry   int     `json:"days_to_expiry"`
	Balance        string  `json:"qty_milli"`
	Movement       string  `json:"period_movement_milli"`
	MovementCount  int64   `json:"period_movement_count"`
	LastSupplier   *string `json:"last_supplier_name"`
	AverageCost    *string `json:"average_cost_cents"`
	EstimatedValue *string `json:"estimated_value_cents"`
}

// BatchStock melaporkan saldo per (batch, status) saat ini; hanya total pergerakan yang mengikuti periode.
// Biaya adalah rata-rata tertimbang harga beli penerimaan batch itu di cabang ini (estimasi, bukan HPP inti).
// filter: all | near_expiry | expired | quarantine. nearDays dipakai oleh near_expiry.
func BatchStock(ctx context.Context, db database.Getter, f PaymentFilter, canViewCost bool, filter string, nearDays int) ([]BatchStockRow, error) {
	rows, err := db.Query(ctx, `WITH today AS (SELECT (clock_timestamp() AT TIME ZONE $6)::date AS d),
 moves AS (
  SELECT batch_id,status,SUM(qty_milli) AS qty,count(*) AS n FROM pharmacy_batch_movements
  WHERE business_id=$1 AND branch_id=$2 AND created_at >= $3 AND created_at < $4 GROUP BY batch_id,status
 ), cost AS (
  SELECT batch_id,SUM(qty_milli::numeric*unit_cost_cents)/NULLIF(SUM(qty_milli),0) AS avg_cost FROM pharmacy_batch_receipts
  WHERE business_id=$1 AND branch_id=$2 GROUP BY batch_id
 ) SELECT b.id::text,b.variant_id::text,v.sku,p.name,b.batch_number,b.expiry_date::text,x.status,
 CASE WHEN x.status='SELLABLE' AND b.expiry_date <= today.d THEN 'EXPIRED' ELSE x.status END,
 (b.expiry_date-today.d),x.qty_milli::text,COALESCE(m.qty,0)::text,COALESCE(m.n,0),
 (SELECT s.name FROM pharmacy_batch_receipts r JOIN suppliers s ON s.business_id=r.business_id AND s.id=r.supplier_id WHERE r.business_id=b.business_id AND r.batch_id=b.id AND r.branch_id=x.branch_id ORDER BY r.created_at DESC,r.id DESC LIMIT 1),
 CASE WHEN $5 THEN ROUND(c.avg_cost)::text END,
 CASE WHEN $5 AND c.avg_cost IS NOT NULL THEN ROUND(x.qty_milli::numeric*c.avg_cost/1000)::text END
 FROM pharmacy_batch_balances x JOIN pharmacy_batches b ON b.business_id=x.business_id AND b.id=x.batch_id
 JOIN variants v ON v.business_id=b.business_id AND v.id=b.variant_id JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id
 LEFT JOIN moves m ON m.batch_id=b.id AND m.status=x.status LEFT JOIN cost c ON c.batch_id=b.id CROSS JOIN today
 WHERE x.business_id=$1 AND x.branch_id=$2 AND x.status<>'DISPOSED' AND (x.qty_milli>0 OR COALESCE(m.n,0)>0)
 AND ($7='all'
  OR ($7='quarantine' AND x.status='QUARANTINE')
  OR ($7='expired' AND b.expiry_date <= today.d AND x.qty_milli>0)
  OR ($7='near_expiry' AND b.expiry_date > today.d AND b.expiry_date <= today.d+$8::int AND x.qty_milli>0))
 ORDER BY b.expiry_date,b.id,x.status LIMIT NULLIF($9,0)`,
		f.BusinessID, f.BranchID, f.From, f.Until, canViewCost, f.Timezone, filter, nearDays, reportLimit(f))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BatchStockRow{}
	for rows.Next() {
		var x BatchStockRow
		if err := rows.Scan(&x.BatchID, &x.VariantID, &x.SKU, &x.ProductName, &x.BatchNumber, &x.ExpiryDate, &x.StoredStatus, &x.Status,
			&x.DaysToExpiry, &x.Balance, &x.Movement, &x.MovementCount, &x.LastSupplier, &x.AverageCost, &x.EstimatedValue); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

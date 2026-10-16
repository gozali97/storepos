package reporting

import (
	"context"
	"github.com/storepos/storepos/internal/platform/database"
)

type StockRow struct {
	Reserved       string  `json:"reserved_milli"`
	Available      string  `json:"available_milli"`
	VariantID      string  `json:"variant_id"`
	SKU            string  `json:"sku"`
	ProductName    string  `json:"product_name"`
	Unit           string  `json:"base_unit"`
	Balance        string  `json:"qty_milli"`
	Minimum        *string `json:"minimum_qty_milli"`
	BelowMinimum   bool    `json:"below_minimum"`
	Negative       bool    `json:"negative"`
	AverageCost    *string `json:"average_cost_cents"`
	EstimatedValue *string `json:"estimated_value_cents"`
	CostUnknown    bool    `json:"cost_unknown"`
	Movement       string  `json:"period_movement_milli"`
	MovementCount  int64   `json:"period_movement_count"`
}

// Balance/HPP are current; only movement totals are period-filtered. RETURN_DAMAGED
// records damaged returns without changing saleable inventory, so excludes it from net movement.
func Stock(ctx context.Context, db database.Getter, f PaymentFilter, canViewCost bool, filter string) ([]StockRow, error) {
	rows, err := db.Query(ctx, `WITH movements AS (
 SELECT variant_id,SUM(CASE WHEN kind='RETURN_DAMAGED' THEN 0 ELSE qty_milli END) AS qty,count(*) AS n
 FROM stock_movements WHERE business_id=$1 AND branch_id=$2 AND created_at >= $3 AND created_at < $4 GROUP BY variant_id
 ) SELECT v.id::text,v.sku,p.name,v.base_unit,COALESCE(b.qty_milli,0)::text,m.minimum_qty_milli::text,
 COALESCE(b.qty_milli,0)-COALESCE(b.reserved_milli,0)<=m.minimum_qty_milli,COALESCE(b.qty_milli,0)<0,
 CASE WHEN $5 THEN b.average_cost_cents::text END,
 CASE WHEN $5 AND b.average_cost_cents IS NOT NULL THEN ROUND(COALESCE(b.qty_milli,0)::numeric*b.average_cost_cents/1000)::text END,
 b.average_cost_cents IS NULL,COALESCE(t.qty,0)::text,COALESCE(t.n,0),COALESCE(b.reserved_milli,0)::text,(COALESCE(b.qty_milli,0)-COALESCE(b.reserved_milli,0))::text
 FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id
 LEFT JOIN inventory_balances b ON b.business_id=v.business_id AND b.variant_id=v.id AND b.branch_id=$2
 LEFT JOIN stock_minimums m ON m.business_id=v.business_id AND m.variant_id=v.id AND m.branch_id=$2
 LEFT JOIN movements t ON t.variant_id=v.id
 WHERE v.business_id=$1 AND p.track_stock
 AND (v.status<>'archived' OR COALESCE(b.qty_milli,0)<>0 OR COALESCE(t.n,0)>0)
 AND ($6='all' OR ($6='below_minimum' AND COALESCE(b.qty_milli,0)-COALESCE(b.reserved_milli,0)<=m.minimum_qty_milli) OR ($6='negative' AND COALESCE(b.qty_milli,0)<0))
 ORDER BY v.id LIMIT NULLIF($7,0)`, f.BusinessID, f.BranchID, f.From, f.Until, canViewCost, filter, reportLimit(f))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StockRow{}
	for rows.Next() {
		var x StockRow
		var below *bool
		if err := rows.Scan(&x.VariantID, &x.SKU, &x.ProductName, &x.Unit, &x.Balance, &x.Minimum, &below, &x.Negative, &x.AverageCost, &x.EstimatedValue, &x.CostUnknown, &x.Movement, &x.MovementCount, &x.Reserved, &x.Available); err != nil {
			return nil, err
		}
		if below != nil {
			x.BelowMinimum = *below
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

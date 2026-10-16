package reporting

import (
	"context"
	"github.com/storepos/storepos/internal/platform/database"
)

type ProductRow struct {
	ProductID         string  `json:"product_id"`
	CategoryID        *string `json:"category_id"`
	Name              string  `json:"product_name"`
	Sold              string  `json:"sold_base_qty_milli"`
	Returned          string  `json:"returned_base_qty_milli"`
	Net               *string `json:"net_sales_cents"`
	Cost              *string `json:"sold_cost_cents"`
	RestockCost       *string `json:"restock_cost_cents"`
	Profit            *string `json:"estimated_gross_profit_cents"`
	MissingCost       bool    `json:"missing_cost"`
	IncompleteRevenue bool    `json:"incomplete_revenue"`
}

func Products(ctx context.Context, db database.Getter, f PaymentFilter, profit bool) ([]ProductRow, error) {
	rows, err := db.Query(ctx, `WITH events AS (
 SELECT l.product_id,l.category_id,l.product_name,l.base_qty_milli AS sold,0::numeric AS returned,
 l.line_total_cents-l.tax_cents-l.service_charge_cents AS revenue,l.cost_cents AS cost,0::bigint AS restock,
 l.cost_cents IS NULL AS missing,false AS incomplete
 FROM sale_lines l JOIN sales s ON s.business_id=l.business_id AND s.id=l.sale_id
 WHERE s.business_id=$1 AND s.branch_id=$2 AND s.status='completed' AND s.completed_at >= $3 AND s.completed_at < $4 AND ($5::uuid IS NULL OR s.created_by=$5)
 UNION ALL
 SELECT l.product_id,l.category_id,l.product_name,0,r.qty_milli::numeric*l.factor_milli/1000,-r.net_cents,0,r.restock_cost_cents,
 r.restock_cost_cents IS NULL,r.net_cents IS NULL
 FROM sale_return_lines r JOIN sale_returns h ON h.business_id=r.business_id AND h.id=r.return_id
 JOIN sale_lines l ON l.business_id=r.business_id AND l.id=r.sale_line_id
 WHERE h.business_id=$1 AND h.branch_id=$2 AND h.created_at >= $3 AND h.created_at < $4 AND ($5::uuid IS NULL OR h.created_by=$5)
 ) SELECT product_id::text,category_id::text,product_name,SUM(sold)::text,ROUND(SUM(returned))::text,
 CASE WHEN NOT BOOL_OR(incomplete) THEN SUM(revenue)::text END,
 CASE WHEN $6 AND NOT BOOL_OR(missing) THEN SUM(cost)::text END,
 CASE WHEN $6 AND NOT BOOL_OR(missing) THEN SUM(restock)::text END,
 CASE WHEN $6 AND NOT BOOL_OR(missing) AND NOT BOOL_OR(incomplete) THEN (SUM(revenue)-SUM(cost)+SUM(restock))::text END,
 BOOL_OR(missing),BOOL_OR(incomplete)
 FROM events GROUP BY product_id,category_id,product_name ORDER BY product_id,category_id,product_name LIMIT NULLIF($7,0)`, f.BusinessID, f.BranchID, f.From, f.Until, f.UserID, profit, reportLimit(f))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProductRow{}
	for rows.Next() {
		var p ProductRow
		if err := rows.Scan(&p.ProductID, &p.CategoryID, &p.Name, &p.Sold, &p.Returned, &p.Net, &p.Cost, &p.RestockCost, &p.Profit, &p.MissingCost, &p.IncompleteRevenue); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

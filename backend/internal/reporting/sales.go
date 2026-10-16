package reporting

import (
	"context"
	"github.com/storepos/storepos/internal/platform/database"
)

type SalesRow struct {
	Date              string  `json:"date"`
	Hour              int     `json:"hour"`
	UserID            string  `json:"user_id"`
	Gross             string  `json:"gross_cents"`
	Discount          string  `json:"discount_cents"`
	Returns           *string `json:"returns_cents"`
	Net               *string `json:"net_cents"`
	Tax               *string `json:"tax_cents"`
	ServiceCharge     *string `json:"service_charge_cents"`
	DeliveryFee       *string `json:"delivery_fee_cents"`
	BillNet           *string `json:"net_bill_cents"`
	Rounding          *string `json:"cash_rounding_cents"`
	SaleCount         int64   `json:"sale_count"`
	Average           string  `json:"average_sale_cents"`
	IncompleteReturns int64   `json:"incomplete_return_count"`
}

// Sales uses sale completion date and return document date in branch time. Voids are excluded.
// Unknown historical return components yield NULL, not a misleading zero.
func Sales(ctx context.Context, db database.Getter, f PaymentFilter) ([]SalesRow, error) {
	rows, err := db.Query(ctx, `WITH line_totals AS (
 SELECT s.id,s.business_id,s.completed_at,s.created_by,s.cash_rounding_cents,s.total_cents,s.delivery_fee_cents,
 SUM(l.gross_cents-CASE WHEN l.tax_mode='inclusive' THEN l.tax_cents ELSE 0 END) AS gross,
 SUM(l.line_discount_cents+l.alloc_discount_cents) AS discount,SUM(l.tax_cents) AS tax,SUM(l.service_charge_cents) AS sc
 FROM sales s JOIN sale_lines l ON l.business_id=s.business_id AND l.sale_id=s.id
 WHERE s.business_id=$1 AND s.branch_id=$2 AND s.status='completed' AND s.completed_at >= $3 AND s.completed_at < $4
 AND ($5::uuid IS NULL OR s.created_by=$5) GROUP BY s.id
 ), returned AS (
 SELECT r.id,r.created_at,r.created_by,r.cash_rounding_cents,r.delivery_fee_cents,SUM(l.net_cents) AS net,SUM(l.tax_cents) AS tax,SUM(l.service_charge_cents) AS sc,
 BOOL_OR(l.net_cents IS NULL OR l.tax_cents IS NULL OR l.service_charge_cents IS NULL OR r.cash_rounding_cents IS NULL OR r.delivery_fee_cents IS NULL) AS incomplete
 FROM sale_returns r JOIN sale_return_lines l ON l.business_id=r.business_id AND l.return_id=r.id
 WHERE r.business_id=$1 AND r.branch_id=$2 AND r.created_at >= $3 AND r.created_at < $4 AND ($5::uuid IS NULL OR r.created_by=$5) GROUP BY r.id
 ), events AS (
 SELECT completed_at AS at,created_by AS actor,gross,discount,0::numeric AS returns,tax,sc,cash_rounding_cents AS rounding,delivery_fee_cents AS delivery,1::bigint AS count,total_cents AS total,0::bigint AS incomplete FROM line_totals
 UNION ALL
 SELECT created_at,created_by,0,0,net,-tax,-sc,-cash_rounding_cents,-delivery_fee_cents,0,0,CASE WHEN incomplete THEN 1 ELSE 0 END FROM returned
 ) SELECT (at AT TIME ZONE $6)::date::text,EXTRACT(HOUR FROM at AT TIME ZONE $6)::int,actor::text,
 SUM(gross)::text,SUM(discount)::text,
 CASE WHEN SUM(incomplete)=0 THEN SUM(returns)::text END,
 CASE WHEN SUM(incomplete)=0 THEN (SUM(gross)-SUM(discount)-SUM(returns))::text END,
 CASE WHEN SUM(incomplete)=0 THEN SUM(tax)::text END,
 CASE WHEN SUM(incomplete)=0 THEN SUM(sc)::text END,
 CASE WHEN SUM(incomplete)=0 THEN SUM(delivery)::text END,
 CASE WHEN SUM(incomplete)=0 THEN (SUM(gross)-SUM(discount)-SUM(returns)+SUM(tax)+SUM(sc)+SUM(delivery)+SUM(rounding))::text END,
 CASE WHEN SUM(incomplete)=0 THEN SUM(rounding)::text END,
 SUM(count)::bigint,CASE WHEN SUM(count)>0 THEN ROUND(SUM(total)/SUM(count))::text ELSE '0' END,SUM(incomplete)::bigint
 FROM events GROUP BY 1,2,3 ORDER BY 1,2,3 LIMIT NULLIF($7,0)`, f.BusinessID, f.BranchID, f.From, f.Until, f.UserID, f.Timezone, reportLimit(f))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SalesRow{}
	for rows.Next() {
		var x SalesRow
		if err := rows.Scan(&x.Date, &x.Hour, &x.UserID, &x.Gross, &x.Discount, &x.Returns, &x.Net, &x.Tax, &x.ServiceCharge, &x.DeliveryFee, &x.BillNet, &x.Rounding, &x.SaleCount, &x.Average, &x.IncompleteReturns); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

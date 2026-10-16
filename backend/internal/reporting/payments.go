package reporting

import (
	"context"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"time"
)

type PaymentFilter struct {
	Export               bool // background query has no row-count truncation; artifact size is bounded separately.
	BusinessID, BranchID idkit.ID
	UserID               *idkit.ID
	From, Until          time.Time
	Timezone             string
}
type PaymentRow struct {
	Date         string `json:"date"`
	Hour         int    `json:"hour"`
	UserID       string `json:"user_id"`
	Method       string `json:"method"`
	Verification string `json:"verification"`
	Received     string `json:"received_cents"`
	Refunded     string `json:"refunded_cents"`
	Net          string `json:"net_cents"`
}

// Payment receipts are already net of cash change. Refunds use their own document date,
// not the original sale date, and are grouped under the actor issuing the refund.
func Payments(ctx context.Context, db database.Getter, f PaymentFilter) ([]PaymentRow, error) {
	rows, err := db.Query(ctx, `WITH events AS (
 SELECT p.created_at AS event_at,s.created_by AS actor,p.method,p.verification,p.amount_cents AS received,0::bigint AS refunded
 FROM payments p JOIN sales s ON s.business_id=p.business_id AND s.id=p.sale_id
 WHERE s.business_id=$1 AND s.branch_id=$2 AND s.status='completed' AND p.status='confirmed'
 AND p.created_at >= $3 AND p.created_at < $4 AND ($5::uuid IS NULL OR s.created_by=$5)
 UNION ALL
 SELECT r.created_at,r.created_by,f.method,'MANUAL',0,f.amount_cents
 FROM refunds f JOIN sale_returns r ON r.business_id=f.business_id AND r.id=f.return_id
 WHERE r.business_id=$1 AND r.branch_id=$2 AND r.created_at >= $3 AND r.created_at < $4 AND ($5::uuid IS NULL OR r.created_by=$5)
 ) SELECT (event_at AT TIME ZONE $6)::date::text,EXTRACT(HOUR FROM event_at AT TIME ZONE $6)::int,
 actor::text,method,verification,SUM(received)::text,SUM(refunded)::text,(SUM(received)-SUM(refunded))::text
 FROM events GROUP BY 1,2,3,4,5 ORDER BY 1,2,3,4,5 LIMIT NULLIF($7,0)`, f.BusinessID, f.BranchID, f.From, f.Until, f.UserID, f.Timezone, reportLimit(f))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PaymentRow{}
	for rows.Next() {
		var p PaymentRow
		if err := rows.Scan(&p.Date, &p.Hour, &p.UserID, &p.Method, &p.Verification, &p.Received, &p.Refunded, &p.Net); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

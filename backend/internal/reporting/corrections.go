package reporting

import (
	"context"
	"github.com/storepos/storepos/internal/platform/database"
	"time"
)

type CorrectionRow struct {
	Kind                string    `json:"kind"`
	DocumentID          string    `json:"document_id"`
	SaleID              string    `json:"sale_id"`
	Receipt             string    `json:"receipt_number"`
	At                  time.Time `json:"occurred_at"`
	LocalDate           string    `json:"local_date"`
	Actor               string    `json:"actor_user_id"`
	Approver            *string   `json:"approved_by_user_id"`
	Reason              string    `json:"reason"`
	LineDiscount        string    `json:"line_discount_cents"`
	TransactionDiscount string    `json:"transaction_discount_cents"`
	Amount              string    `json:"amount_cents"`
}

// Each event uses its own timestamp/actor. Discount rows on voided sales are retained
// for accountability, not counted as net sales. Amounts must not be added across kinds.
func Corrections(ctx context.Context, db database.Getter, f PaymentFilter) ([]CorrectionRow, error) {
	rows, err := db.Query(ctx, `WITH events AS (
 SELECT 'return' AS kind,r.id AS document_id,r.sale_id,s.receipt_number,r.created_at AS at,r.created_by AS actor,
 r.approved_by,r.reason,0::bigint AS line_discount,0::bigint AS tx_discount,r.amount_cents AS amount
 FROM sale_returns r JOIN sales s ON s.business_id=r.business_id AND s.id=r.sale_id
 WHERE r.business_id=$1 AND r.branch_id=$2
 UNION ALL
 SELECT 'void',s.id,s.id,s.receipt_number,s.voided_at,s.voided_by,s.void_approved_by,COALESCE(s.void_reason,''),0,0,s.total_cents
 FROM sales s WHERE s.business_id=$1 AND s.branch_id=$2 AND s.status='voided'
 UNION ALL
 SELECT 'discount',s.id,s.id,s.receipt_number,s.completed_at,s.created_by,s.discount_approved_by,COALESCE(s.discount_reason,''),s.line_discount_cents,s.tx_discount_cents,s.line_discount_cents+s.tx_discount_cents
 FROM sales s WHERE s.business_id=$1 AND s.branch_id=$2 AND (s.line_discount_cents>0 OR s.tx_discount_cents>0)
 ) SELECT kind,document_id::text,sale_id::text,receipt_number,at,(at AT TIME ZONE $6)::date::text,
 actor::text,approved_by::text,reason,line_discount::text,tx_discount::text,amount::text
 FROM events WHERE at >= $3 AND at < $4 AND ($5::uuid IS NULL OR actor=$5)
 ORDER BY at,kind,document_id LIMIT NULLIF($7,0)`, f.BusinessID, f.BranchID, f.From, f.Until, f.UserID, f.Timezone, reportLimit(f))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CorrectionRow{}
	for rows.Next() {
		var x CorrectionRow
		if err := rows.Scan(&x.Kind, &x.DocumentID, &x.SaleID, &x.Receipt, &x.At, &x.LocalDate, &x.Actor, &x.Approver, &x.Reason, &x.LineDiscount, &x.TransactionDiscount, &x.Amount); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

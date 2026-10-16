package reporting

import (
	"context"
	"encoding/json"
	"github.com/storepos/storepos/internal/platform/database"
	"time"
)

type KitchenCancellation struct {
	LineID           string          `json:"line_id"`
	OrderID          string          `json:"order_id"`
	OrderType        string          `json:"order_type"`
	ProductName      string          `json:"product_name"`
	VariantName      string          `json:"variant_name"`
	Qty              string          `json:"qty_milli"`
	Unit             string          `json:"base_unit"`
	ModifierSnapshot json.RawMessage `json:"modifiers"`
	CancelledFrom    string          `json:"cancelled_from_status"`
	PotentialWaste   bool            `json:"potential_waste"`
	Reason           string          `json:"reason"`
	Actor            string          `json:"actor_user_id"`
	Approver         *string         `json:"approved_by_user_id"`
	At               time.Time       `json:"cancelled_at"`
	LocalDate        string          `json:"local_date"`
	MenuValue        string          `json:"menu_value_cents"`
}

// Menu value is estimated at snapshotted selling price, never labeled financial loss/HPP.
func KitchenCancellations(ctx context.Context, db database.Getter, f PaymentFilter) ([]KitchenCancellation, error) {
	rows, err := db.Query(ctx, `SELECT l.id::text,l.order_id::text,o.order_type,l.product_name,l.variant_name,l.qty_milli::text,l.base_unit,l.modifiers,
 l.cancelled_from_status,l.cancelled_from_status IN ('IN_PROGRESS','READY','SERVED'),l.cancel_reason,l.cancelled_by::text,l.cancel_approved_by::text,l.cancelled_at,
 (l.cancelled_at AT TIME ZONE $6)::date::text,(ROUND((l.unit_price_cents::numeric+l.modifier_delta_cents)*l.qty_milli/100000)*100)::text
 FROM restaurant_order_lines l JOIN restaurant_orders o ON o.business_id=l.business_id AND o.id=l.order_id
 WHERE l.business_id=$1 AND o.branch_id=$2 AND l.kitchen_status='CANCELLED' AND l.cancelled_at >= $3 AND l.cancelled_at < $4
 AND ($5::uuid IS NULL OR l.cancelled_by=$5) ORDER BY l.cancelled_at,l.id LIMIT NULLIF($7,0)`, f.BusinessID, f.BranchID, f.From, f.Until, f.UserID, f.Timezone, reportLimit(f))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []KitchenCancellation{}
	for rows.Next() {
		var x KitchenCancellation
		if err := rows.Scan(&x.LineID, &x.OrderID, &x.OrderType, &x.ProductName, &x.VariantName, &x.Qty, &x.Unit, &x.ModifierSnapshot, &x.CancelledFrom, &x.PotentialWaste, &x.Reason, &x.Actor, &x.Approver, &x.At, &x.LocalDate, &x.MenuValue); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

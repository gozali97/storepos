package sales

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/inventory"
	"github.com/storepos/storepos/internal/org"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/purchasing"
)

var ErrReturnQuantity = errors.New("jumlah retur melebihi sisa terjual atau baris tidak valid")
var ErrReturnState = errors.New("transaksi voided atau sudah diretur penuh tidak dapat diretur")
var ErrReturnMethod = errors.New("transaksi tidak memiliki pembayaran yang dapat direfund")

type ReturnLine struct {
	LineID    idkit.ID
	Qty       int64
	Condition string
}
type ReturnInput struct {
	BusinessID, SaleID, UserID, RegisterID idkit.ID
	ApprovedBy                             *idkit.ID
	Reason                                 string
	Lines                                  []ReturnLine
	HasBranch                              func(idkit.ID) bool
}
type ReturnResult struct {
	ID      string       `json:"return_id"`
	Amount  string       `json:"amount_cents"`
	Manual  bool         `json:"manual_refund_required"`
	Refunds []RefundPart `json:"refunds"`
}

// RefundSlice rounds to whole rupiah. The final quantity receives the exact remaining value.
func RefundSlice(value, sold, returned, refunded, qty int64) (int64, error) {
	if sold <= 0 || qty <= 0 || returned < 0 || returned > sold || qty > sold-returned || value < 0 || refunded < 0 || refunded > value {
		return 0, ErrReturnQuantity
	}
	if qty == sold-returned {
		return value - refunded, nil
	}
	n := new(big.Int).Mul(big.NewInt(value), big.NewInt(qty))
	den := new(big.Int).Mul(big.NewInt(sold), big.NewInt(100))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, den, r)
	if new(big.Int).Lsh(r, 1).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	q.Mul(q, big.NewInt(100))
	amount := q.Int64()
	if amount > value-refunded {
		amount = value - refunded
	}
	return amount, nil
}

// Return runs atomically in the caller transaction. Refund uses the original payment method.
// Refund split tender dialokasikan per metode asal (AllocateRefund); retur apotek masuk karantina batch asal.
func Return(ctx context.Context, db database.Getter, in ReturnInput) (*ReturnResult, error) {
	if strings.TrimSpace(in.Reason) == "" {
		return nil, ErrReasonRequired
	}
	if len(in.Lines) == 0 {
		return nil, ErrReturnQuantity
	}
	var branch idkit.ID
	err := db.QueryRow(ctx, `SELECT branch_id FROM sales WHERE business_id=$1 AND id=$2`, in.BusinessID, in.SaleID).Scan(&branch)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !in.HasBranch(branch) {
		return nil, ErrNotShiftOwner
	}
	var businessType string
	if err := db.QueryRow(ctx, `SELECT business_type FROM businesses WHERE id=$1`, in.BusinessID).Scan(&businessType); err != nil {
		return nil, err
	}
	policy, err := org.LoadPolicy(ctx, db, in.BusinessID)
	if err != nil {
		return nil, err
	}
	if policy.ReturnWindowDays > 0 {
		var sold, now time.Time
		var timezone string
		if err := db.QueryRow(ctx, `SELECT s.completed_at,b.timezone,clock_timestamp() FROM sales s JOIN branches b ON b.business_id=s.business_id AND b.id=s.branch_id WHERE s.business_id=$1 AND s.id=$2`, in.BusinessID, in.SaleID).Scan(&sold, &timezone, &now); err != nil {
			return nil, err
		}
		loc, err := time.LoadLocation(timezone)
		if err != nil {
			return nil, err
		}
		if !WithinReturnWindow(sold, now, loc, policy.ReturnWindowDays) {
			return nil, ErrReturnWindow
		}
	}
	isPharmacy := businessType == "pharmacy"
	// Shift dikunci lebih dulu (urutan kunci sama dengan finalisasi) bila ada porsi tunai yang mungkin direfund.
	var hasCash bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM payments WHERE business_id=$1 AND sale_id=$2 AND method='cash')`, in.BusinessID, in.SaleID).Scan(&hasCash); err != nil {
		return nil, err
	}
	var shift *idkit.ID
	if hasCash {
		id, _, _, err := lockOpenShift(ctx, db, in.BusinessID, branch, in.RegisterID, "FOR SHARE")
		if err != nil {
			return nil, err
		}
		shift = &id
	}
	var status, returnStatus string
	if err := db.QueryRow(ctx, `SELECT status,return_status FROM sales WHERE business_id=$1 AND id=$2 FOR UPDATE`, in.BusinessID, in.SaleID).Scan(&status, &returnStatus); err != nil {
		return nil, err
	}
	if status != "completed" || returnStatus == "full" {
		return nil, ErrReturnState
	}
	type line struct {
		ReturnLine
		variant           idkit.ID
		amount            int64
		tracked           bool
		baseQty           int64
		net, tax, sc      int64
		cost, restockCost *int64
	}
	lines := make([]line, 0, len(in.Lines))
	seen := map[idkit.ID]bool{}
	var total int64
	for _, l := range in.Lines {
		if seen[l.LineID] || (l.Condition != "restock" && l.Condition != "damaged") {
			return nil, ErrReturnQuantity
		}
		seen[l.LineID] = true
		var variant idkit.ID
		var sold, value, returned, refunded, factor int64
		var fractional bool
		err := db.QueryRow(ctx, `SELECT l.variant_id,l.qty_milli,l.line_total_cents,l.allow_fractional,l.factor_milli,
  COALESCE((SELECT SUM(r.qty_milli) FROM sale_return_lines r WHERE r.business_id=l.business_id AND r.sale_line_id=l.id),0),
  COALESCE((SELECT SUM(r.amount_cents) FROM sale_return_lines r WHERE r.business_id=l.business_id AND r.sale_line_id=l.id),0)
  FROM sale_lines l WHERE l.business_id=$1 AND l.sale_id=$2 AND l.id=$3`, in.BusinessID, in.SaleID, l.LineID).Scan(&variant, &sold, &value, &fractional, &factor, &returned, &refunded)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnQuantity
		}
		if err != nil {
			return nil, err
		}
		if !fractional && l.Qty%1000 != 0 {
			return nil, ErrReturnQuantity
		}
		amount, err := RefundSlice(value, sold, returned, refunded, l.Qty)
		if err != nil {
			return nil, err
		}
		var tracked bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stock_movements WHERE business_id=$1 AND ref_id=$2 AND variant_id=$3 AND kind='SALE')
 OR EXISTS(SELECT 1 FROM sale_line_batches WHERE business_id=$1 AND sale_line_id=$4)`, in.BusinessID, in.SaleID, variant, l.LineID).Scan(&tracked); err != nil {
			return nil, err
		}
		total += amount
		base, err := purchasing.Convert(l.Qty, factor)
		if err != nil {
			return nil, ErrReturnQuantity
		}
		net, tax, sc, err := refundComponents(ctx, db, in.BusinessID, l.LineID, sold, returned, l.Qty, amount)
		if err != nil {
			return nil, err
		}
		var totalCost *int64
		var priorCost int64
		var missingCost bool
		if err := db.QueryRow(ctx, `SELECT cost_cents,COALESCE((SELECT SUM(returned_cost_cents) FROM sale_return_lines WHERE business_id=$1 AND sale_line_id=$2),0),EXISTS(SELECT 1 FROM sale_return_lines WHERE business_id=$1 AND sale_line_id=$2 AND returned_cost_cents IS NULL) FROM sale_lines WHERE business_id=$1 AND id=$2`, in.BusinessID, l.LineID).Scan(&totalCost, &priorCost, &missingCost); err != nil {
			return nil, err
		}
		var cost, restockCost *int64
		if totalCost != nil && !missingCost {
			value, err := CostSlice(*totalCost, sold, returned, priorCost, l.Qty)
			if err != nil {
				return nil, err
			}
			cost = &value
			if l.Condition == "restock" {
				restockCost = &value
			} else {
				zero := int64(0)
				restockCost = &zero
			}
		}
		lines = append(lines, line{l, variant, amount, tracked, base, net, tax, sc, cost, restockCost})
	}
	id := idkit.New()
	if _, err := db.Exec(ctx, `INSERT INTO sale_returns(id,business_id,sale_id,branch_id,reason,amount_cents,created_by,approved_by,cash_rounding_cents) VALUES($1,$2,$3,$4,$5,$6,$7,$8,0)`, id, in.BusinessID, in.SaleID, branch, strings.TrimSpace(in.Reason), total, in.UserID, in.ApprovedBy); err != nil {
		return nil, err
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].variant.String() < lines[j].variant.String() })
	for _, l := range lines {
		if _, err := db.Exec(ctx, `INSERT INTO sale_return_lines(business_id,return_id,sale_line_id,qty_milli,amount_cents,condition,net_cents,tax_cents,service_charge_cents,returned_cost_cents,restock_cost_cents) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, in.BusinessID, id, l.LineID, l.Qty, l.amount, l.Condition, l.net, l.tax, l.sc, l.cost, l.restockCost); err != nil {
			return nil, err
		}
		if l.tracked && isPharmacy {
			// Apotek: stok kembali lewat karantina batch asal, bukan langsung sellable (APT-010).
			if err := returnToBatches(ctx, db, in.BusinessID, branch, l.LineID, in.UserID, id, l.baseQty, l.Condition == "restock"); err != nil {
				return nil, err
			}
		} else if l.tracked {
			if l.Condition == "restock" {
				if err := restockWithCost(ctx, db, in.BusinessID, branch, l.variant, in.UserID, id, l.baseQty, l.restockCost); err != nil {
					return nil, err
				}
			} else {
				balance, err := inventory.Lock(ctx, db, in.BusinessID, branch, l.variant)
				if err != nil {
					return nil, err
				}
				if _, err := db.Exec(ctx, `INSERT INTO stock_movements(id,business_id,branch_id,variant_id,kind,qty_milli,balance_after_milli,ref_type,ref_id,created_by) VALUES($1,$2,$3,$4,'RETURN_DAMAGED',$5,$6,'return',$7,$8)`, idkit.New(), in.BusinessID, branch, l.variant, l.baseQty, balance, id, in.UserID); err != nil {
					return nil, err
				}
			}
		}
	}
	var full bool
	if err := db.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM sale_lines l WHERE l.business_id=$1 AND l.sale_id=$2 AND l.qty_milli>COALESCE((SELECT SUM(r.qty_milli) FROM sale_return_lines r WHERE r.business_id=l.business_id AND r.sale_line_id=l.id),0))`, in.BusinessID, in.SaleID).Scan(&full); err != nil {
		return nil, err
	}
	if full {
		// Cash rounding is returned only on the full-return document (§13.9).
		var rounding, deliveryFee int64
		if err := db.QueryRow(ctx, `SELECT cash_rounding_cents,delivery_fee_cents FROM sales WHERE business_id=$1 AND id=$2`, in.BusinessID, in.SaleID).Scan(&rounding, &deliveryFee); err != nil {
			return nil, err
		}
		// Ongkir bukan bagian baris barang; dikembalikan hanya pada retur penuh [REK], seperti pembulatan tunai.
		total += rounding + deliveryFee
		if total < 0 {
			return nil, ErrReturnQuantity
		}
		if _, err := db.Exec(ctx, `UPDATE sale_returns SET amount_cents=$3,cash_rounding_cents=$4,delivery_fee_cents=$5 WHERE business_id=$1 AND id=$2`, in.BusinessID, id, total, rounding, deliveryFee); err != nil {
			return nil, err
		}
	}
	var parts []RefundPart
	manual := false
	if total > 0 {
		left, err := refundable(ctx, db, in.BusinessID, in.SaleID)
		if err != nil {
			return nil, err
		}
		if len(left) == 0 {
			return nil, ErrReturnMethod
		}
		if parts, err = AllocateRefund(total, left); err != nil {
			return nil, err
		}
		for i := range parts {
			var refundShift *idkit.ID
			if parts[i].Method == "cash" {
				refundShift = shift
			}
			if _, err := db.Exec(ctx, `INSERT INTO refunds(id,business_id,return_id,method,amount_cents,shift_id,manual_refund_required) VALUES($1,$2,$3,$4,$5,$6,$7)`, idkit.New(), in.BusinessID, id, parts[i].Method, parts[i].Amount, refundShift, parts[i].Manual); err != nil {
				return nil, err
			}
			parts[i].Cents = strconv.FormatInt(parts[i].Amount, 10)
			manual = manual || parts[i].Manual
		}
	}
	rs := "partial"
	if full {
		rs = "full"
	}
	if _, err := db.Exec(ctx, `UPDATE sales SET return_status=$3 WHERE business_id=$1 AND id=$2`, in.BusinessID, in.SaleID, rs); err != nil {
		return nil, err
	}
	if err := audit.Write(ctx, db, audit.Entry{BusinessID: in.BusinessID, BranchID: &branch, ActorUserID: in.UserID, ApprovedBy: in.ApprovedBy, Action: "sale.returned", EntityType: "return", EntityID: id.String(), After: map[string]any{"sale_id": in.SaleID.String(), "amount_cents": total}, Reason: in.Reason}); err != nil {
		return nil, err
	}
	return &ReturnResult{ID: id.String(), Amount: strconv.FormatInt(total, 10), Manual: manual, Refunds: parts}, nil
}

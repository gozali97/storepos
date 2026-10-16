package sales

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

var ErrShiftStillOpen = errors.New("ringkasan kas hanya tersedia setelah shift ditutup")

// ShiftSummary hanya untuk shift tertutup agar tidak membocorkan kas diharapkan sebelum hitung buta.
type ShiftSummary struct {
	RefundCash          string           `json:"cash_refund_cents"`
	VoidCount           int              `json:"void_count"`
	ID                  string           `json:"shift_id"`
	BranchID            string           `json:"branch_id"`
	Opening             string           `json:"opening_cash_cents"`
	Expected            string           `json:"expected_cash_cents"`
	Counted             string           `json:"counted_cash_cents"`
	Variance            string           `json:"variance_cents"`
	CashIn              string           `json:"cash_in_cents"`
	CashOut             string           `json:"cash_out_cents"`
	LineDiscount        string           `json:"line_discount_cents"`
	TransactionDiscount string           `json:"transaction_discount_cents"`
	SaleCount           int              `json:"sale_count"`
	Payments            []PaymentSummary `json:"payments"`
}

type PaymentSummary struct {
	Method       string `json:"method"`
	Verification string `json:"verification"`
	Amount       string `json:"amount_cents"`
	Tendered     string `json:"tendered_cents"`
	Change       string `json:"change_cents"`
}

// Summary memeriksa tenant, cabang, pelaku dan status sebelum mengembalikan angka kas.
func Summary(ctx context.Context, db database.Getter, biz, shift, user idkit.ID, canViewAll bool, hasBranch func(idkit.ID) bool) (*ShiftSummary, error) {
	var branch, owner idkit.ID
	var status string
	var opening int64
	var expected, counted, variance *int64
	err := db.QueryRow(ctx, `SELECT branch_id,opened_by,status,opening_cash_cents,expected_cash_cents,counted_cash_cents,variance_cents
 FROM shifts WHERE business_id=$1 AND id=$2`, biz, shift).Scan(&branch, &owner, &status, &opening, &expected, &counted, &variance)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !hasBranch(branch) || (owner != user && !canViewAll) {
		return nil, ErrNotShiftOwner
	}
	if status != "closed" {
		return nil, ErrShiftStillOpen
	}
	if expected == nil || counted == nil || variance == nil {
		return nil, errors.New("ringkasan shift tertutup belum lengkap")
	}
	var cashIn, cashOut, ld, td int64
	var n int
	err = db.QueryRow(ctx, `SELECT
 COALESCE((SELECT SUM(amount_cents) FROM cash_movements WHERE business_id=$1 AND shift_id=$2 AND kind='in'),0),
 COALESCE((SELECT SUM(amount_cents) FROM cash_movements WHERE business_id=$1 AND shift_id=$2 AND kind='out'),0),
 COALESCE((SELECT SUM(line_discount_cents) FROM sales WHERE business_id=$1 AND shift_id=$2 AND status='completed'),0),
 COALESCE((SELECT SUM(tx_discount_cents) FROM sales WHERE business_id=$1 AND shift_id=$2 AND status='completed'),0),
 (SELECT count(*) FROM sales WHERE business_id=$1 AND shift_id=$2 AND status='completed')`, biz, shift).Scan(&cashIn, &cashOut, &ld, &td, &n)
	if err != nil {
		return nil, err
	}
	f := func(v int64) string { return strconv.FormatInt(v, 10) }
	out := &ShiftSummary{ID: shift.String(), BranchID: branch.String(), Opening: f(opening), Expected: f(*expected), Counted: f(*counted), Variance: f(*variance), CashIn: f(cashIn), CashOut: f(cashOut), LineDiscount: f(ld), TransactionDiscount: f(td), SaleCount: n, Payments: []PaymentSummary{}}
	var refunded int64
	if err := db.QueryRow(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM refunds WHERE business_id=$1 AND shift_id=$2 AND method='cash'`, biz, shift).Scan(&refunded); err != nil {
		return nil, err
	}
	out.RefundCash = f(refunded)
	if err := db.QueryRow(ctx, `SELECT count(*) FROM sales WHERE business_id=$1 AND shift_id=$2 AND status='voided'`, biz, shift).Scan(&out.VoidCount); err != nil {
		return nil, err
	}
	rows, err := db.Query(ctx, `SELECT p.method,p.verification,SUM(p.amount_cents),SUM(p.tendered_cents),SUM(p.change_cents)
 FROM payments p JOIN sales s ON s.business_id=p.business_id AND s.id=p.sale_id
 WHERE s.business_id=$1 AND s.shift_id=$2 AND s.status='completed' AND p.status='confirmed' GROUP BY p.method,p.verification ORDER BY p.method,p.verification`, biz, shift)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p PaymentSummary
		var amount, tendered, change int64
		if err := rows.Scan(&p.Method, &p.Verification, &amount, &tendered, &change); err != nil {
			return nil, err
		}
		p.Amount, p.Tendered, p.Change = f(amount), f(tendered), f(change)
		out.Payments = append(out.Payments, p)
	}
	return out, rows.Err()
}

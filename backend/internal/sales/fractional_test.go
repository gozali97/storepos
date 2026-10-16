package sales

import (
	"context"
	"errors"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"testing"
)

func TestFractionalSalesAndReturnSnapshot(t *testing.T) {
	e := setup(t, 10)
	ctx := context.Background()
	db := e.pool.Underlying()
	sell := func(qty int64) (*Result, error) {
		var r *Result
		err := e.inTx(func(ctx context.Context, tx database.Tx) error {
			var err error
			r, err = Finalize(ctx, tx, Input{BusinessID: e.biz, BranchID: e.branch, RegisterID: e.register, UserID: e.user, Lines: []LineIn{{VariantID: e.variant, QtyMilli: qty}}, Payments: []PaymentIn{{Method: "qris", AmountCents: 525000}}})
			return err
		})
		return r, err
	}
	if _, err := sell(375); !errors.Is(err, ErrInvalidQty) {
		t.Fatalf("fractional rejected by default: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE variants SET allow_fractional=true,price_cents=1400000 WHERE id=$1`, e.variant); err != nil {
		t.Fatal(err)
	}
	r, err := sell(375)
	if err != nil || r.TotalCents != 525000 {
		t.Fatalf("0.375 kg @14000: %+v %v", r, err)
	}
	if _, err := db.Exec(ctx, `UPDATE variants SET allow_fractional=false WHERE id=$1`, e.variant); err != nil {
		t.Fatal(err)
	}
	var line idkit.ID
	if err := db.QueryRow(ctx, `SELECT id FROM sale_lines WHERE sale_id=$1`, r.SaleID).Scan(&line); err != nil {
		t.Fatal(err)
	}
	if err := e.inTx(func(ctx context.Context, tx database.Tx) error {
		_, err := Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: r.SaleID, UserID: e.user, Reason: "snapshot", Lines: []ReturnLine{{LineID: line, Qty: 125, Condition: "restock"}}, HasBranch: func(idkit.ID) bool { return true }})
		return err
	}); err != nil {
		t.Fatalf("historical fractional return: %v", err)
	}
}

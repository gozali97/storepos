package sales

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

func TestRefundSliceRemainder(t *testing.T) {
	refunded := int64(0)
	for i, want := range []int64{300, 300, 400} {
		got, err := RefundSlice(1000, 3000, int64(i)*1000, refunded, 1000)
		if err != nil || got != want {
			t.Fatalf("slice %d: %d %v", i, got, err)
		}
		refunded += got
	}
	if _, err := RefundSlice(1000, 3000, 3000, 1000, 1); err == nil {
		t.Fatal("over-return accepted")
	}
}

func TestReturnsPartialFinalAndCashReconciliation(t *testing.T) {
	e := setup(t, 5)
	ctx := context.Background()
	r, err := e.sell(2, 3000000)
	if err != nil {
		t.Fatal(err)
	}
	var line idkit.ID
	if err := e.pool.Underlying().QueryRow(ctx, `SELECT id FROM sale_lines WHERE sale_id=$1`, r.SaleID).Scan(&line); err != nil {
		t.Fatal(err)
	}
	run := func(condition string) (*ReturnResult, error) {
		var out *ReturnResult
		err := e.inTx(func(ctx context.Context, tx database.Tx) error {
			var err error
			out, err = Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: r.SaleID, UserID: e.user, RegisterID: e.register, Reason: "uji", Lines: []ReturnLine{{LineID: line, Qty: 1000, Condition: condition}}, HasBranch: func(idkit.ID) bool { return true }})
			return err
		})
		return out, err
	}
	for _, condition := range []string{"restock", "damaged"} {
		out, err := run(condition)
		if err != nil || out.Amount != "1500000" {
			t.Fatalf("refund: %+v %v", out, err)
		}
	}
	if _, err := run("restock"); !errors.Is(err, ErrReturnState) {
		t.Fatalf("over-return: %v", err)
	}
	if qty := e.count(`SELECT qty_milli FROM inventory_balances`); qty != 4000 {
		t.Fatalf("damaged increased stock: %d", qty)
	}
	if n := e.count(`SELECT count(*) FROM stock_movements WHERE kind='RETURN_DAMAGED'`); n != 1 {
		t.Fatalf("damaged log: %d", n)
	}
	if n := e.count(`SELECT SUM(amount_cents) FROM refunds`); n != 3000000 {
		t.Fatalf("refund total: %d", n)
	}
	closed, err := e.closeShift(false, 0, "", nil)
	if err != nil || closed.ExpectedCents != 0 {
		t.Fatalf("cash reconciliation: %+v %v", closed, err)
	}
}

func TestConcurrentReturnLastQuantity(t *testing.T) {
	e := setup(t, 2)
	r, err := e.sell(1, 1500000)
	if err != nil {
		t.Fatal(err)
	}
	var line idkit.ID
	if err := e.pool.Underlying().QueryRow(context.Background(), `SELECT id FROM sale_lines WHERE sale_id=$1`, r.SaleID).Scan(&line); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- e.inTx(func(ctx context.Context, tx database.Tx) error {
				_, err := Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: r.SaleID, UserID: e.user, RegisterID: e.register, Reason: "uji", Lines: []ReturnLine{{LineID: line, Qty: 1000, Condition: "restock"}}, HasBranch: func(idkit.ID) bool { return true }})
				return err
			})
		}()
	}
	wg.Wait()
	close(results)
	ok := 0
	for err := range results {
		if err == nil {
			ok++
		} else if !errors.Is(err, ErrReturnState) {
			t.Fatal(err)
		}
	}
	if ok != 1 || e.count(`SELECT count(*) FROM refunds`) != 1 {
		t.Fatal("double refund")
	}
}

package sales

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

func TestVoidReversalConcurrentAndShiftReconciliation(t *testing.T) {
	e := setup(t, 5)
	ctx := context.Background()
	db := e.pool.Underlying()
	boss := idkit.New()
	if _, err := db.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,'boss@void.test','x')`, boss); err != nil {
		t.Fatal(err)
	}
	r, err := e.sell(1, 2000000)
	if err != nil {
		t.Fatal(err)
	}
	// Reverse historical movements even if the master is changed later.
	if _, err := db.Exec(ctx, `UPDATE products SET track_stock=false`); err != nil {
		t.Fatal(err)
	}
	call := func() error {
		return e.inTx(func(ctx context.Context, tx database.Tx) error {
			return Void(ctx, tx, e.biz, r.SaleID, e.user, boss, "salah input", func(idkit.ID) bool { return true })
		})
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- call() }()
	}
	wg.Wait()
	close(results)
	ok, rejected := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if errors.Is(err, ErrVoidState) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || rejected != 1 {
		t.Fatalf("double reversal: %d %d", ok, rejected)
	}
	if n := e.count(`SELECT qty_milli FROM inventory_balances`); n != 5000 {
		t.Fatalf("stock: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM payments WHERE status='reversed'`); n != 1 {
		t.Fatalf("payments: %d", n)
	}
	if n := e.count(`SELECT count(*) FROM stock_movements WHERE kind='SALE_VOID'`); n != 1 {
		t.Fatalf("movements: %d", n)
	}
	if n := e.count(`SELECT last_number FROM document_counters`); n != 1 {
		t.Fatalf("receipt reused: %d", n)
	}
	closed, err := e.closeShift(false, 0, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if closed.ExpectedCents != 0 {
		t.Fatalf("void cash still counted: %+v", closed)
	}
	var sid idkit.ID
	if err := db.QueryRow(ctx, `SELECT id FROM shifts`).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	summary, err := Summary(ctx, db, e.biz, sid, e.user, false, func(idkit.ID) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if summary.SaleCount != 0 || len(summary.Payments) != 0 {
		t.Fatalf("void included in sales summary: %+v", summary)
	}
}

func TestVoidClosedShiftAndMissingApproval(t *testing.T) {
	e := setup(t, 5)
	r, err := e.sell(1, 1500000)
	if err != nil {
		t.Fatal(err)
	}
	boss := idkit.New()
	if _, err := e.pool.Underlying().Exec(context.Background(), `INSERT INTO users(id,email,password_hash) VALUES($1,'boss@closed.test','x')`, boss); err != nil {
		t.Fatal(err)
	}
	call := func(biz, approver idkit.ID) error {
		return e.inTx(func(ctx context.Context, tx database.Tx) error {
			return Void(ctx, tx, biz, r.SaleID, e.user, approver, "salah", func(idkit.ID) bool { return true })
		})
	}
	if err := call(e.biz, idkit.ID{}); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("approval: %v", err)
	}
	if err := call(idkit.New(), boss); !errors.Is(err, ErrNotFound) {
		t.Fatalf("tenant: %v", err)
	}
	if _, err := e.closeShift(false, 1500000, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := call(e.biz, boss); !errors.Is(err, ErrVoidClosedShift) {
		t.Fatalf("closed: %v", err)
	}
	if n := e.count(`SELECT qty_milli FROM inventory_balances`); n != 4000 {
		t.Fatalf("rollback: %d", n)
	}
}

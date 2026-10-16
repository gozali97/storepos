package sales

import (
	"context"
	"errors"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"testing"
	"time"
)

func TestReturnWindowCalendarBoundary(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Makassar")
	if err != nil {
		t.Fatal(err)
	}
	sold := time.Date(2026, 10, 13, 15, 30, 0, 0, time.UTC) // 23:30 WITA
	last := time.Date(2026, 10, 14, 15, 59, 59, 0, time.UTC)
	if !WithinReturnWindow(sold, last, loc, 1) {
		t.Fatal("last local day rejected")
	}
	if WithinReturnWindow(sold, last.Add(time.Second), loc, 1) {
		t.Fatal("next local day accepted")
	}
	if !WithinReturnWindow(sold, last.AddDate(1, 0, 0), loc, 0) {
		t.Fatal("unlimited rejected")
	}
}
func TestExpiredReturnHasNoFinancialOrStockEffects(t *testing.T) {
	e := setup(t, 5)
	ctx := context.Background()
	db := e.pool.Underlying()
	sale, err := e.sell(1, 1500000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE businesses SET settings=jsonb_build_object('operational_policy',jsonb_build_object('return_window_days',1))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE sales SET completed_at=now()-interval '10 days' WHERE id=$1`, sale.SaleID); err != nil {
		t.Fatal(err)
	}
	var line idkit.ID
	if err := db.QueryRow(ctx, `SELECT id FROM sale_lines WHERE sale_id=$1`, sale.SaleID).Scan(&line); err != nil {
		t.Fatal(err)
	}
	err = e.inTx(func(ctx context.Context, tx database.Tx) error {
		_, err := Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: sale.SaleID, RegisterID: e.register, UserID: e.user, Reason: "expired", Lines: []ReturnLine{{LineID: line, Qty: 1000, Condition: "restock"}}, HasBranch: func(idkit.ID) bool { return true }})
		return err
	})
	if !errors.Is(err, ErrReturnWindow) {
		t.Fatal(err)
	}
	if e.count(`SELECT count(*) FROM refunds`) != 0 || e.count(`SELECT count(*) FROM sale_returns`) != 0 || e.count(`SELECT qty_milli FROM inventory_balances`) != 4000 {
		t.Fatal("expired return changed data")
	}
}

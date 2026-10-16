package sales

import (
	"context"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/reporting"
	"testing"
	"time"
)

func TestProductProfitUsesCostSnapshotAndFlagsMissing(t *testing.T) {
	e := setup(t, 10)
	ctx := context.Background()
	db := e.pool.Underlying()
	if _, err := db.Exec(ctx, `UPDATE inventory_balances SET average_cost_cents=500000`); err != nil {
		t.Fatal(err)
	}
	sale, err := e.sell(2, 3000000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE inventory_balances SET average_cost_cents=900000`); err != nil {
		t.Fatal(err)
	}
	var line idkit.ID
	if err := db.QueryRow(ctx, `SELECT id FROM sale_lines WHERE sale_id=$1`, sale.SaleID).Scan(&line); err != nil {
		t.Fatal(err)
	}
	err = e.inTx(func(ctx context.Context, tx database.Tx) error {
		_, err := Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: sale.SaleID, UserID: e.user, RegisterID: e.register, Reason: "retur", Lines: []ReturnLine{{LineID: line, Qty: 1000, Condition: "restock"}}, HasBranch: func(idkit.ID) bool { return true }})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var average int64
	if err := db.QueryRow(ctx, `SELECT average_cost_cents FROM inventory_balances`).Scan(&average); err != nil || average != 855556 {
		t.Fatalf("restock average: %d %v", average, err)
	}
	now := time.Now()
	f := reporting.PaymentFilter{BusinessID: e.biz, BranchID: e.branch, From: now.Add(-time.Hour), Until: now.Add(time.Hour), Timezone: "Asia/Jakarta"}
	rows, err := reporting.Products(ctx, db, f, true)
	if err != nil || len(rows) != 1 {
		t.Fatalf("report: %+v %v", rows, err)
	}
	p := rows[0]
	if p.Sold != "2000" || p.Returned != "1000" || *p.Net != "1500000" || *p.Cost != "1000000" || *p.RestockCost != "500000" || *p.Profit != "1000000" {
		t.Fatalf("snapshot profit: %+v", p)
	}
	rows, err = reporting.Products(ctx, db, f, false)
	if err != nil || rows[0].Profit != nil || rows[0].Cost != nil {
		t.Fatal("cost disclosure", rows, err)
	}
	if _, err := db.Exec(ctx, `UPDATE sale_lines SET cost_cents=NULL`); err != nil {
		t.Fatal(err)
	}
	rows, err = reporting.Products(ctx, db, f, true)
	if err != nil || !rows[0].MissingCost || rows[0].Profit != nil {
		t.Fatal("missing HPP treated as zero", rows, err)
	}
	f.BusinessID = idkit.New()
	rows, err = reporting.Products(ctx, db, f, true)
	if err != nil || len(rows) != 0 {
		t.Fatal("tenant", rows, err)
	}
}
func TestCostRoundingAndResidual(t *testing.T) {
	if got, err := SnapshotCost(1001, 500); err != nil || got != 501 {
		t.Fatal(got, err)
	}
	prior := int64(0)
	for i, want := range []int64{333, 333, 334} {
		got, err := CostSlice(1000, 3000, int64(i)*1000, prior, 1000)
		if err != nil || got != want {
			t.Fatal(got, err)
		}
		prior += got
	}
}

package sales

import (
	"context"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/reporting"
	"testing"
	"time"
)

func TestPaymentReportReconcilesShiftAndRefundDate(t *testing.T) {
	e := setup(t, 10)
	ctx := context.Background()
	db := e.pool.Underlying()
	r, err := e.sell(2, 4000000)
	if err != nil {
		t.Fatal(err)
	}
	var line idkit.ID
	if err := db.QueryRow(ctx, `SELECT id FROM sale_lines WHERE sale_id=$1`, r.SaleID).Scan(&line); err != nil {
		t.Fatal(err)
	}
	err = e.inTx(func(ctx context.Context, tx database.Tx) error {
		_, err := Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: r.SaleID, UserID: e.user, RegisterID: e.register, Reason: "uji", Lines: []ReturnLine{{LineID: line, Qty: 1000, Condition: "restock"}}, HasBranch: func(idkit.ID) bool { return true }})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE payments SET created_at='2026-10-12T17:30:00Z'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE sale_returns SET created_at='2026-10-13T17:30:00Z'`); err != nil {
		t.Fatal(err)
	}
	closed, err := e.closeShift(false, 1500000, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 10, 13, 0, 0, 0, 0, loc)
	filter := reporting.PaymentFilter{BusinessID: e.biz, BranchID: e.branch, From: from, Until: from.AddDate(0, 0, 2), Timezone: "Asia/Jakarta"}
	rows, err := reporting.Payments(ctx, db, filter)
	if err != nil || len(rows) != 2 {
		t.Fatalf("report: %+v %v", rows, err)
	}
	if rows[0].Date != "2026-10-13" || rows[0].Hour != 0 || rows[0].Received != "3000000" || rows[1].Date != "2026-10-14" || rows[1].Refunded != "1500000" {
		t.Fatal(rows)
	}
	if closed.ExpectedCents != 1500000 {
		t.Fatalf("shift net differs: %+v", closed)
	}
	filter.BusinessID = idkit.New()
	rows, err = reporting.Payments(ctx, db, filter)
	if err != nil || len(rows) != 0 {
		t.Fatalf("tenant: %+v %v", rows, err)
	}
}

package sales

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/reporting"
)

func TestDeliveryFeeReportReconcilesPartialAndFullReturns(t *testing.T) {
	e := setup(t, 10)
	sale, err := e.sellWithFee(2, 1000000, 4000000)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	db := e.pool.Underlying()
	var line idkit.ID
	if err := db.QueryRow(ctx, `SELECT id FROM sale_lines WHERE sale_id=$1`, sale.SaleID).Scan(&line); err != nil {
		t.Fatal(err)
	}
	filter := reporting.PaymentFilter{BusinessID: e.biz, BranchID: e.branch, From: time.Now().Add(-time.Hour), Until: time.Now().Add(time.Hour), Timezone: "Asia/Jakarta"}
	assertTotals := func(wantFee, wantBill int64) {
		t.Helper()
		rows, err := reporting.Sales(ctx, db, filter)
		if err != nil {
			t.Fatal(err)
		}
		var fee, bill int64
		for _, r := range rows {
			if r.DeliveryFee == nil || r.BillNet == nil {
				t.Fatalf("missing new report components: %+v", r)
			}
			f, err := strconv.ParseInt(*r.DeliveryFee, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			b, err := strconv.ParseInt(*r.BillNet, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			fee += f
			bill += b
		}
		if fee != wantFee || bill != wantBill {
			t.Fatalf("fee=%d bill=%d want %d/%d", fee, bill, wantFee, wantBill)
		}
		paid, err := reporting.Payments(ctx, db, filter)
		if err != nil {
			t.Fatal(err)
		}
		var net int64
		for _, p := range paid {
			v, err := strconv.ParseInt(p.Net, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			net += v
		}
		if bill != net {
			t.Fatalf("net bill %d != net payments %d", bill, net)
		}
	}
	assertTotals(1000000, 4000000)
	for i := 0; i < 2; i++ {
		err := e.inTx(func(ctx context.Context, tx database.Tx) error {
			_, err := Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: sale.SaleID, UserID: e.user, RegisterID: e.register, Reason: "return", Lines: []ReturnLine{{LineID: line, Qty: 1000, Condition: "restock"}}, HasBranch: func(idkit.ID) bool { return true }})
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			assertTotals(1000000, 2500000)
		} else {
			assertTotals(0, 0)
		}
	}
	var refunds []int64
	rows, err := db.Query(ctx, `SELECT delivery_fee_cents FROM sale_returns WHERE sale_id=$1 ORDER BY created_at,id`, sale.SaleID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var n int64
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		refunds = append(refunds, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(refunds) != 2 || refunds[0] != 0 || refunds[1] != 1000000 {
		t.Fatalf("fee refund snapshots: %v", refunds)
	}
	// Jangan menyatakan laporan lengkap bila snapshot ongkir historis tidak diketahui.
	if _, err := db.Exec(ctx, `UPDATE sale_returns SET delivery_fee_cents=NULL WHERE sale_id=$1`, sale.SaleID); err != nil {
		t.Fatal(err)
	}
	reported, err := reporting.Sales(ctx, db, filter)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range reported {
		if r.DeliveryFee != nil || r.BillNet != nil || r.IncompleteReturns == 0 {
			t.Fatalf("historical unknown must remain incomplete: %+v", r)
		}
	}
}

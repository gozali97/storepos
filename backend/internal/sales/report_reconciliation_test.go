package sales

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/reporting"
)

func TestDailyReportReconcilesAllClosedShifts(t *testing.T) {
	e := setup(t, 20)
	ctx := context.Background()
	db := e.pool.Underlying()
	from := time.Date(2026, 10, 13, 0, 0, 0, 0, time.FixedZone("Jakarta", 7*3600))
	f := reporting.PaymentFilter{BusinessID: e.biz, BranchID: e.branch, From: from, Until: from.AddDate(0, 0, 1), Timezone: "Asia/Jakarta"}
	var totalShiftCash int64
	var totalCount int
	for i := 0; i < 3; i++ {
		if i > 0 {
			if _, err := OpenShift(ctx, db, e.biz, e.branch, e.register, e.user, 0); err != nil {
				t.Fatal(err)
			}
		}
		sale, err := e.sell(int64(i+1), 10000000)
		if err != nil {
			t.Fatal(err)
		}
		// First event occurs exactly at local start; remaining ones occur later in the same day.
		at := from.Add(time.Duration(i) * time.Hour)
		if _, err := db.Exec(ctx, `UPDATE sales SET completed_at=$2 WHERE id=$1`, sale.SaleID, at); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, `UPDATE payments SET created_at=$2 WHERE sale_id=$1`, sale.SaleID, at); err != nil {
			t.Fatal(err)
		}
		closed, err := e.closeShift(false, sale.TotalCents, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		var shift idkit.ID
		if err := db.QueryRow(ctx, `SELECT shift_id FROM sales WHERE id=$1`, sale.SaleID).Scan(&shift); err != nil {
			t.Fatal(err)
		}
		summary, err := Summary(ctx, db, e.biz, shift, e.user, false, func(idkit.ID) bool { return true })
		if err != nil {
			t.Fatal(err)
		}
		if summary.Expected != strconv.FormatInt(closed.ExpectedCents, 10) {
			t.Fatal("summary mismatch")
		}
		totalShiftCash += closed.ExpectedCents
		totalCount += summary.SaleCount
	}
	rows, err := reporting.Sales(ctx, db, f)
	if err != nil {
		t.Fatal(err)
	}
	var net int64
	var count int64
	for _, r := range rows {
		if r.Net == nil {
			t.Fatal("incomplete report")
		}
		v, err := strconv.ParseInt(*r.Net, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		net += v
		count += r.SaleCount
	}
	if net != totalShiftCash || count != int64(totalCount) {
		t.Fatalf("daily/shift reconciliation: net %d/%d count %d/%d", net, totalShiftCash, count, totalCount)
	}
	// End is exclusive: a next-day sale must not change the report.
	if _, err := OpenShift(ctx, db, e.biz, e.branch, e.register, e.user, 0); err != nil {
		t.Fatal(err)
	}
	sale, err := e.sell(1, 1500000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE sales SET completed_at=$2 WHERE id=$1`, sale.SaleID, f.Until); err != nil {
		t.Fatal(err)
	}
	again, err := reporting.Sales(ctx, db, f)
	if err != nil || len(again) != len(rows) {
		t.Fatalf("next-day boundary: %v", err)
	}
	var afterCount int64
	for _, row := range again {
		afterCount += row.SaleCount
	}
	if afterCount != count {
		t.Fatalf("next-day sale leaked into daily report: %d != %d", afterCount, count)
	}
}

// The scale check is skipped with -short and uses an isolated temporary database.
func TestSalesReport5000TransactionsPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("scale test disabled by -short")
	}
	e := setup(t, 10000)
	ctx := context.Background()
	db := e.pool.Underlying()
	sample, err := e.sell(1, 1500000)
	if err != nil {
		t.Fatal(err)
	}
	// A dedicated temporary DB; fixtures replicate complete line snapshots, not application data.
	_, err = db.Exec(ctx, `WITH seeded AS (
 INSERT INTO sales(id,business_id,branch_id,register_id,shift_id,receipt_number,subtotal_gross_cents,line_discount_cents,tx_discount_cents,service_charge_cents,tax_cents,cash_rounding_cents,total_cents,change_cents,created_by,completed_at)
 SELECT gen_random_uuid(),business_id,branch_id,register_id,shift_id,'PERF-'||g,subtotal_gross_cents,line_discount_cents,tx_discount_cents,service_charge_cents,tax_cents,cash_rounding_cents,total_cents,change_cents,created_by,'2026-10-13T01:00:00Z'
 FROM sales CROSS JOIN generate_series(1,4999) g WHERE id=$1 RETURNING id,business_id
 ) INSERT INTO sale_lines(id,business_id,sale_id,line_no,variant_id,sku,product_name,variant_name,base_unit,qty_milli,unit_price_cents,gross_cents,line_discount_cents,alloc_discount_cents,net_cents,service_charge_cents,tax_cents,line_total_cents,allow_fractional,base_qty_milli,product_id,category_id,cost_cents)
 SELECT gen_random_uuid(),s.business_id,s.id,l.line_no,l.variant_id,l.sku,l.product_name,l.variant_name,l.base_unit,l.qty_milli,l.unit_price_cents,l.gross_cents,l.line_discount_cents,l.alloc_discount_cents,l.net_cents,l.service_charge_cents,l.tax_cents,l.line_total_cents,l.allow_fractional,l.base_qty_milli,l.product_id,l.category_id,l.cost_cents
 FROM seeded s CROSS JOIN sale_lines l WHERE l.sale_id=$1`, sample.SaleID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE sales SET completed_at='2026-10-13T01:00:00Z' WHERE id=$1`, sample.SaleID); err != nil {
		t.Fatal(err)
	}
	f := reporting.PaymentFilter{BusinessID: e.biz, BranchID: e.branch, From: time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC), Until: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Jakarta"}
	for i := 0; i < 5; i++ {
		start := time.Now()
		rows, err := reporting.Sales(ctx, db, f)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		var count int64
		for _, r := range rows {
			count += r.SaleCount
		}
		if count != 5000 {
			t.Fatalf("fixture count: %d", count)
		}
		t.Logf("5000 sales, run %d: %s", i+1, elapsed)
		if elapsed > 3*time.Second {
			t.Fatalf("NFR-004 local target exceeded: %s", elapsed)
		}
	}
}

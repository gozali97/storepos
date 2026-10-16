package sales

import (
	"context"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/pricing"
	"github.com/storepos/storepos/internal/reporting"
	"testing"
	"time"
)

func TestCorrectionsReportActorsApproversAndDates(t *testing.T) {
	e := setup(t, 10)
	ctx := context.Background()
	db := e.pool.Underlying()
	boss := idkit.New()
	if _, err := db.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,'boss@correction.test','x')`, boss); err != nil {
		t.Fatal(err)
	}
	var sale *Result
	err := e.inTx(func(ctx context.Context, tx database.Tx) error {
		var err error
		sale, err = Finalize(ctx, tx, Input{BusinessID: e.biz, BranchID: e.branch, RegisterID: e.register, UserID: e.user, Lines: []LineIn{{VariantID: e.variant, QtyMilli: 2000}}, Payments: []PaymentIn{{Method: "cash", AmountCents: 4000000}}, TxDiscount: pricing.Discount{Kind: pricing.DiscountPercent, Value: 1000}, DiscountLimitBps: 1000, DiscountReason: "promo"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var line idkit.ID
	if err := db.QueryRow(ctx, `SELECT id FROM sale_lines WHERE sale_id=$1`, sale.SaleID).Scan(&line); err != nil {
		t.Fatal(err)
	}
	if err := e.inTx(func(ctx context.Context, tx database.Tx) error {
		_, err := Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: sale.SaleID, UserID: e.user, RegisterID: e.register, ApprovedBy: &boss, Reason: "barang rusak", Lines: []ReturnLine{{LineID: line, Qty: 1000, Condition: "damaged"}}, HasBranch: func(idkit.ID) bool { return true }})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	second, err := e.sell(1, 1500000)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.inTx(func(ctx context.Context, tx database.Tx) error {
		return Void(ctx, tx, e.biz, second.SaleID, e.user, boss, "salah input", func(idkit.ID) bool { return true })
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE sales SET completed_at='2026-10-12T17:30:00Z',voided_at=CASE WHEN status='voided' THEN '2026-10-14T17:30:00Z'::timestamptz END`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE sale_returns SET created_at='2026-10-13T17:30:00Z'`); err != nil {
		t.Fatal(err)
	}
	f := reporting.PaymentFilter{BusinessID: e.biz, BranchID: e.branch, From: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), Until: time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Jakarta"}
	rows, err := reporting.Corrections(ctx, db, f)
	if err != nil || len(rows) != 3 {
		t.Fatalf("report: %+v %v", rows, err)
	}
	if rows[0].Kind != "discount" || rows[0].TransactionDiscount != "300000" || rows[0].LocalDate != "2026-10-13" {
		t.Fatal(rows)
	}
	if rows[1].Kind != "return" || rows[1].Amount != "1350000" || rows[1].Approver == nil || *rows[1].Approver != boss.String() || rows[1].LocalDate != "2026-10-14" {
		t.Fatal(rows)
	}
	if rows[2].Kind != "void" || rows[2].Reason != "salah input" || rows[2].LocalDate != "2026-10-15" {
		t.Fatal(rows)
	}
	f.UserID = &boss
	rows, err = reporting.Corrections(ctx, db, f)
	if err != nil || len(rows) != 0 {
		t.Fatalf("actor filter wrongly matched approver: %+v %v", rows, err)
	}
	f.UserID = nil
	f.BusinessID = idkit.New()
	rows, err = reporting.Corrections(ctx, db, f)
	if err != nil || len(rows) != 0 {
		t.Fatalf("tenant: %+v %v", rows, err)
	}
}

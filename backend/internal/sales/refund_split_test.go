package sales

import (
	"context"
	"errors"
	"testing"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

func TestAllocateRefundNoncashFirstThenCashWithinRemaining(t *testing.T) {
	left := []methodLeft{{"cash", 2000000}, {"qris", 1000000}, {"card", 500000}}
	parts, err := AllocateRefund(1700000, left)
	if err != nil {
		t.Fatal(err)
	}
	// card (500000) -> qris (1000000) -> cash sisa 200000; urutan nontunai alfabet.
	want := []RefundPart{{Method: "card", Amount: 500000, Manual: true}, {Method: "qris", Amount: 1000000, Manual: true}, {Method: "cash", Amount: 200000}}
	if len(parts) != len(want) {
		t.Fatalf("parts: %+v", parts)
	}
	for i := range want {
		if parts[i] != want[i] {
			t.Fatalf("bagian %d: %+v != %+v", i, parts[i], want[i])
		}
	}
	if _, err := AllocateRefund(3600000, left); !errors.Is(err, ErrReturnQuantity) {
		t.Fatalf("refund melebihi pembayaran harus ditolak: %v", err)
	}
}

func TestReturnSplitTenderRefundsPerOriginalMethod(t *testing.T) {
	e := setup(t, 10)
	var res *Result
	err := e.inTx(func(ctx context.Context, tx database.Tx) error {
		r, err := Finalize(ctx, tx, Input{BusinessID: e.biz, BranchID: e.branch, RegisterID: e.register, UserID: e.user,
			Lines:    []LineIn{{VariantID: e.variant, QtyMilli: 2000}},
			Payments: []PaymentIn{{Method: "qris", AmountCents: 1000000}, {Method: "cash", AmountCents: 2000000}}})
		res = r
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var line idkit.ID
	if err := e.pool.Underlying().QueryRow(context.Background(), `SELECT id FROM sale_lines WHERE sale_id=$1`, res.SaleID).Scan(&line); err != nil {
		t.Fatal(err)
	}
	var out *ReturnResult
	err = e.inTx(func(ctx context.Context, tx database.Tx) error {
		r, err := Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: res.SaleID, UserID: e.user, RegisterID: e.register,
			Reason: "salah beli", Lines: []ReturnLine{{LineID: line, Qty: 1000, Condition: "restock"}}, HasBranch: func(idkit.ID) bool { return true }})
		out = r
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Amount != "1500000" || len(out.Refunds) != 2 || !out.Manual {
		t.Fatalf("hasil retur: %+v", out)
	}
	if n := e.count(`SELECT COALESCE(SUM(amount_cents),0) FROM refunds WHERE method='qris'`); n != 1000000 {
		t.Fatalf("qris: %d", n)
	}
	if n := e.count(`SELECT COALESCE(SUM(amount_cents),0) FROM refunds WHERE method='cash' AND shift_id IS NOT NULL`); n != 500000 {
		t.Fatalf("tunai: %d", n)
	}
	// Retur kedua tidak boleh melewati sisa: tersisa tunai 1500000 saja.
	err = e.inTx(func(ctx context.Context, tx database.Tx) error {
		_, err := Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: res.SaleID, UserID: e.user, RegisterID: e.register,
			Reason: "sisa", Lines: []ReturnLine{{LineID: line, Qty: 1000, Condition: "restock"}}, HasBranch: func(idkit.ID) bool { return true }})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT COALESCE(SUM(amount_cents),0) FROM refunds`); n != 3000000 {
		t.Fatalf("total refund harus sama dengan total bayar: %d", n)
	}
}

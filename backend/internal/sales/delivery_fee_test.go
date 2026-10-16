package sales

import (
	"context"
	"errors"
	"testing"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

func (e *env) sellWithFee(units, fee, cash int64) (*Result, error) {
	var res *Result
	err := e.inTx(func(ctx context.Context, tx database.Tx) error {
		r, err := Finalize(ctx, tx, Input{BusinessID: e.biz, BranchID: e.branch, RegisterID: e.register, UserID: e.user,
			DeliveryFeeCents: fee,
			Lines:            []LineIn{{VariantID: e.variant, QtyMilli: units * 1000}},
			Payments:         []PaymentIn{{Method: "cash", AmountCents: cash}}})
		res = r
		return err
	})
	return res, err
}

func TestDeliveryFeeAddsToTotalWithoutTaxAndPaymentsMatch(t *testing.T) {
	e := setup(t, 10)
	r, err := e.sellWithFee(1, 1000000, 2500000) // Rp15.000 + ongkir Rp10.000
	if err != nil {
		t.Fatal(err)
	}
	if r.TotalCents != 2500000 || r.DeliveryFeeCents != 1000000 || r.ChangeCents != 0 {
		t.Fatalf("hasil: %+v", r)
	}
	if n := e.count(`SELECT delivery_fee_cents FROM sales`); n != 1000000 {
		t.Fatalf("ongkir tersimpan: %d", n)
	}
	if n := e.count(`SELECT SUM(amount_cents) FROM payments`); n != 2500000 {
		t.Fatalf("pembayaran harus sama dengan total termasuk ongkir: %d", n)
	}
	if n := e.count(`SELECT tax_cents FROM sales`); n != 0 {
		t.Fatalf("ongkir tidak boleh kena pajak: %d", n)
	}
}

func TestDeliveryFeeIsIncludedInCashRounding(t *testing.T) {
	e := setup(t, 10)
	// 1.500.000 + 5.000 = 1.505.000 → dibulatkan ke 1.510.000 (kelipatan Rp100, half-up).
	r, err := e.sellWithFee(1, 5000, 1510000)
	if err != nil {
		t.Fatal(err)
	}
	if r.TotalCents != 1510000 || r.CashRoundingCents != 5000 {
		t.Fatalf("pembulatan atas total+ongkir: %+v", r)
	}
}

func TestDeliveryFeeRejectsNegativeAndOversized(t *testing.T) {
	e := setup(t, 10)
	for _, fee := range []int64{-1, maxDeliveryFeeCents + 1} {
		if _, err := e.sellWithFee(1, fee, 99999999); !errors.Is(err, ErrInvalidDeliveryFee) {
			t.Fatalf("fee %d harus ditolak: %v", fee, err)
		}
	}
	if n := e.count(`SELECT count(*) FROM sales`); n != 0 {
		t.Fatalf("transaksi gagal tidak boleh tersimpan: %d", n)
	}
}

func TestFullReturnRefundsDeliveryFeeSoRefundEqualsPaid(t *testing.T) {
	e := setup(t, 10)
	r, err := e.sellWithFee(1, 5000, 1510000)
	if err != nil {
		t.Fatal(err)
	}
	var line idkit.ID
	if err := e.pool.Underlying().QueryRow(context.Background(), `SELECT id FROM sale_lines WHERE sale_id=$1`, r.SaleID).Scan(&line); err != nil {
		t.Fatal(err)
	}
	var out *ReturnResult
	err = e.inTx(func(ctx context.Context, tx database.Tx) error {
		x, err := Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: r.SaleID, UserID: e.user, RegisterID: e.register,
			Reason: "batal kirim", Lines: []ReturnLine{{LineID: line, Qty: 1000, Condition: "restock"}}, HasBranch: func(idkit.ID) bool { return true }})
		out = x
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Amount != "1510000" {
		t.Fatalf("retur penuh harus mengembalikan barang+pembulatan+ongkir: %s", out.Amount)
	}
}

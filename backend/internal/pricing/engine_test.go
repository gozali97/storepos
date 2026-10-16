package pricing

import (
	"testing"

	"github.com/storepos/storepos/internal/platform/money"
)

func TestSimpleCashNoTax(t *testing.T) {
	in := Input{
		Lines: []Line{{
			ID:             "L1",
			UnitPriceCents: money.Rupiah(14_000),
			Qty:            money.Milli(375),
		}},
		RoundingMode: money.HalfUp,
	}
	got, err := Compute(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.TotalCents != money.Rupiah(5_250) {
		t.Fatalf("total = %s", got.TotalCents)
	}
}

func TestLineDiscountCappedAtGross(t *testing.T) {
	in := Input{
		Lines: []Line{{
			ID:             "L1",
			UnitPriceCents: money.Rupiah(50_000),
			Qty:            money.Qty(1),
			LineDiscount:   Discount{Kind: DiscountAmount, Value: int64(money.Rupiah(60_000))},
		}},
		RoundingMode: money.HalfUp,
	}
	// Mesin membatasi ke gross (bukan menolak) agar logika aturan terletak di layer sales.
	// Namun berdasarkan PRD CORE-015, aturan menolak; implementasi aturan ada di modul sales.
	got, err := Compute(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.TotalCents != 0 {
		t.Fatalf("expected 0 (fully discounted), got %s", got.TotalCents)
	}
}

func TestTransactionDiscountAllocation(t *testing.T) {
	// Dua baris eligible: 50.000 dan 27.000, diskon tx Rp7.000.
	// Alokasi proporsional: 50/77 ≈ 4.545, 27/77 ≈ 2.455. Pembulatan half-up ke rupiah.
	in := Input{
		Lines: []Line{
			{ID: "A", UnitPriceCents: money.Rupiah(50_000), Qty: money.Qty(1), DiscountEligible: true},
			{ID: "B", UnitPriceCents: money.Rupiah(27_000), Qty: money.Qty(1), DiscountEligible: true},
		},
		TransactionDiscount: Discount{Kind: DiscountAmount, Value: int64(money.Rupiah(7_000))},
		RoundingMode:        money.HalfUp,
	}
	got, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	// Total alokasi harus persis Rp7.000.
	if got.Lines[0].AllocDiscountCents+got.Lines[1].AllocDiscountCents != money.Rupiah(7_000) {
		t.Fatalf("alloc total = %s", got.Lines[0].AllocDiscountCents+got.Lines[1].AllocDiscountCents)
	}
	// Subtotal net = 70.000
	if got.SubtotalNetCents != money.Rupiah(70_000) {
		t.Fatalf("net = %s", got.SubtotalNetCents)
	}
}

func TestInclusiveTaxExtractsBase(t *testing.T) {
	// PRD: harga 110.000 inklusif 10% → DPP 100.000, pajak 10.000.
	in := Input{
		Lines: []Line{{
			ID:             "L1",
			UnitPriceCents: money.Rupiah(110_000),
			Qty:            money.Qty(1),
			Tax:            &TaxRule{Name: "PPN", RateBps: 1000, Mode: TaxInclusive},
		}},
		RoundingMode: money.HalfUp,
	}
	got, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.TaxCents != money.Rupiah(10_000) {
		t.Fatalf("pajak = %s", got.TaxCents)
	}
	if got.Lines[0].TaxBaseCents != money.Rupiah(100_000) {
		t.Fatalf("dasar pajak = %s", got.Lines[0].TaxBaseCents)
	}
	if got.TotalCents != money.Rupiah(110_000) {
		t.Fatalf("total = %s", got.TotalCents)
	}
}

func TestRejectZeroQty(t *testing.T) {
	in := Input{
		Lines: []Line{{ID: "L1", UnitPriceCents: money.Rupiah(1_000), Qty: 0}},
	}
	if _, err := Compute(in); err == nil {
		t.Fatal("expected error")
	}
}

func TestRejectNegativePrice(t *testing.T) {
	in := Input{
		Lines: []Line{{ID: "L1", UnitPriceCents: -1, Qty: money.Qty(1)}},
	}
	if _, err := Compute(in); err == nil {
		t.Fatal("expected error")
	}
}

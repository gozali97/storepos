package sales

import (
	"context"
	"errors"
	"testing"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/pricing"
)

func TestTaxSnapshot(t *testing.T) {
	for _, mode := range []string{"exclusive", "inclusive"} {
		t.Run(mode, func(t *testing.T) {
			e := setup(t, 10)
			ctx := context.Background()
			set := func(rate int) {
				t.Helper()
				if err := e.inTx(func(ctx context.Context, tx database.Tx) error {
					return SetTaxRule(ctx, tx, e.biz, e.user, "Pajak demo", rate, mode)
				}); err != nil {
					t.Fatal(err)
				}
			}
			set(1000)
			r, err := e.sell(1, 2000000)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "exclusive" && (r.TotalCents != 1650000 || r.TaxCents != 150000) {
				t.Fatalf("eksklusif: %+v", r)
			}
			if mode == "inclusive" && (r.TotalCents != 1500000 || r.TaxCents != 136400) {
				t.Fatalf("inklusif: %+v", r)
			}
			set(2000)
			var rate int
			var name string
			if err := e.pool.Underlying().QueryRow(ctx, `SELECT tax_rate_bps,tax_rule_name FROM sales WHERE id=$1`, r.SaleID).Scan(&rate, &name); err != nil {
				t.Fatal(err)
			}
			if rate != 1000 || name != "Pajak demo" {
				t.Fatalf("snapshot berubah: %d %s", rate, name)
			}
		})
	}
}

func TestDiscountValidationAndApproval(t *testing.T) {
	e := setup(t, 10)
	run := func(line, txdisc pricing.Discount, approved bool, reason string) error {
		return e.inTx(func(ctx context.Context, tx database.Tx) error {
			in := Input{BusinessID: e.biz, BranchID: e.branch, RegisterID: e.register, UserID: e.user,
				Lines:      []LineIn{{VariantID: e.variant, QtyMilli: 1000, Discount: line}},
				Payments:   []PaymentIn{{Method: "cash", AmountCents: 2000000}},
				TxDiscount: txdisc, DiscountLimitBps: 1000, DiscountReason: reason}
			if approved {
				in.DiscountApprovedBy = &e.user
			} // domain trusts verified caller; HTTP forbids self-approval.
			_, err := Finalize(ctx, tx, in)
			return err
		})
	}
	if err := run(pricing.Discount{Kind: pricing.DiscountAmount, Value: 1500001}, pricing.Discount{}, false, ""); !errors.Is(err, ErrDiscountExceeds) {
		t.Fatalf("nominal berlebih: %v", err)
	}
	if err := run(pricing.Discount{}, pricing.Discount{Kind: pricing.DiscountAmount, Value: 1500001}, false, ""); !errors.Is(err, ErrDiscountExceeds) {
		t.Fatalf("diskon transaksi berlebih: %v", err)
	}
	d := pricing.Discount{Kind: pricing.DiscountPercent, Value: 1500}
	if err := run(d, pricing.Discount{}, false, ""); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("batas 10%%: %v", err)
	}
	if err := run(d, pricing.Discount{}, true, ""); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("alasan wajib: %v", err)
	}
	if err := run(d, pricing.Discount{}, true, "promo disetujui"); err != nil {
		t.Fatal(err)
	}
	if err := run(pricing.Discount{Kind: pricing.DiscountPercent, Value: 1000}, pricing.Discount{}, false, ""); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM sales`); n != 2 {
		t.Fatalf("rollback kegagalan: %d sales", n)
	}
}

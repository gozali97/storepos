package sales

import (
	"context"
	"github.com/storepos/storepos/internal/platform/database"
	"testing"
)

func TestCashRoundingPolicyEnabledDisabledAndNonCash(t *testing.T) {
	e := setup(t, 10)
	ctx := context.Background()
	db := e.pool.Underlying()
	if _, err := db.Exec(ctx, `UPDATE variants SET price_cents=8085000`); err != nil {
		t.Fatal(err)
	}
	// Older saved policy without new rounding keys must retain Rp100 defaults.
	if _, err := db.Exec(ctx, `UPDATE businesses SET settings=jsonb_build_object('operational_policy','{"cashier_discount_bps":1000,"cash_out_approval_cents":"50000000","variance_approval_cents":"1000000"}'::jsonb)`); err != nil {
		t.Fatal(err)
	}
	r, err := e.sell(1, 10000000)
	if err != nil || r.TotalCents != 8090000 || r.CashRoundingCents != 5000 {
		t.Fatalf("legacy defaults: %+v %v", r, err)
	}
	if _, err := db.Exec(ctx, `UPDATE businesses SET settings=jsonb_set(settings,'{operational_policy,cash_rounding_enabled}','false')`); err != nil {
		t.Fatal(err)
	}
	r, err = e.sell(1, 10000000)
	if err != nil || r.TotalCents != 8085000 || r.CashRoundingCents != 0 {
		t.Fatalf("disabled: %+v %v", r, err)
	}
	if _, err := db.Exec(ctx, `UPDATE businesses SET settings=jsonb_set(jsonb_set(settings,'{operational_policy,cash_rounding_enabled}','true'),'{operational_policy,cash_rounding_multiple_cents}','"50000"')`); err != nil {
		t.Fatal(err)
	}
	r, err = e.sell(1, 10000000)
	if err != nil || r.TotalCents != 8100000 {
		t.Fatalf("Rp500: %+v %v", r, err)
	}
	err = e.inTx(func(ctx context.Context, tx database.Tx) error {
		var err error
		r, err = Finalize(ctx, tx, Input{BusinessID: e.biz, BranchID: e.branch, RegisterID: e.register, UserID: e.user, Lines: []LineIn{{VariantID: e.variant, QtyMilli: 1000}}, Payments: []PaymentIn{{Method: "qris", AmountCents: 8085000}}})
		return err
	})
	if err != nil || r.TotalCents != 8085000 || r.CashRoundingCents != 0 {
		t.Fatalf("noncash: %+v %v", r, err)
	}
}

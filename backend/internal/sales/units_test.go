package sales

import (
	"context"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"testing"
)

func TestSaleAndReturnUseUnitSnapshot(t *testing.T) {
	e := setup(t, 100)
	ctx := context.Background()
	db := e.pool.Underlying()
	unit := idkit.New()
	if _, err := db.Exec(ctx, `INSERT INTO purchase_units(id,business_id,variant_id,name,factor_milli,sale_price_cents) VALUES($1,$2,$3,'Dus',24000,4800000)`, unit, e.biz, e.variant); err != nil {
		t.Fatal(err)
	}
	var sale *Result
	err := e.inTx(func(ctx context.Context, tx database.Tx) error {
		var err error
		sale, err = Finalize(ctx, tx, Input{BusinessID: e.biz, BranchID: e.branch, RegisterID: e.register, UserID: e.user, Lines: []LineIn{{VariantID: e.variant, UnitID: &unit, QtyMilli: 2000}}, Payments: []PaymentIn{{Method: "cash", AmountCents: 10000000}}})
		return err
	})
	if err != nil || sale.TotalCents != 9600000 {
		t.Fatalf("sale: %+v %v", sale, err)
	}
	if n := e.count(`SELECT qty_milli FROM inventory_balances`); n != 52000 {
		t.Fatalf("2 dus stock: %d", n)
	}
	if _, err := db.Exec(ctx, `UPDATE purchase_units SET factor_milli=1000 WHERE id=$1`, unit); err == nil {
		t.Fatal("used factor changed")
	}
	if _, err := db.Exec(ctx, `UPDATE purchase_units SET sale_price_cents=9999999 WHERE id=$1`, unit); err != nil {
		t.Fatal(err)
	}
	var line idkit.ID
	if err := db.QueryRow(ctx, `SELECT id FROM sale_lines WHERE sale_id=$1`, sale.SaleID).Scan(&line); err != nil {
		t.Fatal(err)
	}
	var refund *ReturnResult
	err = e.inTx(func(ctx context.Context, tx database.Tx) error {
		var err error
		refund, err = Return(ctx, tx, ReturnInput{BusinessID: e.biz, SaleID: sale.SaleID, UserID: e.user, RegisterID: e.register, Reason: "retur 1 dus", Lines: []ReturnLine{{LineID: line, Qty: 1000, Condition: "restock"}}, HasBranch: func(idkit.ID) bool { return true }})
		return err
	})
	if err != nil || refund.Amount != "4800000" {
		t.Fatalf("snapshot refund: %+v %v", refund, err)
	}
	if n := e.count(`SELECT qty_milli FROM inventory_balances`); n != 76000 {
		t.Fatalf("return conversion: %d", n)
	}
}

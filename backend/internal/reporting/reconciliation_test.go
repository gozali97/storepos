package reporting

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestReconciliationDetectsAndPersistsOnlyOnce(t *testing.T) {
	pool := testdb.New(t)
	db := pool.Underlying()
	ctx := context.Background()
	uid, biz, branch, reg, product, variant, shift, sale := idkit.New(), idkit.New(), idkit.New(), idkit.New(), idkit.New(), idkit.New(), idkit.New(), idkit.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO users(id,email,password_hash) VALUES($1,'recon@test.invalid','x')`, uid)
	exec(`INSERT INTO businesses(id,name,business_type,created_by) VALUES($1,'T','retail',$2)`, biz, uid)
	exec(`INSERT INTO branches(id,business_id,code,name) VALUES($1,$2,'T','T')`, branch, biz)
	exec(`INSERT INTO registers(id,business_id,branch_id,name) VALUES($1,$2,$3,'T')`, reg, biz, branch)
	exec(`INSERT INTO products(id,business_id,name) VALUES($1,$2,'P')`, product, biz)
	exec(`INSERT INTO variants(id,business_id,product_id,sku,price_cents) VALUES($1,$2,$3,'P',100)`, variant, biz, product)
	exec(`INSERT INTO shifts(id,business_id,branch_id,register_id,opened_by,opening_cash_cents) VALUES($1,$2,$3,$4,$5,0)`, shift, biz, branch, reg, uid)
	exec(`INSERT INTO sales(id,business_id,branch_id,register_id,shift_id,receipt_number,subtotal_gross_cents,line_discount_cents,tx_discount_cents,service_charge_cents,tax_cents,cash_rounding_cents,total_cents,change_cents,created_by) VALUES($1,$2,$3,$4,$5,'T-1',100,0,0,0,0,0,100,0,$6)`, sale, biz, branch, reg, shift, uid)
	exec(`INSERT INTO payments(id,business_id,sale_id,method,amount_cents,tendered_cents) VALUES($1,$2,$3,'cash',100,100)`, idkit.New(), biz, sale)
	exec(`INSERT INTO inventory_balances(business_id,branch_id,variant_id,qty_milli) VALUES($1,$2,$3,1000)`, biz, branch, variant)
	movement := func(kind string, qty, after int64) {
		exec(`INSERT INTO stock_movements(id,business_id,branch_id,variant_id,kind,qty_milli,balance_after_milli,ref_type,ref_id,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,'test',$8,$9)`, idkit.New(), biz, branch, variant, kind, qty, after, sale, uid)
	}
	movement("RECEIPT", 1000, 1000)
	movement("RETURN_DAMAGED", 1000, 1000)
	fs, err := Check(ctx, db)
	if err != nil || len(fs) != 0 {
		t.Fatalf("clean/damaged: %+v %v", fs, err)
	}
	exec(`UPDATE inventory_balances SET qty_milli=999`)
	exec(`UPDATE payments SET amount_cents=99`)
	fs, err = Check(ctx, db)
	if err != nil || len(fs) != 2 {
		t.Fatalf("corruption not detected: %+v %v", fs, err)
	}
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ran, n, err := RunDaily(ctx, pool, time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC))
			if err != nil {
				t.Error(err)
			}
			if ran && n != 2 {
				t.Errorf("findings: %d", n)
			}
			results <- ran
		}()
	}
	wg.Wait()
	close(results)
	runs := 0
	for ran := range results {
		if ran {
			runs++
		}
	}
	if runs != 1 {
		t.Fatalf("duplicate daily runs: %d", runs)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM reconciliation_findings`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("persistence: %d %v", n, err)
	}
}

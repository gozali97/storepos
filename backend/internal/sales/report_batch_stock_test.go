package sales

import (
	"context"
	"testing"
	"time"

	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/reporting"
)

func (e *env) receiptCost(t *testing.T, batch idkit.ID, units, cost int64) {
	t.Helper()
	var supplier idkit.ID
	db := e.pool.Underlying()
	ctx := context.Background()
	if err := db.QueryRow(ctx, `SELECT id FROM suppliers WHERE business_id=$1 LIMIT 1`, e.biz).Scan(&supplier); err != nil {
		supplier = idkit.New()
		if _, err := db.Exec(ctx, `INSERT INTO suppliers(id,business_id,name,created_by) VALUES($1,$2,'Pemasok',$3)`, supplier, e.biz, e.user); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, `INSERT INTO pharmacy_batch_receipts(id,business_id,branch_id,batch_id,supplier_id,invoice_number,qty_milli,unit_cost_cents,status,created_by) VALUES($1,$2,$3,$4,$5,'INV',$6,$7,'SELLABLE',$8)`,
		idkit.New(), e.biz, e.branch, batch, supplier, units*1000, cost, e.user); err != nil {
		t.Fatal(err)
	}
}

func TestBatchStockReportFiltersDerivedExpiryCostAndPeriod(t *testing.T) {
	e := pharmacyEnv(t)
	ctx := context.Background()
	near := e.addBatch(t, "NEAR", 30, "SELLABLE", 4)
	far := e.addBatch(t, "FAR", 400, "SELLABLE", 10)
	old := e.addBatch(t, "OLD", -5, "SELLABLE", 3)
	quarantine := e.addBatch(t, "Q", 200, "QUARANTINE", 2)
	// Biaya tertimbang untuk FAR: 2 unit @1000 dan 8 unit @2000 = (2*1000+8*2000)/10 = 1800.
	e.receiptCost(t, far, 2, 1000)
	e.receiptCost(t, far, 8, 2000)

	f := reporting.PaymentFilter{BusinessID: e.biz, BranchID: e.branch, Timezone: "Asia/Jakarta",
		From: time.Now().Add(-24 * time.Hour), Until: time.Now().Add(24 * time.Hour)}
	rows := func(filter string, cost bool) []reporting.BatchStockRow {
		t.Helper()
		out, err := reporting.BatchStock(ctx, e.pool.Underlying(), f, cost, filter, 90)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	byNumber := func(list []reporting.BatchStockRow) map[string]reporting.BatchStockRow {
		m := map[string]reporting.BatchStockRow{}
		for _, r := range list {
			m[r.BatchNumber] = r
		}
		return m
	}

	all := byNumber(rows("all", true))
	if len(all) != 4 {
		t.Fatalf("semua batch bersaldo: %d", len(all))
	}
	if all["OLD"].Status != "EXPIRED" || all["OLD"].StoredStatus != "SELLABLE" {
		t.Fatalf("EXPIRED harus diturunkan dari tanggal, bukan disimpan: %+v", all["OLD"])
	}
	if all["Q"].Status != "QUARANTINE" {
		t.Fatalf("status karantina: %+v", all["Q"])
	}
	if got := all["FAR"]; got.AverageCost == nil || *got.AverageCost != "1800" || got.EstimatedValue == nil || *got.EstimatedValue != "18000" {
		t.Fatalf("biaya tertimbang FAR: %+v", got)
	}
	if got := all["NEAR"]; got.AverageCost != nil || got.EstimatedValue != nil {
		t.Fatalf("batch tanpa harga beli harus nol, bukan 0: %+v", got)
	}
	if all["NEAR"].MovementCount != 1 || all["NEAR"].Movement != "4000" {
		t.Fatalf("pergerakan periode: %+v", all["NEAR"])
	}

	if n := len(rows("near_expiry", false)); n != 1 {
		t.Fatalf("near_expiry harus hanya NEAR: %d", n)
	}
	if n := len(rows("expired", false)); n != 1 {
		t.Fatalf("expired harus hanya OLD: %d", n)
	}
	if n := len(rows("quarantine", false)); n != 1 {
		t.Fatalf("quarantine: %d", n)
	}
	// Peran tanpa izin biaya tidak boleh melihat biaya sama sekali.
	for _, r := range rows("all", false) {
		if r.AverageCost != nil || r.EstimatedValue != nil {
			t.Fatalf("biaya bocor ke peran tanpa izin: %+v", r)
		}
	}
	// Periode di masa lalu: saldo tetap tampil (saat ini), pergerakan nol.
	f.From, f.Until = time.Now().AddDate(0, 0, -10), time.Now().AddDate(0, 0, -9)
	if r := byNumber(rows("all", false))["FAR"]; r.MovementCount != 0 || r.Balance != "10000" {
		t.Fatalf("periode lampau: %+v", r)
	}
	_ = near
	_ = old
	_ = quarantine
}

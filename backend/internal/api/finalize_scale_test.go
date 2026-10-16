package apiserver

import (
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
)

// Benchmark HTTP lokal opt-in; bukan pengganti load test produksi.
func TestFinalizeHTTPP95Local(t *testing.T) {
	if os.Getenv("STOREPOS_PERF_CHECK") != "1" {
		t.Skip("set STOREPOS_PERF_CHECK=1 untuk pengukuran lokal")
	}
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Perf", "owner@finalize.scale.test")
	branch, register := idkit.New(), idkit.New()
	if _, err := pool.Underlying().Exec(t.Context(), `INSERT INTO branches(id,business_id,code,name) VALUES($1,$2,'PERF','Perf')`, branch, biz); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Underlying().Exec(t.Context(), `INSERT INTO registers(id,business_id,branch_id,name) VALUES($1,$2,$3,'K')`, register, biz, branch); err != nil {
		t.Fatal(err)
	}
	c := login(t, newServer(t, pool), "owner@finalize.scale.test", biz.String())
	code, product := c.do("POST", "/api/v1/products", map[string]any{"name": "Perf item", "track_stock": true, "variants": []map[string]string{{"sku": "PERF-1", "price_cents": "100000"}}}, nil)
	if code != 201 {
		t.Fatal(code, product)
	}
	variant := product["variants"].([]any)[0].(map[string]any)["id"].(string)
	if code, out := c.do("POST", "/api/v1/stock/receipts", map[string]any{"branch_id": branch.String(), "lines": []map[string]string{{"variant_id": variant, "qty_milli": "100000"}}}, nil); code != 201 {
		t.Fatal(code, out)
	}
	if code, out := c.do("POST", "/api/v1/shifts/open", map[string]string{"branch_id": branch.String(), "register_id": register.String(), "opening_cash_cents": "0"}, nil); code != 201 {
		t.Fatal(code, out)
	}
	body := map[string]any{"branch_id": branch.String(), "register_id": register.String(), "lines": []map[string]string{{"variant_id": variant, "qty_milli": "1000"}}, "payments": []map[string]string{{"method": "cash", "amount_cents": "100000"}}}
	durations := make([]time.Duration, 0, 30)
	for i := 0; i < 31; i++ {
		start := time.Now()
		code, out := c.do("POST", "/api/v1/sales/finalize", body, map[string]string{"Idempotency-Key": "perf-finalize-" + strconv.Itoa(i)})
		elapsed := time.Since(start)
		if code != 201 || out["total_cents"] != "100000" {
			t.Fatalf("finalize %d: %d %v", i, code, out)
		}
		if i > 0 {
			durations = append(durations, elapsed)
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[28]
	t.Logf("finalisasi HTTP (stok/payment/counter/audit/idempotency), 30 sampel sesudah warmup: p95=%s max=%s", p95, durations[29])
	if p95 > 500*time.Millisecond {
		t.Errorf("p95 %s > 500ms", p95)
	}
	var balance int64
	if err := pool.Underlying().QueryRow(t.Context(), `SELECT qty_milli FROM inventory_balances WHERE business_id=$1 AND branch_id=$2 AND variant_id=$3`, biz, branch, variant).Scan(&balance); err != nil || balance != 69000 {
		t.Fatalf("balance=%d err=%v", balance, err)
	}
	var count int
	if err := pool.Underlying().QueryRow(t.Context(), `SELECT count(DISTINCT receipt_number) FROM sales WHERE business_id=$1`, biz).Scan(&count); err != nil || count != 31 {
		t.Fatalf("unique receipts=%d err=%v", count, err)
	}
}

package apiserver

import (
	"net/url"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/storepos/storepos/internal/platform/testdb"
)

// Jalankan eksplisit: STOREPOS_PERF_CHECK=1 go test ./internal/api -run TestCatalogSearchScale50000 -count=1 -v.
// Target HTTP lokal (termasuk auth/tenant/JSON), bukan klaim beban produksi atau perangkat kasir.
func TestCatalogSearchScale50000(t *testing.T) {
	if os.Getenv("STOREPOS_PERF_CHECK") != "1" {
		t.Skip("benchmark HTTP 50k opt-in; set STOREPOS_PERF_CHECK=1")
	}
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Scale", "owner@catalog.scale.test")
	other := seedBusiness(t, pool, "Other", "other@catalog.scale.test")
	for _, bid := range []any{biz, other} {
		if _, err := pool.Underlying().Exec(t.Context(), `WITH made AS (
   INSERT INTO products(id,business_id,name,track_stock)
   SELECT gen_random_uuid(),$1,'Scale Product '||lpad(n::text,5,'0'),false FROM generate_series(1,50000) n RETURNING id,name
  ) INSERT INTO variants(id,business_id,product_id,name,sku,base_unit,price_cents)
  SELECT gen_random_uuid(),$1,id,'','SCALE-'||right(name,5),'pcs',100000 FROM made`, bid); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Underlying().Exec(t.Context(), `INSERT INTO variant_barcodes(business_id,variant_id,barcode) SELECT business_id,id,'SCAN-'||sku FROM variants`); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"products", "variants", "variant_barcodes"} {
		if _, err := pool.Underlying().Exec(t.Context(), "ANALYZE "+table); err != nil {
			t.Fatal(err)
		}
	}
	c := login(t, newServer(t, pool), "owner@catalog.scale.test", biz.String())
	for _, q := range []string{"", "Scale Product 49", "ALE-49999", "SCAN-SCALE-49999", "not-found", "%"} {
		durations := make([]time.Duration, 0, 20)
		for i := 0; i < 21; i++ {
			start := time.Now()
			code, out := c.do("GET", "/api/v1/catalog/search?limit=50&q="+url.QueryEscape(q), nil, nil)
			elapsed := time.Since(start)
			if code != 200 {
				t.Fatalf("q=%q: %d %v", q, code, out)
			}
			hits := out["items"].([]any)
			if (q == "not-found" || q == "%") && len(hits) != 0 {
				t.Fatalf("literal/no-result q=%q: %d rows", q, len(hits))
			}
			if q == "SCAN-SCALE-49999" && len(hits) != 1 {
				t.Fatalf("barcode lookup: %d", len(hits))
			}
			if i > 0 {
				durations = append(durations, elapsed)
			}
		}
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		p95 := durations[18]
		t.Logf("q=%q 2 tenants x 50000 variants: p95=%s max=%s", q, p95, durations[19])
		if p95 > 300*time.Millisecond {
			t.Errorf("q=%q p95 %s > 300ms", q, p95)
		}
	}
}

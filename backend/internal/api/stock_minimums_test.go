package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestMinimumThresholdAndVersion(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@minimum.test")
	other := seedBusiness(t, pool, "Lain", "other@minimum.test")
	db := pool.Underlying()
	ctx := context.Background()
	var branch string
	if err := db.QueryRow(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES(gen_random_uuid(),$1,'A','Pusat') RETURNING id::text`, biz).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	base := newServer(t, pool)
	c := login(t, base, "owner@minimum.test", biz.String())
	b := login(t, base, "other@minimum.test", other.String())
	code, p := c.do("POST", "/api/v1/products", map[string]any{"name": "Kopi", "variants": []map[string]string{{"sku": "MIN1", "price_cents": "100"}}}, nil)
	if code != 201 {
		t.Fatal(code, p)
	}
	vid := p["variants"].([]any)[0].(map[string]any)["id"].(string)
	path := "/api/v1/stock/minimums/" + vid
	body := map[string]string{"branch_id": branch, "minimum_qty_milli": "5000"}
	if code, r := c.do("PUT", path, body, map[string]string{"If-Match": "0"}); code != 200 {
		t.Fatal(code, r)
	}
	if code, _ := c.do("PUT", path, body, map[string]string{"If-Match": "0"}); code != 409 {
		t.Fatalf("version: %d", code)
	}
	if code, _ := b.do("PUT", path, body, map[string]string{"If-Match": "0"}); code != 404 {
		t.Fatalf("tenant: %d", code)
	}
	list := "/api/v1/stock/reorder?branch_id=" + branch
	check := func(want int) {
		t.Helper()
		code, r := c.do("GET", list, nil, nil)
		if code != 200 || len(r["items"].([]any)) != want {
			t.Fatal(code, r)
		}
	}
	check(1) // Missing balance counts as zero.
	adjust := func(qty, key string) {
		t.Helper()
		code, r := c.do("POST", "/api/v1/stock/adjustments", map[string]string{"branch_id": branch, "variant_id": vid, "qty_milli": qty, "reason": "correction"}, map[string]string{"Idempotency-Key": key})
		if code != 201 {
			t.Fatal(code, r)
		}
	}
	adjust("6000", "in")
	check(0)
	adjust("-1000", "out")
	check(1) // AC: exactly minimum appears.
	body["minimum_qty_milli"] = "-1"
	if code, _ := c.do("PUT", path, body, map[string]string{"If-Match": "1"}); code != 422 {
		t.Fatalf("negative minimum: %d", code)
	}
}

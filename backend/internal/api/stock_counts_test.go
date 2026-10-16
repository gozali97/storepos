package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"sync"
	"testing"
)

func TestStockCountUsesBalanceAtCountAndPreservesLaterSales(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@count.test")
	seedUser(t, pool, "warehouse@count.test", "warehouse", biz)
	db := pool.Underlying()
	ctx := context.Background()
	var branch string
	if err := db.QueryRow(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES(gen_random_uuid(),$1,'A','Pusat') RETURNING id::text`, biz).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	base := newServer(t, pool)
	c := login(t, base, "owner@count.test", biz.String())
	warehouse := login(t, base, "warehouse@count.test", biz.String())
	if _, err := db.Exec(ctx, `INSERT INTO membership_branches(membership_id,business_id,branch_id) SELECT m.id,m.business_id,$2::uuid FROM memberships m JOIN users u ON u.id=m.user_id WHERE m.business_id=$1 AND u.email='warehouse@count.test'`, biz, branch); err != nil {
		t.Fatal(err)
	}
	createProduct := func(sku string) string {
		t.Helper()
		code, p := c.do("POST", "/api/v1/products", map[string]any{"name": sku, "variants": []map[string]string{{"sku": sku, "price_cents": "100"}}}, nil)
		if code != 201 {
			t.Fatal(code, p)
		}
		return p["variants"].([]any)[0].(map[string]any)["id"].(string)
	}
	vid := createProduct("COUNT1")
	ignored := createProduct("COUNT2")
	adjust := func(id, qty, key string) {
		t.Helper()
		code, r := c.do("POST", "/api/v1/stock/adjustments", map[string]string{"branch_id": branch, "variant_id": id, "qty_milli": qty, "reason": "correction"}, map[string]string{"Idempotency-Key": key})
		if code != 201 {
			t.Fatal(code, r)
		}
	}
	adjust(vid, "10000", "opening1")
	adjust(ignored, "5000", "opening2")
	code, res := warehouse.do("POST", "/api/v1/stock/counts", map[string]string{"branch_id": branch}, nil)
	if code != 201 {
		t.Fatal(code, res)
	}
	path := "/api/v1/stock/counts/" + res["count_id"].(string)
	adjust(vid, "-1000", "sale-before-count")
	if code, r := warehouse.do("PUT", path+"/lines/"+vid, map[string]string{"counted_qty_milli": "8000"}, map[string]string{"If-Match": "1"}); code != 200 {
		t.Fatal(code, r)
	}
	code, r := warehouse.do("GET", path, nil, nil)
	if code != 200 {
		t.Fatal(code, r)
	}
	found := false
	for _, x := range r["lines"].([]any) {
		l := x.(map[string]any)
		if l["variant_id"] == vid {
			found = true
			if l["snapshot_qty_milli"] != "10000" || l["system_qty_at_count"] != "9000" {
				t.Fatal(l)
			}
		}
	}
	if !found {
		t.Fatal("count line missing")
	}
	if code, _ := warehouse.do("PUT", path+"/lines/"+vid, map[string]string{"counted_qty_milli": "8000"}, map[string]string{"If-Match": "1"}); code != 409 {
		t.Fatalf("stale: %d", code)
	}
	adjust(vid, "-1000", "sale-after-count")
	if code, _ := warehouse.do("POST", path+"/post", nil, map[string]string{"If-Match": "2"}); code != 403 {
		t.Fatalf("warehouse cannot post: %d", code)
	}
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := c.do("POST", path+"/post", nil, map[string]string{"If-Match": "2"})
			codes <- code
		}()
	}
	wg.Wait()
	close(codes)
	success, conflicts := 0, 0
	for code := range codes {
		if code == 200 {
			success++
		} else if code == 409 {
			conflicts++
		} else {
			t.Fatalf("unexpected post status: %d", code)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("parallel post: %d success, %d conflicts", success, conflicts)
	}
	if code, _ := c.do("POST", path+"/post", nil, map[string]string{"If-Match": "2"}); code != 409 {
		t.Fatalf("double posting: %d", code)
	}
	if code, _ := c.do("PUT", path+"/lines/"+vid, map[string]string{"counted_qty_milli": "1000"}, map[string]string{"If-Match": "3"}); code != 409 {
		t.Fatalf("edit posted: %d", code)
	}
	var qty int64
	if err := db.QueryRow(ctx, `SELECT qty_milli FROM inventory_balances WHERE variant_id=$1`, vid).Scan(&qty); err != nil || qty != 7000 {
		t.Fatalf("later sales lost: %d %v", qty, err)
	}
	if err := db.QueryRow(ctx, `SELECT qty_milli FROM inventory_balances WHERE variant_id=$1`, ignored).Scan(&qty); err != nil || qty != 5000 {
		t.Fatalf("uncounted changed: %d %v", qty, err)
	}
}

package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestSuppliersUsedArchiveAndTenantIsolation(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "A", "owner@supplier.test")
	other := seedBusiness(t, pool, "B", "other@supplier.test")
	base := newServer(t, pool)
	c := login(t, base, "owner@supplier.test", biz.String())
	b := login(t, base, "other@supplier.test", other.String())
	req := map[string]any{"name": "Distributor", "contact": "kontak", "address": "alamat", "note": "catatan"}
	code, x := c.do("POST", "/api/v1/suppliers", req, nil)
	if code != 201 {
		t.Fatal(code, x)
	}
	id := x["id"].(string)
	path := "/api/v1/suppliers/" + id
	if code, _ := b.do("PUT", path, req, map[string]string{"If-Match": "1"}); code != 404 {
		t.Fatalf("tenant: %d", code)
	}
	var branch string
	db := pool.Underlying()
	ctx := context.Background()
	if err := db.QueryRow(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES(gen_random_uuid(),$1,'A','Pusat') RETURNING id::text`, biz).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	code, p := c.do("POST", "/api/v1/products", map[string]any{"name": "Kopi", "variants": []map[string]string{{"sku": "SUP1", "price_cents": "100"}}}, nil)
	if code != 201 {
		t.Fatal(code, p)
	}
	vid := p["variants"].([]any)[0].(map[string]any)["id"].(string)
	code, u := c.do("POST", "/api/v1/purchase-units", map[string]string{"variant_id": vid, "name": "pcs", "factor_milli": "1000"}, nil)
	if code != 201 {
		t.Fatal(code, u)
	}
	receipt := map[string]any{"branch_id": branch, "supplier_id": id, "received_date": "2026-10-13", "lines": []map[string]string{{"unit_id": u["unit_id"].(string), "qty_milli": "1000", "purchase_price_cents": "100"}}}
	if code, r := c.do("POST", "/api/v1/goods-receipts", receipt, nil); code != 201 {
		t.Fatal(code, r)
	}
	if code, _ := c.do("DELETE", path, nil, map[string]string{"If-Match": "1"}); code != 409 {
		t.Fatalf("used supplier deleted: %d", code)
	}
	req["archived"] = true
	if code, _ := c.do("PUT", path, req, map[string]string{"If-Match": "1"}); code != 200 {
		t.Fatalf("archive: %d", code)
	}
	if code, _ := c.do("PUT", path, req, map[string]string{"If-Match": "1"}); code != 409 {
		t.Fatalf("stale version: %d", code)
	}
	if code, _ := c.do("POST", "/api/v1/goods-receipts", receipt, nil); code != 409 {
		t.Fatalf("archived supplier accepted: %d", code)
	}
	code, list := c.do("GET", "/api/v1/suppliers", nil, nil)
	if code != 200 || len(list["suppliers"].([]any)) != 1 {
		t.Fatal(code, list)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM goods_receipts WHERE supplier_id=$1`, id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("history: %d %v", n, err)
	}
	req["archived"] = false
	req["name"] = "Unused"
	code, y := c.do("POST", "/api/v1/suppliers", req, nil)
	if code != 201 {
		t.Fatal(code, y)
	}
	if code, _ := c.do("DELETE", "/api/v1/suppliers/"+y["id"].(string), nil, map[string]string{"If-Match": "1"}); code != 204 {
		t.Fatalf("unused delete: %d", code)
	}
}

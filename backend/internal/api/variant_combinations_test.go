package apiserver

import (
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestGeneratedVariantsPersistAndDeactivate(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@variants.test")
	c := login(t, newServer(t, pool), "owner@variants.test", biz.String())
	code, g := c.do("POST", "/api/v1/catalog/variant-combinations", map[string]any{"sku_prefix": "KAOS", "attributes": []map[string]any{{"name": "Ukuran", "values": []string{"S", "M", "L"}}, {"name": "Warna", "values": []string{"Hitam", "Putih"}}}}, nil)
	if code != 200 {
		t.Fatal(code, g)
	}
	variants := g["variants"].([]any)
	if len(variants) != 6 {
		t.Fatal(g)
	}
	for _, v := range variants {
		v.(map[string]any)["price_cents"] = "1000000"
	}
	code, p := c.do("POST", "/api/v1/products", map[string]any{"name": "Kaos Polos", "variants": variants}, nil)
	if code != 201 {
		t.Fatal(code, p)
	}
	saved := p["variants"].([]any)
	if len(saved) != 6 || len(saved[0].(map[string]any)["attributes"].(map[string]any)) != 2 {
		t.Fatal(p)
	}
	vid := saved[0].(map[string]any)["id"].(string)
	if code, r := c.do("PATCH", "/api/v1/variants/"+vid+"/status", map[string]string{"status": "inactive"}, map[string]string{"If-Match": "1"}); code != 200 {
		t.Fatal(code, r)
	}
	code, r := c.do("GET", "/api/v1/catalog/search?q=Kaos", nil, nil)
	if code != 200 || len(r["items"].([]any)) != 5 {
		t.Fatal(code, r)
	}
	if code, _ := c.do("PATCH", "/api/v1/variants/"+vid+"/status", map[string]string{"status": "active"}, map[string]string{"If-Match": "1"}); code != 409 {
		t.Fatalf("stale: %d", code)
	}
}

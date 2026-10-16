package apiserver

import (
	"testing"

	"github.com/storepos/storepos/internal/platform/testdb"
)

// NFR-013: pengguna bisnis B yang mengetahui ID sumber daya bisnis A tetap tidak boleh membacanya.
// Harus 404 (bukan 403 atau 200) agar keberadaan data bisnis lain tidak bisa ditebak.
func TestResourceIDsFromAnotherBusinessAreNotReadable(t *testing.T) {
	pool := testdb.New(t)
	bizA := seedBusiness(t, pool, "Toko A", "owner@a.resource.test")
	bizB := seedBusiness(t, pool, "Toko B", "owner@b.resource.test")
	base := newServer(t, pool)
	a := login(t, base, "owner@a.resource.test", bizA.String())
	b := login(t, base, "owner@b.resource.test", bizB.String())

	create := func(path string, body map[string]any) map[string]any {
		t.Helper()
		code, out := a.do("POST", path, body, nil)
		if code != 201 {
			t.Fatalf("buat %s: %d %v", path, code, out)
		}
		return out
	}
	customer := create("/api/v1/customers", map[string]any{"name": "Pelanggan A"})
	supplier := create("/api/v1/suppliers", map[string]any{"name": "Supplier A"})
	category := create("/api/v1/categories", map[string]any{"name": "Kategori A"})
	product := create("/api/v1/products", map[string]any{"name": "Produk A", "variants": []map[string]any{{"sku": "A-1", "price_cents": "100000"}}})
	variants, _ := product["variants"].([]any)
	if len(variants) != 1 {
		t.Fatalf("varian produk: %v", product)
	}
	variant, _ := variants[0].(map[string]any)

	paths := []string{
		"/api/v1/customers/" + customer["id"].(string),
		"/api/v1/suppliers/" + supplier["id"].(string),
		"/api/v1/categories/" + category["id"].(string),
		"/api/v1/products/" + product["id"].(string),
		"/api/v1/variants/" + variant["id"].(string),
	}
	for _, path := range paths {
		if code, _ := a.do("GET", path, nil, nil); code != 200 {
			t.Fatalf("pemilik sendiri harus bisa membaca %s: %d", path, code)
		}
		if code, _ := b.do("GET", path, nil, nil); code != 404 {
			t.Errorf("%s dibaca bisnis lain menghasilkan %d, seharusnya 404", path, code)
		}
	}
}

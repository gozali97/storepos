package apiserver

import (
	"strings"
	"testing"

	"github.com/storepos/storepos/internal/platform/testdb"
)

func searchCount(t *testing.T, c *client, q string) int {
	t.Helper()
	code, out := c.do("GET", "/api/v1/catalog/search?q="+q+"&limit=50", nil, nil)
	if code != 200 {
		t.Fatalf("cari %s: %d", q, code)
	}
	items, _ := out["items"].([]any)
	return len(items)
}

// §8.1: data contoh bisa dibuat, dihapus (diarsipkan), dan dibuat ulang tanpa menggandakan produk.
func TestDemoDataCreateRemoveRestoreIsIdempotentAndTenantScoped(t *testing.T) {
	pool := testdb.New(t)
	bizA := seedBusiness(t, pool, "Toko A", "owner@a.onboard.test")
	bizB := seedBusiness(t, pool, "Toko B", "owner@b.onboard.test")
	seedUser(t, pool, "kasir@a.onboard.test", "cashier", bizA)
	base := newServer(t, pool)
	a := login(t, base, "owner@a.onboard.test", bizA.String())
	b := login(t, base, "owner@b.onboard.test", bizB.String())
	cashier := login(t, base, "kasir@a.onboard.test", bizA.String())

	if code, _ := cashier.do("POST", "/api/v1/onboarding/demo", map[string]any{}, nil); code != 403 {
		t.Fatalf("kasir tidak boleh membuat data demo: %d", code)
	}
	code, out := a.do("POST", "/api/v1/onboarding/demo", map[string]any{}, nil)
	if code != 201 || out["created"].(float64) != 3 {
		t.Fatalf("buat demo: %d %v", code, out)
	}
	if n := searchCount(t, a, "DEMO"); n != 3 {
		t.Fatalf("produk demo: %d", n)
	}
	// Panggilan kedua tidak menggandakan.
	if code, out := a.do("POST", "/api/v1/onboarding/demo", map[string]any{}, nil); code != 201 || out["created"].(float64) != 0 {
		t.Fatalf("idempoten: %d %v", code, out)
	}
	if n := searchCount(t, a, "DEMO"); n != 3 {
		t.Fatalf("setelah panggilan ulang: %d", n)
	}
	if n := searchCount(t, b, "DEMO"); n != 0 {
		t.Fatalf("data demo bocor ke bisnis lain: %d", n)
	}

	code, out = a.do("DELETE", "/api/v1/onboarding/demo", nil, nil)
	if code != 200 || out["archived"].(float64) != 3 {
		t.Fatalf("hapus demo: %d %v", code, out)
	}
	if n := searchCount(t, a, "DEMO"); n != 0 {
		t.Fatalf("setelah dihapus masih tampil: %d", n)
	}
	code, out = a.do("POST", "/api/v1/onboarding/demo", map[string]any{}, nil)
	if code != 201 || out["restored"].(float64) != 3 || out["created"].(float64) != 0 {
		t.Fatalf("pulihkan demo: %d %v", code, out)
	}
	if n := searchCount(t, a, "DEMO"); n != 3 {
		t.Fatalf("setelah dipulihkan: %d", n)
	}
}

// Produk milik pengguna dengan SKU yang sama tidak boleh tertimpa atau ikut diarsipkan.
func TestDemoDataNeverTouchesUserProducts(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@clash.onboard.test")
	base := newServer(t, pool)
	c := login(t, base, "owner@clash.onboard.test", biz.String())
	code, _ := c.do("POST", "/api/v1/products", map[string]any{"name": "Punya Saya", "variants": []map[string]any{{"sku": "demo-air", "price_cents": "100000"}}}, nil)
	if code != 201 {
		t.Fatal(code)
	}
	if code, out := c.do("POST", "/api/v1/onboarding/demo", map[string]any{}, nil); code != 409 {
		t.Fatalf("bentrok SKU harus 409: %d %v", code, out)
	}
	if n := searchCount(t, c, "Punya"); n != 1 {
		t.Fatalf("produk pengguna harus tetap aktif: %d", n)
	}
	if n := searchCount(t, c, "DEMO-KOPI"); n != 0 {
		t.Fatalf("transaksi gagal tidak boleh meninggalkan produk demo: %d", n)
	}
	if code, out := c.do("DELETE", "/api/v1/onboarding/demo", nil, nil); code != 200 || out["archived"].(float64) != 0 {
		t.Fatalf("hapus demo tidak boleh mengarsipkan produk pengguna: %d %v", code, out)
	}
	if n := searchCount(t, c, "Punya"); n != 1 {
		t.Fatalf("produk pengguna terarsip: %d", n)
	}
}

func TestOnboardingChecklistIsDerivedAndDismissible(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@check.onboard.test")
	base := newServer(t, pool)
	c := login(t, base, "owner@check.onboard.test", biz.String())

	code, out := c.do("GET", "/api/v1/onboarding", nil, nil)
	if code != 200 || out["done"].(float64) != 0 || out["total"].(float64) != 5 || out["dismissed"].(bool) || out["m2_minutes"] != nil {
		t.Fatalf("awal: %d %v", code, out)
	}
	// Produk contoh tidak dihitung sebagai produk pertama pengguna.
	c.do("POST", "/api/v1/onboarding/demo", map[string]any{}, nil)
	_, out = c.do("GET", "/api/v1/onboarding", nil, nil)
	if out["demo_present"] != true || out["done"].(float64) != 0 {
		t.Fatalf("demo tidak boleh dihitung selesai: %v", out)
	}
	c.do("POST", "/api/v1/products", map[string]any{"name": "Asli", "variants": []map[string]any{{"sku": "ASLI-1", "price_cents": "100000"}}}, nil)
	_, out = c.do("GET", "/api/v1/onboarding", nil, nil)
	items, _ := out["items"].([]any)
	got := map[string]bool{}
	for _, it := range items {
		m := it.(map[string]any)
		got[m["key"].(string)] = m["done"].(bool)
	}
	if !got["first_product"] || got["first_sale"] || out["done"].(float64) != 1 {
		t.Fatalf("setelah produk asli: %v", out)
	}
	if code, out := c.do("PUT", "/api/v1/onboarding/dismiss", map[string]any{}, nil); code != 422 {
		t.Fatalf("dismissed wajib: %d %v", code, out)
	}
	if code, _ := c.do("PUT", "/api/v1/onboarding/dismiss", map[string]any{"dismissed": true}, nil); code != 200 {
		t.Fatal(code)
	}
	_, out = c.do("GET", "/api/v1/onboarding", nil, nil)
	if out["dismissed"] != true {
		t.Fatalf("dismissed: %v", out)
	}
	if !strings.Contains(out["items"].([]any)[0].(map[string]any)["key"].(string), "branch") {
		t.Fatalf("urutan item: %v", out["items"])
	}
}

func TestOnboardingEventsAreWhitelistedTenantScopedAndDeduplicated(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@event.onboard.test")
	other := seedBusiness(t, pool, "Lain", "owner@other.event.test")
	seedUser(t, pool, "cashier@event.test", "cashier", biz)
	base := newServer(t, pool)
	owner := login(t, base, "owner@event.onboard.test", biz.String())
	cashier := login(t, base, "cashier@event.test", biz.String())
	path := "/api/v1/onboarding/events"
	for _, body := range []map[string]string{{"step": "password", "status": "completed"}, {"step": "business", "status": "invalid"}} {
		if code, _ := owner.do("POST", path, body, nil); code != 422 {
			t.Fatalf("invalid milestone: %d", code)
		}
	}
	body := map[string]string{"step": "business", "status": "completed"}
	if code, _ := cashier.do("POST", path, body, nil); code != 403 {
		t.Fatalf("cashier: %d", code)
	}
	for i := 0; i < 2; i++ {
		if code, out := owner.do("POST", path, body, nil); code != 200 {
			t.Fatalf("event: %d %v", code, out)
		}
	}
	var n int
	if err := pool.Underlying().QueryRow(t.Context(), `SELECT count(*) FROM audit_logs WHERE business_id=$1 AND action='onboarding.step'`, biz).Scan(&n); err != nil || n != 1 {
		t.Fatalf("duplicate milestone: %d %v", n, err)
	}
	owner.biz = other.String()
	if code, _ := owner.do("POST", path, body, nil); code != 403 {
		t.Fatalf("cross tenant: %d", code)
	}
}

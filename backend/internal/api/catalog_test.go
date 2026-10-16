package apiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/storepos/storepos/internal/platform/auth"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
)

type client struct {
	t    *testing.T
	base string
	hc   *http.Client
	csrf string
	biz  string
}

func seedUser(t *testing.T, pool *database.Pool, email, role string, bid idkit.ID) {
	t.Helper()
	h, err := auth.Hash("rahasia-123")
	if err != nil {
		t.Fatal(err)
	}
	uid := idkit.New()
	ctx := context.Background()
	db := pool.Underlying()
	if _, err := db.Exec(ctx, `INSERT INTO users (id,email,password_hash) VALUES ($1,$2,$3)`, uid, email, h); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO memberships (id,user_id,business_id,role) VALUES ($1,$2,$3,$4)`,
		idkit.New(), uid, bid, role); err != nil {
		t.Fatal(err)
	}
}

func seedBusiness(t *testing.T, pool *database.Pool, name, owner string) idkit.ID {
	t.Helper()
	h, _ := auth.Hash("rahasia-123")
	uid, bid := idkit.New(), idkit.New()
	db := pool.Underlying()
	ctx := context.Background()
	if _, err := db.Exec(ctx, `INSERT INTO users (id,email,password_hash) VALUES ($1,$2,$3)`, uid, owner, h); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO businesses (id,name,business_type,enabled_modules,created_by)
		VALUES ($1,$2,'retail',ARRAY['retail'],$3)`, bid, name, uid); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO memberships (id,user_id,business_id,role) VALUES ($1,$2,$3,'owner')`,
		idkit.New(), uid, bid); err != nil {
		t.Fatal(err)
	}
	return bid
}

func newServer(t *testing.T, pool *database.Pool) string {
	t.Helper()
	s, err := NewServer(pool, time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.Routes(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts.URL
}

func login(t *testing.T, base, email, biz string) *client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &client{t: t, base: base, hc: &http.Client{Jar: jar}, biz: biz}
	code, _ := c.do("POST", "/api/v1/auth/login", map[string]string{"email": email, "password": "rahasia-123"}, nil)
	if code != 200 {
		t.Fatalf("login %s: %d", email, code)
	}
	for _, ck := range jar.Cookies(mustURL(base)) {
		if ck.Name == CookieCSRF {
			c.csrf = ck.Value
		}
	}
	return c
}

func (c *client) do(method, path string, body any, hdr map[string]string) (int, map[string]any) {
	c.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, c.base+path, rd)
	if c.csrf != "" {
		req.Header.Set(HeaderCSRF, c.csrf)
	}
	if c.biz != "" {
		req.Header.Set(HeaderBusinessID, c.biz)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestCatalogEndpointsChain(t *testing.T) {
	pool := testdb.New(t)
	bizA := seedBusiness(t, pool, "Toko A", "owner@a.test")
	bizB := seedBusiness(t, pool, "Toko B", "owner@b.test")
	seedUser(t, pool, "kasir@a.test", "cashier", bizA)
	base := newServer(t, pool)

	owner := login(t, base, "owner@a.test", bizA.String())
	cashier := login(t, base, "kasir@a.test", bizA.String())
	prod := map[string]any{"name": "Teh Botol", "variants": []map[string]any{
		{"sku": "TEH-1", "price_cents": "500000", "barcodes": []string{"899001"}}}}

	// Tanpa sesi → 401.
	anon := &client{t: t, base: base, hc: &http.Client{}, biz: bizA.String()}
	if code, _ := anon.do("GET", "/api/v1/catalog/search", nil, nil); code != 401 {
		t.Fatalf("tanpa sesi harus 401, dapat %d", code)
	}
	// Kasir hanya boleh melihat, tidak boleh membuat produk → 403.
	if code, _ := cashier.do("POST", "/api/v1/products", prod, nil); code != 403 {
		t.Fatalf("kasir membuat produk harus 403, dapat %d", code)
	}
	// Tanpa token CSRF → 403.
	owner2 := *owner
	owner2.csrf = ""
	if code, _ := owner2.do("POST", "/api/v1/products", prod, nil); code != 403 {
		t.Fatalf("tanpa CSRF harus 403, dapat %d", code)
	}
	// Owner membuat produk.
	code, res := owner.do("POST", "/api/v1/products", prod, nil)
	if code != 201 {
		t.Fatalf("owner buat produk: %d %v", code, res)
	}
	vid := res["variants"].([]any)[0].(map[string]any)["id"].(string)

	// Barcode ganda → 409 menyebut pemilik.
	prod2 := map[string]any{"name": "Teh Lain", "variants": []map[string]any{
		{"sku": "TEH-2", "price_cents": "100", "barcodes": []string{"899001"}}}}
	if code, res := owner.do("POST", "/api/v1/products", prod2, nil); code != 409 {
		t.Fatalf("barcode ganda harus 409, dapat %d %v", code, res)
	}
	// Kasir boleh mencari lewat barcode.
	code, res = cashier.do("GET", "/api/v1/catalog/search?q=899001", nil, nil)
	if code != 200 || len(res["items"].([]any)) != 1 {
		t.Fatalf("kasir cari: %d %v", code, res)
	}
	// Harga: versi lama → 409, versi benar → 200 + audit.
	if code, _ := owner.do("PATCH", "/api/v1/variants/"+vid+"/price", map[string]string{"price_cents": "600000"}, map[string]string{"If-Match": "1"}); code != 200 {
		t.Fatalf("ubah harga harus 200, dapat %d", code)
	}
	if code, _ := owner.do("PATCH", "/api/v1/variants/"+vid+"/price", map[string]string{"price_cents": "700000"}, map[string]string{"If-Match": "1"}); code != 409 {
		t.Fatalf("versi lama harus 409, dapat %d", code)
	}
	var n int
	_ = pool.Underlying().QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE business_id=$1 AND action IN ('product.created','variant.price_changed')`, bizA).Scan(&n)
	if n != 2 {
		t.Fatalf("audit harus 2 catatan, dapat %d", n)
	}
	// Lintas tenant: owner A memakai X-Business-ID bisnis B → 403.
	evil := *owner
	evil.biz = bizB.String()
	if code, _ := evil.do("GET", "/api/v1/catalog/search?q=teh", nil, nil); code != 403 {
		t.Fatalf("lintas tenant harus 403, dapat %d", code)
	}
}

func mustURL(s string) *url.URL { u, _ := url.Parse(s); return u }

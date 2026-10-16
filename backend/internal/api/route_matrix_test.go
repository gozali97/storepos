package apiserver

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/storepos/storepos/internal/platform/testdb"
)

// Rute yang sengaja publik; menambah rute publik baru wajib menambah baris di sini (dan ditinjau).
// /leads dipakai form demo di landing page (LP-011): tanpa sesi, dibatasi ukuran body dan rate limit.
var publicRoutes = map[string]bool{
	"POST /api/v1/auth/login":     true,
	"POST /api/v1/public/receipt": true,
	"POST /api/v1/leads":          true,
}

// Rute yang didaftarkan lewat perulangan (nama dibangun dari string) tidak terbaca oleh regex literal.
// shortcut: daftar ini ditulis manual; tes di bawah gagal bila jumlah baris dinamis di kode berubah.
var dynamicRoutes = []string{
	"POST /api/v1/reports/payments/exports", "POST /api/v1/reports/sales/exports",
	"POST /api/v1/reports/corrections/exports", "POST /api/v1/reports/stock/exports",
	"POST /api/v1/reports/products/exports",
	"GET /api/v1/products/{id}/tax-rule", "GET /api/v1/categories/{id}/tax-rule",
	"PUT /api/v1/products/{id}/tax-rule", "PUT /api/v1/categories/{id}/tax-rule",
}

const expectedDynamicRegistrations = 3 // reports/exports, GET tax-rule, PUT tax-rule

var routePattern = regexp.MustCompile(`"(GET|POST|PUT|PATCH|DELETE) (/api/v1[^"]*)"`)

// registeredRoutes membaca rute dari pemanggilan mux.Handle/HandleFunc di kode sumber paket ini,
// sehingga rute baru otomatis ikut diuji. Potongan string lain (mis. kunci idempotency) diabaikan.
func registeredRoutes(t *testing.T) (routes []string, dynamicLines int) {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "mux.Handle") {
				continue
			}
			m := routePattern.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			if strings.Contains(line, `"+`) {
				dynamicLines++
				continue
			}
			seen[m[1]+" "+m[2]] = true
		}
	}
	for _, r := range dynamicRoutes {
		seen[r] = true
	}
	for r := range seen {
		routes = append(routes, r)
	}
	sort.Strings(routes)
	return routes, dynamicLines
}

var pathParam = regexp.MustCompile(`\{[^}]+\}`)

// NFR-013 / CORE-005: tanpa sesi, setiap endpoint non-publik harus menolak (401/403), tidak pernah 2xx, 404, atau 5xx.
func TestEveryNonPublicEndpointRejectsUnauthenticatedRequests(t *testing.T) {
	routes, dynamic := registeredRoutes(t)
	if dynamic != expectedDynamicRegistrations {
		t.Fatalf("ada %d pendaftaran rute dinamis (diharapkan %d): perbarui dynamicRoutes di tes ini", dynamic, expectedDynamicRegistrations)
	}
	if len(routes) < 150 {
		t.Fatalf("pembacaan rute mencurigakan: hanya %d rute", len(routes))
	}
	pool := testdb.New(t)
	base := newServer(t, pool)
	const sampleID = "0192f0a0-0000-7000-8000-000000000001"
	checked := 0
	for _, route := range routes {
		if publicRoutes[route] {
			continue
		}
		method, path, _ := strings.Cut(route, " ")
		path = pathParam.ReplaceAllString(path, sampleID)
		req, err := http.NewRequest(method, base+path, strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", route, err)
		}
		_ = resp.Body.Close()
		checked++
		if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s tanpa sesi menghasilkan %d, seharusnya 401/403", route, resp.StatusCode)
		}
	}
	t.Logf("%d rute non-publik diperiksa", checked)
}

// Rute publik harus tetap terdaftar; jika berubah nama, daftar putih di atas ikut diperbarui.
func TestPublicRouteAllowlistMatchesRegisteredRoutes(t *testing.T) {
	routes, _ := registeredRoutes(t)
	have := map[string]bool{}
	for _, r := range routes {
		have[r] = true
	}
	for route := range publicRoutes {
		if !have[route] {
			t.Errorf("rute publik %q tidak lagi terdaftar; perbarui daftar putih", route)
		}
	}
}

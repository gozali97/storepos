package apiserver

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
	"github.com/storepos/storepos/internal/platform/testdb"
)

var (
	permConst     = regexp.MustCompile(`(Perm\w+)\s+Permission\s*=\s*"([^"]+)"`)
	guardedRoute  = regexp.MustCompile(`"(GET|POST|PUT|PATCH|DELETE) (/api/v1[^"]*)",\s*s\.guard\(authz\.(Perm\w+),\s*(true|false)`)
	sampleIDParam = regexp.MustCompile(`\{[^}]+\}`)
)

type guardedEndpoint struct {
	method, path string
	perm         authz.Permission
	write        bool
}

// guardedEndpoints membaca rute yang dipasang lewat s.guard(authz.PermX, write, ...) dari kode sumber,
// sehingga matriks diuji terhadap definisi izin yang sama dengan yang dipakai server.
// shortcut: hanya s.guard; guard khusus modul (restoran/apotek) memerlukan bisnis dengan modul aktif dan belum dicakup.
func guardedEndpoints(t *testing.T) []guardedEndpoint {
	t.Helper()
	authzSrc, err := os.ReadFile("../platform/authz/authz.go")
	if err != nil {
		t.Fatal(err)
	}
	perms := map[string]authz.Permission{}
	for _, m := range permConst.FindAllStringSubmatch(string(authzSrc), -1) {
		perms[m[1]] = authz.Permission(m[2])
	}
	files := sourceFiles(t)
	var out []guardedEndpoint
	for _, src := range files {
		for _, m := range guardedRoute.FindAllStringSubmatch(src, -1) {
			perm, ok := perms[m[3]]
			if !ok {
				t.Fatalf("konstanta izin %s tidak ditemukan di authz.go", m[3])
			}
			out = append(out, guardedEndpoint{m[1], m[2], perm, m[4] == "true"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].method+out[i].path < out[j].method+out[j].path })
	return out
}

func sourceFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(b))
	}
	return out
}

const sampleID = "0192f0a0-0000-7000-8000-000000000001"

// CORE-005: untuk setiap peran dan setiap endpoint ber-guard, hasil mengikuti matriks izin:
// Denied (atau ReadOnly pada aksi tulis) harus 403; selain itu tidak boleh ditolak karena izin (bukan 401/403).
func TestRoleMatrixMatchesPermissionDefinitions(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko Matriks", "owner@matrix.test")
	roles := []tenant.Role{tenant.RoleManager, tenant.RoleCashier, tenant.RoleWarehouse, tenant.RoleKitchen, tenant.RolePharmacist}
	for _, r := range roles {
		seedUser(t, pool, string(r)+"@matrix.test", string(r), biz)
	}
	// Cabang nyata yang ditugaskan ke semua peran non-pemilik, agar 403 hanya berarti penolakan izin, bukan cakupan cabang.
	branch := idkit.New()
	for _, q := range []string{
		`INSERT INTO branches (id,business_id,code,name) VALUES ($1,$2,'MTX','Matriks')`,
		`INSERT INTO membership_branches (membership_id,business_id,branch_id) SELECT m.id,m.business_id,$1 FROM memberships m WHERE m.business_id=$2 AND m.role<>'owner'`,
	} {
		args := []any{branch, biz}
		if _, err := pool.Underlying().Exec(t.Context(), q, args...); err != nil {
			t.Fatal(err)
		}
	}
	base := newServer(t, pool)
	clients := map[tenant.Role]*client{tenant.RoleOwner: login(t, base, "owner@matrix.test", biz.String())}
	for _, r := range roles {
		clients[r] = login(t, base, string(r)+"@matrix.test", biz.String())
	}

	endpoints := guardedEndpoints(t)
	if len(endpoints) < 100 {
		t.Fatalf("pembacaan rute ber-guard mencurigakan: hanya %d", len(endpoints))
	}
	checks := 0
	for _, ep := range endpoints {
		path := sampleIDParam.ReplaceAllString(ep.path, branch.String())
		for role, c := range clients {
			var body any
			if ep.method != "GET" {
				body = map[string]any{}
			}
			code, _ := c.do(ep.method, path, body, nil)
			grant := authz.Check(role, ep.perm)
			denied := grant == authz.Denied || (grant == authz.ReadOnly && ep.write)
			checks++
			switch {
			case denied && code != 403:
				t.Errorf("%s %s sebagai %s: izin %s ditolak matriks tetapi hasilnya %d (seharusnya 403)", ep.method, ep.path, role, ep.perm, code)
			case !denied && (code == 401 || code == 403):
				t.Errorf("%s %s sebagai %s: izin %s diizinkan matriks tetapi hasilnya %d", ep.method, ep.path, role, ep.perm, code)
			}
		}
	}
	t.Logf("%d endpoint x %d peran = %d pemeriksaan", len(endpoints), len(clients), checks)
}

// Rute tingkat akun: hanya butuh sesi dan hanya menyentuh data milik pengguna itu sendiri,
// sehingga header bisnis tidak relevan. Menambah rute di sini wajib ditinjau.
var authOnlyRoutes = map[string]bool{
	"GET /api/v1/me":           true,
	"GET /api/v1/me/profile":   true,
	"GET /api/v1/me/sessions":  true,
	"POST /api/v1/auth/logout": true,
	// Tidak membaca konteks tenant: keanggotaan diperiksa dari isi body / token undangan, atau hanya menyentuh data pengguna sendiri.
	"PUT /api/v1/session/business":           true,
	"POST /api/v1/invitations/accept":        true,
	"PUT /api/v1/auth/password":              true,
	"PUT /api/v1/me/profile":                 true,
	"POST /api/v1/me/sessions/revoke-others": true,
}

// NFR-013: pemilik bisnis B yang mengirim X-Business-ID milik bisnis A harus ditolak (403) di setiap endpoint ber-tenant.
func TestUserOfAnotherBusinessIsRejectedOnEveryTenantEndpoint(t *testing.T) {
	pool := testdb.New(t)
	bizA := seedBusiness(t, pool, "Toko A", "owner@a.isolation.test")
	bizB := seedBusiness(t, pool, "Toko B", "owner@b.isolation.test")
	base := newServer(t, pool)
	outsider := login(t, base, "owner@b.isolation.test", bizB.String())
	outsider.biz = bizA.String()

	routes, _ := registeredRoutes(t)
	checked := 0
	for _, route := range routes {
		if publicRoutes[route] || authOnlyRoutes[route] {
			continue
		}
		method, path, _ := strings.Cut(route, " ")
		path = sampleIDParam.ReplaceAllString(path, sampleID)
		var body any
		if method != "GET" {
			body = map[string]any{}
		}
		code, _ := outsider.do(method, path, body, nil)
		checked++
		if code != 403 {
			t.Errorf("%s dengan header bisnis lain menghasilkan %d, seharusnya 403", route, code)
		}
	}
	if checked < 150 {
		t.Fatalf("hanya %d rute diperiksa", checked)
	}
	t.Logf("%d rute diperiksa", checked)
}

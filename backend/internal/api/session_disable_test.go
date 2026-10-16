package apiserver

import (
	"testing"

	"github.com/storepos/storepos/internal/platform/testdb"
)

// CORE-004: akun yang dinonaktifkan kehilangan akses pada request berikutnya, tanpa menunggu sesi kedaluwarsa.
func TestDisabledUserLosesSessionImmediatelyAndCannotLogIn(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@disable.test")
	base := newServer(t, pool)
	c := login(t, base, "owner@disable.test", biz.String())
	if code, _ := c.do("GET", "/api/v1/me", nil, nil); code != 200 {
		t.Fatalf("sebelum dinonaktifkan: %d", code)
	}
	if _, err := pool.Underlying().Exec(t.Context(), `UPDATE users SET status='disabled' WHERE email='owner@disable.test'`); err != nil {
		t.Fatal(err)
	}
	if code, _ := c.do("GET", "/api/v1/me", nil, nil); code != 401 {
		t.Fatalf("sesi akun nonaktif harus ditolak: %d", code)
	}
	if code, _ := c.do("POST", "/api/v1/auth/login", map[string]string{"email": "owner@disable.test", "password": "rahasia-123"}, nil); code != 401 {
		t.Fatalf("login akun nonaktif harus ditolak: %d", code)
	}
}

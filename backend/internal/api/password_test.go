package apiserver

import (
	"testing"

	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestChangePasswordPolicyWrongCurrentAndSessionRevocation(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@pwd.test")
	base := newServer(t, pool)
	a := login(t, base, "owner@pwd.test", biz.String())
	b := login(t, base, "owner@pwd.test", biz.String()) // sesi kedua (perangkat lain)

	cases := []struct {
		name string
		body map[string]string
		want int
	}{
		{"kata sandi lama salah", map[string]string{"current_password": "salah-total", "new_password": "cangkir-kopi-9"}, 422},
		{"kata sandi baru terlalu umum", map[string]string{"current_password": "rahasia-123", "new_password": "password123"}, 422},
		{"kata sandi baru terlalu pendek", map[string]string{"current_password": "rahasia-123", "new_password": "abc"}, 422},
		{"sama dengan yang lama", map[string]string{"current_password": "rahasia-123", "new_password": "rahasia-123"}, 422},
	}
	for _, c := range cases {
		if code, _ := a.do("PUT", "/api/v1/auth/password", c.body, nil); code != c.want {
			t.Fatalf("%s: %d", c.name, code)
		}
	}
	if code, _ := b.do("GET", "/api/v1/me", nil, nil); code != 200 {
		t.Fatalf("penolakan tidak boleh mencabut sesi: %d", code)
	}

	if code, _ := a.do("PUT", "/api/v1/auth/password", map[string]string{"current_password": "rahasia-123", "new_password": "cangkir-kopi-9"}, nil); code != 200 {
		t.Fatalf("ganti kata sandi: %d", code)
	}
	if code, _ := a.do("GET", "/api/v1/me", nil, nil); code != 200 {
		t.Fatalf("sesi saat ini harus tetap aktif: %d", code)
	}
	if code, _ := b.do("GET", "/api/v1/me", nil, nil); code != 401 {
		t.Fatalf("sesi lain harus dicabut: %d", code)
	}
	if code, _ := a.do("POST", "/api/v1/auth/login", map[string]string{"email": "owner@pwd.test", "password": "rahasia-123"}, nil); code != 401 {
		t.Fatalf("kata sandi lama harus ditolak: %d", code)
	}
	if code, _ := a.do("POST", "/api/v1/auth/login", map[string]string{"email": "owner@pwd.test", "password": "cangkir-kopi-9"}, nil); code != 200 {
		t.Fatalf("kata sandi baru harus berlaku: %d", code)
	}

	var leaks int
	err := pool.Underlying().QueryRow(t.Context(), `SELECT count(*) FROM audit_logs WHERE action='auth.password_changed' AND (before_jsonb::text LIKE '%cangkir%' OR after_jsonb::text LIKE '%cangkir%' OR after_jsonb::text LIKE '%rahasia%')`).Scan(&leaks)
	if err != nil || leaks != 0 {
		t.Fatalf("audit memuat rahasia: %v %d", err, leaks)
	}
	var n int
	if err := pool.Underlying().QueryRow(t.Context(), `SELECT count(*) FROM audit_logs WHERE action='auth.password_changed'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit ganti kata sandi: %v %d", err, n)
	}
}

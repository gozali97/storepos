package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestDisplayNameProfileVersionAndAuditPrivacy(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@profile.test")
	c := login(t, newServer(t, pool), "owner@profile.test", biz.String())
	path := "/api/v1/me/profile"
	if code, r := c.do("GET", path, nil, nil); code != 200 || r["version"] != float64(1) {
		t.Fatal(code, r)
	}
	if code, r := c.do("PUT", path, map[string]string{"display_name": "Kasir Rahasia"}, map[string]string{"If-Match": "1"}); code != 200 {
		t.Fatal(code, r)
	}
	if code, _ := c.do("PUT", path, map[string]string{"display_name": "Usang"}, map[string]string{"If-Match": "1"}); code != 409 {
		t.Fatalf("version: %d", code)
	}
	if code, _ := c.do("PUT", path, map[string]string{"display_name": "bad\nname"}, map[string]string{"If-Match": "2"}); code != 422 {
		t.Fatalf("control chars: %d", code)
	}
	var n int
	if err := pool.Underlying().QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action='user.profile_changed' AND after_jsonb::text LIKE '%Kasir Rahasia%'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("audit PII: %d %v", n, err)
	}
}

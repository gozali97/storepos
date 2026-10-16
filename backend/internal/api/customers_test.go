package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestCustomerAnonymizationAndTenant(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@customer.test")
	other := seedBusiness(t, pool, "Other", "other@customer.test")
	seedUser(t, pool, "cashier@customer.test", "cashier", biz)
	base := newServer(t, pool)
	c := login(t, base, "owner@customer.test", biz.String())
	b := login(t, base, "other@customer.test", other.String())
	cashier := login(t, base, "cashier@customer.test", biz.String())
	code, x := c.do("POST", "/api/v1/customers", map[string]string{"name": "Nama Privat", "phone": "08123456789", "note": "catatan privat"}, nil)
	if code != 201 {
		t.Fatal(code, x)
	}
	path := "/api/v1/customers/" + x["id"].(string)
	hdr := map[string]string{"If-Match": "1"}
	if code, _ := b.do("DELETE", path, nil, hdr); code != 404 {
		t.Fatalf("tenant: %d", code)
	}
	if code, _ := cashier.do("DELETE", path, nil, hdr); code != 403 {
		t.Fatalf("erasure permission: %d", code)
	}
	if code, _ := c.do("DELETE", path, nil, hdr); code != 204 {
		t.Fatalf("anonymize: %d", code)
	}
	var name, phone, note string
	if err := pool.Underlying().QueryRow(context.Background(), `SELECT name,phone,note FROM customers WHERE id=$1`, x["id"]).Scan(&name, &phone, &note); err != nil {
		t.Fatal(err)
	}
	if name != "Pelanggan dihapus" || phone != "" || note != "" {
		t.Fatal("PII remains")
	}
	code, r := c.do("GET", "/api/v1/customers", nil, nil)
	if code != 200 || len(r["customers"].([]any)) != 0 {
		t.Fatal(code, r)
	}
	var leaks int
	if err := pool.Underlying().QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE before_jsonb::text LIKE '%Nama Privat%' OR after_jsonb::text LIKE '%08123456789%'`).Scan(&leaks); err != nil || leaks != 0 {
		t.Fatalf("audit PII: %d %v", leaks, err)
	}
}

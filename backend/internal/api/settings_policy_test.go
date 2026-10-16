package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestOperationalPolicyVersionsAndCashEnforcement(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@policy.test")
	seedUser(t, pool, "manager@policy.test", "manager", biz)
	base := newServer(t, pool)
	c := login(t, base, "owner@policy.test", biz.String())
	manager := login(t, base, "manager@policy.test", biz.String())
	path := "/api/v1/settings/operational-policy"
	code, p := c.do("GET", path, nil, nil)
	if code != 200 || p["policy"].(map[string]any)["cash_out_approval_cents"] != "50000000" {
		t.Fatal(code, p)
	}
	body := map[string]any{"cashier_discount_bps": 500, "cash_out_approval_cents": "10000", "variance_approval_cents": "100"}
	if code, _ := manager.do("PUT", path, body, map[string]string{"If-Match": "1"}); code != 403 {
		t.Fatalf("manager: %d", code)
	}
	if code, r := c.do("PUT", path, body, map[string]string{"If-Match": "1"}); code != 200 {
		t.Fatal(code, r)
	}
	if code, _ := c.do("PUT", path, body, map[string]string{"If-Match": "1"}); code != 409 {
		t.Fatalf("stale: %d", code)
	}
	var branch, reg string
	db := pool.Underlying()
	ctx := context.Background()
	if err := db.QueryRow(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES(gen_random_uuid(),$1,'A','A') RETURNING id::text`, biz).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO registers(id,business_id,branch_id,name) VALUES(gen_random_uuid(),$1,$2,'K') RETURNING id::text`, biz, branch).Scan(&reg); err != nil {
		t.Fatal(err)
	}
	if code, r := c.do("POST", "/api/v1/shifts/open", map[string]string{"branch_id": branch, "register_id": reg, "opening_cash_cents": "0"}, nil); code != 201 {
		t.Fatal(code, r)
	}
	if code, _ := c.do("POST", "/api/v1/shifts/cash", map[string]string{"branch_id": branch, "register_id": reg, "kind": "out", "amount_cents": "20000", "reason": "test"}, nil); code != 403 {
		t.Fatalf("configured cash threshold ignored: %d", code)
	}
	if code, _ := c.do("POST", "/api/v1/shifts/close", map[string]string{"branch_id": branch, "register_id": reg, "counted_cash_cents": "1000"}, nil); code != 422 {
		t.Fatalf("configured variance ignored: %d", code)
	}
	body["cash_rounding_multiple_cents"] = "101"
	if code, _ := c.do("PUT", path, body, map[string]string{"If-Match": "2"}); code != 422 {
		t.Fatalf("fractional rupiah multiple: %d", code)
	}
	body["cash_rounding_multiple_cents"] = "50000"
	body["cash_rounding_enabled"] = false
	if code, r := c.do("PUT", path, body, map[string]string{"If-Match": "2"}); code != 200 {
		t.Fatal(code, r)
	}
	if code, r := c.do("GET", path, nil, nil); code != 200 || r["policy"].(map[string]any)["cash_rounding_enabled"] != false || r["policy"].(map[string]any)["cash_rounding_multiple_cents"] != "50000" {
		t.Fatal(code, r)
	}
	body["cashier_discount_bps"] = 10001
	if code, _ := c.do("PUT", path, body, map[string]string{"If-Match": "3"}); code != 422 {
		t.Fatalf("invalid: %d", code)
	}
}

package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestBusinessTypePolicyOwnerVersionAndLock(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@type.test")
	seedUser(t, pool, "manager@type.test", "manager", biz)
	base := newServer(t, pool)
	owner := login(t, base, "owner@type.test", biz.String())
	manager := login(t, base, "manager@type.test", biz.String())
	path := "/api/v1/settings/business-type"
	if code, _ := manager.do("PUT", path, map[string]string{"business_type": "restaurant"}, map[string]string{"If-Match": "1"}); code != 403 {
		t.Fatalf("manager: %d", code)
	}
	if code, r := owner.do("PUT", path, map[string]string{"business_type": "restaurant"}, map[string]string{"If-Match": "1"}); code != 200 {
		t.Fatal(code, r)
	}
	if code, _ := owner.do("PUT", path, map[string]string{"business_type": "retail"}, map[string]string{"If-Match": "1"}); code != 409 {
		t.Fatalf("stale: %d", code)
	}
	if _, err := pool.Underlying().Exec(context.Background(), `UPDATE businesses SET first_sale_completed_at=now() WHERE id=$1`, biz); err != nil {
		t.Fatal(err)
	}
	if code, _ := owner.do("PUT", path, map[string]string{"business_type": "retail"}, map[string]string{"If-Match": "2"}); code != 409 {
		t.Fatalf("locked: %d", code)
	}
	if code, r := owner.do("GET", path, nil, nil); code != 200 || r["locked"] != true {
		t.Fatal(code, r)
	}
}

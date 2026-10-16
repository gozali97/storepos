package apiserver

import (
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestOrganizationPermissionsVersionsAndInactiveRegister(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@org.test")
	seedUser(t, pool, "manager@org.test", "manager", biz)
	base := newServer(t, pool)
	owner := login(t, base, "owner@org.test", biz.String())
	manager := login(t, base, "manager@org.test", biz.String())
	body := map[string]string{"code": "WITA", "name": "Bali", "timezone": "Asia/Makassar", "receipt_header": "Toko"}
	if code, _ := manager.do("POST", "/api/v1/branches", body, nil); code != 403 {
		t.Fatalf("manager manage: %d", code)
	}
	body["timezone"] = "Invalid/Timezone"
	if code, _ := owner.do("POST", "/api/v1/branches", body, nil); code != 422 {
		t.Fatalf("timezone: %d", code)
	}
	body["timezone"] = "Asia/Makassar"
	code, b := owner.do("POST", "/api/v1/branches", body, nil)
	if code != 201 {
		t.Fatal(code, b)
	}
	branch := b["branch_id"].(string)
	if code, x := owner.do("GET", "/api/v1/branches/"+branch, nil, nil); code != 200 || x["timezone"] != "Asia/Makassar" || x["version"] != float64(1) {
		t.Fatal(code, x)
	}
	if code, _ := manager.do("GET", "/api/v1/branches/"+branch, nil, nil); code != 403 {
		t.Fatalf("manager unassigned branch: %d", code)
	}
	if code, r := owner.do("PUT", "/api/v1/branches/"+branch, body, map[string]string{"If-Match": "1"}); code != 200 {
		t.Fatal(code, r)
	}
	if code, _ := owner.do("PUT", "/api/v1/branches/"+branch, body, map[string]string{"If-Match": "1"}); code != 409 {
		t.Fatalf("version: %d", code)
	}
	code, reg := owner.do("POST", "/api/v1/branches/"+branch+"/registers", map[string]string{"name": "K1"}, nil)
	if code != 201 {
		t.Fatal(code, reg)
	}
	id := reg["register_id"].(string)
	if code, x := owner.do("GET", "/api/v1/registers/"+id, nil, nil); code != 200 || x["branch_id"] != branch || x["version"] != float64(1) {
		t.Fatal(code, x)
	}
	path := "/api/v1/registers/" + id + "/status"
	if code, _ := owner.do("PATCH", path, map[string]string{"status": "inactive"}, map[string]string{"If-Match": "1"}); code != 200 {
		t.Fatalf("inactive: %d", code)
	}
	open := map[string]string{"branch_id": branch, "register_id": id, "opening_cash_cents": "0"}
	if code, _ := owner.do("POST", "/api/v1/shifts/open", open, nil); code != 404 {
		t.Fatalf("open inactive: %d", code)
	}
	if code, _ := owner.do("PATCH", path, map[string]string{"status": "active"}, map[string]string{"If-Match": "2"}); code != 200 {
		t.Fatalf("active: %d", code)
	}
	if code, r := owner.do("POST", "/api/v1/shifts/open", open, nil); code != 201 {
		t.Fatal(code, r)
	}
	if code, _ := owner.do("PATCH", path, map[string]string{"status": "inactive"}, map[string]string{"If-Match": "3"}); code != 409 {
		t.Fatalf("disable open shift: %d", code)
	}
	body["timezone"] = "Asia/Jakarta"
	if code, _ := owner.do("PUT", "/api/v1/branches/"+branch, body, map[string]string{"If-Match": "2"}); code != 409 {
		t.Fatalf("timezone open shift: %d", code)
	}
}

package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestMembershipRoleRestrictionsDisableAndLastOwner(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@member.test")
	seedUser(t, pool, "manager@member.test", "manager", biz)
	seedUser(t, pool, "cashier@member.test", "cashier", biz)
	db := pool.Underlying()
	ctx := context.Background()
	var ownerID, cashierID string
	if err := db.QueryRow(ctx, `SELECT m.id::text FROM memberships m JOIN users u ON u.id=m.user_id WHERE u.email='owner@member.test'`).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SELECT m.id::text FROM memberships m JOIN users u ON u.id=m.user_id WHERE u.email='cashier@member.test'`).Scan(&cashierID); err != nil {
		t.Fatal(err)
	}
	base := newServer(t, pool)
	owner := login(t, base, "owner@member.test", biz.String())
	manager := login(t, base, "manager@member.test", biz.String())
	cashier := login(t, base, "cashier@member.test", biz.String())
	body := map[string]any{"role": "owner", "status": "active", "branch_ids": []string{}}
	if code, _ := manager.do("PUT", "/api/v1/memberships/"+cashierID, body, map[string]string{"If-Match": "1"}); code != 403 {
		t.Fatalf("manager escalation: %d", code)
	}
	body["role"] = "cashier"
	if code, _ := owner.do("PUT", "/api/v1/memberships/"+ownerID, body, map[string]string{"If-Match": "1"}); code != 409 {
		t.Fatalf("last owner: %d", code)
	}
	body["status"] = "disabled"
	if code, r := owner.do("PUT", "/api/v1/memberships/"+cashierID, body, map[string]string{"If-Match": "1"}); code != 200 {
		t.Fatal(code, r)
	}
	if code, _ := cashier.do("GET", "/api/v1/catalog/search", nil, nil); code != 403 {
		t.Fatalf("old session access: %d", code)
	}
	if code, _ := owner.do("PUT", "/api/v1/memberships/"+cashierID, body, map[string]string{"If-Match": "1"}); code != 409 {
		t.Fatalf("stale: %d", code)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action='membership.changed'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit: %d %v", n, err)
	}
	if code, r := owner.do("GET", "/api/v1/memberships", nil, nil); code != 200 || len(r["memberships"].([]any)) != 3 {
		t.Fatal(code, r)
	}
}

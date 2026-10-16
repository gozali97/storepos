package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestAuditLogTenantBranchPermissionsAndCursor(t *testing.T) {
	pool := testdb.New(t)
	a := seedBusiness(t, pool, "A", "owner@audit.test")
	b := seedBusiness(t, pool, "B", "other@audit.test")
	seedUser(t, pool, "manager@audit.test", "manager", a)
	seedUser(t, pool, "cashier@audit.test", "cashier", a)
	ctx := context.Background()
	db := pool.Underlying()
	first, second := idkit.New(), idkit.New()
	for _, branch := range []idkit.ID{first, second} {
		if _, err := db.Exec(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES($1,$2,$3,'T')`, branch, a, branch.String()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, `INSERT INTO membership_branches(membership_id,business_id,branch_id) SELECT m.id,m.business_id,$2 FROM memberships m JOIN users u ON u.id=m.user_id WHERE m.business_id=$1 AND u.email='manager@audit.test'`, a, first); err != nil {
		t.Fatal(err)
	}
	insert := func(biz idkit.ID, branch *idkit.ID) string {
		t.Helper()
		id := idkit.New()
		if _, err := db.Exec(ctx, `INSERT INTO audit_logs(id,business_id,branch_id,action,entity_type,occurred_at,after_jsonb) VALUES($1,$2,$3,'test.event','test','2026-10-13T00:00:00Z','{"version":1}')`, id, biz, branch); err != nil {
			t.Fatal(err)
		}
		return id.String()
	}
	insert(a, &first)
	insert(a, &first)
	hidden := insert(a, &second)
	insert(a, nil)
	insert(b, nil)
	base := newServer(t, pool)
	owner := login(t, base, "owner@audit.test", a.String())
	manager := login(t, base, "manager@audit.test", a.String())
	cashier := login(t, base, "cashier@audit.test", a.String())
	path := "/api/v1/audit-logs?from=2026-10-12T00:00:00Z&until=2026-10-14T00:00:00Z"
	code, r := owner.do("GET", path, nil, nil)
	if code != 200 || len(r["events"].([]any)) != 4 {
		t.Fatal(code, r)
	}
	code, r = manager.do("GET", path+"&limit=1", nil, nil)
	if code != 200 || len(r["events"].([]any)) != 1 || r["next_cursor"] == "" {
		t.Fatal(code, r)
	}
	firstID := r["events"].([]any)[0].(map[string]any)["id"]
	code, next := manager.do("GET", path+"&limit=1&before="+r["next_cursor"].(string), nil, nil)
	if code != 200 || len(next["events"].([]any)) != 1 || next["events"].([]any)[0].(map[string]any)["id"] == firstID || next["next_cursor"] != "" {
		t.Fatal(code, next)
	}
	if code, _ := manager.do("GET", path+"&branch_id="+second.String(), nil, nil); code != 403 {
		t.Fatalf("branch: %d", code)
	}
	if code, _ := manager.do("GET", path+"&before="+hidden, nil, nil); code != 422 {
		t.Fatalf("hidden cursor: %d", code)
	}
	if code, _ := cashier.do("GET", path, nil, nil); code != 403 {
		t.Fatalf("cashier audit permission: %d", code)
	}
	if code, _ := owner.do("GET", path+"&action=no.event", nil, nil); code != 200 {
		t.Fatalf("filter: %d", code)
	}
}

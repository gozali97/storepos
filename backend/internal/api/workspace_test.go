package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestWorkspaceRestrictsBranchesRegistersAndTenant(t *testing.T) {
	pool := testdb.New(t)
	a := seedBusiness(t, pool, "A", "owner@workspace.test")
	b := seedBusiness(t, pool, "B", "other@workspace.test")
	seedUser(t, pool, "cashier@workspace.test", "cashier", a)
	ctx := context.Background()
	db := pool.Underlying()
	first, second := idkit.New(), idkit.New()
	for _, id := range []idkit.ID{first, second} {
		if _, err := db.Exec(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES($1,$2,$3,'Cabang')`, id, a, id.String()); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, `INSERT INTO registers(id,business_id,branch_id,name) VALUES($1,$2,$3,'Kasir')`, idkit.New(), a, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, `INSERT INTO registers(id,business_id,branch_id,name,status) VALUES($1,$2,$3,'Tidak aktif','inactive')`, idkit.New(), a, first); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO membership_branches(membership_id,business_id,branch_id) SELECT m.id,m.business_id,$2 FROM memberships m JOIN users u ON u.id=m.user_id WHERE m.business_id=$1 AND u.email='cashier@workspace.test'`, a, first); err != nil {
		t.Fatal(err)
	}
	base := newServer(t, pool)
	owner := login(t, base, "owner@workspace.test", a.String())
	cashier := login(t, base, "cashier@workspace.test", a.String())
	code, result := owner.do("GET", "/api/v1/workspace", nil, nil)
	if code != 200 || len(result["branches"].([]any)) != 2 || len(result["registers"].([]any)) != 2 {
		t.Fatal(code, result)
	}
	code, result = cashier.do("GET", "/api/v1/workspace", nil, nil)
	if code != 200 || len(result["branches"].([]any)) != 1 || len(result["registers"].([]any)) != 1 {
		t.Fatal(code, result)
	}
	if result["branches"].([]any)[0].(map[string]any)["id"] != first.String() {
		t.Fatal("cabang tanpa penugasan terekspos")
	}
	outsider := *owner
	outsider.biz = b.String()
	if code, _ := outsider.do("GET", "/api/v1/workspace", nil, nil); code != 403 {
		t.Fatal("akses lintas tenant", code)
	}
}

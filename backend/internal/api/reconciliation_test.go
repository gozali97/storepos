package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestReconciliationFindingsTenantAndPermissions(t *testing.T) {
	pool := testdb.New(t)
	a := seedBusiness(t, pool, "A", "owner@recon.test")
	b := seedBusiness(t, pool, "B", "other@recon.test")
	seedUser(t, pool, "cashier@recon.test", "cashier", a)
	ctx := context.Background()
	db := pool.Underlying()
	if _, err := db.Exec(ctx, `INSERT INTO reconciliation_runs(run_date) VALUES('2026-10-13')`); err != nil {
		t.Fatal(err)
	}
	for _, biz := range []idkit.ID{a, b} {
		if _, err := db.Exec(ctx, `INSERT INTO reconciliation_findings(id,run_date,business_id,kind,entity_key,expected_value,actual_value) VALUES($1,'2026-10-13',$2,'stock','test',1,2)`, idkit.New(), biz); err != nil {
			t.Fatal(err)
		}
	}
	base := newServer(t, pool)
	owner := login(t, base, "owner@recon.test", a.String())
	cashier := login(t, base, "cashier@recon.test", a.String())
	path := "/api/v1/reconciliation/findings?date=2026-10-13"
	code, r := owner.do("GET", path, nil, nil)
	if code != 200 || len(r["findings"].([]any)) != 1 {
		t.Fatal(code, r)
	}
	if code, _ := cashier.do("GET", path, nil, nil); code != 403 {
		t.Fatalf("cashier: %d", code)
	}
	if code, _ := owner.do("GET", "/api/v1/reconciliation/findings?date=bad", nil, nil); code != 422 {
		t.Fatalf("date: %d", code)
	}
}

package reporting

import (
	"context"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestCleanupPreservesFreshAndLockedDraftsAnd72HourKeys(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	db := pool.Underlying()
	user, biz, branch, reg := idkit.New(), idkit.New(), idkit.New(), idkit.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO users(id,email,password_hash) VALUES($1,'cleanup@invalid.test','x')`, user)
	exec(`INSERT INTO businesses(id,name,business_type,created_by) VALUES($1,'T','retail',$2)`, biz, user)
	exec(`INSERT INTO branches(id,business_id,code,name) VALUES($1,$2,'A','A')`, branch, biz)
	exec(`INSERT INTO registers(id,business_id,branch_id,name) VALUES($1,$2,$3,'K')`, reg, biz, branch)
	old, locked, fresh, completed := idkit.New(), idkit.New(), idkit.New(), idkit.New()
	for _, id := range []idkit.ID{old, locked, fresh, completed} {
		exec(`INSERT INTO sale_drafts(id,business_id,branch_id,register_id,status,payload,created_by) VALUES($1,$2,$3,$4,'held','{}',$5)`, id, biz, branch, reg, user)
	}
	exec(`UPDATE sale_drafts SET updated_at=now()-interval '31 days' WHERE id<>$1`, fresh)
	exec(`UPDATE sale_drafts SET status='completed' WHERE id=$1`, completed)
	exec(`INSERT INTO idempotency_keys(business_id,key,endpoint,request_hash,created_at,expires_at) VALUES($1,'old','test','x',now()-interval '73 hours',now()-interval '1 hour'),($1,'fresh','test','x',now()-interval '1 hour',now()-interval '1 minute'),($1,'unexpired','test','x',now()-interval '100 hours',now()+interval '1 day')`, biz)
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT id FROM sale_drafts WHERE id=$1 FOR UPDATE`, locked); err != nil {
		t.Fatal(err)
	}
	result, err := Cleanup(ctx, pool)
	if err != nil || result.Drafts != 1 || result.IdempotencyKeys != 1 {
		t.Fatal(result, err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM sale_drafts`).Scan(&n); err != nil || n != 3 {
		t.Fatal(n, err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action='draft.expired' AND actor_user_id IS NULL`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	result, err = Cleanup(ctx, pool)
	if err != nil || result.Drafts != 1 || result.IdempotencyKeys != 0 {
		t.Fatal(result, err)
	}
	result, err = Cleanup(ctx, pool)
	if err != nil || result.Drafts != 0 || result.IdempotencyKeys != 0 {
		t.Fatal(result, err)
	}
}

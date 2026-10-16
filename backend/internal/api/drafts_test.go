package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"strconv"
	"testing"
)

func TestDraftPersistenceHoldLimitsAndVersions(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@draft.test")
	ctx := context.Background()
	db := pool.Underlying()
	var branch, reg string
	if err := db.QueryRow(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES(gen_random_uuid(),$1,'JKT','Pusat') RETURNING id::text`, biz).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO registers(id,business_id,branch_id,name) VALUES(gen_random_uuid(),$1,$2,'K1') RETURNING id::text`, biz, branch).Scan(&reg); err != nil {
		t.Fatal(err)
	}
	c := login(t, newServer(t, pool), "owner@draft.test", biz.String())
	cart := map[string]any{"lines": []any{}, "note": "keranjang tahan refresh"}
	create := func() (int, map[string]any) {
		return c.do("POST", "/api/v1/drafts", map[string]any{"branch_id": branch, "register_id": reg, "cart": cart}, nil)
	}
	code, r := create()
	if code != 201 {
		t.Fatalf("create: %d %v", code, r)
	}
	id := r["id"].(string)
	if code, _ := create(); code != 409 {
		t.Fatalf("one active: %d", code)
	}
	code, r = c.do("GET", "/api/v1/drafts?register_id="+reg, nil, nil)
	if code != 200 || len(r["drafts"].([]any)) != 1 || r["drafts"].([]any)[0].(map[string]any)["cart"].(map[string]any)["note"] != "keranjang tahan refresh" {
		t.Fatalf("reload: %d %v", code, r)
	}
	update := func(id, status string, v int) (int, map[string]any) {
		return c.do("PUT", "/api/v1/drafts/"+id, map[string]any{"status": status, "cart": cart}, map[string]string{"If-Match": strconv.Itoa(v)})
	}
	if code, _ := update(id, "held", 1); code != 200 {
		t.Fatalf("hold: %d", code)
	}
	if code, _ := update(id, "active", 1); code != 409 {
		t.Fatalf("stale version: %d", code)
	}
	if code, _ := update(id, "active", 2); code != 200 {
		t.Fatalf("resume: %d", code)
	}
	if code, _ := update(id, "held", 3); code != 200 {
		t.Fatalf("hold again: %d", code)
	}
	for i := 0; i < 9; i++ {
		code, r = create()
		if code != 201 {
			t.Fatal(code, r)
		}
		if code, _ := update(r["id"].(string), "held", 1); code != 200 {
			t.Fatal(code)
		}
	}
	code, r = create()
	if code != 201 {
		t.Fatal(code, r)
	}
	last := r["id"].(string)
	if code, _ := update(last, "held", 1); code != 409 {
		t.Fatalf("ten held limit: %d", code)
	}
	if code, _ := update(last, "cancelled", 1); code != 200 {
		t.Fatalf("cancel: %d", code)
	}
	if code, _ := update(last, "active", 2); code != 409 {
		t.Fatalf("cancelled terminal state: %d", code)
	}
}

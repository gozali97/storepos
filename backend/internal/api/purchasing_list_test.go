package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestReceiptSearchFiltersCursorAndBranchScope(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@receipts.test")
	seedUser(t, pool, "warehouse@receipts.test", "warehouse", biz)
	db := pool.Underlying()
	ctx := context.Background()
	var uid string
	if err := db.QueryRow(ctx, `SELECT id::text FROM users WHERE email='owner@receipts.test'`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	branch := idkit.New()
	if _, err := db.Exec(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES($1,$2,'A','A')`, branch, biz); err != nil {
		t.Fatal(err)
	}
	supplier := idkit.New()
	if _, err := db.Exec(ctx, `INSERT INTO suppliers(id,business_id,name,created_by) VALUES($1,$2,'Supplier',$3)`, supplier, biz, uid); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := db.Exec(ctx, `INSERT INTO goods_receipts(id,business_id,branch_id,received_date,invoice_number,created_by,supplier_id,created_at) VALUES($1,$2,$3,'2026-10-13','INV',$4,$5,'2026-10-13T00:00:00Z')`, idkit.New(), biz, branch, uid, supplier); err != nil {
			t.Fatal(err)
		}
	}
	base := newServer(t, pool)
	owner := login(t, base, "owner@receipts.test", biz.String())
	warehouse := login(t, base, "warehouse@receipts.test", biz.String())
	path := "/api/v1/goods-receipts?branch_id=" + branch.String() + "&from=2026-10-13&to=2026-10-13&status=draft&invoice_number=INV&supplier_id=" + supplier.String() + "&limit=2"
	code, r := owner.do("GET", path, nil, nil)
	if code != 200 || len(r["receipts"].([]any)) != 2 || r["next_cursor"] == "" {
		t.Fatal(code, r)
	}
	code, last := owner.do("GET", path+"&before="+r["next_cursor"].(string), nil, nil)
	if code != 200 || len(last["receipts"].([]any)) != 1 || last["next_cursor"] != "" {
		t.Fatal(code, last)
	}
	if code, _ := warehouse.do("GET", path, nil, nil); code != 403 {
		t.Fatalf("branch: %d", code)
	}
	if code, _ := owner.do("GET", path+"&before="+idkit.New().String(), nil, nil); code != 422 {
		t.Fatalf("cursor: %d", code)
	}
	code, posted := owner.do("GET", "/api/v1/goods-receipts?branch_id="+branch.String()+"&from=2026-10-13&to=2026-10-13&status=posted", nil, nil)
	if code != 200 || len(posted["receipts"].([]any)) != 0 {
		t.Fatal(code, posted)
	}
}

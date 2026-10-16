package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"strconv"
	"testing"
)

func TestGoodsReceiptConversionAndCost(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@buy.test")
	db := pool.Underlying()
	var branch string
	if err := db.QueryRow(context.Background(), `INSERT INTO branches(id,business_id,code,name) VALUES(gen_random_uuid(),$1,'JKT','Pusat') RETURNING id::text`, biz).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	otherBiz := seedBusiness(t, pool, "Other", "other@buy.test")
	seedUser(t, pool, "warehouse@buy.test", "warehouse", biz)
	base := newServer(t, pool)
	c := login(t, base, "owner@buy.test", biz.String())
	other := login(t, base, "other@buy.test", otherBiz.String())
	unassigned := login(t, base, "warehouse@buy.test", biz.String())
	code, p := c.do("POST", "/api/v1/products", map[string]any{"name": "Kopi", "variants": []map[string]string{{"sku": "BUY1", "price_cents": "300000"}}}, nil)
	if code != 201 {
		t.Fatal(code, p)
	}
	vid := p["variants"].([]any)[0].(map[string]any)["id"].(string)
	code, u := c.do("POST", "/api/v1/purchase-units", map[string]string{"variant_id": vid, "name": "Dus", "factor_milli": "24000"}, nil)
	if code != 201 {
		t.Fatal(code, u)
	}
	for i := 0; i < 2; i++ {
		price := "4800000"
		if i == 1 {
			price = "9600000"
		}
		createBody := map[string]any{"branch_id": branch, "received_date": "2026-10-13", "lines": []map[string]string{{"unit_id": u["unit_id"].(string), "qty_milli": "2000", "purchase_price_cents": price}}}
		createHeaders := map[string]string{"Idempotency-Key": "create-receipt-" + strconv.Itoa(i)}
		code, r := c.do("POST", "/api/v1/goods-receipts", createBody, createHeaders)
		if code != 201 {
			t.Fatal(code, r)
		}
		if replayCode, replay := c.do("POST", "/api/v1/goods-receipts", createBody, createHeaders); replayCode != 201 || replay["receipt_id"] != r["receipt_id"] {
			t.Fatalf("create replay: %d %v", replayCode, replay)
		}
		createBody["invoice_number"] = "changed"
		if conflictCode, _ := c.do("POST", "/api/v1/goods-receipts", createBody, createHeaders); conflictCode != 409 {
			t.Fatalf("create key payload conflict: %d", conflictCode)
		}
		var qty int64
		if i == 0 {
			if err := db.QueryRow(context.Background(), `SELECT COALESCE(SUM(qty_milli),0) FROM inventory_balances`).Scan(&qty); err != nil {
				t.Fatal(err)
			}
			if qty != 0 {
				t.Fatal("draft changed stock")
			}
		}
		detailPath := "/api/v1/goods-receipts/" + r["receipt_id"].(string)
		if code, d := c.do("GET", detailPath, nil, nil); code != 200 || d["status"] != "draft" || d["lines"].([]any)[0].(map[string]any)["variant_id"] != vid {
			t.Fatal(code, d)
		}
		if code, _ := other.do("GET", detailPath, nil, nil); code != 404 {
			t.Fatalf("cross-tenant detail: %d", code)
		}
		if code, _ := unassigned.do("GET", detailPath, nil, nil); code != 403 {
			t.Fatalf("unassigned branch: %d", code)
		}
		edit := map[string]any{"received_date": "2026-10-13", "invoice_number": "INV-EDIT", "lines": []map[string]string{{"unit_id": u["unit_id"].(string), "qty_milli": "2000", "purchase_price_cents": price}}}
		if code, _ := other.do("PUT", detailPath, edit, map[string]string{"If-Match": "1"}); code != 404 {
			t.Fatalf("cross-tenant edit: %d", code)
		}
		if code, _ := unassigned.do("PUT", detailPath, edit, map[string]string{"If-Match": "1"}); code != 403 {
			t.Fatalf("unassigned edit: %d", code)
		}
		if code, d := c.do("PUT", detailPath, edit, map[string]string{"If-Match": "1"}); code != 200 {
			t.Fatal(code, d)
		}
		if code, _ := c.do("PUT", detailPath, edit, map[string]string{"If-Match": "1"}); code != 409 {
			t.Fatalf("stale edit: %d", code)
		}
		path := detailPath + "/post"
		if code, res := c.do("POST", path, nil, map[string]string{"If-Match": "2", "Idempotency-Key": r["receipt_id"].(string)}); code != 200 {
			t.Fatal(code, res)
		}
		if code, replayed := c.do("POST", path, nil, map[string]string{"If-Match": "2", "Idempotency-Key": r["receipt_id"].(string)}); code != 200 || replayed["version"] != float64(3) {
			t.Fatalf("posting replay: %d %v", code, replayed)
		}
		if code, _ := c.do("POST", path, nil, map[string]string{"If-Match": "3", "Idempotency-Key": r["receipt_id"].(string)}); code != 409 {
			t.Fatalf("key/version conflict: %d", code)
		}
		if code, d := c.do("GET", detailPath, nil, nil); code != 200 || d["status"] != "posted" || d["version"] != float64(3) || d["lines"].([]any)[0].(map[string]any)["base_qty_milli"] != "48000" {
			t.Fatal(code, d)
		}
		if code, _ := c.do("PUT", detailPath, edit, map[string]string{"If-Match": "3"}); code != 409 {
			t.Fatalf("posted edit: %d", code)
		}
		if code, _ := c.do("POST", path, nil, map[string]string{"If-Match": "2"}); code != 409 {
			t.Fatalf("double post: %d", code)
		}
	}
	var qty, cost int64
	if err := db.QueryRow(context.Background(), `SELECT qty_milli,average_cost_cents FROM inventory_balances`).Scan(&qty, &cost); err != nil {
		t.Fatal(err)
	}
	if qty != 96000 || cost != 300000 {
		t.Fatalf("moving average: qty=%d cost=%d", qty, cost)
	}
	if _, err := db.Exec(context.Background(), `UPDATE purchase_units SET factor_milli=1000`); err == nil {
		t.Fatal("used conversion factor changed")
	}
}

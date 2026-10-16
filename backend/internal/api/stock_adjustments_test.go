package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"testing"
)

func TestStockAdjustmentReplayAndNegativeRollback(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@stock.test")
	db := pool.Underlying()
	ctx := context.Background()
	var branch string
	if err := db.QueryRow(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES(gen_random_uuid(),$1,'JKT','Pusat') RETURNING id::text`, biz).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	c := login(t, newServer(t, pool), "owner@stock.test", biz.String())
	code, p := c.do("POST", "/api/v1/products", map[string]any{"name": "Kopi", "variants": []map[string]string{{"sku": "K1", "price_cents": "100000"}}}, nil)
	if code != 201 {
		t.Fatal(code, p)
	}
	vid := p["variants"].([]any)[0].(map[string]any)["id"].(string)
	body := map[string]string{"branch_id": branch, "variant_id": vid, "qty_milli": "5000", "reason": "correction"}
	header := map[string]string{"Idempotency-Key": "adjust-in"}
	for i := 0; i < 2; i++ {
		if code, r := c.do("POST", "/api/v1/stock/adjustments", body, header); code != 201 || r["balance_after_milli"] != "5000" {
			t.Fatal(code, r)
		}
	}
	body["qty_milli"] = "-6000"
	if code, _ := c.do("POST", "/api/v1/stock/adjustments", body, map[string]string{"Idempotency-Key": "negative"}); code != 409 {
		t.Fatalf("negative: %d", code)
	}
	body["reason"] = "other"
	if code, _ := c.do("POST", "/api/v1/stock/adjustments", body, map[string]string{"Idempotency-Key": "other"}); code != 422 {
		t.Fatalf("reason: %d", code)
	}
	body["reason"] = "damaged"
	body["qty_milli"] = "-1000"
	if code, _ := c.do("POST", "/api/v1/stock/adjustments", body, map[string]string{"Idempotency-Key": "adjust-out"}); code != 201 {
		t.Fatalf("out: %d", code)
	}
	code, card := c.do("GET", "/api/v1/stock/card?branch_id="+branch+"&variant_id="+vid, nil, nil)
	if code != 200 || len(card["movements"].([]any)) != 2 {
		t.Fatal(code, card)
	}
	var balance, delta int64
	if err := db.QueryRow(ctx, `SELECT qty_milli FROM inventory_balances`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SELECT SUM(qty_milli) FROM stock_movements`).Scan(&delta); err != nil {
		t.Fatal(err)
	}
	if balance != 4000 || delta != balance {
		t.Fatalf("reconciliation: %d %d", balance, delta)
	}
}

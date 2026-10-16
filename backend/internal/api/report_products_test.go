package apiserver

import (
	"context"
	"encoding/csv"
	"github.com/storepos/storepos/internal/platform/testdb"
	"net/http"
	"testing"
)

func TestProductAndCategoryCSVMatchJSON(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@profitcsv.test")
	db := pool.Underlying()
	ctx := context.Background()
	var branch, reg string
	if err := db.QueryRow(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES(gen_random_uuid(),$1,'A','A') RETURNING id::text`, biz).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO registers(id,business_id,branch_id,name) VALUES(gen_random_uuid(),$1,$2,'K') RETURNING id::text`, biz, branch).Scan(&reg); err != nil {
		t.Fatal(err)
	}
	base := newServer(t, pool)
	owner := login(t, base, "owner@profitcsv.test", biz.String())
	code, p := owner.do("POST", "/api/v1/products", map[string]any{"name": "Kopi", "variants": []map[string]string{{"sku": "PCSV", "price_cents": "1500000"}}}, nil)
	if code != 201 {
		t.Fatal(code, p)
	}
	vid := p["variants"].([]any)[0].(map[string]any)["id"].(string)
	if code, r := owner.do("POST", "/api/v1/stock/adjustments", map[string]string{"branch_id": branch, "variant_id": vid, "qty_milli": "10000", "reason": "correction"}, map[string]string{"Idempotency-Key": "stock"}); code != 201 {
		t.Fatal(code, r)
	}
	if _, err := db.Exec(ctx, `UPDATE inventory_balances SET average_cost_cents=500000`); err != nil {
		t.Fatal(err)
	}
	if code, r := owner.do("POST", "/api/v1/shifts/open", map[string]string{"branch_id": branch, "register_id": reg, "opening_cash_cents": "0"}, nil); code != 201 {
		t.Fatal(code, r)
	}
	code, sale := owner.do("POST", "/api/v1/sales/finalize", map[string]any{"branch_id": branch, "register_id": reg, "lines": []map[string]string{{"variant_id": vid, "qty_milli": "1000"}}, "payments": []map[string]string{{"method": "cash", "amount_cents": "1500000"}}}, map[string]string{"Idempotency-Key": "sell"})
	if code != 201 {
		t.Fatal(code, sale)
	}
	if _, err := db.Exec(ctx, `UPDATE sales SET completed_at='2026-10-13T01:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	for _, group := range []string{"product", "category"} {
		path := "/api/v1/reports/products?branch_id=" + branch + "&from=2026-10-13&to=2026-10-13&group_by=" + group
		code, r := owner.do("GET", path, nil, nil)
		if code != 200 || len(r["rows"].([]any)) != 1 {
			t.Fatal(code, r)
		}
		row := r["rows"].([]any)[0].(map[string]any)
		req, err := http.NewRequest("GET", base+path+"&format=csv", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(HeaderBusinessID, biz.String())
		res, err := owner.hc.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		records, err := csv.NewReader(res.Body).ReadAll()
		res.Body.Close()
		if err != nil || len(records) != 2 {
			t.Fatal(records, err)
		}
		for i, key := range records[0] {
			if key == "net_sales_cents" || key == "estimated_gross_profit_cents" || key == "sold_cost_cents" {
				if records[1][i] != row[key] {
					t.Fatalf("CSV/JSON %s: %s %v", key, records[1][i], row[key])
				}
			}
		}
	}
}

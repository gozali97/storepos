package apiserver

import (
	"context"
	"encoding/csv"
	"github.com/storepos/storepos/internal/platform/testdb"
	"net/http"
	"testing"
)

func TestStockReportValuationFiltersAndCostPermissions(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Toko", "owner@stockreport.test")
	seedUser(t, pool, "cashier@stockreport.test", "cashier", biz)
	db := pool.Underlying()
	ctx := context.Background()
	var branch string
	if err := db.QueryRow(ctx, `INSERT INTO branches(id,business_id,code,name) VALUES(gen_random_uuid(),$1,'A','A') RETURNING id::text`, biz).Scan(&branch); err != nil {
		t.Fatal(err)
	}
	base := newServer(t, pool)
	owner := login(t, base, "owner@stockreport.test", biz.String())
	cashier := login(t, base, "cashier@stockreport.test", biz.String())
	if _, err := db.Exec(ctx, `INSERT INTO membership_branches(membership_id,business_id,branch_id) SELECT m.id,m.business_id,$2::uuid FROM memberships m JOIN users u ON u.id=m.user_id WHERE m.business_id=$1 AND u.email='cashier@stockreport.test'`, biz, branch); err != nil {
		t.Fatal(err)
	}
	code, p := owner.do("POST", "/api/v1/products", map[string]any{"name": "=FORMULA", "variants": []map[string]string{{"sku": "REPORT1", "price_cents": "100"}}}, nil)
	if code != 201 {
		t.Fatal(code, p)
	}
	vid := p["variants"].([]any)[0].(map[string]any)["id"].(string)
	if code, r := owner.do("POST", "/api/v1/stock/adjustments", map[string]string{"branch_id": branch, "variant_id": vid, "qty_milli": "5000", "reason": "correction"}, map[string]string{"Idempotency-Key": "report-opening"}); code != 201 {
		t.Fatal(code, r)
	}
	if code, r := owner.do("PUT", "/api/v1/stock/minimums/"+vid, map[string]string{"branch_id": branch, "minimum_qty_milli": "5000"}, map[string]string{"If-Match": "0"}); code != 200 {
		t.Fatal(code, r)
	}
	path := "/api/v1/reports/stock?branch_id=" + branch + "&from=2000-01-01&to=2000-01-02"
	code, r := owner.do("GET", path, nil, nil)
	if code != 200 {
		t.Fatal(code, r)
	}
	row := r["rows"].([]any)[0].(map[string]any)
	if row["qty_milli"] != "5000" || row["period_movement_milli"] != "0" || row["estimated_value_cents"] != nil || row["cost_unknown"] != true || row["below_minimum"] != true {
		t.Fatal(row)
	}
	if _, err := db.Exec(ctx, `UPDATE inventory_balances SET average_cost_cents=200000`); err != nil {
		t.Fatal(err)
	}
	code, r = owner.do("GET", path+"&filter=below_minimum", nil, nil)
	if code != 200 {
		t.Fatal(code, r)
	}
	row = r["rows"].([]any)[0].(map[string]any)
	if row["estimated_value_cents"] != "1000000" {
		t.Fatal(row)
	}
	code, r = cashier.do("GET", path, nil, nil)
	if code != 200 {
		t.Fatal(code, r)
	}
	row = r["rows"].([]any)[0].(map[string]any)
	if row["average_cost_cents"] != nil || row["estimated_value_cents"] != nil || r["cost_visible"] != false {
		t.Fatal("cost leak", r)
	}
	req, err := http.NewRequest("GET", base+path+"&format=csv", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(HeaderBusinessID, biz.String())
	res, err := owner.hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	records, err := csv.NewReader(res.Body).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1][11] != "1000000" || records[1][2][0] != '\'' {
		t.Fatal(records)
	}
	code, r = owner.do("GET", path+"&filter=negative", nil, nil)
	if code != 200 || len(r["rows"].([]any)) != 0 {
		t.Fatal(code, r)
	}
}

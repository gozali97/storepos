package apiserver

import (
	"net/url"
	"testing"
	"time"

	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestCustomerSalesFilterAndCursorCannotCrossCustomerOrTenant(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "A", "owner@customer.history.test")
	otherBiz := seedBusiness(t, pool, "B", "other@customer.history.test")
	base := newServer(t, pool)
	owner := login(t, base, "owner@customer.history.test", biz.String())
	other := login(t, base, "other@customer.history.test", otherBiz.String())
	customer := func(c *client, name string) string {
		t.Helper()
		code, x := c.do("POST", "/api/v1/customers", map[string]string{"name": name}, nil)
		if code != 201 {
			t.Fatal(code, x)
		}
		return x["id"].(string)
	}
	a, b, foreign := customer(owner, "A"), customer(owner, "B"), customer(other, "Foreign")
	branch, reg := idkit.New(), idkit.New()
	db := pool.Underlying()
	if _, err := db.Exec(t.Context(), `INSERT INTO branches(id,business_id,code,name) VALUES($1,$2,'A','A')`, branch, biz); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(t.Context(), `INSERT INTO registers(id,business_id,branch_id,name) VALUES($1,$2,$3,'K')`, reg, biz, branch); err != nil {
		t.Fatal(err)
	}
	if code, out := owner.do("POST", "/api/v1/shifts/open", map[string]string{"branch_id": branch.String(), "register_id": reg.String(), "opening_cash_cents": "0"}, nil); code != 201 {
		t.Fatal(code, out)
	}
	code, p := owner.do("POST", "/api/v1/products", map[string]any{"name": "Service", "track_stock": false, "variants": []map[string]string{{"sku": "HIST-1", "price_cents": "100000"}}}, nil)
	if code != 201 {
		t.Fatal(code, p)
	}
	variant := p["variants"].([]any)[0].(map[string]any)["id"].(string)
	sell := func(cid, key string) string {
		t.Helper()
		code, x := owner.do("POST", "/api/v1/sales/finalize", map[string]any{"branch_id": branch.String(), "register_id": reg.String(), "customer_id": cid, "lines": []map[string]string{{"variant_id": variant, "qty_milli": "1000"}}, "payments": []map[string]string{{"method": "cash", "amount_cents": "100000"}}}, map[string]string{"Idempotency-Key": key})
		if code != 201 {
			t.Fatal(code, x)
		}
		return x["sale_id"].(string)
	}
	saleA, saleB := sell(a, "cust-a"), sell(b, "cust-b")
	q := url.Values{"branch_id": {branch.String()}, "from": {time.Now().AddDate(0, 0, -1).Format("2006-01-02")}, "to": {time.Now().AddDate(0, 0, 1).Format("2006-01-02")}, "customer_id": {a}}
	path := "/api/v1/sales?"
	if code, out := owner.do("GET", path+q.Encode(), nil, nil); code != 200 || len(out["sales"].([]any)) != 1 || out["sales"].([]any)[0].(map[string]any)["sale_id"] != saleA {
		t.Fatalf("filter customer: %d %v", code, out)
	}
	q.Set("before", saleB)
	if code, _ := owner.do("GET", path+q.Encode(), nil, nil); code != 422 {
		t.Fatalf("cursor belonging to another customer: %d", code)
	}
	q.Del("before")
	q.Set("customer_id", foreign)
	if code, _ := owner.do("GET", path+q.Encode(), nil, nil); code != 404 {
		t.Fatalf("foreign customer: %d", code)
	}
	q.Set("customer_id", "invalid")
	if code, _ := owner.do("GET", path+q.Encode(), nil, nil); code != 422 {
		t.Fatalf("malformed customer: %d", code)
	}
	q.Del("customer_id")
	if code, out := owner.do("GET", path+q.Encode(), nil, nil); code != 200 || len(out["sales"].([]any)) != 2 {
		t.Fatalf("unfiltered compatible: %d %v", code, out)
	}
}

package apiserver

import (
	"strings"
	"testing"

	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestTwentyOperationalContractsMatchLiveHTTP(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Operations", "owner@operations.contract.test")
	branch, reg := idkit.New(), idkit.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO branches(id,business_id,code,name) VALUES($1,$2,'OPS','Operations')`, []any{branch, biz}},
		{`INSERT INTO registers(id,business_id,branch_id,name) VALUES($1,$2,$3,'Cashier')`, []any{reg, biz, branch}},
	} {
		if _, err := pool.Underlying().Exec(t.Context(), q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	c := login(t, newServer(t, pool), "owner@operations.contract.test", biz.String())
	contracts := operationalContracts()
	seen := map[string]bool{}
	call := func(route, path string, body any, version, key string) map[string]any {
		t.Helper()
		contract, ok := contracts[route]
		if !ok {
			t.Fatal(route)
		}
		if contract.Request != nil {
			if err := checkContractSchema(contract.Request, body, "request"); err != nil {
				t.Fatalf("%s request: %v", route, err)
			}
		}
		headers := map[string]string{}
		if version != "" {
			headers["If-Match"] = version
		}
		if key != "" {
			headers["Idempotency-Key"] = key
		}
		method, _, _ := strings.Cut(route, " ")
		code, out := c.do(method, path, body, headers)
		if code != contract.Status {
			t.Fatalf("%s status=%d want=%d: %v", route, code, contract.Status, out)
		}
		if err := checkContractSchema(contract.Response, out, "response"); err != nil {
			t.Fatalf("%s response: %v", route, err)
		}
		seen[route] = true
		return out
	}
	setup := func(path string, body any) map[string]any {
		t.Helper()
		code, out := c.do("POST", path, body, nil)
		if code != 201 {
			t.Fatal(path, code, out)
		}
		return out
	}
	policy := call("GET /api/v1/settings/operational-policy", "/api/v1/settings/operational-policy", nil, "", "")
	call("PUT /api/v1/settings/operational-policy", "/api/v1/settings/operational-policy", policy["policy"], "1", "")
	call("PUT /api/v1/memberships/me/pin", "/api/v1/memberships/me/pin", map[string]any{"pin": "123456"}, "", "")
	shiftQuery := "?branch_id=" + branch.String() + "&register_id=" + reg.String()
	call("GET /api/v1/shifts/open", "/api/v1/shifts/open"+shiftQuery, nil, "", "")
	shift := call("POST /api/v1/shifts/open", "/api/v1/shifts/open", map[string]any{"branch_id": branch.String(), "register_id": reg.String(), "opening_cash_cents": "100000"}, "", "")
	call("GET /api/v1/shifts/open", "/api/v1/shifts/open"+shiftQuery, nil, "", "")
	call("POST /api/v1/shifts/cash", "/api/v1/shifts/cash", map[string]any{"branch_id": branch.String(), "register_id": reg.String(), "kind": "in", "amount_cents": "1000", "reason": "change"}, "", "")
	expense := call("POST /api/v1/expenses", "/api/v1/expenses", map[string]any{"branch_id": branch.String(), "expense_date": "2026-10-14", "category": "Rent", "amount_cents": "10000", "method": "outside_cash", "note": "operations"}, "", "expense-contract")
	period := "?branch_id=" + branch.String() + "&from=2026-10-14&to=2026-10-14"
	call("GET /api/v1/expenses", "/api/v1/expenses"+period, nil, "", "")
	call("GET /api/v1/expenses/{id}", "/api/v1/expenses/"+expense["expense_id"].(string), nil, "", "")
	call("POST /api/v1/shifts/close", "/api/v1/shifts/close", map[string]any{"branch_id": branch.String(), "register_id": reg.String(), "counted_cash_cents": "101000", "note": "balanced"}, "", "")
	call("GET /api/v1/shifts/{id}/summary", "/api/v1/shifts/"+shift["shift_id"].(string)+"/summary", nil, "", "")
	product := setup("/api/v1/products", map[string]any{"name": "Stock item", "track_stock": true, "variants": []map[string]string{{"sku": "OPS-STOCK", "price_cents": "100000"}}})
	variant := product["variants"].([]any)[0].(map[string]any)["id"].(string)
	call("POST /api/v1/stock/receipts", "/api/v1/stock/receipts", map[string]any{"branch_id": branch.String(), "lines": []any{map[string]any{"variant_id": variant, "qty_milli": "1000"}}}, "", "")
	call("PUT /api/v1/stock/variants/{id}/opening-cost", "/api/v1/stock/variants/"+variant+"/opening-cost", map[string]any{"branch_id": branch.String(), "average_cost_cents": "50000", "reason": "Opening"}, "", "opening-contract")
	call("PUT /api/v1/stock/minimums/{variant}", "/api/v1/stock/minimums/"+variant, map[string]any{"branch_id": branch.String(), "minimum_qty_milli": "10000"}, "0", "")
	call("GET /api/v1/stock/reorder", "/api/v1/stock/reorder?branch_id="+branch.String(), nil, "", "")
	unit := setup("/api/v1/purchase-units", map[string]string{"variant_id": variant, "name": "Box", "factor_milli": "1000"})
	body := map[string]any{"branch_id": branch.String(), "received_date": "2026-10-14", "invoice_number": "OPS-INV", "lines": []any{map[string]any{"unit_id": unit["unit_id"], "qty_milli": "2000", "purchase_price_cents": "50000"}}}
	receipt := call("POST /api/v1/goods-receipts", "/api/v1/goods-receipts", body, "", "receipt-contract")
	rp := "/api/v1/goods-receipts/" + receipt["receipt_id"].(string)
	call("GET /api/v1/goods-receipts", "/api/v1/goods-receipts"+period, nil, "", "")
	call("GET /api/v1/goods-receipts/{id}", rp, nil, "", "")
	delete(body, "branch_id")
	call("PUT /api/v1/goods-receipts/{id}", rp, body, "1", "")
	call("POST /api/v1/goods-receipts/{id}/post", rp+"/post", nil, "2", "post-contract")
	call("GET /api/v1/goods-receipts/{id}", rp, nil, "", "")
	call("GET /api/v1/goods-receipts", "/api/v1/goods-receipts"+period, nil, "", "")
	for route := range contracts {
		if !seen[route] {
			t.Errorf("missing HTTP coverage: %s", route)
		}
	}
	t.Logf("%d operational contracts verified against HTTP/PostgreSQL", len(seen))
}

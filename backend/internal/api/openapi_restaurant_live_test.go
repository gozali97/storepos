package apiserver

import (
	"strings"
	"testing"

	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestTwentyRestaurantContractsMatchLiveHTTP(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Restaurant", "owner@restaurant.contract.test")
	branch := idkit.New()
	if _, err := pool.Underlying().Exec(t.Context(), `UPDATE businesses SET business_type='restaurant',enabled_modules=ARRAY['restaurant'] WHERE id=$1`, biz); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Underlying().Exec(t.Context(), `INSERT INTO branches(id,business_id,code,name) VALUES($1,$2,'R','Restaurant')`, branch, biz); err != nil {
		t.Fatal(err)
	}
	c := login(t, newServer(t, pool), "owner@restaurant.contract.test", biz.String())
	batch := restaurantConfigContracts()
	seen := map[string]bool{}
	call := func(route, path string, body any, version string) map[string]any {
		t.Helper()
		contract, ok := batch[route]
		if !ok {
			t.Fatalf("unknown route %s", route)
		}
		if contract.Request != nil {
			if err := checkContractSchema(contract.Request, body, "request"); err != nil {
				t.Fatalf("%s: %v", route, err)
			}
		}
		method, _, _ := strings.Cut(route, " ")
		headers := map[string]string{}
		if version != "" {
			headers["If-Match"] = version
		}
		code, out := c.do(method, path, body, headers)
		if code != contract.Status {
			t.Fatalf("%s: status %d want %d: %v", route, code, contract.Status, out)
		}
		if err := checkContractSchema(contract.Response, out, "response"); err != nil {
			t.Fatalf("%s: %v", route, err)
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
	area := setup("/api/v1/restaurant/areas", map[string]any{"branch_id": branch.String(), "name": "Indoor"})
	table := setup("/api/v1/restaurant/tables", map[string]any{"branch_id": branch.String(), "area_id": area["area_id"], "name": "T1", "capacity": 4})
	suffix := "?branch_id=" + branch.String()
	call("GET /api/v1/restaurant/areas", "/api/v1/restaurant/areas"+suffix, nil, "")
	call("GET /api/v1/restaurant/tables", "/api/v1/restaurant/tables"+suffix, nil, "")
	tablePath := "/api/v1/restaurant/tables/" + table["table_id"].(string)
	call("PUT /api/v1/restaurant/tables/{id}", tablePath, map[string]any{"name": "T2", "capacity": float64(6)}, "1")
	call("DELETE /api/v1/restaurant/tables/{id}", tablePath, nil, "2")
	call("POST /api/v1/restaurant/tables/{id}/restore", tablePath+"/restore", nil, "3")
	call("POST /api/v1/restaurant/tables/{id}/clean", tablePath+"/clean", nil, "4")
	station := call("POST /api/v1/restaurant/stations", "/api/v1/restaurant/stations", map[string]any{"branch_id": branch.String(), "name": "Hot kitchen"}, "")
	call("GET /api/v1/restaurant/stations", "/api/v1/restaurant/stations"+suffix, nil, "")
	call("PUT /api/v1/restaurant/stations/{id}", "/api/v1/restaurant/stations/"+station["station_id"].(string), map[string]any{"name": "Kitchen", "archived": false}, "1")
	category := setup("/api/v1/categories", map[string]any{"name": "Food"})
	routePath := "/api/v1/restaurant/category-routes/" + category["id"].(string)
	call("PUT /api/v1/restaurant/category-routes/{id}", routePath, map[string]any{"branch_id": branch.String(), "station_id": station["station_id"]}, "0")
	call("GET /api/v1/restaurant/category-routes", "/api/v1/restaurant/category-routes"+suffix, nil, "")
	product := setup("/api/v1/products", map[string]any{"name": "Menu", "category_id": category["id"], "track_stock": false, "variants": []map[string]string{{"sku": "REST-CONTRACT", "price_cents": "100000"}}})
	pid := product["id"].(string)
	vid := product["variants"].([]any)[0].(map[string]any)["id"].(string)
	group := call("POST /api/v1/restaurant/modifier-groups", "/api/v1/restaurant/modifier-groups", map[string]any{"product_id": pid, "name": "Sugar", "min_choices": float64(0), "max_choices": float64(2)}, "")
	groupPath := "/api/v1/restaurant/modifier-groups/" + group["group_id"].(string)
	call("PUT /api/v1/restaurant/modifier-groups/{id}", groupPath, map[string]any{"name": "Sweetness", "min_choices": float64(0), "max_choices": float64(2), "archived": false}, "1")
	option := call("POST /api/v1/restaurant/modifier-groups/{id}/options", groupPath+"/options", map[string]any{"name": "Less sugar", "price_delta_cents": "0"}, "2")
	call("PUT /api/v1/restaurant/modifier-groups/{id}/options/{option}", groupPath+"/options/"+option["option_id"].(string), map[string]any{"name": "No sugar", "price_delta_cents": "10000", "active": true}, "3")
	call("GET /api/v1/restaurant/products/{id}/modifiers", "/api/v1/restaurant/products/"+pid+"/modifiers", nil, "")
	call("PUT /api/v1/restaurant/menu/{id}/availability", "/api/v1/restaurant/menu/"+vid+"/availability", map[string]any{"branch_id": branch.String(), "sold_out": true}, "0")
	call("GET /api/v1/restaurant/menu/availability", "/api/v1/restaurant/menu/availability"+suffix, nil, "")
	call("PUT /api/v1/restaurant/service-charge", "/api/v1/restaurant/service-charge", map[string]any{"branch_id": branch.String(), "order_type": "DINE_IN", "rate_bps": float64(500), "include_in_tax_base": true}, "0")
	call("GET /api/v1/restaurant/service-charge", "/api/v1/restaurant/service-charge"+suffix, nil, "")
	for route := range batch {
		if !seen[route] {
			t.Errorf("contract not exercised: %s", route)
		}
	}
	t.Logf("%d restaurant configuration operations verified", len(seen))
}

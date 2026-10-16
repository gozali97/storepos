package apiserver

import (
	"strings"
	"testing"

	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestTwentyOpenAPIContractsMatchLiveHTTP(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Contract", "owner@contract.test")
	c := login(t, newServer(t, pool), "owner@contract.test", biz.String())
	contracts := initialContracts()
	seen := map[string]bool{}
	call := func(route, path string, body any, version string) map[string]any {
		t.Helper()
		contract, ok := contracts[route]
		if !ok {
			t.Fatalf("unknown contract %s", route)
		}
		method, _, _ := strings.Cut(route, " ")
		if contract.Request != nil {
			if err := checkContractSchema(contract.Request, body, "request"); err != nil {
				t.Fatalf("%s %v", route, err)
			}
		}
		headers := map[string]string{}
		if version != "" {
			headers["If-Match"] = version
		}
		code, out := c.do(method, path, body, headers)
		if code != contract.Status {
			t.Fatalf("%s status=%d want=%d response=%v", route, code, contract.Status, out)
		}
		if err := checkContractSchema(contract.Response, out, "response"); err != nil {
			t.Fatalf("%s: %v", route, err)
		}
		seen[route] = true
		return out
	}
	call("GET /api/v1/me/profile", "/api/v1/me/profile", nil, "")
	call("PUT /api/v1/me/profile", "/api/v1/me/profile", map[string]any{"display_name": "Kasir"}, "1")
	call("GET /api/v1/settings/business-profile", "/api/v1/settings/business-profile", nil, "")
	call("PUT /api/v1/settings/business-profile", "/api/v1/settings/business-profile", map[string]any{"name": "Restoran"}, "1")
	call("GET /api/v1/settings/business-type", "/api/v1/settings/business-type", nil, "")
	call("PUT /api/v1/settings/business-type", "/api/v1/settings/business-type", map[string]any{"business_type": "restaurant"}, "2")
	code, p := c.do("POST", "/api/v1/products", map[string]any{"name": "Menu", "track_stock": false, "variants": []map[string]string{{"sku": "CONTRACT-1", "price_cents": "100000"}}}, nil)
	if code != 201 {
		t.Fatal(code, p)
	}
	variant := p["variants"].([]any)[0].(map[string]any)["id"].(string)
	path := "/api/v1/variants/" + variant + "/unit-defaults"
	call("GET /api/v1/variants/{id}/unit-defaults", path, nil, "")
	call("PUT /api/v1/variants/{id}/unit-defaults", path, map[string]any{"default_purchase_unit_id": nil, "default_sale_unit_id": nil}, "1")
	call("GET /api/v1/settings/tax", "/api/v1/settings/tax", nil, "")
	call("PUT /api/v1/settings/tax", "/api/v1/settings/tax", map[string]any{"name": "PPN", "rate_bps": float64(1100), "mode": "exclusive"}, "")
	call("GET /api/v1/settings/tax", "/api/v1/settings/tax", nil, "")
	call("GET /api/v1/onboarding", "/api/v1/onboarding", nil, "")
	call("PUT /api/v1/onboarding/dismiss", "/api/v1/onboarding/dismiss", map[string]any{"dismissed": true}, "")
	call("POST /api/v1/onboarding/events", "/api/v1/onboarding/events", map[string]any{"step": "business", "status": "completed"}, "")
	call("POST /api/v1/onboarding/demo", "/api/v1/onboarding/demo", nil, "")
	call("DELETE /api/v1/onboarding/demo", "/api/v1/onboarding/demo", nil, "")
	branch := idkit.New()
	if _, err := pool.Underlying().Exec(t.Context(), `INSERT INTO branches(id,business_id,code,name) VALUES($1,$2,'C','Contract')`, branch, biz); err != nil {
		t.Fatal(err)
	}
	area := call("POST /api/v1/restaurant/areas", "/api/v1/restaurant/areas", map[string]any{"branch_id": branch.String(), "name": "Indoor"}, "")
	areaPath := "/api/v1/restaurant/areas/" + area["area_id"].(string)
	call("PUT /api/v1/restaurant/areas/{id}", areaPath, map[string]any{"name": "Main"}, "1")
	call("DELETE /api/v1/restaurant/areas/{id}", areaPath, nil, "2")
	call("POST /api/v1/restaurant/areas/{id}/restore", areaPath+"/restore", nil, "3")
	call("POST /api/v1/restaurant/tables", "/api/v1/restaurant/tables", map[string]any{"branch_id": branch.String(), "area_id": area["area_id"], "name": "Meja 1", "capacity": float64(4)}, "")
	for route := range contracts {
		if !seen[route] {
			t.Errorf("contract without live test: %s", route)
		}
	}
	if len(seen) != 20 {
		t.Fatalf("only %d contracts tested", len(seen))
	}
	t.Logf("%d operation contracts verified against live HTTP/PostgreSQL", len(seen))
}

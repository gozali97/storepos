package apiserver

import (
	"strings"
	"testing"

	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestTwentyCatalogContractsMatchLiveHTTP(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Catalog", "owner@catalog.contract.test")
	c := login(t, newServer(t, pool), "owner@catalog.contract.test", biz.String())
	contracts := catalogContracts()
	seen := map[string]bool{}
	call := func(route, path string, body any, version string) map[string]any {
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
		method, _, _ := strings.Cut(route, " ")
		headers := map[string]string{}
		if version != "" {
			headers["If-Match"] = version
		}
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
	product := call("POST /api/v1/products", "/api/v1/products", map[string]any{"name": "Coffee", "track_stock": true, "variants": []any{map[string]any{"sku": "CAT-CONTRACT", "name": "Regular", "base_unit": "cup", "price_cents": "100000", "attributes": map[string]any{"size": "regular"}, "barcodes": []any{"BASE-CONTRACT"}}}}, "")
	pid := product["id"].(string)
	vid := product["variants"].([]any)[0].(map[string]any)["id"].(string)
	pp := "/api/v1/products/" + pid
	vp := "/api/v1/variants/" + vid
	call("GET /api/v1/products/{id}", pp, nil, "")
	call("PUT /api/v1/products/{id}", pp, map[string]any{"name": "Coffee edit", "category_id": nil}, "1")
	call("PATCH /api/v1/products/{id}/status", pp+"/status", map[string]any{"status": "inactive"}, "2")
	call("PATCH /api/v1/products/{id}/status", pp+"/status", map[string]any{"status": "active"}, "3")
	call("GET /api/v1/variants/{id}", vp, nil, "")
	call("PATCH /api/v1/variants/{id}/identity", vp+"/identity", map[string]any{"name": "Large", "sku": "CAT-CONTRACT-2"}, "1")
	call("PATCH /api/v1/variants/{id}/fractional", vp+"/fractional", map[string]any{"allow_fractional": true}, "2")
	call("PATCH /api/v1/variants/{id}/price", vp+"/price", map[string]any{"price_cents": "200000"}, "3")
	call("PATCH /api/v1/variants/{id}/status", vp+"/status", map[string]any{"status": "inactive"}, "4")
	call("PATCH /api/v1/variants/{id}/status", vp+"/status", map[string]any{"status": "active"}, "5")
	call("GET /api/v1/variants/{id}/barcodes", vp+"/barcodes", nil, "")
	call("DELETE /api/v1/variants/{id}/barcodes", vp+"/barcodes", map[string]any{"barcode": "BASE-CONTRACT"}, "6")
	unit := call("POST /api/v1/purchase-units", "/api/v1/purchase-units", map[string]any{"variant_id": vid, "name": "Box", "factor_milli": "10000"}, "")
	up := "/api/v1/purchase-units/" + unit["unit_id"].(string)
	call("GET /api/v1/purchase-units/{id}", up, nil, "")
	call("PUT /api/v1/purchase-units/{id}", up, map[string]any{"name": "Pack", "factor_milli": "12000", "archived": false}, "1")
	call("PUT /api/v1/purchase-units/{id}/sale-price", up+"/sale-price", map[string]any{"sale_price_cents": "2000000"}, "2")
	call("POST /api/v1/purchase-units/{id}/barcodes", up+"/barcodes", map[string]any{"barcode": "PACK-CONTRACT"}, "")
	call("GET /api/v1/variants/{id}/units", vp+"/units", nil, "")
	call("GET /api/v1/catalog/barcode", "/api/v1/catalog/barcode?barcode=PACK-CONTRACT", nil, "")
	call("GET /api/v1/catalog/search", "/api/v1/catalog/search?q=Coffee&limit=10", nil, "")
	call("GET /api/v1/catalog/search", "/api/v1/catalog/search?q=no-match-at-all", nil, "")
	// Isi kategori arsip agar schema array tidak hanya diuji pada hasil kosong.
	code, category := c.do("POST", "/api/v1/categories", map[string]any{"name": "Archive"}, nil)
	if code != 201 {
		t.Fatal(code, category)
	}
	if code, out := c.do("DELETE", "/api/v1/categories/"+category["id"].(string), nil, map[string]string{"If-Match": "1"}); code != 200 {
		t.Fatal(code, out)
	}
	call("GET /api/v1/categories/archived", "/api/v1/categories/archived", nil, "")
	for route := range contracts {
		if !seen[route] {
			t.Errorf("missing live request: %s", route)
		}
	}
	t.Logf("%d catalogue/unit operation contracts verified", len(seen))
}

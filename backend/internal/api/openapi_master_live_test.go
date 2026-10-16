package apiserver

import (
	"strings"
	"testing"

	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestTwentyMasterContractsMatchLiveHTTP(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Master contracts", "owner@master.contract.test")
	c := login(t, newServer(t, pool), "owner@master.contract.test", biz.String())
	contracts := masterContracts()
	seen := map[string]bool{}
	call := func(route, path string, body any, version string) map[string]any {
		t.Helper()
		contract := contracts[route]
		method, _, _ := strings.Cut(route, " ")
		if contract.Request != nil {
			if err := checkContractSchema(contract.Request, body, "request"); err != nil {
				t.Fatalf("%s: %v", route, err)
			}
		}
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
	category := call("POST /api/v1/categories", "/api/v1/categories", map[string]any{"name": "Minuman"}, "")
	catPath := "/api/v1/categories/" + category["id"].(string)
	call("GET /api/v1/categories", "/api/v1/categories", nil, "")
	call("GET /api/v1/categories/{id}", catPath, nil, "")
	call("PUT /api/v1/categories/{id}", catPath, map[string]any{"name": "Minuman hangat"}, "1")
	call("DELETE /api/v1/categories/{id}", catPath, nil, "2")
	call("POST /api/v1/categories/{id}/restore", catPath+"/restore", nil, "3")
	customer := call("POST /api/v1/customers", "/api/v1/customers", map[string]any{"name": "Pelanggan", "phone": "0812345", "note": "Catatan"}, "")
	customerPath := "/api/v1/customers/" + customer["id"].(string)
	call("GET /api/v1/customers", "/api/v1/customers?q=Pelanggan", nil, "")
	call("GET /api/v1/customers/{id}", customerPath, nil, "")
	call("PUT /api/v1/customers/{id}", customerPath, map[string]any{"name": "Pelanggan edit", "phone": "+62812345", "note": "Baru"}, "1")
	supplier := call("POST /api/v1/suppliers", "/api/v1/suppliers", map[string]any{"name": "Supplier", "contact": "Kontak", "address": "Alamat", "note": "Catatan", "archived": false}, "")
	supplierPath := "/api/v1/suppliers/" + supplier["id"].(string)
	call("GET /api/v1/suppliers", "/api/v1/suppliers", nil, "")
	call("GET /api/v1/suppliers/{id}", supplierPath, nil, "")
	call("PUT /api/v1/suppliers/{id}", supplierPath, map[string]any{"name": "Supplier arsip", "contact": "Kontak", "address": "Alamat", "note": "Catatan", "archived": true}, "1")
	branchBody := map[string]any{"code": "MASTER", "name": "Pusat", "timezone": "Asia/Jakarta", "address": "Alamat", "receipt_header": "Header", "receipt_footer": "Footer"}
	branch := call("POST /api/v1/branches", "/api/v1/branches", branchBody, "")
	branchPath := "/api/v1/branches/" + branch["branch_id"].(string)
	call("GET /api/v1/branches/{id}", branchPath, nil, "")
	branchBody["name"] = "Pusat edit"
	call("PUT /api/v1/branches/{id}", branchPath, branchBody, "1")
	register := call("POST /api/v1/branches/{id}/registers", branchPath+"/registers", map[string]any{"name": "Kasir utama"}, "")
	registerPath := "/api/v1/registers/" + register["register_id"].(string)
	call("GET /api/v1/registers/{id}", registerPath, nil, "")
	call("PATCH /api/v1/registers/{id}/status", registerPath+"/status", map[string]any{"status": "inactive"}, "1")
	for route := range contracts {
		if !seen[route] {
			t.Errorf("contract without live response: %s", route)
		}
	}
	t.Logf("%d master-data contracts verified against HTTP/PostgreSQL", len(seen))
}

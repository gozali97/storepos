package apiserver

import (
	"strings"
	"testing"
	"time"

	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/testdb"
)

func TestTwentyPharmacyRecoveryContractsMatchLiveHTTP(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Pharmacy", "owner@pharmacy.contract.test")
	seedUser(t, pool, "pharmacist@contract.test", "pharmacist", biz)
	branch := idkit.New()
	if _, err := pool.Underlying().Exec(t.Context(), `UPDATE businesses SET business_type='pharmacy',enabled_modules=ARRAY['pharmacy'] WHERE id=$1`, biz); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Underlying().Exec(t.Context(), `INSERT INTO branches(id,business_id,code,name) VALUES($1,$2,'APT','Pharmacy')`, branch, biz); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Underlying().Exec(t.Context(), `INSERT INTO membership_branches(membership_id,business_id,branch_id) SELECT id,business_id,$1 FROM memberships WHERE business_id=$2 AND role='pharmacist'`, branch, biz); err != nil {
		t.Fatal(err)
	}
	base := newServer(t, pool)
	owner := login(t, base, "owner@pharmacy.contract.test", biz.String())
	pharmacist := login(t, base, "pharmacist@contract.test", biz.String())
	contracts := pharmacyAndRecoveryContracts()
	seen := map[string]bool{}
	call := func(c *client, route, path string, body any, version, key string) map[string]any {
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
		if key != "" {
			headers["Idempotency-Key"] = key
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
	setup := func(path string, body any) map[string]any {
		t.Helper()
		code, out := owner.do("POST", path, body, nil)
		if code != 201 {
			t.Fatal(path, code, out)
		}
		return out
	}
	product := setup("/api/v1/products", map[string]any{"name": "Health item", "track_stock": true, "variants": []map[string]string{{"sku": "APT-CONTRACT", "price_cents": "100000"}}})
	pid := product["id"].(string)
	vid := product["variants"].([]any)[0].(map[string]any)["id"].(string)
	drugPath := "/api/v1/pharmacy/products/" + pid
	call(owner, "PUT /api/v1/pharmacy/products/{id}", drugPath, map[string]any{"active_ingredient": "example", "dosage_form": "tablet", "strength": "100 mg", "manufacturer": "Example", "registration_number": "TEST", "classification": "test only", "prescription_required": false, "special_restrictions": false}, "1", "")
	call(owner, "GET /api/v1/pharmacy/products/{id}", drugPath, nil, "", "")
	call(owner, "GET /api/v1/pharmacy/products", "/api/v1/pharmacy/products", nil, "", "")
	supplier := setup("/api/v1/suppliers", map[string]any{"name": "Supplier"})
	receiptBody := map[string]any{"branch_id": branch.String(), "variant_id": vid, "batch_number": "CONTRACT-B1", "expiry_date": time.Now().AddDate(1, 0, 0).Format("2006-01-02"), "supplier_id": supplier["id"], "invoice_number": "INV", "qty_milli": "10000", "unit_cost_cents": "50000", "status": "QUARANTINE"}
	receipt := call(owner, "POST /api/v1/pharmacy/batch-receipts", "/api/v1/pharmacy/batch-receipts", receiptBody, "", "receive-contract")
	bp := "/api/v1/pharmacy/batches/" + receipt["batch_id"].(string)
	query := "?branch_id=" + branch.String()
	call(owner, "GET /api/v1/pharmacy/batch-receipts", "/api/v1/pharmacy/batch-receipts"+query, nil, "", "")
	call(owner, "GET /api/v1/pharmacy/batches", "/api/v1/pharmacy/batches"+query, nil, "", "")
	call(pharmacist, "POST /api/v1/pharmacy/batches/{id}/release", bp+"/release", map[string]any{"branch_id": branch.String(), "qty_milli": "4000", "reason": "inspection"}, "", "release-contract")
	call(owner, "POST /api/v1/pharmacy/batches/{id}/dispose", bp+"/dispose", map[string]any{"branch_id": branch.String(), "qty_milli": "1000", "status": "QUARANTINE", "reason": "damaged"}, "", "dispose-contract")
	call(owner, "POST /api/v1/pharmacy/batches/{id}/count", bp+"/count", map[string]any{"branch_id": branch.String(), "status": "QUARANTINE", "counted_milli": "5000", "reason": "count"}, "", "count-contract")
	call(owner, "GET /api/v1/pharmacy/fefo-preview", "/api/v1/pharmacy/fefo-preview"+query+"&variant_id="+vid+"&qty_milli=2000", nil, "", "")
	call(owner, "GET /api/v1/pharmacy/batches/{id}/movements", bp+"/movements"+query, nil, "", "")
	call(owner, "GET /api/v1/pharmacy/batches/{id}/trace", bp+"/trace"+query, nil, "", "")
	// Penjualan snapshot untuk memastikan schema trace.sales juga diuji, bukan hanya array kosong.
	reg := idkit.New()
	if _, err := pool.Underlying().Exec(t.Context(), `INSERT INTO registers(id,business_id,branch_id,name) VALUES($1,$2,$3,'K')`, reg, biz, branch); err != nil {
		t.Fatal(err)
	}
	setup("/api/v1/shifts/open", map[string]any{"branch_id": branch.String(), "register_id": reg.String(), "opening_cash_cents": "0"})
	if code, out := owner.do("POST", "/api/v1/sales/finalize", map[string]any{"branch_id": branch.String(), "register_id": reg.String(), "lines": []map[string]string{{"variant_id": vid, "qty_milli": "1000"}}, "payments": []map[string]string{{"method": "cash", "amount_cents": "100000"}}}, map[string]string{"Idempotency-Key": "trace-sale-contract"}); code != 201 {
		t.Fatal(code, out)
	}
	call(owner, "GET /api/v1/pharmacy/batches/{id}/trace", bp+"/trace"+query, nil, "", "")
	// Opname stok inti diuji pada saldo inti tersendiri, tidak menganggap saldo batch = saldo inti.
	setup("/api/v1/stock/receipts", map[string]any{"branch_id": branch.String(), "lines": []map[string]string{{"variant_id": vid, "qty_milli": "2000"}}})
	count := call(owner, "POST /api/v1/stock/counts", "/api/v1/stock/counts", map[string]any{"branch_id": branch.String()}, "", "")
	cp := "/api/v1/stock/counts/" + count["count_id"].(string)
	call(owner, "GET /api/v1/stock/counts/{id}", cp, nil, "", "")
	call(owner, "PUT /api/v1/stock/counts/{id}/lines/{variant}", cp+"/lines/"+vid, map[string]any{"counted_qty_milli": "2000"}, "1", "")
	call(owner, "POST /api/v1/stock/counts/{id}/post", cp+"/post", nil, "2", "core-count-contract")
	call(owner, "GET /api/v1/stock/counts/{id}", cp, nil, "", "")
	call(owner, "GET /api/v1/stock/counts", "/api/v1/stock/counts"+query, nil, "", "")
	call(owner, "GET /api/v1/me/sessions", "/api/v1/me/sessions", nil, "", "")
	call(owner, "POST /api/v1/me/sessions/revoke-others", "/api/v1/me/sessions/revoke-others", nil, "", "")
	call(owner, "PUT /api/v1/auth/password", "/api/v1/auth/password", map[string]any{"current_password": "rahasia-123", "new_password": "new-contract-pass-123"}, "", "")
	for route := range contracts {
		if !seen[route] {
			t.Errorf("untested contract %s", route)
		}
	}
	t.Logf("%d pharmacy/count/session operations verified", len(seen))
}

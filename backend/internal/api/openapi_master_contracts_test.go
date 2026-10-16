package apiserver

import "testing"

func masterContracts() map[string]apiContract {
	name := objectSchema(map[string]any{"name": textSchema(200)}, "name")
	short := versionedSchema("id")
	category := objectSchema(map[string]any{"id": uuidSchema(false), "name": textSchema(200), "version": integerSchema(1)}, "id", "name", "version")
	categoryDetail := objectSchema(map[string]any{"id": uuidSchema(false), "name": textSchema(200), "version": integerSchema(1), "status": enumSchema("active", "archived"), "tax_rule_id": uuidSchema(true)}, "id", "name", "version", "status", "tax_rule_id")
	customerProps := func() map[string]any {
		return map[string]any{"id": uuidSchema(false), "name": textSchema(200), "phone": textSchema(32), "note": textSchema(200), "version": integerSchema(1)}
	}
	customer := objectSchema(customerProps(), "id", "name", "phone", "note", "version")
	cp := customerProps()
	cp["anonymized"] = boolSchema()
	customerDetail := objectSchema(cp, "id", "name", "phone", "note", "version", "anonymized")
	customerRequest := objectSchema(map[string]any{"name": textSchema(200), "phone": textSchema(32), "note": textSchema(200)}, "name")
	supplierProps := func() map[string]any {
		return map[string]any{"id": uuidSchema(false), "name": textSchema(200), "contact": textSchema(200), "address": textSchema(1000), "note": textSchema(1000), "archived": boolSchema(), "version": integerSchema(1)}
	}
	supplier := objectSchema(supplierProps(), "id", "name", "contact", "address", "note", "archived", "version")
	sp := supplierProps()
	delete(sp, "id")
	delete(sp, "version")
	supplierRequest := objectSchema(sp, "name")
	branchProps := func() map[string]any {
		return map[string]any{"code": textSchema(20), "name": textSchema(200), "timezone": textSchema(0), "address": textSchema(1000), "receipt_header": textSchema(1000), "receipt_footer": textSchema(1000)}
	}
	branchRequest := objectSchema(branchProps(), "code", "name", "timezone")
	bp := branchProps()
	bp["branch_id"] = uuidSchema(false)
	bp["status"] = enumSchema("active", "inactive", "archived")
	bp["version"] = integerSchema(1)
	branch := objectSchema(bp, "branch_id", "code", "name", "timezone", "address", "receipt_header", "receipt_footer", "status", "version")
	register := objectSchema(map[string]any{"register_id": uuidSchema(false), "branch_id": uuidSchema(false), "name": textSchema(200), "status": enumSchema("active", "inactive", "archived"), "version": integerSchema(1)}, "register_id", "branch_id", "name", "status", "version")
	cursor := textSchema(0)
	return map[string]apiContract{
		"POST /api/v1/categories":              {Request: name, Response: category, Status: 201},
		"GET /api/v1/categories":               {Response: objectSchema(map[string]any{"categories": arraySchema(category)}, "categories"), Status: 200},
		"GET /api/v1/categories/{id}":          {Response: categoryDetail, Status: 200},
		"PUT /api/v1/categories/{id}":          {Request: name, Response: short, Status: 200, Version: true},
		"DELETE /api/v1/categories/{id}":       {Response: objectSchema(map[string]any{"id": uuidSchema(false), "archived": boolSchema(), "version": integerSchema(1)}, "id", "archived", "version"), Status: 200, Version: true},
		"POST /api/v1/categories/{id}/restore": {Response: short, Status: 200, Version: true},
		"POST /api/v1/customers":               {Request: customerRequest, Response: customer, Status: 201},
		"GET /api/v1/customers":                {Response: objectSchema(map[string]any{"customers": arraySchema(customer), "next_cursor": cursor}, "customers", "next_cursor"), Status: 200, Query: map[string]map[string]any{"q": textSchema(100), "after": uuidSchema(false)}},
		"GET /api/v1/customers/{id}":           {Response: customerDetail, Status: 200},
		"PUT /api/v1/customers/{id}":           {Request: customerRequest, Response: short, Status: 200, Version: true},
		"POST /api/v1/suppliers":               {Request: supplierRequest, Response: supplier, Status: 201},
		"GET /api/v1/suppliers":                {Response: objectSchema(map[string]any{"suppliers": arraySchema(supplier), "next_cursor": cursor}, "suppliers", "next_cursor"), Status: 200, Query: map[string]map[string]any{"after": uuidSchema(false)}},
		"GET /api/v1/suppliers/{id}":           {Response: supplier, Status: 200},
		"PUT /api/v1/suppliers/{id}":           {Request: supplierRequest, Response: supplier, Status: 200, Version: true},
		"POST /api/v1/branches":                {Request: branchRequest, Response: versionedSchema("branch_id"), Status: 201},
		"GET /api/v1/branches/{id}":            {Response: branch, Status: 200},
		"PUT /api/v1/branches/{id}":            {Request: branchRequest, Response: versionedSchema("branch_id"), Status: 200, Version: true},
		"POST /api/v1/branches/{id}/registers": {Request: name, Response: versionedSchema("register_id"), Status: 201},
		"GET /api/v1/registers/{id}":           {Response: register, Status: 200},
		"PATCH /api/v1/registers/{id}/status":  {Request: objectSchema(map[string]any{"status": enumSchema("active", "inactive")}, "status"), Response: objectSchema(map[string]any{"register_id": uuidSchema(false), "status": enumSchema("active", "inactive"), "version": integerSchema(1)}, "register_id", "status", "version"), Status: 200, Version: true},
	}
}

func TestMasterContractBatchHasTwentyDistinctOperations(t *testing.T) {
	batch := masterContracts()
	if len(batch) != 20 {
		t.Fatalf("batch contains %d operations", len(batch))
	}
	initial := initialContracts()
	for route := range batch {
		if _, ok := initial[route]; ok {
			t.Errorf("duplicate contract: %s", route)
		}
	}
}

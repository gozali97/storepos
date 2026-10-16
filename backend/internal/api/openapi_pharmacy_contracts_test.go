package apiserver

import "testing"

func pharmacyAndRecoveryContracts() map[string]apiContract {
	money := map[string]any{"type": "string", "pattern": "^[0-9]+$"}
	signed := map[string]any{"type": "string", "pattern": "^-?[0-9]+$"}
	qty := map[string]any{"type": "string", "pattern": "^[1-9][0-9]*000$"}
	// Satu unit = 1000 milli. Hasil hitung nol sah, tetapi penerimaan/pelepasan harus positif.
	counted := map[string]any{"type": "string", "pattern": "^(0|[1-9][0-9]*000)$"}
	date := map[string]any{"type": "string", "format": "date"}
	at := map[string]any{"type": "string", "format": "date-time"}
	status := enumSchema("SELLABLE", "QUARANTINE")
	ok := objectSchema(map[string]any{"status": enumSchema("ok")}, "status")
	page := func(key string, row map[string]any) map[string]any {
		return objectSchema(map[string]any{key: arraySchema(row), "next_cursor": textSchema(0)}, key, "next_cursor")
	}
	branchQuery := func() map[string]map[string]any {
		return map[string]map[string]any{"branch_id": uuidSchema(false), "after": uuidSchema(false)}
	}
	drugProps := func() map[string]any {
		return map[string]any{"active_ingredient": textSchema(500), "dosage_form": textSchema(500), "strength": textSchema(500), "manufacturer": textSchema(500), "registration_number": textSchema(500), "classification": textSchema(500), "prescription_required": boolSchema(), "special_restrictions": boolSchema()}
	}
	drug := objectSchema(drugProps(), "active_ingredient", "dosage_form", "strength", "manufacturer", "registration_number", "classification", "prescription_required", "special_restrictions")
	drugs := objectSchema(map[string]any{"product_id": uuidSchema(false), "name": textSchema(200), "classification": textSchema(500), "prescription_required": boolSchema(), "special_restrictions": boolSchema(), "product_version": integerSchema(1)}, "product_id", "name", "classification", "prescription_required", "special_restrictions", "product_version")
	receipt := objectSchema(map[string]any{"receipt_id": uuidSchema(false), "batch_id": uuidSchema(false), "batch_number": textSchema(100), "expiry_date": date, "supplier_id": uuidSchema(false), "invoice_number": textSchema(200), "qty_milli": qty, "unit_cost_cents": money, "status": status, "created_at": at}, "receipt_id", "batch_id", "batch_number", "expiry_date", "supplier_id", "invoice_number", "qty_milli", "unit_cost_cents", "status", "created_at")
	batch := objectSchema(map[string]any{"batch_id": uuidSchema(false), "variant_id": uuidSchema(false), "sku": textSchema(100), "batch_number": textSchema(100), "expiry_date": date, "stored_status": enumSchema("SELLABLE", "QUARANTINE", "DISPOSED"), "qty_milli": money, "status": enumSchema("SELLABLE", "QUARANTINE", "EXPIRED", "DISPOSED"), "days_to_expiry": map[string]any{"type": "integer"}, "last_supplier_name": nullableSchema(textSchema(200))}, "batch_id", "variant_id", "sku", "batch_number", "expiry_date", "stored_status", "qty_milli", "status", "days_to_expiry", "last_supplier_name")
	allocation := objectSchema(map[string]any{"batch_id": uuidSchema(false), "batch_number": textSchema(100), "expiry_date": date, "qty_milli": qty, "days_to_expiry": map[string]any{"type": "integer"}}, "batch_id", "batch_number", "expiry_date", "qty_milli", "days_to_expiry")
	movement := objectSchema(map[string]any{"id": uuidSchema(false), "status": textSchema(0), "kind": textSchema(0), "qty_milli": signed, "balance_after_milli": money, "ref_id": uuidSchema(false), "reason": textSchema(200), "created_by": uuidSchema(false), "created_at": at}, "id", "status", "kind", "qty_milli", "balance_after_milli", "ref_id", "reason", "created_by", "created_at")
	action := objectSchema(map[string]any{"branch_id": uuidSchema(false), "qty_milli": qty, "status": status, "reason": textSchema(200)}, "branch_id", "qty_milli", "reason")
	countStatus := enumSchema("draft", "posted")
	countResponse := objectSchema(map[string]any{"count_id": uuidSchema(false), "status": countStatus, "version": integerSchema(1)}, "count_id", "status", "version")
	countLine := objectSchema(map[string]any{"variant_id": uuidSchema(false), "snapshot_qty_milli": signed, "system_qty_at_count": nullableSchema(textSchema(0)), "counted_qty_milli": nullableSchema(textSchema(0)), "counted_at": nullableSchema(map[string]any{"type": "string", "format": "date-time"})}, "variant_id", "snapshot_qty_milli", "system_qty_at_count", "counted_qty_milli", "counted_at")
	traceReceipt := objectSchema(map[string]any{"receipt_id": uuidSchema(false), "supplier_id": uuidSchema(false), "invoice_number": textSchema(200), "qty_milli": qty, "status": status, "created_at": at}, "receipt_id", "supplier_id", "invoice_number", "qty_milli", "status", "created_at")
	traceSale := objectSchema(map[string]any{"sale_id": uuidSchema(false), "receipt_number": textSchema(100), "status": enumSchema("completed", "voided"), "return_status": enumSchema("none", "partial", "full"), "qty_milli": money, "returned_milli": money, "override_reason": textSchema(200), "completed_at": at}, "sale_id", "receipt_number", "status", "return_status", "qty_milli", "returned_milli", "override_reason", "completed_at")
	session := objectSchema(map[string]any{"session_id": uuidSchema(false), "created_at": at, "last_seen_at": at, "expires_at": at, "user_agent": textSchema(0), "ip": textSchema(0), "current": boolSchema()}, "session_id", "created_at", "last_seen_at", "expires_at", "user_agent", "ip", "current")
	return map[string]apiContract{
		"PUT /api/v1/pharmacy/products/{id}":   {Request: objectSchema(drugProps()), Response: versionedSchema("product_id"), Status: 200, Version: true},
		"GET /api/v1/pharmacy/products/{id}":   {Response: objectSchema(map[string]any{"drug": drug, "drug_version": integerSchema(1), "product_id": uuidSchema(false)}, "drug", "drug_version", "product_id"), Status: 200},
		"GET /api/v1/pharmacy/products":        {Response: page("drugs", drugs), Status: 200, Query: map[string]map[string]any{"after": uuidSchema(false)}},
		"POST /api/v1/pharmacy/batch-receipts": {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "variant_id": uuidSchema(false), "batch_number": textSchema(100), "expiry_date": map[string]any{"type": "string", "pattern": "^[0-9]{4}-[0-9]{2}(-[0-9]{2})?$"}, "supplier_id": uuidSchema(false), "invoice_number": textSchema(200), "qty_milli": qty, "unit_cost_cents": money, "status": status}, "branch_id", "variant_id", "batch_number", "expiry_date", "supplier_id", "qty_milli", "unit_cost_cents", "status"), Response: objectSchema(map[string]any{"receipt_id": uuidSchema(false), "batch_id": uuidSchema(false)}, "receipt_id", "batch_id"), Status: 201, Idempotency: true},
		"GET /api/v1/pharmacy/batch-receipts":  {Response: page("receipts", receipt), Status: 200, Query: branchQuery(), RequiredQuery: []string{"branch_id"}},
		"GET /api/v1/pharmacy/batches": {Response: objectSchema(map[string]any{"batches": arraySchema(batch), "truncated": boolSchema(), "next_cursor": textSchema(0), "local_date": date, "expiry_warning_days": integerSchema(0)}, "batches", "truncated", "next_cursor", "local_date", "expiry_warning_days"), Status: 200, Query: func() map[string]map[string]any {
			q := branchQuery()
			q["filter"] = enumSchema("all", "near_expiry", "expired", "quarantine")
			q["days"] = map[string]any{"type": "integer", "minimum": 0, "maximum": 3650}
			return q
		}(), RequiredQuery: []string{"branch_id"}},
		"GET /api/v1/pharmacy/fefo-preview":             {Response: objectSchema(map[string]any{"allocations": arraySchema(allocation), "shortage_milli": money, "preview_only": boolSchema(), "finalization_blocked": boolSchema()}, "allocations", "shortage_milli", "preview_only", "finalization_blocked"), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false), "variant_id": uuidSchema(false), "qty_milli": qty}, RequiredQuery: []string{"branch_id", "variant_id", "qty_milli"}},
		"GET /api/v1/pharmacy/batches/{id}/movements":   {Response: page("movements", movement), Status: 200, Query: branchQuery(), RequiredQuery: []string{"branch_id"}},
		"POST /api/v1/pharmacy/batches/{id}/count":      {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "status": status, "counted_milli": counted, "reason": textSchema(200)}, "branch_id", "status", "counted_milli", "reason"), Response: ok, Status: 200, Idempotency: true},
		"POST /api/v1/pharmacy/batches/{id}/dispose":    {Request: objectSchema(action["properties"].(map[string]any), "branch_id", "qty_milli", "status", "reason"), Response: ok, Status: 200, Idempotency: true},
		"POST /api/v1/pharmacy/batches/{id}/release":    {Request: action, Response: ok, Status: 200, Idempotency: true},
		"GET /api/v1/pharmacy/batches/{id}/trace":       {Response: objectSchema(map[string]any{"receipts": arraySchema(traceReceipt), "sales": arraySchema(traceSale)}, "receipts", "sales"), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false)}, RequiredQuery: []string{"branch_id"}},
		"POST /api/v1/stock/counts":                     {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false)}, "branch_id"), Response: countResponse, Status: 201},
		"GET /api/v1/stock/counts/{id}":                 {Response: objectSchema(map[string]any{"count_id": uuidSchema(false), "branch_id": uuidSchema(false), "status": countStatus, "version": integerSchema(1), "lines": arraySchema(countLine)}, "count_id", "branch_id", "status", "version", "lines"), Status: 200},
		"PUT /api/v1/stock/counts/{id}/lines/{variant}": {Request: objectSchema(map[string]any{"counted_qty_milli": money}, "counted_qty_milli"), Response: versionedSchema("count_id"), Status: 200, Version: true},
		"POST /api/v1/stock/counts/{id}/post":           {Response: countResponse, Status: 200, Version: true, OptionalIdempotency: true},
		"GET /api/v1/stock/counts":                      {Response: page("counts", objectSchema(map[string]any{"count_id": uuidSchema(false), "status": countStatus, "version": integerSchema(1), "created_at": at, "posted_at": nullableSchema(map[string]any{"type": "string", "format": "date-time"})}, "count_id", "status", "version", "created_at", "posted_at")), Status: 200, Query: func() map[string]map[string]any { q := branchQuery(); q["status"] = countStatus; return q }(), RequiredQuery: []string{"branch_id"}},
		"GET /api/v1/me/sessions":                       {Response: page("sessions", session), Status: 200, Account: true, Query: map[string]map[string]any{"after": uuidSchema(false)}},
		"POST /api/v1/me/sessions/revoke-others":        {Response: objectSchema(map[string]any{"revoked": integerSchema(0)}, "revoked"), Status: 200, Account: true},
		"PUT /api/v1/auth/password":                     {Request: objectSchema(map[string]any{"current_password": map[string]any{"type": "string", "writeOnly": true}, "new_password": map[string]any{"type": "string", "minLength": 8, "writeOnly": true}}, "current_password", "new_password"), Response: objectSchema(map[string]any{"status": enumSchema("ok"), "other_sessions_revoked": integerSchema(0)}, "status", "other_sessions_revoked"), Status: 200, Account: true},
	}
}

func TestPharmacyRecoveryContractBatchHasTwentyDistinctOperations(t *testing.T) {
	batch := pharmacyAndRecoveryContracts()
	if len(batch) != 20 {
		t.Fatalf("got %d operations", len(batch))
	}
	for _, old := range []map[string]apiContract{initialContracts(), masterContracts(), restaurantConfigContracts(), catalogContracts(), operationalContracts()} {
		for route := range batch {
			if _, ok := old[route]; ok {
				t.Errorf("duplicate %s", route)
			}
		}
	}
}

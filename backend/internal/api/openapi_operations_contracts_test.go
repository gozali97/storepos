package apiserver

import "testing"

func operationalContracts() map[string]apiContract {
	money := map[string]any{"type": "string", "pattern": "^[0-9]+$"}
	signed := map[string]any{"type": "string", "pattern": "^-?[0-9]+$"}
	date := map[string]any{"type": "string", "format": "date"}
	timestamp := map[string]any{"type": "string", "format": "date-time"}
	approval := nullableSchema(objectSchema(map[string]any{"email": textSchema(0), "pin": map[string]any{"type": "string", "pattern": "^[0-9]{6}$"}}, "email", "pin"))
	ok := objectSchema(map[string]any{"status": enumSchema("ok")}, "status")
	page := func(key string, row map[string]any) map[string]any {
		return objectSchema(map[string]any{key: arraySchema(row), "next_cursor": textSchema(0)}, key, "next_cursor")
	}
	periodQuery := func() map[string]map[string]any {
		return map[string]map[string]any{"branch_id": uuidSchema(false), "from": date, "to": date}
	}
	policy := objectSchema(map[string]any{"return_window_days": integerSchema(0), "cash_rounding_enabled": boolSchema(), "cash_rounding_multiple_cents": money, "cashier_discount_bps": integerSchema(0), "cash_out_approval_cents": money, "variance_approval_cents": money})
	policyOut := objectSchema(map[string]any{"policy": policy, "version": integerSchema(1)}, "policy", "version")
	shift := objectSchema(map[string]any{"shift_id": uuidSchema(false), "opening_cash_cents": money, "opened_at": timestamp, "opened_by_current_user": boolSchema()}, "shift_id", "opening_cash_cents", "opened_at", "opened_by_current_user")
	summaryProps := map[string]any{"shift_id": uuidSchema(false), "branch_id": uuidSchema(false), "sale_count": integerSchema(0), "void_count": integerSchema(0), "payments": arraySchema(objectSchema(map[string]any{"method": textSchema(0), "verification": textSchema(0), "amount_cents": money, "tendered_cents": money, "change_cents": money}, "method", "verification", "amount_cents", "tendered_cents", "change_cents"))}
	required := []string{"shift_id", "branch_id", "sale_count", "void_count", "payments"}
	for _, key := range []string{"cash_refund_cents", "opening_cash_cents", "expected_cash_cents", "counted_cash_cents", "variance_cents", "cash_in_cents", "cash_out_cents", "line_discount_cents", "transaction_discount_cents"} {
		summaryProps[key] = signed
		required = append(required, key)
	}
	receiptLine := objectSchema(map[string]any{"unit_id": uuidSchema(false), "qty_milli": money, "purchase_price_cents": money}, "unit_id", "qty_milli", "purchase_price_cents")
	receiptInput := func(create bool) map[string]any {
		p := map[string]any{"received_date": date, "invoice_number": textSchema(200), "supplier_id": uuidSchema(true), "lines": arraySchema(receiptLine)}
		req := []string{"received_date", "lines"}
		if create {
			p["branch_id"] = uuidSchema(false)
			p["supplier_id"] = uuidSchema(false)
			req = append(req, "branch_id")
		}
		return objectSchema(p, req...)
	}
	receiptResponse := objectSchema(map[string]any{"receipt_id": uuidSchema(false), "status": enumSchema("draft", "posted"), "version": integerSchema(1)}, "receipt_id", "status", "version")
	detailLineProps := map[string]any{"line_id": uuidSchema(false), "unit_id": uuidSchema(false), "variant_id": uuidSchema(false), "qty_milli": money, "purchase_price_cents": money, "unit_name": nullableSchema(textSchema(0)), "factor_milli": nullableSchema(textSchema(0)), "base_qty_milli": nullableSchema(textSchema(0)), "base_cost_cents": nullableSchema(textSchema(0))}
	detail := objectSchema(map[string]any{"receipt_id": uuidSchema(false), "branch_id": uuidSchema(false), "status": enumSchema("draft", "posted"), "version": integerSchema(1), "received_date": date, "invoice_number": textSchema(200), "supplier_id": uuidSchema(true), "posted_at": nullableSchema(map[string]any{"type": "string", "format": "date-time"}), "lines": arraySchema(objectSchema(detailLineProps, "line_id", "unit_id", "variant_id", "qty_milli", "purchase_price_cents", "unit_name", "factor_milli", "base_qty_milli", "base_cost_cents"))}, "receipt_id", "branch_id", "status", "version", "received_date", "invoice_number", "supplier_id", "posted_at", "lines")
	receiptRow := objectSchema(map[string]any{"receipt_id": uuidSchema(false), "received_date": date, "invoice_number": textSchema(200), "supplier_id": uuidSchema(true), "supplier_name": nullableSchema(textSchema(200)), "status": enumSchema("draft", "posted"), "version": integerSchema(1), "created_at": timestamp, "posted_at": nullableSchema(map[string]any{"type": "string", "format": "date-time"})}, "receipt_id", "received_date", "invoice_number", "supplier_id", "supplier_name", "status", "version", "created_at", "posted_at")
	listReceipt := page("receipts", receiptRow)
	listReceipt["properties"].(map[string]any)["date_basis"] = enumSchema("received_date")
	expenseProps := func() map[string]any {
		return map[string]any{"expense_id": uuidSchema(false), "expense_date": date, "category": textSchema(100), "amount_cents": money, "method": enumSchema("shift_cash", "outside_cash"), "note": textSchema(200), "shift_id": uuidSchema(true)}
	}
	expense := objectSchema(expenseProps(), "expense_id", "expense_date", "category", "amount_cents", "method", "note", "shift_id")
	ed := expenseProps()
	ed["branch_id"] = uuidSchema(false)
	ed["approved_by"] = uuidSchema(true)
	reorderProps := map[string]any{"variant_id": uuidSchema(false), "sku": textSchema(100), "product_name": textSchema(200), "base_unit": textSchema(0), "qty_milli": signed, "reserved_milli": money, "available_milli": signed, "minimum_qty_milli": money, "version": integerSchema(1), "last_supplier_id": uuidSchema(true), "last_supplier_name": nullableSchema(textSchema(200))}
	return map[string]apiContract{
		"GET /api/v1/settings/operational-policy": {Response: policyOut, Status: 200},
		"PUT /api/v1/settings/operational-policy": {Request: policy, Response: policyOut, Status: 200, Version: true},
		"POST /api/v1/shifts/open":                {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "register_id": uuidSchema(false), "opening_cash_cents": money}, "branch_id", "register_id", "opening_cash_cents"), Response: objectSchema(map[string]any{"shift_id": uuidSchema(false)}, "shift_id"), Status: 201},
		"GET /api/v1/shifts/open":                 {Response: objectSchema(map[string]any{"shift": nullableSchema(shift)}, "shift"), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false), "register_id": uuidSchema(false)}, RequiredQuery: []string{"branch_id", "register_id"}},
		"POST /api/v1/shifts/cash":                {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "register_id": uuidSchema(false), "kind": enumSchema("in", "out"), "amount_cents": money, "reason": textSchema(200), "approval": approval}, "branch_id", "register_id", "kind", "amount_cents", "reason"), Response: ok, Status: 201},
		"POST /api/v1/shifts/close":               {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "register_id": uuidSchema(false), "counted_cash_cents": money, "note": textSchema(200), "force": boolSchema(), "approval": approval}, "branch_id", "register_id", "counted_cash_cents"), Response: objectSchema(map[string]any{"shift_id": uuidSchema(false), "expected_cash_cents": signed, "counted_cash_cents": money, "variance_cents": signed}, "shift_id", "expected_cash_cents", "counted_cash_cents", "variance_cents"), Status: 200},
		"GET /api/v1/shifts/{id}/summary":         {Response: objectSchema(summaryProps, required...), Status: 200},
		"PUT /api/v1/memberships/me/pin":          {Request: objectSchema(map[string]any{"pin": map[string]any{"type": "string", "pattern": "^[0-9]{6}$"}}, "pin"), Response: ok, Status: 200},
		"POST /api/v1/goods-receipts":             {Request: receiptInput(true), Response: receiptResponse, Status: 201, OptionalIdempotency: true},
		"GET /api/v1/goods-receipts": {Response: listReceipt, Status: 200, Query: func() map[string]map[string]any {
			q := periodQuery()
			q["status"] = enumSchema("draft", "posted")
			q["before"] = uuidSchema(false)
			q["supplier_id"] = uuidSchema(false)
			q["invoice_number"] = textSchema(200)
			q["limit"] = map[string]any{"type": "integer", "minimum": 1, "maximum": 100}
			return q
		}(), RequiredQuery: []string{"branch_id", "from", "to"}},
		"GET /api/v1/goods-receipts/{id}":       {Response: detail, Status: 200},
		"PUT /api/v1/goods-receipts/{id}":       {Request: receiptInput(false), Response: receiptResponse, Status: 200, Version: true},
		"POST /api/v1/goods-receipts/{id}/post": {Response: receiptResponse, Status: 200, Version: true, OptionalIdempotency: true},
		"POST /api/v1/expenses":                 {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "expense_date": date, "category": textSchema(100), "amount_cents": money, "method": enumSchema("shift_cash", "outside_cash"), "note": textSchema(200), "register_id": uuidSchema(false), "approval": approval}, "branch_id", "expense_date", "category", "amount_cents", "method"), Response: objectSchema(map[string]any{"expense_id": uuidSchema(false)}, "expense_id"), Status: 201, Idempotency: true},
		"GET /api/v1/expenses":                  {Response: page("expenses", expense), Status: 200, Query: func() map[string]map[string]any { q := periodQuery(); q["after"] = uuidSchema(false); return q }(), RequiredQuery: []string{"branch_id", "from", "to"}},
		"GET /api/v1/expenses/{id}":             {Response: objectSchema(ed, "expense_id", "branch_id", "expense_date", "category", "amount_cents", "method", "note", "shift_id", "approved_by"), Status: 200},
		"PUT /api/v1/stock/minimums/{variant}":  {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "minimum_qty_milli": money}, "branch_id", "minimum_qty_milli"), Response: objectSchema(map[string]any{"variant_id": uuidSchema(false), "branch_id": uuidSchema(false), "minimum_qty_milli": money, "version": integerSchema(1)}, "variant_id", "branch_id", "minimum_qty_milli", "version"), Status: 200, Version: true, ZeroVersion: true},
		"GET /api/v1/stock/reorder": {Response: func() map[string]any {
			out := page("items", objectSchema(reorderProps, "variant_id", "sku", "product_name", "base_unit", "qty_milli", "reserved_milli", "available_milli", "minimum_qty_milli", "version", "last_supplier_id", "last_supplier_name"))
			out["properties"].(map[string]any)["balance_basis"] = enumSchema("sellable_nonexpired_batches")
			return out
		}(), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false), "after": uuidSchema(false)}, RequiredQuery: []string{"branch_id"}},
		"POST /api/v1/stock/receipts":                  {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "lines": arraySchema(objectSchema(map[string]any{"variant_id": uuidSchema(false), "qty_milli": money}, "variant_id", "qty_milli"))}, "branch_id", "lines"), Response: objectSchema(map[string]any{"receipt_id": uuidSchema(false)}, "receipt_id"), Status: 201},
		"PUT /api/v1/stock/variants/{id}/opening-cost": {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "average_cost_cents": money, "reason": textSchema(200)}, "branch_id", "average_cost_cents", "reason"), Response: objectSchema(map[string]any{"variant_id": uuidSchema(false), "branch_id": uuidSchema(false), "average_cost_cents": money}, "variant_id", "branch_id", "average_cost_cents"), Status: 200, Idempotency: true},
	}
}

func TestOperationalContractBatchHasTwentyDistinctOperations(t *testing.T) {
	batch := operationalContracts()
	if len(batch) != 20 {
		t.Fatalf("batch contains %d", len(batch))
	}
	for _, old := range []map[string]apiContract{initialContracts(), masterContracts(), restaurantConfigContracts(), catalogContracts()} {
		for route := range batch {
			if _, ok := old[route]; ok {
				t.Errorf("duplicate %s", route)
			}
		}
	}
}

func TestOperationalMoneyAndIdempotencyContracts(t *testing.T) {
	contracts := operationalContracts()
	expense := contracts["POST /api/v1/expenses"]
	if !expense.Idempotency {
		t.Fatal("expense missing required idempotency")
	}
	for _, route := range []string{"POST /api/v1/goods-receipts", "POST /api/v1/goods-receipts/{id}/post"} {
		if !contracts[route].OptionalIdempotency {
			t.Fatalf("legacy key support missing: %s", route)
		}
	}
	req := map[string]any{"branch_id": "01920000-0000-7000-8000-000000000001", "expense_date": "2026-10-14", "category": "Rent", "amount_cents": float64(100), "method": "outside_cash"}
	if err := checkContractSchema(expense.Request, req, "request"); err == nil {
		t.Fatal("numeric money accepted")
	}
	req["amount_cents"] = "100"
	if err := checkContractSchema(expense.Request, req, "request"); err != nil {
		t.Fatal(err)
	}
}

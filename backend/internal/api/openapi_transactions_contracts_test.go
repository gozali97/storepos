package apiserver

// B460: implementation only. Testing explicitly deferred; these are not verified contracts.
func transactionsAndStockContracts() map[string]apiContract {
	str := textSchema(0)
	at := map[string]any{"type": "string", "format": "date-time"}
	date := map[string]any{"type": "string", "format": "date"}
	money := map[string]any{"type": "string", "pattern": "^[0-9]+$"}
	signed := map[string]any{"type": "string", "pattern": "^-?[0-9]+$"}
	positive := map[string]any{"type": "string", "pattern": "^[1-9][0-9]*$"}
	discount := objectSchema(map[string]any{"kind": enumSchema("none", "amount", "percent"), "value": money})
	approval := nullableSchema(objectSchema(map[string]any{"email": textSchema(254), "pin": map[string]any{"type": "string", "pattern": "^[0-9]{6}$", "writeOnly": true}}, "email", "pin"))
	role := enumSchema("owner", "manager", "cashier", "warehouse", "kitchen", "pharmacist")
	page := func(key string, row map[string]any) map[string]any {
		return objectSchema(map[string]any{key: arraySchema(row), "next_cursor": str}, key, "next_cursor")
	}
	period := func() map[string]map[string]any {
		return map[string]map[string]any{"branch_id": uuidSchema(false), "from": date, "to": date, "user_id": uuidSchema(false), "format": enumSchema("json", "csv")}
	}
	user := objectSchema(map[string]any{"id": uuidSchema(false), "email": str, "phone": str}, "id")
	membership := objectSchema(map[string]any{"business_id": uuidSchema(false), "business_name": str, "business_type": enumSchema("retail", "restaurant", "pharmacy"), "role": role}, "business_id", "business_name", "business_type", "role")
	session := objectSchema(map[string]any{"user": user, "memberships": arraySchema(membership)}, "user", "memberships")
	modifier := objectSchema(map[string]any{"option_id": uuidSchema(false), "group_id": uuidSchema(false), "group_name": str, "name": str, "price_delta_cents": money}, "option_id", "group_id", "group_name", "name", "price_delta_cents")
	modifiers := nullableSchema(arraySchema(modifier))
	// Header is a nullable historical JSON object; unavailable snapshots must not be reconstructed from today's masters.
	headerSnapshot := map[string]any{"type": "object", "nullable": true, "additionalProperties": nullableSchema(textSchema(0))}
	saleProps := map[string]any{"sale_id": uuidSchema(false), "branch_id": uuidSchema(false), "receipt_number": str, "status": enumSchema("completed", "voided"), "return_status": enumSchema("none", "partial", "full"), "total_cents": money, "change_cents": money, "cash_rounding_cents": signed, "tax_cents": money, "delivery_fee_cents": str, "completed_at": at, "customer_name": nullableSchema(textSchema(200)), "note": textSchema(200), "header_snapshot": headerSnapshot}
	saleRequired := []string{"sale_id", "branch_id", "receipt_number", "status", "return_status", "total_cents", "change_cents", "cash_rounding_cents", "tax_cents", "delivery_fee_cents", "completed_at", "customer_name", "note", "header_snapshot"}
	saleHeaderSchema := objectSchema(saleProps, saleRequired...)
	lineProps := map[string]any{"sale_line_id": uuidSchema(false), "variant_id": uuidSchema(false), "sku": str, "product_name": str, "variant_name": str, "base_unit": str, "sale_unit": nullableSchema(textSchema(0)), "allow_fractional": boolSchema(), "cost_cents": nullableSchema(textSchema(0)), "note": str, "tax_rule_name": nullableSchema(textSchema(0)), "tax_rate_bps": nullableSchema(integerSchema(0)), "tax_mode": nullableSchema(textSchema(0)), "modifiers": modifiers}
	lineRequired := []string{"sale_line_id", "variant_id", "sku", "product_name", "variant_name", "base_unit", "sale_unit", "allow_fractional", "cost_cents", "note", "tax_rule_name", "tax_rate_bps", "tax_mode", "modifiers"}
	for _, k := range []string{"gross_cents", "line_discount_cents", "alloc_discount_cents", "net_cents", "tax_cents", "service_charge_cents", "refunded_cents", "factor_milli", "qty_milli", "base_qty_milli", "unit_price_cents", "line_total_cents", "returned_qty_milli", "remaining_qty_milli"} {
		lineProps[k] = money
		lineRequired = append(lineRequired, k)
	}
	payment := objectSchema(map[string]any{"method": str, "verification": str, "amount_cents": money, "tendered_cents": money, "change_cents": money, "status": str, "manual_refund_required": boolSchema()}, "method", "verification", "amount_cents", "tendered_cents", "change_cents", "status", "manual_refund_required")
	inputLine := objectSchema(map[string]any{"variant_id": uuidSchema(false), "unit_id": uuidSchema(false), "qty_milli": positive, "note": textSchema(200), "discount": discount, "batch_id": uuidSchema(false), "batch_reason": textSchema(200)}, "variant_id", "qty_milli")
	checkoutResponse := orderFlowContracts()["POST /api/v1/restaurant/orders/{id}/checkout"].Response
	stockProps := map[string]any{"variant_id": uuidSchema(false), "sku": str, "product_name": str, "base_unit": str, "qty_milli": signed, "reserved_milli": money, "available_milli": signed, "minimum_qty_milli": nullableSchema(textSchema(0)), "below_minimum": boolSchema(), "negative": boolSchema(), "average_cost_cents": nullableSchema(textSchema(0)), "estimated_value_cents": nullableSchema(textSchema(0)), "cost_unknown": boolSchema(), "period_movement_milli": signed, "period_movement_count": integerSchema(0)}
	stockRow := objectSchema(stockProps, "variant_id", "sku", "product_name", "base_unit", "qty_milli", "reserved_milli", "available_milli", "minimum_qty_milli", "below_minimum", "negative", "average_cost_cents", "estimated_value_cents", "cost_unknown", "period_movement_milli", "period_movement_count")
	batchProps := map[string]any{"batch_id": uuidSchema(false), "variant_id": uuidSchema(false), "sku": str, "product_name": str, "batch_number": str, "expiry_date": date, "stored_status": str, "status": str, "days_to_expiry": map[string]any{"type": "integer"}, "qty_milli": money, "period_movement_milli": signed, "period_movement_count": integerSchema(0), "last_supplier_name": nullableSchema(textSchema(0)), "average_cost_cents": nullableSchema(textSchema(0)), "estimated_value_cents": nullableSchema(textSchema(0))}
	stockEnvelope := func(row map[string]any, batch bool) map[string]any {
		p := map[string]any{"timezone": str, "balance_basis": enumSchema("current"), "generated_at": at, "cost_visible": boolSchema(), "valuation_label": str, "rows": arraySchema(row)}
		req := []string{"timezone", "balance_basis", "generated_at", "cost_visible", "valuation_label", "rows"}
		if batch {
			p["expiry_warning_days"] = integerSchema(0)
			req = append(req, "expiry_warning_days")
		}
		return objectSchema(p, req...)
	}
	productProps := map[string]any{"product_id": uuidSchema(false), "category_id": uuidSchema(true), "product_name": str, "sold_base_qty_milli": str, "returned_base_qty_milli": str, "net_sales_cents": nullableSchema(textSchema(0)), "sold_cost_cents": nullableSchema(textSchema(0)), "restock_cost_cents": nullableSchema(textSchema(0)), "estimated_gross_profit_cents": nullableSchema(textSchema(0)), "missing_cost": boolSchema(), "incomplete_revenue": boolSchema()}
	productRow := objectSchema(productProps, "product_id", "category_id", "product_name", "sold_base_qty_milli", "returned_base_qty_milli", "net_sales_cents", "sold_cost_cents", "restock_cost_cents", "estimated_gross_profit_cents", "missing_cost", "incomplete_revenue")
	categoryRow := objectSchema(map[string]any{"category_id": uuidSchema(true), "product_count": integerSchema(0), "products_without_cost": integerSchema(0), "sold_cost_cents": nullableSchema(textSchema(0)), "restock_cost_cents": nullableSchema(textSchema(0)), "net_sales_cents": nullableSchema(textSchema(0)), "estimated_gross_profit_cents": nullableSchema(textSchema(0))}, "category_id", "product_count", "net_sales_cents", "estimated_gross_profit_cents", "products_without_cost", "sold_cost_cents", "restock_cost_cents")
	reportProductRows := map[string]any{"oneOf": []any{arraySchema(productRow), arraySchema(categoryRow)}}
	publicLineProps := map[string]any{"product_name": str, "variant_name": str, "unit": str, "qty_milli": money, "unit_price_cents": money, "line_total_cents": money, "tax_rule_name": nullableSchema(textSchema(0)), "tax_rate_bps": nullableSchema(integerSchema(0)), "tax_mode": nullableSchema(textSchema(0)), "discount_cents": money, "tax_cents": money, "service_charge_cents": money, "modifiers": modifiers}
	publicProps := map[string]any{"receipt_number": str, "status": enumSchema("completed", "voided"), "completed_at": at, "customer_name": nullableSchema(textSchema(0)), "header_snapshot": headerSnapshot, "header_snapshot_missing": boolSchema(), "lines": arraySchema(objectSchema(publicLineProps, "product_name", "variant_name", "unit", "qty_milli", "unit_price_cents", "line_total_cents", "tax_rule_name", "tax_rate_bps", "tax_mode", "discount_cents", "tax_cents", "service_charge_cents", "modifiers"))}
	publicReq := []string{"receipt_number", "status", "completed_at", "customer_name", "header_snapshot", "header_snapshot_missing", "lines"}
	for _, k := range []string{"total_cents", "change_cents", "tax_cents", "cash_rounding_cents", "delivery_fee_cents", "subtotal_gross_cents", "discount_cents", "service_charge_cents"} {
		publicProps[k] = signed
		publicReq = append(publicReq, k)
	}
	return map[string]apiContract{
		"POST /api/v1/auth/login":                   {Request: objectSchema(map[string]any{"email": textSchema(254), "password": map[string]any{"type": "string", "writeOnly": true}}, "email", "password"), Response: session, Status: 200, Account: true},
		"GET /api/v1/me":                            {Response: session, Status: 200, Account: true},
		"GET /api/v1/workspace":                     {Response: objectSchema(map[string]any{"branches": arraySchema(objectSchema(map[string]any{"id": uuidSchema(false), "name": str, "timezone": str}, "id", "name", "timezone")), "registers": arraySchema(objectSchema(map[string]any{"id": uuidSchema(false), "name": str, "branch_id": uuidSchema(false)}, "id", "name", "branch_id")), "enabled_modules": arraySchema(str)}, "branches", "registers", "enabled_modules"), Status: 200},
		"GET /api/v1/memberships":                   {Response: page("memberships", objectSchema(map[string]any{"membership_id": uuidSchema(false), "user_id": uuidSchema(false), "role": role, "status": enumSchema("active", "disabled"), "version": integerSchema(1), "branch_ids": arraySchema(uuidSchema(false))}, "membership_id", "user_id", "role", "status", "version", "branch_ids")), Status: 200, Query: map[string]map[string]any{"after": uuidSchema(false)}},
		"PUT /api/v1/memberships/{id}":              {Request: objectSchema(map[string]any{"role": role, "status": enumSchema("active", "disabled"), "branch_ids": arraySchema(uuidSchema(false))}, "role", "status", "branch_ids"), Response: versionedSchema("membership_id"), Status: 200, Version: true},
		"PUT /api/v1/session/business":              {Request: objectSchema(map[string]any{"business_id": uuidSchema(false)}, "business_id"), Response: objectSchema(map[string]any{"business_id": uuidSchema(false)}, "business_id"), Status: 200, Account: true},
		"DELETE /api/v1/customers/{id}":             {Status: 204, Version: true},
		"DELETE /api/v1/suppliers/{id}":             {Status: 204, Version: true},
		"POST /api/v1/catalog/variant-combinations": {Request: objectSchema(map[string]any{"sku_prefix": textSchema(100), "attributes": arraySchema(objectSchema(map[string]any{"name": textSchema(100), "values": arraySchema(textSchema(100))}, "name", "values"))}, "sku_prefix", "attributes"), Response: objectSchema(map[string]any{"variants": arraySchema(objectSchema(map[string]any{"name": str, "sku": str, "attributes": map[string]any{"type": "object", "additionalProperties": str}}, "name", "sku", "attributes"))}, "variants"), Status: 200},
		"GET /api/v1/sales":                         {Response: page("sales", saleHeaderSchema), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false), "from": date, "to": date, "receipt_number": textSchema(100), "customer_id": uuidSchema(false), "before": uuidSchema(false)}, RequiredQuery: []string{"branch_id"}},
		"GET /api/v1/sales/{id}":                    {Response: objectSchema(map[string]any{"sale": saleHeaderSchema, "lines": arraySchema(objectSchema(lineProps, lineRequired...)), "payments": arraySchema(payment)}, "sale", "lines", "payments"), Status: 200},
		"POST /api/v1/sales/finalize":               {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "register_id": uuidSchema(false), "customer_id": uuidSchema(false), "draft_id": uuidSchema(false), "draft_version": integerSchema(1), "note": textSchema(200), "lines": arraySchema(inputLine), "discount": discount, "discount_reason": textSchema(200), "approval": approval, "prescription_reference": textSchema(200), "pharmacist_verification": approval, "payments": arraySchema(objectSchema(map[string]any{"method": enumSchema("cash", "qris", "card", "transfer"), "amount_cents": positive}, "method", "amount_cents"))}, "branch_id", "register_id", "lines", "payments"), Response: checkoutResponse, Status: 201, Idempotency: true},
		"POST /api/v1/sales/{id}/returns":           {Request: objectSchema(map[string]any{"register_id": uuidSchema(false), "reason": textSchema(200), "approval": approval, "lines": arraySchema(objectSchema(map[string]any{"sale_line_id": uuidSchema(false), "qty_milli": positive, "condition": enumSchema("restock", "damaged")}, "sale_line_id", "qty_milli", "condition"))}, "reason", "lines"), Response: objectSchema(map[string]any{"return_id": uuidSchema(false), "amount_cents": money, "manual_refund_required": boolSchema(), "refunds": nullableSchema(arraySchema(objectSchema(map[string]any{"method": str, "amount_cents": money, "manual_refund_required": boolSchema()}, "method", "amount_cents", "manual_refund_required")))}, "return_id", "amount_cents", "manual_refund_required", "refunds"), Status: 201, Idempotency: true},
		"POST /api/v1/sales/{id}/void":              {Request: objectSchema(map[string]any{"reason": textSchema(200), "approval": approval}, "reason", "approval"), Response: objectSchema(map[string]any{"status": enumSchema("voided")}, "status"), Status: 200, Idempotency: true},
		"POST /api/v1/stock/adjustments":            {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "variant_id": uuidSchema(false), "qty_milli": signed, "reason": enumSchema("damaged", "lost", "expired", "own_use", "correction", "other"), "note": textSchema(200), "approval": approval}, "branch_id", "variant_id", "qty_milli", "reason"), Response: objectSchema(map[string]any{"adjustment_id": uuidSchema(false), "balance_after_milli": signed}, "adjustment_id", "balance_after_milli"), Status: 201, Idempotency: true},
		"GET /api/v1/stock/card":                    {Response: page("movements", objectSchema(map[string]any{"id": uuidSchema(false), "kind": str, "qty_milli": signed, "balance_after_milli": signed, "ref_type": str, "ref_id": uuidSchema(false), "created_by": uuidSchema(false), "reason": str, "note": str, "created_at": at}, "id", "kind", "qty_milli", "balance_after_milli", "ref_type", "ref_id", "created_by", "reason", "note", "created_at")), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false), "variant_id": uuidSchema(false), "from": at, "until": at, "before": uuidSchema(false)}, RequiredQuery: []string{"branch_id", "variant_id"}},
		"GET /api/v1/reports/products": {Response: objectSchema(map[string]any{"rows": reportProductRows, "profit_visible": boolSchema(), "valuation_label": str, "products_without_cost": integerSchema(0)}, "rows", "profit_visible", "valuation_label", "products_without_cost"), Status: 200, Query: func() map[string]map[string]any {
			q := period()
			q["group_by"] = enumSchema("product", "category")
			return q
		}(), RequiredQuery: []string{"branch_id", "from", "to"}, CSV: true},
		"GET /api/v1/reports/stock": {Response: stockEnvelope(stockRow, false), Status: 200, Query: func() map[string]map[string]any {
			q := period()
			q["filter"] = enumSchema("all", "below_minimum", "negative")
			return q
		}(), RequiredQuery: []string{"branch_id", "from", "to"}, CSV: true},
		"GET /api/v1/reports/stock/batches": {Response: stockEnvelope(objectSchema(batchProps, "batch_id", "variant_id", "sku", "product_name", "batch_number", "expiry_date", "stored_status", "status", "days_to_expiry", "qty_milli", "period_movement_milli", "period_movement_count", "last_supplier_name", "average_cost_cents", "estimated_value_cents"), true), Status: 200, Query: func() map[string]map[string]any {
			q := period()
			q["filter"] = enumSchema("all", "near_expiry", "expired", "quarantine")
			q["days"] = integerSchema(0)
			return q
		}(), RequiredQuery: []string{"branch_id", "from", "to"}, CSV: true},
		"POST /api/v1/public/receipt": {Request: objectSchema(map[string]any{"token": map[string]any{"type": "string", "writeOnly": true}}, "token"), Response: objectSchema(publicProps, publicReq...), Status: 200, Account: true},
	}
}

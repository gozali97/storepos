package apiserver

// B450: kontrak diimplementasikan tanpa menjalankan testing.
func reportingAndReadContracts() map[string]apiContract {
	str := textSchema(0)
	at := map[string]any{"type": "string", "format": "date-time"}
	date := map[string]any{"type": "string", "format": "date"}
	amount := map[string]any{"type": "string", "pattern": "^-?[0-9]+$"}
	nullableAmount := nullableSchema(map[string]any{"type": "string", "pattern": "^-?[0-9]+$"})
	page := func(key string, row map[string]any) map[string]any {
		return objectSchema(map[string]any{key: arraySchema(row), "next_cursor": str}, key, "next_cursor")
	}
	period := func() map[string]map[string]any {
		return map[string]map[string]any{"branch_id": uuidSchema(false), "from": date, "to": date, "user_id": uuidSchema(false), "format": enumSchema("json", "csv")}
	}
	response := func(row map[string]any) map[string]any {
		return objectSchema(map[string]any{"timezone": str, "rows": arraySchema(row)}, "timezone", "rows")
	}
	saleProps := map[string]any{"date": date, "hour": integerSchema(0), "user_id": uuidSchema(false), "sale_count": integerSchema(0), "average_sale_cents": amount, "incomplete_return_count": integerSchema(0)}
	saleReq := []string{"date", "hour", "user_id", "sale_count", "average_sale_cents", "incomplete_return_count"}
	for _, k := range []string{"gross_cents", "discount_cents"} {
		saleProps[k] = amount
		saleReq = append(saleReq, k)
	}
	for _, k := range []string{"returns_cents", "net_cents", "tax_cents", "service_charge_cents", "delivery_fee_cents", "net_bill_cents", "cash_rounding_cents"} {
		saleProps[k] = nullableAmount
		saleReq = append(saleReq, k)
	}
	payment := objectSchema(map[string]any{"date": date, "hour": integerSchema(0), "user_id": uuidSchema(false), "method": str, "verification": str, "received_cents": amount, "refunded_cents": amount, "net_cents": amount}, "date", "hour", "user_id", "method", "verification", "received_cents", "refunded_cents", "net_cents")
	correction := objectSchema(map[string]any{"kind": enumSchema("return", "void", "discount"), "document_id": uuidSchema(false), "sale_id": uuidSchema(false), "receipt_number": str, "occurred_at": at, "local_date": date, "actor_user_id": uuidSchema(false), "approved_by_user_id": uuidSchema(true), "reason": str, "line_discount_cents": amount, "transaction_discount_cents": amount, "amount_cents": amount}, "kind", "document_id", "sale_id", "receipt_number", "occurred_at", "local_date", "actor_user_id", "approved_by_user_id", "reason", "line_discount_cents", "transaction_discount_cents", "amount_cents")
	exportStatus := enumSchema("pending", "processing", "completed", "failed", "expired")
	exportRow := objectSchema(map[string]any{"export_id": uuidSchema(false), "report_kind": enumSchema("sales", "payments", "corrections", "stock", "products"), "status": exportStatus, "row_count": nullableSchema(integerSchema(0)), "created_at": at, "expires_at": at}, "export_id", "report_kind", "status", "row_count", "created_at", "expires_at")
	branch := objectSchema(map[string]any{"branch_id": uuidSchema(false), "code": str, "name": str, "timezone": str, "status": str, "version": integerSchema(1)}, "branch_id", "code", "name", "timezone", "status", "version")
	register := objectSchema(map[string]any{"register_id": uuidSchema(false), "name": str, "status": str, "version": integerSchema(1)}, "register_id", "name", "status", "version")
	auditProps := map[string]any{"id": uuidSchema(false), "branch_id": uuidSchema(true), "register_id": uuidSchema(true), "actor_user_id": uuidSchema(true), "approved_by_user_id": uuidSchema(true), "action": str, "entity_type": str, "entity_id": nullableSchema(textSchema(0)), "before": map[string]any{"nullable": true}, "after": map[string]any{"nullable": true}, "reason": nullableSchema(textSchema(0)), "ip": nullableSchema(textSchema(0)), "user_agent": nullableSchema(textSchema(0)), "occurred_at": at}
	auditQuery := map[string]map[string]any{"from": at, "until": at, "branch_id": uuidSchema(false), "actor_user_id": uuidSchema(false), "before": uuidSchema(false), "action": textSchema(100), "entity_type": textSchema(100), "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}}
	finding := objectSchema(map[string]any{"business_id": uuidSchema(false), "kind": str, "entity_key": str, "expected_value": amount, "actual_value": amount}, "business_id", "kind", "entity_key", "expected_value", "actual_value")
	orderType := enumSchema("DINE_IN", "TAKEAWAY", "DELIVERY_MANUAL")
	counts := nullableSchema(map[string]any{"type": "object", "additionalProperties": integerSchema(0)})
	statusRow := objectSchema(map[string]any{"order_id": uuidSchema(false), "version": integerSchema(1), "order_type": orderType, "table_id": uuidSchema(true), "item_status_counts": counts, "ready_count": integerSchema(0)}, "order_id", "version", "order_type", "table_id", "item_status_counts", "ready_count")
	aged := objectSchema(map[string]any{"order_id": uuidSchema(false), "order_type": orderType, "table_name": nullableSchema(textSchema(100)), "created_at": at, "version": integerSchema(1), "age_seconds": integerSchema(0)}, "order_id", "order_type", "table_name", "created_at", "version", "age_seconds")
	ticketList := orderFlowContracts()["GET /api/v1/restaurant/kitchen/tickets"].Response
	ticket := ticketList["properties"].(map[string]any)["tickets"].(map[string]any)["items"].(map[string]any)
	ticketProps := map[string]any{"ticket_id": uuidSchema(false), "order_id": uuidSchema(false), "station_name": nullableSchema(textSchema(100)), "station_snapshot_missing": boolSchema(), "order_type": orderType, "table_name": nullableSchema(textSchema(100)), "created_at": at, "server_time": at, "order_status": str, "order_version": integerSchema(1), "label": str, "items": ticket["properties"].(map[string]any)["items"]}
	out := map[string]apiContract{
		"GET /api/v1/reports/sales":                                        {Response: response(objectSchema(saleProps, saleReq...)), Status: 200, Query: period(), RequiredQuery: []string{"branch_id", "from", "to"}, CSV: true},
		"GET /api/v1/reports/payments":                                     {Response: response(payment), Status: 200, Query: period(), RequiredQuery: []string{"branch_id", "from", "to"}, CSV: true},
		"GET /api/v1/reports/corrections":                                  {Response: response(correction), Status: 200, Query: period(), RequiredQuery: []string{"branch_id", "from", "to"}, CSV: true},
		"GET /api/v1/report-exports/{id}":                                  {Response: objectSchema(map[string]any{"export_id": uuidSchema(false), "status": exportStatus, "row_count": nullableSchema(integerSchema(0)), "error_code": nullableSchema(textSchema(0)), "expires_at": at}, "export_id", "status", "row_count", "error_code", "expires_at"), Status: 200, Query: map[string]map[string]any{"download": boolSchema()}, CSV: true},
		"GET /api/v1/report-exports":                                       {Response: page("exports", exportRow), Status: 200, Query: map[string]map[string]any{"after": uuidSchema(false)}},
		"GET /api/v1/branches":                                             {Response: page("branches", branch), Status: 200, Query: map[string]map[string]any{"after": uuidSchema(false)}},
		"GET /api/v1/branches/{id}/registers":                              {Response: page("registers", register), Status: 200, Query: map[string]map[string]any{"after": uuidSchema(false), "status": enumSchema("active", "inactive", "archived", "all")}},
		"GET /api/v1/audit-logs":                                           {Response: page("events", objectSchema(auditProps, "id", "branch_id", "register_id", "actor_user_id", "approved_by_user_id", "action", "entity_type", "entity_id", "before", "after", "reason", "ip", "user_agent", "occurred_at")), Status: 200, Query: auditQuery, RequiredQuery: []string{"from", "until"}},
		"GET /api/v1/reconciliation/findings":                              {Response: objectSchema(map[string]any{"date": date, "findings": arraySchema(finding), "truncated": boolSchema(), "run_started_at": nullableSchema(map[string]any{"type": "string", "format": "date-time"}), "run_completed_at": nullableSchema(map[string]any{"type": "string", "format": "date-time"}), "tenant_finding_count": integerSchema(0), "date_basis": enumSchema("UTC")}, "date", "findings", "truncated", "run_started_at", "run_completed_at", "tenant_finding_count", "date_basis"), Status: 200, Query: map[string]map[string]any{"date": date}, RequiredQuery: []string{"date"}},
		"GET /api/v1/restaurant/order-status":                              {Response: objectSchema(map[string]any{"orders": arraySchema(statusRow), "truncated": boolSchema(), "server_time": at}, "orders", "truncated", "server_time"), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false)}, RequiredQuery: []string{"branch_id"}},
		"GET /api/v1/restaurant/reports/aged-orders":                       {Response: objectSchema(map[string]any{"orders": arraySchema(aged), "threshold_hours": integerSchema(0), "next_cursor": str}, "orders", "threshold_hours", "next_cursor"), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false), "hours": integerSchema(1), "after": uuidSchema(false)}, RequiredQuery: []string{"branch_id"}},
		"GET /api/v1/restaurant/kitchen/tickets/{id}":                      {Response: objectSchema(ticketProps, "ticket_id", "order_id", "station_name", "station_snapshot_missing", "order_type", "table_name", "created_at", "server_time", "order_status", "order_version", "label", "items"), Status: 200},
		"PATCH /api/v1/restaurant/orders/{id}/lines/{line}/kitchen-status": {Request: objectSchema(map[string]any{"status": enumSchema("SENT", "IN_PROGRESS", "READY", "SERVED"), "reason": textSchema(200)}, "status"), Response: objectSchema(map[string]any{"line_id": uuidSchema(false), "status": str, "order_version": integerSchema(1)}, "line_id", "status", "order_version"), Status: 200, Version: true},
		"DELETE /api/v1/restaurant/category-routes/{id}":                   {Status: 204, Version: true, Query: map[string]map[string]any{"branch_id": uuidSchema(false)}, RequiredQuery: []string{"branch_id"}},
		"DELETE /api/v1/restaurant/orders/{id}/contact":                    {Response: versionedSchema("order_id"), Status: 200, Version: true},
	}
	for _, kind := range []string{"sales", "payments", "corrections", "stock", "products"} {
		q := period()
		q["filter"] = enumSchema("all", "below_minimum", "negative")
		q["group_by"] = enumSchema("product", "category")
		out["POST /api/v1/reports/"+kind+"/exports"] = apiContract{Response: objectSchema(map[string]any{"export_id": uuidSchema(false), "status": enumSchema("pending")}, "export_id", "status"), Status: 202, Query: q, RequiredQuery: []string{"branch_id", "from", "to"}}
	}
	return out
}

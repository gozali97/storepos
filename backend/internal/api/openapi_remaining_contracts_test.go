package apiserver

// Remaining media-specific operations; implementation only, validation deferred.
func remainingContracts() map[string]apiContract {
	str := textSchema(0)
	issue := objectSchema(map[string]any{"row": integerSchema(1), "field": str, "message": str}, "row", "field", "message")
	csv := map[string]any{"type": "string", "maxLength": 5242880, "description": "Raw UTF-8 CSV, not JSON/multipart; <=5MiB and <=5000 data rows. Header from template; money integer cents and quantities milli-units."}
	cancellation := objectSchema(map[string]any{"line_id": uuidSchema(false), "order_id": uuidSchema(false), "order_type": str, "product_name": str, "variant_name": str, "qty_milli": str, "base_unit": str, "modifiers": nullableSchema(arraySchema(objectSchema(map[string]any{"option_id": uuidSchema(false), "group_id": uuidSchema(false), "group_name": str, "name": str, "price_delta_cents": str}, "option_id", "group_id", "group_name", "name", "price_delta_cents"))), "cancelled_from_status": str, "potential_waste": boolSchema(), "reason": str, "actor_user_id": uuidSchema(false), "approved_by_user_id": uuidSchema(true), "cancelled_at": map[string]any{"type": "string", "format": "date-time"}, "local_date": map[string]any{"type": "string", "format": "date"}, "menu_value_cents": str}, "line_id", "order_id", "order_type", "product_name", "variant_name", "qty_milli", "base_unit", "modifiers", "cancelled_from_status", "potential_waste", "reason", "actor_user_id", "approved_by_user_id", "cancelled_at", "local_date", "menu_value_cents")
	leadProps := map[string]any{}
	for _, key := range []string{"name", "phone", "business_name", "business_type", "city", "email", "branches", "terminals", "current_system", "notes", "contact_time", "consent_version", "privacy_version", "started_at", "website", "source_page", "source_tab"} {
		leadProps[key] = str
	}
	leadProps["consent"] = boolSchema()
	leadProps["marketing_consent"] = boolSchema()
	branchQuery := map[string]map[string]any{"branch_id": uuidSchema(false)}
	return map[string]apiContract{
		"POST /api/v1/catalog/import/preview":          {Request: csv, RequestMedia: "text/csv", Response: objectSchema(map[string]any{"row_count": integerSchema(0), "valid": boolSchema(), "errors": arraySchema(issue)}, "row_count", "valid", "errors"), Status: 200, Query: branchQuery, RequiredQuery: []string{"branch_id"}},
		"POST /api/v1/catalog/import/commit":           {Request: csv, RequestMedia: "text/csv", Response: objectSchema(map[string]any{"imported_rows": integerSchema(0)}, "imported_rows"), Status: 201, Idempotency: true, Query: branchQuery, RequiredQuery: []string{"branch_id"}},
		"GET /api/v1/catalog/import/template":          {Response: map[string]any{"type": "string"}, ResponseMedia: "text/csv", Status: 200},
		"GET /api/v1/restaurant/events":                {Response: map[string]any{"type": "string", "description": "SSE refresh invalidations; heartbeat/retry, stream reconnects after ~50s. No customer/payment payload."}, ResponseMedia: "text/event-stream", Status: 200, Query: branchQuery, RequiredQuery: []string{"branch_id"}},
		"GET /api/v1/restaurant/reports/cancellations": {Response: objectSchema(map[string]any{"rows": arraySchema(cancellation), "timezone": str, "value_label": str}, "rows", "timezone", "value_label"), Status: 200, CSV: true, Query: map[string]map[string]any{"branch_id": uuidSchema(false), "from": map[string]any{"type": "string", "format": "date"}, "to": map[string]any{"type": "string", "format": "date"}, "user_id": uuidSchema(false), "format": enumSchema("json", "csv")}, RequiredQuery: []string{"branch_id", "from", "to"}},
		"POST /api/v1/leads":                           {Request: objectSchema(leadProps), Response: objectSchema(map[string]any{"accepted": boolSchema()}, "accepted"), Status: 202},
	}
}

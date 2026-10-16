package apiserver

// B440: implementasi schema; testing ditunda, tidak diberi label verified.
func orderFlowContracts() map[string]apiContract {
	cents := map[string]any{"type": "string", "pattern": "^[0-9]+$"}
	positive := map[string]any{"type": "string", "pattern": "^[1-9][0-9]*$"}
	discount := objectSchema(map[string]any{"kind": enumSchema("none", "amount", "percent"), "value": cents})
	modifier := objectSchema(map[string]any{"option_id": uuidSchema(false), "group_id": uuidSchema(false), "group_name": textSchema(100), "name": textSchema(100), "price_delta_cents": cents}, "option_id", "group_id", "group_name", "name", "price_delta_cents")
	modifiers := nullableSchema(arraySchema(modifier))
	approval := nullableSchema(objectSchema(map[string]any{"email": textSchema(254), "pin": map[string]any{"type": "string", "pattern": "^[0-9]{6}$", "writeOnly": true}}, "email", "pin"))
	draftLine := objectSchema(map[string]any{"unit_id": uuidSchema(false), "variant_id": uuidSchema(false), "qty_milli": positive, "note": textSchema(200), "discount": discount}, "variant_id", "qty_milli")
	cart := objectSchema(map[string]any{"lines": arraySchema(draftLine), "note": textSchema(200), "discount": discount, "discount_reason": textSchema(200)})
	draft := objectSchema(map[string]any{"id": uuidSchema(false), "branch_id": uuidSchema(false), "register_id": uuidSchema(false), "status": enumSchema("active", "held", "cancelled", "completed"), "version": integerSchema(1), "cart": cart}, "id", "branch_id", "register_id", "status", "version", "cart")
	orderType := enumSchema("DINE_IN", "TAKEAWAY", "DELIVERY_MANUAL")
	headerProps := func() map[string]any {
		return map[string]any{"order_id": uuidSchema(false), "branch_id": uuidSchema(false), "table_id": uuidSchema(true), "order_type": orderType, "status": enumSchema("open", "completed", "cancelled"), "payment_status": enumSchema("unpaid", "paid"), "version": integerSchema(1), "note": textSchema(200), "channel": textSchema(100), "delivery_fee_cents": cents, "contact_name": textSchema(200), "contact_phone": textSchema(32), "queue_label": textSchema(100), "created_at": map[string]any{"type": "string", "format": "date-time"}}
	}
	header := objectSchema(headerProps(), "order_id", "branch_id", "table_id", "order_type", "status", "payment_status", "version", "note", "channel", "delivery_fee_cents", "contact_name", "contact_phone", "queue_label", "created_at")
	orderVersion := versionedSchema("order_id")
	kitchenStatus := enumSchema("NOT_SENT", "SENT", "IN_PROGRESS", "READY", "SERVED", "CANCELLED")
	orderLine := objectSchema(map[string]any{"line_id": uuidSchema(false), "variant_id": uuidSchema(false), "qty_milli": positive, "unit_price_cents": cents, "product_name": textSchema(200), "variant_name": textSchema(200), "base_unit": textSchema(100), "note": textSchema(140), "kitchen_status": kitchenStatus, "modifiers": modifiers, "modifier_delta_cents": cents, "effective_unit_price_cents": cents}, "line_id", "variant_id", "qty_milli", "unit_price_cents", "product_name", "variant_name", "base_unit", "note", "kitchen_status", "modifiers", "modifier_delta_cents", "effective_unit_price_cents")
	billLine := objectSchema(map[string]any{"line_id": uuidSchema(false), "product_name": textSchema(200), "variant_name": textSchema(200), "unit": textSchema(100), "note": textSchema(140), "modifiers": modifiers, "qty_milli": positive, "effective_unit_price_cents": cents, "line_total_cents": cents, "tax_cents": cents, "service_charge_cents": cents}, "line_id", "product_name", "variant_name", "unit", "note", "modifiers", "qty_milli", "effective_unit_price_cents", "line_total_cents", "tax_cents", "service_charge_cents")
	billProps := map[string]any{"order_id": uuidSchema(false), "version": integerSchema(1), "label": enumSchema("BELUM LUNAS"), "not_sent_count": integerSchema(0), "payment_blocked": boolSchema(), "lines": arraySchema(billLine)}
	billRequired := []string{"order_id", "version", "label", "not_sent_count", "payment_blocked", "lines"}
	for _, key := range []string{"subtotal_gross_cents", "tax_cents", "service_charge_cents", "delivery_fee_cents", "total_cents", "line_discount_cents", "transaction_discount_cents"} {
		billProps[key] = cents
		billRequired = append(billRequired, key)
	}
	discountRequest := objectSchema(map[string]any{"discount": discount, "reason": textSchema(200)}, "discount")
	payment := objectSchema(map[string]any{"method": enumSchema("cash", "qris", "card", "transfer"), "amount_cents": positive}, "method", "amount_cents")
	saleProps := map[string]any{"sale_id": uuidSchema(false), "receipt_number": textSchema(100)}
	saleRequired := []string{"sale_id", "receipt_number"}
	for _, key := range []string{"total_cents", "change_cents", "cash_rounding_cents", "tax_cents", "delivery_fee_cents"} {
		saleProps[key] = map[string]any{"type": "string", "pattern": "^-?[0-9]+$"}
		saleRequired = append(saleRequired, key)
	}
	snapshot := objectSchema(map[string]any{"product_name": textSchema(200), "variant_name": textSchema(200), "qty_milli": positive, "base_unit": textSchema(100), "note": textSchema(140), "modifiers": modifiers}, "product_name", "variant_name", "qty_milli", "base_unit", "note", "modifiers")
	ticketItem := objectSchema(map[string]any{"line_id": uuidSchema(false), "snapshot": snapshot, "status": kitchenStatus, "cancel_reason": nullableSchema(textSchema(200)), "cancelled_at": nullableSchema(map[string]any{"type": "string", "format": "date-time"})}, "line_id", "snapshot", "status", "cancel_reason", "cancelled_at")
	ticket := objectSchema(map[string]any{"ticket_id": uuidSchema(false), "order_id": uuidSchema(false), "order_type": orderType, "table_name": nullableSchema(textSchema(100)), "created_at": map[string]any{"type": "string", "format": "date-time"}, "order_version": integerSchema(1), "items": arraySchema(ticketItem)}, "ticket_id", "order_id", "order_type", "table_name", "created_at", "order_version", "items")
	return map[string]apiContract{
		"POST /api/v1/drafts":                                      {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "register_id": uuidSchema(false), "cart": cart}, "branch_id", "register_id", "cart"), Response: draft, Status: 201},
		"GET /api/v1/drafts":                                       {Response: objectSchema(map[string]any{"drafts": arraySchema(draft)}, "drafts"), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false), "register_id": uuidSchema(false)}, RequiredQuery: []string{"branch_id", "register_id"}},
		"PUT /api/v1/drafts/{id}":                                  {Request: objectSchema(map[string]any{"status": enumSchema("active", "held", "cancelled"), "cart": cart}, "status", "cart"), Response: draft, Status: 200, Version: true},
		"POST /api/v1/restaurant/orders":                           {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "table_id": uuidSchema(true), "order_type": orderType, "note": textSchema(200), "channel": textSchema(100), "delivery_fee_cents": cents, "contact_name": textSchema(200), "contact_phone": textSchema(32), "queue_label": textSchema(100)}, "branch_id", "order_type"), Response: objectSchema(map[string]any{"order_id": uuidSchema(false), "version": integerSchema(1), "resumed": boolSchema(), "detail_path": textSchema(0)}, "order_id", "version", "resumed", "detail_path"), Status: 201, AlternateSuccess: []int{200}, OptionalIdempotency: true},
		"GET /api/v1/restaurant/orders/{id}":                       {Response: header, Status: 200},
		"GET /api/v1/restaurant/orders":                            {Response: objectSchema(map[string]any{"orders": arraySchema(header), "next_cursor": textSchema(0)}, "orders", "next_cursor"), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false), "after": uuidSchema(false), "status": enumSchema("open", "completed", "cancelled", "all"), "order_type": orderType, "from": map[string]any{"type": "string", "format": "date"}, "to": map[string]any{"type": "string", "format": "date"}}, RequiredQuery: []string{"branch_id"}},
		"PUT /api/v1/restaurant/orders/{id}":                       {Request: objectSchema(map[string]any{"note": textSchema(200), "channel": textSchema(100), "delivery_fee_cents": cents, "contact_name": textSchema(200), "contact_phone": textSchema(32), "queue_label": textSchema(100)}), Response: orderVersion, Status: 200, Version: true},
		"POST /api/v1/restaurant/orders/{id}/lines":                {Request: objectSchema(map[string]any{"variant_id": uuidSchema(false), "qty_milli": positive, "note": textSchema(140), "modifier_option_ids": arraySchema(uuidSchema(false))}, "variant_id", "qty_milli"), Response: versionedSchema("line_id"), Status: 201, Version: true},
		"GET /api/v1/restaurant/orders/{id}/lines":                 {Response: objectSchema(map[string]any{"order_id": uuidSchema(false), "version": integerSchema(1), "lines": arraySchema(orderLine)}, "order_id", "version", "lines"), Status: 200},
		"PUT /api/v1/restaurant/orders/{id}/lines/{line}":          {Request: objectSchema(map[string]any{"qty_milli": positive, "note": textSchema(140)}, "qty_milli"), Response: orderVersion, Status: 200, Version: true},
		"DELETE /api/v1/restaurant/orders/{id}/lines/{line}":       {Response: orderVersion, Status: 200, Version: true},
		"PUT /api/v1/restaurant/orders/{id}/discount":              {Request: discountRequest, Response: orderVersion, Status: 200, Version: true},
		"PUT /api/v1/restaurant/orders/{id}/lines/{line}/discount": {Request: discountRequest, Response: orderVersion, Status: 200, Version: true},
		"GET /api/v1/restaurant/orders/{id}/discounts":             {Response: objectSchema(map[string]any{"discount": discount, "reason": textSchema(200), "version": integerSchema(1), "lines": arraySchema(objectSchema(map[string]any{"line_id": uuidSchema(false), "kind": enumSchema("none", "amount", "percent"), "value": cents}, "line_id", "kind", "value")), "approval_checked_at_checkout": boolSchema()}, "discount", "reason", "version", "lines", "approval_checked_at_checkout"), Status: 200},
		"POST /api/v1/restaurant/orders/{id}/cancel":               {Request: objectSchema(map[string]any{"reason": textSchema(200)}, "reason"), Response: objectSchema(map[string]any{"order_id": uuidSchema(false), "status": enumSchema("cancelled"), "version": integerSchema(1)}, "order_id", "status", "version"), Status: 200, Version: true},
		"POST /api/v1/restaurant/orders/{id}/lines/{line}/cancel":  {Request: objectSchema(map[string]any{"reason": textSchema(200), "approval": approval}, "reason"), Response: objectSchema(map[string]any{"line_id": uuidSchema(false), "status": enumSchema("CANCELLED"), "order_version": integerSchema(1)}, "line_id", "status", "order_version"), Status: 200, Version: true},
		"POST /api/v1/restaurant/orders/{id}/send":                 {Response: objectSchema(map[string]any{"order_id": uuidSchema(false), "version": integerSchema(1), "ticket_ids": arraySchema(uuidSchema(false))}, "order_id", "version", "ticket_ids"), Status: 201, Version: true, Idempotency: true},
		"GET /api/v1/restaurant/orders/{id}/bill":                  {Response: objectSchema(billProps, billRequired...), Status: 200},
		"POST /api/v1/restaurant/orders/{id}/checkout":             {Request: objectSchema(map[string]any{"register_id": uuidSchema(false), "payments": arraySchema(payment), "approval": approval}, "register_id", "payments"), Response: objectSchema(saleProps, saleRequired...), Status: 201, Version: true, Idempotency: true},
		"GET /api/v1/restaurant/kitchen/tickets":                   {Response: objectSchema(map[string]any{"tickets": arraySchema(ticket), "next_cursor": textSchema(0), "server_time": map[string]any{"type": "string", "format": "date-time"}}, "tickets", "next_cursor", "server_time"), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false), "station_id": uuidSchema(false), "after": uuidSchema(false)}, RequiredQuery: []string{"branch_id", "station_id"}},
	}
}

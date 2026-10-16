package apiserver

// Implementasi kontrak B430; testing ditunda atas permintaan pengguna.
func taxAndAccessContracts() map[string]apiContract {
	at := map[string]any{"type": "string", "format": "date-time"}
	tax := objectSchema(map[string]any{"id": uuidSchema(false), "name": textSchema(100), "rate_bps": map[string]any{"type": "integer", "minimum": 0, "maximum": 10000}, "mode": enumSchema("inclusive", "exclusive"), "archived": boolSchema(), "version": integerSchema(1)}, "id", "name", "rate_bps", "mode", "archived", "version")
	taxInput := objectSchema(map[string]any{"name": textSchema(100), "rate_bps": map[string]any{"type": "integer", "minimum": 0, "maximum": 10000}, "mode": enumSchema("inclusive", "exclusive")}, "name", "rate_bps", "mode")
	short := versionedSchema("id")
	assigned := objectSchema(map[string]any{"id": uuidSchema(false), "version": integerSchema(1), "tax_rule_id": uuidSchema(true)}, "id", "version", "tax_rule_id")
	invitation := objectSchema(map[string]any{"invitation_id": uuidSchema(false), "contact": textSchema(0), "role": enumSchema("owner", "manager", "cashier", "warehouse", "kitchen", "pharmacist"), "branch_ids": arraySchema(uuidSchema(false)), "expires_at": at, "accepted_at": nullableSchema(map[string]any{"type": "string", "format": "date-time"}), "revoked_at": nullableSchema(map[string]any{"type": "string", "format": "date-time"})}, "invitation_id", "contact", "role", "branch_ids", "expires_at", "accepted_at", "revoked_at")
	ret := objectSchema(map[string]any{"return_id": uuidSchema(false), "amount_cents": textSchema(0), "cash_rounding_cents": nullableSchema(textSchema(0)), "reason": textSchema(0), "created_by": uuidSchema(false), "approved_by": uuidSchema(true), "created_at": at}, "return_id", "amount_cents", "cash_rounding_cents", "reason", "created_by", "approved_by", "created_at")
	return map[string]apiContract{
		"POST /api/v1/tax-rules":                  {Request: taxInput, Response: short, Status: 201},
		"PUT /api/v1/tax-rules/{id}":              {Request: taxInput, Response: short, Status: 200, Version: true},
		"GET /api/v1/tax-rules/{id}":              {Response: tax, Status: 200},
		"GET /api/v1/tax-rules":                   {Response: objectSchema(map[string]any{"rules": arraySchema(tax), "next_cursor": textSchema(0)}, "rules", "next_cursor"), Status: 200, Query: map[string]map[string]any{"after": uuidSchema(false)}},
		"DELETE /api/v1/tax-rules/{id}":           {Response: short, Status: 200, Version: true},
		"POST /api/v1/tax-rules/{id}/restore":     {Response: objectSchema(map[string]any{"id": uuidSchema(false), "version": integerSchema(1), "archived": boolSchema()}, "id", "version", "archived"), Status: 200, Version: true},
		"PUT /api/v1/products/{id}/tax-rule":      {Request: objectSchema(map[string]any{"tax_rule_id": uuidSchema(true)}), Response: assigned, Status: 200, Version: true},
		"GET /api/v1/products/{id}/tax-rule":      {Response: assigned, Status: 200},
		"PUT /api/v1/categories/{id}/tax-rule":    {Request: objectSchema(map[string]any{"tax_rule_id": uuidSchema(true)}), Response: assigned, Status: 200, Version: true},
		"GET /api/v1/categories/{id}/tax-rule":    {Response: assigned, Status: 200},
		"POST /api/v1/invitations":                {Request: objectSchema(map[string]any{"email": textSchema(0), "role": enumSchema("owner", "manager", "cashier", "warehouse", "kitchen", "pharmacist"), "branch_ids": arraySchema(uuidSchema(false))}, "email", "role"), Response: objectSchema(map[string]any{"invitation_id": uuidSchema(false), "token": textSchema(0), "expires_at": at, "delivery": enumSchema("manual")}, "invitation_id", "token", "expires_at", "delivery"), Status: 201},
		"GET /api/v1/invitations":                 {Response: objectSchema(map[string]any{"invitations": arraySchema(invitation), "next_cursor": textSchema(0)}, "invitations", "next_cursor"), Status: 200, Query: map[string]map[string]any{"after": uuidSchema(false)}},
		"DELETE /api/v1/invitations/{id}":         {Status: 204},
		"POST /api/v1/invitations/accept":         {Request: objectSchema(map[string]any{"token": map[string]any{"type": "string", "writeOnly": true}}, "token"), Response: objectSchema(map[string]any{"membership_id": uuidSchema(false), "business_id": uuidSchema(false)}, "membership_id", "business_id"), Status: 200, Account: true},
		"POST /api/v1/sales/{id}/receipt-links":   {Response: objectSchema(map[string]any{"token": textSchema(0), "receipt_path": textSchema(0)}, "token", "receipt_path"), Status: 201},
		"DELETE /api/v1/sales/{id}/receipt-links": {Status: 204},
		"POST /api/v1/sales/{id}/reprints":        {Response: objectSchema(map[string]any{"label": enumSchema("SALINAN"), "sale_id": uuidSchema(false), "detail_path": textSchema(0)}, "label", "sale_id", "detail_path"), Status: 201},
		"GET /api/v1/sales/{id}/returns":          {Response: objectSchema(map[string]any{"returns": arraySchema(ret), "next_cursor": textSchema(0)}, "returns", "next_cursor"), Status: 200, Query: map[string]map[string]any{"after": uuidSchema(false)}},
		"GET /api/v1/sales/{id}/void":             {Response: objectSchema(map[string]any{"sale_id": uuidSchema(false), "status": enumSchema("completed", "voided"), "voided_by": uuidSchema(true), "approved_by": uuidSchema(true), "reason": nullableSchema(textSchema(0)), "voided_at": nullableSchema(map[string]any{"type": "string", "format": "date-time"})}, "sale_id", "status", "voided_by", "approved_by", "reason", "voided_at"), Status: 200},
		"POST /api/v1/auth/logout":                {Response: objectSchema(map[string]any{"status": enumSchema("ok")}, "status"), Status: 200, Account: true},
	}
}

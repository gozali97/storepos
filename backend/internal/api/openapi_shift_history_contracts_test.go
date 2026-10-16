package apiserver

// Kontrak B500: implementasi saja, tidak diuji.
func shiftHistoryContracts() map[string]apiContract {
	at := map[string]any{"type": "string", "format": "date-time"}
	date := map[string]any{"type": "string", "format": "date"}
	nullableMoney := nullableSchema(map[string]any{"type": "string", "pattern": "^-?[0-9]+$"})
	row := objectSchema(map[string]any{"shift_id": uuidSchema(false), "register_id": uuidSchema(false), "register_name": textSchema(200), "opened_by": uuidSchema(false), "cashier_name": textSchema(200), "opened_at": at, "closed_at": nullableSchema(map[string]any{"type": "string", "format": "date-time"}), "status": enumSchema("open", "closed"), "expected_cash_cents": nullableMoney, "counted_cash_cents": nullableMoney, "variance_cents": nullableMoney}, "shift_id", "register_id", "register_name", "opened_by", "cashier_name", "opened_at", "closed_at", "status", "expected_cash_cents", "counted_cash_cents", "variance_cents")
	note := objectSchema(map[string]any{"note_id": uuidSchema(false), "body": textSchema(1000), "created_by": uuidSchema(false), "author_name": textSchema(200), "created_at": at}, "note_id", "body", "created_by", "author_name", "created_at")
	return map[string]apiContract{
		"GET /api/v1/shifts":             {Response: objectSchema(map[string]any{"shifts": arraySchema(row), "next_cursor": textSchema(0), "timezone": textSchema(0), "scope": enumSchema("own", "branch")}, "shifts", "next_cursor", "timezone", "scope"), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false), "register_id": uuidSchema(false), "from": date, "to": date, "status": enumSchema("all", "open", "closed"), "before": uuidSchema(false)}, RequiredQuery: []string{"branch_id", "from", "to"}},
		"GET /api/v1/shifts/{id}/notes":  {Response: objectSchema(map[string]any{"notes": arraySchema(note), "next_cursor": textSchema(0)}, "notes", "next_cursor"), Status: 200, Query: map[string]map[string]any{"after": uuidSchema(false)}},
		"POST /api/v1/shifts/{id}/notes": {Request: objectSchema(map[string]any{"body": map[string]any{"type": "string", "minLength": 1, "maxLength": 1000}}, "body"), Response: objectSchema(map[string]any{"note_id": uuidSchema(false), "status": enumSchema("recorded")}, "note_id", "status"), Status: 201, Idempotency: true},
	}
}

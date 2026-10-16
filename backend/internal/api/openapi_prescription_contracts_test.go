package apiserver

func prescriptionAccessContracts() map[string]apiContract {
	return map[string]apiContract{
		"GET /api/v1/pharmacy/sales/{id}/prescription": {Response: objectSchema(map[string]any{"sale_id": uuidSchema(false), "prescription_reference": nullableSchema(textSchema(200)), "verified_by": uuidSchema(true)}, "sale_id", "prescription_reference", "verified_by"), Status: 200},
	}
}

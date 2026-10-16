package apiserver

// Implementasi belum diuji; konfigurasi manual, bukan payment gateway.
func paymentMethodContracts() map[string]apiContract {
	method := objectSchema(map[string]any{"code": enumSchema("cash", "qris", "card", "transfer"), "name": textSchema(100), "enabled": boolSchema()}, "code", "name", "enabled")
	response := objectSchema(map[string]any{"methods": arraySchema(method), "version": integerSchema(1)}, "methods", "version")
	return map[string]apiContract{
		"GET /api/v1/settings/payment-methods": {Response: response, Status: 200},
		"PUT /api/v1/settings/payment-methods": {Request: objectSchema(map[string]any{"methods": arraySchema(method)}, "methods"), Response: response, Status: 200, Version: true},
	}
}

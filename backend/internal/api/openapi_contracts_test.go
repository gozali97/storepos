package apiserver

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
)

// Kontrak terkurasi untuk operasi yang sudah diuji terhadap respons HTTP nyata.
// Operasi lain tetap diberi x-contract-status: skeleton, bukan dianggap lengkap.
type apiContract struct {
	RequestMedia        string
	ResponseMedia       string
	CSV                 bool
	AlternateSuccess    []int
	Request             map[string]any
	Response            map[string]any
	Status              int
	Version             bool
	Account             bool
	Query               map[string]map[string]any
	RequiredQuery       []string
	ZeroVersion         bool
	OptionalVersion     bool
	Idempotency         bool
	OptionalIdempotency bool
}

func textSchema(max int) map[string]any {
	s := map[string]any{"type": "string"}
	if max > 0 {
		s["maxLength"] = max
	}
	return s
}
func integerSchema(min int) map[string]any { return map[string]any{"type": "integer", "minimum": min} }
func boolSchema() map[string]any           { return map[string]any{"type": "boolean"} }
func enumSchema(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}
func uuidSchema(nullable bool) map[string]any {
	return map[string]any{"type": "string", "format": "uuid", "nullable": nullable}
}
func arraySchema(item map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": item}
}
func objectSchema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
func nullableSchema(s map[string]any) map[string]any { s["nullable"] = true; return s }
func versionedSchema(key string) map[string]any {
	return objectSchema(map[string]any{key: uuidSchema(false), "version": integerSchema(1)}, key, "version")
}

func curatedContracts() map[string]apiContract {
	out := initialContracts()
	for route, contract := range masterContracts() {
		out[route] = contract
	}
	for route, contract := range restaurantConfigContracts() {
		out[route] = contract
	}
	for route, contract := range catalogContracts() {
		out[route] = contract
	}
	for route, contract := range operationalContracts() {
		out[route] = contract
	}
	for route, contract := range pharmacyAndRecoveryContracts() {
		out[route] = contract
	}
	for route, contract := range taxAndAccessContracts() {
		out[route] = contract
	}
	for route, contract := range orderFlowContracts() {
		out[route] = contract
	}
	for route, contract := range reportingAndReadContracts() {
		out[route] = contract
	}
	for route, contract := range transactionsAndStockContracts() {
		out[route] = contract
	}
	for route, contract := range shiftHistoryContracts() {
		out[route] = contract
	}
	for route, contract := range remainingContracts() {
		out[route] = contract
	}
	for route, contract := range paymentMethodContracts() {
		out[route] = contract
	}
	for route, contract := range prescriptionAccessContracts() {
		out[route] = contract
	}
	return out
}

func initialContracts() map[string]apiContract {
	profile := objectSchema(map[string]any{"display_name": textSchema(200), "version": integerSchema(1)}, "display_name", "version")
	nameRequest := objectSchema(map[string]any{"name": textSchema(200)}, "name")
	business := objectSchema(map[string]any{"name": textSchema(200), "version": integerSchema(1)}, "name", "version")
	kind := enumSchema("retail", "restaurant", "pharmacy")
	typeProps := func() map[string]any {
		return map[string]any{"business_type": kind, "enabled_modules": arraySchema(textSchema(0)), "version": integerSchema(1)}
	}
	readType := typeProps()
	readType["locked"] = boolSchema()
	readType["first_sale_completed_at"] = nullableSchema(map[string]any{"type": "string", "format": "date-time"})
	defaultsProps := func() map[string]any {
		return map[string]any{"default_purchase_unit_id": uuidSchema(true), "default_sale_unit_id": uuidSchema(true)}
	}
	defaultOut := defaultsProps()
	defaultOut["variant_id"] = uuidSchema(false)
	defaultOut["version"] = integerSchema(1)
	tax := objectSchema(map[string]any{"name": textSchema(100), "rate_bps": map[string]any{"type": "integer", "minimum": 0, "maximum": 10000}, "mode": enumSchema("inclusive", "exclusive")}, "name", "rate_bps", "mode")
	taxRule := objectSchema(map[string]any{"id": uuidSchema(false), "name": textSchema(100), "rate_bps": integerSchema(0), "mode": enumSchema("inclusive", "exclusive")}, "id", "name", "rate_bps", "mode")
	ok := objectSchema(map[string]any{"status": enumSchema("ok")}, "status")
	areaRequest := objectSchema(map[string]any{"branch_id": uuidSchema(false), "name": textSchema(100)}, "branch_id", "name")
	archive := objectSchema(map[string]any{"id": uuidSchema(false), "archived": boolSchema(), "version": integerSchema(1)}, "id", "archived", "version")
	tableRequest := objectSchema(map[string]any{"branch_id": uuidSchema(false), "area_id": uuidSchema(false), "name": textSchema(100), "capacity": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000}}, "branch_id", "area_id", "name", "capacity")
	return map[string]apiContract{
		"GET /api/v1/me/profile":                     {Response: profile, Status: 200, Account: true},
		"PUT /api/v1/me/profile":                     {Request: objectSchema(map[string]any{"display_name": textSchema(200)}, "display_name"), Response: profile, Status: 200, Version: true, Account: true},
		"GET /api/v1/settings/business-profile":      {Response: business, Status: 200},
		"PUT /api/v1/settings/business-profile":      {Request: nameRequest, Response: business, Status: 200, Version: true},
		"GET /api/v1/settings/business-type":         {Response: objectSchema(readType, "business_type", "enabled_modules", "version", "locked", "first_sale_completed_at"), Status: 200},
		"PUT /api/v1/settings/business-type":         {Request: objectSchema(map[string]any{"business_type": kind}, "business_type"), Response: objectSchema(typeProps(), "business_type", "enabled_modules", "version"), Status: 200, Version: true},
		"GET /api/v1/variants/{id}/unit-defaults":    {Response: objectSchema(defaultOut, "variant_id", "default_purchase_unit_id", "default_sale_unit_id", "version"), Status: 200},
		"PUT /api/v1/variants/{id}/unit-defaults":    {Request: objectSchema(defaultsProps()), Response: objectSchema(defaultOut, "variant_id", "default_purchase_unit_id", "default_sale_unit_id", "version"), Status: 200, Version: true},
		"GET /api/v1/settings/tax":                   {Response: objectSchema(map[string]any{"rule": nullableSchema(taxRule)}, "rule"), Status: 200},
		"PUT /api/v1/settings/tax":                   {Request: tax, Response: ok, Status: 200},
		"GET /api/v1/onboarding":                     {Response: objectSchema(map[string]any{"items": arraySchema(objectSchema(map[string]any{"key": enumSchema("branch_address", "first_product", "invite_team", "open_shift", "first_sale"), "done": boolSchema()}, "key", "done")), "done": integerSchema(0), "total": integerSchema(0), "dismissed": boolSchema(), "demo_present": boolSchema(), "m2_minutes": nullableSchema(integerSchema(0))}, "items", "done", "total", "dismissed", "demo_present", "m2_minutes"), Status: 200},
		"PUT /api/v1/onboarding/dismiss":             {Request: objectSchema(map[string]any{"dismissed": boolSchema()}, "dismissed"), Response: objectSchema(map[string]any{"dismissed": boolSchema()}, "dismissed"), Status: 200},
		"POST /api/v1/onboarding/events":             {Request: objectSchema(map[string]any{"step": enumSchema("business", "type", "branch", "settings", "product"), "status": enumSchema("completed", "skipped")}, "step", "status"), Response: ok, Status: 200},
		"POST /api/v1/onboarding/demo":               {Response: objectSchema(map[string]any{"created": integerSchema(0), "restored": integerSchema(0)}, "created", "restored"), Status: 201},
		"DELETE /api/v1/onboarding/demo":             {Response: objectSchema(map[string]any{"archived": integerSchema(0)}, "archived"), Status: 200},
		"POST /api/v1/restaurant/areas":              {Request: areaRequest, Response: versionedSchema("area_id"), Status: 201},
		"PUT /api/v1/restaurant/areas/{id}":          {Request: objectSchema(map[string]any{"name": textSchema(100)}, "name"), Response: versionedSchema("area_id"), Status: 200, Version: true},
		"DELETE /api/v1/restaurant/areas/{id}":       {Response: archive, Status: 200, Version: true},
		"POST /api/v1/restaurant/areas/{id}/restore": {Response: archive, Status: 200, Version: true},
		"POST /api/v1/restaurant/tables":             {Request: tableRequest, Response: versionedSchema("table_id"), Status: 201},
	}
}

func schemaJSON(s map[string]any) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}
func contractResponseYAML(status int, schema map[string]any) string {
	if status == 204 {
		return "        '204': {description: Berhasil tanpa body}\n"
	}
	return fmt.Sprintf("        '%d':\n          description: Berhasil\n          content:\n            application/json:\n              schema: %s\n", status, schemaJSON(schema))
}

// Validator subset schema yang dipakai kontrak ini saja; tidak mengklaim validator OpenAPI umum.
func checkContractSchema(schema map[string]any, value any, path string) error {
	if value == nil {
		if schema["nullable"] == true {
			return nil
		}
		return fmt.Errorf("%s: unexpected null", path)
	}
	switch schema["type"] {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected object", path)
		}
		props, _ := schema["properties"].(map[string]any)
		required, _ := schema["required"].([]string)
		for _, key := range required {
			if _, ok := obj[key]; !ok {
				return fmt.Errorf("%s.%s missing", path, key)
			}
		}
		for key, v := range obj {
			prop, ok := props[key]
			if !ok {
				if extra, accepts := schema["additionalProperties"].(map[string]any); accepts {
					if err := checkContractSchema(extra, v, path+"."+key); err != nil {
						return err
					}
					continue
				}
				return fmt.Errorf("%s.%s undocumented", path, key)
			}
			if err := checkContractSchema(prop.(map[string]any), v, path+"."+key); err != nil {
				return err
			}
		}
	case "array":
		values, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s: expected array", path)
		}
		for _, v := range values {
			if err := checkContractSchema(schema["items"].(map[string]any), v, path+"[]"); err != nil {
				return err
			}
		}
	case "string":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s: expected string", path)
		}
		if min, ok := schema["minLength"].(int); ok && len([]rune(s)) < min {
			return fmt.Errorf("%s: too short", path)
		}
		if max, ok := schema["maxLength"].(int); ok && len([]rune(s)) > max {
			return fmt.Errorf("%s: too long", path)
		}
		if pattern, ok := schema["pattern"].(string); ok && !regexp.MustCompile(pattern).MatchString(s) {
			return fmt.Errorf("%s: pattern mismatch", path)
		}
		if values, ok := schema["enum"].([]string); ok {
			found := false
			for _, v := range values {
				found = found || s == v
			}
			if !found {
				return fmt.Errorf("%s: invalid enum %q", path, s)
			}
		}
		if schema["format"] == "uuid" && !regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`).MatchString(s) {
			return fmt.Errorf("%s: invalid uuid", path)
		}
	case "integer":
		n, ok := value.(float64)
		if !ok || n != float64(int64(n)) {
			return fmt.Errorf("%s: expected integer", path)
		}
		if min, ok := schema["minimum"].(int); ok && n < float64(min) {
			return fmt.Errorf("%s: below minimum", path)
		}
		if max, ok := schema["maximum"].(int); ok && n > float64(max) {
			return fmt.Errorf("%s: above maximum", path)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: expected bool", path)
		}
	default:
		return fmt.Errorf("%s: unknown type %v", path, schema["type"])
	}
	return nil
}

func TestCuratedContractsReferenceRegisteredRoutes(t *testing.T) {
	routes, _ := registeredRoutes(t)
	have := map[string]bool{}
	for _, r := range routes {
		have[r] = true
	}
	c := curatedContracts()
	if len(c) != 212 {
		t.Fatalf("expected 212 curated contracts, got %d", len(c))
	}
	for route, schema := range c {
		if !have[route] {
			t.Errorf("contract route missing: %s", route)
		}
		if schema.Status < 200 || schema.Status > 299 || (schema.Response == nil && schema.Status != 204) {
			t.Errorf("incomplete contract: %s", route)
		}
	}
}

func TestContractValidatorRejectsSchemaDrift(t *testing.T) {
	c := curatedContracts()["GET /api/v1/me/profile"]
	for _, bad := range []any{map[string]any{"display_name": "A"}, map[string]any{"display_name": "A", "version": "1"}, map[string]any{"display_name": "A", "version": float64(1), "secret": "leak"}} {
		if err := checkContractSchema(c.Response, bad, "response"); err == nil {
			t.Errorf("accepted invalid response: %v", bad)
		}
	}
}

func contractErrorYAML(status int, description string) string {
	return fmt.Sprintf("        '%d':\n          description: %s\n          content:\n            application/json:\n              schema: {$ref: '#/components/schemas/ErrorResponse'}\n", status, description)
}

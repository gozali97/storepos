package apiserver

import "testing"

func catalogContracts() map[string]apiContract {
	cents := map[string]any{"type": "string", "pattern": "^[0-9]+$"}
	milli := map[string]any{"type": "string", "pattern": "^[1-9][0-9]*$"}
	attributes := map[string]any{"type": "object", "additionalProperties": textSchema(0), "nullable": true}
	variantProps := func() map[string]any {
		return map[string]any{"id": uuidSchema(false), "name": textSchema(200), "sku": textSchema(100), "base_unit": textSchema(100), "price_cents": cents, "price_includes_tax": boolSchema(), "allow_fractional": boolSchema(), "attributes": attributes, "version": integerSchema(1), "barcodes": arraySchema(textSchema(200))}
	}
	variant := objectSchema(variantProps(), "id", "sku", "base_unit", "price_cents", "price_includes_tax", "allow_fractional", "attributes", "version")
	vp := variantProps()
	delete(vp, "id")
	delete(vp, "version")
	createVariant := objectSchema(vp, "sku", "price_cents")
	short := versionedSchema("id")
	status := enumSchema("active", "inactive", "archived")
	unitProps := func() map[string]any {
		return map[string]any{"unit_id": uuidSchema(false), "name": textSchema(200), "factor_milli": milli, "sale_price_cents": nullableSchema(map[string]any{"type": "string", "pattern": "^[0-9]+$"}), "factor_locked": boolSchema(), "version": integerSchema(1)}
	}
	unit := objectSchema(unitProps(), "unit_id", "name", "factor_milli", "sale_price_cents", "factor_locked", "version")
	up := unitProps()
	up["variant_id"] = uuidSchema(false)
	up["archived"] = boolSchema()
	unitDetail := objectSchema(up, "unit_id", "variant_id", "name", "factor_milli", "sale_price_cents", "factor_locked", "version", "archived")
	category := objectSchema(map[string]any{"id": uuidSchema(false), "name": textSchema(200), "version": integerSchema(1)}, "id", "name", "version")
	return map[string]apiContract{
		"POST /api/v1/products":                      {Request: objectSchema(map[string]any{"name": textSchema(200), "category_id": uuidSchema(true), "track_stock": boolSchema(), "variants": arraySchema(createVariant)}, "name", "variants"), Response: objectSchema(map[string]any{"id": uuidSchema(false), "name": textSchema(200), "variants": arraySchema(variant)}, "id", "name", "variants"), Status: 201},
		"GET /api/v1/products/{id}":                  {Response: objectSchema(map[string]any{"id": uuidSchema(false), "name": textSchema(200), "status": status, "category_id": uuidSchema(true), "tax_rule_id": uuidSchema(true), "track_stock": boolSchema(), "version": integerSchema(1)}, "id", "name", "status", "category_id", "tax_rule_id", "track_stock", "version"), Status: 200},
		"PUT /api/v1/products/{id}":                  {Request: objectSchema(map[string]any{"name": textSchema(200), "category_id": uuidSchema(true)}, "name"), Response: short, Status: 200, Version: true},
		"PATCH /api/v1/products/{id}/status":         {Request: objectSchema(map[string]any{"status": status}, "status"), Response: objectSchema(map[string]any{"id": uuidSchema(false), "status": status, "version": integerSchema(1)}, "id", "status", "version"), Status: 200, Version: true},
		"GET /api/v1/variants/{id}":                  {Response: objectSchema(map[string]any{"variant": variant, "status": status, "product_id": uuidSchema(false)}, "variant", "status", "product_id"), Status: 200},
		"PATCH /api/v1/variants/{id}/identity":       {Request: objectSchema(map[string]any{"name": textSchema(200), "sku": textSchema(100)}, "sku"), Response: short, Status: 200, Version: true},
		"PATCH /api/v1/variants/{id}/fractional":     {Request: objectSchema(map[string]any{"allow_fractional": boolSchema()}, "allow_fractional"), Response: short, Status: 200, Version: true},
		"PATCH /api/v1/variants/{id}/price":          {Request: objectSchema(map[string]any{"price_cents": cents}, "price_cents"), Response: objectSchema(map[string]any{"id": uuidSchema(false), "price_cents": cents, "version": integerSchema(1)}, "id", "price_cents", "version"), Status: 200, Version: true},
		"PATCH /api/v1/variants/{id}/status":         {Request: objectSchema(map[string]any{"status": enumSchema("active", "inactive")}, "status"), Response: objectSchema(map[string]any{"id": uuidSchema(false), "status": status, "version": integerSchema(1)}, "id", "status", "version"), Status: 200, Version: true},
		"GET /api/v1/variants/{id}/barcodes":         {Response: objectSchema(map[string]any{"barcodes": arraySchema(objectSchema(map[string]any{"barcode": textSchema(200), "unit_id": uuidSchema(true)}, "barcode", "unit_id")), "next_cursor": textSchema(0)}, "barcodes", "next_cursor"), Status: 200, Query: map[string]map[string]any{"after": textSchema(200)}},
		"DELETE /api/v1/variants/{id}/barcodes":      {Request: objectSchema(map[string]any{"barcode": textSchema(200)}, "barcode"), Response: short, Status: 200, Version: true},
		"POST /api/v1/purchase-units":                {Request: objectSchema(map[string]any{"variant_id": uuidSchema(false), "name": textSchema(200), "factor_milli": milli}, "variant_id", "name", "factor_milli"), Response: objectSchema(map[string]any{"unit_id": uuidSchema(false)}, "unit_id"), Status: 201},
		"GET /api/v1/purchase-units/{id}":            {Response: unitDetail, Status: 200},
		"PUT /api/v1/purchase-units/{id}":            {Request: objectSchema(map[string]any{"name": textSchema(200), "factor_milli": milli, "archived": boolSchema()}, "name", "factor_milli"), Response: objectSchema(map[string]any{"unit_id": uuidSchema(false), "version": integerSchema(1), "archived": boolSchema()}, "unit_id", "version", "archived"), Status: 200, Version: true},
		"PUT /api/v1/purchase-units/{id}/sale-price": {Request: objectSchema(map[string]any{"sale_price_cents": cents}, "sale_price_cents"), Response: objectSchema(map[string]any{"status": enumSchema("ok"), "version": integerSchema(1)}, "status", "version"), Status: 200, OptionalVersion: true},
		"POST /api/v1/purchase-units/{id}/barcodes":  {Request: objectSchema(map[string]any{"barcode": textSchema(200)}, "barcode"), Response: objectSchema(map[string]any{"barcode": textSchema(200), "unit_id": uuidSchema(false)}, "barcode", "unit_id"), Status: 201},
		"GET /api/v1/variants/{id}/units":            {Response: objectSchema(map[string]any{"units": arraySchema(unit)}, "units"), Status: 200},
		"GET /api/v1/catalog/barcode":                {Response: objectSchema(map[string]any{"variant_id": uuidSchema(false), "sku": textSchema(100), "unit_id": uuidSchema(true), "unit_name": textSchema(200), "factor_milli": milli, "price_cents": cents, "allow_fractional": boolSchema()}, "variant_id", "sku", "unit_id", "unit_name", "factor_milli", "price_cents", "allow_fractional"), Status: 200, Query: map[string]map[string]any{"barcode": textSchema(200)}, RequiredQuery: []string{"barcode"}},
		"GET /api/v1/catalog/search":                 {Response: objectSchema(map[string]any{"items": nullableSchema(arraySchema(objectSchema(map[string]any{"variant": variant, "product_id": uuidSchema(false), "product_name": textSchema(200), "track_stock": boolSchema()}, "variant", "product_id", "product_name", "track_stock")))}, "items"), Status: 200, Query: map[string]map[string]any{"q": textSchema(0), "limit": map[string]any{"type": "integer"}}},
		"GET /api/v1/categories/archived":            {Response: objectSchema(map[string]any{"categories": arraySchema(category), "next_cursor": textSchema(0)}, "categories", "next_cursor"), Status: 200, Query: map[string]map[string]any{"after": uuidSchema(false)}},
	}
}

func TestCatalogContractBatchHasTwentyDistinctOperations(t *testing.T) {
	batch := catalogContracts()
	if len(batch) != 20 {
		t.Fatalf("got %d operations", len(batch))
	}
	for _, previous := range []map[string]apiContract{initialContracts(), masterContracts(), restaurantConfigContracts()} {
		for route := range batch {
			if _, ok := previous[route]; ok {
				t.Errorf("duplicate %s", route)
			}
		}
	}
}

func TestCatalogContractRejectsMalformedAttributesAndNumericMoney(t *testing.T) {
	schema := catalogContracts()["POST /api/v1/products"].Request
	good := map[string]any{"name": "P", "variants": []any{map[string]any{"sku": "S", "price_cents": "100", "attributes": map[string]any{"color": "blue"}}}}
	if err := checkContractSchema(schema, good, "request"); err != nil {
		t.Fatal(err)
	}
	variant := good["variants"].([]any)[0].(map[string]any)
	variant["attributes"] = map[string]any{"color": float64(12)}
	if err := checkContractSchema(schema, good, "request"); err == nil {
		t.Fatal("numeric attribute accepted")
	}
	variant["attributes"] = nil
	variant["price_cents"] = float64(100)
	if err := checkContractSchema(schema, good, "request"); err == nil {
		t.Fatal("numeric money accepted")
	}
}

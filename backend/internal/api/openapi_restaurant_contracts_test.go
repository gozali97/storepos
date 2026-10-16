package apiserver

import "testing"

func restaurantConfigContracts() map[string]apiContract {
	page := func(key string, row map[string]any) map[string]any {
		return objectSchema(map[string]any{key: arraySchema(row), "next_cursor": textSchema(0)}, key, "next_cursor")
	}
	query := func(archive bool) map[string]map[string]any {
		q := map[string]map[string]any{"branch_id": uuidSchema(false), "after": uuidSchema(false)}
		if archive {
			q["include_archived"] = boolSchema()
		}
		return q
	}
	name := textSchema(100)
	capacity := map[string]any{"type": "integer", "minimum": 1, "maximum": 1000}
	area := objectSchema(map[string]any{"area_id": uuidSchema(false), "name": name, "version": integerSchema(1), "archived": boolSchema()}, "area_id", "name", "version", "archived")
	table := objectSchema(map[string]any{"table_id": uuidSchema(false), "area_id": uuidSchema(false), "area_name": name, "name": name, "capacity": capacity, "version": integerSchema(1), "status": enumSchema("AVAILABLE", "OCCUPIED", "NEEDS_CLEANING"), "archived": boolSchema()}, "table_id", "area_id", "area_name", "name", "capacity", "version", "status", "archived")
	archived := objectSchema(map[string]any{"id": uuidSchema(false), "version": integerSchema(1), "archived": boolSchema()}, "id", "version", "archived")
	station := objectSchema(map[string]any{"station_id": uuidSchema(false), "name": name, "version": integerSchema(1), "archived": boolSchema()}, "station_id", "name", "version", "archived")
	route := objectSchema(map[string]any{"category_id": uuidSchema(false), "category_name": textSchema(200), "station_id": uuidSchema(false), "station_name": name, "version": integerSchema(1)}, "category_id", "category_name", "station_id", "station_name", "version")
	groupRequest := func(create bool) map[string]any {
		props := map[string]any{"name": name, "min_choices": integerSchema(0), "max_choices": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}}
		required := []string{"name", "min_choices", "max_choices"}
		if create {
			props["product_id"] = uuidSchema(false)
			required = append(required, "product_id")
		} else {
			props["archived"] = boolSchema()
		}
		return objectSchema(props, required...)
	}
	cents := map[string]any{"type": "string", "pattern": "^[0-9]+$"}
	option := objectSchema(map[string]any{"option_id": uuidSchema(false), "name": name, "price_delta_cents": cents, "active": boolSchema()}, "option_id", "name", "price_delta_cents", "active")
	group := objectSchema(map[string]any{"group_id": uuidSchema(false), "name": name, "min_choices": integerSchema(0), "max_choices": integerSchema(1), "version": integerSchema(1), "archived": boolSchema(), "options": arraySchema(option)}, "group_id", "name", "min_choices", "max_choices", "version", "archived", "options")
	optionRequest := func(edit bool) map[string]any {
		props := map[string]any{"name": name, "price_delta_cents": cents}
		if edit {
			props["active"] = boolSchema()
		}
		return objectSchema(props, "name", "price_delta_cents")
	}
	optionResponse := objectSchema(map[string]any{"option_id": uuidSchema(false), "group_version": integerSchema(1)}, "option_id", "group_version")
	availability := objectSchema(map[string]any{"variant_id": uuidSchema(false), "product_name": textSchema(200), "variant_name": textSchema(200), "sold_out": boolSchema(), "version": integerSchema(0), "track_stock": boolSchema(), "qty_milli": textSchema(0), "reserved_milli": textSchema(0), "available_milli": textSchema(0)}, "variant_id", "product_name", "variant_name", "sold_out", "version", "track_stock", "qty_milli", "reserved_milli", "available_milli")
	rate := map[string]any{"type": "integer", "minimum": 0, "maximum": 10000}
	orderType := enumSchema("DINE_IN", "TAKEAWAY", "DELIVERY_MANUAL")
	sc := objectSchema(map[string]any{"order_type": orderType, "rate_bps": rate, "include_in_tax_base": boolSchema(), "version": integerSchema(1)}, "order_type", "rate_bps", "include_in_tax_base", "version")
	return map[string]apiContract{
		"GET /api/v1/restaurant/areas":                                 {Response: page("areas", area), Status: 200, Query: query(true), RequiredQuery: []string{"branch_id"}},
		"GET /api/v1/restaurant/tables":                                {Response: page("tables", table), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false), "after": uuidSchema(false), "include_archived": boolSchema(), "area_id": uuidSchema(false)}, RequiredQuery: []string{"branch_id"}},
		"PUT /api/v1/restaurant/tables/{id}":                           {Request: objectSchema(map[string]any{"name": name, "capacity": capacity}, "name", "capacity"), Response: versionedSchema("table_id"), Status: 200, Version: true},
		"DELETE /api/v1/restaurant/tables/{id}":                        {Response: archived, Status: 200, Version: true},
		"POST /api/v1/restaurant/tables/{id}/restore":                  {Response: archived, Status: 200, Version: true},
		"POST /api/v1/restaurant/stations":                             {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "name": name}, "branch_id", "name"), Response: versionedSchema("station_id"), Status: 201},
		"GET /api/v1/restaurant/stations":                              {Response: page("stations", station), Status: 200, Query: query(true), RequiredQuery: []string{"branch_id"}},
		"PUT /api/v1/restaurant/stations/{id}":                         {Request: objectSchema(map[string]any{"name": name, "archived": boolSchema()}, "name"), Response: stationResponseSchema(), Status: 200, Version: true},
		"PUT /api/v1/restaurant/category-routes/{id}":                  {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "station_id": uuidSchema(false)}, "branch_id", "station_id"), Response: objectSchema(map[string]any{"category_id": uuidSchema(false), "station_id": uuidSchema(false), "version": integerSchema(1)}, "category_id", "station_id", "version"), Status: 200, Version: true, ZeroVersion: true},
		"GET /api/v1/restaurant/category-routes":                       {Response: page("routes", route), Status: 200, Query: query(false), RequiredQuery: []string{"branch_id"}},
		"POST /api/v1/restaurant/modifier-groups":                      {Request: groupRequest(true), Response: versionedSchema("group_id"), Status: 201},
		"PUT /api/v1/restaurant/modifier-groups/{id}":                  {Request: groupRequest(false), Response: versionedSchema("group_id"), Status: 200, Version: true},
		"POST /api/v1/restaurant/modifier-groups/{id}/options":         {Request: optionRequest(false), Response: optionResponse, Status: 201, Version: true},
		"PUT /api/v1/restaurant/modifier-groups/{id}/options/{option}": {Request: optionRequest(true), Response: optionResponse, Status: 200, Version: true},
		"GET /api/v1/restaurant/products/{id}/modifiers":               {Response: objectSchema(map[string]any{"groups": arraySchema(group)}, "groups"), Status: 200, Query: map[string]map[string]any{"include_archived": boolSchema()}},
		"PUT /api/v1/restaurant/menu/{id}/availability":                {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "sold_out": boolSchema()}, "branch_id", "sold_out"), Response: objectSchema(map[string]any{"variant_id": uuidSchema(false), "sold_out": boolSchema(), "version": integerSchema(1)}, "variant_id", "sold_out", "version"), Status: 200, Version: true, ZeroVersion: true},
		"GET /api/v1/restaurant/menu/availability":                     {Response: page("items", availability), Status: 200, Query: query(false), RequiredQuery: []string{"branch_id"}},
		"PUT /api/v1/restaurant/service-charge":                        {Request: objectSchema(map[string]any{"branch_id": uuidSchema(false), "order_type": orderType, "rate_bps": rate, "include_in_tax_base": boolSchema()}, "branch_id", "order_type", "rate_bps"), Response: objectSchema(map[string]any{"version": integerSchema(1)}, "version"), Status: 200, Version: true, ZeroVersion: true},
		"GET /api/v1/restaurant/service-charge":                        {Response: objectSchema(map[string]any{"rules": arraySchema(sc)}, "rules"), Status: 200, Query: map[string]map[string]any{"branch_id": uuidSchema(false)}, RequiredQuery: []string{"branch_id"}},
		"POST /api/v1/restaurant/tables/{id}/clean":                    {Response: objectSchema(map[string]any{"table_id": uuidSchema(false), "status": enumSchema("AVAILABLE"), "version": integerSchema(1)}, "table_id", "status", "version"), Status: 200, Version: true},
	}
}
func stationResponseSchema() map[string]any {
	return objectSchema(map[string]any{"station_id": uuidSchema(false), "version": integerSchema(1), "archived": boolSchema()}, "station_id", "version", "archived")
}

func TestRestaurantContractBatchHasTwentyDistinctOperations(t *testing.T) {
	batch := restaurantConfigContracts()
	if len(batch) != 20 {
		t.Fatalf("batch contains %d", len(batch))
	}
	existing := initialContracts()
	for route := range masterContracts() {
		existing[route] = apiContract{}
	}
	for route := range batch {
		if _, ok := existing[route]; ok {
			t.Errorf("duplicate contract %s", route)
		}
	}
}

func TestRestaurantContractStringMoneyAndQueryRequirements(t *testing.T) {
	batch := restaurantConfigContracts()
	money := batch["POST /api/v1/restaurant/modifier-groups/{id}/options"].Request
	for _, bad := range []any{map[string]any{"name": "Option", "price_delta_cents": float64(100)}, map[string]any{"name": "Option", "price_delta_cents": "1.5"}, map[string]any{"name": "Option", "price_delta_cents": "-100"}} {
		if err := checkContractSchema(money, bad, "request"); err == nil {
			t.Errorf("invalid string money accepted: %v", bad)
		}
	}
	for route, c := range batch {
		for _, name := range c.RequiredQuery {
			if _, ok := c.Query[name]; !ok {
				t.Errorf("%s required query missing schema: %s", route, name)
			}
		}
		if c.ZeroVersion && !c.Version {
			t.Errorf("%s zero version without version header", route)
		}
	}
}

package apiserver

import (
	"strings"
	"testing"

	errs "github.com/storepos/storepos/internal/platform/errors"
	"github.com/storepos/storepos/internal/platform/testdb"
)

// CORE-002: bisnis retail tidak boleh membuka endpoint restoran/apotek walau perannya Pemilik.
func TestEveryRestaurantAndPharmacyRouteRejectsWrongBusinessModule(t *testing.T) {
	pool := testdb.New(t)
	biz := seedBusiness(t, pool, "Retail", "owner@module.test")
	base := newServer(t, pool)
	c := login(t, base, "owner@module.test", biz.String())
	routes, _ := registeredRoutes(t)
	checked := 0
	for _, route := range routes {
		_, path, _ := strings.Cut(route, " ")
		moduleRoute := strings.HasPrefix(path, "/api/v1/restaurant/") || strings.HasPrefix(path, "/api/v1/pharmacy/") || path == "/api/v1/reports/stock/batches"
		if !moduleRoute {
			continue
		}
		method, _, _ := strings.Cut(route, " ")
		path = sampleIDParam.ReplaceAllString(path, sampleID)
		var body any
		if method != "GET" {
			body = map[string]any{}
		}
		code, out := c.do(method, path, body, nil)
		checked++
		if code != 403 {
			t.Errorf("%s bisnis retail menghasilkan %d, seharusnya 403", route, code)
			continue
		}
		errBody, _ := out["error"].(map[string]any)
		if errBody["code"] != errs.CodeModuleDisabled {
			t.Errorf("%s kode %v, seharusnya MODULE_DISABLED", route, errBody["code"])
		}
	}
	if checked < 60 {
		t.Fatalf("hanya %d rute modul diperiksa", checked)
	}
	t.Logf("%d rute restoran/apotek diperiksa", checked)
}

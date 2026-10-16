package apiserver

import (
	"sort"
	"testing"
)

// Setiap DELETE wajib ditinjau: hard-delete hanya untuk data belum dipakai/artefak sementara;
// master historis memakai arsip/status/anonymize/revoke. Rute baru membuat tes ini gagal (CORE-043).
var reviewedDeleteRoutes = map[string]string{
	"DELETE /api/v1/categories/{id}":                     "archive",
	"DELETE /api/v1/customers/{id}":                      "anonymize",
	"DELETE /api/v1/invitations/{id}":                    "revoke",
	"DELETE /api/v1/onboarding/demo":                     "archive",
	"DELETE /api/v1/restaurant/areas/{id}":               "archive",
	"DELETE /api/v1/restaurant/category-routes/{id}":     "delete configuration",
	"DELETE /api/v1/restaurant/orders/{id}/contact":      "erase PII",
	"DELETE /api/v1/restaurant/orders/{id}/lines/{line}": "delete draft line only",
	"DELETE /api/v1/restaurant/tables/{id}":              "archive",
	"DELETE /api/v1/sales/{id}/receipt-links":            "revoke bearer links",
	"DELETE /api/v1/suppliers/{id}":                      "delete only when unused",
	"DELETE /api/v1/tax-rules/{id}":                      "archive",
	"DELETE /api/v1/variants/{id}/barcodes":              "delete identifier only",
}

func TestEveryDeleteRouteHasReviewedLifecyclePolicy(t *testing.T) {
	routes, _ := registeredRoutes(t)
	seen := map[string]bool{}
	for _, route := range routes {
		if len(route) >= 7 && route[:7] == "DELETE " {
			seen[route] = true
			if _, ok := reviewedDeleteRoutes[route]; !ok {
				t.Errorf("rute %s belum punya keputusan lifecycle di reviewedDeleteRoutes", route)
			}
		}
	}
	for route := range reviewedDeleteRoutes {
		if !seen[route] {
			t.Errorf("kebijakan DELETE usang, rute tak ditemukan: %s", route)
		}
	}
}

func TestHistoricalMastersHaveNoPermanentDeleteEndpoint(t *testing.T) {
	routes, _ := registeredRoutes(t)
	forbidden := map[string]bool{
		"DELETE /api/v1/products/{id}":       true,
		"DELETE /api/v1/variants/{id}":       true,
		"DELETE /api/v1/purchase-units/{id}": true,
		"DELETE /api/v1/branches/{id}":       true,
		"DELETE /api/v1/registers/{id}":      true,
		"DELETE /api/v1/memberships/{id}":    true,
		"DELETE /api/v1/users/{id}":          true,
		"DELETE /api/v1/payments/{id}":       true,
		"DELETE /api/v1/shifts/{id}":         true,
		"DELETE /api/v1/audit/{id}":          true,
	}
	sort.Strings(routes)
	for _, route := range routes {
		if forbidden[route] {
			t.Errorf("master/dokumen historis punya DELETE permanen: %s", route)
		}
	}
}

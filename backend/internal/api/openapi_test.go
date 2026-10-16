package apiserver

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const openAPIPath = "../../../docs/openapi.yaml"

var openAPIParam = regexp.MustCompile(`\{([^}]+)\}`)

// buildOpenAPI membangkitkan kerangka kontrak /api/v1 dari rute yang terdaftar di kode.
// shortcut: operasi yang belum punya kontrak terkurasi tetap skeleton; lengkapi bertahap dengan tes HTTP per operasi.
func buildOpenAPI(t *testing.T) string {
	t.Helper()
	routes, _ := registeredRoutes(t)
	contracts := curatedContracts()
	perms := map[string]guardedEndpoint{}
	for _, ep := range guardedEndpoints(t) {
		perms[ep.method+" "+ep.path] = ep
	}
	byPath := map[string][]string{}
	for _, r := range routes {
		method, path, _ := strings.Cut(r, " ")
		byPath[path] = append(byPath[path], method)
	}
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	order := map[string]int{"GET": 0, "POST": 1, "PUT": 2, "PATCH": 3, "DELETE": 4}

	var b strings.Builder
	b.WriteString("# DIBANGKITKAN dari kode: UPDATE_OPENAPI=1 go test ./internal/api -run TestOpenAPI\n")
	b.WriteString("# Jangan edit manual; tes TestOpenAPIMatchesRegisteredRoutes gagal bila berkas ini tertinggal dari rute.\n")
	b.WriteString("openapi: 3.0.3\ninfo:\n  title: StorePOS API\n  version: v1\n  description: Kontrak bertahap. Operasi verified memiliki schema dan tes HTTP; skeleton belum lengkap. Uang berupa string sen, kuantitas berupa string milli-unit.\n")
	b.WriteString("servers:\n  - url: /\n")
	b.WriteString("components:\n  securitySchemes:\n    sessionCookie:\n      type: apiKey\n      in: cookie\n      name: " + CookieSession + "\n")
	errorSchema := objectSchema(map[string]any{"error": objectSchema(map[string]any{"code": textSchema(0), "message": textSchema(0), "details": arraySchema(objectSchema(map[string]any{"field": textSchema(0), "code": textSchema(0), "message": textSchema(0)}, "field", "code", "message")), "request_id": textSchema(0)}, "code", "message")}, "error")
	b.WriteString("  schemas:\n    ErrorResponse: " + schemaJSON(errorSchema) + "\n")
	b.WriteString("  parameters:\n    BusinessID:\n      name: " + HeaderBusinessID + "\n      in: header\n      required: false\n      description: Bisnis aktif; bila kosong memakai bisnis aktif pada sesi.\n      schema: {type: string, format: uuid}\n")
	b.WriteString("    CSRFToken:\n      name: " + HeaderCSRF + "\n      in: header\n      required: true\n      schema: {type: string}\n")
	b.WriteString("    IdempotencyKey:\n      name: Idempotency-Key\n      in: header\n      required: false\n      description: Wajib pada aksi finansial/stok tertentu (maks 128 karakter).\n      schema: {type: string}\n")
	b.WriteString("    IfMatch:\n      name: If-Match\n      in: header\n      required: false\n      description: Versi data untuk optimistic locking.\n      schema: {type: string}\n")
	b.WriteString("paths:\n")
	for _, path := range paths {
		methods := byPath[path]
		sort.Slice(methods, func(i, j int) bool { return order[methods[i]] < order[methods[j]] })
		fmt.Fprintf(&b, "  %s:\n", path)
		for _, method := range methods {
			route := method + " " + path
			public := publicRoutes[route]
			contract, verified := contracts[route]
			tag := strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")[0]
			fmt.Fprintf(&b, "    %s:\n      tags: [%s]\n      operationId: %s\n", strings.ToLower(method), tag, operationID(method, path))
			_, pendingPharmacy := pharmacyAndRecoveryContracts()[route]
			_, pendingTax := taxAndAccessContracts()[route]
			_, pendingOrder := orderFlowContracts()[route]
			_, pendingReports := reportingAndReadContracts()[route]
			_, pendingTransactions := transactionsAndStockContracts()[route]
			_, pendingShiftHistory := shiftHistoryContracts()[route]
			_, pendingRemaining := remainingContracts()[route]
			_, pendingMethods := paymentMethodContracts()[route]
			_, pendingPrescription := prescriptionAccessContracts()[route]
			if verified && (pendingPharmacy || pendingTax || pendingOrder || pendingReports || pendingTransactions || pendingShiftHistory || pendingRemaining || pendingMethods || pendingPrescription || route == "GET /api/v1/stock/reorder") {
				b.WriteString("      x-contract-status: implemented-not-tested\n")
			} else if verified {
				b.WriteString("      x-contract-status: verified\n")
			} else {
				b.WriteString("      x-contract-status: skeleton\n")
			}
			if ep, ok := perms[route]; ok {
				fmt.Fprintf(&b, "      x-permission: %s\n", ep.perm)
			}
			if public {
				b.WriteString("      security: []\n")
			} else {
				b.WriteString("      security:\n        - sessionCookie: []\n")
			}
			var params []string
			for _, m := range openAPIParam.FindAllStringSubmatch(path, -1) {
				params = append(params, fmt.Sprintf("        - {name: %s, in: path, required: true, schema: {type: string, format: uuid}}\n", m[1]))
			}
			if !public {
				if !verified || !contract.Account {
					params = append(params, "        - $ref: '#/components/parameters/BusinessID'\n")
				}
				if method != "GET" {
					params = append(params, "        - $ref: '#/components/parameters/CSRFToken'\n")
				}
			}
			if method != "GET" && !verified {
				params = append(params, "        - $ref: '#/components/parameters/IdempotencyKey'\n", "        - $ref: '#/components/parameters/IfMatch'\n")
			}
			if verified && (contract.Version || contract.OptionalVersion) {
				pattern := "^[1-9][0-9]*$"
				if contract.ZeroVersion {
					pattern = "^(0|[1-9][0-9]*)$"
				}
				params = append(params, fmt.Sprintf("        - {name: If-Match, in: header, required: %t, schema: {type: string, pattern: '%s'}}\n", contract.Version, pattern))
			}
			if verified && (contract.Idempotency || contract.OptionalIdempotency) {
				params = append(params, fmt.Sprintf("        - {name: Idempotency-Key, in: header, required: %t, schema: {type: string, maxLength: 128}}\n", contract.Idempotency))
			}
			queryNames := make([]string, 0, len(contract.Query))
			for name := range contract.Query {
				queryNames = append(queryNames, name)
			}
			sort.Strings(queryNames)
			for _, name := range queryNames {
				required := false
				for _, key := range contract.RequiredQuery {
					if key == name {
						required = true
					}
				}
				params = append(params, fmt.Sprintf("        - {name: %s, in: query, required: %t, schema: %s}\n", name, required, schemaJSON(contract.Query[name])))
			}
			if len(params) > 0 {
				b.WriteString("      parameters:\n" + strings.Join(params, ""))
			}
			if verified && contract.Request != nil {
				media := contract.RequestMedia
				if media == "" {
					media = "application/json"
				}
				b.WriteString("      requestBody:\n        required: true\n        content:\n          " + media + ":\n            schema: " + schemaJSON(contract.Request) + "\n")
			}
			b.WriteString("      responses:\n")
			if verified {
				if contract.ResponseMedia == "" {
					b.WriteString(contractResponseYAML(contract.Status, contract.Response))
				} else {
					b.WriteString(fmt.Sprintf("        '%d':\n          description: Berhasil\n          content:\n            %s:\n              schema: %s\n", contract.Status, contract.ResponseMedia, schemaJSON(contract.Response)))
				}
				if contract.CSV {
					b.WriteString("            text/csv:\n              schema: {type: string, format: binary}\n")
				}
				for _, status := range contract.AlternateSuccess {
					b.WriteString(contractResponseYAML(status, contract.Response))
				}
				b.WriteString(contractErrorYAML(404, "Data tidak ditemukan"))
				b.WriteString(contractErrorYAML(409, "Konflik versi atau lifecycle"))
			} else {
				b.WriteString("        '200': {description: Berhasil}\n")
			}
			if !public {
				if verified {
					b.WriteString(contractErrorYAML(401, "Belum masuk"))
					b.WriteString(contractErrorYAML(403, "Izin atau bisnis ditolak"))
				} else {
					b.WriteString("        '401': {description: Belum masuk}\n        '403': {description: Izin atau bisnis ditolak}\n")
				}
			}
			if verified {
				b.WriteString(contractErrorYAML(422, "Validasi gagal"))
				b.WriteString(contractErrorYAML(500, "Kesalahan internal"))
			} else {
				b.WriteString("        '422': {description: Validasi gagal}\n")
			}
		}
	}
	return b.String()
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]+`)

func operationID(method, path string) string {
	words := nonAlnum.Split(strings.TrimPrefix(path, "/api/v1/"), -1)
	id := strings.ToLower(method)
	for _, w := range words {
		if w != "" {
			id += strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return id
}

func TestOpenAPIMatchesRegisteredRoutes(t *testing.T) {
	want := buildOpenAPI(t)
	if os.Getenv("UPDATE_OPENAPI") == "1" {
		if err := os.WriteFile(openAPIPath, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(openAPIPath)
	if err != nil {
		t.Fatalf("docs/openapi.yaml tidak ada: %v (jalankan dengan UPDATE_OPENAPI=1)", err)
	}
	if string(got) != want {
		t.Fatal("docs/openapi.yaml tertinggal dari rute. Jalankan: UPDATE_OPENAPI=1 go test ./internal/api -run TestOpenAPI")
	}
}

func TestOperationIDsAreUnique(t *testing.T) {
	routes, _ := registeredRoutes(t)
	seen := map[string]string{}
	for _, r := range routes {
		method, path, _ := strings.Cut(r, " ")
		id := operationID(method, path)
		if prev, dup := seen[id]; dup {
			t.Errorf("operationId %s dipakai dua rute: %s dan %s", id, prev, r)
		}
		seen[id] = r
	}
}

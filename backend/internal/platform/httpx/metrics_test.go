package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsAggregateByRoutePatternAndExposePrometheusFormat(t *testing.T) {
	m := NewMetrics()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /widgets/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "bad" {
			http.Error(w, "boom", 500)
			return
		}
		w.WriteHeader(204)
	})
	h := m.Middleware(mux)
	for _, path := range []string{"/widgets/0192f0a0-0000-7000-8000-000000000001", "/widgets/bad"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/internal/metrics", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	m.Handler(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		`storepos_http_requests_total{method="GET",route="/widgets/{id}"} 2`,
		`storepos_http_errors_total{method="GET",route="/widgets/{id}"} 1`,
		`storepos_http_request_duration_seconds_bucket{method="GET",route="/widgets/{id}",le="+Inf"} 2`,
		`storepos_http_request_duration_seconds_count{method="GET",route="/widgets/{id}"} 2`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("tidak ada %q dalam:\n%s", want, body)
		}
	}
	if strings.Contains(body, "0192f0a0") {
		t.Fatal("UUID path bocor sebagai label metrik")
	}
}

func TestMetricsEndpointRejectsNonLoopbackAndDoesNotObserveItself(t *testing.T) {
	m := NewMetrics()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/internal/metrics", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	m.Handler(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-loopback: %d", rec.Code)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /internal/metrics", m.Handler)
	h := m.Middleware(mux)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/internal/metrics", nil)
	req.RemoteAddr = "127.0.0.1:1"
	h.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), `route="/internal/metrics"`) {
		t.Fatal("endpoint metrik mengukur dirinya sendiri")
	}
}

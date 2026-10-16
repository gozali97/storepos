package httpx

import (
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var metricBuckets = [...]time.Duration{10 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond, 250 * time.Millisecond, 500 * time.Millisecond, time.Second, 3 * time.Second}

type metricKey struct{ method, route string }
type metricValue struct {
	requests uint64
	errors   uint64
	sumSec   float64
	buckets  [len(metricBuckets) + 1]uint64 // indeks akhir = +Inf
}

// Metrics menyimpan counter/histogram latency bounded-cardinality per method+route pattern.
// Tidak mencatat tenant, user, path literal, query, atau data pelanggan.
type Metrics struct {
	mu sync.RWMutex
	m  map[metricKey]metricValue
}

func NewMetrics() *Metrics { return &Metrics{m: map[metricKey]metricValue{}} }

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		// ServeMux Pattern memuat metode; simpan path-nya saja agar method tidak ganda.
		if _, p, ok := strings.Cut(route, " "); ok {
			route = p
		}
		d := time.Since(start)
		key := metricKey{r.Method, route}
		m.mu.Lock()
		v := m.m[key]
		v.requests++
		v.sumSec += d.Seconds()
		if sw.status >= 500 {
			v.errors++
		}
		for i, bound := range metricBuckets {
			if d <= bound {
				v.buckets[i]++
			}
		}
		v.buckets[len(metricBuckets)]++
		m.m[key] = v
		m.mu.Unlock()
	})
}

// Handler menyajikan Prometheus text format hanya dari loopback. Reverse proxy jangan mengekspos path ini.
func (m *Metrics) Handler(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	m.mu.RLock()
	keys := make([]metricKey, 0, len(m.m))
	for k := range m.m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].route == keys[j].route {
			return keys[i].method < keys[j].method
		}
		return keys[i].route < keys[j].route
	})
	fmt.Fprintln(w, "# HELP storepos_http_requests_total Total HTTP requests by route pattern.")
	fmt.Fprintln(w, "# TYPE storepos_http_requests_total counter")
	fmt.Fprintln(w, "# HELP storepos_http_errors_total HTTP 5xx responses by route pattern.")
	fmt.Fprintln(w, "# TYPE storepos_http_errors_total counter")
	fmt.Fprintln(w, "# HELP storepos_http_request_duration_seconds HTTP request duration.")
	fmt.Fprintln(w, "# TYPE storepos_http_request_duration_seconds histogram")
	for _, k := range keys {
		v := m.m[k]
		labels := `method="` + escapeMetric(k.method) + `",route="` + escapeMetric(k.route) + `"`
		fmt.Fprintf(w, "storepos_http_requests_total{%s} %d\n", labels, v.requests)
		fmt.Fprintf(w, "storepos_http_errors_total{%s} %d\n", labels, v.errors)
		for i, bound := range metricBuckets {
			fmt.Fprintf(w, "storepos_http_request_duration_seconds_bucket{%s,le=\"%s\"} %d\n", labels, strconv.FormatFloat(bound.Seconds(), 'f', 3, 64), v.buckets[i])
		}
		fmt.Fprintf(w, "storepos_http_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n", labels, v.buckets[len(metricBuckets)])
		fmt.Fprintf(w, "storepos_http_request_duration_seconds_sum{%s} %s\n", labels, strconv.FormatFloat(v.sumSec, 'f', 6, 64))
		fmt.Fprintf(w, "storepos_http_request_duration_seconds_count{%s} %d\n", labels, v.requests)
	}
	m.mu.RUnlock()
}

func escapeMetric(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s)
}

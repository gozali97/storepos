package apiserver

import (
	"context"
	"github.com/storepos/storepos/internal/platform/testdb"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPublicReceiptRateLimitSharedAndRetry(t *testing.T) {
	pool := testdb.New(t)
	a, err := NewServer(pool, time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewServer(pool, time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 61; i++ {
		req := httptest.NewRequest("POST", "/api/v1/public/receipt", strings.NewReader(`{"token":"invalid"}`))
		req.RemoteAddr = "192.0.2.1:12345"
		req.Header.Set("X-Forwarded-For", "198.51.100.123")
		recorder := httptest.NewRecorder()
		server := a
		if i%2 == 1 {
			server = b
		}
		server.publicReceipt(recorder, req)
		if i < 60 && recorder.Code != http.StatusNotFound {
			t.Fatalf("request %d: %d", i, recorder.Code)
		}
		if i == 60 && (recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") == "") {
			t.Fatalf("limit: %d", recorder.Code)
		}
	}
	if _, err := pool.Underlying().Exec(context.Background(), `UPDATE public_receipt_limits SET window_start=now()-interval '2 minutes'`); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/v1/public/receipt", strings.NewReader(`{"token":"invalid"}`))
	req.RemoteAddr = "192.0.2.1:1"
	recorder := httptest.NewRecorder()
	a.publicReceipt(recorder, req)
	if recorder.Code != 404 {
		t.Fatalf("window reset: %d", recorder.Code)
	}
}

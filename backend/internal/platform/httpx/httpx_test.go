package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	errs "github.com/storepos/storepos/internal/platform/errors"
)

func TestRequestIDMiddlewareUsesHeader(t *testing.T) {
	var seen string
	mw := RequestIDMiddleware(nil)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestID(r.Context())
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(HeaderRequestID, "abc123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen != "abc123" {
		t.Errorf("got rid=%q", seen)
	}
	if got := rec.Header().Get(HeaderRequestID); got != "abc123" {
		t.Errorf("response rid=%q", got)
	}
}

func TestRequestIDMiddlewareGeneratesID(t *testing.T) {
	mw := RequestIDMiddleware(nil)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestID(r.Context()) == "" {
			t.Error("rid kosong")
		}
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get(HeaderRequestID) == "" {
		t.Error("header kosong")
	}
}

func TestWriteErrorMapsAppError(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	WriteError(rec, req, errs.NotFound("produk"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	var body ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != errs.CodeNotFound {
		t.Fatalf("code = %q", body.Error.Code)
	}
}

func TestWriteErrorWrapsUnknown(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	WriteError(rec, req, http.ErrAbortHandler)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestDecodeJSONDisallowsUnknown(t *testing.T) {
	type P struct {
		A string `json:"a"`
	}
	body := strings.NewReader(`{"a":"x","b":"y"}`)
	req := httptest.NewRequest("POST", "/", body)
	var p P
	if err := DecodeJSON(req, &p); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestRecoverTranslatesPanic(t *testing.T) {
	h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	// Wrap dengan RequestID middleware supaya context memiliki logger.
	h = RequestIDMiddleware(nil)(h)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestWriteErrorMapsDatabaseStateTransitionTo409(t *testing.T) {
	cause := fmt.Errorf("sales: ubah status: %w", &pgconn.PgError{Code: SQLStateInvalidTransition, Message: "transisi tidak diizinkan"})
	rec := httptest.NewRecorder()
	WriteError(rec, httptest.NewRequest("POST", "/", nil), cause)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), errs.CodeInvalidStateTransit) {
		t.Fatalf("status/isi: %d %s", rec.Code, rec.Body.String())
	}
	// Error database lain tetap 500 dan tidak membocorkan pesan internal.
	rec = httptest.NewRecorder()
	WriteError(rec, httptest.NewRequest("POST", "/", nil), &pgconn.PgError{Code: "23505", Message: "rahasia internal"})
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "rahasia") {
		t.Fatalf("error lain: %d %s", rec.Code, rec.Body.String())
	}
}

func TestForbiddenResponsesLogSafeRouteWithoutSensitivePath(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /private/{id}", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, errs.PermissionDenied("private-error-secret"))
	})
	h := RequestIDMiddleware(log)(mux)
	req := httptest.NewRequest("POST", "/private/customer-secret?token=token-secret", strings.NewReader("body-secret"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	text := buf.String()
	for _, want := range []string{"access_denied", "/private/{id}", errs.CodePermissionDenied} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s in %s", want, text)
		}
	}
	for _, secret := range []string{"customer-secret", "token-secret", "body-secret", "private-error-secret"} {
		if strings.Contains(text, secret) {
			t.Errorf("sensitive data logged: %s", secret)
		}
	}
}

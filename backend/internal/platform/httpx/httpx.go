// Package httpx menyediakan utilitas HTTP: request ID, respons JSON, error terstruktur.
package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	errs "github.com/storepos/storepos/internal/platform/errors"
)

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxLogger
)

const HeaderRequestID = "X-Request-ID"

// SQLStateInvalidTransition dipakai trigger state machine di database (migrasi 0062).
const SQLStateInvalidTransition = "ST001"

// RequestID mengembalikan request ID dari context. "" bila tidak ada.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxRequestID).(string); ok {
		return v
	}
	return ""
}

// Logger mengembalikan logger ber-request-id dari context. Fallback: slog.Default.
func Logger(ctx context.Context) *slog.Logger {
	if v, ok := ctx.Value(ctxLogger).(*slog.Logger); ok {
		return v
	}
	return slog.Default()
}

// RequestIDMiddleware memastikan setiap request memiliki ID stabil (header atau baru).
// Menyimpan ID + logger ber-field "request_id" di context, mengatur response header.
func RequestIDMiddleware(base *slog.Logger) func(http.Handler) http.Handler {
	if base == nil {
		base = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get(HeaderRequestID)
			if rid == "" {
				rid = newID()
			}
			w.Header().Set(HeaderRequestID, rid)
			log := base.With("request_id", rid)
			ctx := context.WithValue(r.Context(), ctxRequestID, rid)
			ctx = context.WithValue(ctx, ctxLogger, log)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AccessLogMiddleware mencatat ringkasan setiap request.
// Tidak mencatat body atau header sensitif.
func AccessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		Logger(r.Context()).Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes", sw.bytes,
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

// Unwrap lets ResponseController access Flush/deadlines through access-log middleware.
func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// JSON menulis status + body JSON.
func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

// ErrorResponse adalah format error seragam (PRD §16.4).
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Details   []errs.FieldError `json:"details,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

// WriteError memetakan error ke respons JSON seragam.
// AppError memakai kode dan status aslinya. Error lain dianggap INTERNAL.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	ae, ok := errs.As(err)
	if !ok {
		// Trigger state machine database (SQLSTATE ST001) berarti klien meminta transisi status yang tidak sah.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == SQLStateInvalidTransition {
			ae = errs.New(errs.CodeInvalidStateTransit, "transisi status tidak diizinkan", http.StatusConflict).Wrap(err)
		} else {
			ae = errs.Internal("terjadi kesalahan pada server").Wrap(err)
		}
	}
	if ae.HTTP == http.StatusForbidden {
		Logger(r.Context()).Info("access_denied", "method", r.Method, "route", r.Pattern, "code", ae.Code)
	}
	rid := RequestID(r.Context())
	body := ErrorResponse{Error: ErrorBody{
		Code:      ae.Code,
		Message:   ae.Message,
		Details:   ae.Details,
		RequestID: rid,
	}}
	if ae.HTTP >= 500 {
		Logger(r.Context()).Error("request failed",
			"code", ae.Code,
			"err", ae.Error(),
		)
	}
	JSON(w, ae.HTTP, body)
}

// DecodeJSON mem-parse body JSON dengan batas 1 MB.
func DecodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errs.Validation("payload JSON tidak valid").Wrap(err)
	}
	if dec.More() {
		return errs.Validation("payload JSON mengandung data tambahan")
	}
	return nil
}

func newID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback: timestamp. Tabrakan di jalur ini tidak kritis karena hanya untuk request id.
		return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000")))
	}
	return hex.EncodeToString(b[:])
}

// Recover memastikan panic dalam handler menjadi 500 dengan request id tercatat.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				err, ok := rec.(error)
				if !ok {
					err = errors.New("panic")
				}
				Logger(r.Context()).Error("panic recovered", "panic", rec)
				WriteError(w, r, errs.Internal("terjadi kesalahan tak terduga").Wrap(err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Package errs mendefinisikan tipe error aplikasi dengan kode stabil.
// Kode dipakai di respons API (PRD §16.4) dan dicocokkan di uji.
package errs

import (
	"errors"
	"fmt"
	"net/http"
)

// Kode error stabil yang dirujuk PRD. Jangan diubah tanpa menaikkan versi API.
const (
	CodeInternal              = "INTERNAL"
	CodeValidation            = "VALIDATION"
	CodeNotFound              = "NOT_FOUND"
	CodeConflict              = "CONFLICT"
	CodePermissionDenied      = "PERMISSION_DENIED"
	CodeUnauthenticated       = "UNAUTHENTICATED"
	CodeModuleDisabled        = "MODULE_DISABLED"
	CodeIdempotencyConflict   = "IDEMPOTENCY_CONFLICT"
	CodeInvalidStateTransit   = "INVALID_STATE_TRANSITION"
	CodeStockInsufficient     = "STOCK_INSUFFICIENT"
	CodeShiftNotOpen          = "SHIFT_NOT_OPEN"
	CodeBatchExpired          = "BATCH_EXPIRED"
	CodePrescriptionRequired  = "PRESCRIPTION_REFERENCE_REQUIRED"
	CodeApprovalRequired      = "APPROVAL_REQUIRED"
	CodeRateLimited           = "RATE_LIMITED"
	CodeTooManyRequests       = "TOO_MANY_REQUESTS"
	CodeOptimisticLockFailure = "VERSION_CONFLICT"
)

// AppError adalah error terstruktur dengan kode stabil.
type AppError struct {
	Code    string
	Message string
	HTTP    int
	Details []FieldError
	cause   error
}

// FieldError menjelaskan error validasi per field.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.cause }

// Wrap melampirkan error asal untuk logging tanpa mengubah kode.
func (e *AppError) Wrap(cause error) *AppError {
	e.cause = cause
	return e
}

// WithDetails menambahkan daftar FieldError.
func (e *AppError) WithDetails(d ...FieldError) *AppError {
	e.Details = append(e.Details, d...)
	return e
}

// New membuat AppError dengan kode dan pesan.
func New(code, msg string, status int) *AppError {
	return &AppError{Code: code, Message: msg, HTTP: status}
}

// Validation mengembalikan error validasi dengan status 422.
func Validation(msg string, details ...FieldError) *AppError {
	return &AppError{Code: CodeValidation, Message: msg, HTTP: http.StatusUnprocessableEntity, Details: details}
}

// NotFound mengembalikan error 404.
func NotFound(what string) *AppError {
	return &AppError{Code: CodeNotFound, Message: what + " tidak ditemukan", HTTP: http.StatusNotFound}
}

// Conflict mengembalikan error 409.
func Conflict(msg string) *AppError {
	return &AppError{Code: CodeConflict, Message: msg, HTTP: http.StatusConflict}
}

// PermissionDenied mengembalikan error 403.
func PermissionDenied(msg string) *AppError {
	if msg == "" {
		msg = "akses ditolak"
	}
	return &AppError{Code: CodePermissionDenied, Message: msg, HTTP: http.StatusForbidden}
}

// Unauthenticated mengembalikan error 401.
func Unauthenticated(msg string) *AppError {
	if msg == "" {
		msg = "butuh autentikasi"
	}
	return &AppError{Code: CodeUnauthenticated, Message: msg, HTTP: http.StatusUnauthorized}
}

// ModuleDisabled mengembalikan error 403 MODULE_DISABLED (PRD CORE-002).
func ModuleDisabled(mod string) *AppError {
	return &AppError{Code: CodeModuleDisabled, Message: "modul " + mod + " tidak aktif", HTTP: http.StatusForbidden}
}

// Internal mengembalikan error 500.
func Internal(msg string) *AppError {
	return &AppError{Code: CodeInternal, Message: msg, HTTP: http.StatusInternalServerError}
}

// As memanggil errors.As dengan tipe AppError.
func As(err error) (*AppError, bool) {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

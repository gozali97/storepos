package errs_test

import (
	"errors"
	"net/http"
	"testing"

	errs "github.com/storepos/storepos/internal/platform/errors"
)

func TestNotFoundDefaultsHTTP404(t *testing.T) {
	e := errs.NotFound("produk")
	if e.HTTP != http.StatusNotFound {
		t.Errorf("HTTP = %d, want 404", e.HTTP)
	}
	if e.Code != errs.CodeNotFound {
		t.Errorf("Code = %q", e.Code)
	}
}

func TestAppErrorUnwrap(t *testing.T) {
	inner := errors.New("boom")
	e := errs.Internal("gagal").Wrap(inner)
	if !errors.Is(e, inner) {
		t.Fatal("Unwrap tidak mengembalikan cause")
	}
}

func TestAsRecognisesAppError(t *testing.T) {
	e := errs.PermissionDenied("")
	ae, ok := errs.As(e)
	if !ok || ae.Code != errs.CodePermissionDenied {
		t.Fatal("As gagal mengenali AppError")
	}
}

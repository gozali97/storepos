package identity

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// nullIfEmpty mengubah string kosong menjadi nil agar kolom UNIQUE tetap
// memperbolehkan banyak baris tanpa nilai (email NULL vs email ”).
func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// isUniqueViolation memeriksa pelanggaran constraint UNIQUE untuk nama constraint tertentu.
// Jika nama kosong, setiap pelanggaran unique dianggap cocok.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != "23505" {
		return false
	}
	if constraint == "" {
		return true
	}
	return pgErr.ConstraintName == constraint
}

package org

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// isUniqueViolation memeriksa pelanggaran constraint UNIQUE untuk nama constraint tertentu.
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

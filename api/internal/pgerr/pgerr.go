// Package pgerr translates raw Postgres error codes into booleans the domain
// repositories can branch on, so each repo turns the right driver error into
// its own sentinel (ErrConflict, etc.) without re-learning the code numbers.
//
// Course mapping: Chapter 12 — error handling (the course keeps this in
// internal/storage/postgres/errors.go; we give it its own tiny leaf package).
package pgerr

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// IsUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505).
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// IsForeignKeyViolation reports whether err is a Postgres foreign-key
// violation (SQLSTATE 23503).
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

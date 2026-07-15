//go:build !postgresql

// Package deltascope provides the PostgreSQL session stub when built without the postgresql tag.
// input: none (stub only)
// output: ErrPostgreSQLSessionNotAvailable for all calls
// pos: public stub for non-PostgreSQL builds
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"database/sql"
)

// NewPostgreSQLQueryAccessSessionFromConn returns ErrPostgreSQLSessionNotAvailable when built without the postgresql tag.
func NewPostgreSQLQueryAccessSessionFromConn(_ context.Context, _ *sql.Conn) (*PostgreSQLQueryAccessSession, error) {
	return nil, ErrPostgreSQLSessionNotAvailable
}

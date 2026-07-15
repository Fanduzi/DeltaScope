//go:build !postgresql

// Package deltascope provides the PostgreSQL session stub when built without the postgresql tag.
// input: none (stub only)
// output: ErrPostgreSQLSessionNotAvailable for all calls
// pos: public stub for non-PostgreSQL builds
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"database/sql"
	"errors"
)

// ErrPostgreSQLSessionNotAvailable indicates PostgreSQL session support was not compiled in.
var ErrPostgreSQLSessionNotAvailable = errors.New("postgresql session support requires build tag: go build -tags postgresql")

// PostgreSQLQueryAccessSession is an opaque wrapper for a caller-owned PostgreSQL connection.
// It is not available without the postgresql build tag.
type PostgreSQLQueryAccessSession struct{}

// NewPostgreSQLQueryAccessSessionFromConn returns ErrPostgreSQLSessionNotAvailable when built without the postgresql tag.
func NewPostgreSQLQueryAccessSessionFromConn(_ *sql.Conn) (*PostgreSQLQueryAccessSession, error) {
	return nil, ErrPostgreSQLSessionNotAvailable
}

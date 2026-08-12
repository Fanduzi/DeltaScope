//go:build !postgresql

// Package deltascope provides the PostgreSQL session stub when built without the postgresql tag.
// input: none (stub only)
// output: ErrPostgreSQLSessionNotAvailable for legacy calls; fail-closed unified PG17 route stub
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

// AnalyzePostgreSQLQueryAccessWithSession returns ErrPostgreSQLSessionNotAvailable when built without the postgresql tag.
func AnalyzePostgreSQLQueryAccessWithSession(_ context.Context, _ *PostgreSQLQueryAccessSession, _ QueryAccessRequest) (*QueryAccessResult, error) {
	return nil, ErrPostgreSQLSessionNotAvailable
}

// analyzePostgreSQLOnline is the no-tag fail-closed stub for the unified
// online entry's PostgreSQL route. The capability seam already fails an
// observed PostgreSQL target before routing, so this stub exists only to keep
// the untagged routing shell compilable; it must never be reached by a
// public caller.
func analyzePostgreSQLOnline(_ context.Context, _ *sql.Conn, _ QueryAccessRequest) (*QueryAccessResult, error) {
	return nil, ErrOnlineQueryAccessCapabilityUnsupported
}

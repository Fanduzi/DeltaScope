// Package deltascope exposes the public library surface for consumers.
// input: caller-owned *sql.Conn for PostgreSQL trusted query access
// output: shared types and errors for session API across build tags
// pos: public shared session types (no build tag)
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"database/sql"
	"errors"
)

// ErrPostgreSQLSessionNotAvailable indicates PostgreSQL session support was not compiled in.
// This error is returned by the stub constructor when built without the postgresql tag.
var ErrPostgreSQLSessionNotAvailable = errors.New("postgresql session support requires build tag: go build -tags postgresql")

// PostgreSQLQueryAccessSession is an opaque wrapper around a caller-owned
// *sql.Conn for trusted PostgreSQL query access analysis.
//
// The session does not own or close the caller's connection. The caller
// retains full lifecycle control. Analysis on an already-closed connection
// returns a bounded error.
//
// The wrapper exposes no OIDs, manifest entries, catalog SQL, credentials,
// session binding, or Trusted flag. It has no JSON-marshalable fields.
type PostgreSQLQueryAccessSession struct {
	conn *sql.Conn
}

//go:build postgresql

// Package deltascope exposes the public library surface for consumers.
// input: caller-owned *sql.Conn for PostgreSQL trusted query access
// output: opaque session wrapper for manifest-gated analysis
// pos: public trusted query access session API
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	pgmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/postgresql"
)

var (
	errNilConnection      = errors.New("postgresql session: connection must not be nil")
	errConnectionNotAlive = errors.New("postgresql session: connection is not alive")
)

// NewPostgreSQLQueryAccessSessionFromConn creates an opaque session from a
// caller-owned *sql.Conn. The connection must be non-nil and alive.
//
// The ctx parameter controls the liveness check timeout. If ctx is nil,
// the connection is validated without a timeout. If ctx is canceled or
// exceeds its deadline, the liveness check fails with a bounded error.
//
// The session does not take ownership of the connection. The caller must
// close it after analysis. A *sql.DB constructor is intentionally not
// provided: DeltaScope cannot return a pool-allocated connection for
// subsequent execution without a separate execution-affinity design.
//
// Returns a bounded error if the connection is nil, closed, or the context
// is canceled. Error text never contains driver internals, DSN, username,
// password, SQL, or catalog query text.
func NewPostgreSQLQueryAccessSessionFromConn(ctx context.Context, conn *sql.Conn) (*PostgreSQLQueryAccessSession, error) {
	if conn == nil {
		return nil, errNilConnection
	}
	if ctx == nil {
		return nil, errNilConnection
	}
	// Verify the connection is alive with a minimal probe.
	if err := conn.PingContext(ctx); err != nil {
		return nil, errConnectionNotAlive
	}
	return &PostgreSQLQueryAccessSession{conn: conn}, nil
}

// newTrustedServiceFromSession creates a trusted application Service from
// the session's connection. This is the private assembly point that wires:
//   - *sql.Conn-backed metadata resolver (same connection)
//   - PinnedSession + EffectIdentityAdapter (same connection)
//   - PG17 trust policy
//
// All resolvers use the same *sql.Conn, ensuring same-backend catalog access.
func newTrustedServiceFromSession(session *PostgreSQLQueryAccessSession) (*appqa.Service, error) {
	if session == nil || session.conn == nil {
		return nil, errNilConnection
	}

	conn := session.conn

	// 1. Create pinned session from the same *sql.Conn.
	pinned, err := pgmeta.NewPinnedSessionFromConn(conn)
	if err != nil {
		return nil, fmt.Errorf("pin session: %w", err)
	}

	// 2. Create identity adapter from the pinned session.
	adapter, err := pgmeta.NewEffectIdentityAdapter(pinned)
	if err != nil {
		return nil, fmt.Errorf("identity adapter: %w", err)
	}

	// 3. Create conn-backed metadata resolver from the same *sql.Conn.
	connResolver, err := pgmeta.NewQueryAccessConnResolver(conn)
	if err != nil {
		return nil, fmt.Errorf("conn resolver: %w", err)
	}

	// 4. Create trust policy with PG17 manifest.
	manifest := appqa.NewPG17Manifest()
	policy, err := appqa.NewTrustPolicy(manifest)
	if err != nil {
		return nil, fmt.Errorf("trust policy: %w", err)
	}

	// 5. Create trusted service.
	svc, err := appqa.NewTrustedService(adapter, policy, connResolver)
	if err != nil {
		return nil, fmt.Errorf("trusted service: %w", err)
	}

	return svc, nil
}

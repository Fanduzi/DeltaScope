//go:build postgresql

// Package deltascope exposes the public library surface for consumers.
// input: caller-owned *sql.Conn for PostgreSQL trusted query access
// output: opaque session wrapper for manifest-gated analysis plus the shared private PG17 proof core used by the unified online entry
// pos: public trusted query access session API and shared PG proof execution core (postgresql build tag)
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	online "github.com/Fanduzi/DeltaScope/internal/application/online"
	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	pgmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/postgresql"
)

var (
	errNilConnection            = errors.New("postgresql session: connection must not be nil")
	errConnectionNotAlive       = errors.New("postgresql session: connection is not alive")
	errTrustedDialectRequired   = errors.New("trusted analysis requires PostgreSQL dialect")
	errSchemaResolverNotAllowed = errors.New("trusted analysis does not accept an external schema resolver")
	errIdentityNotPostgreSQL    = errors.New("server identity is not PostgreSQL 17")
)

// NewPostgreSQLQueryAccessSessionFromConn creates an opaque session from a
// caller-owned *sql.Conn. The connection must be non-nil and alive.
// Server identity is validated at construction time.
//
// Deprecated: Use NewOnlineQueryAccessSessionFromConn.
func NewPostgreSQLQueryAccessSessionFromConn(ctx context.Context, conn *sql.Conn) (*PostgreSQLQueryAccessSession, error) {
	if conn == nil {
		return nil, errNilConnection
	}
	if ctx == nil {
		return nil, errNilConnection
	}
	if err := conn.PingContext(ctx); err != nil {
		return nil, errConnectionNotAlive
	}

	identity, err := online.IdentifyFromConn(ctx, conn, "postgresql")
	if err != nil {
		return nil, errIdentityNotPostgreSQL
	}

	target := online.DeriveCapabilityTarget(identity)
	if target != online.TargetPG17 {
		return nil, errIdentityNotPostgreSQL
	}

	return &PostgreSQLQueryAccessSession{conn: conn, target: target}, nil
}

// AnalyzePostgreSQLQueryAccessWithSession performs trusted PostgreSQL query
// access analysis using a caller-owned connection session.
// Rejects a non-empty caller AnalysisProfile; PostgreSQL always uses PG17 manifest.
//
// Deprecated: Use AnalyzeOnlineQueryAccessWithSession.
func AnalyzePostgreSQLQueryAccessWithSession(
	ctx context.Context,
	session *PostgreSQLQueryAccessSession,
	req QueryAccessRequest,
) (*QueryAccessResult, error) {
	if ctx == nil {
		return nil, errNilConnection
	}
	if session == nil {
		return nil, errNilConnection
	}
	if req.Dialect != DialectPostgreSQL {
		return nil, errTrustedDialectRequired
	}
	if req.AnalysisProfile != QueryAccessAnalysisProfileEmpty {
		return nil, ErrPostgreSQLQueryAccessProfileNotAllowed
	}
	if req.SchemaResolver != nil {
		return nil, errSchemaResolverNotAllowed
	}

	return analyzePostgreSQLOnline(ctx, session.conn, req)
}

// analyzePostgreSQLOnline is the shared private execution core for trusted
// PostgreSQL 17 proof. It binds the same-connection pinned session, effect
// identity adapter, and schema resolver, runs the exact PG17 manifest and
// trust policy with the COUNT(1) envelope, and never executes user SQL. Both
// the existing PostgreSQL session API and the unified online session entry
// route through this core; public validation policy stays in each public
// function. The connection remains caller-owned: it is never closed, pooled,
// or retried here.
func analyzePostgreSQLOnline(
	ctx context.Context,
	conn *sql.Conn,
	req QueryAccessRequest,
) (*QueryAccessResult, error) {
	svc, err := newTrustedServiceFromConn(conn)
	if err != nil {
		return nil, err
	}

	mode, err := toDomainQAMode(req.Mode)
	if err != nil {
		return nil, err
	}

	appResult, err := svc.Analyze(ctx, appqa.QueryAccessRequest{
		SQL:           req.SQL,
		Dialect:       "postgresql",
		Mode:          string(mode),
		DefaultSchema: req.DefaultSchema,
	})
	if err != nil {
		if errors.Is(err, appqa.ErrExtractionFailed) {
			return nil, err
		}
		return nil, fmt.Errorf("query access analysis: %w", err)
	}

	result := fromDomainQAResult(appResult.DomainResult)
	return &result, nil
}

// newTrustedServiceFromSession creates a trusted application Service from
// the session's connection. All resolvers use the same *sql.Conn, ensuring
// same-backend catalog access.
func newTrustedServiceFromSession(session *PostgreSQLQueryAccessSession) (*appqa.Service, error) {
	if session == nil || session.conn == nil {
		return nil, errNilConnection
	}
	return newTrustedServiceFromConn(session.conn)
}

// newTrustedServiceFromConn creates a trusted application Service from a
// caller-owned *sql.Conn. All resolvers use the same connection, ensuring
// same-backend catalog access for identity, relation, column, and function
// proof.
func newTrustedServiceFromConn(conn *sql.Conn) (*appqa.Service, error) {
	if conn == nil {
		return nil, errNilConnection
	}

	pinned, err := pgmeta.NewPinnedSessionFromConn(conn)
	if err != nil {
		return nil, fmt.Errorf("pin session: %w", err)
	}

	adapter, err := pgmeta.NewEffectIdentityAdapter(pinned)
	if err != nil {
		return nil, fmt.Errorf("identity adapter: %w", err)
	}

	connResolver, err := pgmeta.NewQueryAccessConnResolver(conn)
	if err != nil {
		return nil, fmt.Errorf("conn resolver: %w", err)
	}

	manifest := appqa.NewPG17Manifest()
	policy, err := appqa.NewTrustPolicy(manifest)
	if err != nil {
		return nil, fmt.Errorf("trust policy: %w", err)
	}

	svc, err := appqa.NewTrustedService(adapter, policy, connResolver)
	if err != nil {
		return nil, fmt.Errorf("trusted service: %w", err)
	}

	return svc, nil
}

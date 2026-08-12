// Package deltascope exposes the explicit MySQL/TiDB query access session boundary.
// input: caller-owned *sql.Conn and a validated MySQL/TiDB query access request
// output: query access results with same-connection relation metadata resolution
// pos: public opt-in session API and shared private proof core for same-connection semantic promotion
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	online "github.com/Fanduzi/DeltaScope/internal/application/online"
	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

var (
	ErrMySQLTiDBQueryAccessSessionUnavailable       = errors.New("mysql/tidb query access session is unavailable")
	ErrMySQLTiDBQueryAccessDialectRequired          = errors.New("mysql/tidb query access session requires MySQL or TiDB dialect")
	ErrMySQLTiDBQueryAccessSchemaResolverNotAllowed = errors.New("mysql/tidb query access session does not accept an external schema resolver")
	ErrMySQLTiDBQueryAccessProfileNotAllowed        = errors.New("mysql/tidb query access session rejects caller analysis profile; capability is derived from server identity")
)

// MySQLTiDBQueryAccessSession is an opaque wrapper around a caller-owned *sql.Conn.
// The session derives its capability target from server identity at construction time.
type MySQLTiDBQueryAccessSession struct {
	conn   *sql.Conn
	target online.CapabilityTarget
}

// NewMySQLTiDBQueryAccessSessionFromConn creates a session after a context-controlled
// liveness check and server identity validation.
func NewMySQLTiDBQueryAccessSessionFromConn(ctx context.Context, conn *sql.Conn) (*MySQLTiDBQueryAccessSession, error) {
	if ctx == nil || conn == nil {
		return nil, ErrMySQLTiDBQueryAccessSessionUnavailable
	}
	if err := conn.PingContext(ctx); err != nil {
		return nil, ErrMySQLTiDBQueryAccessSessionUnavailable
	}

	identity, err := online.IdentifyFromConn(ctx, conn, "")
	if err != nil {
		return nil, ErrMySQLTiDBQueryAccessSessionUnavailable
	}

	target := online.DeriveCapabilityTarget(identity)
	if target == "" {
		return nil, ErrMySQLTiDBQueryAccessSessionUnavailable
	}

	return &MySQLTiDBQueryAccessSession{conn: conn, target: target}, nil
}

// AnalyzeMySQLTiDBQueryAccessWithSession resolves relation metadata on the caller's connection and
// enables the private application semantic capability for the session-owned resolver.
// Rejects a non-empty caller AnalysisProfile; the capability is derived from server identity.
func AnalyzeMySQLTiDBQueryAccessWithSession(
	ctx context.Context,
	session *MySQLTiDBQueryAccessSession,
	req QueryAccessRequest,
) (*QueryAccessResult, error) {
	if ctx == nil {
		return nil, ErrMySQLTiDBQueryAccessSessionUnavailable
	}
	if req.Dialect != DialectMySQL && req.Dialect != DialectTiDB {
		return nil, ErrMySQLTiDBQueryAccessDialectRequired
	}
	if req.AnalysisProfile != QueryAccessAnalysisProfileEmpty {
		return nil, ErrMySQLTiDBQueryAccessProfileNotAllowed
	}
	if req.SchemaResolver != nil {
		return nil, ErrMySQLTiDBQueryAccessSchemaResolverNotAllowed
	}
	if session == nil || session.conn == nil {
		return nil, ErrMySQLTiDBQueryAccessSessionUnavailable
	}

	return analyzeMySQLTiDBOnline(ctx, session.conn, session.target, toDomainQADialect(req.Dialect), req)
}

// analyzeMySQLTiDBOnline is the shared private execution core for MySQL/TiDB
// online proof. It derives the analysis profile from the identity-derived
// capability target, binds the same-connection resolver, and runs the private
// builtin semantic service. Both the dialect-specific session API and the
// unified online session entry route through this helper; public validation
// policy stays in each public function.
func analyzeMySQLTiDBOnline(
	ctx context.Context,
	conn *sql.Conn,
	target online.CapabilityTarget,
	dialect string,
	req QueryAccessRequest,
) (*QueryAccessResult, error) {
	profile := capabilityTargetToAnalysisProfile(target)

	resolver, err := mysqlmeta.NewQueryAccessConnResolver(conn)
	if err != nil {
		return nil, ErrMySQLTiDBQueryAccessSessionUnavailable
	}
	mode, err := toDomainQAMode(req.Mode)
	if err != nil {
		return nil, err
	}

	service, err := appqa.NewMySQLTiDBSemanticService(resolver)
	if err != nil {
		return nil, ErrMySQLTiDBQueryAccessSessionUnavailable
	}
	appResult, err := service.Analyze(ctx, appqa.QueryAccessRequest{
		SQL:             req.SQL,
		Dialect:         dialect,
		Mode:            string(mode),
		DefaultSchema:   req.DefaultSchema,
		AnalysisProfile: profile,
		SchemaResolver:  resolver,
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

// capabilityTargetToAnalysisProfile maps an identity-derived capability target
// to the application-layer analysis profile.
func capabilityTargetToAnalysisProfile(target online.CapabilityTarget) appqa.AnalysisProfile {
	switch target {
	case online.TargetMySQL57:
		return appqa.AnalysisProfileMySQL57
	case online.TargetMySQL80:
		return appqa.AnalysisProfileMySQL80
	case online.TargetMySQL84:
		return appqa.AnalysisProfileMySQL84
	case online.TargetTiDB85:
		return appqa.AnalysisProfileTiDB85
	default:
		return appqa.AnalysisProfileEmpty
	}
}

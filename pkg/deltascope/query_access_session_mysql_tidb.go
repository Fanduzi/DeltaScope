// Package deltascope exposes the explicit MySQL/TiDB query access session boundary.
// input: caller-owned *sql.Conn and a validated MySQL/TiDB query access request
// output: query access results with same-connection relation metadata resolution
// pos: public opt-in session API for same-connection semantic promotion
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

var (
	ErrMySQLTiDBQueryAccessSessionUnavailable       = errors.New("mysql/tidb query access session is unavailable")
	ErrMySQLTiDBQueryAccessDialectRequired          = errors.New("mysql/tidb query access session requires MySQL or TiDB dialect")
	ErrMySQLTiDBQueryAccessSchemaResolverNotAllowed = errors.New("mysql/tidb query access session does not accept an external schema resolver")
)

// MySQLTiDBQueryAccessSession is an opaque wrapper around a caller-owned *sql.Conn.
type MySQLTiDBQueryAccessSession struct {
	conn *sql.Conn
}

// NewMySQLTiDBQueryAccessSessionFromConn creates a session after a context-controlled liveness check.
func NewMySQLTiDBQueryAccessSessionFromConn(ctx context.Context, conn *sql.Conn) (*MySQLTiDBQueryAccessSession, error) {
	if ctx == nil || conn == nil {
		return nil, ErrMySQLTiDBQueryAccessSessionUnavailable
	}
	if err := conn.PingContext(ctx); err != nil {
		return nil, ErrMySQLTiDBQueryAccessSessionUnavailable
	}
	return &MySQLTiDBQueryAccessSession{conn: conn}, nil
}

// AnalyzeMySQLTiDBQueryAccessWithSession resolves relation metadata on the caller's connection and
// enables the private application semantic capability for the session-owned resolver.
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
	dialect := toDomainQADialect(req.Dialect)
	if err := validateQueryAccessAnalysisProfile(req.AnalysisProfile, dialect); err != nil {
		return nil, err
	}
	if req.SchemaResolver != nil {
		return nil, ErrMySQLTiDBQueryAccessSchemaResolverNotAllowed
	}
	if session == nil || session.conn == nil {
		return nil, ErrMySQLTiDBQueryAccessSessionUnavailable
	}

	resolver, err := mysqlmeta.NewQueryAccessConnResolver(session.conn)
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
		AnalysisProfile: appqa.AnalysisProfile(req.AnalysisProfile),
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

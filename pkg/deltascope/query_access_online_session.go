// Package deltascope exposes the unified online query access session boundary.
// input: caller-owned *sql.Conn, context, and a query access request with optional dialect constraint
// output: opaque unified session, generic online analysis entry, five bounded sentinel errors, and MySQL/TiDB/PG17 routing
// pos: public unified online query access session API above dialect-specific entries
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"database/sql"
	"errors"

	online "github.com/Fanduzi/DeltaScope/internal/application/online"
)

// Generic online query access sentinel errors. These are the bounded public
// boundary of the unified entry; callers must use errors.Is, not string
// matching. They never embed credentials, endpoints, raw versions, catalog
// facts, or driver text.
var (
	// ErrOnlineQueryAccessSessionUnavailable indicates the context or caller-owned
	// connection session was unusable (nil input, failed liveness, or failed
	// identity acquisition).
	ErrOnlineQueryAccessSessionUnavailable = errors.New("online query access session is unavailable")
	// ErrOnlineQueryAccessDialectMismatch indicates a non-empty request dialect
	// did not match the identity observed on the connection.
	ErrOnlineQueryAccessDialectMismatch = errors.New("online query access dialect mismatch")
	// ErrOnlineQueryAccessProfileNotAllowed indicates a caller supplied an
	// analysis profile; capability is derived from observed server identity.
	ErrOnlineQueryAccessProfileNotAllowed = errors.New("online query access rejects caller analysis profile; capability is derived from server identity")
	// ErrOnlineQueryAccessSchemaResolverNotAllowed indicates a caller supplied an
	// external schema resolver; online proof must use the same-connection resolver.
	ErrOnlineQueryAccessSchemaResolverNotAllowed = errors.New("online query access does not accept an external schema resolver")
	// ErrOnlineQueryAccessCapabilityUnsupported indicates the observed server
	// capability is recognized but not supported by this build or milestone.
	ErrOnlineQueryAccessCapabilityUnsupported = errors.New("online query access capability is not supported")
)

// OnlineQueryAccessSession is an opaque wrapper around a caller-owned *sql.Conn.
//
// Construction pings and identifies the server on that connection and stores
// only the private connection and identity-derived routing target. The session
// never opens, pools, closes, or retries the connection; the caller retains
// full lifecycle control. It exposes no identity, product, dialect, profile,
// capability, connection, or JSON-visible state and has no public getters.
type OnlineQueryAccessSession struct {
	conn   *sql.Conn
	target online.CapabilityTarget
}

// NewOnlineQueryAccessSessionFromConn creates a unified online session from a
// caller-owned *sql.Conn. The constructor performs its own liveness check and
// derives the routing target from observed server identity; callers cannot
// supply a product, dialect, profile, version, or capability target.
//
// Nil context or connection, failed liveness, and untrustworthy identity
// acquisition (unknown product, malformed version, failed version query) map
// to ErrOnlineQueryAccessSessionUnavailable. A recognized product whose
// version series is outside the supported set (for example MySQL 8.1, TiDB
// 7.x, PostgreSQL 16) and an observed PostgreSQL target in a source build
// without the postgresql tag map to ErrOnlineQueryAccessCapabilityUnsupported.
func NewOnlineQueryAccessSessionFromConn(ctx context.Context, conn *sql.Conn) (*OnlineQueryAccessSession, error) {
	if ctx == nil || conn == nil {
		return nil, ErrOnlineQueryAccessSessionUnavailable
	}
	if err := conn.PingContext(ctx); err != nil {
		return nil, ErrOnlineQueryAccessSessionUnavailable
	}

	identity, err := online.IdentifyFromConn(ctx, conn, "")
	if err != nil {
		if errors.Is(err, online.ErrIdentityUnsupported) {
			return nil, ErrOnlineQueryAccessCapabilityUnsupported
		}
		return nil, ErrOnlineQueryAccessSessionUnavailable
	}

	target := online.DeriveCapabilityTarget(identity)
	if target == "" {
		return nil, ErrOnlineQueryAccessSessionUnavailable
	}

	if !queryAccessOnlineCapabilityLinked(target) {
		return nil, ErrOnlineQueryAccessCapabilityUnsupported
	}

	return &OnlineQueryAccessSession{conn: conn, target: target}, nil
}

// AnalyzeOnlineQueryAccessWithSession analyzes a request through a unified
// online session. Validation is fixed in this order:
//
//  1. context and session usability;
//  2. non-empty request dialect against observed identity;
//  3. empty caller analysis profile;
//  4. absent caller schema resolver;
//  5. linked capability availability;
//  6. existing mode and analysis validation.
//
// An empty request dialect delegates dialect selection to the observed
// identity. A non-empty request dialect is only a constraint and must match
// exactly; it never selects or overrides the proof route.
func AnalyzeOnlineQueryAccessWithSession(
	ctx context.Context,
	session *OnlineQueryAccessSession,
	req QueryAccessRequest,
) (*QueryAccessResult, error) {
	if ctx == nil || session == nil || session.conn == nil {
		return nil, ErrOnlineQueryAccessSessionUnavailable
	}
	if req.Dialect != "" && req.Dialect != observedDialectFromTarget(session.target) {
		return nil, ErrOnlineQueryAccessDialectMismatch
	}
	if req.AnalysisProfile != QueryAccessAnalysisProfileEmpty {
		return nil, ErrOnlineQueryAccessProfileNotAllowed
	}
	if req.SchemaResolver != nil {
		return nil, ErrOnlineQueryAccessSchemaResolverNotAllowed
	}
	if !queryAccessOnlineCapabilityLinked(session.target) {
		return nil, ErrOnlineQueryAccessCapabilityUnsupported
	}

	switch session.target {
	case online.TargetMySQL57, online.TargetMySQL80, online.TargetMySQL84, online.TargetTiDB85:
		return analyzeMySQLTiDBOnline(
			ctx,
			session.conn,
			session.target,
			toDomainQADialect(observedDialectFromTarget(session.target)),
			req,
		)
	case online.TargetPG17:
		return analyzePostgreSQLOnline(ctx, session.conn, req)
	default:
		// Fail closed: no other capability is routed in this milestone.
		return nil, ErrOnlineQueryAccessCapabilityUnsupported
	}
}

// queryAccessOnlineCapabilityLinked is defined in query_access_online_capability.go
// as the single private routing definition shared by the constructor and the
// analysis entry.

// observedDialectFromTarget maps an identity-derived capability target to the
// public dialect of the observed server.
func observedDialectFromTarget(target online.CapabilityTarget) Dialect {
	switch target {
	case online.TargetMySQL57, online.TargetMySQL80, online.TargetMySQL84:
		return DialectMySQL
	case online.TargetTiDB85:
		return DialectTiDB
	case online.TargetPG17:
		return DialectPostgreSQL
	default:
		return ""
	}
}

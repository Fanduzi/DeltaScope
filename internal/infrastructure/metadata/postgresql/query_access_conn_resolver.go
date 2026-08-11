//go:build postgresql

// Package postgresqlmeta provides the conn-backed PostgreSQL query access adapter.
// input: caller-owned *sql.Conn, context, dialect, and schema-qualified relation name
// output: same-session RelationSchema or the existing PostgreSQL catalog resolution error
// pos: session-pinned ownership adapter delegating catalog behavior to the private core
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"context"
	"database/sql"
	"fmt"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
)

// QueryAccessConnResolver resolves relation metadata using a single *sql.Conn.
// It does not contain a *sql.DB field and cannot fall back to a connection pool.
// All catalog queries execute on the same caller-owned connection.
type QueryAccessConnResolver struct {
	conn *sql.Conn
}

// NewQueryAccessConnResolver builds a conn-backed query access resolver.
// The caller retains ownership of conn; the resolver does not close it.
func NewQueryAccessConnResolver(conn *sql.Conn) (*QueryAccessConnResolver, error) {
	if conn == nil {
		return nil, ErrSessionNotPinned
	}
	return &QueryAccessConnResolver{conn: conn}, nil
}

var _ appqa.SchemaResolver = (*QueryAccessConnResolver)(nil)

// ResolveRelation returns the schema for a relation by querying pg_catalog
// on the pinned *sql.Conn.
func (r *QueryAccessConnResolver) ResolveRelation(ctx context.Context, dialect string, schema, name string) (appqa.RelationSchema, error) {
	if err := ctx.Err(); err != nil {
		return appqa.RelationSchema{}, fmt.Errorf("resolve cancelled: %w", err)
	}
	if r == nil || r.conn == nil {
		return appqa.RelationSchema{}, ErrSessionClosed
	}
	return resolvePostgreSQLRelation(ctx, r.conn, schema, name)
}

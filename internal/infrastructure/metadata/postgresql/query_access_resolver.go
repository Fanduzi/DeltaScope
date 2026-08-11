//go:build postgresql

// Package postgresqlmeta provides the DB-backed PostgreSQL query access adapter.
// input: *sql.DB, context, dialect, and schema-qualified relation name
// output: RelationSchema or the existing PostgreSQL catalog resolution error
// pos: pool-backed ownership adapter delegating catalog behavior to the private core
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"context"
	"database/sql"
	"fmt"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
)

// QueryAccessResolver resolves relation metadata for query access analysis.
type QueryAccessResolver struct {
	db *sql.DB
}

// NewQueryAccessResolver builds a query access resolver on top of an existing SQL handle.
func NewQueryAccessResolver(db *sql.DB) *QueryAccessResolver {
	return &QueryAccessResolver{db: db}
}

var _ appqa.SchemaResolver = (*QueryAccessResolver)(nil)

// ResolveRelation returns the schema for a relation by querying pg_catalog.
func (r *QueryAccessResolver) ResolveRelation(ctx context.Context, dialect string, schema, name string) (appqa.RelationSchema, error) {
	if err := ctx.Err(); err != nil {
		return appqa.RelationSchema{}, fmt.Errorf("resolve cancelled: %w", err)
	}
	return resolvePostgreSQLRelation(ctx, r.db, schema, name)
}

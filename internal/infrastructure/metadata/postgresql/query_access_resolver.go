//go:build postgresql

// Package postgresqlmeta provides the SchemaResolver implementation for PostgreSQL.
// input: database/sql handle and relation metadata queries via pg_catalog
// output: RelationSchema with table/view kind and column listing ordered by attnum
// pos: infrastructure metadata adapter for query access resolution
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

	rs := appqa.RelationSchema{
		Schema: schema,
		Name:   name,
	}

	relkind, err := r.resolveRelkind(ctx, schema, name)
	if err != nil {
		return appqa.RelationSchema{}, err
	}
	if err := rejectUnsupportedRelkind(relkind, schema, name); err != nil {
		return appqa.RelationSchema{}, err
	}

	rs.Kind = relkindToKind(relkind)
	rs.IsView = relkind == "v" || relkind == "m"

	columns, err := r.resolveColumns(ctx, schema, name)
	if err != nil {
		return appqa.RelationSchema{}, err
	}
	rs.Columns = columns

	return rs, nil
}

func (r *QueryAccessResolver) resolveRelkind(ctx context.Context, schema, name string) (string, error) {
	var relkind string
	err := r.db.QueryRowContext(ctx, `
		select c.relkind
		from pg_catalog.pg_class c
		join pg_catalog.pg_namespace n on n.oid = c.relnamespace
		where n.nspname = $1 and c.relname = $2 and c.relkind in ('r','p','v','m','f')
	`, schema, name).Scan(&relkind)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("relation %s.%s not found", schema, name)
		}
		return "", fmt.Errorf("query relation type for %s.%s: %w", schema, name, err)
	}
	return relkind, nil
}

func (r *QueryAccessResolver) resolveColumns(ctx context.Context, schema, name string) ([]appqa.ColumnSchema, error) {
	rows, err := r.db.QueryContext(ctx, `
		select a.attname as column_name, a.attnum as ordinal_position
		from pg_catalog.pg_attribute a
		join pg_catalog.pg_class c on c.oid = a.attrelid
		join pg_catalog.pg_namespace n on n.oid = c.relnamespace
		where n.nspname = $1 and c.relname = $2 and a.attnum > 0 and not a.attisdropped
		order by a.attnum
	`, schema, name)
	if err != nil {
		return nil, fmt.Errorf("query columns for %s.%s: %w", schema, name, err)
	}
	defer rows.Close()

	var columns []appqa.ColumnSchema
	for rows.Next() {
		var col appqa.ColumnSchema
		if err := rows.Scan(&col.Name, &col.Ordinal); err != nil {
			return nil, fmt.Errorf("scan column for %s.%s: %w", schema, name, err)
		}
		columns = append(columns, col)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate columns for %s.%s: %w", schema, name, err)
	}
	return columns, nil
}

func relkindToKind(relkind string) string {
	switch relkind {
	case "v", "m":
		return "view"
	case "r", "p":
		return "table"
	default:
		// Foreign tables (relkind f) and any other kind must never reach here.
		return ""
	}
}

// rejectUnsupportedRelkind fail-closes relation kinds that must not be promoted
// to physical base tables for query access (notably foreign tables, relkind f).
// The error matches the missing-relation shape so callers stay fail-closed without
// expanding public result contracts or leaking catalog details.
func rejectUnsupportedRelkind(relkind, schema, name string) error {
	if relkind == "f" {
		return fmt.Errorf("relation %s.%s not found", schema, name)
	}
	return nil
}

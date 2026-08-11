//go:build postgresql

// Package postgresqlmeta provides a *sql.Conn-backed SchemaResolver for query access.
// input: a single caller-owned *sql.Conn (not a *sql.DB pool)
// output: RelationSchema with table/view kind and column listing via pg_catalog
// pos: infrastructure same-connection metadata resolver for trusted SDK path
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

func (r *QueryAccessConnResolver) resolveRelkind(ctx context.Context, schema, name string) (string, error) {
	var relkind string
	err := r.conn.QueryRowContext(ctx, `
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

func (r *QueryAccessConnResolver) resolveColumns(ctx context.Context, schema, name string) ([]appqa.ColumnSchema, error) {
	rows, err := r.conn.QueryContext(ctx, `
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

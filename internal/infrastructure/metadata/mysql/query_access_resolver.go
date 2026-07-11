// Package mysqlmeta provides the SchemaResolver implementation for MySQL/TiDB.
// input: database/sql handle and relation metadata queries
// output: RelationSchema with table/view kind and column listing ordered by ordinal_position
// pos: infrastructure metadata adapter for query access resolution
// note: if this file changes, update this header and module README.md.
package mysqlmeta

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

// ResolveRelation returns the schema for a relation by querying information_schema.
func (r *QueryAccessResolver) ResolveRelation(ctx context.Context, dialect string, schema, name string) (appqa.RelationSchema, error) {
	if err := ctx.Err(); err != nil {
		return appqa.RelationSchema{}, fmt.Errorf("resolve cancelled: %w", err)
	}

	rs := appqa.RelationSchema{
		Schema: schema,
		Name:   name,
	}

	tableType, err := r.resolveTableType(ctx, schema, name)
	if err != nil {
		return appqa.RelationSchema{}, err
	}

	rs.Kind = tableType
	rs.IsView = tableType == "view"

	columns, err := r.resolveColumns(ctx, schema, name)
	if err != nil {
		return appqa.RelationSchema{}, err
	}
	rs.Columns = columns

	return rs, nil
}

func (r *QueryAccessResolver) resolveTableType(ctx context.Context, schema, name string) (string, error) {
	var tableType string
	err := r.db.QueryRowContext(ctx, `
		select table_type
		from information_schema.tables
		where table_schema = ? and table_name = ?
	`, schema, name).Scan(&tableType)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("relation %s.%s not found", schema, name)
		}
		return "", fmt.Errorf("query relation type for %s.%s: %w", schema, name, err)
	}

	if tableType == "VIEW" {
		return "view", nil
	}
	return "table", nil
}

func (r *QueryAccessResolver) resolveColumns(ctx context.Context, schema, name string) ([]appqa.ColumnSchema, error) {
	rows, err := r.db.QueryContext(ctx, `
		select column_name, ordinal_position
		from information_schema.columns
		where table_schema = ? and table_name = ?
		order by ordinal_position
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

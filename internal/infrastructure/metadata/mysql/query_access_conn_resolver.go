// Package mysqlmeta provides the same-connection MySQL/TiDB query access resolver.
// input: caller-owned *sql.Conn and relation metadata lookups
// output: relation schemas bound to the supplied connection
// pos: internal metadata boundary for explicit query access sessions
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
)

// QueryAccessConnResolver resolves relation metadata through one *sql.Conn.
type QueryAccessConnResolver struct {
	conn *sql.Conn
}

// NewQueryAccessConnResolver creates a resolver bound to conn.
func NewQueryAccessConnResolver(conn *sql.Conn) (*QueryAccessConnResolver, error) {
	if conn == nil {
		return nil, errors.New("query access connection resolver: connection must not be nil")
	}
	return &QueryAccessConnResolver{conn: conn}, nil
}

var _ appqa.SchemaResolver = (*QueryAccessConnResolver)(nil)

// ResolveRelation returns relation metadata from the caller-owned connection.
func (r *QueryAccessConnResolver) ResolveRelation(ctx context.Context, dialect string, schema, name string) (appqa.RelationSchema, error) {
	if err := ctx.Err(); err != nil {
		return appqa.RelationSchema{}, fmt.Errorf("resolve cancelled: %w", err)
	}

	var tableType string
	err := r.conn.QueryRowContext(ctx, `
		select table_type
		from information_schema.tables
		where table_schema = ? and table_name = ?
	`, schema, name).Scan(&tableType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appqa.RelationSchema{}, fmt.Errorf("relation not found")
		}
		return appqa.RelationSchema{}, fmt.Errorf("relation metadata unavailable")
	}

	rows, err := r.conn.QueryContext(ctx, `
		select column_name, ordinal_position
		from information_schema.columns
		where table_schema = ? and table_name = ?
		order by ordinal_position
	`, schema, name)
	if err != nil {
		return appqa.RelationSchema{}, fmt.Errorf("column metadata unavailable")
	}
	defer rows.Close()

	columns := make([]appqa.ColumnSchema, 0)
	for rows.Next() {
		var column appqa.ColumnSchema
		if err := rows.Scan(&column.Name, &column.Ordinal); err != nil {
			return appqa.RelationSchema{}, fmt.Errorf("column metadata unavailable")
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return appqa.RelationSchema{}, fmt.Errorf("column metadata unavailable")
	}

	var kind string
	switch {
	case strings.EqualFold(tableType, "BASE TABLE"):
		kind = "table"
	case strings.EqualFold(tableType, "VIEW"):
		kind = "view"
	default:
		return appqa.RelationSchema{}, fmt.Errorf("unsupported relation type")
	}
	return appqa.RelationSchema{
		Schema:  schema,
		Name:    name,
		Kind:    kind,
		Columns: columns,
		IsView:  kind == "view",
	}, nil
}

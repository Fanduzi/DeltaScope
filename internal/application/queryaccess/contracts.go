// Package queryaccess defines application-level contracts for query access analysis.
// input: SQL text, dialect, mode, and optional schema resolver
// output: domain-typed query access results for transport adapters
// pos: application contract layer for the query access analysis foundation
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// SchemaResolver resolves relation metadata for name resolution.
type SchemaResolver interface {
	ResolveRelation(ctx context.Context, dialect string, schema, name string) (RelationSchema, error)
}

// RelationSchema contains metadata about a relation for resolution.
type RelationSchema struct {
	Schema  string
	Name    string
	Kind    string // "table" or "view"
	Columns []ColumnSchema
	IsView  bool
}

// ColumnSchema contains metadata about a column.
type ColumnSchema struct {
	Name    string
	Ordinal int
}

// QueryAccessRequest is the input for query access analysis.
type QueryAccessRequest struct {
	SQL            string
	Dialect        string
	Mode           string
	DefaultSchema  string
	SchemaResolver SchemaResolver // optional
}

// QueryAccessResult wraps the domain result for application-layer consumption.
type QueryAccessResult struct {
	DomainResult domain.Result
}

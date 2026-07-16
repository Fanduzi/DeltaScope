//go:build !postgresql

// Package postgresql provides the PostgreSQL query access stub when built without the postgresql tag.
// input: none (stub only)
// output: ErrPostgreSQLNotAvailable for all calls
// pos: infrastructure stub for non-PostgreSQL builds
// note: if this file changes, update this header and module README.md.
package postgresql

import (
	"context"
	"errors"
)

// ErrPostgreSQLNotAvailable indicates PostgreSQL support was not compiled in.
var ErrPostgreSQLNotAvailable = errors.New("postgresql support requires build tag: go build -tags postgresql")

// QueryAccessFacts is the intermediate result from PostgreSQL query access extraction.
type QueryAccessFacts struct {
	ReadClassification string
	Relations          []RelationFacts
	ColumnReferences   []ColumnRefFacts
	Outputs            []OutputFacts
	Unresolved         []UnresolvedFacts
}

// RelationFacts describes a relation reference extracted from the AST.
type RelationFacts struct {
	Schema string
	Name   string
	Alias  string
	Kind   string
}

// ColumnRefFacts describes a column reference with usage contexts.
type ColumnRefFacts struct {
	Schema  string
	Table   string
	Column  string
	Usages  []string
	QualRef string
}

// OutputFacts describes an output column with source lineage.
type OutputFacts struct {
	Name    string
	Sources []string
}

// UnresolvedFacts describes a reference that could not be resolved.
type UnresolvedFacts struct {
	Reference string
	Reason    string
}

// QueryAccessExtractor extracts query access facts from PostgreSQL AST.
type QueryAccessExtractor struct{}

// ExtractQueryAccess returns ErrPostgreSQLNotAvailable when built without the postgresql tag.
func (e *QueryAccessExtractor) ExtractQueryAccess(_ context.Context, _ string, _ string, _ string) (*QueryAccessFacts, error) {
	return nil, ErrPostgreSQLNotAvailable
}

// Package queryaccess defines application-level contracts for query access analysis.
// input: SQL text, dialect, mode, profile, and optional schema resolver
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
	// TypeOID is the catalog type OID when known (PostgreSQL atttypid).
	// Zero means unknown / not provided. Facts only — never a trust signal.
	// MySQL/TiDB resolvers may leave this zero. T6 does not implement catalog
	// population; T7 may fill it when needed for identity resolution.
	TypeOID uint32
}

// QueryAccessRequest is the input for query access analysis.
// Callers cannot inject effect candidates or trust bits: there is no candidate
// or Trusted field on the request.
//
// EffectIdentityResolver remains intentionally absent. Profile selection is
// compatibility metadata only and does not inject candidates or trust facts.
type QueryAccessRequest struct {
	SQL             string
	Dialect         string
	Mode            string
	DefaultSchema   string
	AnalysisProfile AnalysisProfile
	SchemaResolver  SchemaResolver // optional
}

// EffectCandidateKind mirrors parser-internal candidate kinds (application copy).
type EffectCandidateKind string

const (
	EffectCandidateOperator EffectCandidateKind = "operator"
	EffectCandidateFunction EffectCandidateKind = "function"
	EffectCandidateCast     EffectCandidateKind = "cast"
	EffectCandidateUnknown  EffectCandidateKind = "unknown"
)

// OperandColumnRef identifies a base-table column for operand type resolution.
// Schema may be empty for unqualified references (resolved via search_path).
type OperandColumnRef struct {
	Schema string
	Table  string
	Column string
}

// EffectCandidate is an internal, untrusted effect fact for future catalog
// identity resolution. It is NOT a trust root and is never serialized on
// domain.Result or SDK/CLI/HTTP JSON.
type EffectCandidate struct {
	Kind                      EffectCandidateKind
	Ordinal                   int
	NamePath                  []string
	OriginalNamePath          []string
	ExplicitSchema            bool
	IsQuoted                  bool
	Canonical                 bool
	Ambiguous                 bool
	ParserClassification      string
	UnqualifiedRelation       bool
	Arity                     int
	OperandKinds              []string
	IsAggregate               bool
	HasWindow                 bool
	HasFilter                 bool
	HasDistinct               bool               `json:"-"`
	HasAggOrder               bool               `json:"-"`
	HasWithinGroup            bool               `json:"-"`
	HasFrame                  bool               `json:"-"`
	HasNamedWindow            bool               `json:"-"`
	HasWindowPartition        bool               `json:"-"`
	HasWindowOrder            bool               `json:"-"`
	WindowPartitionKinds      []string           `json:"-"`
	WindowOrderKinds          []string           `json:"-"`
	WindowFrameKinds          []string           `json:"-"`
	WindowPartitionColumnRefs []OperandColumnRef `json:"-"`
	WindowOrderColumnRefs     []OperandColumnRef `json:"-"`
	TargetTypePath            []string
	// OperandColumnRefs maps operand position to base-table column reference.
	// Indexed by operand position; nil entries indicate non-column operands.
	// Only populated for column operands against base tables.
	OperandColumnRefs []OperandColumnRef
}

// QueryAccessResult wraps the domain result for application-layer consumption.
// EffectCandidates are internal-only (untrusted, non-public).
type QueryAccessResult struct {
	DomainResult                  domain.Result
	EffectCandidates              []EffectCandidate `json:"-"` // internal only; never public transport fields
	ExactCountIntegerOneStatement bool              `json:"-"`
}

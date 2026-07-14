// Package queryaccess defines transport-neutral domain types for query access analysis.
// input: SQL statement references, relation references, column references, and usage contexts
// output: pure domain models for query access requirements, read classification, and admission decisions
// pos: domain model for the query access analysis foundation shared across CLI, HTTP, and MCP surfaces
// note: if this file changes, update this header and module README.md.
package queryaccess

// Mode controls which column references become requirements.
type Mode string

const (
	// ModeStrict requires all referenced columns to be authorized.
	ModeStrict Mode = "strict"
	// ModeProjectionOnly requires only projected (SELECT-list) columns to be authorized.
	ModeProjectionOnly Mode = "projection_only"
)

// ReadClassification describes whether SQL is demonstrably read-only.
type ReadClassification string

const (
	// ReadOnly indicates the statement contains no write operations.
	ReadOnly ReadClassification = "read_only"
	// NotReadOnly indicates the statement contains at least one write operation.
	NotReadOnly ReadClassification = "not_read_only"
	// Indeterminate indicates the read-only status could not be determined.
	Indeterminate ReadClassification = "indeterminate"
)

// Admission describes whether SQL is eligible for caller authorization.
type Admission string

const (
	// Admissible indicates the statement is eligible for authorization checks.
	Admissible Admission = "admissible"
	// Rejected indicates the statement is not eligible for authorization checks.
	Rejected Admission = "rejected"
	// IndeterminateAdmission indicates the admission status could not be determined.
	IndeterminateAdmission Admission = "indeterminate"
)

// RelationKind describes the type of relation reference.
type RelationKind string

const (
	// RelationTable indicates a base table reference.
	RelationTable RelationKind = "table"
	// RelationView indicates a view reference.
	RelationView RelationKind = "view"
	// RelationCTE indicates a common table expression reference.
	RelationCTE RelationKind = "cte"
	// RelationDerived indicates a derived table (subquery) reference.
	RelationDerived RelationKind = "derived"
)

// UsageContext describes how a source column is used.
type UsageContext string

const (
	// UsageProjection indicates the column appears in the SELECT list.
	UsageProjection UsageContext = "projection"
	// UsageFilter indicates the column appears in a WHERE clause.
	UsageFilter UsageContext = "filter"
	// UsageJoin indicates the column appears in a JOIN condition.
	UsageJoin UsageContext = "join"
	// UsageGrouping indicates the column appears in a GROUP BY clause.
	UsageGrouping UsageContext = "grouping"
	// UsageHaving indicates the column appears in a HAVING clause.
	UsageHaving UsageContext = "having"
	// UsageOrdering indicates the column appears in an ORDER BY clause.
	UsageOrdering UsageContext = "ordering"
	// UsageWindow indicates the column appears in a window function.
	UsageWindow UsageContext = "window"
)

// ReasonCode is a bounded machine identifier for why something is indeterminate or rejected.
// Reason codes are stable machine identifiers only: never SQL text, object names,
// function/operator/cast spellings, OIDs, literals, credentials, or driver errors.
type ReasonCode string

const (
	// ReasonParseFailure indicates the statement could not be parsed.
	ReasonParseFailure ReasonCode = "parse_failure"
	// ReasonUnsupportedDialect indicates the dialect is not supported.
	ReasonUnsupportedDialect ReasonCode = "unsupported_dialect"
	// ReasonWriteOperation indicates a write operation was detected.
	ReasonWriteOperation ReasonCode = "write_operation"
	// ReasonMultiStatement indicates multiple statements were provided.
	ReasonMultiStatement ReasonCode = "multi_statement"
	// ReasonSchemaUnavailable indicates schema metadata was not available.
	ReasonSchemaUnavailable ReasonCode = "schema_unavailable"
	// ReasonAmbiguousReference indicates a reference could not be uniquely resolved.
	ReasonAmbiguousReference ReasonCode = "ambiguous_reference"
	// ReasonFunctionEffect indicates a function call with unknown side effects.
	// Used by MySQL/TiDB empty-allowlist path (legacy name).
	ReasonFunctionEffect ReasonCode = "unknown_function_effect"

	// ReasonUnprovenOperatorEffect indicates an operator expression was present
	// but its catalog identity was not proven trusted for pure-read admission.
	ReasonUnprovenOperatorEffect ReasonCode = "unproven_operator_effect"
	// ReasonUnprovenFunctionEffect indicates a function or aggregate call was
	// present but its catalog identity was not proven trusted.
	ReasonUnprovenFunctionEffect ReasonCode = "unproven_function_effect"
	// ReasonUnprovenCastEffect indicates a cast expression was present but its
	// cast path identity was not proven trusted.
	ReasonUnprovenCastEffect ReasonCode = "unproven_cast_effect"

	// ReasonIdentityResolverUnavailable indicates effect-identity resolution was
	// required but no identity resolver was configured.
	ReasonIdentityResolverUnavailable ReasonCode = "identity_resolver_unavailable"
	// ReasonIdentityUnknown indicates the resolver returned an unknown / no-match identity.
	ReasonIdentityUnknown ReasonCode = "identity_unknown"
	// ReasonIdentityLookupFailed indicates the resolver failed with a transport or catalog error.
	// Public results must never embed the underlying error text.
	ReasonIdentityLookupFailed ReasonCode = "identity_lookup_failed"
	// ReasonIdentityAmbiguous indicates multi-match identity resolution (non-unique).
	ReasonIdentityAmbiguous ReasonCode = "identity_ambiguous"
	// ReasonIdentityCoercionGap indicates type coercion required for unique identity
	// is outside the supported bounded resolution graph.
	ReasonIdentityCoercionGap ReasonCode = "identity_coercion_gap"

	// ReasonUnqualifiedRelationBlocked indicates that an unqualified relation
	// was present in a PostgreSQL query with a trusted bundle, which blocks
	// promotion to admissible due to search_path ambiguity.
	ReasonUnqualifiedRelationBlocked ReasonCode = "unqualified_relation_blocked"
)

// IdentityFailure is a bounded category for effect-identity resolution outcomes.
// Callers map transport/catalog errors to these categories before attaching a
// reason code; free-text error strings must never become reason codes.
type IdentityFailure string

const (
	// IdentityFailureUnavailable means no identity resolver was configured.
	IdentityFailureUnavailable IdentityFailure = "unavailable"
	// IdentityFailureUnknown means the resolver returned unknown / no rows.
	IdentityFailureUnknown IdentityFailure = "unknown"
	// IdentityFailureError means the resolver hit a transport or catalog error.
	IdentityFailureError IdentityFailure = "error"
	// IdentityFailureAmbiguous means multi-match non-unique identity.
	IdentityFailureAmbiguous IdentityFailure = "ambiguous"
	// IdentityFailureCoercionGap means required coercion is unsupported.
	IdentityFailureCoercionGap IdentityFailure = "coercion_gap"
)

// IdentityStatus is the bounded per-candidate outcome of effect-identity resolution.
// It extends IdentityFailure with a success value (resolved). Free-text strings
// are never valid statuses; unknown strings map to fail-closed via helpers.
//
// Naming aligns with public identity_* reason codes where possible:
// lookup_failed (status) ↔ IdentityFailureError ("error") ↔ identity_lookup_failed.
type IdentityStatus string

const (
	// IdentityStatusResolved means a unique catalog identity was established.
	// Facts may be present; this is NOT a trust claim.
	IdentityStatusResolved IdentityStatus = "resolved"
	// IdentityStatusUnknown means no matching catalog row / no unique identity.
	IdentityStatusUnknown IdentityStatus = "unknown"
	// IdentityStatusAmbiguous means multi-match non-unique identity.
	IdentityStatusAmbiguous IdentityStatus = "ambiguous"
	// IdentityStatusCoercionGap means required coercion is outside the bounded graph.
	IdentityStatusCoercionGap IdentityStatus = "coercion_gap"
	// IdentityStatusLookupFailed means transport/catalog error during lookup.
	IdentityStatusLookupFailed IdentityStatus = "lookup_failed"
	// IdentityStatusUnavailable means no identity resolver was configured or usable.
	IdentityStatusUnavailable IdentityStatus = "unavailable"
)

// WarningCode is a bounded machine identifier for warnings.
type WarningCode string

const (
	// WarningAmbiguousColumn indicates a column reference matched multiple relations.
	WarningAmbiguousColumn WarningCode = "ambiguous_column"
	// WarningMissingSchema indicates the default schema was not specified.
	WarningMissingSchema WarningCode = "missing_schema"
	// WarningDeprecatedSyntax indicates deprecated SQL syntax was encountered.
	WarningDeprecatedSyntax WarningCode = "deprecated_syntax"
	// WarningInferenceRisk indicates projection-only mode may leak data via non-projected columns.
	WarningInferenceRisk WarningCode = "projection_only_inference_risk"
)

// RelationReference represents a permission-bearing relation read by the query.
type RelationReference struct {
	Schema             string       `json:"schema,omitempty"`
	Name               string       `json:"name"`
	Alias              string       `json:"alias,omitempty"`
	Kind               RelationKind `json:"kind"`
	PermissionRequired bool         `json:"permission_required"`
}

// ColumnReference represents a source column reference with usage contexts.
type ColumnReference struct {
	Schema string         `json:"schema,omitempty"`
	Table  string         `json:"table"`
	Column string         `json:"column"`
	Usages []UsageContext `json:"usages"`
}

// OutputColumn represents a final output column with source lineage.
type OutputColumn struct {
	Name    string   `json:"name"`
	Sources []string `json:"sources"`
}

// Requirement represents a permission the caller must authorize.
type Requirement struct {
	Object    string `json:"object"`
	Privilege string `json:"privilege"`
}

// Unresolved represents a reference that could not be resolved.
type Unresolved struct {
	Reference string     `json:"reference"`
	Reason    ReasonCode `json:"reason"`
}

// Result is the complete query access analysis result.
type Result struct {
	Dialect            string              `json:"dialect"`
	Mode               Mode                `json:"mode"`
	ReadClassification ReadClassification  `json:"read_classification"`
	Admission          Admission           `json:"admission"`
	ReasonCodes        []ReasonCode        `json:"reason_codes,omitempty"`
	Relations          []RelationReference `json:"relations,omitempty"`
	ReferencedColumns  []ColumnReference   `json:"referenced_columns,omitempty"`
	Outputs            []OutputColumn      `json:"outputs,omitempty"`
	Requirements       []Requirement       `json:"requirements,omitempty"`
	Unresolved         []Unresolved        `json:"unresolved,omitempty"`
	Warnings           []WarningCode       `json:"warnings,omitempty"`
}

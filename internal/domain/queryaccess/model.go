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

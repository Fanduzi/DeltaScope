// Package spec defines normalized statement specifications for rule evaluation.
// input: DML facts extracted from parser-specific AST adapters
// output: parser-neutral DML specification components for rules
// pos: domain DML specification model under the unified Statement spec
// note: if this file changes, update this header and module README.md.
package spec

// DMLOperation identifies the normalized DML statement operation.
type DMLOperation string

const (
	DMLOperationUnknown DMLOperation = "unknown"
	DMLOperationInsert  DMLOperation = "insert"
	DMLOperationUpdate  DMLOperation = "update"
	DMLOperationDelete  DMLOperation = "delete"
)

// DML contains the structural metadata extracted from a DML statement.
type DML struct {
	Operation      DMLOperation    `json:"operation"`
	Tables         []Table         `json:"tables,omitempty"`
	HasWhere       bool            `json:"has_where"`
	HasLimit       bool            `json:"has_limit"`
	HasOrderBy     bool            `json:"has_order_by"`
	HasSubquery    bool            `json:"has_subquery"`
	HasJoin        bool            `json:"has_join"`
	HasJoinOn      bool            `json:"has_join_on"`
	InsertRows     int             `json:"insert_rows,omitempty"`
	IsReplace      bool            `json:"is_replace,omitempty"`
	IsInsertSelect bool            `json:"is_insert_select,omitempty"`
	HasOnDuplicate bool            `json:"has_on_duplicate,omitempty"`
	HasReturning   bool            `json:"has_returning,omitempty"`
	PredicateShape PredicateShape  `json:"predicate_shape,omitempty"`
	LookupColumns  []string        `json:"lookup_columns,omitempty"`
	MatchedKeyName string          `json:"matched_key_name,omitempty"`
	MatchedKeyKind IndexKind       `json:"matched_key_kind,omitempty"`
	IsSingleTable  bool            `json:"is_single_table,omitempty"`
	Impact         *ImpactEstimate `json:"impact,omitempty"`
}

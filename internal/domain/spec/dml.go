// Package spec defines normalized statement specifications for rule evaluation.
// input: DML facts extracted from parser-specific AST adapters
// output: parser-neutral DML specification components for rules
// pos: domain DML specification model under the unified Statement spec
// note: if this file changes, update this header and module README.md.
package spec

// DML contains the structural metadata extracted from a DML statement.
type DML struct {
	HasWhere     bool `json:"has_where"`
	HasLimit     bool `json:"has_limit"`
	HasOrderBy   bool `json:"has_order_by"`
	HasSubquery  bool `json:"has_subquery"`
	HasJoinOn    bool `json:"has_join_on"`
	InsertRows   int  `json:"insert_rows,omitempty"`
	IsReplace    bool `json:"is_replace,omitempty"`
	IsSelectInto bool `json:"is_select_into,omitempty"`
}

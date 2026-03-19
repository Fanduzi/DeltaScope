// Package spec defines normalized statement specifications for rule evaluation.
// input: DDL facts extracted from parser-specific AST adapters
// output: parser-neutral DDL specification components for rules
// pos: domain DDL specification model under the unified Statement spec
// note: if this file changes, update this header and module README.md.
package spec

// DDL contains the structural metadata extracted from a DDL statement.
type DDL struct {
	Table       *Table            `json:"table,omitempty"`
	Columns     []Column          `json:"columns,omitempty"`
	PrimaryKey  *Index            `json:"primary_key,omitempty"`
	Indexes     []Index           `json:"indexes,omitempty"`
	Constraints []Constraint      `json:"constraints,omitempty"`
	Alter       []Alter           `json:"alter,omitempty"`
	Options     map[string]string `json:"options,omitempty"`
}

// Table describes a table-level object.
type Table struct {
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`
}

// Column describes a table column.
type Column struct {
	Name    string `json:"name"`
	Type    string `json:"type,omitempty"`
	Comment string `json:"comment,omitempty"`
}

// Index describes an index declaration.
type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns,omitempty"`
}

// Constraint describes a non-index table constraint worth preserving for later rules.
type Constraint struct {
	Type    string   `json:"type"`
	Name    string   `json:"name,omitempty"`
	Columns []string `json:"columns,omitempty"`
}

// Alter describes a normalized alter action.
type Alter struct {
	Action string `json:"action"`
	Name   string `json:"name,omitempty"`
}

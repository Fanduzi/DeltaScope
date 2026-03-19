// Package spec defines normalized statement specifications for rule evaluation.
// input: statement data extracted from parser-specific AST adapters
// output: parser-neutral statement models for domain rule processing
// pos: domain specification model for all auditable SQL statements
// note: if this file changes, update this header and module README.md.
package spec

// Statement is the normalized domain input for rule evaluation.
type Statement struct {
	Kind          string   `json:"kind"`
	Dialect       string   `json:"dialect"`
	RawSQL        string   `json:"raw_sql"`
	NormalizedSQL string   `json:"normalized_sql,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	Line          int      `json:"line,omitempty"`
	Column        int      `json:"column,omitempty"`
	DDL           *DDL     `json:"ddl,omitempty"`
	DML           *DML     `json:"dml,omitempty"`
}

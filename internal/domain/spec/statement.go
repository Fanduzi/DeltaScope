// Package spec defines normalized statement specifications for rule evaluation.
// input: statement data extracted from parser-specific AST adapters
// output: parser-neutral statement models for domain rule processing
// pos: domain specification model for all auditable SQL statements
// note: if this file changes, update this header and module README.md.
package spec

// Kind identifies the normalized statement family.
type Kind string

const (
	KindUnknown Kind = "unknown"
	KindDDL     Kind = "ddl"
	KindDML     Kind = "dml"
)

// String returns the string form of the statement kind.
func (k Kind) String() string {
	return string(k)
}

// Dialect identifies the SQL dialect the statement belongs to.
type Dialect string

const (
	DialectUnknown    Dialect = "unknown"
	DialectMySQL      Dialect = "mysql"
	DialectTiDB       Dialect = "tidb"
	DialectPostgreSQL Dialect = "postgresql"
)

// String returns the string form of the dialect.
func (d Dialect) String() string {
	return string(d)
}

// StatementExtractor converts one parser-owned statement into the parser-neutral domain model.
type StatementExtractor interface {
	Extract(dialect Dialect, rawSQL string) (Statement, error)
}

// Statement is the normalized domain input for rule evaluation.
type Statement struct {
	Kind          Kind               `json:"kind"`
	Dialect       Dialect            `json:"dialect"`
	RawSQL        string             `json:"raw_sql"`
	NormalizedSQL string             `json:"normalized_sql,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
	Line          int                `json:"line,omitempty"`
	Column        int                `json:"column,omitempty"`
	Metadata      *Metadata          `json:"metadata,omitempty"`
	DDL           *DDL               `json:"ddl,omitempty"`
	DML           *DML               `json:"dml,omitempty"`
	Unsupported   *UnsupportedDetail `json:"unsupported,omitempty"`
}

// UnsupportedDetail captures one parser-recognized but unsupported statement or feature.
type UnsupportedDetail struct {
	Index    int            `json:"index,omitempty"`
	Feature  string         `json:"feature"`
	SQL      string         `json:"sql,omitempty"`
	Reason   string         `json:"reason"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Package spec defines normalized statement specifications for rule evaluation.
// input: DDL facts extracted from parser-specific AST adapters
// output: parser-neutral DDL specification components for rules
// pos: domain DDL specification model under the unified Statement spec
// note: if this file changes, update this header and module README.md.
package spec

// DDL contains the structural metadata extracted from a DDL statement.
type DDL struct {
	Table         *Table            `json:"table,omitempty"`
	Columns       []Column          `json:"columns,omitempty"`
	PrimaryKey    *Index            `json:"primary_key,omitempty"`
	Indexes       []Index           `json:"indexes,omitempty"`
	Constraints   []Constraint      `json:"constraints,omitempty"`
	Alter         []Alter           `json:"alter,omitempty"`
	Options       map[string]string `json:"options,omitempty"`
	HasReferTable bool              `json:"has_refer_table,omitempty"`
	HasSelect     bool              `json:"has_select,omitempty"`
	HasPartition  bool              `json:"has_partition,omitempty"`
}

// Table describes a table-level object.
type Table struct {
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`
}

// Column describes a table column.
type Column struct {
	Name                      string `json:"name"`
	Type                      string `json:"type,omitempty"`
	Length                    int    `json:"length,omitempty"`
	Comment                   string `json:"comment,omitempty"`
	Unsigned                  bool   `json:"unsigned,omitempty"`
	NotNull                   bool   `json:"not_null,omitempty"`
	AutoIncrement             bool   `json:"auto_increment,omitempty"`
	HasDefault                bool   `json:"has_default,omitempty"`
	DefaultValue              string `json:"default_value,omitempty"`
	DefaultIsNull             bool   `json:"default_is_null,omitempty"`
	DefaultIsCurrentTimestamp bool   `json:"default_is_current_timestamp,omitempty"`
	OnUpdateCurrentTimestamp  bool   `json:"on_update_current_timestamp,omitempty"`
}

// IndexKind identifies the semantic class of an index declaration.
type IndexKind string

// Supported index kinds.
const (
	IndexKindUnknown   IndexKind = "unknown"
	IndexKindPrimary   IndexKind = "primary"
	IndexKindSecondary IndexKind = "secondary"
	IndexKindUnique    IndexKind = "unique"
	IndexKindFulltext  IndexKind = "fulltext"
)

// Index describes an index declaration.
type Index struct {
	Name    string    `json:"name"`
	Kind    IndexKind `json:"kind,omitempty"`
	Columns []string  `json:"columns,omitempty"`
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

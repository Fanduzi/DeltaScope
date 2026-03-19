package spec

// DDL contains the structural metadata extracted from a DDL statement.
type DDL struct {
	Table   *Table            `json:"table,omitempty"`
	Columns []Column          `json:"columns,omitempty"`
	Indexes []Index           `json:"indexes,omitempty"`
	Alter   []Alter           `json:"alter,omitempty"`
	Options map[string]string `json:"options,omitempty"`
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

// Alter describes a normalized alter action.
type Alter struct {
	Action string `json:"action"`
	Name   string `json:"name,omitempty"`
}

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

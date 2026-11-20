package spec

import "strings"

// PGDestructiveTable stores private DROP TABLE and TRUNCATE facts from the AST.
type PGDestructiveTable struct {
	IfExists, Cascade, RestartIdentity bool
	Targets                            []PGDestructiveTableTarget
}

// PGDestructiveTableTarget preserves exact qualification and traversal scope.
type PGDestructiveTableTarget struct {
	Table            Table
	CatalogQualified bool
	Only             bool
}

// Qualified reports whether the target can use an exact schema-local observation.
func (t PGDestructiveTableTarget) Qualified() bool {
	schema := t.Table.Schema
	return !t.CatalogQualified && schema != "" && schema != "pg_temp" && t.Table.Name != "" &&
		!strings.HasPrefix(schema, "pg_temp_") && !strings.HasPrefix(schema, "pg_toast_temp_")
}

// PGDestructiveTableRelation binds private target facts to the observed schema.
type PGDestructiveTableRelation struct {
	Schema   string
	Relation PGRelationState
}

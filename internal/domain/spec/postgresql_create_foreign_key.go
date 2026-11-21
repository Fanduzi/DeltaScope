package spec

// PGCreateForeignKeys retains direct CREATE TABLE reference declarations in AST order.
type PGCreateForeignKeys struct {
	Target       PGDestructiveTableTarget
	IfNotExists  bool
	Unbound      bool
	Declarations []PGForeignKeySelection
}

// PGCreateForeignKeyReference binds the remote directory and candidate keys for CREATE TABLE.
type PGCreateForeignKeyReference struct {
	Target Table
	Table  *TableSnapshot
	Keys   *PGForeignKeyKeys
}

// CreateReferenceQualified reports whether CREATE TABLE can bind the reference outside temporary namespaces.
func (f PGForeignKeySelection) CreateReferenceQualified() bool {
	return f.Reference.Schema != "pg_toast_temp" && (PGDestructiveTableTarget{
		Table: f.Reference, CatalogQualified: f.ReferenceCatalogQualified,
	}).Qualified()
}

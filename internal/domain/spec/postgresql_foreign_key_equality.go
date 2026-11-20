package spec

import "slices"

// PGForeignKeyEqualityTarget binds ordered source and referenced column names.
// An empty referenced list selects the native primary key.
type PGForeignKeyEqualityTarget struct {
	Source, Reference          Table
	Columns, ReferencedColumns []string
}

func (t PGForeignKeyEqualityTarget) Matches(other PGForeignKeyEqualityTarget) bool {
	return t.Source.Schema == other.Source.Schema && t.Source.Name == other.Source.Name &&
		t.Reference.Schema == other.Reference.Schema && t.Reference.Name == other.Reference.Name &&
		slices.Equal(t.Columns, other.Columns) && slices.Equal(t.ReferencedColumns, other.ReferencedColumns)
}

// PGForeignKeyEquality records the first incompatible pair, numbered from one.
// Position zero proves the scoped catalog equality checks succeeded.
type PGForeignKeyEquality struct {
	Target               PGForeignKeyEqualityTarget
	IncompatiblePosition int
}

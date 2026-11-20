package spec

// PGForeignKeyKey holds the ordered key slots of a qualifying index shape.
// The provider has already checked the live, valid, unique and plain-index flags.
type PGForeignKeyKey struct {
	Columns   []string
	Primary   bool
	Immediate bool
}

// PGForeignKeyKeys binds a complete candidate directory to one exact table.
// An empty Keys slice means the query observed the table and found no candidates.
type PGForeignKeyKeys struct {
	Schema, Table string
	Keys          []PGForeignKeyKey
}

package spec

// PGConstraintUsingIndex observes one exact ALTER TABLE index selection.
// An empty Kind on a bound observation means no relation has that name.
type PGConstraintUsingIndex struct {
	Schema, Table, Name                     string
	Kind                                    string
	IndexKnown, OnTable                     bool
	Valid, Unique                           bool
	Expression, Partial, BTree              bool
	NullsNotDistinct                        bool
	ConstraintOwned                         bool
	KeyDefaults, KeyDefaultsKnown           bool
	OpclassesDefault, OpclassesDefaultKnown bool
}

// PGConstraintUsingIndexTarget returns the index name of one authored attachment.
func PGConstraintUsingIndexTarget(s Statement) string {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || s.DDL.Table == nil || len(s.DDL.Alter) != 1 {
		return ""
	}
	a := s.DDL.Alter[0]
	kind := a.Options["constraint_type"]
	if a.Action == "add_constraint" && a.Options["using_index"] == "true" && (kind == "primary_key" || kind == "unique") {
		return a.PGUsingIndexName
	}
	return ""
}

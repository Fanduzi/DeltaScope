package spec

// PGDropConstraintReference binds the direct foreign-key dependency observation
// for a primary key or unique constraint to its exact relation and name.
type PGDropConstraintReference struct {
	Schema, Table, Name, Kind string
	Blocked                   bool
}

// PGDropConstraintReferenceTarget returns the bound key eligible for the
// single-action RESTRICT dependency check.
func PGDropConstraintReferenceTarget(s Statement) *PGConstraintState {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || len(s.DDL.Alter) != 1 ||
		!PGOrdinaryTableBound(s) || !PG17Observed(s.Metadata.Instance) {
		return nil
	}
	a := s.DDL.Alter[0]
	if a.Action != "drop_constraint" || a.Options["cascade"] != "false" {
		return nil
	}
	member, known := s.Metadata.TargetTable.FindPGConstraint(a.Name)
	if !known || member == nil || member.Type != "p" && member.Type != "u" {
		return nil
	}
	return member
}

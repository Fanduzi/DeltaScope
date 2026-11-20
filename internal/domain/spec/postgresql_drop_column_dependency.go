package spec

// PGDropColumnDependency binds a direct RESTRICT blocker to one exact column.
// An empty Kind on a bound observation proves that the observed dependency
// classes did not supply a blocker.
type PGDropColumnDependency struct {
	Schema, Table, Column, Kind string
}

// PGDropColumnDependencyTarget returns the exact column for a live scoped read.
func PGDropColumnDependencyTarget(s Statement) string {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || len(s.DDL.Alter) != 1 || !PGOrdinaryTableBound(s) || !PG17Observed(s.Metadata.Instance) {
		return ""
	}
	action := s.DDL.Alter[0]
	if action.Action != "drop_column" || action.Options["cascade"] != "false" {
		return ""
	}
	member, known := s.Metadata.TargetTable.PGColumns.FindColumn(action.Name)
	if !known || member == nil || member.System {
		return ""
	}
	return action.Name
}

package spec

// PGDropColumnDependency binds a scoped dependency observation to one exact column.
// An empty Kind on a bound observation proves that the observed dependency
// classes did not supply a blocker.
type PGDropColumnDependency struct {
	Schema, Table, Column, Kind string
}

// PGColumnDependencyTarget returns the exact column for a live scoped read.
func PGColumnDependencyTarget(s Statement) string {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || len(s.DDL.Alter) != 1 || !PGOrdinaryTableBound(s) || !PG17Observed(s.Metadata.Instance) {
		return ""
	}
	action := s.DDL.Alter[0]
	switch action.Action {
	case "drop_column":
		if action.Options["cascade"] != "false" {
			return ""
		}
	case "set_data_type":
		if reason, known := PGColumnTypeSource(s); !known || reason != "" || !action.PGColumnType.Qualified() {
			return ""
		}
		if reason, fact := PGColumnTypeDestinationPremises(s); reason != "" || fact != "" {
			return ""
		}
	default:
		return ""
	}
	member, known := s.Metadata.TargetTable.PGColumns.FindColumn(action.Name)
	if !known || member == nil || member.System {
		return ""
	}
	return action.Name
}

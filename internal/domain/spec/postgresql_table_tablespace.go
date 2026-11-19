package spec

// PGTableTablespace observes one destination on one exact, scoped table.
// SameKnown distinguishes an unobserved current tablespace from a different one.
type PGTableTablespace struct {
	Schema, Table, Name string
	Exists, Global      bool
	Same, SameKnown     bool
}

// PGTableTablespaceTarget returns the exact single-action destination name.
func PGTableTablespaceTarget(s Statement) string {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || s.DDL.Table == nil || len(s.DDL.Alter) != 1 {
		return ""
	}
	a := s.DDL.Alter[0]
	if a.Action == "set_tablespace" {
		return a.Options["tablespace"]
	}
	return ""
}

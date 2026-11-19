package spec

// PGTableOwnerTarget retains the decoded owner selection from RoleSpec.
// Name is populated only for a literal role.
type PGTableOwnerTarget struct {
	Kind, Name string
}

// PGTableOwnerState records one target lookup in the observed session.
// A missing observation is represented by a nil state.
type PGTableOwnerState struct {
	Schema, Table string
	Target        PGTableOwnerTarget
	Exists, Same  bool
}

// PGTableOwnerSelection returns the modeled target of one OWNER TO action.
func PGTableOwnerSelection(s Statement) PGTableOwnerTarget {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || s.DDL.Table == nil || len(s.DDL.Alter) != 1 || s.DDL.Alter[0].Action != "change_owner" {
		return PGTableOwnerTarget{}
	}
	target := s.DDL.Alter[0].PGTableOwner
	switch target.Kind {
	case "named":
		if target.Name != "" {
			return target
		}
	case "current_user", "current_role", "session_user", "public":
		if target.Name == "" {
			return target
		}
	}
	return PGTableOwnerTarget{}
}

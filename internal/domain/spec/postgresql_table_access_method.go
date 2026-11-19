package spec

// PGTableAccessMethod binds one authored selection to the real method catalog.
// An empty Kind means the resolved method was absent.
type PGTableAccessMethod struct {
	Schema, Table, Name string
	Default             bool
	Kind                string
	Same, SameKnown     bool
}

// PGTableAccessMethodTarget preserves the distinction between bare DEFAULT
// and the exact method named "default".
func PGTableAccessMethodTarget(s Statement) (name string, useDefault bool) {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || s.DDL.Table == nil || len(s.DDL.Alter) != 1 {
		return "", false
	}
	a := s.DDL.Alter[0]
	if a.Action != "set_access_method" {
		return "", false
	}
	name = a.Options["access_method"]
	switch a.Options["is_default"] {
	case "true":
		if name == "default" {
			return name, true
		}
	case "false":
		return name, false
	}
	return "", false
}

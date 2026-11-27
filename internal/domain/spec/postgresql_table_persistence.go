package spec

// PGTablePersistence carries private PG17 scope and transition facts from one
// catalog observation. Current is either permanent (p) or unlogged (u).
type PGTablePersistence struct {
	Schema, Table, Current string
	Logged                 bool
	DirectPublication      bool
	ForeignKeyConflict     bool
	ForeignKeyUnknown      bool
	PartitionRoot          bool `json:"-"`
}

// PGTablePersistenceTarget identifies a single authored persistence action.
func PGTablePersistenceTarget(s Statement) string {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || s.DDL.Table == nil || len(s.DDL.Alter) != 1 {
		return ""
	}
	switch s.DDL.Alter[0].Action {
	case "set_logged", "set_unlogged":
		return s.DDL.Alter[0].Action
	}
	return ""
}

// Same reports whether the requested persistence already holds.
func (p PGTablePersistence) Same() bool {
	return p.Logged && p.Current == "p" || !p.Logged && p.Current == "u"
}

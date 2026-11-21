package spec

// PGTableNamespace records the bounded destination observation for RENAME TO
// or SET SCHEMA, including named companions moved with SET SCHEMA.
type PGTableNamespace struct {
	Schema, Table, Action, Destination string
	SourceKind                         string
	SourceKindKnown                    bool
	SchemaExists, SameSchema           bool
	RelationConflict, RowTypeConflict  bool
	RowTypeConflictKnown               bool
	ArrayTypeConflictKnown             bool
	ArrayTypeConflict                  bool
	IndexConflict                      bool
	OwnedSequenceConflict              bool
}

// PGTableNamespaceTarget reports the exact destination of one table action.
func PGTableNamespaceTarget(s Statement) (action, destination string) {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || s.DDL.Table == nil || len(s.DDL.Alter) != 1 {
		return "", ""
	}
	a := s.DDL.Alter[0]
	switch a.Action {
	case "rename_table":
		return a.Action, a.Options["new_name"]
	case "set_schema":
		return a.Action, a.Options["new_schema"]
	}
	return "", ""
}

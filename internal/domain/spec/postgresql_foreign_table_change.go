package spec

// PGForeignTableChange preserves a single supported foreign table action.
type PGForeignTableChange struct {
	Target                PGDestructiveTableTarget
	Action, Column        string
	Destination           string
	IfExists, ColumnGuard bool
	Owner                 PGTableOwnerTarget
	Options               []PGForeignOption
}

// PGForeignTableChangeState contains the catalog facts for an exact declaration.
type PGForeignTableChangeState struct {
	Target                      PGDestructiveTableTarget
	Action, Column, Destination string
	Owner                       PGTableOwnerTarget
	RelationKind                string
	ColumnExists, SystemColumn  bool
	DestinationSchema           bool
	DestinationOccupied         bool
	DestinationColumnExists     bool
	DestinationForbidden        bool
	OwnerExists, HasValidator   bool
	Options                     []string
}

// PGForeignTableChangeLookup requests catalog facts for one qualified table action.
func PGForeignTableChangeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterForeignTable || s.DDL.PGForeignTableChange == nil {
		return nil
	}
	d := s.DDL.PGForeignTableChange
	if !d.Target.Qualified() || d.Target.Table.Schema == "pg_toast_temp" {
		return nil
	}
	switch d.Action {
	case "alter_options":
	case "add_column", "drop_column":
		if d.Column == "" {
			return nil
		}
	case "rename_column":
		if d.Column == "" || d.Destination == "" {
			return nil
		}
	case "rename", "set_schema":
		if d.Destination == "" || d.Action == "set_schema" && d.Destination == "pg_temp" {
			return nil
		}
	case "set_owner":
		if !d.Owner.Valid() {
			return nil
		}
	default:
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_foreign_table_change", Schema: d.Target.Table.Schema, Name: d.Target.Table.Name,
		Qualifiers: map[string]string{"action": d.Action, "column": d.Column, "destination": d.Destination,
			"role_kind": d.Owner.Kind, "role_name": d.Owner.Name}}
}

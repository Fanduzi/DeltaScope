package spec

// PGTypeChange identifies a qualified type and its authored rename, schema move or owner.
type PGTypeChange struct {
	Source              PGDestructiveTableTarget
	Action, Destination string
	Owner               PGTableOwnerTarget
}

// PGTypeChangeState binds type eligibility and destination occupancy to a declaration.
type PGTypeChangeState struct {
	Target                                                  PGTypeChange
	SourceExists, Defined, Array, RowType, Composite        bool
	DestinationSchema, NamespaceIneligible                  bool
	DestinationType, DestinationRelation, CompanionOccupied bool
	OwnerExists                                             bool
}

// PGTypeChangeLookup requests the catalog facts for a literal PostgreSQL type change.
func PGTypeChangeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterType || s.DDL.PGTypeChange == nil {
		return nil
	}
	t := s.DDL.PGTypeChange
	if !t.Source.Qualified() || t.Source.Table.Schema == "pg_toast_temp" {
		return nil
	}
	switch t.Action {
	case "set_owner":
		if !t.Owner.Valid() {
			return nil
		}
	case "set_schema", "rename":
		if t.Destination == "" || t.Action == "set_schema" && t.Destination == "pg_temp" {
			return nil
		}
	default:
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_type_change", Schema: t.Source.Table.Schema, Name: t.Source.Table.Name,
		Qualifiers: map[string]string{"action": t.Action, "destination": t.Destination, "role_kind": t.Owner.Kind, "role_name": t.Owner.Name}}
}

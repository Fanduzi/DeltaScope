package spec

// PGOperatorFamilyChange retains the exact source and requested namespace or owner change.
type PGOperatorFamilyChange struct {
	Source      PGOperatorFamilyTarget
	Action      string
	Destination string
	Owner       PGTableOwnerTarget
}

// PGOperatorFamilyChangeState binds the catalog premises to one authored change.
type PGOperatorFamilyChangeState struct {
	Target               PGOperatorFamilyChange
	MethodKind           string
	SourceSchemaExists   bool
	SourceExists         bool
	DestinationExists    bool
	DestinationOccupied  bool
	SameNamespace        bool
	NamespaceUnsupported bool
	OwnerExists          bool
}

// PGOperatorFamilyChangeLookup binds an exact qualified family target.
func PGOperatorFamilyChangeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterOperatorFamily || s.DDL.PGOperatorFamilyChange == nil {
		return nil
	}
	t := *s.DDL.PGOperatorFamilyChange
	if !(PGDestructiveTableTarget{Table: Table{Schema: t.Source.Schema, Name: t.Source.Name}}).Qualified() || t.Source.Method == "" ||
		t.Action == "set_schema" && t.Destination == "pg_temp" {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_operator_family_change", Schema: t.Source.Schema, Name: t.Source.Name,
		Qualifiers: map[string]string{"method": t.Source.Method, "action": t.Action, "destination": t.Destination,
			"role_kind": t.Owner.Kind, "role_name": t.Owner.Name}}
}

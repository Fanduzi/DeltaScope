package spec

// PGCollationChangeTarget is one authored namespace or owner change.
type PGCollationChangeTarget struct {
	Source      PGColumnCollation
	Action      string
	Destination string
	Owner       PGTableOwnerTarget
}

// PGCollationChangeState contains only the catalog facts for this change.
type PGCollationChangeState struct {
	Target               PGCollationChangeTarget
	SourceKnown          bool
	SourceExists         bool
	DestinationExists    bool
	SameNamespace        bool
	NamespaceUnsupported bool
	DestinationOccupied  bool
	OwnerExists          bool
}

// PGCollationChangeLookup binds the exact source and authored change.
func PGCollationChangeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterCollation || s.DDL.PGCollationChange == nil {
		return nil
	}
	t := *s.DDL.PGCollationChange
	if !t.Source.Qualified() || t.Action == "set_schema" && t.Destination == "pg_temp" {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_collation_change", Schema: t.Source.Schema, Name: t.Source.Name, Qualifiers: map[string]string{
		"action": t.Action, "destination": t.Destination, "role_kind": t.Owner.Kind, "role_name": t.Owner.Name,
	}}
}

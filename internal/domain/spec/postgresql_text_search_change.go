package spec

// PGTextSearchChange preserves exact identities and the ordered mapping or option declaration.
type PGTextSearchChange struct {
	Target                    PGDestructiveTableTarget
	Kind, Action, Destination string
	Owner                     PGTableOwnerTarget
	Tokens                    []string
	Dictionaries              []Table
	Options                   []PGTextSearchDictionaryOption
	IfExists                  bool
}

// PGTextSearchDictionaryOption preserves absence separately from a known Boolean value.
type PGTextSearchDictionaryOption struct {
	Name                                string
	Present, BooleanKnown, BooleanValid bool
}

// PGTextSearchMapping records a built-in parser token and whether it has a mapping.
type PGTextSearchMapping struct {
	Token  string
	Mapped bool
}

// PGTextSearchChangeState binds catalog facts to the requested source and action.
type PGTextSearchChangeState struct {
	Target                                                       PGDestructiveTableTarget
	Kind, Action, Destination                                    string
	Owner                                                        PGTableOwnerTarget
	SourceSchema, SourceExists                                   bool
	DestinationSchema, DestinationOccupied, DestinationForbidden bool
	OwnerExists, BuiltinParser, SimpleTemplate                   bool
	Mappings                                                     []PGTextSearchMapping
	Dictionaries                                                 []Table
	DictionaryExists                                             []bool
}

// PGTextSearchChangeLookup requests exact PostgreSQL text search facts.
func PGTextSearchChangeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.PGTextSearchChange == nil {
		return nil
	}
	d := s.DDL.PGTextSearchChange
	if s.DDL.Operation != DDLOperationAlterTextSearchConfiguration && s.DDL.Operation != DDLOperationAlterTextSearchDictionary {
		return nil
	}
	if !d.Target.Qualified() || d.Target.Table.Schema == "pg_toast_temp" || d.Action == "set_schema" && d.Destination == "pg_temp" || d.Action == "set_owner" && !d.Owner.Valid() {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_text_search_change", Schema: d.Target.Table.Schema, Name: d.Target.Table.Name,
		PGTextSearchDictionaries: d.Dictionaries,
		Qualifiers: map[string]string{"kind": d.Kind, "action": d.Action, "destination": d.Destination,
			"role_kind": d.Owner.Kind, "role_name": d.Owner.Name}}
}

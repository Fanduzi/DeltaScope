package spec

// PGIndexOptionsState binds the observed kind and access method to an exact index.
type PGIndexOptionsState struct {
	Target PGDestructiveTableTarget
	Kind   string
	Method string
}

// PGIndexOptionsAction identifies one modeled ALTER INDEX SET or RESET action.
func PGIndexOptionsAction(s Statement) string {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterIndex || s.DDL.Table == nil || len(s.DDL.Alter) != 1 {
		return ""
	}
	a := s.DDL.Alter[0]
	if len(a.PGTableReloptions) > 0 && (a.Action == "set_options" || a.Action == "reset_options") {
		return a.Action
	}
	return ""
}

// PGIndexOptionsLookup requests the exact qualified index named by the action.
func PGIndexOptionsLookup(s Statement) *ObjectLookupRequest {
	if PGIndexOptionsAction(s) == "" {
		return nil
	}
	target := PGDestructiveTableTarget{Table: *s.DDL.Table, CatalogQualified: s.DDL.PGTableCatalogQualified}
	if !target.Qualified() {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_index_options", Schema: target.Table.Schema, Name: target.Table.Name}
}

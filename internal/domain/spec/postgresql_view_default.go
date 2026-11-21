package spec

// PGViewDefaultState binds the observed relation kind and user column to one exact view target.
type PGViewDefaultState struct {
	Schema, View, Column string
	Kind                 string
	ColumnExists         bool
}

// PGViewDefaultAction identifies one modeled ALTER VIEW column default action.
func PGViewDefaultAction(s Statement) string {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterView || s.DDL.Table == nil || len(s.DDL.Alter) != 1 {
		return ""
	}
	a := s.DDL.Alter[0]
	if a.Action == "set_default" || a.Action == "drop_default" {
		return a.Action
	}
	return ""
}

// PGViewDefaultLookup binds the exact qualified relation and column.
func PGViewDefaultLookup(s Statement) *ObjectLookupRequest {
	if PGViewDefaultAction(s) == "" || s.DDL.Alter[0].Name == "" {
		return nil
	}
	target := PGDestructiveTableTarget{Table: *s.DDL.Table, CatalogQualified: s.DDL.PGTableCatalogQualified}
	if !target.Qualified() {
		return nil
	}
	return &ObjectLookupRequest{Type: "view_column_default", Schema: target.Table.Schema, Name: target.Table.Name,
		Qualifiers: map[string]string{"column": s.DDL.Alter[0].Name}}
}

// MatchesViewDefault reports whether the catalog facts describe the requested identity.
func (s PGViewDefaultState) MatchesViewDefault(req *ObjectLookupRequest) bool {
	return s.Schema == req.Schema && s.View == req.Name && s.Column == req.Qualifiers["column"]
}

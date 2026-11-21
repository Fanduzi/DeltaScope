package spec

// PGMaterializedViewOptionsLookup identifies one storage option change on a qualified materialized view.
func PGMaterializedViewOptionsLookup(s Statement) *ObjectLookupRequest {
	if !PGMaterializedViewOptionsApplies(s) {
		return nil
	}
	target := PGDestructiveTableTarget{Table: *s.DDL.Table, CatalogQualified: s.DDL.PGTableCatalogQualified}
	if !target.Qualified() {
		return nil
	}
	return &ObjectLookupRequest{Type: "materialized_view_options", Schema: target.Table.Schema, Name: target.Table.Name}
}

// PGMaterializedViewOptionsApplies reports whether the statement models one SET or RESET action.
func PGMaterializedViewOptionsApplies(s Statement) bool {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterMaterializedView || s.DDL.Table == nil || len(s.DDL.Alter) != 1 {
		return false
	}
	a := s.DDL.Alter[0]
	return len(a.PGTableReloptions) > 0 && (a.Action == "set_options" || a.Action == "reset_options")
}

package spec

// PGViewOwnerSelection returns the decoded owner and expected relation kind for a view OWNER TO action.
func PGViewOwnerSelection(s Statement) (PGTableOwnerTarget, string) {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Table == nil || len(s.DDL.Alter) != 1 || s.DDL.Alter[0].Action != "change_owner" {
		return PGTableOwnerTarget{}, ""
	}
	var kind string
	switch s.DDL.Operation {
	case DDLOperationAlterView:
		kind = "v"
	case DDLOperationAlterMaterializedView:
		kind = "m"
	default:
		return PGTableOwnerTarget{}, ""
	}
	return s.DDL.Alter[0].PGTableOwner, kind
}

// PGViewOwnerLookup binds a modeled OWNER TO action to a qualified view.
func PGViewOwnerLookup(s Statement) *ObjectLookupRequest {
	owner, kind := PGViewOwnerSelection(s)
	if kind == "" || !owner.Valid() {
		return nil
	}
	target := PGDestructiveTableTarget{Table: *s.DDL.Table, CatalogQualified: s.DDL.PGTableCatalogQualified}
	if !target.Qualified() {
		return nil
	}
	return &ObjectLookupRequest{Type: "view_owner", Schema: target.Table.Schema, Name: owner.Name,
		Qualifiers: map[string]string{"table": target.Table.Name, "kind": owner.Kind, "relation_kind": kind}}
}

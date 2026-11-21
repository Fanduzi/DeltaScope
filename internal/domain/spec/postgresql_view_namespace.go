package spec

// PGViewNamespaceSource retains the exact identity and guard of a view namespace action.
type PGViewNamespaceSource struct {
	Schema, Name               string
	IfExists, CatalogQualified bool
}

// PGViewNamespaceLookup binds one authored ALTER VIEW or ALTER MATERIALIZED VIEW action.
func PGViewNamespaceLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.PGViewNamespaceSource == nil {
		return nil
	}
	var kind string
	switch s.DDL.Operation {
	case DDLOperationAlterView:
		kind = "v"
	case DDLOperationAlterMaterializedView:
		kind = "m"
	default:
		return nil
	}
	target := s.DDL.PGViewNamespaceSource
	if !(PGDestructiveTableTarget{Table: Table{Schema: target.Schema, Name: target.Name}, CatalogQualified: target.CatalogQualified}).Qualified() {
		return nil
	}
	action := s.DDL.Options["action"]
	var destination string
	switch action {
	case "rename", "rename_view":
		action, destination = "rename_table", s.DDL.Options["new_name"]
	case "set_schema":
		destination = s.DDL.Options["new_schema"]
		if !(PGDestructiveTableTarget{Table: Table{Schema: destination, Name: target.Name}}).Qualified() {
			return nil
		}
	}
	if destination == "" {
		return nil
	}
	return &ObjectLookupRequest{Schema: target.Schema, Name: destination, Type: "view_namespace",
		Qualifiers: map[string]string{"table": target.Name, "action": action, "kind": kind}}
}

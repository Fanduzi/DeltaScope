package spec

// PGMaterializedViewExtensionState binds the exact source kind and installed extension identity.
type PGMaterializedViewExtensionState struct {
	Schema, View, Extension string
	Kind                    string
	ExtensionExists         bool
}

// PGMaterializedViewExtensionLookup returns the exact source and extension of an attachment change.
func PGMaterializedViewExtensionLookup(s Statement) *ObjectLookupRequest {
	if !PGMaterializedViewExtensionApplies(s) {
		return nil
	}
	target := PGDestructiveTableTarget{Table: *s.DDL.Table, CatalogQualified: s.DDL.PGTableCatalogQualified}
	if !target.Qualified() {
		return nil
	}
	return &ObjectLookupRequest{Type: "materialized_view_extension", Schema: target.Table.Schema, Name: target.Table.Name,
		Qualifiers: map[string]string{"extension": s.DDL.PGMaterializedViewExtension}}
}

// PGMaterializedViewExtensionApplies reports whether this is a modeled dependency change.
func PGMaterializedViewExtensionApplies(s Statement) bool {
	return s.Dialect == DialectPostgreSQL && s.Kind == KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == DDLOperationAlterMaterializedView && s.DDL.Table != nil && s.DDL.PGMaterializedViewExtension != ""
}

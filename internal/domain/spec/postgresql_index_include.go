package spec

// PGIndexIncludeState binds the target relation kind and exact column names.
// Nil column lists mean the catalog did not establish the directory.
type PGIndexIncludeState struct {
	Target  PGDestructiveTableTarget
	Kind    string
	Columns []string
	System  []string
	Method  string
}

// PGIndexIncludeLookup requests the exact bound relation and authored access method.
func PGIndexIncludeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationCreateIndex || s.DDL.Table == nil || len(s.DDL.Indexes) != 1 || len(s.DDL.Indexes[0].IncludedColumns) == 0 {
		return nil
	}
	target := PGDestructiveTableTarget{Table: *s.DDL.Table, CatalogQualified: s.DDL.PGTableCatalogQualified}
	if !target.Qualified() {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_index_include", Schema: target.Table.Schema, Name: target.Table.Name,
		Qualifiers: map[string]string{"method": s.DDL.Indexes[0].AccessMethod}}
}

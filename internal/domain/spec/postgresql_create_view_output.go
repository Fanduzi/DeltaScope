package spec

import "strings"

// PGViewOutputColumn describes the part of an output column that PostgreSQL
// checks when replacing a view. Type and collation identifiers stay private.
type PGViewOutputColumn struct {
	Name      string
	TypeOID   int64
	Typmod    int32
	Collation int64
}

// PGCreateViewOutput retains a fully determined literal SELECT output or an
// explicitly unknown output when parse analysis is required.
type PGCreateViewOutput struct {
	Columns    []PGViewOutputColumn // nil means the raw tree cannot prove the shape
	AliasCount int
	Temporary  bool
	Unlogged   bool
}

// PGCreateViewTarget binds the existing relation and ordered user attributes
// in the target namespace. An empty Kind proves no relation was found.
type PGCreateViewTarget struct {
	Table        Table
	SchemaExists bool
	Kind         string
	Columns      []PGViewOutputColumn // nil means output was unobserved
}

// PGCreateViewTargetLookup requests an exact qualified creation target outside session schemas.
func PGCreateViewTargetLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.Operation != DDLOperationCreateView ||
		s.DDL.Table == nil || s.DDL.PGCreateViewOutput == nil || s.DDL.PGCreateViewOutput.Unlogged {
		return nil
	}
	target := PGDestructiveTableTarget{Table: *s.DDL.Table, CatalogQualified: s.DDL.PGTableCatalogQualified}
	if !target.Qualified() || target.Table.Schema == "pg_temp" || strings.HasPrefix(target.Table.Schema, "pg_temp_") || strings.HasPrefix(target.Table.Schema, "pg_toast_temp_") {
		return nil
	}
	return &ObjectLookupRequest{Type: "create_view_output", Schema: target.Table.Schema, Name: target.Table.Name}
}

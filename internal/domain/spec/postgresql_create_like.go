package spec

// PGCreateLikeSource retains the exact source of one PostgreSQL LIKE clause.
type PGCreateLikeSource struct {
	Target  PGDestructiveTableTarget
	Options uint32
}

// PGCreateSources records destination facts and the ordered source directory.
type PGCreateSources struct {
	Target       PGDestructiveTableTarget
	SchemaExists bool
	Exists       bool
	Sources      []PGCreateSource
}

// PGCreateSource keeps the source identity, kind and copied column names private.
type PGCreateSource struct {
	Target    PGDestructiveTableTarget
	Kind      string
	Partition *bool
	Columns   []string
}

// PGCreateLikeLookup requests the destination and sources of a qualified CREATE TABLE LIKE.
func PGCreateLikeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationCreateTable || s.DDL.Table == nil || len(s.DDL.PGCreateLike) == 0 {
		return nil
	}
	target := PGDestructiveTableTarget{Table: *s.DDL.Table, CatalogQualified: s.DDL.PGTableCatalogQualified}
	if !target.Qualified() || s.DDL.PGCreateLikeUnbound {
		return nil
	}
	sources := make([]PGDestructiveTableTarget, len(s.DDL.PGCreateLike))
	for i, source := range s.DDL.PGCreateLike {
		sources[i] = source.Target
	}
	return &ObjectLookupRequest{Type: "pg_create_like", Schema: target.Table.Schema, Name: target.Table.Name, PGDestructiveTargets: sources}
}

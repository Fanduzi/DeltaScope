package spec

// PGCollationRefreshState binds an exact collation identity in the database encoding.
type PGCollationRefreshState struct {
	Target              PGColumnCollation
	Known               bool
	Exists              bool
	Default             bool
	VersionKnown        bool
	VersionIncompatible bool
}

// PGCollationRefreshLookup returns the qualified target of ALTER COLLATION REFRESH VERSION.
func PGCollationRefreshLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterCollation || s.DDL.PGCollationRefresh == nil {
		return nil
	}
	target := *s.DDL.PGCollationRefresh
	if !target.Qualified() {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_collation_refresh", Schema: target.Schema, Name: target.Name}
}

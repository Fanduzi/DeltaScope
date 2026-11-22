package spec

// PGCheckQualifiedReference records local database and possible whole-row fallback facts.
type PGCheckQualifiedReference struct {
	Database         string
	Member           string
	FallbackPossible bool
}

// PGCheckQualifiedReferenceLookup requests only facts needed for the simple null test.
func PGCheckQualifiedReferenceLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || len(s.DDL.Alter) != 1 || !PGOrdinaryTableBound(s) {
		return nil
	}
	a := s.DDL.Alter[0]
	qualifiers := a.PGCheckNullTest
	member := ""
	if !a.PGCheckNullTestWholeRow && len(qualifiers) != 0 {
		member = qualifiers[len(qualifiers)-1]
		qualifiers = qualifiers[:len(qualifiers)-1]
	}
	if len(qualifiers) == 3 {
		return &ObjectLookupRequest{Type: "pg_check_qualified_reference", Name: member}
	}
	if member == "" || len(qualifiers) == 0 || qualifiers[len(qualifiers)-1] != s.DDL.Table.Name ||
		len(qualifiers) == 2 && qualifiers[0] != s.DDL.Table.Schema {
		return nil
	}
	if column, known := s.Metadata.TargetTable.PGColumns.FindColumn(member); known && column == nil {
		return &ObjectLookupRequest{Type: "pg_check_qualified_reference", Name: member}
	}
	return nil
}

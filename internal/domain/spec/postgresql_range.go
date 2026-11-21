package spec

// PGRangeDeclaration retains exact range subtype and option declarations.
type PGRangeDeclaration struct {
	Target  PGDestructiveTableTarget
	Subtype PGColumnTypeTarget
	Options []string
}

// PGRangeSubtypeLookup requests a type observation for a qualified range declaration.
func PGRangeSubtypeLookup(s Statement) *ObjectLookupRequest {
	if PGTypeDeclarationLookup(s) == nil || s.DDL.PGRange == nil {
		return nil
	}
	t := s.DDL.PGRange.Subtype
	if !t.Qualified() || t.Schema == "pg_toast_temp" || t.Array {
		return nil
	}
	return &ObjectLookupRequest{Type: "column_type_destination", Schema: t.Schema, Name: t.Name}
}

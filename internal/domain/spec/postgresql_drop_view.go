package spec

// PGDropViewState binds the observed kind and rewrite dependency to a DROP VIEW target.
// A blank kind on a bound state means the relation is absent.
type PGDropViewState struct {
	Target          PGDestructiveTableTarget
	Kind            string
	DependentView   bool
	DependentColumn bool
	CheckOption     bool
	Heap            bool
	HasToast        bool
}

// PGDropViewLookup returns the exact qualified target of a single DROP VIEW.
func PGDropViewLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationDropView || s.DDL.PGDropViewTarget == nil {
		return nil
	}
	target := s.DDL.PGDropViewTarget
	if !target.Qualified() {
		return nil
	}
	return &ObjectLookupRequest{Type: "drop_view_target", Schema: target.Table.Schema, Name: target.Table.Name}
}

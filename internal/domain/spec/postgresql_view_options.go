package spec

// PGViewOption stores the exact declaration name and its bounded value assessment.
type PGViewOption struct {
	Name, Namespace string
	ValueKnown      bool
	ValueValid      bool
	HasValue        bool
}

// PGAlterViewOptionsAction identifies a single supported ALTER VIEW option action.
func PGAlterViewOptionsAction(s Statement) string {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterView || s.DDL.Table == nil || len(s.DDL.Alter) != 1 || len(s.DDL.PGViewOptions) == 0 {
		return ""
	}
	action := s.DDL.Alter[0].Action
	if action == "set_options" || action == "reset_options" {
		return action
	}
	return ""
}

// PGAlterViewOptionsLookup preserves the exact qualified identity of an option target.
func PGAlterViewOptionsLookup(s Statement) *ObjectLookupRequest {
	if PGAlterViewOptionsAction(s) == "" {
		return nil
	}
	target := PGDestructiveTableTarget{Table: *s.DDL.Table, CatalogQualified: s.DDL.PGTableCatalogQualified}
	if !target.Qualified() {
		return nil
	}
	return &ObjectLookupRequest{Type: "alter_view_options", Schema: target.Table.Schema, Name: target.Table.Name}
}

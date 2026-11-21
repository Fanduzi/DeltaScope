package spec

// PGCreateInheritance preserves every authored INHERITS parent in order.
type PGCreateInheritance struct {
	Target      PGDestructiveTableTarget
	Parents     []PGDestructiveTableTarget
	IfNotExists bool
	Unbound     bool
}

// PGCreateInheritanceLookup requests the exact source relation directory for INHERITS.
func PGCreateInheritanceLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationCreateTable || s.DDL.Table == nil || s.DDL.PGCreateInheritance == nil {
		return nil
	}
	decl := s.DDL.PGCreateInheritance
	if decl.Unbound || !decl.Target.Qualified() || len(decl.Parents) == 0 {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_create_inheritance", Schema: decl.Target.Table.Schema,
		Name: decl.Target.Table.Name, PGDestructiveTargets: decl.Parents}
}

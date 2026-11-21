package spec

// PGOperatorFamilyTarget is the exact authored namespace, family and index method.
type PGOperatorFamilyTarget struct {
	Schema, Name, Method string
}

// PGOperatorFamilyState carries the observed catalog premises for creation.
type PGOperatorFamilyState struct {
	Target       PGOperatorFamilyTarget
	SchemaExists bool
	MethodKind   string
	Exists       bool
}

// PGOperatorFamilyLookup binds a qualified CREATE OPERATOR FAMILY declaration.
func PGOperatorFamilyLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationCreateOperatorFamily || s.DDL.PGOperatorFamily == nil {
		return nil
	}
	target := s.DDL.PGOperatorFamily
	if !(PGDestructiveTableTarget{Table: Table{Schema: target.Schema, Name: target.Name}}).Qualified() || target.Method == "" {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_operator_family", Schema: target.Schema, Name: target.Name,
		Qualifiers: map[string]string{"method": target.Method}}
}

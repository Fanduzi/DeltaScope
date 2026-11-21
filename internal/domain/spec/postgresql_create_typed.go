package spec

// PGCreateTyped retains the exact destination and source identities of CREATE TABLE OF.
type PGCreateTyped struct {
	Target      PGDestructiveTableTarget
	Type        PGDestructiveTableTarget
	Temporary   bool
	IfNotExists bool
}

// PGCreateTypedState records the destination and standalone composite type facts.
type PGCreateTypedState struct {
	Target, Type                    PGDestructiveTableTarget
	TargetSchema, TargetExists      bool
	TypeSchema, TypeExists, Defined bool
	Composite                       bool
	Columns                         []string
}

// PGCreateTypedLookup returns a request for an exact persistent destination and type.
func PGCreateTypedLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationCreateTable || s.DDL.PGCreateTyped == nil {
		return nil
	}
	typed := s.DDL.PGCreateTyped
	if typed.Temporary || !typed.Target.Qualified() || !typed.Type.Qualified() {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_create_typed", Schema: typed.Target.Table.Schema, Name: typed.Target.Table.Name,
		Qualifiers: map[string]string{"type_schema": typed.Type.Table.Schema, "type_name": typed.Type.Table.Name}}
}

package spec

// PGCreateEnum retains the exact type target and all authored labels.
type PGCreateEnum struct {
	Target PGDestructiveTableTarget
	Labels []string
}

// PGEnumState records exact type and label facts for a single enum declaration.
type PGEnumState struct {
	Target                    PGDestructiveTableTarget
	SchemaExists              bool
	NameOccupied              bool
	Encoding                  string
	LabelBytes                int
	TypeKind                  string
	TypeExists, Defined       bool
	Storage                   string
	VariableLength, TrueArray bool
	Labels                    []string
}

// PGAlterEnum retains the exact label values and distinguishes absent fields from empty labels.
type PGAlterEnum struct {
	Target             PGDestructiveTableTarget
	Known, IfNotExists bool
	Value              string
	Old, Neighbor      *string
}

// PGTypeDeclarationLookup returns the exact PostgreSQL enum creation or alteration target.
func PGTypeDeclarationLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil {
		return nil
	}
	var target PGDestructiveTableTarget
	var kind string
	switch {
	case s.DDL.PGBaseType != nil && (s.DDL.Operation == DDLOperationCreateType || s.DDL.Operation == DDLOperationAlterType):
		target, kind = s.DDL.PGBaseType.Target, "pg_base_type"
	case s.DDL.Operation == DDLOperationCreateType && s.DDL.PGRange != nil:
		target, kind = s.DDL.PGRange.Target, "pg_create_range"
	case s.DDL.Operation == DDLOperationCreateType && s.DDL.PGCreateEnum != nil:
		target, kind = s.DDL.PGCreateEnum.Target, "pg_create_enum"
	case s.DDL.Operation == DDLOperationAlterType && s.DDL.PGAlterEnum != nil && s.DDL.PGAlterEnum.Known:
		target, kind = s.DDL.PGAlterEnum.Target, "pg_alter_enum"
	}
	if !target.Qualified() || target.Table.Schema == "pg_toast_temp" {
		return nil
	}
	return &ObjectLookupRequest{Type: kind, Schema: target.Table.Schema, Name: target.Table.Name}
}

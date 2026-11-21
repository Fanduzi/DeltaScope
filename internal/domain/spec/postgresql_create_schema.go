package spec

// PGCreateSchemaTarget retains the authored name and optional owner.
// A blank name with an explicit owner means PostgreSQL derives the name from that role.
type PGCreateSchemaTarget struct {
	Name  string
	Owner PGTableOwnerTarget
}

// PGCreateSchemaState binds exact catalog premises to the authored declaration.
type PGCreateSchemaState struct {
	Target                   PGCreateSchemaTarget
	OwnerExists              bool
	NameKnown                bool
	Exists                   bool
	Reserved                 bool
	SystemSchemaAllowed      bool
	SystemSchemaSettingKnown bool
}

// PGCreateSchemaLookup requests the exact namespace and, when authored, owner fact.
func PGCreateSchemaLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationCreateSchema || s.DDL.PGCreateSchema == nil {
		return nil
	}
	t := *s.DDL.PGCreateSchema
	if t.Owner != (PGTableOwnerTarget{}) && !t.Owner.Valid() || t.Name == "" && !t.Owner.Valid() {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_create_schema", Name: t.Name, Qualifiers: map[string]string{
		"role_kind": t.Owner.Kind, "role_name": t.Owner.Name,
	}}
}

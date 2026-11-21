package spec

// PGGrantDeclaration preserves bounded PostgreSQL GRANT and REVOKE operands.
type PGGrantDeclaration struct {
	ObjectKind           string
	Grant, GrantOption   bool
	Cascade, AllInSchema bool
	Targets              []PGDestructiveTableTarget
	Privileges           []PGGrantPrivilege
	Grantees             []PGTableOwnerTarget
	Grantor              PGTableOwnerTarget
}

// PGGrantPrivilege keeps a privilege and its authored column order.
type PGGrantPrivilege struct {
	Name    string
	Columns []string
}

// PGGrantTargetState records exact object and column facts without ACL entries.
type PGGrantTargetState struct {
	Target  PGDestructiveTableTarget
	Exists  bool
	Kind    string
	Columns []string
}

// PGGrantState binds the declaration to an observed database catalog.
type PGGrantState struct {
	ObjectKind, Database string
	AllInSchema          bool
	Targets              []PGGrantTargetState
	Grantees             []PGTableOwnerTarget
	GranteeExists        []bool
	Grantor              PGTableOwnerTarget
	GrantorExists        bool
	GrantorCurrent       bool
}

// PGGrantLookup requests object and role identity facts only.
func PGGrantLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.PGGrant == nil {
		return nil
	}
	d := s.DDL.PGGrant
	if len(d.Targets) == 0 || len(d.Grantees) == 0 {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_grant_declaration", Name: d.ObjectKind, PGGrant: d}
}

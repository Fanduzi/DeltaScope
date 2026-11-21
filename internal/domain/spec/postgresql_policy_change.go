package spec

// PGPolicyChange retains a policy identity, authored role order and bounded predicate facts.
type PGPolicyChange struct {
	Table     PGDestructiveTableTarget
	Name      string
	RenameTo  string
	Roles     []PGTableOwnerTarget
	Using     PGPolicyPredicate
	WithCheck PGPolicyPredicate
}

// PGPolicyPredicate preserves clause presence and exact single-column references.
type PGPolicyPredicate struct {
	Present       bool
	Column        string
	BooleanResult bool
}

// PGPolicyChangeState binds an exact policy to its command and table catalog.
type PGPolicyChangeState struct {
	Table               Table
	Name, RenameTo      string
	RelationKind        string
	PolicyCommand       string
	DestinationOccupied bool
	MissingRole         bool
	NamedRoles          []string
	Columns             []string
}

// NamedPolicyRoles returns roles that PostgreSQL resolves before PUBLIC.
func (p *PGPolicyChange) NamedPolicyRoles() []string {
	var roles []string
	for _, role := range p.Roles {
		if role.Kind == "public" {
			break
		}
		if role.Kind == "named" {
			roles = append(roles, role.Name)
		}
	}
	return roles
}

// PGPolicyChangeLookup requests fresh facts about a qualified table and policy.
func PGPolicyChangeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterPolicy || s.DDL.PGPolicyChange == nil {
		return nil
	}
	p := s.DDL.PGPolicyChange
	if p.Name == "" || !p.Table.Qualified() || p.Table.Table.Schema == "pg_toast_temp" {
		return nil
	}
	for _, role := range p.Roles {
		if !role.Valid() {
			return nil
		}
	}
	return &ObjectLookupRequest{Type: "pg_policy_change", Schema: p.Table.Table.Schema, Name: p.Table.Table.Name,
		Qualifiers: map[string]string{"policy": p.Name, "rename_to": p.RenameTo}, PGPolicyRoles: p.NamedPolicyRoles()}
}

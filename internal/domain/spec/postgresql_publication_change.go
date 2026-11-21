package spec

// PGPublicationChange preserves publication actions and authored targets.
type PGPublicationChange struct {
	Name, Action, RenameTo string
	Owner                  PGTableOwnerTarget
	Members                []PGPublicationMember
	Options                []PGReplicationOption
}

// PGPublicationMember is one authored table or schema selector.
type PGPublicationMember struct {
	Table                    PGDestructiveTableTarget
	Kind, Schema             string
	HasColumns, HasRowFilter bool
}

// PGPublicationMemberState contains exact membership and eligibility facts.
type PGPublicationMemberState struct {
	Target                             PGPublicationMember
	Exists, Member, Eligible, Inherits bool
}

// PGPublicationChangeState binds current-database publication facts.
type PGPublicationChangeState struct {
	Name, RenameTo                                  string
	Owner                                           PGTableOwnerTarget
	Exists, AllTables, HasColumns, HasPartitionSpec bool
	DestinationOccupied, OwnerExists                bool
	Members                                         []PGPublicationMemberState
}

// PGPublicationChangeLookup requests exact local publication facts.
func PGPublicationChangeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.PGPublicationChange == nil {
		return nil
	}
	d := s.DDL.PGPublicationChange
	if d.Name == "" || d.Owner.Kind != "" && !d.Owner.Valid() {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_publication_change", Name: d.Name, PGPublicationMembers: d.Members,
		Qualifiers: map[string]string{"rename_to": d.RenameTo, "role_kind": d.Owner.Kind, "role_name": d.Owner.Name}}
}

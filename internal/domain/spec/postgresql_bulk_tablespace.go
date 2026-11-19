package spec

import "slices"

// PGBulkTablespaceTarget retains the decoded endpoints and ordered role filter.
type PGBulkTablespaceTarget struct {
	Source, Destination string
	Roles               []PGTableOwnerTarget
	Nowait              bool
}

// Matches binds one observation to the exact authored request.
func (t PGBulkTablespaceTarget) Matches(other PGBulkTablespaceTarget) bool {
	return t.Source == other.Source && t.Destination == other.Destination &&
		t.Nowait == other.Nowait && slices.Equal(t.Roles, other.Roles)
}

// PGBulkTablespaceState observes only the named endpoints and ordered roles.
type PGBulkTablespaceState struct {
	Target                          PGBulkTablespaceTarget
	SourceExists, DestinationExists bool
	SourceGlobal, DestinationGlobal bool
	Same                            bool
	RoleExists                      []bool
}

// PGBulkTablespaceSelection returns the modeled PostgreSQL table bulk move.
func PGBulkTablespaceSelection(s Statement) *PGBulkTablespaceTarget {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || s.DDL.Table != nil || len(s.DDL.Alter) != 1 || s.DDL.Alter[0].Action != "move_all_tablespace" {
		return nil
	}
	t := s.DDL.Alter[0].PGBulkTablespace
	if t == nil || t.Source == "" || t.Destination == "" {
		return nil
	}
	for _, role := range t.Roles {
		if !role.Valid() {
			return nil
		}
	}
	return t
}

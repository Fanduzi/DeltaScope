package spec

import "slices"

// PGDropViewSetState observes one ordered, fully qualified DROP VIEW list.
// A zero position marks the absence of a covered refusal.
type PGDropViewSetState struct {
	Targets  []PGDestructiveTableTarget
	IfExists bool
	Cascade  bool
	Position int
	Reason   string
}

// Matches verifies that the observed refusal belongs to the exact statement.
func (state PGDropViewSetState) Matches(targets []PGDestructiveTableTarget, ifExists, cascade bool) bool {
	return len(targets) > 1 && slices.Equal(state.Targets, targets) && state.IfExists == ifExists && state.Cascade == cascade &&
		state.Position >= 0 && state.Position <= len(targets) && (state.Position == 0) == (state.Reason == "")
}

// PGDropViewSetLookup binds one group of authored DROP VIEW targets.
func PGDropViewSetLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationDropView || len(s.DDL.PGDropViewTargets) < 2 {
		return nil
	}
	for _, target := range s.DDL.PGDropViewTargets {
		if !target.Qualified() {
			return nil
		}
	}
	return &ObjectLookupRequest{Type: "drop_view_set", PGDestructiveTargets: slices.Clone(s.DDL.PGDropViewTargets),
		PGDropIfExists: s.DDL.Options["if_exists"] == "true", Qualifiers: map[string]string{"cascade": s.DDL.Options["cascade"]}}
}

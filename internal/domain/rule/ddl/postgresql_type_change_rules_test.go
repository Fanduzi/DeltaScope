package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGTypeChangePrecedence(t *testing.T) {
	source := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "kind"}}
	for _, tc := range []struct {
		action, dest, want string
		owner              spec.PGTableOwnerTarget
		change             func(*spec.PGTypeChangeState)
	}{
		{action: "rename", dest: "kind", want: "alter_type_relation_occupied", change: func(s *spec.PGTypeChangeState) {
			s.Composite, s.DestinationType, s.DestinationRelation = true, true, true
		}},
		{action: "set_schema", dest: "app", change: func(s *spec.PGTypeChangeState) {
			s.DestinationType, s.DestinationRelation, s.CompanionOccupied = true, true, true
		}},
		{action: "set_schema", dest: "absent", want: "alter_type_schema_absent", change: func(s *spec.PGTypeChangeState) {
			s.Array, s.DestinationSchema = true, false
		}},
		{action: "set_schema", dest: "other", want: "alter_type_destination_occupied", change: func(s *spec.PGTypeChangeState) {
			s.RowType, s.DestinationType = true, true
		}},
		{action: "set_schema", dest: "other", want: "alter_type_companion_occupied", change: func(s *spec.PGTypeChangeState) {
			s.CompanionOccupied = true
		}},
		{action: "set_owner", owner: spec.PGTableOwnerTarget{Kind: "named", Name: "absent"}, want: "alter_type_owner_absent", change: func(s *spec.PGTypeChangeState) {
			s.SourceExists = false
		}},
		{action: "set_owner", owner: spec.PGTableOwnerTarget{Kind: "current_role"}, change: func(s *spec.PGTypeChangeState) {
			s.OwnerExists, s.Defined = true, false
		}},
	} {
		declaration := &spec.PGTypeChange{Source: source, Action: tc.action, Destination: tc.dest, Owner: tc.owner}
		state := &spec.PGTypeChangeState{Target: *declaration, SourceExists: true, Defined: true, DestinationSchema: true}
		tc.change(state)
		s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
			DDL:      &spec.DDL{Operation: spec.DDLOperationAlterType, PGTypeChange: declaration},
			Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGTypeChange: state}}
		if reason, gap := assessPGTypeChange(s); reason != tc.want || gap != "" {
			t.Fatalf("%s %s: assessment = (%q, %q), want (%q, no gap)", tc.action, tc.dest, reason, gap, tc.want)
		}
	}
}

package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGPolicyChangeAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, reason, fact string
		change             func(*spec.PGPolicyChange, *spec.PGPolicyChangeState)
	}{
		{name: "select prohibits with check", reason: "alter_policy_with_check_inapplicable", change: func(d *spec.PGPolicyChange, _ *spec.PGPolicyChangeState) {
			d.WithCheck = spec.PGPolicyPredicate{Present: true, BooleanResult: true}
		}},
		{name: "insert prohibits using", reason: "alter_policy_using_inapplicable", change: func(d *spec.PGPolicyChange, s *spec.PGPolicyChangeState) {
			s.PolicyCommand = "a"
			d.Using = spec.PGPolicyPredicate{Present: true, BooleanResult: true}
		}},
		{name: "role before absent table", reason: "alter_policy_role_absent", change: func(d *spec.PGPolicyChange, s *spec.PGPolicyChangeState) {
			d.Roles = []spec.PGTableOwnerTarget{{Kind: "named", Name: "missing"}, {Kind: "public"}}
			s.NamedRoles, s.MissingRole, s.RelationKind = []string{"missing"}, true, ""
		}},
		{name: "rename destination before absent source", reason: "alter_policy_rename_occupied", change: func(d *spec.PGPolicyChange, s *spec.PGPolicyChangeState) {
			d.RenameTo, s.RenameTo, s.PolicyCommand, s.DestinationOccupied = "next", "next", "", true
		}},
		{name: "column before absent policy", reason: "alter_policy_column_absent", change: func(d *spec.PGPolicyChange, s *spec.PGPolicyChangeState) {
			d.Using = spec.PGPolicyPredicate{Present: true, Column: " x ", BooleanResult: true}
			s.PolicyCommand = ""
		}},
		{name: "exact null test", change: func(d *spec.PGPolicyChange, _ *spec.PGPolicyChangeState) {
			d.Using = spec.PGPolicyPredicate{Present: true, Column: " X ", BooleanResult: true}
		}},
		{name: "unknown predicate result", fact: "alter_policy.predicate_type", change: func(d *spec.PGPolicyChange, _ *spec.PGPolicyChangeState) {
			d.Using = spec.PGPolicyPredicate{Present: true, Column: " X "}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := spec.Table{Schema: "app", Name: "t"}
			d := &spec.PGPolicyChange{Table: spec.PGDestructiveTableTarget{Table: table}, Name: "p"}
			state := &spec.PGPolicyChangeState{Table: table, Name: "p", RelationKind: "r", PolicyCommand: "r", Columns: []string{" X "}}
			tc.change(d, state)
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationAlterPolicy, PGPolicyChange: d},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGPolicyChange: state}}
			if reason, fact := assessPGPolicyChange(s); reason != tc.reason || fact != tc.fact {
				t.Fatalf("assessment = (%q, %q), want (%q, %q)", reason, fact, tc.reason, tc.fact)
			}
		})
	}
}

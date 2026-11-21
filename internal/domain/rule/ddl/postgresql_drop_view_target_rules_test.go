package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGDropViewTargetAssessment(t *testing.T) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: " Exact View "}}
	for _, tc := range []struct {
		name, kind, reason, gap             string
		guard, cascade, dependent, misbound bool
	}{
		{name: "guarded absence", guard: true},
		{name: "unguarded absence", reason: "drop_view_absent"},
		{name: "guard preserves wrong-kind refusal", kind: "r", guard: true, reason: "drop_view_wrong_kind"},
		{name: "direct dependent view", kind: "v", dependent: true, reason: "drop_view_has_direct_dependency"},
		{name: "cascade handles the dependency", kind: "v", cascade: true, dependent: true},
		{name: "mismatched catalog identity", kind: "v", dependent: true, misbound: true, gap: "view.pg_drop_state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observed := target
			if tc.misbound {
				observed.Table.Name = " exact View "
			}
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationDropView, PGDropViewTarget: &target, Options: map[string]string{}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGDropViewState: &spec.PGDropViewState{Target: observed, Kind: tc.kind, DependentView: tc.dependent}},
			}
			if tc.guard {
				s.DDL.Options["if_exists"] = "true"
			}
			if tc.cascade {
				s.DDL.Options["cascade"] = "true"
			}
			if reason, gap := assessPGDropViewTarget(s); reason != tc.reason || gap != tc.gap {
				t.Fatalf("got %q/%q, want %q/%q", reason, gap, tc.reason, tc.gap)
			}
		})
	}
}

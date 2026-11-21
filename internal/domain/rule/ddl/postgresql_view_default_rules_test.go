package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGViewDefaultAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, kind, action, reason, gap          string
		guard, columnExists, invalid, mismatched bool
	}{
		{name: "guard skips missing view", action: "set_default", guard: true, invalid: true},
		{name: "source kind precedes expression", kind: "m", action: "set_default", invalid: true, reason: "view_default_target_wrong_kind"},
		{name: "column absence precedes expression", kind: "v", action: "set_default", invalid: true, reason: "view_default_column_absent"},
		{name: "invalid reference", kind: "v", action: "set_default", columnExists: true, invalid: true, reason: "view_default_reference_inapplicable"},
		{name: "valid drop", kind: "v", action: "drop_default", columnExists: true},
		{name: "case-sensitive column binding", kind: "v", action: "set_default", mismatched: true, gap: "view.pg_default_target"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &spec.PGViewDefaultState{Schema: "app", View: "v", Column: " Exact Column ", Kind: tc.kind, ColumnExists: tc.columnExists}
			if tc.mismatched {
				state.Column = " exact Column "
			}
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterView, Table: &spec.Table{Schema: "app", Name: "v"}, PGTableIfExists: tc.guard,
					Alter: []spec.Alter{{Action: tc.action, Name: " Exact Column ", PGColumnTransition: &spec.PGColumnTransition{DefaultHasDisallowedReference: tc.invalid}}}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGViewDefault: state},
			}
			if reason, gap := assessPGViewDefault(s); reason != tc.reason || gap != tc.gap {
				t.Fatalf("got %q/%q, want %q/%q", reason, gap, tc.reason, tc.gap)
			}
		})
	}
}

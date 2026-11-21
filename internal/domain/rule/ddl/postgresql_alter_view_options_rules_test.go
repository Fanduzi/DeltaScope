package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGAlterViewOptionAssessment(t *testing.T) {
	validCheck := spec.PGViewOption{Name: "check_option", ValueKnown: true, ValueValid: true, HasValue: true}
	unknown := spec.PGViewOption{Name: "unknown"}
	for _, tc := range []struct {
		name, kind, action, reason, gap string
		options                         []spec.PGViewOption
		guard, storedCheckOption, stale bool
	}{
		{name: "guarded absence", action: "reset_options", options: []spec.PGViewOption{validCheck}, guard: true},
		{name: "source kind before reset declaration", kind: "m", action: "reset_options", options: []spec.PGViewOption{validCheck}, guard: true, reason: "view_options_target_wrong_kind"},
		{name: "reset with a value", kind: "v", action: "reset_options", options: []spec.PGViewOption{validCheck}, reason: "view_option_reset_has_value"},
		{name: "reset unknown name repeatedly", kind: "v", action: "reset_options", options: []spec.PGViewOption{unknown, unknown}},
		{name: "unknown set namespace", kind: "v", action: "set_options", options: []spec.PGViewOption{{Name: "unknown", Namespace: "outside"}}, reason: "view_option_namespace_unrecognized"},
		{name: "ignored toast namespace", kind: "v", action: "set_options", options: []spec.PGViewOption{{Name: "check_option", Namespace: "toast", HasValue: true}}},
		{name: "set check option", kind: "v", action: "set_options", options: []spec.PGViewOption{validCheck}, gap: "view.check_option_updatability"},
		{name: "stored check option remains", kind: "v", action: "reset_options", options: []spec.PGViewOption{unknown}, storedCheckOption: true, gap: "view.check_option_updatability"},
		{name: "reset check option", kind: "v", action: "reset_options", options: []spec.PGViewOption{{Name: "check_option"}}, storedCheckOption: true},
		{name: "guarded unknown state", kind: "v", action: "reset_options", options: []spec.PGViewOption{unknown}, stale: true, guard: true, gap: "view.pg_option_state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterView, Table: &spec.Table{Schema: "app", Name: "v"},
					Alter: []spec.Alter{{Action: tc.action}}, PGViewOptions: tc.options, PGTableIfExists: tc.guard},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}},
			}
			if !tc.stale {
				s.Metadata.PGDropViewState = &spec.PGDropViewState{Target: spec.PGDestructiveTableTarget{Table: *s.DDL.Table}, Kind: tc.kind, CheckOption: tc.storedCheckOption}
			}
			if reason, gap := assessPGAlterViewOptions(s); reason != tc.reason || gap != tc.gap {
				t.Fatalf("got %q/%q, want %q/%q", reason, gap, tc.reason, tc.gap)
			}
		})
	}
}

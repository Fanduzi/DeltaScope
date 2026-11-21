package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGMaterializedViewOptionsAssessment(t *testing.T) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}
	for _, tc := range []struct {
		name, kind, action string
		guard, heap, toast bool
		option             spec.PGTableReloption
		reason, fact       string
	}{
		{name: "guarded absence", guard: true, action: "set_options", option: spec.PGTableReloption{Namespace: "invalid"}},
		{name: "wrong kind precedes option", kind: "v", guard: true, action: "set_options", option: spec.PGTableReloption{Namespace: "invalid"}, reason: "materialized_view_options_target_wrong_kind"},
		{name: "heap bounds", kind: "m", heap: true, action: "set_options", option: spec.PGTableReloption{Name: "fillfactor", HasValue: true, Value: spec.PGTableReloptionInteger, Integer: 101}, reason: "materialized_view_option_out_of_range"},
		{name: "missing heap facts", kind: "m", action: "set_options", option: spec.PGTableReloption{Name: "fillfactor", HasValue: true}, fact: "materialized_view.pg_heap"},
		{name: "toast option ignored", kind: "m", heap: true, action: "set_options", option: spec.PGTableReloption{Namespace: "toast", Name: "fillfactor"}},
		{name: "toast rejects heap fillfactor", kind: "m", heap: true, toast: true, action: "set_options", option: spec.PGTableReloption{Namespace: "toast", Name: "fillfactor"}, reason: "materialized_view_option_unrecognized"},
		{name: "reset lacks values", kind: "m", heap: true, action: "reset_options", option: spec.PGTableReloption{Namespace: "unknown", Name: "future"}},
		{name: "reset value", kind: "m", heap: true, action: "reset_options", option: spec.PGTableReloption{Namespace: "unknown", Name: "future", HasValue: true}, reason: "materialized_view_option_reset_has_value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterMaterializedView, Table: &target.Table, PGTableIfExists: tc.guard,
					Alter: []spec.Alter{{Action: tc.action, PGTableReloptions: []spec.PGTableReloption{tc.option}}}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
					PGDropViewState: &spec.PGDropViewState{Target: target, Kind: tc.kind, Heap: tc.heap, HasToast: tc.toast}},
			}
			if reason, fact := assessPGMaterializedViewOptions(s); reason != tc.reason || fact != tc.fact {
				t.Fatalf("assessment=(%q,%q), want (%q,%q)", reason, fact, tc.reason, tc.fact)
			}
		})
	}
}

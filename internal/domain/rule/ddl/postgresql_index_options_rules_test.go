package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGIndexOptionsAssessment(t *testing.T) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "idx"}}
	tooLow := spec.PGTableReloption{Name: "fillfactor", HasValue: true, Value: spec.PGTableReloptionInteger, Integer: 9}
	for _, tc := range []struct {
		name, kind, method, action string
		guard                      bool
		options                    []spec.PGTableReloption
		reason, fact               string
	}{
		{name: "guarded missing target", guard: true, action: "set_options", options: []spec.PGTableReloption{tooLow}},
		{name: "partitioned index precedes option checks", kind: "I", action: "set_options", options: []spec.PGTableReloption{tooLow}, reason: "index_options_target_wrong_kind"},
		{name: "B-tree lower bound", kind: "i", method: "btree", action: "set_options", options: []spec.PGTableReloption{tooLow}, reason: "index_option_out_of_range"},
		{name: "hash lower bound", kind: "i", method: "hash", action: "set_options", options: []spec.PGTableReloption{tooLow}, reason: "index_option_out_of_range"},
		{name: "unobserved method", kind: "i", action: "set_options", options: []spec.PGTableReloption{tooLow}, fact: "index.access_method"},
		{name: "reset rejects values with unknown method", kind: "i", action: "reset_options", options: []spec.PGTableReloption{tooLow}, reason: "index_option_reset_has_value"},
		{name: "reset ignores duplicate unknown names", kind: "i", method: "btree", action: "reset_options", options: []spec.PGTableReloption{{Name: "unknown"}, {Name: "unknown"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterIndex, Table: &target.Table, PGTableIfExists: tc.guard,
					Alter: []spec.Alter{{Action: tc.action, PGTableReloptions: tc.options}}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
					PGIndexOptions: &spec.PGIndexOptionsState{Target: target, Kind: tc.kind, Method: tc.method}},
			}
			if reason, fact := assessPGIndexOptions(s); reason != tc.reason || fact != tc.fact {
				t.Fatalf("assessment = (%q, %q), want (%q, %q)", reason, fact, tc.reason, tc.fact)
			}
		})
	}
}

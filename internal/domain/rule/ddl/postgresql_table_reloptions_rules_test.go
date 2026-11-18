package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGTableReloptionsAssessment(t *testing.T) {
	integer := func(n int32) spec.PGTableReloption {
		return spec.PGTableReloption{Name: "fillfactor", HasValue: true, Value: spec.PGTableReloptionInteger, Integer: n}
	}
	for _, tc := range []struct {
		name, action, reason, parameter, gap string
		options                              []spec.PGTableReloption
		change                               func(*spec.Statement)
	}{
		{name: "lower bound", options: []spec.PGTableReloption{integer(10)}},
		{name: "upper bound", options: []spec.PGTableReloption{integer(100)}},
		{name: "below limit", options: []spec.PGTableReloption{integer(9)}, reason: "table_reloption_out_of_range", parameter: "fillfactor"},
		{name: "above limit", options: []spec.PGTableReloption{integer(101)}, reason: "table_reloption_out_of_range", parameter: "fillfactor"},
		{name: "missing", options: []spec.PGTableReloption{{Name: "fillfactor"}}, reason: "table_reloption_value_required", parameter: "fillfactor"},
		{name: "duplicate", options: []spec.PGTableReloption{integer(80), integer(80)}, reason: "table_reloption_duplicate", parameter: "fillfactor"},
		{name: "opaque numeric", options: []spec.PGTableReloption{{Name: "fillfactor", HasValue: true}}, gap: "table.reloptions"},
		{name: "unknown SET", options: []spec.PGTableReloption{{Name: "private_key", HasValue: true}}, gap: "table.reloptions"},
		{name: "TOAST SET", options: []spec.PGTableReloption{{Name: "autovacuum_enabled", Namespace: "toast", HasValue: true}}, gap: "table.reloptions"},
		{name: "known refusal with unknown", options: []spec.PGTableReloption{{Name: "opaque"}, integer(9)}, reason: "table_reloption_out_of_range", parameter: "fillfactor"},
		{name: "boolean", options: []spec.PGTableReloption{{Name: "autovacuum_enabled", Value: spec.PGTableReloptionBoolean}}},
		{name: "bad boolean", options: []spec.PGTableReloption{{Name: "autovacuum_enabled", HasValue: true, Value: spec.PGTableReloptionInvalidBoolean}}, reason: "table_reloption_invalid_boolean", parameter: "autovacuum_enabled"},
		{name: "reset unknown and duplicate", action: "reset_reloptions", options: []spec.PGTableReloption{{Name: "private_key", Namespace: "arbitrary"}, {Name: "fillfactor"}, {Name: "fillfactor"}}},
		{name: "reset valued opaque", action: "reset_reloptions", options: []spec.PGTableReloption{{Name: "private_key", HasValue: true}}, reason: "table_reloption_reset_has_value"},
		{name: "no AST members", gap: "table.reloptions"},
		{name: "offline", gap: "instance.version", change: func(s *spec.Statement) { s.Metadata = nil }},
		{name: "not heap", options: []spec.PGTableReloption{integer(9)}, gap: "target_table.pg_heap", change: func(s *spec.Statement) { s.Metadata.TargetTable.PGRelation.Heap = false }},
		{name: "typed", gap: "target_table.pg_heap", change: func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Typed = true }},
		{name: "unqualified", gap: "target_table.pg_heap", change: func(s *spec.Statement) { s.DDL.Table.Schema = "" }},
		{name: "three-part", gap: "target_table.pg_heap", change: func(s *spec.Statement) { s.DDL.PGTableCatalogQualified = true }},
		{name: "stale", gap: "target_table.pg_heap", change: func(s *spec.Statement) { s.Metadata.TargetTable.PGRelation = spec.PGRelationState{} }},
		{name: "multi-action", gap: "target_table.pg_heap", change: func(s *spec.Statement) { s.DDL.Alter = append(s.DDL.Alter, spec.Alter{Action: "set_access_method"}) }},
		{name: "absent with guard", options: []spec.PGTableReloption{integer(9)}, change: func(s *spec.Statement) {
			s.DDL.PGTableIfExists = true
			s.Metadata.TargetTable.Exists, s.Metadata.TargetTable.PGRelation.Kind = false, ""
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			action := tc.action
			if action == "" {
				action = "set_reloptions"
			}
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL, DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
				Alter: []spec.Alter{{Action: action, PGTableReloptions: tc.options}},
			}, Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
				TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
					PGRelation: spec.PGRelationState{Name: "t", Kind: "r", Heap: true}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
			if tc.change != nil {
				tc.change(&s)
			}
			r := pgTableReloptionsRule{level: rule.LevelBlocker}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") {
				t.Fatalf("findings=%+v err=%v", findings, err)
			}
			if len(findings) == 1 {
				f := findings[0]
				parameter, _ := f.Metadata["parameter"].(string)
				if f.Metadata["reason_code"] != tc.reason || parameter != tc.parameter || f.Level != rule.LevelBlocker {
					t.Fatalf("finding=%+v", f)
				}
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.gap != "") || len(gaps) == 1 && (gaps[0].ReasonCode != "unproven_table_reloptions" || gaps[0].RequiredFacts[0] != tc.gap) {
				t.Fatalf("gaps=%+v", gaps)
			}
			s.Dialect = spec.DialectMySQL
			if r.AppliesTo(s) {
				t.Fatal("PostgreSQL rule applies to MySQL")
			}
		})
	}
}

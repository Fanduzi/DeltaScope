package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGTableAccessMethodAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, reason, fact string
		change             func(*spec.Statement, *spec.PGTableAccessMethod)
	}{
		{name: "same"},
		{name: "move", change: func(_ *spec.Statement, f *spec.PGTableAccessMethod) { f.Same = false }},
		{name: "missing", reason: "access_method_absent", change: func(_ *spec.Statement, f *spec.PGTableAccessMethod) { f.Kind = "" }},
		{name: "index kind", reason: "access_method_wrong_kind", change: func(_ *spec.Statement, f *spec.PGTableAccessMethod) { f.Kind = "i" }},
		{name: "unbound", fact: "target_table.pg_access_method", change: func(_ *spec.Statement, f *spec.PGTableAccessMethod) { f.Name = "mixed method" }},
		{name: "unknown comparison", fact: "target_table.pg_access_method", change: func(_ *spec.Statement, f *spec.PGTableAccessMethod) { f.SameKnown = false }},
		{name: "quoted default does not use the setting", fact: "target_table.pg_access_method", change: func(s *spec.Statement, f *spec.PGTableAccessMethod) {
			s.DDL.Alter[0].Options["access_method"] = "default"
			f.Name, f.Default = "default", true
		}},
		{name: "bare default with observed current method", change: func(s *spec.Statement, f *spec.PGTableAccessMethod) {
			s.DDL.Alter[0].Options["access_method"], s.DDL.Alter[0].Options["is_default"] = "default", "true"
			f.Name, f.Default = "default", true
		}},
		{name: "stale", fact: "target_table.pg_access_method", change: func(s *spec.Statement, _ *spec.PGTableAccessMethod) {
			s.Metadata.TargetTable.PGRelation = spec.PGRelationState{}
		}},
		{name: "offline", fact: "instance.version", change: func(s *spec.Statement, _ *spec.PGTableAccessMethod) { s.Metadata = nil }},
		{name: "guarded absence", change: func(s *spec.Statement, _ *spec.PGTableAccessMethod) {
			s.DDL.PGTableIfExists = true
			s.Metadata.TargetTable.Exists, s.Metadata.TargetTable.PGRelation.Kind = false, ""
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &spec.PGTableAccessMethod{Schema: "app", Table: "t", Name: " Mixed Method ", Kind: "t", Same: true, SameKnown: true}
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL, DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
				Alter: []spec.Alter{{Action: "set_access_method", Options: map[string]string{"access_method": f.Name, "is_default": "false"}}},
			}, Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGTableAccessMethod: f,
				TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
					PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
			if tc.change != nil {
				tc.change(&s, f)
			}
			r := pgTableAccessMethodRule{level: rule.LevelBlocker}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") {
				t.Fatalf("findings=%+v err=%v", findings, err)
			}
			if len(findings) == 1 && (findings[0].Metadata["reason_code"] != tc.reason || len(findings[0].Metadata) != 4) {
				t.Fatalf("finding=%+v", findings[0])
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.fact != "") || len(gaps) == 1 && (gaps[0].ReasonCode != "unproven_table_access_method" || gaps[0].RequiredFacts[0] != tc.fact) {
				t.Fatalf("gaps=%+v", gaps)
			}
			advice, err := mustNewSetAccessMethodWarnRule(t, policy.RulePolicy{}).Evaluate(context.Background(), s)
			if err != nil || len(advice) != 1 || strings.Contains(advice[0].Message, "selects the current access method") != (tc.name == "same" || tc.name == "bare default with observed current method") ||
				strings.Contains(advice[0].Explanation.Why, "does not rewrite table files") != (tc.name == "same" || tc.name == "bare default with observed current method") {
				t.Fatalf("advice=%+v err=%v", advice, err)
			}
			s.DDL.Alter[0].Action = "set_tablespace"
			if r.AppliesTo(s) {
				t.Fatal("method rule applied to a tablespace change")
			}
		})
	}
}

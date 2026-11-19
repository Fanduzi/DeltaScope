package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGTableTablespaceAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, reason, fact string
		change             func(*spec.Statement, *spec.PGTableTablespace)
	}{
		{name: "same"},
		{name: "move", change: func(_ *spec.Statement, f *spec.PGTableTablespace) { f.Same = false }},
		{name: "missing", reason: "tablespace_absent", change: func(_ *spec.Statement, f *spec.PGTableTablespace) { f.Exists = false }},
		{name: "global", reason: "tablespace_global_inapplicable", change: func(_ *spec.Statement, f *spec.PGTableTablespace) { f.Global = true }},
		{name: "unbound", fact: "target_table.pg_tablespace", change: func(_ *spec.Statement, f *spec.PGTableTablespace) { f.Name = "mixed space" }},
		{name: "unknown comparison", fact: "target_table.pg_tablespace", change: func(_ *spec.Statement, f *spec.PGTableTablespace) { f.SameKnown = false }},
		{name: "stale", fact: "target_table.pg_tablespace", change: func(s *spec.Statement, _ *spec.PGTableTablespace) {
			s.Metadata.TargetTable.PGRelation = spec.PGRelationState{}
		}},
		{name: "offline", fact: "instance.version", change: func(s *spec.Statement, _ *spec.PGTableTablespace) { s.Metadata = nil }},
		{name: "guarded absence", change: func(s *spec.Statement, _ *spec.PGTableTablespace) {
			s.DDL.PGTableIfExists = true
			s.Metadata.TargetTable.Exists, s.Metadata.TargetTable.PGRelation.Kind = false, ""
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &spec.PGTableTablespace{Schema: "app", Table: "t", Name: " Mixed Space ", Exists: true, Same: true, SameKnown: true}
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL, DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
				Alter: []spec.Alter{{Action: "set_tablespace", Options: map[string]string{"tablespace": f.Name}}},
			}, Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGTableTablespace: f,
				TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
					PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
			if tc.change != nil {
				tc.change(&s, f)
			}
			r := pgTableTablespaceRule{level: rule.LevelBlocker}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") {
				t.Fatalf("findings=%+v err=%v", findings, err)
			}
			if len(findings) == 1 && (findings[0].Metadata["reason_code"] != tc.reason || len(findings[0].Metadata) != 4) {
				t.Fatalf("finding=%+v", findings[0])
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.fact != "") || len(gaps) == 1 && (gaps[0].ReasonCode != "unproven_table_tablespace" || gaps[0].RequiredFacts[0] != tc.fact) {
				t.Fatalf("gaps=%+v", gaps)
			}
			advice, err := mustNewSetTablespaceNoticeRule(t, policy.RulePolicy{}).Evaluate(context.Background(), s)
			if err != nil || len(advice) != 1 || strings.Contains(advice[0].Message, "selects the current tablespace") != (tc.name == "same") ||
				strings.Contains(advice[0].Explanation.Why, "does not copy table files") != (tc.name == "same") {
				t.Fatalf("advice=%+v err=%v", advice, err)
			}
			s.DDL.Alter[0].Action = "move_all_tablespace"
			if r.AppliesTo(s) {
				t.Fatal("single-table rule applied to a bulk move")
			}
		})
	}
}

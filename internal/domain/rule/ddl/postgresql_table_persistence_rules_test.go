package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGTablePersistenceAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, reason, gap string
		change            func(*spec.Statement, *spec.PGTablePersistence)
		noRewrite         bool
	}{
		{name: "legal change"},
		{name: "direct publication before foreign key", reason: "persistence_direct_publication", change: func(_ *spec.Statement, f *spec.PGTablePersistence) {
			f.DirectPublication, f.ForeignKeyConflict = true, true
		}},
		{name: "inbound", reason: "persistence_inbound_foreign_key", change: func(_ *spec.Statement, f *spec.PGTablePersistence) {
			f.ForeignKeyConflict, f.ForeignKeyUnknown = true, true
		}},
		{name: "outbound", reason: "persistence_outbound_foreign_key", change: func(s *spec.Statement, f *spec.PGTablePersistence) {
			s.DDL.Alter[0].Action, f.Logged, f.Current, f.ForeignKeyConflict = "set_logged", true, "u", true
		}},
		{name: "same unlogged bypasses dependencies", change: func(_ *spec.Statement, f *spec.PGTablePersistence) {
			f.Current, f.DirectPublication, f.ForeignKeyConflict = "u", true, true
		}, noRewrite: true},
		{name: "same logged", change: func(s *spec.Statement, f *spec.PGTablePersistence) {
			s.DDL.Alter[0].Action, f.Logged = "set_logged", true
		}, noRewrite: true},
		{name: "unknown peer", gap: "target_table.pg_persistence", change: func(_ *spec.Statement, f *spec.PGTablePersistence) { f.ForeignKeyUnknown = true }},
		{name: "wrong catalog scope", gap: "target_table.pg_persistence", change: func(_ *spec.Statement, f *spec.PGTablePersistence) { f.Current = "t" }},
		{name: "misbound", gap: "target_table.pg_persistence", change: func(_ *spec.Statement, f *spec.PGTablePersistence) { f.Logged = true }},
		{name: "stale", gap: "target_table.pg_persistence", change: func(s *spec.Statement, _ *spec.PGTablePersistence) {
			s.Metadata.TargetTable.PGRelation = spec.PGRelationState{}
		}},
		{name: "three-part", gap: "target_table.pg_persistence", change: func(s *spec.Statement, _ *spec.PGTablePersistence) { s.DDL.PGTableCatalogQualified = true }},
		{name: "offline", gap: "instance.version", change: func(s *spec.Statement, _ *spec.PGTablePersistence) { s.Metadata = nil }},
		{name: "guarded absence", change: func(s *spec.Statement, _ *spec.PGTablePersistence) {
			s.DDL.PGTableIfExists = true
			s.Metadata.TargetTable.Exists, s.Metadata.TargetTable.PGRelation.Kind = false, ""
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fact := &spec.PGTablePersistence{Schema: "app", Table: "t", Current: "p"}
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL, DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
				Alter: []spec.Alter{{Action: "set_unlogged"}},
			}, Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGTablePersistence: fact,
				TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}}}}
			if tc.change != nil {
				tc.change(&s, fact)
			}
			r := pgTablePersistenceRule{level: rule.LevelBlocker}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") {
				t.Fatalf("findings=%+v err=%v", findings, err)
			}
			if len(findings) == 1 && (findings[0].Metadata["reason_code"] != tc.reason || len(findings[0].Metadata) != 4 || findings[0].Level != rule.LevelBlocker) {
				t.Fatalf("finding=%+v", findings[0])
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.gap != "") || len(gaps) == 1 && (gaps[0].ReasonCode != "unproven_table_persistence" || gaps[0].RequiredFacts[0] != tc.gap) {
				t.Fatalf("gaps=%+v", gaps)
			}
			notice := mustNewSetUnloggedNoticeRule(t, policy.RulePolicy{})
			if s.DDL.Alter[0].Action == "set_logged" {
				notice = mustNewSetLoggedNoticeRule(t, policy.RulePolicy{})
			}
			advice, err := notice.Evaluate(context.Background(), s)
			if err != nil || len(advice) != 1 || advice[0].Level != rule.LevelNotice || strings.Contains(advice[0].Explanation.Why, "does not rewrite table files") != tc.noRewrite {
				t.Fatalf("advice=%+v err=%v", advice, err)
			}
			s.Dialect = spec.DialectMySQL
			if r.AppliesTo(s) {
				t.Fatal("PostgreSQL rule applied to MySQL")
			}
		})
	}
}

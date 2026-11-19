package ddl

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGTableNamespaceDestination(t *testing.T) {
	const destinationFact = "target_table.pg_namespace_destination"
	for _, tc := range []struct {
		name, action, reason, gap string
		change                    func(*spec.Statement, *spec.PGTableNamespace)
		same                      bool
	}{
		{name: "rename available"},
		{name: "rename relation conflict including self", reason: "destination_relation_conflict", change: func(_ *spec.Statement, f *spec.PGTableNamespace) { f.RelationConflict = true }},
		{name: "rename type conflict", reason: "destination_row_type_conflict", change: func(_ *spec.Statement, f *spec.PGTableNamespace) { f.RowTypeConflict = true }},
		{name: "rename array ownership unknown", gap: destinationFact, change: func(_ *spec.Statement, f *spec.PGTableNamespace) { f.RowTypeConflictKnown = false }},
		{name: "rename relation conflict with unknown type", reason: "destination_relation_conflict", change: func(_ *spec.Statement, f *spec.PGTableNamespace) {
			f.RelationConflict, f.RowTypeConflictKnown = true, false
		}},
		{name: "available schema", action: "set_schema"},
		{name: "missing schema", action: "set_schema", reason: "destination_schema_absent", change: func(_ *spec.Statement, f *spec.PGTableNamespace) { f.SchemaExists = false }},
		{name: "same schema", action: "set_schema", same: true, change: func(s *spec.Statement, f *spec.PGTableNamespace) {
			s.DDL.Alter[0].Options["new_schema"] = "app"
			f.Destination, f.SameSchema, f.RelationConflict, f.RowTypeConflict = "app", true, true, true
		}},
		{name: "schema relation conflict", action: "set_schema", reason: "destination_relation_conflict", change: func(_ *spec.Statement, f *spec.PGTableNamespace) { f.RelationConflict = true }},
		{name: "schema type conflict", action: "set_schema", reason: "destination_row_type_conflict", change: func(_ *spec.Statement, f *spec.PGTableNamespace) { f.RowTypeConflict = true }},
		{name: "offline", gap: "instance.version", change: func(s *spec.Statement, _ *spec.PGTableNamespace) { s.Metadata = nil }},
		{name: "stale", gap: destinationFact, change: func(s *spec.Statement, _ *spec.PGTableNamespace) {
			s.Metadata.TargetTable.PGRelation = spec.PGRelationState{}
		}},
		{name: "misbound", gap: destinationFact, change: func(_ *spec.Statement, f *spec.PGTableNamespace) { f.Destination = "exact destination" }},
		{name: "wrong action witness", gap: destinationFact, change: func(_ *spec.Statement, f *spec.PGTableNamespace) { f.Action = "set_schema" }},
		{name: "typed", gap: destinationFact, change: func(s *spec.Statement, _ *spec.PGTableNamespace) { s.Metadata.TargetTable.PGColumns.Typed = true }},
		{name: "unqualified", gap: destinationFact, change: func(s *spec.Statement, _ *spec.PGTableNamespace) { s.DDL.Table.Schema = "" }},
		{name: "guarded absence", change: func(s *spec.Statement, _ *spec.PGTableNamespace) {
			s.DDL.PGTableIfExists = true
			s.Metadata.TargetTable.Exists, s.Metadata.TargetTable.PGRelation.Kind = false, ""
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			action, key := "rename_table", "new_name"
			r, err := newPGRenameTableTargetRule(policy.RulePolicy{})
			if tc.action == "set_schema" {
				action, key = tc.action, "new_schema"
				r, err = newPGSetSchemaTargetRule(policy.RulePolicy{})
			}
			if err != nil {
				t.Fatal(err)
			}
			f := &spec.PGTableNamespace{Schema: "app", Table: "t", Action: action, Destination: " Exact Destination ", SchemaExists: true, RowTypeConflictKnown: true}
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL, DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
				Alter: []spec.Alter{{Action: action, Options: map[string]string{key: f.Destination}}},
			}, Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGTableNamespace: f,
				TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
					PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
			if tc.change != nil {
				tc.change(&s, f)
			}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || len(findings) > 1 || (len(findings) == 1) != (tc.reason != "") {
				t.Fatalf("findings=%+v err=%v", findings, err)
			}
			if len(findings) == 1 {
				b, err := json.Marshal(findings[0])
				if err != nil || strings.Contains(string(b), "Exact Destination") || findings[0].Metadata["reason_code"] != tc.reason || findings[0].Level != rule.LevelBlocker {
					t.Fatalf("finding=%s err=%v", b, err)
				}
			}
			gaps := r.(rule.EvidenceReporter).EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.gap != "") || len(gaps) == 1 && (gaps[0].ReasonCode != "unproven_table_namespace_destination" || gaps[0].RequiredFacts[0] != tc.gap) {
				t.Fatalf("gaps=%+v", gaps)
			}
			if action == "set_schema" {
				n, err := newSetSchemaAdvisoryRule(policy.RulePolicy{})
				if err != nil {
					t.Fatal(err)
				}
				notices, err := n.Evaluate(context.Background(), s)
				if err != nil || len(notices) != 1 || notices[0].Level != rule.LevelNotice || strings.Contains(notices[0].Message, "selects the current schema") != tc.same {
					t.Fatalf("notices=%+v err=%v", notices, err)
				}
			}
			s.Dialect = spec.DialectMySQL
			if r.AppliesTo(s) {
				t.Fatal("PostgreSQL rule applied to MySQL")
			}
		})
	}
}

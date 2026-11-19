package ddl

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnTypeSourceAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, source, reason, fact string
		using                      bool
		change                     func(*spec.Statement)
	}{
		{name: "ordinary", source: "id"},
		{name: "ordinary using", source: "id", using: true},
		{name: "same spelling in quotes", source: "ID", reason: "column_absent"},
		{name: "system", source: "ctid", reason: "system_column_inapplicable"},
		{name: "unobserved system", source: "xmin", fact: "target_table.pg_columns"},
		{name: "generated without using", source: "stored"},
		{name: "generated using", source: "stored", using: true, reason: "generated_column_using_inapplicable"},
		{name: "quoted user CTID", source: "CTID"},
		{name: "ordinary spaces", source: " "},
		{name: "offline", source: "id", fact: "instance.version", change: func(s *spec.Statement) { s.Metadata = nil }},
		{name: "stale", source: "id", fact: "target_table.pg_columns", change: func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns = nil }},
		{name: "duplicate", source: "id", fact: "target_table.pg_columns", change: func(s *spec.Statement) {
			c := s.Metadata.TargetTable.PGColumns
			c.Columns = append(c.Columns, c.Columns[0])
		}},
		{name: "unqualified", source: "id", fact: "target_table.pg_columns", change: func(s *spec.Statement) { s.DDL.Table.Schema = "" }},
		{name: "database qualifier", source: "id", fact: "target_table.pg_columns", change: func(s *spec.Statement) { s.DDL.PGTableCatalogQualified = true }},
		{name: "typed", source: "id", fact: "target_table.pg_columns", change: func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Typed = true }},
		{name: "multiple actions", source: "id", fact: "target_table.pg_columns", change: func(s *spec.Statement) { s.DDL.Alter = append(s.DDL.Alter, spec.Alter{Action: "add_column"}) }},
		{name: "guarded absence", source: "id", change: func(s *spec.Statement) {
			s.DDL.PGTableIfExists = true
			s.Metadata.TargetTable.Exists, s.Metadata.TargetTable.PGRelation.Kind = false, ""
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL, DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
				Alter: []spec.Alter{{Action: "set_data_type", Name: tc.source, Column: &spec.AlterColumn{Definition: &spec.Column{Type: "private_target_type"}}}},
			}, Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
				TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"},
					PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}, {Name: "stored", Generated: "s"}, {Name: "CTID"}, {Name: " "}}, SystemColumns: []spec.PGColumnState{{Name: "ctid", System: true}}}}}}
			if tc.using {
				s.DDL.Alter[0].Options = map[string]string{"has_using": "true", "private_expression": "opaque_using_payload"}
			}
			if tc.change != nil {
				tc.change(&s)
			}
			r := pgColumnTypeSourceRule{level: rule.LevelBlocker}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") {
				t.Fatalf("findings=%+v err=%v", findings, err)
			}
			if len(findings) == 1 {
				f := findings[0]
				if f.Metadata["reason_code"] != tc.reason || f.Metadata["column"] != tc.source || f.Level != rule.LevelBlocker || len(f.Metadata) != 5 {
					t.Fatalf("finding=%+v", f)
				}
				b, err := json.Marshal(f)
				if err != nil || strings.Contains(string(b), "private_target_type") || strings.Contains(string(b), "opaque_using_payload") {
					t.Fatalf("finding=%s err=%v", b, err)
				}
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.fact != "") || len(gaps) == 1 && (gaps[0].ReasonCode != "unproven_column_type_source" || gaps[0].RequiredFacts[0] != tc.fact) {
				t.Fatalf("gaps=%+v", gaps)
			}
			s.Dialect = spec.DialectTiDB
			if r.AppliesTo(s) {
				t.Fatal("PG source rule applied to TiDB")
			}
		})
	}
}

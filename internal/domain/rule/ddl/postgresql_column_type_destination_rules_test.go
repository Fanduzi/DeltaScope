package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnTypeOnlyParentPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, reason, gap                   string
		missing, incompatible, using, stale bool
	}{
		{name: "compatible", reason: "column_type_requires_child_traversal"},
		{name: "missing type", missing: true, reason: "destination_type_absent"},
		{name: "incompatible type", incompatible: true, reason: "destination_type_assignment_unavailable"},
		{name: "USING", using: true, gap: "target_table.pg_columns"},
		{name: "stale", stale: true, gap: "target_table.pg_columns"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := spec.Table{Schema: "app", Name: "t"}
			target := spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int8"}
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &table, PGTableOnly: true,
					Alter: []spec.Alter{{Action: "set_data_type", Name: "Id", PGColumnType: target, Options: map[string]string{}}}},
				Metadata: &spec.Metadata{Schema: table.Schema, Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
					TargetTable: &spec.TableSnapshot{Schema: table.Schema, Exists: true, Table: &table,
						PGRelation: spec.PGRelationState{Name: table.Name, Kind: "r"}, PGColumns: &spec.PGColumnCatalog{}},
					PGColumnTraversal: &spec.PGColumnTraversal{Schema: table.Schema, Table: table.Name, Column: "Id", ColumnExists: true, AttachedChildren: true},
					PGColumnType: &spec.PGColumnTypeState{Target: target, SchemaExists: true, BaseExists: !tc.missing, Resolved: true, Defined: true,
						AssignmentKnown: true, Assignable: !tc.incompatible, SourceSchema: table.Schema, SourceTable: table.Name, SourceColumn: "Id"}}}
			if tc.using {
				s.DDL.Alter[0].Options["has_using"] = "true"
			}
			if tc.stale {
				s.Metadata.TargetTable.PGRelation = spec.PGRelationState{}
			}
			if reason, gap := assessPGColumnTypeDestination(s); reason != tc.reason || gap != tc.gap {
				t.Fatalf("reason=%q gap=%q, want reason=%q gap=%q", reason, gap, tc.reason, tc.gap)
			}
		})
	}
}

func TestPGColumnTypeDestinationAssessment(t *testing.T) {
	const evidence = "column.pg_type_destination"
	for _, tc := range []struct {
		name, reason, gap string
		change            func(*spec.Statement, *spec.PGColumnTypeState)
	}{
		{name: "defined"},
		{name: "schema absent", reason: "destination_type_schema_absent", change: func(_ *spec.Statement, f *spec.PGColumnTypeState) { f.SchemaExists = false }},
		{name: "type absent", reason: "destination_type_absent", change: func(_ *spec.Statement, f *spec.PGColumnTypeState) { f.BaseExists = false }},
		{name: "array absent", reason: "destination_array_type_absent", change: func(_ *spec.Statement, f *spec.PGColumnTypeState) { f.Resolved = false }},
		{name: "shell before pseudo", reason: "destination_type_undefined", change: func(_ *spec.Statement, f *spec.PGColumnTypeState) { f.Defined, f.Pseudo = false, true }},
		{name: "pseudo", reason: "destination_pseudo_type_inapplicable", change: func(_ *spec.Statement, f *spec.PGColumnTypeState) { f.Pseudo = true }},
		{name: "mismatch", gap: evidence, change: func(_ *spec.Statement, f *spec.PGColumnTypeState) { f.Target.Name = "exact type" }},
		{name: "unobserved", gap: evidence, change: func(s *spec.Statement, _ *spec.PGColumnTypeState) { s.Metadata.PGColumnType = nil }},
		{name: "offline", gap: "instance.version", change: func(s *spec.Statement, _ *spec.PGColumnTypeState) { s.Metadata = nil }},
		{name: "unqualified type", gap: "column.type_destination_identity", change: func(s *spec.Statement, _ *spec.PGColumnTypeState) { s.DDL.Alter[0].PGColumnType.Schema = "" }},
		{name: "stale table", gap: "target_table.pg_columns", change: func(s *spec.Statement, _ *spec.PGColumnTypeState) {
			s.Metadata.TargetTable.PGRelation = spec.PGRelationState{}
		}},
		{name: "absent source", change: func(s *spec.Statement, f *spec.PGColumnTypeState) { s.DDL.Alter[0].Name = "Missing"; f.Pseudo = true }},
		{name: "guarded absent table", change: func(s *spec.Statement, f *spec.PGColumnTypeState) {
			s.DDL.PGTableIfExists = true
			s.Metadata.TargetTable = &spec.TableSnapshot{Schema: "app", PGRelation: spec.PGRelationState{Name: "t"}}
			f.Pseudo = true
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := spec.PGColumnTypeTarget{Schema: " Type.Schema ", Name: "Exact Type", Array: true}
			facts := spec.PGColumnTypeState{Target: target, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true}
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
					Alter: []spec.Alter{{Action: "set_data_type", Name: "Id", PGColumnType: target}}},
				Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGColumnType: &facts,
					TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"},
						PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "Id"}}}}}}
			s.Metadata.PGDropColumnDependency = &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: "Id"}
			if tc.change != nil {
				tc.change(&s, &facts)
			}
			r := pgColumnTypeDestinationRule{level: rule.LevelBlocker}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") {
				t.Fatalf("findings=%+v, error=%v", findings, err)
			}
			if len(findings) == 1 && (findings[0].Metadata["reason_code"] != tc.reason || len(findings[0].Metadata) != 5 || strings.Contains(findings[0].Message, target.Name)) {
				t.Fatalf("unexpected finding: %+v", findings[0])
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.gap != "") || len(gaps) == 1 && (len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.gap) {
				t.Fatalf("gaps=%+v", gaps)
			}
			s.Dialect = spec.DialectMySQL
			if r.AppliesTo(s) {
				t.Fatal("PostgreSQL destination rule matched MySQL")
			}
		})
	}
}

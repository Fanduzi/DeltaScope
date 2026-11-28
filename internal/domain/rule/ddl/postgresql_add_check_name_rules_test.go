package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGAddCheckName(t *testing.T) {
	root := func(s *spec.Statement) {
		s.DDL.Alter[0].Name = ""
		s.DDL.Alter[0].PGCheckTrue = true
		s.DDL.PGTableOnly = true
		s.Metadata.TargetTable.PGRelation = spec.PGRelationState{Name: "t", Kind: "p", PersistentPartitionRoot: true, AttachedPartition: true}
		s.Metadata.TargetTable.PGColumns = nil
	}
	for _, tc := range []struct {
		name, gap, reason, literal string
		entries                    []spec.PGConstraintState
		change                     func(*spec.Statement)
		occupied                   bool
		unmatched                  bool
		subquery                   bool
		coercion                   *spec.PGCheckIntegerCoercion
		nullTest                   []string
		columns                    []spec.PGColumnState
	}{
		{name: "only with partitions", occupied: true, reason: "check_requires_partition_traversal", change: root},
		{name: "no inherit on childless root", occupied: true, reason: "check_no_inherit_partition_root", change: func(s *spec.Statement) {
			root(s)
			s.Metadata.TargetTable.PGRelation.AttachedPartition = false
			s.DDL.Alter[0].PGCheckNoInherit = true
		}},
		{name: "only on childless root", unmatched: true, change: func(s *spec.Statement) {
			root(s)
			s.Metadata.TargetTable.PGRelation.AttachedPartition = false
		}},
		{name: "offline unnamed stays unchanged", unmatched: true, change: func(s *spec.Statement) { root(s); s.Metadata = nil }},
		{name: "named check not broadened", gap: "target_table.pg_constraint_scope", change: func(s *spec.Statement) { root(s); s.DDL.Alter[0].Name = "named" }},
		{name: "nonliteral predicate not broadened", unmatched: true, change: func(s *spec.Statement) { root(s); s.DDL.Alter[0].PGCheckTrue = false }},
		{name: "exact check", entries: []spec.PGConstraintState{{Name: " Exact Check ", Type: "c"}}, occupied: true},
		{name: "different kind", entries: []spec.PGConstraintState{{Name: " Exact Check ", Type: "p"}}, occupied: true},
		{name: "exact spaces and case", entries: []spec.PGConstraintState{{Name: "Exact Check", Type: "c"}, {Name: " exact check ", Type: "c"}}},
		{name: "known empty", entries: []spec.PGConstraintState{}},
		{name: "subquery precedes exact collision", entries: []spec.PGConstraintState{{Name: " Exact Check ", Type: "c"}}, subquery: true, occupied: true},
		{name: "unnamed subquery", subquery: true, occupied: true, change: func(s *spec.Statement) { s.DDL.Alter[0].Name = "" }},
		{name: "integer before name collision", literal: "integer", coercion: &spec.PGCheckIntegerCoercion{Source: "integer"}, reason: "check_predicate_boolean_required", occupied: true,
			entries: []spec.PGConstraintState{{Name: " Exact Check ", Type: "c"}}},
		{name: "bigint cast not allowed", literal: "bigint", coercion: &spec.PGCheckIntegerCoercion{Source: "bigint"}, reason: "check_predicate_boolean_required", occupied: true},
		{name: "numeric cast possible", literal: "numeric", coercion: &spec.PGCheckIntegerCoercion{Source: "numeric", Allowed: true}, gap: "check.numeric_boolean_coercion"},
		{name: "misbound cast witness", literal: "numeric", coercion: &spec.PGCheckIntegerCoercion{Source: "bigint"}, gap: "check.numeric_boolean_coercion"},
		{name: "integer cast unknown", literal: "integer", gap: "check.integer_boolean_coercion"},
		{name: "assignment cast needs result evidence", literal: "integer", coercion: &spec.PGCheckIntegerCoercion{Source: "integer", Allowed: true}, gap: "check.integer_boolean_coercion"},
		{name: "malformed Boolean string", literal: "invalid_boolean", reason: "check_predicate_invalid_boolean", occupied: true, change: func(s *spec.Statement) { s.DDL.Alter[0].Name = "" }},
		{name: "known absent exact column", nullTest: []string{" exact Flag "}, columns: []spec.PGColumnState{{Name: " Exact Flag "}}, reason: "check_predicate_column_absent", occupied: true},
		{name: "unknown column catalog", nullTest: []string{"missing"}, gap: "target_table.pg_columns"},
		{name: "whole-row spelling", nullTest: []string{"t"}, columns: []spec.PGColumnState{}, entries: []spec.PGConstraintState{}},
		{name: "system column refusal", nullTest: []string{"ctid"}, columns: []spec.PGColumnState{}, reason: "check_predicate_system_column", occupied: true, change: func(s *spec.Statement) {
			s.Metadata.TargetTable.PGColumns.SystemColumns = []spec.PGColumnState{{Name: "ctid", System: true}}
		}},
		{name: "mismatched qualifier", nullTest: []string{"other", "id"}, reason: "check_predicate_qualifier_absent", occupied: true},
		{name: "mismatched schema", nullTest: []string{"other", "t", "id"}, reason: "check_predicate_qualifier_absent", occupied: true},
		{name: "local database unobserved", nullTest: []string{"postgres", "app", "t", "id"}, gap: "check.database_identity"},
		{name: "whole-row fallback unobserved", nullTest: []string{"app", "t", "missing"}, columns: []spec.PGColumnState{}, gap: "check.whole_row_function_or_cast"},
		{name: "whole-row fallback possible", nullTest: []string{"t", "label"}, columns: []spec.PGColumnState{}, gap: "check.whole_row_function_or_cast", change: func(s *spec.Statement) {
			s.Metadata.PGCheckQualifiedReference = &spec.PGCheckQualifiedReference{Database: "postgres", Member: "label", FallbackPossible: true}
		}},
		{name: "missing directory", gap: "target_table.pg_constraints"},
		{name: "ambiguous names", gap: "target_table.pg_constraints", entries: []spec.PGConstraintState{{Name: " Exact Check ", Type: "c"}, {Name: " Exact Check ", Type: "p"}}},
		{name: "stale binding", gap: "target_table.pg_constraint_scope", change: func(s *spec.Statement) { s.Metadata.TargetTable.PGRelation = spec.PGRelationState{} }},
		{name: "inherited table", gap: "target_table.pg_constraint_scope", change: func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Ordinary = false }},
		{name: "offline", gap: "instance.version", change: func(s *spec.Statement) { s.Metadata = nil }},
		{name: "qualified guard", change: func(s *spec.Statement) {
			s.DDL.PGTableIfExists = true
			s.Metadata.TargetTable = &spec.TableSnapshot{Schema: "app", PGRelation: spec.PGRelationState{Name: "t"}}
		}},
		{name: "unnamed", unmatched: true, change: func(s *spec.Statement) { s.DDL.Alter[0].Name = "" }},
		{name: "multiple actions", unmatched: true, change: func(s *spec.Statement) {
			s.DDL.Alter = append(s.DDL.Alter, spec.Alter{Action: "drop_constraint", Name: " Exact Check "})
		}},
		{name: "foreign key", unmatched: true, change: func(s *spec.Statement) { s.DDL.Alter[0].Options["constraint_type"] = "foreign_key" }},
		{name: "MySQL", unmatched: true, change: func(s *spec.Statement) { s.Dialect = spec.DialectMySQL }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec.Statement{
				Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
					Alter: []spec.Alter{{Action: "add_constraint", Name: " Exact Check ", Options: map[string]string{"constraint_type": "check", "not_valid": "true"}}}},
				Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
					TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGConstraints: tc.entries,
						PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}},
			}
			s.DDL.Alter[0].PGCheckHasSubquery = tc.subquery
			s.DDL.Alter[0].PGCheckLiteral = tc.literal
			s.DDL.Alter[0].PGCheckNullTest = tc.nullTest
			s.Metadata.TargetTable.PGColumns.Columns = tc.columns
			s.Metadata.PGCheckIntegerCoercion = tc.coercion
			if tc.change != nil {
				tc.change(&s)
			}
			r := pgAddCheckNameRule{level: rule.LevelBlocker}
			if r.AppliesTo(s) == tc.unmatched {
				t.Fatal("unexpected rule applicability")
			}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != tc.occupied {
				t.Fatalf("findings=%+v, err=%v", findings, err)
			}
			reason := "constraint_name_occupied"
			if tc.reason != "" {
				reason = tc.reason
			}
			if tc.subquery {
				reason = "check_predicate_subquery_inapplicable"
			}
			if len(findings) == 1 && (findings[0].Metadata["constraint"] != s.DDL.Alter[0].Name || findings[0].Metadata["reason_code"] != reason || len(findings[0].Metadata) != 5) {
				t.Fatalf("unexpected finding: %+v", findings[0])
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.gap != "") || len(gaps) == 1 && (len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.gap) {
				t.Fatalf("gaps=%+v", gaps)
			}
		})
	}
}

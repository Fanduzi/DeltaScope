package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGConstraintUsingIndex(t *testing.T) {
	for _, tc := range []struct {
		name, reason, gap string
		change            func(*spec.Statement, *spec.PGConstraintUsingIndex)
	}{
		{name: "eligible"},
		{name: "missing", reason: "constraint_index_absent", change: func(_ *spec.Statement, f *spec.PGConstraintUsingIndex) { f.Kind = "" }},
		{name: "wrong kind", reason: "constraint_index_wrong_kind", change: func(_ *spec.Statement, f *spec.PGConstraintUsingIndex) { f.Kind = "r" }},
		{name: "owned", reason: "constraint_index_already_owned", change: func(_ *spec.Statement, f *spec.PGConstraintUsingIndex) { f.ConstraintOwned = true }},
		{name: "different table", reason: "constraint_index_wrong_table", change: func(_ *spec.Statement, f *spec.PGConstraintUsingIndex) { f.OnTable = false }},
		{name: "invalid", reason: "constraint_index_invalid", change: func(_ *spec.Statement, f *spec.PGConstraintUsingIndex) { f.Valid = false }},
		{name: "nonunique", reason: "constraint_index_not_unique", change: func(_ *spec.Statement, f *spec.PGConstraintUsingIndex) { f.Unique = false }},
		{name: "expression", reason: "constraint_index_has_expression", change: func(_ *spec.Statement, f *spec.PGConstraintUsingIndex) { f.Expression = true }},
		{name: "partial", reason: "constraint_index_partial", change: func(_ *spec.Statement, f *spec.PGConstraintUsingIndex) { f.Partial = true }},
		{name: "non-btree", reason: "constraint_index_not_btree", change: func(_ *spec.Statement, f *spec.PGConstraintUsingIndex) { f.BTree = false }},
		{name: "nondefault key", reason: "constraint_index_nondefault_key", change: func(_ *spec.Statement, f *spec.PGConstraintUsingIndex) { f.KeyDefaults = false }},
		{name: "primary nulls not distinct", reason: "constraint_index_nulls_not_distinct_primary", change: func(_ *spec.Statement, f *spec.PGConstraintUsingIndex) { f.NullsNotDistinct = true }},
		{name: "unique nulls not distinct", change: func(s *spec.Statement, f *spec.PGConstraintUsingIndex) {
			s.DDL.Alter[0].Options["constraint_type"] = "unique"
			f.NullsNotDistinct = true
		}},
		{name: "unobserved key", gap: "index.constraint_key_defaults", change: func(_ *spec.Statement, f *spec.PGConstraintUsingIndex) { f.KeyDefaultsKnown = false }},
		{name: "misbound", gap: "target_table.pg_constraint_index", change: func(_ *spec.Statement, f *spec.PGConstraintUsingIndex) { f.Name = "EXACT INDEX" }},
		{name: "prior DDL", gap: "target_table.pg_constraint_index", change: func(s *spec.Statement, _ *spec.PGConstraintUsingIndex) {
			s.Metadata.TargetTable.PGRelation = spec.PGRelationState{}
		}},
		{name: "multi-action", gap: "target_table.pg_constraint_index", change: func(s *spec.Statement, _ *spec.PGConstraintUsingIndex) {
			s.DDL.Alter = append(s.DDL.Alter, spec.Alter{Action: "add_column"})
		}},
		{name: "offline", gap: "instance.version", change: func(s *spec.Statement, _ *spec.PGConstraintUsingIndex) { s.Metadata = nil }},
		{name: "qualified guard", change: func(s *spec.Statement, _ *spec.PGConstraintUsingIndex) {
			s.DDL.PGTableIfExists = true
			s.Metadata.TargetTable = &spec.TableSnapshot{Schema: "app", PGRelation: spec.PGRelationState{Name: "t"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const name = " Exact Index "
			facts := spec.PGConstraintUsingIndex{Schema: "app", Table: "t", Name: name, Kind: "i", IndexKnown: true, OnTable: true, Valid: true, Unique: true, BTree: true, KeyDefaults: true, KeyDefaultsKnown: true}
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
					Alter: []spec.Alter{{Action: "add_constraint", PGUsingIndexName: name, Options: map[string]string{"constraint_type": "primary_key", "using_index": "true"}}}},
				Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGConstraintUsingIndex: &facts,
					TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
						PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
			if tc.change != nil {
				tc.change(&s, &facts)
			}
			r := pgConstraintUsingIndexRule{level: rule.LevelBlocker}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") {
				t.Fatalf("findings=%+v error=%v", findings, err)
			}
			if len(findings) == 1 && (findings[0].Metadata["reason_code"] != tc.reason || len(findings[0].Metadata) != 5 || strings.Contains(findings[0].Message, name)) {
				t.Fatalf("unexpected finding: %+v", findings[0])
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.gap != "") || len(gaps) == 1 && (len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.gap) {
				t.Fatalf("gaps=%+v", gaps)
			}
			s.Dialect = spec.DialectMySQL
			if r.AppliesTo(s) {
				t.Fatal("matched MySQL")
			}
		})
	}
}

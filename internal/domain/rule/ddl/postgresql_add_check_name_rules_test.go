package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGAddCheckName(t *testing.T) {
	for _, tc := range []struct {
		name, gap string
		entries   []spec.PGConstraintState
		change    func(*spec.Statement)
		occupied  bool
		unmatched bool
	}{
		{name: "exact check", entries: []spec.PGConstraintState{{Name: " Exact Check ", Type: "c"}}, occupied: true},
		{name: "different kind", entries: []spec.PGConstraintState{{Name: " Exact Check ", Type: "p"}}, occupied: true},
		{name: "exact spaces and case", entries: []spec.PGConstraintState{{Name: "Exact Check", Type: "c"}, {Name: " exact check ", Type: "c"}}},
		{name: "known empty", entries: []spec.PGConstraintState{}},
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
			if len(findings) == 1 && (findings[0].Metadata["constraint"] != " Exact Check " || findings[0].Metadata["reason_code"] != "constraint_name_occupied" || len(findings[0].Metadata) != 5) {
				t.Fatalf("unexpected finding: %+v", findings[0])
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.gap != "") || len(gaps) == 1 && (len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.gap) {
				t.Fatalf("gaps=%+v", gaps)
			}
		})
	}
}

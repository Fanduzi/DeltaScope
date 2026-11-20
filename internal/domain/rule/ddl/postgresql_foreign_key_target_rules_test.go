package ddl

import (
	"context"
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGForeignKeyTargets(t *testing.T) {
	for _, tc := range []struct {
		name, reason, fact, column string
		change                     func(*spec.Statement, *spec.PGForeignKeySelection)
		equalityPosition           int
	}{
		{name: "exact identifiers"},
		{name: "mismatched scalar pair", reason: "foreign_key_column_types_incompatible", column: "A,B", equalityPosition: 1},
		{name: "unobserved scalar pair", fact: "foreign_key.pg_equality_compatibility", equalityPosition: -1},
		{name: "invalid scalar pair position", fact: "foreign_key.pg_equality_compatibility", equalityPosition: 2},
		{name: "action duplicate accepted", change: func(_ *spec.Statement, fk *spec.PGForeignKeySelection) { fk.DeleteSetColumns = []string{"A,B", "A,B"} }},
		{name: "action present outside key", reason: "foreign_key_delete_set_column_outside_key", column: " extra ", change: func(s *spec.Statement, fk *spec.PGForeignKeySelection) {
			fk.DeleteSetColumns = []string{" extra "}
			s.Metadata.TargetTable.PGColumns.Columns = append(s.Metadata.TargetTable.PGColumns.Columns, spec.PGColumnState{Name: " extra "})
		}},
		{name: "action missing before nonmember", reason: "foreign_key_delete_set_column_absent", column: "Missing", change: func(s *spec.Statement, fk *spec.PGForeignKeySelection) {
			fk.DeleteSetColumns = []string{" extra ", "Missing"}
			s.Metadata.TargetTable.PGColumns.Columns = append(s.Metadata.TargetTable.PGColumns.Columns, spec.PGColumnState{Name: " extra "})
		}},
		{name: "action system member", reason: "foreign_key_delete_set_system_column", column: "ctid", change: func(_ *spec.Statement, fk *spec.PGForeignKeySelection) { fk.DeleteSetColumns = []string{"ctid"} }},
		{name: "unobserved unique key", fact: "referenced_table.unique_key", change: func(s *spec.Statement, _ *spec.PGForeignKeySelection) { s.Metadata.PGForeignKeyKeys = nil }},
		{name: "empty directory", reason: "foreign_key_reference_unique_key_absent", change: func(s *spec.Statement, _ *spec.PGForeignKeySelection) { s.Metadata.PGForeignKeyKeys.Keys = nil }},
		{name: "case-distinct key", reason: "foreign_key_reference_unique_key_absent", change: func(s *spec.Statement, _ *spec.PGForeignKeySelection) {
			s.Metadata.PGForeignKeyKeys.Keys[0].Columns[0] = "c,d"
		}},
		{name: "deferred candidate", reason: "foreign_key_reference_key_deferrable", change: func(s *spec.Statement, _ *spec.PGForeignKeySelection) {
			s.Metadata.PGForeignKeyKeys.Keys[0].Immediate = false
		}},
		{name: "immediate candidate wins", change: func(s *spec.Statement, _ *spec.PGForeignKeySelection) {
			s.Metadata.PGForeignKeyKeys.Keys = append([]spec.PGForeignKeyKey{{Columns: []string{"C,D"}}}, s.Metadata.PGForeignKeyKeys.Keys...)
		}},
		{name: "duplicate reference", reason: "foreign_key_reference_column_duplicate", column: "C,D", change: func(_ *spec.Statement, fk *spec.PGForeignKeySelection) { fk.ReferencedColumns = []string{"C,D", "C,D"} }},
		{name: "arity counts source positions", reason: "foreign_key_column_count_mismatch", change: func(_ *spec.Statement, fk *spec.PGForeignKeySelection) { fk.Columns = []string{"A,B", "A,B"} }},
		{name: "omitted actual primary", change: func(_ *spec.Statement, fk *spec.PGForeignKeySelection) { fk.ReferencedColumns = nil }},
		{name: "standalone unique cannot replace primary", reason: "foreign_key_reference_primary_key_absent", change: func(s *spec.Statement, fk *spec.PGForeignKeySelection) {
			fk.ReferencedColumns = nil
			s.Metadata.PGForeignKeyKeys.Keys[0].Primary = false
		}},
		{name: "ambiguous primary", fact: "referenced_table.primary_key", change: func(s *spec.Statement, fk *spec.PGForeignKeySelection) {
			fk.ReferencedColumns = nil
			s.Metadata.PGForeignKeyKeys.Keys = append(s.Metadata.PGForeignKeyKeys.Keys, s.Metadata.PGForeignKeyKeys.Keys[0])
		}},
		{name: "missing reference before missing source", reason: "foreign_key_reference_absent", change: func(s *spec.Statement, fk *spec.PGForeignKeySelection) {
			s.Metadata.PGForeignKeyReference.Exists = false
			s.Metadata.PGForeignKeyReference.PGRelation.Kind = ""
			fk.Columns[0] = "missing"
		}},
		{name: "wrong kind", reason: "foreign_key_reference_wrong_kind", change: func(s *spec.Statement, _ *spec.PGForeignKeySelection) {
			s.Metadata.PGForeignKeyReference.PGRelation.Kind = "v"
		}},
		{name: "missing source", reason: "foreign_key_source_column_absent", column: "a,b", change: func(_ *spec.Statement, fk *spec.PGForeignKeySelection) { fk.Columns[0] = "a,b" }},
		{name: "missing source with omitted references", reason: "foreign_key_source_column_absent", column: "a,b", change: func(_ *spec.Statement, fk *spec.PGForeignKeySelection) {
			fk.Columns[0] = "a,b"
			fk.ReferencedColumns = nil
		}},
		{name: "system source", reason: "foreign_key_source_system_column", column: "ctid", change: func(_ *spec.Statement, fk *spec.PGForeignKeySelection) { fk.Columns[0] = "ctid" }},
		{name: "missing reference column", reason: "foreign_key_reference_column_absent", column: "c,d", change: func(_ *spec.Statement, fk *spec.PGForeignKeySelection) { fk.ReferencedColumns[0] = "c,d" }},
		{name: "system reference", reason: "foreign_key_reference_system_column", column: "ctid", change: func(_ *spec.Statement, fk *spec.PGForeignKeySelection) { fk.ReferencedColumns[0] = "ctid" }},
		{name: "omitted reference columns", fact: "referenced_table.primary_key", change: func(s *spec.Statement, fk *spec.PGForeignKeySelection) {
			fk.ReferencedColumns = nil
			s.Metadata.PGForeignKeyKeys = nil
		}},
		{name: "partitioned reference", fact: "referenced_table.pg_columns", change: func(s *spec.Statement, _ *spec.PGForeignKeySelection) {
			s.Metadata.PGForeignKeyReference.PGRelation.Kind = "p"
		}},
		{name: "nonordinary reference", fact: "referenced_table.pg_columns", change: func(s *spec.Statement, _ *spec.PGForeignKeySelection) {
			s.Metadata.PGForeignKeyReference.PGColumns.Ordinary = false
		}},
		{name: "unqualified reference", fact: "foreign_key.target_identity", change: func(_ *spec.Statement, fk *spec.PGForeignKeySelection) { fk.Reference.Schema = "" }},
		{name: "catalog-qualified reference", fact: "foreign_key.target_identity", change: func(_ *spec.Statement, fk *spec.PGForeignKeySelection) { fk.ReferenceCatalogQualified = true }},
		{name: "unobserved reference", fact: "referenced_table.pg_relation", change: func(s *spec.Statement, _ *spec.PGForeignKeySelection) { s.Metadata.PGForeignKeyReference = nil }},
		{name: "unobserved version", fact: "instance.version", change: func(s *spec.Statement, _ *spec.PGForeignKeySelection) { s.Metadata = nil }},
		{name: "guarded absent source", change: func(s *spec.Statement, _ *spec.PGForeignKeySelection) {
			s.DDL.PGTableIfExists = true
			s.Metadata.TargetTable = &spec.TableSnapshot{Schema: "app", PGRelation: spec.PGRelationState{Name: "t"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			catalog := func(name string) *spec.PGColumnCatalog {
				return &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: name}},
					SystemColumns: []spec.PGColumnState{{Name: "ctid", System: true}}}
			}
			fk := spec.PGForeignKeySelection{Reference: spec.Table{Schema: "app", Name: "parent"}, Columns: []string{"A,B"}, ReferencedColumns: []string{"C,D"}}
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL, DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
				Alter: []spec.Alter{{Action: "add_constraint", PGForeignKey: &fk, Options: map[string]string{"constraint_type": "foreign_key"}}},
			}, Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
				TargetTable: &spec.TableSnapshot{Schema: "app", Table: &spec.Table{Name: "t"}, Exists: true,
					PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: catalog("A,B")},
				PGForeignKeyKeys: &spec.PGForeignKeyKeys{Schema: "app", Table: "parent", Keys: []spec.PGForeignKeyKey{{Columns: []string{"C,D"}, Primary: true, Immediate: true}}},
				PGForeignKeyReference: &spec.TableSnapshot{Schema: "app", Exists: true,
					PGRelation: spec.PGRelationState{Name: "parent", Kind: "r"}, PGColumns: catalog("C,D")}}}
			if tc.change != nil {
				tc.change(&s, &fk)
			}
			if s.Metadata != nil && tc.equalityPosition >= 0 {
				s.Metadata.PGForeignKeyEquality = &spec.PGForeignKeyEquality{Target: spec.PGForeignKeyEqualityTarget{
					Source: *s.DDL.Table, Reference: fk.Reference, Columns: fk.Columns, ReferencedColumns: fk.ReferencedColumns},
					IncompatiblePosition: tc.equalityPosition}
			}
			r := pgForeignKeyTargetRule{level: rule.LevelBlocker}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") {
				t.Fatalf("findings=%+v err=%v", findings, err)
			}
			if len(findings) == 1 {
				metadata := map[string]any{"action": "add_constraint", "schema": "app", "table": "t", "reason_code": tc.reason}
				if tc.column != "" {
					metadata["column"] = tc.column
				}
				if !reflect.DeepEqual(findings[0].Metadata, metadata) {
					t.Fatalf("metadata=%+v", findings[0].Metadata)
				}
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.fact != "") || len(gaps) == 1 && (gaps[0].ReasonCode != "unproven_foreign_key_targets" || !reflect.DeepEqual(gaps[0].RequiredFacts, []string{tc.fact})) {
				t.Fatalf("gaps=%+v", gaps)
			}
		})
	}
}

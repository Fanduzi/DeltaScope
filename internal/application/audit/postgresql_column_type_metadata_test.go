package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnTypeDestinationBinding(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(*spec.Statement)
		stale   bool
		misbind bool
		read    bool
	}{
		{name: "fresh", read: true},
		{name: "wrong array binding", read: true, misbind: true},
		{name: "stale after DDL", stale: true},
		{name: "missing source", change: func(s *spec.Statement) { s.DDL.Alter[0].Name = "Missing" }},
		{name: "generated source with using", change: func(s *spec.Statement) {
			s.Metadata.TargetTable.PGColumns.Columns[0].Generated = "s"
			s.DDL.Alter[0].Options = map[string]string{"has_using": "true"}
		}},
		{name: "unqualified type", change: func(s *spec.Statement) { s.DDL.Alter[0].PGColumnType.Schema = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := spec.PGColumnTypeTarget{Schema: " Exact.Schema ", Name: " Exact Type ", Array: true}
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
					Alter: []spec.Alter{{Action: "set_data_type", Name: "Id", PGColumnType: target}}},
				Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
					TargetTable: &spec.TableSnapshot{Schema: "app", Table: &spec.Table{Name: "t"}, Exists: true,
						PGRelation: spec.PGRelationState{Name: "t", Kind: "r"},
						PGColumns:  &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "Id"}}}}}}
			if tc.change != nil {
				tc.change(&s)
			}
			statements := []spec.Statement{s}
			if tc.stale {
				statements = append([]spec.Statement{{Kind: spec.KindDDL}}, statements...)
			}
			invalidateStalePGTargetCatalogs(statements, nil)
			facts := &spec.PGColumnTypeState{Target: target}
			if tc.misbind {
				facts.Target.Array = false
			}
			called := false
			provider := replicaLookupProvider{lookup: func(req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
				called = true
				if req.Type != "column_type_destination" || req.Schema != target.Schema || req.Name != target.Name || req.Qualifiers["array"] != "true" {
					t.Fatalf("lookup=%+v", req)
				}
				return &spec.ObjectSnapshot{PGColumnType: facts}, nil
			}}
			if err := enrichPGColumnTypes(context.Background(), &MetadataRequest{Provider: provider}, statements); err != nil {
				t.Fatal(err)
			}
			got := statements[len(statements)-1].Metadata.PGColumnType
			if called != tc.read || (got != nil) != (tc.read && !tc.misbind) || got == facts {
				t.Fatalf("lookup=%t, facts=%+v", called, got)
			}
		})
	}
}

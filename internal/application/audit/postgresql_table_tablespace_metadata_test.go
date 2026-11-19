package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGTableTablespaceFreshBinding(t *testing.T) {
	for _, tc := range []struct {
		name     string
		stale    bool
		misbound bool
	}{
		{name: "fresh"}, {name: "stale", stale: true}, {name: "misbound", misbound: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := &spec.PGTableTablespace{Schema: "app", Table: "t", Name: " Mixed Space ", Exists: true}
			if tc.misbound {
				facts.Name = "mixed space"
			}
			statements := []spec.Statement{{
				Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
					Alter: []spec.Alter{{Action: "set_tablespace", Options: map[string]string{"tablespace": " Mixed Space "}}}},
				Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
					TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
						PGColumns: &spec.PGColumnCatalog{Ordinary: true}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}}},
			}}
			if tc.stale {
				statements[0].Metadata.TargetTable.PGRelation = spec.PGRelationState{}
			}
			called := false
			provider := replicaLookupProvider{lookup: func(req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
				called = true
				if req.Type != "table_tablespace" || req.Schema != "app" || req.Name != " Mixed Space " || req.Qualifiers["table"] != "t" {
					t.Fatalf("lookup=%+v", req)
				}
				return &spec.ObjectSnapshot{PGTableTablespace: facts}, nil
			}}
			if err := enrichPGTableTablespaces(context.Background(), &MetadataRequest{Provider: provider}, statements); err != nil {
				t.Fatal(err)
			}
			got := statements[0].Metadata.PGTableTablespace
			if called == tc.stale || (got == nil) != (tc.stale || tc.misbound) || got == facts {
				t.Fatalf("lookup=%v facts=%+v", called, got)
			}
		})
	}
}

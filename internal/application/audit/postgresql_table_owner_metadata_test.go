package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGTableOwnerFreshBinding(t *testing.T) {
	for _, tc := range []struct {
		name     string
		stale    bool
		misbound bool
	}{
		{name: "fresh"}, {name: "stale", stale: true}, {name: "misbound", misbound: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := spec.PGTableOwnerTarget{Kind: "named", Name: " Exact Owner "}
			facts := &spec.PGTableOwnerState{Schema: "app", Table: "t", Target: target, Exists: true}
			if tc.misbound {
				facts.Target.Name = "exact owner"
			}
			statements := []spec.Statement{{
				Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
					Alter: []spec.Alter{{Action: "change_owner", PGTableOwner: target}}},
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
				if req.Type != "table_owner" || req.Qualifiers["kind"] != "named" || req.Schema != "app" || req.Name != " Exact Owner " || req.Qualifiers["table"] != "t" {
					t.Fatalf("lookup=%+v", req)
				}
				return &spec.ObjectSnapshot{PGTableOwner: facts}, nil
			}}
			if err := enrichPGTableOwners(context.Background(), &MetadataRequest{Provider: provider}, statements); err != nil {
				t.Fatal(err)
			}
			got := statements[0].Metadata.PGTableOwner
			if called == tc.stale || (got == nil) != (tc.stale || tc.misbound) || got == facts {
				t.Fatalf("lookup=%v facts=%+v", called, got)
			}
		})
	}
}

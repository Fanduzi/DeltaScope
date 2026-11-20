package audit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGDropColumnDependencyMetadataBinding(t *testing.T) {
	const name = " Exact Target "
	failure := errors.New("provider-private-catalog-error")
	for _, tc := range []struct {
		name             string
		mutate           func(*spec.Statement)
		mismatch         bool
		providerError    error
		lookup, attached bool
	}{
		{name: "exact", lookup: true, attached: true},
		{name: "misbound observation", mismatch: true, lookup: true},
		{name: "provider failure", providerError: failure, lookup: true},
		{name: "cascade", mutate: func(s *spec.Statement) { s.DDL.Alter[0].Options["cascade"] = "true" }},
		{name: "stale", mutate: func(s *spec.Statement) { s.Metadata.TargetTable.PGRelation = spec.PGRelationState{} }},
		{name: "version unknown", mutate: func(s *spec.Statement) { s.Metadata.Instance.Version = "PostgreSQL 18.0" }},
		{name: "unqualified", mutate: func(s *spec.Statement) { s.DDL.Table.Schema = "" }},
		{name: "table absent", mutate: func(s *spec.Statement) { s.Metadata.TargetTable.Exists = false }},
		{name: "column absent", mutate: func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Columns = []spec.PGColumnState{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
					Alter: []spec.Alter{{Action: "drop_column", Name: name, Options: map[string]string{"cascade": "false", "if_exists": "true"}}}},
				Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
					TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
						PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: name}}}}}}
			if tc.mutate != nil {
				tc.mutate(&s)
			}
			fact := &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: name, Kind: "view"}
			if tc.mismatch {
				fact.Column = " exact target "
			}
			called := false
			provider := replicaLookupProvider{lookup: func(req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
				called = true
				if req.Type != "drop_column_dependency" || req.Schema != "app" || req.Name != name || req.Qualifiers["table"] != "t" {
					t.Fatalf("lookup=%+v", req)
				}
				return &spec.ObjectSnapshot{PGDropColumnDependency: fact}, tc.providerError
			}}
			err := enrichPGDropColumnDependencies(context.Background(), &MetadataRequest{Provider: provider}, []spec.Statement{s})
			if !errors.Is(err, tc.providerError) || err != nil && strings.Contains(err.Error(), failure.Error()) {
				t.Fatalf("error=%v", err)
			}
			if got := s.Metadata.PGDropColumnDependency; called != tc.lookup || (got != nil) != tc.attached || got == fact || got != nil && *got != *fact {
				t.Fatalf("called=%v fact=%+v", called, got)
			}
			encoded, err := json.Marshal(s.Metadata)
			if err != nil || strings.Contains(string(encoded), "PGDropColumnDependency") {
				t.Fatalf("private dependency encoded: %s (%v)", encoded, err)
			}
		})
	}
}

package audit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type fkReferenceTestProvider struct {
	*fakeMetadataProvider
	lookup func(schema, table string) (*spec.TableSnapshot, error)
}

func (p fkReferenceTestProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	return p.lookup(schema, table)
}

func TestPGForeignKeyReferenceBinding(t *testing.T) {
	readErr := errors.New("private provider detail")
	for _, tc := range []struct {
		name        string
		calls       int
		bound       bool
		mismatch    bool
		cached      bool
		catalog     bool
		previousDDL bool
		failure     error
	}{
		{name: "exact", calls: 1, bound: true},
		{name: "cached", cached: true, bound: true},
		{name: "wrong identity", calls: 1, mismatch: true},
		{name: "three-part reference", catalog: true},
		{name: "prior DDL", previousDDL: true},
		{name: "lookup error", calls: 1, failure: readErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref := spec.Table{Schema: " Ref.Schema ", Name: " Ref "}
			statement := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL, DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
				Alter: []spec.Alter{{Action: "add_constraint", Options: map[string]string{"constraint_type": "foreign_key"},
					PGForeignKey: &spec.PGForeignKeySelection{Reference: ref, ReferenceCatalogQualified: tc.catalog, Columns: []string{"id"}}}},
			}, Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
				TargetTable: &spec.TableSnapshot{Schema: "app", Table: &spec.Table{Name: "t"}, Exists: true,
					PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
			statements := []spec.Statement{statement}
			if tc.previousDDL {
				statements = append([]spec.Statement{{Kind: spec.KindDDL, DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "other"}}}}, statements...)
			}
			prefix := invalidateStalePGTargetCatalogs(statements, nil)
			snapshot := &spec.TableSnapshot{Schema: ref.Schema, Table: &spec.Table{Name: ref.Name}, Exists: true,
				PGRelation: spec.PGRelationState{Name: ref.Name, Kind: "r"},
				PGColumns:  &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "private_ref"}}}}
			if tc.mismatch {
				snapshot.PGRelation.Name = "REF"
			}
			snapshots := make(map[[2]string]*spec.TableSnapshot)
			if tc.cached {
				snapshots[[2]string{ref.Schema, ref.Name}] = snapshot
			}
			calls := 0
			provider := fkReferenceTestProvider{lookup: func(schema, table string) (*spec.TableSnapshot, error) {
				calls++
				if schema != ref.Schema || table != ref.Name {
					t.Fatalf("unbound lookup: %q.%q", schema, table)
				}
				return snapshot, tc.failure
			}}
			err := enrichPGForeignKeyReferences(context.Background(), &MetadataRequest{Provider: provider}, statements[:prefix], snapshots)
			if !errors.Is(err, tc.failure) || err != nil && strings.Contains(err.Error(), readErr.Error()) {
				t.Fatalf("lookup error = %v", err)
			}
			got := statements[len(statements)-1].Metadata.PGForeignKeyReference
			if calls != tc.calls || (got != nil) != tc.bound || got == snapshot {
				t.Fatalf("calls=%d reference=%+v", calls, got)
			}
			if got != nil {
				got.PGColumns.Columns[0].Name = "changed"
				if snapshot.PGColumns.Columns[0].Name != "private_ref" {
					t.Fatal("bound reference aliases provider memory")
				}
			}
			b, err := json.Marshal(statements[len(statements)-1].Metadata)
			if err != nil || strings.Contains(string(b), "PGForeignKeyReference") || strings.Contains(string(b), " Ref ") {
				t.Fatalf("private reference leaked: %s (%v)", b, err)
			}
		})
	}
}

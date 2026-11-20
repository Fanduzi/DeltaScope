package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolveForeignKeyScalarEquality(t *testing.T) {
	for _, tc := range []struct {
		name     string
		position driver.Value
	}{
		{name: "unobserved"},
		{name: "compatible", position: int64(0)},
		{name: "incompatible second pair", position: int64(2)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := &spec.PGForeignKeyEqualityTarget{
				Source:    spec.Table{Schema: "app", Name: " Exact Source "},
				Reference: spec.Table{Schema: " Ref.Schema ", Name: "target"},
				Columns:   []string{"a,b", " C "}, ReferencedColumns: []string{"ref_b", "ref_a"},
			}
			db, log := openTestDB(t, map[string]testQueryResult{"from comparison": {
				columns: []string{"position"}, rows: [][]driver.Value{{tc.position}},
			}})
			defer db.Close()
			result, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL,
				spec.ObjectLookupRequest{Type: "foreign_key_scalar_equality", PGForeignKeyEqualityTarget: target})
			if err != nil || (result == nil) != (tc.position == nil) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			queries := log.Queries()
			if len(queries) != 1 || strings.Contains(queries[0], target.Source.Name) || strings.Contains(queries[0], target.Columns[0]) {
				t.Fatalf("catalog query was not parameterized: %v", queries)
			}
			if result == nil {
				return
			}
			fact := result.PGForeignKeyEquality
			if fact == nil || !fact.Target.Matches(*target) || fact.IncompatiblePosition != int(tc.position.(int64)) {
				t.Fatalf("observation=%+v", fact)
			}
			target.Columns[0] = "changed"
			target.ReferencedColumns[0] = "changed"
			if fact.Target.Columns[0] != "a,b" || fact.Target.ReferencedColumns[0] != "ref_b" {
				t.Fatal("provider result aliases the requested column lists")
			}
			encoded, err := json.Marshal(result)
			if err != nil || strings.Contains(string(encoded), "PGForeignKeyEquality") || strings.Contains(string(encoded), " Ref.Schema ") {
				t.Fatalf("private observation: %s (%v)", encoded, err)
			}
		})
	}
}

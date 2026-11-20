package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolvePGColumnTraversal(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows [][]driver.Value
		want *spec.PGColumnTraversal
	}{
		{name: "unobserved attribute"},
		{name: "known local", rows: [][]driver.Value{{false, false, false}}, want: &spec.PGColumnTraversal{}},
		{name: "inherited attribute", rows: [][]driver.Value{{true, false, false}}, want: &spec.PGColumnTraversal{Inherited: true}},
		{name: "attached partitioned key", rows: [][]driver.Value{{false, true, true}}, want: &spec.PGColumnTraversal{DirectPartitionKey: true, AttachedChildren: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, log := openTestDB(t, map[string]testQueryResult{"a.attinhcount": {columns: []string{"inherited", "partition_key", "children"}, rows: tc.rows}})
			defer db.Close()
			req := spec.ObjectLookupRequest{Type: "column_traversal", Schema: " Exact Schema ", Name: " Exact Column ", Qualifiers: map[string]string{"table": " Exact Table "}}
			result, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
			if err != nil || (result == nil) != (tc.want == nil) {
				t.Fatalf("fact=%+v err=%v", result, err)
			}
			query := log.Queries()
			if len(query) != 1 || strings.Contains(query[0], req.Name) || !strings.Contains(query[0], "not h.inhdetachpending") || !strings.Contains(query[0], "any(pt.partattrs)") {
				t.Fatal(query)
			}
			if result == nil {
				return
			}
			want := *tc.want
			want.Schema, want.Table, want.Column = req.Schema, req.Qualifiers["table"], req.Name
			if result.PGColumnTraversal == nil || *result.PGColumnTraversal != want {
				t.Fatalf("fact=%+v want=%+v", result.PGColumnTraversal, want)
			}
			encoded, err := json.Marshal(result)
			if err != nil || strings.Contains(string(encoded), "PGColumnTraversal") || strings.Contains(string(encoded), req.Name) {
				t.Fatalf("private result=%s err=%v", encoded, err)
			}
		})
	}
}

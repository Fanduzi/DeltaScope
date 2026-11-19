package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolvePGColumnType(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result []driver.Value
		want   spec.PGColumnTypeState
	}{
		{name: "schema absent", result: []driver.Value{false, false, false, false, false}},
		{name: "type absent", result: []driver.Value{true, false, false, false, false}, want: spec.PGColumnTypeState{SchemaExists: true}},
		{name: "array absent", result: []driver.Value{true, true, false, false, false}, want: spec.PGColumnTypeState{SchemaExists: true, BaseExists: true}},
		{name: "array resolved", result: []driver.Value{true, true, true, true, false}, want: spec.PGColumnTypeState{SchemaExists: true, BaseExists: true, Resolved: true, Defined: true}},
		{name: "pseudo array", result: []driver.Value{true, true, true, true, true}, want: spec.PGColumnTypeState{SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, Pseudo: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, log := openTestDB(t, map[string]testQueryResult{"resolved.typsubscript": {columns: []string{"schema", "base", "resolved", "defined", "pseudo"}, rows: [][]driver.Value{tc.result}}})
			defer db.Close()
			target := spec.PGColumnTypeTarget{Schema: " Exact.Schema ", Name: " Exact Type ", Array: true}
			request := spec.ObjectLookupRequest{Type: "column_type_destination", Schema: target.Schema, Name: target.Name, Qualifiers: map[string]string{"array": "true"}}
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, request)
			tc.want.Target = target
			if err != nil || got == nil || got.PGColumnType == nil || *got.PGColumnType != tc.want {
				t.Fatalf("object=%+v err=%v", got, err)
			}
			queries := log.Queries()
			if len(queries) != 1 || !strings.Contains(queries[0], "OPERATOR(pg_catalog.=) resolved.typelem") || strings.Contains(queries[0], target.Name) || strings.Contains(queries[0], target.Schema) {
				t.Fatalf("query=%v", queries)
			}
			b, err := json.Marshal(got)
			if err != nil || strings.Contains(string(b), target.Name) || strings.Contains(string(b), "PGColumnType") {
				t.Fatalf("private facts leaked: %s %v", b, err)
			}
		})
	}
}

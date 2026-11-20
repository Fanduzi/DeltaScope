package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolveDropColumnDependency(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows [][]driver.Value
		kind string
	}{
		{name: "view", rows: [][]driver.Value{{"view"}}, kind: "view"},
		{name: "foreign key", rows: [][]driver.Value{{"foreign_key"}}, kind: "foreign_key"},
		{name: "observed empty", rows: [][]driver.Value{{""}}},
		{name: "unobserved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, log := openTestDB(t, map[string]testQueryResult{"join pg_catalog.pg_rewrite": {columns: []string{"kind"}, rows: tc.rows}})
			defer db.Close()
			req := spec.ObjectLookupRequest{Type: "drop_column_dependency", Schema: " Exact.Schema ", Name: " Exact Column ", Qualifiers: map[string]string{"table": " Exact Table "}}
			result, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
			if err != nil || (result != nil) != (tc.rows != nil) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			queries := log.Queries()
			if len(queries) != 1 || strings.Contains(queries[0], req.Name) || !strings.Contains(queries[0], "d.refobjsubid OPERATOR(pg_catalog.=) a.attnum") || !strings.Contains(queries[0], "any(fk.conkey)") {
				t.Fatalf("query=%v", queries)
			}
			if result == nil {
				return
			}
			want := spec.PGDropColumnDependency{Schema: req.Schema, Table: req.Qualifiers["table"], Column: req.Name, Kind: tc.kind}
			if got := result.PGDropColumnDependency; got == nil || *got != want {
				t.Fatalf("fact=%+v want=%+v", got, want)
			}
			encoded, err := json.Marshal(result)
			if err != nil || strings.Contains(string(encoded), req.Name) || strings.Contains(string(encoded), "PGDropColumnDependency") {
				t.Fatalf("private dependency encoded: %s (%v)", encoded, err)
			}
		})
	}
}

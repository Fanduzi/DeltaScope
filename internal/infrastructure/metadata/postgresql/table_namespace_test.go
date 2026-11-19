package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolveTableNamespace(t *testing.T) {
	failure := errors.New("read unavailable")
	for _, tc := range []struct {
		name, action string
		row          []driver.Value
		want         spec.PGTableNamespace
		err          error
	}{
		{name: "outside table scope", action: "rename_table"},
		{name: "occupied relation", action: "rename_table", row: []driver.Value{true, true, true, false}, want: spec.PGTableNamespace{SchemaExists: true, SameSchema: true, RelationConflict: true, RowTypeConflictKnown: true}},
		{name: "occupied type", action: "rename_table", row: []driver.Value{true, true, false, true}, want: spec.PGTableNamespace{SchemaExists: true, SameSchema: true, RowTypeConflict: true, RowTypeConflictKnown: true}},
		{name: "relocatable array", action: "rename_table", row: []driver.Value{true, true, false, false}, want: spec.PGTableNamespace{SchemaExists: true, SameSchema: true, RowTypeConflictKnown: true}},
		{name: "unobserved array element", action: "rename_table", row: []driver.Value{true, true, false, nil}, want: spec.PGTableNamespace{SchemaExists: true, SameSchema: true}},
		{name: "missing schema", action: "set_schema", row: []driver.Value{false, false, false, false}, want: spec.PGTableNamespace{RowTypeConflictKnown: true}},
		{name: "same schema", action: "set_schema", row: []driver.Value{true, true, true, true}, want: spec.PGTableNamespace{SchemaExists: true, SameSchema: true, RelationConflict: true, RowTypeConflict: true, RowTypeConflictKnown: true}},
		{name: "query error", action: "set_schema", err: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := testQueryResult{columns: []string{"schema_exists", "same_schema", "relation_conflict", "row_type_conflict"}, err: tc.err}
			if tc.row != nil {
				result.rows = [][]driver.Value{tc.row}
			}
			db, log := openTestDB(t, map[string]testQueryResult{"elem.typarray": result})
			defer db.Close()
			req := spec.ObjectLookupRequest{Schema: "app", Name: " Exact Destination ", Type: "table_namespace", Qualifiers: map[string]string{"table": "t", "action": tc.action}}
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
			if !errors.Is(err, tc.err) || (got != nil) != (tc.row != nil) {
				t.Fatalf("observation=%+v err=%v", got, err)
			}
			if got == nil {
				return
			}
			want := tc.want
			want.Schema, want.Table, want.Action, want.Destination = req.Schema, req.Qualifiers["table"], tc.action, req.Name
			if got.PGTableNamespace == nil || *got.PGTableNamespace != want {
				t.Fatalf("got=%+v want=%+v", got.PGTableNamespace, want)
			}
			q := log.Queries()
			if len(q) != 1 || !strings.Contains(q[0], "not typ.typisdefined") ||
				!strings.Contains(q[0], "typ.typsubscript OPERATOR(pg_catalog.<>) 6179::pg_catalog.oid") ||
				!strings.Contains(q[0], "else elem.typarray OPERATOR(pg_catalog.<>) typ.oid") || strings.Contains(q[0], req.Name) {
				t.Fatalf("unexpected query: %v", q)
			}
			b, err := json.Marshal(got)
			if err != nil || strings.Contains(string(b), req.Name) || strings.Contains(string(b), "PGTableNamespace") {
				t.Fatalf("private observation leaked: %s %v", b, err)
			}
		})
	}
}

func TestResolveTableNamespaceWithholdsTemporaryAlias(t *testing.T) {
	db, log := openTestDB(t, nil)
	defer db.Close()
	req := spec.ObjectLookupRequest{Schema: "app", Name: "pg_temp", Type: "table_namespace", Qualifiers: map[string]string{"table": "t", "action": "set_schema"}}
	got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
	if got != nil || err != nil || len(log.Queries()) != 0 {
		t.Fatalf("observation=%+v err=%v queries=%v", got, err, log.Queries())
	}
}

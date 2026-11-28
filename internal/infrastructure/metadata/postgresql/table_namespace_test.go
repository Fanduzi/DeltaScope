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
		name, action, kind string
		row                []driver.Value
		want               spec.PGTableNamespace
		err                error
	}{
		{name: "outside table scope", action: "rename_table"},
		{name: "partition root collision", action: "rename_table", kind: "p", row: []driver.Value{true, true, true, false}, want: spec.PGTableNamespace{SourceKindKnown: true, SourceKind: "p", SchemaExists: true, SameSchema: true, RelationConflict: true, RowTypeConflictKnown: true}},
		{name: "partition root schema conflict", action: "set_schema", kind: "p", row: []driver.Value{true, false, false, false, true, true, false}, want: spec.PGTableNamespace{SourceKindKnown: true, SourceKind: "p", SchemaExists: true, RowTypeConflictKnown: true, IndexConflict: true, OwnedSequenceConflict: true, ArrayTypeConflictKnown: true}},
		{name: "occupied relation", action: "rename_table", row: []driver.Value{true, true, true, false}, want: spec.PGTableNamespace{SchemaExists: true, SameSchema: true, RelationConflict: true, RowTypeConflictKnown: true}},
		{name: "occupied type", action: "rename_table", row: []driver.Value{true, true, false, true}, want: spec.PGTableNamespace{SchemaExists: true, SameSchema: true, RowTypeConflict: true, RowTypeConflictKnown: true}},
		{name: "relocatable array", action: "rename_table", row: []driver.Value{true, true, false, false}, want: spec.PGTableNamespace{SchemaExists: true, SameSchema: true, RowTypeConflictKnown: true}},
		{name: "unobserved array element", action: "rename_table", row: []driver.Value{true, true, false, nil}, want: spec.PGTableNamespace{SchemaExists: true, SameSchema: true}},
		{name: "missing schema", action: "set_schema", row: []driver.Value{false, false, false, false}, want: spec.PGTableNamespace{RowTypeConflictKnown: true}},
		{name: "same schema", action: "set_schema", row: []driver.Value{true, true, true, true}, want: spec.PGTableNamespace{SchemaExists: true, SameSchema: true, RelationConflict: true, RowTypeConflict: true, RowTypeConflictKnown: true}},
		{name: "companion conflicts", action: "set_schema", row: []driver.Value{true, false, false, false, true, true, true}, want: spec.PGTableNamespace{SchemaExists: true, RowTypeConflictKnown: true, IndexConflict: true, OwnedSequenceConflict: true, ArrayTypeConflict: true, ArrayTypeConflictKnown: true}},
		{name: "query error", action: "set_schema", err: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := testQueryResult{columns: []string{"schema_exists", "same_schema", "relation_conflict", "row_type_conflict", "index_conflict", "sequence_conflict", "array_type_conflict"}, err: tc.err}
			if tc.row != nil {
				if len(tc.row) == 4 {
					tc.row = append(tc.row, false, false, nil)
				}
				result.rows = [][]driver.Value{tc.row}
			}
			db, log := openTestDB(t, map[string]testQueryResult{"elem.typarray": result})
			defer db.Close()
			req := spec.ObjectLookupRequest{Schema: "app", Name: " Exact Destination ", Type: "table_namespace", Qualifiers: map[string]string{"table": "t", "action": tc.action, "kind": tc.kind}}
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

func TestResolveViewNamespaceChecksSourceBeforeDestination(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		reads      int
	}{
		{name: "missing", reads: 1},
		{name: "wrong kind", kind: "r", reads: 1},
		{name: "matching materialized view", kind: "m", reads: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := testQueryResult{columns: []string{"relkind"}}
			if tc.kind != "" {
				source.rows = [][]driver.Value{{tc.kind}}
			}
			db, log := openTestDB(t, map[string]testQueryResult{
				"select c.relkind::pg_catalog.text": source,
				"elem.typarray": {columns: []string{"schema_exists", "same_schema", "relation_conflict", "row_type_conflict", "index_conflict", "sequence_conflict", "array_type_conflict"},
					rows: [][]driver.Value{{true, false, false, false, true, false, false}}},
			})
			defer db.Close()
			request := spec.ObjectLookupRequest{Type: "view_namespace", Schema: "app", Name: " Destination ",
				Qualifiers: map[string]string{"table": " Source View ", "action": "set_schema", "kind": "m"}}
			got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, request)
			if err != nil || got == nil || got.PGTableNamespace == nil {
				t.Fatalf("snapshot=%+v err=%v", got, err)
			}
			state := got.PGTableNamespace
			if !state.SourceKindKnown || state.SourceKind != tc.kind || state.IndexConflict != (tc.kind == "m") || len(log.Queries()) != tc.reads {
				t.Fatalf("state=%+v queries=%v", state, log.Queries())
			}
		})
	}
}

func TestResolveTableNamespaceWithholdsTemporaryAlias(t *testing.T) {
	db, log := openTestDB(t, nil)
	defer db.Close()
	for _, name := range []string{"pg_temp", "pg_temp_3", "pg_toast_temp_3", "pg_toast"} {
		req := spec.ObjectLookupRequest{Schema: "app", Name: name, Type: "table_namespace", Qualifiers: map[string]string{"table": "t", "action": "set_schema", "kind": "p"}}
		got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, req)
		if got != nil || err != nil || len(log.Queries()) != 0 {
			t.Fatalf("destination=%q observation=%+v err=%v queries=%v", name, got, err, log.Queries())
		}
	}
}

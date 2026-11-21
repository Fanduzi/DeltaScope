package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestResolveViewDefaultBindsQualifiedUserColumn(t *testing.T) {
	db, log := openTestDB(t, map[string]testQueryResult{
		"a.attnum is not null": {columns: []string{"kind", "column_exists"}, rows: [][]driver.Value{{"v", true}}},
	})
	defer db.Close()
	request := spec.ObjectLookupRequest{Type: "view_column_default", Schema: " Exact Schema ", Name: " Exact View ", Qualifiers: map[string]string{"column": " Exact Column "}}
	got, err := NewProvider(db).ResolveObject(context.Background(), spec.DialectPostgreSQL, request)
	want := spec.PGViewDefaultState{Schema: request.Schema, View: request.Name, Column: request.Qualifiers["column"], Kind: "v", ColumnExists: true}
	if err != nil || got == nil || got.PGViewDefault == nil || *got.PGViewDefault != want {
		t.Fatalf("snapshot=%+v want=%+v err=%v", got, want, err)
	}
	queries := log.Queries()
	if len(queries) != 1 || !strings.Contains(queries[0], "a.attnum OPERATOR(pg_catalog.>) 0 and not a.attisdropped") || strings.Contains(queries[0], want.Column) {
		t.Fatalf("queries=%v", queries)
	}
	encoded, err := json.Marshal(got)
	if err != nil || strings.Contains(string(encoded), "PGViewDefault") || strings.Contains(string(encoded), want.Column) {
		t.Fatalf("public snapshot=%s (%v)", encoded, err)
	}
}

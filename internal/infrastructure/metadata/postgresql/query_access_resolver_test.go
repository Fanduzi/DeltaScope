//go:build postgresql

// Package postgresqlmeta verifies PostgreSQL Query Access resolver behavior.
// input: synthetic pg_catalog rows via the package test driver
// output: equivalent DB/Conn relation metadata, errors, ordering, and fail-closed behavior
// pos: parameterized contract and adapter-specific unit coverage
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
	"testing"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
)

type queryAccessResolverFactory struct {
	name string
	new  func(t *testing.T, db any) appqa.SchemaResolver
}

func TestQueryAccessResolvers_ResolveRelationContract(t *testing.T) {
	factories := []queryAccessResolverFactory{
		{
			name: "conn",
			new: func(t *testing.T, handle any) appqa.SchemaResolver {
				conn, err := handle.(*sql.DB).Conn(context.Background())
				if err != nil {
					t.Fatalf("db.Conn: %v", err)
				}
				t.Cleanup(func() { _ = conn.Close() })
				resolver, err := NewQueryAccessConnResolver(conn)
				if err != nil {
					t.Fatalf("NewQueryAccessConnResolver: %v", err)
				}
				return resolver
			},
		},
	}

	cases := []struct {
		name         string
		results      map[string]testQueryResult
		want         appqa.RelationSchema
		wantErrExact string
		wantLogs     []string
	}{
		{
			name:    "base table",
			results: relationResults("r", [][]driver.Value{{"id", int64(1)}, {"name", int64(2)}}),
			want: appqa.RelationSchema{
				Schema: "app", Name: "orders", Kind: "table",
				Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}, {Name: "name", Ordinal: 2}},
			},
			wantLogs: []string{"select c.relkind", "select a.attname"},
		},
		{
			name:    "partitioned table",
			results: relationResults("p", [][]driver.Value{{"id", int64(1)}}),
			want: appqa.RelationSchema{
				Schema: "app", Name: "orders", Kind: "table",
				Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
			},
			wantLogs: []string{"select c.relkind", "select a.attname"},
		},
		{
			name:    "view",
			results: relationResults("v", [][]driver.Value{{"id", int64(1)}}),
			want: appqa.RelationSchema{
				Schema: "app", Name: "summary", Kind: "view", IsView: true,
				Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
			},
			wantLogs: []string{"select c.relkind", "select a.attname"},
		},
		{
			name:    "materialized view",
			results: relationResults("m", [][]driver.Value{{"id", int64(1)}}),
			want: appqa.RelationSchema{
				Schema: "app", Name: "summary", Kind: "view", IsView: true,
				Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
			},
			wantLogs: []string{"select c.relkind", "select a.attname"},
		},
		{
			name: "missing relation",
			results: map[string]testQueryResult{
				"select c.relkind": {columns: []string{"relkind"}, rows: nil},
			},
			wantErrExact: "relation app.missing not found",
			wantLogs:     []string{"select c.relkind"},
		},
		{
			name:         "foreign table fails closed",
			results:      relationResults("f", [][]driver.Value{{"id", int64(1)}}),
			wantErrExact: "relation app.remote_orders not found",
			wantLogs:     []string{"select c.relkind"},
		},
		{
			name: "relation query error",
			results: map[string]testQueryResult{
				"select c.relkind": {err: errors.New("relation query failed")},
			},
			wantErrExact: "query relation type for app.relation_error: relation query failed",
			wantLogs:     []string{"select c.relkind"},
		},
		{
			name: "relation scan error",
			results: map[string]testQueryResult{
				"select c.relkind": {columns: []string{}, rows: [][]driver.Value{{int64(7)}}},
			},
			wantErrExact: "query relation type for app.relation_scan_error: sql: expected 0 destination arguments in Scan, not 1",
			wantLogs:     []string{"select c.relkind"},
		},
		{
			name: "relation iteration error",
			results: map[string]testQueryResult{
				"select c.relkind": {columns: []string{"relkind"}, rowErr: errors.New("relation iteration failed")},
			},
			wantErrExact: "query relation type for app.relation_iteration_error: relation iteration failed",
			wantLogs:     []string{"select c.relkind"},
		},

		{
			name: "cancellation",
			results: map[string]testQueryResult{
				"select c.relkind": {columns: []string{"relkind"}, rows: [][]driver.Value{{"r"}}},
			},
			wantErrExact: "resolve cancelled: context canceled",
			wantLogs:     nil,
		}, {
			name: "column query error",
			results: map[string]testQueryResult{
				"select c.relkind": {columns: []string{"relkind"}, rows: [][]driver.Value{{"r"}}},
				"select a.attname": {err: errors.New("column query failed")},
			},
			wantErrExact: "query columns for app.orders: column query failed",
			wantLogs:     []string{"select c.relkind", "select a.attname"},
		},

		{
			name:         "column scan error",
			results:      relationResults("r", [][]driver.Value{{"id", "not-an-integer"}}),
			wantErrExact: "scan column for app.orders: sql: Scan error on column index 1, name \"ordinal_position\": converting driver.Value type string (\"not-an-integer\") to a int: invalid syntax",
			wantLogs:     []string{"select c.relkind", "select a.attname"},
		},
		{
			name: "column iteration error",
			results: map[string]testQueryResult{
				"select c.relkind": {columns: []string{"relkind"}, rows: [][]driver.Value{{"r"}}},
				"select a.attname": {columns: []string{"column_name", "ordinal_position"}, rowErr: errors.New("column iteration failed")},
			},
			wantErrExact: "iterate columns for app.orders: column iteration failed",
			wantLogs:     []string{"select c.relkind", "select a.attname"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, factory := range factories {
				t.Run(factory.name, func(t *testing.T) {
					db, queryLog := openTestDB(t, tc.results)
					defer db.Close()
					resolver := factory.new(t, db)

					ctx := context.Background()
					if tc.name == "cancellation" {
						var cancel context.CancelFunc
						ctx, cancel = context.WithCancel(ctx)
						cancel()
					}
					got, err := resolver.ResolveRelation(ctx, "postgresql", "app", relationName(tc.name))
					if tc.wantErrExact != "" {
						if err == nil || err.Error() != tc.wantErrExact {
							t.Fatalf("error=%v, want exact %q", err, tc.wantErrExact)
						}
					} else if err != nil {
						t.Fatalf("ResolveRelation: %v", err)
					} else if !reflect.DeepEqual(got, tc.want) {
						t.Fatalf("result=%+v, want %+v", got, tc.want)
					}
					assertQueryOrder(t, queryLog.Queries(), tc.wantLogs)
				})
			}
		})
	}
}

func relationResults(relkind string, columns [][]driver.Value) map[string]testQueryResult {
	return map[string]testQueryResult{
		"select c.relkind": {columns: []string{"relkind"}, rows: [][]driver.Value{{relkind}}},
		"select a.attname": {columns: []string{"column_name", "ordinal_position"}, rows: columns},
	}
}

func relationName(name string) string {
	if name == "missing relation" {
		return "missing"
	}
	if name == "foreign table fails closed" {
		return "remote_orders"
	}
	return map[string]string{
		"base table": "orders", "partitioned table": "orders", "view": "summary",
		"materialized view": "summary", "relation query error": "relation_error", "relation scan error": "relation_scan_error", "relation iteration error": "relation_iteration_error",
		"column query error": "orders", "column scan error": "orders", "column iteration error": "orders",
	}[name]
}

func assertQueryOrder(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("queries=%q, want %d queries %q", got, len(want), want)
	}
	for i, fragment := range want {
		if !strings.Contains(got[i], fragment) {
			t.Fatalf("query[%d]=%q, want fragment %q", i, got[i], fragment)
		}
	}
}

func TestQueryAccessConnResolver_LifecycleContract(t *testing.T) {
	resolver, err := NewQueryAccessConnResolver(nil)
	if !errors.Is(err, ErrSessionNotPinned) || resolver != nil {
		t.Fatalf("nil constructor: resolver=%v err=%v", resolver, err)
	}

	var nilResolver *QueryAccessConnResolver
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := nilResolver.ResolveRelation(ctx, "postgresql", "app", "orders"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled nil receiver: %v", err)
	}
	if _, err := nilResolver.ResolveRelation(context.Background(), "postgresql", "app", "orders"); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("nil receiver: %v", err)
	}

	db, _ := openTestDB(t, nil)
	defer db.Close()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	resolver = &QueryAccessConnResolver{}
	if _, err := resolver.ResolveRelation(context.Background(), "postgresql", "app", "orders"); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("nil conn: %v", err)
	}
	resolver, err = NewQueryAccessConnResolver(conn)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolveRelation(context.Background(), "postgresql", "app", "orders"); err == nil || !strings.Contains(err.Error(), "query relation type") {
		t.Fatalf("closed conn error=%v, want wrapped query-path error", err)
	}
}

func TestQueryAccessConnResolver_RetainsConcreteConnField(t *testing.T) {
	typ := reflect.TypeOf(QueryAccessConnResolver{})
	if typ.NumField() != 1 || typ.Field(0).Name != "conn" || typ.Field(0).Type != reflect.TypeOf((*sql.Conn)(nil)) {
		t.Fatalf("fields=%v, want exactly conn *sql.Conn", typ.Field(0))
	}
}

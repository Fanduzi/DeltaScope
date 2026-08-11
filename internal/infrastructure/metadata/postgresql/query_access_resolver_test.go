//go:build postgresql

// Package postgresqlmeta verifies QueryAccessResolver relkind fail-closed behavior.
// input: synthetic pg_catalog rows via the package test driver
// output: base/partitioned tables admit; foreign tables reject without column probes
// pos: unit coverage for DB-backed and conn-backed query access resolvers
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
)

func TestQueryAccessResolver_RejectsForeignTable(t *testing.T) {
	db, queryLog := openTestDB(t, map[string]testQueryResult{
		"select c.relkind": {
			columns: []string{"relkind"},
			rows:    [][]driver.Value{{"f"}},
		},
		"select a.attname": {
			columns: []string{"column_name", "ordinal_position"},
			rows:    [][]driver.Value{{"id", int64(1)}},
		},
	})
	defer db.Close()

	resolver := NewQueryAccessResolver(db)
	_, err := resolver.ResolveRelation(context.Background(), "postgresql", "app", "remote_orders")
	if err == nil {
		t.Fatal("foreign table was accepted as a physical relation")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error should stay missing-relation shaped, got: %v", err)
	}
	for _, query := range queryLog.Queries() {
		if strings.Contains(query, "select a.attname") {
			t.Fatalf("foreign table must fail closed before column probe: %v", queryLog.Queries())
		}
	}
}

func TestQueryAccessConnResolver_RejectsForeignTable(t *testing.T) {
	db, queryLog := openTestDB(t, map[string]testQueryResult{
		"select c.relkind": {
			columns: []string{"relkind"},
			rows:    [][]driver.Value{{"f"}},
		},
		"select a.attname": {
			columns: []string{"column_name", "ordinal_position"},
			rows:    [][]driver.Value{{"id", int64(1)}},
		},
	})
	defer db.Close()

	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}
	defer conn.Close()

	resolver, err := NewQueryAccessConnResolver(conn)
	if err != nil {
		t.Fatalf("NewQueryAccessConnResolver: %v", err)
	}
	if _, err := resolver.ResolveRelation(context.Background(), "postgresql", "app", "remote_orders"); err == nil {
		t.Fatal("foreign table was accepted as a physical relation")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error should stay missing-relation shaped, got: %v", err)
	}
	for _, query := range queryLog.Queries() {
		if strings.Contains(query, "select a.attname") {
			t.Fatalf("foreign table must fail closed before column probe: %v", queryLog.Queries())
		}
	}
}

func TestQueryAccessResolver_AcceptsBaseAndPartitionedTables(t *testing.T) {
	for _, relkind := range []string{"r", "p"} {
		relkind := relkind
		t.Run("relkind_"+relkind, func(t *testing.T) {
			db, _ := openTestDB(t, map[string]testQueryResult{
				"select c.relkind": {
					columns: []string{"relkind"},
					rows:    [][]driver.Value{{relkind}},
				},
				"select a.attname": {
					columns: []string{"column_name", "ordinal_position"},
					rows:    [][]driver.Value{{"id", int64(1)}, {"name", int64(2)}},
				},
			})
			defer db.Close()

			resolver := NewQueryAccessResolver(db)
			rs, err := resolver.ResolveRelation(context.Background(), "postgresql", "app", "orders")
			if err != nil {
				t.Fatalf("ResolveRelation: %v", err)
			}
			if rs.Kind != "table" || rs.IsView {
				t.Fatalf("kind=%q isView=%v, want table/false", rs.Kind, rs.IsView)
			}
			if len(rs.Columns) != 2 || rs.Columns[0].Name != "id" {
				t.Fatalf("columns: %+v", rs.Columns)
			}
		})
	}
}

func TestQueryAccessResolver_ViewsRemainViews(t *testing.T) {
	for _, relkind := range []string{"v", "m"} {
		relkind := relkind
		t.Run("relkind_"+relkind, func(t *testing.T) {
			db, _ := openTestDB(t, map[string]testQueryResult{
				"select c.relkind": {
					columns: []string{"relkind"},
					rows:    [][]driver.Value{{relkind}},
				},
				"select a.attname": {
					columns: []string{"column_name", "ordinal_position"},
					rows:    [][]driver.Value{{"id", int64(1)}},
				},
			})
			defer db.Close()

			resolver := NewQueryAccessResolver(db)
			rs, err := resolver.ResolveRelation(context.Background(), "postgresql", "app", "user_summary")
			if err != nil {
				t.Fatalf("ResolveRelation: %v", err)
			}
			if rs.Kind != "view" || !rs.IsView {
				t.Fatalf("kind=%q isView=%v, want view/true", rs.Kind, rs.IsView)
			}
		})
	}
}

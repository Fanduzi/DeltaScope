// Package mysqlmeta verifies the caller-owned conn-backed MySQL/TiDB query access resolver.
// input: synthetic information_schema results via custom test driver on a *sql.Conn
// output: stable conn resolver behavior without requiring a live database
// pos: infrastructure metadata adapter test coverage for the conn-backed resolver
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"testing"
)

func newQAResolver(t *testing.T, db *sql.DB) *QueryAccessConnResolver {
	t.Helper()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	resolver, err := NewQueryAccessConnResolver(conn)
	if err != nil {
		t.Fatalf("NewQueryAccessConnResolver: %v", err)
	}
	return resolver
}

func TestQueryAccessConnResolver_TableKindAndColumns(t *testing.T) {
	db := openQATestDB(t, map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"table_type"},
			rows:    [][]driver.Value{{"BASE TABLE"}},
		},
		"from information_schema.columns": {
			columns: []string{"column_name", "ordinal_position"},
			rows: [][]driver.Value{
				{"id", int64(1)},
				{"name", int64(2)},
				{"email", int64(3)},
			},
		},
	})
	defer db.Close()

	resolver := newQAResolver(t, db)
	rs, err := resolver.ResolveRelation(context.Background(), "mysql", "app", "users")
	if err != nil {
		t.Fatalf("ResolveRelation: %v", err)
	}
	if rs.Schema != "app" {
		t.Errorf("schema: got %q, want %q", rs.Schema, "app")
	}
	if rs.Name != "users" {
		t.Errorf("name: got %q, want %q", rs.Name, "users")
	}
	if rs.Kind != "table" {
		t.Errorf("kind: got %q, want %q", rs.Kind, "table")
	}
	if rs.IsView {
		t.Error("IsView should be false for table")
	}
	if len(rs.Columns) != 3 {
		t.Fatalf("columns: got %d, want 3", len(rs.Columns))
	}
	if rs.Columns[0].Name != "id" || rs.Columns[0].Ordinal != 1 {
		t.Errorf("column[0]: got %q@%d, want id@1", rs.Columns[0].Name, rs.Columns[0].Ordinal)
	}
	if rs.Columns[2].Name != "email" || rs.Columns[2].Ordinal != 3 {
		t.Errorf("column[2]: got %q@%d, want email@3", rs.Columns[2].Name, rs.Columns[2].Ordinal)
	}
}

func TestQueryAccessConnResolver_ColumnOrderPreserved(t *testing.T) {
	db := openQATestDB(t, map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"table_type"},
			rows:    [][]driver.Value{{"BASE TABLE"}},
		},
		"from information_schema.columns": {
			columns: []string{"column_name", "ordinal_position"},
			rows: [][]driver.Value{
				{"z_col", int64(26)},
				{"a_col", int64(1)},
				{"m_col", int64(13)},
			},
		},
	})
	defer db.Close()

	resolver := newQAResolver(t, db)
	rs, err := resolver.ResolveRelation(context.Background(), "mysql", "app", "t")
	if err != nil {
		t.Fatalf("ResolveRelation: %v", err)
	}
	// Driver row order must be preserved; ordering is delegated to the catalog query.
	if len(rs.Columns) != 3 {
		t.Fatalf("columns: got %d, want 3", len(rs.Columns))
	}
	if rs.Columns[0].Name != "z_col" || rs.Columns[0].Ordinal != 26 {
		t.Errorf("column[0]: got %q@%d", rs.Columns[0].Name, rs.Columns[0].Ordinal)
	}
}

func TestQueryAccessConnResolver_ViewKind(t *testing.T) {
	db := openQATestDB(t, map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"table_type"},
			rows:    [][]driver.Value{{"VIEW"}},
		},
		"from information_schema.columns": {
			columns: []string{"column_name", "ordinal_position"},
			rows: [][]driver.Value{
				{"id", int64(1)},
				{"display_name", int64(2)},
			},
		},
	})
	defer db.Close()

	resolver := newQAResolver(t, db)
	rs, err := resolver.ResolveRelation(context.Background(), "mysql", "app", "user_view")
	if err != nil {
		t.Fatalf("ResolveRelation: %v", err)
	}
	if rs.Kind != "view" {
		t.Errorf("kind: got %q, want %q", rs.Kind, "view")
	}
	if !rs.IsView {
		t.Error("IsView should be true for view")
	}
	if len(rs.Columns) != 2 {
		t.Fatalf("columns: got %d, want 2", len(rs.Columns))
	}
}

func TestQueryAccessConnResolver_MissingRelation(t *testing.T) {
	db := openQATestDB(t, map[string]testQueryResult{})
	defer db.Close()

	resolver := newQAResolver(t, db)
	_, err := resolver.ResolveRelation(context.Background(), "mysql", "app", "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing table")
	}
	if err.Error() != "relation not found" {
		t.Errorf("error=%q, want exact conn text %q", err, "relation not found")
	}
}

func TestQueryAccessConnResolver_EmptyColumns(t *testing.T) {
	db := openQATestDB(t, map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"table_type"},
			rows:    [][]driver.Value{{"BASE TABLE"}},
		},
		"from information_schema.columns": {
			columns: []string{"column_name", "ordinal_position"},
			rows:    [][]driver.Value{},
		},
	})
	defer db.Close()

	resolver := newQAResolver(t, db)
	rs, err := resolver.ResolveRelation(context.Background(), "mysql", "app", "empty_table")
	if err != nil {
		t.Fatalf("ResolveRelation: %v", err)
	}
	if len(rs.Columns) != 0 {
		t.Errorf("columns: got %d, want 0", len(rs.Columns))
	}
}

func TestQueryAccessConnResolver_Cancellation(t *testing.T) {
	db := openQATestDB(t, map[string]testQueryResult{})
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resolver := newQAResolver(t, db)
	_, err := resolver.ResolveRelation(ctx, "mysql", "app", "users")
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
	if err.Error() != "resolve cancelled: context canceled" {
		t.Errorf("error=%q, want exact conn text %q", err, "resolve cancelled: context canceled")
	}
}

func TestQueryAccessConnResolver_RejectsSystemView(t *testing.T) {
	db := openQATestDB(t, map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"table_type"},
			rows:    [][]driver.Value{{"SYSTEM VIEW"}},
		},
		"from information_schema.columns": {
			columns: []string{"column_name", "ordinal_position"},
			rows: [][]driver.Value{
				{"id", int64(1)},
			},
		},
	})
	defer db.Close()

	resolver := newQAResolver(t, db)
	_, err := resolver.ResolveRelation(context.Background(), "mysql", "app", "system_view")
	if err == nil {
		t.Fatal("system view was accepted as a physical relation")
	}
	if err.Error() != "unsupported relation type" {
		t.Errorf("error=%q, want exact conn text %q", err, "unsupported relation type")
	}
}

// qaTestDriver is a custom sql driver for query access resolver tests.
type qaTestDriver struct {
	results map[string]testQueryResult
}

type qaTestConn struct {
	results map[string]testQueryResult
}

type qaTestRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

var (
	qaTestDriverRegistry sync.Once
	qaTestDriverCounter  int
)

func openQATestDB(t *testing.T, results map[string]testQueryResult) *sql.DB {
	t.Helper()

	qaTestDriverRegistry.Do(func() {
		sql.Register("mysqlmeta-qa-test", qaTestDriver{})
	})

	name := "mysqlmeta-qa-test-" + strings.ReplaceAll(t.Name(), "/", "_")
	registerQATestDriverResults(name, results)

	db, err := sql.Open("mysqlmeta-qa-test", name)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { unregisterQATestDriverResults(name) })
	return db
}

var qaTestDriverResults sync.Map

func registerQATestDriverResults(name string, results map[string]testQueryResult) {
	cloned := make(map[string]testQueryResult, len(results))
	for key, result := range results {
		rows := make([][]driver.Value, len(result.rows))
		for i := range result.rows {
			rows[i] = append([]driver.Value(nil), result.rows[i]...)
		}
		cloned[key] = testQueryResult{
			columns: append([]string(nil), result.columns...),
			rows:    rows,
		}
	}
	qaTestDriverResults.Store(name, cloned)
}

func unregisterQATestDriverResults(name string) {
	qaTestDriverResults.Delete(name)
}

func (d qaTestDriver) Open(name string) (driver.Conn, error) {
	value, _ := qaTestDriverResults.Load(name)
	results, _ := value.(map[string]testQueryResult)
	return qaTestConn{results: results}, nil
}

func (c qaTestConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c qaTestConn) Close() error                        { return nil }
func (c qaTestConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c qaTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	for fragment, result := range c.results {
		if strings.Contains(query, fragment) {
			return &qaTestRows{
				columns: append([]string(nil), result.columns...),
				rows:    append([][]driver.Value(nil), result.rows...),
			}, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *qaTestRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (r *qaTestRows) Close() error { return nil }

func (r *qaTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

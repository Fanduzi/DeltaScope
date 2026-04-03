// Package mysqlmeta verifies metadata normalization helpers for the MySQL provider.
// input: synthetic variable, version, DSN, collation, type, and index classification scenarios
// output: stable provider helper behavior without requiring a live database
// pos: infrastructure metadata adapter test coverage
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

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestConnectionConfigDSNUsesTCPHostAndPort(t *testing.T) {
	config := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     3307,
		User:     "root",
		Password: "secret",
	}

	if got := config.Address(); got != "127.0.0.1:3307" {
		t.Fatalf("expected tcp address 127.0.0.1:3307, got %q", got)
	}
	if got := config.Network(); got != "tcp" {
		t.Fatalf("expected tcp network, got %q", got)
	}
}

func TestConnectionConfigDSNUsesUnixSocket(t *testing.T) {
	config := ConnectionConfig{
		Socket:   "/tmp/mysql.sock",
		User:     "root",
		Password: "secret",
	}

	if got := config.Address(); got != "/tmp/mysql.sock" {
		t.Fatalf("expected socket address /tmp/mysql.sock, got %q", got)
	}
	if got := config.Network(); got != "unix" {
		t.Fatalf("expected unix network, got %q", got)
	}
}

func TestDetectDialectFromVersion(t *testing.T) {
	if got := detectDialectFromVersion("8.0.36"); got != spec.DialectMySQL {
		t.Fatalf("expected mysql dialect, got %q", got)
	}
	if got := detectDialectFromVersion("8.0.11-TiDB-v8.5.0"); got != spec.DialectTiDB {
		t.Fatalf("expected tidb dialect, got %q", got)
	}
}

func TestNormalizeOnOff(t *testing.T) {
	if !normalizeOnOff("ON") {
		t.Fatalf("expected ON to normalize to true")
	}
	if !normalizeOnOff("1") {
		t.Fatalf("expected 1 to normalize to true")
	}
	if normalizeOnOff("OFF") {
		t.Fatalf("expected OFF to normalize to false")
	}
}

func TestCharsetFromCollation(t *testing.T) {
	if got := charsetFromCollation("utf8mb4_general_ci"); got != "utf8mb4" {
		t.Fatalf("expected utf8mb4, got %q", got)
	}
	if got := charsetFromCollation(""); got != "" {
		t.Fatalf("expected empty charset, got %q", got)
	}
}

func TestParseColumnType(t *testing.T) {
	baseType, length, unsigned := parseColumnType("varchar(255)")
	if baseType != "varchar" || length != 255 || unsigned {
		t.Fatalf("unexpected varchar parse result: %q %d %v", baseType, length, unsigned)
	}

	baseType, length, unsigned = parseColumnType("bigint(20) unsigned")
	if baseType != "bigint" || length != 20 || !unsigned {
		t.Fatalf("unexpected bigint parse result: %q %d %v", baseType, length, unsigned)
	}
}

func TestClassifyIndex(t *testing.T) {
	if kind := classifyIndex("PRIMARY", 0, "BTREE"); kind != spec.IndexKindPrimary {
		t.Fatalf("expected primary index kind, got %q", kind)
	}
	if kind := classifyIndex("uniq_email", 0, "BTREE"); kind != spec.IndexKindUnique {
		t.Fatalf("expected unique index kind, got %q", kind)
	}
	if kind := classifyIndex("full_body", 1, "FULLTEXT"); kind != spec.IndexKindFulltext {
		t.Fatalf("expected fulltext index kind, got %q", kind)
	}
	if kind := classifyIndex("idx_email", 1, "BTREE"); kind != spec.IndexKindSecondary {
		t.Fatalf("expected secondary index kind, got %q", kind)
	}
}

func TestAccumulateIndexCardinalityPreservesMaximumNonNilValue(t *testing.T) {
	indexes := make(map[string]*spec.Index)
	order := make([]string, 0)

	accumulateIndexRow(indexes, &order, "PRIMARY", 0, "BTREE", "id", nil)
	accumulateIndexRow(indexes, &order, "PRIMARY", 0, "BTREE", "tenant_id", testPtrInt64(12))
	accumulateIndexRow(indexes, &order, "PRIMARY", 0, "BTREE", "account_id", testPtrInt64(7))

	index := indexes["PRIMARY"]
	if index == nil {
		t.Fatalf("expected primary index to be accumulated")
	}
	if index.Cardinality == nil || *index.Cardinality != 12 {
		t.Fatalf("expected max non-nil cardinality 12, got %#v", index)
	}
	if len(index.Columns) != 3 {
		t.Fatalf("expected all index columns to be preserved, got %#v", index.Columns)
	}
}

func TestLoadTableSnapshotPreservesIndexCardinalityFromStatisticsRows(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows: [][]driver.Value{{
				"InnoDB", "utf8mb4_general_ci", "users table", nil, "Dynamic", int64(100),
			}},
		},
		"from information_schema.columns": {
			columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra"},
			rows: [][]driver.Value{
				{"id", "bigint(20)", nil, nil, "", nil, "NO", ""},
				{"email", "varchar(255)", "utf8mb4", "utf8mb4_general_ci", "", nil, "YES", ""},
			},
		},
		"from information_schema.statistics": {
			columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality"},
			rows: [][]driver.Value{
				{"PRIMARY", int64(0), "BTREE", "id", int64(120)},
				{"idx_users_email", int64(1), "BTREE", "email", int64(45)},
				{"idx_users_email", int64(1), "BTREE", "tenant_id", int64(42)},
			},
		},
	})
	defer db.Close()

	provider := NewProvider(db)
	snapshot, err := provider.LoadTableSnapshot(context.Background(), spec.DialectMySQL, "app", "users")
	if err != nil {
		t.Fatalf("load table snapshot: %v", err)
	}

	if snapshot.PrimaryKey == nil || snapshot.PrimaryKey.Cardinality == nil || *snapshot.PrimaryKey.Cardinality != 120 {
		t.Fatalf("expected primary key cardinality 120, got %#v", snapshot.PrimaryKey)
	}
	if len(snapshot.Indexes) != 1 {
		t.Fatalf("expected one secondary index, got %#v", snapshot.Indexes)
	}
	if snapshot.Indexes[0].Cardinality == nil || *snapshot.Indexes[0].Cardinality != 45 {
		t.Fatalf("expected max secondary index cardinality 45, got %#v", snapshot.Indexes[0])
	}
	if len(snapshot.Indexes[0].Columns) != 2 {
		t.Fatalf("expected secondary index columns to be preserved, got %#v", snapshot.Indexes[0])
	}
}

func testPtrInt64(value int64) *int64 {
	return &value
}

type testQueryResult struct {
	columns []string
	rows    [][]driver.Value
}

type testDriver struct {
	results map[string]testQueryResult
}

type testConn struct {
	results map[string]testQueryResult
}

type testRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

var (
	testDriverRegistry sync.Once
	testDriverCounter  int
)

func openTestDB(t *testing.T, results map[string]testQueryResult) *sql.DB {
	t.Helper()

	testDriverRegistry.Do(func() {
		sql.Register("mysqlmeta-test", testDriver{})
	})

	name := "mysqlmeta-test-" + strings.ReplaceAll(t.Name(), "/", "_")
	registerTestDriverResults(name, results)

	db, err := sql.Open("mysqlmeta-test", name)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { unregisterTestDriverResults(name) })
	return db
}

var testDriverResults sync.Map

func registerTestDriverResults(name string, results map[string]testQueryResult) {
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
	testDriverResults.Store(name, cloned)
}

func unregisterTestDriverResults(name string) {
	testDriverResults.Delete(name)
}

func (d testDriver) Open(name string) (driver.Conn, error) {
	value, _ := testDriverResults.Load(name)
	results, _ := value.(map[string]testQueryResult)
	return testConn{results: results}, nil
}

func (c testConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c testConn) Close() error                        { return nil }
func (c testConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c testConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	for fragment, result := range c.results {
		if strings.Contains(query, fragment) {
			return &testRows{
				columns: append([]string(nil), result.columns...),
				rows:    append([][]driver.Value(nil), result.rows...),
			}, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *testRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (r *testRows) Close() error { return nil }

func (r *testRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

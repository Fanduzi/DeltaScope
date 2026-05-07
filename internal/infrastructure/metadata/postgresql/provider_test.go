// Package postgresqlmeta verifies metadata normalization helpers for the PostgreSQL provider.
// input: synthetic version, schema, instance-fact, table snapshot, and plan-estimate query scenarios
// output: stable provider helper behavior without requiring a live database
// pos: infrastructure metadata adapter test coverage
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

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

func TestDetectDialectFromVersion(t *testing.T) {
	if got := detectDialectFromVersion("PostgreSQL 16.3 on arm64-apple-darwin"); got != spec.DialectPostgreSQL {
		t.Fatalf("expected postgresql dialect, got %q", got)
	}
	if got := detectDialectFromVersion("CockroachDB CCL v24.1.0"); got != spec.DialectUnknown {
		t.Fatalf("expected unknown dialect for non-PostgreSQL version, got %q", got)
	}
}

func TestNormalizeYesNo(t *testing.T) {
	if !normalizeYesNo("on") {
		t.Fatalf("expected on to normalize to true")
	}
	if !normalizeYesNo("yes") {
		t.Fatalf("expected yes to normalize to true")
	}
	if normalizeYesNo("off") {
		t.Fatalf("expected off to normalize to false")
	}
}

func TestParseDataType(t *testing.T) {
	baseType, length, unsigned := parseDataType("character varying", sql.NullInt64{Int64: 255, Valid: true}, "")
	if baseType != "varchar" || length != 255 || unsigned {
		t.Fatalf("unexpected varying parse result: %q %d %v", baseType, length, unsigned)
	}

	baseType, length, unsigned = parseDataType("integer", sql.NullInt64{}, "nextval('users_id_seq'::regclass)")
	if baseType != "integer" || length != 0 || unsigned {
		t.Fatalf("unexpected integer parse result: %q %d %v", baseType, length, unsigned)
	}
}

func TestFindSchemasForTableUsesCatalogTables(t *testing.T) {
	db, queryLog := openTestDB(t, map[string]testQueryResult{
		"from pg_catalog.pg_class": {
			columns: []string{"schema_name"},
			rows:    [][]driver.Value{{"analytics"}, {"public"}},
		},
	})
	defer db.Close()

	provider := NewProvider(db)
	schemas, err := provider.FindSchemasForTable(context.Background(), "users")
	if err != nil {
		t.Fatalf("find schemas for table: %v", err)
	}

	if len(schemas) != 2 || schemas[0] != "analytics" || schemas[1] != "public" {
		t.Fatalf("expected schemas [analytics public], got %#v", schemas)
	}
	if !containsQueryFragment(queryLog.Queries(), "from pg_catalog.pg_class") {
		t.Fatalf("expected catalog-backed schema discovery query, got %#v", queryLog.Queries())
	}
}

func TestResolveTableForIndexUsesCatalogOwnership(t *testing.T) {
	db, queryLog := openTestDB(t, map[string]testQueryResult{
		"from pg_catalog.pg_class idx": {
			columns: []string{"table_name"},
			rows:    [][]driver.Value{{"users"}},
		},
	})
	defer db.Close()

	provider := NewProvider(db)
	tableName, err := provider.ResolveTableForIndex(context.Background(), spec.DialectPostgreSQL, "public", "idx_users_email")
	if err != nil {
		t.Fatalf("resolve table for index: %v", err)
	}
	if tableName != "users" {
		t.Fatalf("expected users table owner, got %q", tableName)
	}
	if !containsQueryFragment(queryLog.Queries(), "from pg_catalog.pg_class idx") {
		t.Fatalf("expected catalog-backed index owner query, got %#v", queryLog.Queries())
	}
}

func TestLoadInstanceFactsMapsPostgreSQLSettings(t *testing.T) {
	db, _ := openTestDB(t, map[string]testQueryResult{
		"select version()": {
			columns: []string{"version"},
			rows:    [][]driver.Value{{"PostgreSQL 16.3"}},
		},
		"from pg_settings": {
			columns: []string{"name", "setting"},
			rows: [][]driver.Value{
				{"server_encoding", "UTF8"},
				{"default_toast_compression", "pglz"},
				{"wal_compression", "on"},
			},
		},
	})
	defer db.Close()

	provider := NewProvider(db)
	facts, err := provider.LoadInstanceFacts(context.Background(), spec.DialectPostgreSQL, "public")
	if err != nil {
		t.Fatalf("load instance facts: %v", err)
	}

	if facts.Version != "PostgreSQL 16.3" {
		t.Fatalf("expected version to be populated, got %#v", facts)
	}
	if facts.DefaultCharset != "UTF8" {
		t.Fatalf("expected default charset UTF8, got %#v", facts)
	}
	if !facts.InnoDBAdaptiveHashEnabled {
		t.Fatalf("expected wal compression on to map to true, got %#v", facts)
	}
	if facts.InnoDBDefaultRowFormat != "pglz" {
		t.Fatalf("expected toast compression to map into row format slot, got %#v", facts)
	}
}

func TestLoadTableSnapshotUsesCatalogsAndPlannerStats(t *testing.T) {
	db, queryLog := openTestDB(t, map[string]testQueryResult{
		"obj_description(c.oid, 'pg_class')": {
			columns: []string{"relkind", "table_comment", "reltuples"},
			rows:    [][]driver.Value{{"r", "application users", float64(321)}},
		},
		"from pg_catalog.pg_attribute": {
			columns: []string{"column_name", "data_type", "character_maximum_length", "character_set_name", "collation_name", "is_nullable", "column_default", "is_identity", "column_comment"},
			rows: [][]driver.Value{
				{"id", "integer", nil, nil, nil, false, "nextval('users_id_seq'::regclass)", false, "primary key"},
				{"email", "character varying", int64(255), nil, "en_US.utf8", false, nil, false, "login email"},
				{"created_at", "timestamp with time zone", nil, nil, nil, false, "CURRENT_TIMESTAMP", false, "creation time"},
			},
		},
		"from pg_catalog.pg_constraint": {
			columns: []string{"constraint_name", "index_name", "column_name", "index_reltuples"},
			rows: [][]driver.Value{
				{"users_pk_constraint", "users_primary_idx", "id", float64(321)},
			},
		},
		"from pg_catalog.pg_index": {
			columns: []string{"index_name", "is_unique", "is_primary", "indexdef", "index_reltuples"},
			rows: [][]driver.Value{
				{"users_primary_idx", true, true, "CREATE UNIQUE INDEX users_primary_idx ON public.users USING btree (id)", float64(321)},
				{"idx_users_email", false, false, "CREATE INDEX idx_users_email ON public.users USING btree (email)", float64(250)},
				{"uniq_users_email", true, false, "CREATE UNIQUE INDEX uniq_users_email ON public.users USING btree (email)", float64(320)},
			},
		},
	})
	defer db.Close()

	provider := NewProvider(db)
	snapshot, err := provider.LoadTableSnapshot(context.Background(), spec.DialectPostgreSQL, "public", "users")
	if err != nil {
		t.Fatalf("load table snapshot: %v", err)
	}

	if !snapshot.Exists {
		t.Fatalf("expected snapshot to exist, got %#v", snapshot)
	}
	if snapshot.Table == nil || snapshot.Table.Name != "users" || snapshot.Table.Comment != "application users" {
		t.Fatalf("expected table metadata to be populated, got %#v", snapshot.Table)
	}
	if got := snapshot.Options["table_type"]; got != "BASE TABLE" {
		t.Fatalf("expected table_type BASE TABLE, got %q", got)
	}
	if got := snapshot.Options["table_rows"]; got != "321" {
		t.Fatalf("expected reltuples-backed table_rows 321, got %q", got)
	}
	if len(snapshot.Columns) != 3 {
		t.Fatalf("expected three columns, got %#v", snapshot.Columns)
	}
	if !snapshot.Columns[0].AutoIncrement {
		t.Fatalf("expected serial-style default to map to auto increment, got %#v", snapshot.Columns[0])
	}
	if snapshot.Columns[1].Comment != "login email" {
		t.Fatalf("expected column comment to be populated, got %#v", snapshot.Columns[1])
	}
	if !snapshot.Columns[2].DefaultIsCurrentTimestamp {
		t.Fatalf("expected current timestamp default to be recognized, got %#v", snapshot.Columns[2])
	}
	if snapshot.PrimaryKey == nil || snapshot.PrimaryKey.Name != "users_primary_idx" || snapshot.PrimaryKey.Kind != spec.IndexKindPrimary {
		t.Fatalf("expected primary key from constraint/catalog truth, got %#v", snapshot.PrimaryKey)
	}
	if snapshot.PrimaryKey.Cardinality == nil || *snapshot.PrimaryKey.Cardinality != 321 {
		t.Fatalf("expected primary key cardinality from planner stats, got %#v", snapshot.PrimaryKey)
	}
	if len(snapshot.Constraints) != 1 || snapshot.Constraints[0].Type != "primary_key" || snapshot.Constraints[0].Name != "users_pk_constraint" {
		t.Fatalf("expected primary key constraint metadata, got %#v", snapshot.Constraints)
	}
	if len(snapshot.Indexes) != 2 {
		t.Fatalf("expected two non-primary indexes, got %#v", snapshot.Indexes)
	}
	if snapshot.Indexes[0].Cardinality == nil || *snapshot.Indexes[0].Cardinality != 250 {
		t.Fatalf("expected reltuples-backed index cardinality, got %#v", snapshot.Indexes[0])
	}
	if snapshot.Indexes[1].Kind != spec.IndexKindUnique {
		t.Fatalf("expected unique secondary index, got %#v", snapshot.Indexes[1])
	}
	if !containsQueryFragment(queryLog.Queries(), "from pg_catalog.pg_constraint") {
		t.Fatalf("expected constraint-backed pk query, got %#v", queryLog.Queries())
	}
	if containsQueryFragment(queryLog.Queries(), "from information_schema.tables") {
		t.Fatalf("expected snapshot to avoid information_schema table lookup, got %#v", queryLog.Queries())
	}
}

func TestLoadPlanEstimateUsesPlainExplainForUpdate(t *testing.T) {
	db, queryLog := openTestDB(t, map[string]testQueryResult{
		"EXPLAIN UPDATE users SET active = false WHERE id = 42": {
			columns: []string{"QUERY PLAN"},
			rows:    [][]driver.Value{{"Update on users  (cost=0.15..8.17 rows=0 width=0)"}, {"  ->  Index Scan using users_pkey on users  (cost=0.15..8.17 rows=1 width=0)"}},
		},
	})
	defer db.Close()

	provider := NewProvider(db)
	estimate, err := provider.LoadPlanEstimate(context.Background(), spec.Statement{
		Kind:   spec.KindDML,
		RawSQL: "UPDATE users SET active = false WHERE id = 42",
		DML: &spec.DML{
			Operation: spec.DMLOperationUpdate,
		},
	})
	if err != nil {
		t.Fatalf("load plan estimate: %v", err)
	}

	if estimate == nil {
		t.Fatalf("expected plan estimate")
	}
	if estimate.Source != spec.ImpactSourcePlan {
		t.Fatalf("expected plan source, got %#v", estimate)
	}
	if estimate.EstimatedRows == nil || *estimate.EstimatedRows != 1 {
		t.Fatalf("expected one estimated row from EXPLAIN, got %#v", estimate)
	}
	queries := queryLog.Queries()
	if !containsQueryFragment(queries, "EXPLAIN UPDATE users SET active = false WHERE id = 42") {
		t.Fatalf("expected plain EXPLAIN query, got %#v", queries)
	}
	if containsQueryFragment(queries, "ANALYZE") {
		t.Fatalf("expected plain EXPLAIN only, got %#v", queries)
	}
}

func TestLoadPlanEstimateSkipsUnsupportedStatementKinds(t *testing.T) {
	db, queryLog := openTestDB(t, map[string]testQueryResult{})
	defer db.Close()

	provider := NewProvider(db)
	estimate, err := provider.LoadPlanEstimate(context.Background(), spec.Statement{
		Kind:   spec.KindDDL,
		RawSQL: "CREATE TABLE users (id bigint primary key)",
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "users"},
		},
	})
	if err != nil {
		t.Fatalf("load plan estimate: %v", err)
	}
	if estimate != nil {
		t.Fatalf("expected unsupported statement kind to skip planning, got %#v", estimate)
	}
	if len(queryLog.Queries()) != 0 {
		t.Fatalf("expected skipped planning to avoid queries, got %#v", queryLog.Queries())
	}
}

func TestParseExplainEstimatedRowsUsesLargestMatch(t *testing.T) {
	rows, ok := parseExplainEstimatedRows([]string{
		"Update on users  (cost=0.00..100.00 rows=0 width=0)",
		"  ->  Seq Scan on users  (cost=0.00..100.00 rows=500 width=4)",
		"        Filter: (tenant_id = 42)",
	})
	if !ok {
		t.Fatal("expected explain rows to be parsed")
	}
	if rows != 500 {
		t.Fatalf("expected largest explain rows match 500, got %d", rows)
	}
}

type testQueryResult struct {
	columns []string
	rows    [][]driver.Value
}

type testDriver struct{}

type testConn struct {
	name    string
	results map[string]testQueryResult
}

type testRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

type testQueryLog struct {
	mu      sync.Mutex
	queries []string
}

func (l *testQueryLog) Add(query string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.queries = append(l.queries, query)
}

func (l *testQueryLog) Queries() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.queries...)
}

var testDriverRegistry sync.Once
var testDriverResults sync.Map
var testDriverLogs sync.Map

func openTestDB(t *testing.T, results map[string]testQueryResult) (*sql.DB, *testQueryLog) {
	t.Helper()

	testDriverRegistry.Do(func() {
		sql.Register("postgresqlmeta-test", testDriver{})
	})

	name := "postgresqlmeta-test-" + strings.ReplaceAll(t.Name(), "/", "_")
	queryLog := &testQueryLog{}
	registerTestDriverResults(name, results)
	testDriverLogs.Store(name, queryLog)

	db, err := sql.Open("postgresqlmeta-test", name)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		unregisterTestDriverResults(name)
		testDriverLogs.Delete(name)
	})
	return db, queryLog
}

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
	return testConn{name: name, results: results}, nil
}

func (c testConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c testConn) Close() error                        { return nil }
func (c testConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c testConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if value, ok := testDriverLogs.Load(c.name); ok {
		if log, ok := value.(*testQueryLog); ok {
			log.Add(query)
		}
	}
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

func containsQueryFragment(queries []string, fragment string) bool {
	for _, query := range queries {
		if strings.Contains(query, fragment) {
			return true
		}
	}
	return false
}

//go:build postgresql

// Package deltascope verifies the PostgreSQL trusted session does not execute analysis SQL.
// input: caller-owned connection backed by a recording database/sql driver
// output: safe session/catalog queries only; submitted marker absent from driver traffic
// pos: no-execution and no-leak boundary test for the PostgreSQL SDK session
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestTrustedSDK_CountIntegerOneDoesNotExecuteUserSQL(t *testing.T) {
	const marker = "SQLNOTEXEC_MARKER_7f3a"
	recorder := &postgresqlRecordingDriver{}
	db := openPostgreSQLRecordingDB(t, recorder)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	session, err := NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewPostgreSQLQueryAccessSessionFromConn: %v", err)
	}
	result, err := AnalyzePostgreSQLQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:           "SELECT COUNT(1) /* " + marker + " */ FROM app.orders",
		Dialect:       DialectPostgreSQL,
		Mode:          QueryAccessModeStrict,
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("AnalyzePostgreSQLQueryAccessWithSession: %v", err)
	}
	if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
		t.Fatalf("expected admitted result: classification=%s admission=%s requirements=%+v unresolved=%+v reasons=%v queries=%v", result.ReadClassification, result.Admission, result.Requirements, result.Unresolved, result.ReasonCodes, recorder.recorded())
	}
	if len(recorder.recorded()) == 0 {
		t.Fatal("expected safe session/catalog probes")
	}
	for _, query := range recorder.recorded() {
		if strings.Contains(query, marker) {
			t.Fatalf("analysis SQL reached driver: %q", query)
		}
	}
	if strings.Contains(fmt.Sprintf("%+v", result), marker) {
		t.Fatal("marker leaked into SDK struct dump")
	}
}

var postgresqlRecordingDriverSequence struct {
	sync.Mutex
	next int
}

type postgresqlRecordingDriver struct {
	mu      sync.Mutex
	queries []string
}

func openPostgreSQLRecordingDB(t *testing.T, recorder *postgresqlRecordingDriver) *sql.DB {
	t.Helper()
	postgresqlRecordingDriverSequence.Lock()
	postgresqlRecordingDriverSequence.next++
	name := fmt.Sprintf("deltascope-pg-recording-%d", postgresqlRecordingDriverSequence.next)
	postgresqlRecordingDriverSequence.Unlock()
	sql.Register(name, recorder)
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("open recording db: %v", err)
	}
	return db
}

func (d *postgresqlRecordingDriver) Open(string) (driver.Conn, error) {
	return postgresqlRecordingConn{recorder: d}, nil
}

func (d *postgresqlRecordingDriver) record(query string) {
	d.mu.Lock()
	d.queries = append(d.queries, query)
	d.mu.Unlock()
}

func (d *postgresqlRecordingDriver) recorded() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.queries...)
}

type postgresqlRecordingConn struct {
	recorder *postgresqlRecordingDriver
}

func (postgresqlRecordingConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (postgresqlRecordingConn) Close() error                        { return nil }
func (postgresqlRecordingConn) Begin() (driver.Tx, error)           { return postgresqlRecordingTx{}, nil }
func (postgresqlRecordingConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return postgresqlRecordingTx{}, nil
}
func (postgresqlRecordingConn) Ping(context.Context) error { return nil }

func (c postgresqlRecordingConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.recorder.record(query)
	switch {
	case strings.Contains(query, "SELECT VERSION()"):
		return newPostgreSQLRecordingRows([]string{"version"}, [][]driver.Value{{"PostgreSQL 17.10"}}), nil
	case strings.Contains(query, "current_database()"):
		return newPostgreSQLRecordingRows(
			[]string{"database_oid", "role_oid", "server_version_num", "backend_pid"},
			[][]driver.Value{{int64(16384), int64(10), int64(170000), int64(42)}},
		), nil
	case strings.Contains(query, "current_schemas(true)") && !strings.Contains(query, "with any_type as"):
		return newPostgreSQLRecordingRows([]string{"oid"}, [][]driver.Value{{int64(11)}}), nil
	case strings.Contains(query, "pg_namespace n where n.nspname = $1"):
		return newPostgreSQLRecordingRows([]string{"oid"}, [][]driver.Value{{int64(11)}}), nil
	case strings.Contains(query, "select c.relkind"):
		return newPostgreSQLRecordingRows([]string{"relkind"}, [][]driver.Value{{"r"}}), nil
	case strings.Contains(query, "select a.attname"):
		return newPostgreSQLRecordingRows([]string{"column_name", "ordinal_position"}, [][]driver.Value{{"id", int64(1)}}), nil
	case strings.Contains(query, "with any_type as"):
		return newPostgreSQLRecordingRows(
			[]string{"oid", "namespace_oid", "result_type", "volatility", "schema_name", "func_name", "arg_types", "prokind", "pronargs", "poly_oid", "poly_name", "poly_schema"},
			[][]driver.Value{{int64(2147), int64(11), int64(20), "i", "pg_catalog", "count", "2276", "a", int64(1), int64(2276), "any", "pg_catalog"}},
		), nil
	default:
		return nil, driver.ErrSkip
	}
}

type postgresqlRecordingTx struct{}

func (postgresqlRecordingTx) Commit() error   { return nil }
func (postgresqlRecordingTx) Rollback() error { return nil }

type postgresqlRecordingRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func newPostgreSQLRecordingRows(columns []string, rows [][]driver.Value) *postgresqlRecordingRows {
	return &postgresqlRecordingRows{columns: columns, rows: rows}
}

func (r *postgresqlRecordingRows) Columns() []string { return r.columns }
func (r *postgresqlRecordingRows) Close() error      { return nil }

func (r *postgresqlRecordingRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

// Package deltascope verifies the MySQL/TiDB session connection boundary.
// input: a custom database/sql driver and caller-owned *sql.Conn
// output: deprecated-session caller ownership and no-execution compatibility
// pos: public MySQL/TiDB session integration tests without Docker
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

func TestMySQLTiDBQueryAccessSessionUsesCallerConnection(t *testing.T) {
	db := openSessionTestDB(t)
	defer db.Close()

	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}

	session, err := NewMySQLTiDBQueryAccessSessionFromConn(t.Context(), conn)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	result, err := AnalyzeMySQLTiDBQueryAccessWithSession(t.Context(), session, QueryAccessRequest{
		SQL:           "SELECT id FROM app.users",
		Dialect:       DialectMySQL,
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
		t.Fatalf("classification=%q admission=%q, want read_only/admissible", result.ReadClassification, result.Admission)
	}
	if err := conn.PingContext(t.Context()); err != nil {
		t.Fatalf("caller connection after analysis: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("caller close: %v", err)
	}
}

// TestMySQLTiDBQueryAccessSession_DoesNotExecuteUserSQL proves the analyzer
// never sends caller-submitted analysis SQL to the database. The recording
// driver captures every query text; the unique marker embedded in the analysis
// SQL must appear in none of them, including for admitted relationless shapes.
func TestMySQLTiDBQueryAccessSession_DoesNotExecuteUserSQL(t *testing.T) {
	const marker = "SQLNOTEXEC_MARKER_7f3a"
	shapes := []struct{ name, sql string }{
		{"relationless_lower", "SELECT LOWER('" + marker + "')"},
		{"relationless_count", "SELECT COUNT(1) /* " + marker + " */"},
		{"relation_bearing", "SELECT LOWER('" + marker + "') FROM app.users"},
	}
	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			rec := &recordingDriver{}
			db := openRecordingTestDB(t, rec)
			defer db.Close()

			conn, err := db.Conn(t.Context())
			if err != nil {
				t.Fatalf("conn: %v", err)
			}
			session, err := NewMySQLTiDBQueryAccessSessionFromConn(t.Context(), conn)
			if err != nil {
				t.Fatalf("new session: %v", err)
			}
			if _, err := AnalyzeMySQLTiDBQueryAccessWithSession(t.Context(), session, QueryAccessRequest{
				SQL:           shape.sql,
				Dialect:       DialectMySQL,
				DefaultSchema: "app",
			}); err != nil {
				t.Fatalf("analyze: %v", err)
			}
			if err := conn.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}

			queries := rec.recorded()
			if len(queries) == 0 {
				t.Fatalf("%s recorded no queries; expected at least the version probe", shape.name)
			}
			for _, q := range queries {
				if strings.Contains(q, marker) {
					t.Fatalf("%s executed user SQL against the database: %q", shape.name, q)
				}
			}
		})
	}
}

var sessionTestDriverOnce sync.Once

func openSessionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sessionTestDriverOnce.Do(func() {
		sql.Register("deltascope-session-test", sessionTestDriver{})
	})
	db, err := sql.Open("deltascope-session-test", "")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

type sessionTestDriver struct{}

func (sessionTestDriver) Open(string) (driver.Conn, error) {
	return sessionTestConn{}, nil
}

type sessionTestConn struct{}

func (sessionTestConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (sessionTestConn) Close() error                        { return nil }
func (sessionTestConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (sessionTestConn) Ping(context.Context) error          { return nil }

func (sessionTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "information_schema.tables"):
		return newSessionTestRows([]string{"table_type"}, [][]driver.Value{{"BASE TABLE"}}), nil
	case strings.Contains(query, "information_schema.columns"):
		return newSessionTestRows([]string{"column_name", "ordinal_position"}, [][]driver.Value{
			{"id", int64(1)},
			{"dept", int64(2)},
			{"amount", int64(3)},
			{"name", int64(4)},
		}), nil
	case strings.Contains(query, "SELECT VERSION()"):
		return newSessionTestRows([]string{"version"}, [][]driver.Value{{"8.4.10"}}), nil
	case strings.Contains(query, "SELECT 1"):
		return newSessionTestRows([]string{"value"}, [][]driver.Value{{int64(1)}}), nil
	default:
		return nil, driver.ErrSkip
	}
}

type sessionTestRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func newSessionTestRows(columns []string, rows [][]driver.Value) *sessionTestRows {
	return &sessionTestRows{columns: columns, rows: rows}
}

func (r *sessionTestRows) Columns() []string { return r.columns }
func (r *sessionTestRows) Close() error      { return nil }

func (r *sessionTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

// recordingDriver captures every query text routed through the caller-owned
// connection so tests can prove analysis never executes user SQL.
type recordingDriver struct {
	mu      sync.Mutex
	queries []string
}

func (d *recordingDriver) record(query string) {
	d.mu.Lock()
	d.queries = append(d.queries, query)
	d.mu.Unlock()
}

func (d *recordingDriver) recorded() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.queries...)
}

func (d *recordingDriver) Open(string) (driver.Conn, error) {
	return recordingConn{rec: d}, nil
}

var (
	recordingDriverSeq  int
	recordingDriverSeqM sync.Mutex
)

func openRecordingTestDB(t *testing.T, rec *recordingDriver) *sql.DB {
	t.Helper()
	recordingDriverSeqM.Lock()
	recordingDriverSeq++
	name := fmt.Sprintf("deltascope-recording-test-%d", recordingDriverSeq)
	recordingDriverSeqM.Unlock()

	sql.Register(name, rec)
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("open recording db: %v", err)
	}
	return db
}

type recordingConn struct {
	rec *recordingDriver
}

func (recordingConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (recordingConn) Close() error                        { return nil }
func (recordingConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (recordingConn) Ping(context.Context) error          { return nil }

func (c recordingConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.rec.record(query)
	switch {
	case strings.Contains(query, "information_schema.tables"):
		return newSessionTestRows([]string{"table_type"}, [][]driver.Value{{"BASE TABLE"}}), nil
	case strings.Contains(query, "information_schema.columns"):
		return newSessionTestRows([]string{"column_name", "ordinal_position"}, [][]driver.Value{
			{"id", int64(1)},
			{"dept", int64(2)},
			{"amount", int64(3)},
			{"name", int64(4)},
		}), nil
	case strings.Contains(query, "SELECT VERSION()"):
		return newSessionTestRows([]string{"version"}, [][]driver.Value{{"8.4.10"}}), nil
	case strings.Contains(query, "SELECT 1"):
		return newSessionTestRows([]string{"value"}, [][]driver.Value{{int64(1)}}), nil
	default:
		return nil, driver.ErrSkip
	}
}

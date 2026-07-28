// Package deltascope verifies the MySQL/TiDB session connection boundary.
// input: a custom database/sql driver and caller-owned *sql.Conn
// output: same-connection metadata resolution and caller ownership
// pos: public MySQL/TiDB session integration tests without Docker
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
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

func TestMySQLTiDBQueryAccessSession_PromotesProvenMySQL84CountStar(t *testing.T) {
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
		SQL:           "SELECT COUNT(*) FROM app.users",
		Dialect:       DialectMySQL,
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
		t.Fatalf("classification=%q admission=%q, want read_only/admissible", result.ReadClassification, result.Admission)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestMySQLTiDBQueryAccessSession_PromotesLiteralAndReversedOperands(t *testing.T) {
	cases := []struct {
		name     string
		sql      string
		wantReqs []QueryAccessRequirement
	}{
		{
			name: "literal_only_lower",
			sql:  "SELECT LOWER('x') FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			},
		},
		{
			name: "count_literal",
			sql:  "SELECT COUNT(1) FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			},
		},
		{
			name: "reversed_coalesce",
			sql:  "SELECT COALESCE('x', name) FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
				{Object: "app.builtin_semantic_facts.name", Privilege: "read_column"},
			},
		},
		{
			name: "all_constant_coalesce",
			sql:  "SELECT COALESCE('x', 'y') FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
				SQL:           tc.sql,
				Dialect:       DialectMySQL,
				DefaultSchema: "app",
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
				t.Fatalf("classification=%q admission=%q, want read_only/admissible", result.ReadClassification, result.Admission)
			}
			if len(result.Requirements) != len(tc.wantReqs) {
				t.Fatalf("requirements=%v, want %v", result.Requirements, tc.wantReqs)
			}
			for i, got := range result.Requirements {
				if got != tc.wantReqs[i] {
					t.Errorf("requirements[%d]=%+v, want %+v", i, got, tc.wantReqs[i])
				}
			}
			if err := conn.PingContext(t.Context()); err != nil {
				t.Fatalf("caller connection after analysis: %v", err)
			}
			if err := conn.Close(); err != nil {
				t.Fatalf("caller close: %v", err)
			}
		})
	}
}

func TestMySQLTiDBQueryAccessSession_RemainsFailClosedForUnknownFunction(t *testing.T) {
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
		SQL:           "SELECT app_specific_rollup(id) FROM app.users",
		Dialect:       DialectMySQL,
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.ReadClassification != QueryAccessIndeterminate || result.Admission != QueryAccessIndeterminateAdmission {
		t.Fatalf("unknown function promoted: classification=%q admission=%q", result.ReadClassification, result.Admission)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// relationlessLiteralShapes enumerates the twelve exact relationless (no FROM)
// literal-only shapes admitted by this milestone. Marker literals prove no
// literal payload leaks into the result contract.
func relationlessLiteralShapes() []struct{ name, sql string } {
	return []struct{ name, sql string }{
		{"lower", "SELECT LOWER('SECRET_LITERAL')"},
		{"upper", "SELECT UPPER('SECRET_LITERAL')"},
		{"length", "SELECT LENGTH('SECRET_LITERAL')"},
		{"char_length", "SELECT CHAR_LENGTH('SECRET_LITERAL')"},
		{"abs", "SELECT ABS(42)"},
		{"ceil", "SELECT CEIL(42)"},
		{"ceiling", "SELECT CEILING(42)"},
		{"floor", "SELECT FLOOR(42)"},
		{"count_literal", "SELECT COUNT(1)"},
		{"coalesce_const_const", "SELECT COALESCE('SECRET_LITERAL', 'SECRET_LITERAL2')"},
		{"nullif_const_const", "SELECT NULLIF('SECRET_LITERAL', 'SECRET_LITERAL2')"},
		{"ifnull_const_const", "SELECT IFNULL('SECRET_LITERAL', 'SECRET_LITERAL2')"},
	}
}

func TestMySQLTiDBQueryAccessSession_PromotesRelationlessLiteralShapes(t *testing.T) {
	for _, shape := range relationlessLiteralShapes() {
		t.Run(shape.name, func(t *testing.T) {
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
				SQL:           shape.sql,
				Dialect:       DialectMySQL,
				DefaultSchema: "app",
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
				t.Fatalf("%s classification=%q admission=%q, want read_only/admissible", shape.name, result.ReadClassification, result.Admission)
			}
			if len(result.Requirements) != 0 {
				t.Errorf("%s requirements: got %d (%v), want 0", shape.name, len(result.Requirements), result.Requirements)
			}
			if len(result.Relations) != 0 {
				t.Errorf("%s relations: got %d (%v), want 0", shape.name, len(result.Relations), result.Relations)
			}
			if len(result.ReferencedColumns) != 0 {
				t.Errorf("%s referenced_columns: got %d (%v), want 0", shape.name, len(result.ReferencedColumns), result.ReferencedColumns)
			}
			if len(result.Unresolved) != 0 {
				t.Errorf("%s unresolved: got %d (%v), want 0", shape.name, len(result.Unresolved), result.Unresolved)
			}
			dump := fmt.Sprintf("%+v", result)
			if strings.Contains(dump, "SECRET_LITERAL") {
				t.Errorf("%s leaked SECRET_LITERAL in struct dump", shape.name)
			}
			data, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("%s marshal: %v", shape.name, err)
			}
			if strings.Contains(string(data), "SECRET_LITERAL") {
				t.Errorf("%s leaked SECRET_LITERAL in JSON", shape.name)
			}
			if err := conn.PingContext(t.Context()); err != nil {
				t.Fatalf("%s caller connection after analysis: %v", shape.name, err)
			}
			if err := conn.Close(); err != nil {
				t.Fatalf("%s caller close: %v", shape.name, err)
			}
		})
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

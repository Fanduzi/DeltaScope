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

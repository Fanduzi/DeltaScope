//go:build postgresql && integration

// Package deltascope verifies the unified online session entry against a real
// PostgreSQL 17 backend.
// input: real PostgreSQL connection from Docker PG17, including foreign-table metadata
// output: unified PG17 routing, aggregate/comparison/parse-failure/fail-closed semantics, equivalence, same-backend-session proof, foreign-table fail-closed, caller ownership
// pos: integration test coverage for the unified online PG17 entry
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestUnifiedSession_PostgreSQLCountOneAdmissible proves the exact supported
// COUNT(1) shape through the unified entry remains read_only + admissible with
// one read_table requirement against a real PG17 backend.
func TestUnifiedSession_PostgreSQLCountOneAdmissible(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewOnlineQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewOnlineQueryAccessSessionFromConn: %v", err)
	}

	result, err := AnalyzeOnlineQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:           "SELECT COUNT(1) FROM app.orders",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("AnalyzeOnlineQueryAccessWithSession: %v", err)
	}
	if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
		t.Fatalf("COUNT(1) must be admissible through unified entry: classification=%s admission=%s reasons=%v",
			result.ReadClassification, result.Admission, result.ReasonCodes)
	}
	if len(result.Relations) != 1 || result.Relations[0].Schema != "app" || result.Relations[0].Name != "orders" {
		t.Fatalf("unexpected relations: %+v", result.Relations)
	}
	if len(result.ReferencedColumns) != 0 {
		t.Fatalf("COUNT(1) must not require columns: %+v", result.ReferencedColumns)
	}
	if len(result.Requirements) != 1 || result.Requirements[0].Object != "app.orders" || result.Requirements[0].Privilege != "read_table" {
		t.Fatalf("COUNT(1) requirements: %+v", result.Requirements)
	}
	assertNoLeak(t, result)
}

// TestUnifiedSession_PostgreSQLSemanticMatrix makes the unified entry the
// direct owner of the PG17 aggregate, comparison, and fail-closed rows that
// remain separately covered by the deprecated API only for compatibility.
func TestUnifiedSession_PostgreSQLSemanticMatrix(t *testing.T) {
	type testCase struct {
		name         string
		sql          string
		admission    QueryAccessAdmission
		requirements []QueryAccessRequirement
	}
	cases := []testCase{
		{name: "count_star", sql: "SELECT count(*) FROM app.users", admission: QueryAccessAdmissible},
		{name: "typed_aggregates", sql: "SELECT count(amount), sum(amount), avg(amount), min(amount), max(amount) FROM app.orders", admission: QueryAccessAdmissible},
		{name: "row_number", sql: "SELECT row_number() OVER (PARTITION BY user_id ORDER BY amount) FROM app.orders", admission: QueryAccessAdmissible},
		{name: "rank_dense_rank", sql: "SELECT rank() OVER (ORDER BY amount), dense_rank() OVER (ORDER BY amount) FROM app.orders", admission: QueryAccessAdmissible},
		{name: "comparison", sql: "SELECT u.id FROM app.users u JOIN app.orders o ON u.id = o.user_id", admission: QueryAccessAdmissible, requirements: []QueryAccessRequirement{{Object: "app.orders", Privilege: "read_table"}, {Object: "app.orders.user_id", Privilege: "read_column"}, {Object: "app.users", Privilege: "read_table"}, {Object: "app.users.id", Privilege: "read_column"}}},
		{name: "sum_filter", sql: "SELECT sum(amount) FILTER (WHERE amount > 0) FROM app.orders", admission: QueryAccessIndeterminateAdmission},
		{name: "sum_distinct", sql: "SELECT sum(DISTINCT amount) FROM app.orders", admission: QueryAccessIndeterminateAdmission},
		{name: "unqualified_relation", sql: "SELECT id FROM users WHERE id = 1", admission: QueryAccessIndeterminateAdmission},
		{name: "literal_comparison", sql: "SELECT id FROM app.users WHERE id = 1", admission: QueryAccessIndeterminateAdmission},
		{name: "parse_failure", sql: "SELECT * FROM WHERE MALFORMED_SQL_MARKER_7f3a", admission: QueryAccessIndeterminateAdmission},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t)
			defer db.Close()
			conn, err := db.Conn(t.Context())
			if err != nil {
				t.Fatalf("conn: %v", err)
			}
			defer conn.Close()
			session, err := NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
			if err != nil {
				t.Fatalf("NewOnlineQueryAccessSessionFromConn: %v", err)
			}
			result, err := AnalyzeOnlineQueryAccessWithSession(t.Context(), session, QueryAccessRequest{SQL: tc.sql, DefaultSchema: "app"})
			if err != nil {
				t.Fatalf("AnalyzeOnlineQueryAccessWithSession: %v", err)
			}
			if result.Admission != tc.admission {
				t.Fatalf("admission=%s, want %s; classification=%s reasons=%v", result.Admission, tc.admission, result.ReadClassification, result.ReasonCodes)
			}
			if tc.admission == QueryAccessAdmissible && result.ReadClassification != QueryAccessReadOnly {
				t.Fatalf("classification=%s, want read_only", result.ReadClassification)
			}
			if tc.admission == QueryAccessIndeterminateAdmission && result.ReadClassification != QueryAccessIndeterminate {
				t.Fatalf("classification=%s, want indeterminate", result.ReadClassification)
			}
			if tc.requirements != nil && !reflect.DeepEqual(result.Requirements, tc.requirements) {
				t.Fatalf("requirements=%+v, want %+v", result.Requirements, tc.requirements)
			}
			if tc.name == "parse_failure" {
				// A PostgreSQL parser failure has no extracted candidate, requirement,
				// or public reason code; it must remain bounded and opaque.
				if result.ReadClassification != QueryAccessIndeterminate || len(result.Requirements) != 0 || len(result.ReasonCodes) != 0 {
					t.Fatalf("parse failure must stay bounded: classification=%s requirements=%+v reasons=%v", result.ReadClassification, result.Requirements, result.ReasonCodes)
				}
				data, err := json.Marshal(result)
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				dump := fmt.Sprintf("%+v", result)
				if strings.Contains(dump, tc.sql) || strings.Contains(dump, "MALFORMED_SQL_MARKER_7f3a") || strings.Contains(string(data), tc.sql) || strings.Contains(string(data), "MALFORMED_SQL_MARKER_7f3a") {
					t.Fatalf("result leaked malformed SQL or marker: dump=%s json=%s", dump, data)
				}
			}
			assertNoLeak(t, result)
		})
	}
}

// TestUnifiedSession_PostgreSQLExcludedShapesRemainIndeterminate proves the
// unified entry keeps excluded COUNT variants fail-closed against a real PG17
// backend, including casts, parameters, modifiers, relationless queries,
// multiple/nonphysical relations, and foreign tables.
func TestUnifiedSession_PostgreSQLExcludedShapesRemainIndeterminate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewOnlineQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewOnlineQueryAccessSessionFromConn: %v", err)
	}

	queries := []string{
		"SELECT COUNT(NULL) FROM app.orders",
		"SELECT COUNT(2) FROM app.orders",
		"SELECT COUNT('1') FROM app.orders",
		"SELECT COUNT(1::integer) FROM app.orders",
		"SELECT COUNT($1) FROM app.orders",
		"SELECT COUNT(1 + 0) FROM app.orders",
		"SELECT COUNT(1) FILTER (WHERE true) FROM app.orders",
		"SELECT COUNT(1) OVER () FROM app.orders",
		"SELECT COUNT(1) FROM app.orders WHERE true",
		"SELECT COUNT(1) FROM app.orders GROUP BY 1",
		"SELECT COUNT(1) FROM app.orders ORDER BY 1",
		"SELECT COUNT(1) FROM app.orders LIMIT 1",
		"SELECT COUNT(1)",
		"SELECT COUNT(1) FROM app.orders JOIN app.users ON true",
		"SELECT COUNT(1) FROM app.orders, app.users",
		"SELECT COUNT(1) FROM app.user_summary",
		"SELECT COUNT(1) FROM app.remote_orders",
		"WITH source AS (SELECT id FROM app.orders) SELECT COUNT(1) FROM source",
		"SELECT COUNT(1) FROM (SELECT id FROM app.orders) AS source",
		"SELECT COUNT(1) FROM orders",
		"SELECT COUNT(1), * FROM app.orders",
		"SELECT COUNT(1) FROM app.missing_relation",
		"SELECT COUNT(1) FROM app.orders UNION ALL SELECT COUNT(1) FROM app.orders",
	}
	for _, sqlText := range queries {
		sqlText := sqlText
		t.Run(sqlText, func(t *testing.T) {
			result, err := AnalyzeOnlineQueryAccessWithSession(ctx, session, QueryAccessRequest{
				SQL:           sqlText,
				DefaultSchema: "app",
			})
			if err != nil {
				t.Fatalf("AnalyzeOnlineQueryAccessWithSession: %v", err)
			}
			if result.ReadClassification != QueryAccessIndeterminate || result.Admission != QueryAccessIndeterminateAdmission {
				t.Fatalf("excluded COUNT(1) shape was promoted through unified entry: classification=%s admission=%s requirements=%+v reasons=%v",
					result.ReadClassification, result.Admission, result.Requirements, result.ReasonCodes)
			}
			assertNoLeak(t, result)
		})
	}
}

// TestUnifiedSession_PostgreSQLForeignTableFailClosed proves foreign-table
// rejection happens before trusted COUNT proof through the unified entry.
func TestUnifiedSession_PostgreSQLForeignTableFailClosed(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewOnlineQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewOnlineQueryAccessSessionFromConn: %v", err)
	}

	result, err := AnalyzeOnlineQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:           "SELECT COUNT(1) FROM app.remote_orders",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("AnalyzeOnlineQueryAccessWithSession: %v", err)
	}
	if result.ReadClassification != QueryAccessIndeterminate || result.Admission != QueryAccessIndeterminateAdmission {
		t.Fatalf("foreign table must remain indeterminate: classification=%s admission=%s reasons=%v",
			result.ReadClassification, result.Admission, result.ReasonCodes)
	}
	assertNoLeak(t, result)
}

// TestUnifiedSession_SameConnectionPG17 proves identity, schema metadata, and
// function identity probes use the caller-owned backend session: the backend
// PID is unchanged across construction and analysis, and the session stores
// the same *sql.Conn pointer.
func TestUnifiedSession_SameConnectionPG17(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()

	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}

	var callerPIDBefore int64
	if err := conn.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&callerPIDBefore); err != nil {
		t.Fatalf("caller PID before: %v", err)
	}
	if callerPIDBefore == 0 {
		t.Fatal("expected non-zero caller PID")
	}

	session, err := NewOnlineQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewOnlineQueryAccessSessionFromConn: %v", err)
	}
	if session.conn != conn {
		t.Fatal("session.conn must be the same *sql.Conn pointer as input")
	}

	result, err := AnalyzeOnlineQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:           "SELECT count(*) FROM app.users",
		Mode:          QueryAccessModeStrict,
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("AnalyzeOnlineQueryAccessWithSession: %v", err)
	}
	if result.Admission != QueryAccessAdmissible {
		t.Fatalf("count(*) must be admissible through unified entry: classification=%s admission=%s reasons=%v",
			result.ReadClassification, result.Admission, result.ReasonCodes)
	}

	var callerPIDAfter int64
	if err := conn.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&callerPIDAfter); err != nil {
		t.Fatalf("caller PID after: %v", err)
	}
	if callerPIDAfter != callerPIDBefore {
		t.Errorf("backend PID changed: before=%d after=%d", callerPIDBefore, callerPIDAfter)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("caller close: %v", err)
	}

	t.Logf("unified same-connection proof: pid_before=%d pid_after=%d pointer_match=true analyze_ok=true",
		callerPIDBefore, callerPIDAfter)
}

// TestUnifiedSession_MatchesLegacyPG17 proves one real PG17 routing case keeps
// the unified entry equivalent to the existing PostgreSQL session API.
func TestUnifiedSession_MatchesLegacyPG17(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	unifiedSession, err := NewOnlineQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("unified new session: %v", err)
	}
	legacySession, err := NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("legacy new session: %v", err)
	}

	queries := []string{"SELECT COUNT(1) FROM app.orders"}
	for _, sqlText := range queries {
		sqlText := sqlText
		t.Run(sqlText, func(t *testing.T) {
			unifiedResult, unifiedErr := AnalyzeOnlineQueryAccessWithSession(ctx, unifiedSession, QueryAccessRequest{
				SQL:           sqlText,
				DefaultSchema: "app",
			})
			legacyResult, legacyErr := AnalyzePostgreSQLQueryAccessWithSession(ctx, legacySession, QueryAccessRequest{
				SQL:           sqlText,
				Dialect:       DialectPostgreSQL,
				Mode:          QueryAccessModeStrict,
				DefaultSchema: "app",
			})
			if (unifiedErr == nil) != (legacyErr == nil) {
				t.Fatalf("error mismatch: unified=%v legacy=%v", unifiedErr, legacyErr)
			}
			if unifiedErr != nil {
				if unifiedErr.Error() != legacyErr.Error() {
					t.Fatalf("error text mismatch: unified=%q legacy=%q", unifiedErr.Error(), legacyErr.Error())
				}
				return
			}
			if !reflect.DeepEqual(unifiedResult, legacyResult) {
				t.Fatalf("results differ:\nunified=%+v\nlegacy =%+v", unifiedResult, legacyResult)
			}
		})
	}
}

//go:build postgresql

// Package deltascope verifies PostgreSQL 17 routing through the unified online session entry.
// input: caller-owned *sql.Conn backed by the recording driver, including relkind='f' responses and count-lookup failures
// output: unified PG17 constructor/analysis routing, exact COUNT(1) admission, excluded-shape fail-closed, no-execution/no-leak, and legacy API equivalence
// pos: postgresql-tagged unified online PG17 routing contract tests
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	online "github.com/Fanduzi/DeltaScope/internal/application/online"
)

// TestOnlineQueryAccessSession_PostgreSQLRoutesThroughUnifiedEntry proves the
// unified constructor accepts a PostgreSQL 17 connection and the unified entry
// routes the exact COUNT(1) envelope through the existing trusted proof core.
func TestOnlineQueryAccessSession_PostgreSQLRoutesThroughUnifiedEntry(t *testing.T) {
	const marker = "SQLNOTEXEC_MARKER_7f3a"
	recorder := &postgresqlRecordingDriver{}
	db := openPostgreSQLRecordingDB(t, recorder)
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
	if session == nil {
		t.Fatal("expected non-nil unified session for PG17")
	}

	result, err := AnalyzeOnlineQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:           "SELECT COUNT(1) /* " + marker + " */ FROM app.orders",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("AnalyzeOnlineQueryAccessWithSession: %v", err)
	}
	if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
		t.Fatalf("COUNT(1) must be admissible through unified entry: classification=%s admission=%s reasons=%v requirements=%+v",
			result.ReadClassification, result.Admission, result.ReasonCodes, result.Requirements)
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
	if len(recorder.recorded()) == 0 {
		t.Fatal("expected safe session/catalog probes")
	}
	for _, query := range recorder.recorded() {
		if strings.Contains(query, marker) {
			t.Fatalf("analysis SQL reached driver: %q", query)
		}
	}
	assertPostgreSQLRecordingSequence(t, recorder.recorded())
	assertPostgreSQLRecordingProbes(t, recorder.recorded())
	assertPostgreSQLRecordingNoLeak(t, result, marker)
}

// TestOnlineQueryAccessSession_PostgreSQLExcludedCountShapesFailClosed proves
// the unified entry keeps excluded COUNT(1) shapes fail-closed (indeterminate)
// exactly like the existing PostgreSQL session API.
func TestOnlineQueryAccessSession_PostgreSQLExcludedCountShapesFailClosed(t *testing.T) {
	recorder := &postgresqlRecordingDriver{}
	db := openPostgreSQLRecordingDB(t, recorder)
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
		"SELECT COUNT(1) FROM (SELECT id FROM app.orders) AS source",
		"SELECT COUNT(1) FROM orders",
		"SELECT COUNT(1), * FROM app.orders",
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
			assertPostgreSQLRecordingNoLeak(t, result)
		})
	}
}

// TestOnlineQueryAccessSession_PostgreSQLForeignTableFailClosed proves foreign
// tables fail closed before trusted COUNT proof through the unified entry.
func TestOnlineQueryAccessSession_PostgreSQLForeignTableFailClosed(t *testing.T) {
	const marker = "SQLNOTEXEC_FOREIGN_MARKER_9c2b"
	recorder := &postgresqlRecordingDriver{relkind: "f"}
	db := openPostgreSQLRecordingDB(t, recorder)
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
		SQL:           "SELECT COUNT(1) /* " + marker + " */ FROM app.remote_orders",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("AnalyzeOnlineQueryAccessWithSession: %v", err)
	}
	if result.ReadClassification != QueryAccessIndeterminate || result.Admission != QueryAccessIndeterminateAdmission {
		t.Fatalf("foreign table must remain indeterminate through unified entry: classification=%s admission=%s reasons=%v",
			result.ReadClassification, result.Admission, result.ReasonCodes)
	}
	for _, query := range recorder.recorded() {
		if strings.Contains(query, marker) || strings.Contains(strings.ToLower(query), "remote_orders") {
			t.Fatalf("analysis SQL reached driver: %q", query)
		}
		if strings.Contains(query, "with any_type as") {
			t.Fatalf("foreign-table fail-closed must not reach COUNT catalog proof: %q", query)
		}
	}
	for _, pattern := range []string{
		"SELECT VERSION()",
		"select c.relkind",
	} {
		found := false
		for _, query := range recorder.recorded() {
			if strings.Contains(query, pattern) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing safe probe %q in %v", pattern, recorder.recorded())
		}
	}
	assertPostgreSQLRecordingNoLeak(t, result, marker, "foreign", "relkind", "pg_foreign")
}

// TestOnlineQueryAccessSession_PostgreSQLCountLookupFailureNoLeak proves the
// unified entry keeps dedicated COUNT lookup failures bounded and indeterminate.
func TestOnlineQueryAccessSession_PostgreSQLCountLookupFailureNoLeak(t *testing.T) {
	const marker = "SQLNOTEXEC_MARKER_7f3a"
	const driverError = "catalog lookup driver error oid=987654321 database_oid=876543210 backend_pid=765432109 dsn=postgres://leak_user:leak_password@leak-host:6543/leak_db"
	recorder := &postgresqlRecordingDriver{countLookupErr: errors.New(driverError)}
	db := openPostgreSQLRecordingDB(t, recorder)
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
		SQL:           "SELECT COUNT(1) /* " + marker + " */ FROM app.orders",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("AnalyzeOnlineQueryAccessWithSession: %v", err)
	}
	if result.ReadClassification != QueryAccessIndeterminate || result.Admission != QueryAccessIndeterminateAdmission {
		t.Fatalf("catalog failure must remain indeterminate through unified entry: classification=%s admission=%s reasons=%v",
			result.ReadClassification, result.Admission, result.ReasonCodes)
	}
	if !containsPostgreSQLReasonCode(result.ReasonCodes, "unproven_function_effect") {
		t.Fatalf("expected bounded unproven function reason, got %v", result.ReasonCodes)
	}
	if containsPostgreSQLReasonCode(result.ReasonCodes, "identity_lookup_failed") {
		t.Fatalf("internal identity failure detail leaked into public reasons: %v", result.ReasonCodes)
	}
	for _, query := range recorder.recorded() {
		if strings.Contains(query, marker) {
			t.Fatalf("analysis SQL reached driver: %q", query)
		}
	}
	assertPostgreSQLRecordingProbes(t, recorder.recorded())
	assertPostgreSQLRecordingNoLeak(t, result, marker, driverError, "987654321", "876543210", "765432109", "leak_user", "leak_password", "leak-host", "6543", "leak_db")
}

// TestOnlineQueryAccessSession_PostgreSQLMatchesLegacyAPI proves one routing
// case keeps the unified entry equivalent to the existing PostgreSQL session API.
func TestOnlineQueryAccessSession_PostgreSQLMatchesLegacyAPI(t *testing.T) {
	const marker = "PG_NO_EXEC_NO_LEAK_EQUIV_MARKER"
	queries := []string{"SELECT COUNT(1) FROM app.orders"}
	for _, sqlText := range queries {
		sqlText := sqlText + " /* " + marker + " */"
		t.Run(sqlText, func(t *testing.T) {
			recorder := &postgresqlRecordingDriver{}
			db := openPostgreSQLRecordingDB(t, recorder)
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

			analysisStart := len(recorder.recorded())
			unifiedResult, unifiedErr := AnalyzeOnlineQueryAccessWithSession(ctx, unifiedSession, QueryAccessRequest{
				SQL:           sqlText,
				DefaultSchema: "app",
			})
			unifiedEnd := len(recorder.recorded())
			legacyResult, legacyErr := AnalyzePostgreSQLQueryAccessWithSession(ctx, legacySession, QueryAccessRequest{
				SQL:           sqlText,
				Dialect:       DialectPostgreSQL,
				Mode:          QueryAccessModeStrict,
				DefaultSchema: "app",
			})
			recorded := recorder.recorded()
			unifiedQueries := recorded[analysisStart:unifiedEnd]
			legacyQueries := recorded[unifiedEnd:]
			if !reflect.DeepEqual(unifiedQueries, legacyQueries) {
				t.Fatalf("recording queries differ:\nunified=%v\nlegacy =%v", unifiedQueries, legacyQueries)
			}
			for _, query := range append(unifiedQueries, legacyQueries...) {
				if strings.Contains(query, marker) {
					t.Fatalf("submitted SQL reached driver: %q", query)
				}
			}
			if (unifiedErr == nil) != (legacyErr == nil) {
				t.Fatalf("error mismatch: unified=%v legacy=%v", unifiedErr, legacyErr)
			}
			if unifiedErr != nil {
				if unifiedErr.Error() != legacyErr.Error() {
					t.Fatalf("error text mismatch: unified=%q legacy=%q", unifiedErr.Error(), legacyErr.Error())
				}
				if strings.Contains(unifiedErr.Error(), marker) {
					t.Fatalf("error leaked submitted SQL marker: %q", unifiedErr)
				}
				return
			}
			if !reflect.DeepEqual(unifiedResult, legacyResult) {
				t.Fatalf("results differ:\nunified=%+v\nlegacy =%+v", unifiedResult, legacyResult)
			}
			assertPostgreSQLRecordingNoLeak(t, unifiedResult, marker)
			assertPostgreSQLRecordingNoLeak(t, legacyResult, marker)
		})
	}
}

// TestOnlineQueryAccessSession_PostgreSQLValidationPriorityTagged proves the
// fixed validation order holds for a PG17 session in the tagged build: a
// non-empty dialect mismatch, profile, and resolver beat routing, while a
// linked capability passes and mode validation runs afterward.
func TestOnlineQueryAccessSession_PostgreSQLValidationPriorityTagged(t *testing.T) {
	recorder := &postgresqlRecordingDriver{}
	db := openPostgreSQLRecordingDB(t, recorder)
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
	if session.target != online.TargetPG17 {
		t.Fatalf("expected PG17 target, got %q", session.target)
	}

	invalidMode := QueryAccessMode("bogus_mode")
	profile := QueryAccessAnalysisProfileMySQL57
	resolver := onlineStubResolver{}

	cases := []struct {
		name    string
		req     QueryAccessRequest
		wantErr error
	}{
		{
			name: "dialect_mismatch_beats_profile_resolver_mode",
			req: QueryAccessRequest{
				Dialect:         DialectMySQL,
				AnalysisProfile: profile,
				SchemaResolver:  resolver,
				Mode:            invalidMode,
			},
			wantErr: ErrOnlineQueryAccessDialectMismatch,
		},
		{
			name: "profile_beats_resolver_mode",
			req: QueryAccessRequest{
				AnalysisProfile: profile,
				SchemaResolver:  resolver,
				Mode:            invalidMode,
			},
			wantErr: ErrOnlineQueryAccessProfileNotAllowed,
		},
		{
			name: "resolver_beats_mode",
			req: QueryAccessRequest{
				SchemaResolver: resolver,
				Mode:           invalidMode,
			},
			wantErr: ErrOnlineQueryAccessSchemaResolverNotAllowed,
		},
		{
			name: "capability_linked_mode_validation_last",
			req: QueryAccessRequest{
				Mode: invalidMode,
			},
			wantErr: ErrInvalidQueryAccessMode,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AnalyzeOnlineQueryAccessWithSession(ctx, session, tc.req)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

// TestOnlineQueryAccessSession_PostgreSQLCallerOwnsConnection proves the
// unified PG17 path never closes the caller-owned connection.
func TestOnlineQueryAccessSession_PostgreSQLCallerOwnsConnection(t *testing.T) {
	recorder := &postgresqlRecordingDriver{}
	db := openPostgreSQLRecordingDB(t, recorder)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}

	session, err := NewOnlineQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewOnlineQueryAccessSessionFromConn: %v", err)
	}
	if _, err := AnalyzeOnlineQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:           "SELECT COUNT(1) FROM app.orders",
		DefaultSchema: "app",
	}); err != nil {
		t.Fatalf("AnalyzeOnlineQueryAccessWithSession: %v", err)
	}
	if err := conn.PingContext(ctx); err != nil {
		t.Fatalf("caller connection after analysis: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("caller close: %v", err)
	}
}

// TestOnlineQueryAccessSession_PostgreSQLConstructorFailures proves unified
// constructor failures map to the bounded unavailable sentinel.
func TestOnlineQueryAccessSession_PostgreSQLConstructorFailures(t *testing.T) {
	t.Run("closed_connection", func(t *testing.T) {
		recorder := &postgresqlRecordingDriver{}
		db := openPostgreSQLRecordingDB(t, recorder)
		defer db.Close()

		ctx := t.Context()
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("conn: %v", err)
		}
		if err := conn.Close(); err != nil {
			t.Fatalf("caller close: %v", err)
		}
		session, err := NewOnlineQueryAccessSessionFromConn(ctx, conn)
		if !errors.Is(err, ErrOnlineQueryAccessSessionUnavailable) {
			t.Fatalf("want ErrOnlineQueryAccessSessionUnavailable, got session=%v err=%v", session, err)
		}
		if session != nil {
			t.Fatal("expected nil session")
		}
	})
}

// TestOnlineQueryAccessSession_PostgreSQLDoesNotExecuteUserSQL proves the
// unified PG17 path never sends caller-submitted analysis SQL to the database
// and only runs the bounded identity/catalog probes.
func TestOnlineQueryAccessSession_PostgreSQLDoesNotExecuteUserSQL(t *testing.T) {
	const marker = "SQLNOTEXEC_MARKER_7f3a"
	recorder := &postgresqlRecordingDriver{}
	db := openPostgreSQLRecordingDB(t, recorder)
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
		SQL:           "SELECT COUNT(1) /* " + marker + " */ FROM app.orders",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("AnalyzeOnlineQueryAccessWithSession: %v", err)
	}
	if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
		t.Fatalf("expected admitted result: classification=%s admission=%s requirements=%+v unresolved=%+v reasons=%v queries=%v",
			result.ReadClassification, result.Admission, result.Requirements, result.Unresolved, result.ReasonCodes, recorder.recorded())
	}
	if len(recorder.recorded()) == 0 {
		t.Fatal("expected safe session/catalog probes")
	}
	for _, query := range recorder.recorded() {
		if strings.Contains(query, marker) {
			t.Fatalf("analysis SQL reached driver: %q", query)
		}
	}
	assertPostgreSQLRecordingProbes(t, recorder.recorded())
	assertPostgreSQLRecordingNoLeak(t, result, marker)
}

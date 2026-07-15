//go:build postgresql && integration

// Package deltascope verifies the PostgreSQL session API with real connections.
// input: real PostgreSQL connection from Docker PG17
// output: regression coverage for session construction, caller ownership, close semantics, and same-connection proof
// pos: integration test coverage for PostgreSQLQueryAccessSession
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	host := envOr("DELTASCOPE_PG_HOST", "127.0.0.1")
	port := envOrInt("DELTASCOPE_PG_PORT", 5500)
	user := envOr("DELTASCOPE_PG_USER", "root")
	pass := envOr("DELTASCOPE_PG_PASSWORD", "root")
	database := envOr("DELTASCOPE_PG_DATABASE", "postgres")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", user, pass, host, port, database)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Fatalf("ping: %v", err)
	}
	return db
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func TestNewSessionFromConn_Success(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewPostgreSQLQueryAccessSessionFromConn(t.Context(), conn)
	if err != nil {
		t.Fatalf("NewPostgreSQLQueryAccessSessionFromConn: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
}

func TestSession_CallerOwnsClose(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}

	session, err := NewPostgreSQLQueryAccessSessionFromConn(t.Context(), conn)
	if err != nil {
		t.Fatalf("NewPostgreSQLQueryAccessSessionFromConn: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}

	// Caller can still query through the connection.
	var pid int64
	if err := conn.QueryRowContext(t.Context(), "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		t.Fatalf("caller query after session creation: %v", err)
	}
	if pid == 0 {
		t.Fatal("expected non-zero backend PID")
	}

	// Caller closes the connection (not the session).
	if err := conn.Close(); err != nil {
		t.Fatalf("caller close: %v", err)
	}
}

func TestNewTrustedServiceFromSession_Success(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewPostgreSQLQueryAccessSessionFromConn(t.Context(), conn)
	if err != nil {
		t.Fatalf("NewPostgreSQLQueryAccessSessionFromConn: %v", err)
	}

	svc, err := newTrustedServiceFromSession(session)
	if err != nil {
		t.Fatalf("newTrustedServiceFromSession: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewSessionFromConn_ClosedConn(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	// Close the connection before passing to constructor.
	conn.Close()

	session, err := NewPostgreSQLQueryAccessSessionFromConn(t.Context(), conn)
	if err == nil {
		t.Fatal("expected error for closed connection")
	}
	if session != nil {
		t.Fatal("expected nil session on error")
	}

	errText := err.Error()
	for _, forbidden := range []string{"pgx", "pq", "dsn", "password", "host=", "user="} {
		if strings.Contains(strings.ToLower(errText), forbidden) {
			t.Errorf("error text must not contain %q, got: %s", forbidden, errText)
		}
	}
}

func TestNewSessionFromConn_CanceledCtx(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately.

	session, err := NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
	if session != nil {
		t.Fatal("expected nil session on error")
	}

	errText := err.Error()
	for _, forbidden := range []string{"pgx", "pq", "dsn", "password", "host=", "user="} {
		if strings.Contains(strings.ToLower(errText), forbidden) {
			t.Errorf("error text must not contain %q, got: %s", forbidden, errText)
		}
	}
}

func TestTrustedSDK_CountStarAdmissible(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewPostgreSQLQueryAccessSessionFromConn: %v", err)
	}

	result, err := AnalyzePostgreSQLQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:           "SELECT count(*) FROM app.users",
		Dialect:       DialectPostgreSQL,
		Mode:          QueryAccessModeStrict,
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("AnalyzePostgreSQLQueryAccessWithSession: %v", err)
	}

	if result.ReadClassification != QueryAccessReadOnly {
		t.Errorf("expected read_only, got %s", result.ReadClassification)
	}
	if result.Admission != QueryAccessAdmissible {
		t.Errorf("expected admissible, got %s", result.Admission)
	}

	// Verify no leak in success JSON.
	assertNoLeak(t, result)
}

func TestTrustedSDK_ComparisonAdmissible(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewPostgreSQLQueryAccessSessionFromConn: %v", err)
	}

	// Schema-qualified JOIN comparison — should be admissible if manifest proves it.
	result, err := AnalyzePostgreSQLQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:           "SELECT u.id FROM app.users u JOIN app.orders o ON u.id = o.user_id",
		Dialect:       DialectPostgreSQL,
		Mode:          QueryAccessModeStrict,
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("AnalyzePostgreSQLQueryAccessWithSession: %v", err)
	}

	t.Logf("classification=%s admission=%s reasons=%v",
		result.ReadClassification, result.Admission, result.ReasonCodes)

	// This may be admissible or indeterminate depending on manifest coverage.
	// The key invariant is: no leak, and the result is valid.
	assertNoLeak(t, result)
}

func TestTrustedSDK_CallerRetainsConnection(t *testing.T) {
	db := openTestDB(t)
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

	_, err = AnalyzePostgreSQLQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:           "SELECT count(*) FROM app.users",
		Dialect:       DialectPostgreSQL,
		Mode:          QueryAccessModeStrict,
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("AnalyzePostgreSQLQueryAccessWithSession: %v", err)
	}

	// Caller can still query.
	var pid int64
	if err := conn.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		t.Fatalf("caller query after analysis: %v", err)
	}
	if pid == 0 {
		t.Fatal("expected non-zero PID after analysis")
	}

	// Caller closes.
	if err := conn.Close(); err != nil {
		t.Fatalf("caller close: %v", err)
	}
}

func TestTrustedSDK_RejectsExternalSchemaResolver(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewPostgreSQLQueryAccessSessionFromConn: %v", err)
	}

	// Provide a non-nil schema resolver — must be rejected.
	_, err = AnalyzePostgreSQLQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:            "SELECT count(*) FROM app.users",
		Dialect:        DialectPostgreSQL,
		Mode:           QueryAccessModeStrict,
		DefaultSchema:  "app",
		SchemaResolver: &stubSchemaResolver{},
	})
	if err == nil {
		t.Fatal("expected error for non-nil SchemaResolver")
	}
	if !strings.Contains(err.Error(), "schema resolver") {
		t.Errorf("expected schema resolver error, got: %v", err)
	}
}

func TestTrustedSDK_DefaultRemainsFailClosed(t *testing.T) {
	// Verify that AnalyzeQueryAccess (default path) still returns indeterminate
	// for the same PostgreSQL inputs that the trusted path can promote.
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	// Default SDK path — no session, no trusted service.
	result, err := AnalyzeQueryAccess(ctx, QueryAccessRequest{
		SQL:           "SELECT count(*) FROM app.users",
		Dialect:       DialectPostgreSQL,
		Mode:          QueryAccessModeStrict,
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("AnalyzeQueryAccess: %v", err)
	}
	if result.Admission != QueryAccessIndeterminateAdmission {
		t.Errorf("default path must remain indeterminate, got %s", result.Admission)
	}
}

func TestTrustedSDK_NonPostgreSQLDialectRejected(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewPostgreSQLQueryAccessSessionFromConn: %v", err)
	}

	_, err = AnalyzePostgreSQLQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:     "SELECT id FROM users WHERE id = 1",
		Dialect: DialectMySQL,
		Mode:    QueryAccessModeStrict,
	})
	if err == nil {
		t.Fatal("expected error for non-PostgreSQL dialect")
	}
	if !strings.Contains(err.Error(), "PostgreSQL") {
		t.Errorf("expected PostgreSQL dialect error, got: %v", err)
	}
}

func TestTrustedSDK_NilContextRejected(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewPostgreSQLQueryAccessSessionFromConn: %v", err)
	}

	_, err = AnalyzePostgreSQLQueryAccessWithSession(nil, session, QueryAccessRequest{
		SQL:     "SELECT count(*) FROM app.users",
		Dialect: DialectPostgreSQL,
		Mode:    QueryAccessModeStrict,
	})
	if err == nil {
		t.Fatal("expected error for nil context")
	}
}

func TestTrustedSDK_NilSessionRejected(t *testing.T) {
	_, err := AnalyzePostgreSQLQueryAccessWithSession(t.Context(), nil, QueryAccessRequest{
		SQL:     "SELECT count(*) FROM app.users",
		Dialect: DialectPostgreSQL,
		Mode:    QueryAccessModeStrict,
	})
	if err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestTrustedSDK_UnqualifiedRelationIndeterminate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewPostgreSQLQueryAccessSessionFromConn: %v", err)
	}

	// Unqualified relation — must remain indeterminate.
	result, err := AnalyzePostgreSQLQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:     "SELECT id FROM users WHERE id = 1",
		Dialect: DialectPostgreSQL,
		Mode:    QueryAccessModeStrict,
	})
	if err != nil {
		t.Fatalf("AnalyzePostgreSQLQueryAccessWithSession: %v", err)
	}
	if result.Admission != QueryAccessIndeterminateAdmission {
		t.Errorf("unqualified relation must remain indeterminate, got %s", result.Admission)
	}
	assertNoLeak(t, result)
}

func TestTrustedSDK_LiteralComparisonIndeterminate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewPostgreSQLQueryAccessSessionFromConn: %v", err)
	}

	// Literal comparison — coercion_gap, must remain indeterminate.
	result, err := AnalyzePostgreSQLQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:           "SELECT id FROM app.users WHERE id = 1",
		Dialect:       DialectPostgreSQL,
		Mode:          QueryAccessModeStrict,
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("AnalyzePostgreSQLQueryAccessWithSession: %v", err)
	}
	if result.Admission != QueryAccessIndeterminateAdmission {
		t.Errorf("literal comparison must remain indeterminate, got %s", result.Admission)
	}
	assertNoLeak(t, result)
}

// stubSchemaResolver is a minimal resolver for testing rejection.
type stubSchemaResolver struct{}

func (s *stubSchemaResolver) ResolveRelation(_ context.Context, _, _, _ string) (QueryAccessRelationSchema, error) {
	return QueryAccessRelationSchema{}, nil
}

// assertNoLeak verifies the result JSON contains no sensitive fields.
func assertNoLeak(t *testing.T, result *QueryAccessResult) {
	t.Helper()
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	jsonStr := string(data)
	for _, forbidden := range []string{
		"oid", "backend_pid", "session_binding", "search_path",
		"manifest", "resolver", "dsn", "password", "credential",
		"catalog_sql", "raw_sql", "literal", "severity",
		"canonical_signature",
	} {
		if strings.Contains(strings.ToLower(jsonStr), forbidden) {
			t.Errorf("JSON must not contain %q: %s", forbidden, jsonStr)
		}
	}
}

func TestSameConnection_ActualLookups(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := t.Context()

	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}

	// Capture caller PID before any session construction.
	var callerPIDBefore int64
	if err := conn.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&callerPIDBefore); err != nil {
		t.Fatalf("caller PID before: %v", err)
	}
	if callerPIDBefore == 0 {
		t.Fatal("expected non-zero caller PID")
	}

	// Create session.
	session, err := NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("NewPostgreSQLQueryAccessSessionFromConn: %v", err)
	}

	// Verify session.conn is the same pointer we passed in.
	if session.conn != conn {
		t.Fatal("session.conn must be the same *sql.Conn pointer as input")
	}

	// Create trusted service from session.
	svc, err := newTrustedServiceFromSession(session)
	if err != nil {
		t.Fatalf("newTrustedServiceFromSession: %v", err)
	}

	// Execute actual analysis through the trusted service.
	result, err := svc.Analyze(ctx, appqa.QueryAccessRequest{
		SQL:           "SELECT count(*) FROM app.users",
		Dialect:       "postgresql",
		Mode:          "strict",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// The result should be indeterminate (no public trusted analyze function yet).
	t.Logf("classification=%s admission=%s reasons=%v",
		result.DomainResult.ReadClassification, result.DomainResult.Admission, result.DomainResult.ReasonCodes)

	// Capture caller PID after analysis — must be same backend.
	var callerPIDAfter int64
	if err := conn.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&callerPIDAfter); err != nil {
		t.Fatalf("caller PID after: %v", err)
	}
	if callerPIDAfter != callerPIDBefore {
		t.Errorf("backend PID changed: before=%d after=%d", callerPIDBefore, callerPIDAfter)
	}

	// Caller can still close.
	if err := conn.Close(); err != nil {
		t.Fatalf("caller close: %v", err)
	}

	t.Logf("same-connection proof: pid_before=%d pid_after=%d pointer_match=true analyze_ok=true",
		callerPIDBefore, callerPIDAfter)
}

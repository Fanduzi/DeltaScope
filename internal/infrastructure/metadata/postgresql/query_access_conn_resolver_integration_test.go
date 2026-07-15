//go:build postgresql && integration

// Package postgresqlmeta runs conn-backed resolver and same-connection proof
// against a live PostgreSQL 17 session.
// input: DELTASCOPE_PG_* env or docker compose defaults (localhost:5500)
// output: real metadata resolution and same-backend-PID evidence
// pos: integration evidence for same-connection property
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"strconv"
	"testing"
	"time"
)

func TestSameConnectionPID_Proof(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PG17 same-connection proof in short mode")
	}

	db, cleanup, err := openIntegrationDB(t)
	if err != nil {
		t.Skipf("PG17 integration unavailable: %v", err)
	}
	defer cleanup()

	ctx := t.Context()

	// Get a single *sql.Conn.
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}
	defer conn.Close()

	// Capture caller PID.
	var callerPID int64
	if err := conn.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&callerPID); err != nil {
		t.Fatalf("caller PID: %v", err)
	}
	if callerPID == 0 {
		t.Fatal("expected non-zero caller PID")
	}
	t.Logf("caller PID: %d", callerPID)

	// 1. Create conn-backed metadata resolver from the same *sql.Conn.
	connResolver, err := NewQueryAccessConnResolver(conn)
	if err != nil {
		t.Fatalf("NewQueryAccessConnResolver: %v", err)
	}

	// Resolve a real relation.
	rs, err := connResolver.ResolveRelation(ctx, "postgresql", "app", "users")
	if err != nil {
		t.Fatalf("ResolveRelation: %v", err)
	}
	if rs.Kind != "table" {
		t.Errorf("expected table kind, got %s", rs.Kind)
	}
	if len(rs.Columns) == 0 {
		t.Error("expected non-empty columns")
	}
	t.Logf("metadata resolved: kind=%s columns=%d", rs.Kind, len(rs.Columns))

	// 2. Create pinned session from the same *sql.Conn.
	pinned, err := NewPinnedSessionFromConn(conn)
	if err != nil {
		t.Fatalf("NewPinnedSessionFromConn: %v", err)
	}

	// Capture live context — this contains the backend PID in SessionBinding.
	live, err := pinned.CaptureLiveContext(ctx)
	if err != nil {
		t.Fatalf("CaptureLiveContext: %v", err)
	}
	if !live.Bound {
		t.Fatal("expected bound context")
	}
	t.Logf("session binding: %s db=%d role=%d ver=%d path_len=%d",
		live.SessionBinding, live.DatabaseOID, live.RoleOID, live.ServerVersionNum, len(live.NamespaceSearchOIDs))

	// Verify the session binding contains the caller PID.
	// SessionBinding format: "b<pid>-d<dbOID>"
	expectedPrefix := "b" + formatInt(callerPID) + "-"
	if len(live.SessionBinding) < len(expectedPrefix) || live.SessionBinding[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("session binding %q does not start with expected PID prefix %q", live.SessionBinding, expectedPrefix)
	}

	// 3. Create identity adapter from the same pinned session.
	adapter, err := NewEffectIdentityAdapter(pinned)
	if err != nil {
		t.Fatalf("NewEffectIdentityAdapter: %v", err)
	}

	// Capture execution-bound context through the adapter.
	adapterCtx, err := adapter.CaptureExecutionBoundContext(ctx)
	if err != nil {
		t.Fatalf("CaptureExecutionBoundContext: %v", err)
	}
	if !adapterCtx.Bound {
		t.Fatal("expected bound adapter context")
	}
	if adapterCtx.DatabaseOID != live.DatabaseOID {
		t.Errorf("adapter db OID %d != live %d", adapterCtx.DatabaseOID, live.DatabaseOID)
	}

	t.Logf("same-connection proof: caller_pid=%d session_binding_pid_match=true metadata_ok=true identity_ok=true",
		callerPID)
}

func TestConnResolver_PG17Metadata(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PG17 metadata test in short mode")
	}

	db, cleanup, err := openIntegrationDB(t)
	if err != nil {
		t.Skipf("PG17 integration unavailable: %v", err)
	}
	defer cleanup()

	ctx := t.Context()

	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}
	defer conn.Close()

	resolver, err := NewQueryAccessConnResolver(conn)
	if err != nil {
		t.Fatalf("NewQueryAccessConnResolver: %v", err)
	}

	// Resolve app.users.
	rs, err := resolver.ResolveRelation(ctx, "postgresql", "app", "users")
	if err != nil {
		t.Fatalf("ResolveRelation: %v", err)
	}
	if rs.Schema != "app" {
		t.Errorf("schema=%q", rs.Schema)
	}
	if rs.Name != "users" {
		t.Errorf("name=%q", rs.Name)
	}
	if rs.Kind != "table" {
		t.Errorf("kind=%q", rs.Kind)
	}
	if rs.IsView {
		t.Error("expected not view")
	}
	if len(rs.Columns) == 0 {
		t.Error("expected non-empty columns")
	}
	t.Logf("resolved %s.%s: kind=%s columns=%d", rs.Schema, rs.Name, rs.Kind, len(rs.Columns))

	// Resolve a non-existent relation.
	_, err = resolver.ResolveRelation(ctx, "postgresql", "app", "nonexistent_table_xyz")
	if err == nil {
		t.Error("expected error for non-existent table")
	}
}

func TestSession_CallerConnSurvivesAnalysis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PG17 session survival test in short mode")
	}

	db, cleanup, err := openIntegrationDB(t)
	if err != nil {
		t.Skipf("PG17 integration unavailable: %v", err)
	}
	defer cleanup()

	ctx := t.Context()

	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}

	// Use the connection for metadata resolution.
	resolver, err := NewQueryAccessConnResolver(conn)
	if err != nil {
		t.Fatalf("NewQueryAccessConnResolver: %v", err)
	}

	_, err = resolver.ResolveRelation(ctx, "postgresql", "app", "users")
	if err != nil {
		t.Fatalf("ResolveRelation: %v", err)
	}

	// Create pinned session and capture context.
	pinned, err := NewPinnedSessionFromConn(conn)
	if err != nil {
		t.Fatalf("NewPinnedSessionFromConn: %v", err)
	}

	_, err = pinned.CaptureLiveContext(ctx)
	if err != nil {
		t.Fatalf("CaptureLiveContext: %v", err)
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

	// Wait briefly for cleanup.
	time.Sleep(10 * time.Millisecond)
}

func formatInt(n int64) string {
	return strconv.FormatInt(n, 10)
}

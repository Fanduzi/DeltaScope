//go:build postgresql

// Package deltascope verifies the PostgreSQL session API with real connections.
// input: real PostgreSQL connection from Docker PG17
// output: regression coverage for session construction, caller ownership, and close semantics
// pos: postgresql-tagged test coverage for PostgreSQLQueryAccessSession
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

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

	session, err := NewPostgreSQLQueryAccessSessionFromConn(conn)
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

	session, err := NewPostgreSQLQueryAccessSessionFromConn(conn)
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

	session, err := NewPostgreSQLQueryAccessSessionFromConn(conn)
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

func TestNewTrustedServiceFromSession_NilSession(t *testing.T) {
	svc, err := newTrustedServiceFromSession(nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
	if svc != nil {
		t.Fatal("expected nil service on error")
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

	session, err := NewPostgreSQLQueryAccessSessionFromConn(conn)
	if err == nil {
		t.Fatal("expected error for closed connection")
	}
	if session != nil {
		t.Fatal("expected nil session on error")
	}
}

//go:build !postgresql

// Package deltascope verifies the unified online session compiles without the
// postgresql tag and fails closed for an observed PostgreSQL target.
// input: caller-owned *sql.Conn backed by the stub driver reporting PostgreSQL 17 identity
// output: ErrOnlineQueryAccessCapabilityUnsupported for PG17, unchanged legacy PostgreSQL stubs, bounded no-leak text
// pos: no-tag source-compatibility contract for unified online PG17 fail-closed routing
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	online "github.com/Fanduzi/DeltaScope/internal/application/online"
)

// TestOnlineQueryAccessSession_NoTagPG17CapabilityUnsupported proves the same
// unified public API compiles without the postgresql tag and an observed
// PostgreSQL 17 target fails with the capability sentinel, not a leak.
func TestOnlineQueryAccessSession_NoTagPG17CapabilityUnsupported(t *testing.T) {
	db := openOnlineStubDB(t, onlineStubConfig{version: "PostgreSQL 17.4"})
	defer db.Close()

	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
	if !errors.Is(err, ErrOnlineQueryAccessCapabilityUnsupported) {
		t.Fatalf("want ErrOnlineQueryAccessCapabilityUnsupported, got session=%v err=%v", session, err)
	}
	if session != nil {
		t.Fatal("expected nil session for unsupported capability")
	}
	text := err.Error()
	for _, forbidden := range []string{
		"PostgreSQL", "postgres", "17.4", "password", "dsn", "host=", "user=", "127.0.0.1", "pgx",
	} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Errorf("error text must not contain %q, got %q", forbidden, text)
		}
	}
}

// TestOnlineQueryAccessSession_NoTagPG17AnalysisFailsClosed proves a PG17
// capability target fails closed at the analysis entry before any proof work,
// with the capability sentinel beating later validation.
func TestOnlineQueryAccessSession_NoTagPG17AnalysisFailsClosed(t *testing.T) {
	db := openOnlineStubDB(t, onlineStubConfig{version: "8.4.10"})
	defer db.Close()

	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	// Package-internal construction: an observed PG17 session cannot be built
	// through the public constructor in this build, so pin the analysis entry
	// fail-closed path directly.
	session := &OnlineQueryAccessSession{conn: conn, target: online.TargetPG17}

	cases := []struct {
		name string
		req  QueryAccessRequest
	}{
		{"valid_request", QueryAccessRequest{SQL: "SELECT COUNT(1) FROM app.orders", DefaultSchema: "app"}},
		{"invalid_mode_beats", QueryAccessRequest{Mode: QueryAccessMode("bogus")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AnalyzeOnlineQueryAccessWithSession(t.Context(), session, tc.req)
			if !errors.Is(err, ErrOnlineQueryAccessCapabilityUnsupported) {
				t.Fatalf("want ErrOnlineQueryAccessCapabilityUnsupported, got %v", err)
			}
		})
	}
}

// TestOnlineQueryAccessSession_NoTagLegacyStubsUnchanged proves the existing
// PostgreSQL session stubs keep returning their historical error in the no-tag
// build; the unified capability sentinel does not replace it.
func TestOnlineQueryAccessSession_NoTagLegacyStubsUnchanged(t *testing.T) {
	if _, err := NewPostgreSQLQueryAccessSessionFromConn(t.Context(), nil); !errors.Is(err, ErrPostgreSQLSessionNotAvailable) {
		t.Fatalf("want ErrPostgreSQLSessionNotAvailable, got %v", err)
	}
	if _, err := AnalyzePostgreSQLQueryAccessWithSession(t.Context(), nil, QueryAccessRequest{}); !errors.Is(err, ErrPostgreSQLSessionNotAvailable) {
		t.Fatalf("want ErrPostgreSQLSessionNotAvailable, got %v", err)
	}
}

// TestOnlineQueryAccessSession_NoTagCompileContract pins the unified public
// signatures in the no-tag build so source shape cannot drift.
func TestOnlineQueryAccessSession_NoTagCompileContract(t *testing.T) {
	var _ func(context.Context, *sql.Conn) (*OnlineQueryAccessSession, error) = NewOnlineQueryAccessSessionFromConn
	var _ func(context.Context, *OnlineQueryAccessSession, QueryAccessRequest) (*QueryAccessResult, error) = AnalyzeOnlineQueryAccessWithSession
	var _ = ErrOnlineQueryAccessSessionUnavailable
	var _ = ErrOnlineQueryAccessDialectMismatch
	var _ = ErrOnlineQueryAccessProfileNotAllowed
	var _ = ErrOnlineQueryAccessSchemaResolverNotAllowed
	var _ = ErrOnlineQueryAccessCapabilityUnsupported
}

// Package deltascope verifies the unified online query access session contract.
// input: caller-owned *sql.Conn backed by configurable stub and recording drivers
// output: contract evidence for signatures, opacity, ownership, validation priority, generic sentinels, and MySQL/TiDB equivalence
// pos: public unified online session contract tests (default and PostgreSQL-tagged builds)
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	online "github.com/Fanduzi/DeltaScope/internal/application/online"
)

// ---------------------------------------------------------------------------
// Compile-time signature contract
// ---------------------------------------------------------------------------

// TestOnlineQueryAccessSession_Signatures pins the exact public signatures of
// the unified online session API. These assignments fail to compile if the
// constructor or analysis entry drifts from the reviewed contract.
func TestOnlineQueryAccessSession_Signatures(t *testing.T) {
	var _ func(context.Context, *sql.Conn) (*OnlineQueryAccessSession, error) = NewOnlineQueryAccessSessionFromConn
	var _ func(context.Context, *OnlineQueryAccessSession, QueryAccessRequest) (*QueryAccessResult, error) = AnalyzeOnlineQueryAccessWithSession
}

// TestOnlineQueryAccessSession_GenericSentinels pins the five generic online
// errors as distinct, errors.Is-comparable sentinels.
func TestOnlineQueryAccessSession_GenericSentinels(t *testing.T) {
	sentinels := []error{
		ErrOnlineQueryAccessSessionUnavailable,
		ErrOnlineQueryAccessDialectMismatch,
		ErrOnlineQueryAccessProfileNotAllowed,
		ErrOnlineQueryAccessSchemaResolverNotAllowed,
		ErrOnlineQueryAccessCapabilityUnsupported,
	}
	seen := map[error]bool{}
	for _, sentinel := range sentinels {
		if sentinel == nil {
			t.Fatal("sentinel must not be nil")
		}
		if seen[sentinel] {
			t.Fatalf("duplicate sentinel %v", sentinel)
		}
		seen[sentinel] = true
		if !errors.Is(sentinel, sentinel) {
			t.Fatalf("sentinel %v must satisfy errors.Is with itself", sentinel)
		}
	}
}

// ---------------------------------------------------------------------------
// Opacity: no exported field, no getter, no JSON-visible state
// ---------------------------------------------------------------------------

// TestOnlineQueryAccessSession_NoExportedState proves the session is opaque:
// no exported field, no JSON tag, no exported method (getter) surface.
func TestOnlineQueryAccessSession_NoExportedState(t *testing.T) {
	typ := reflect.TypeOf(OnlineQueryAccessSession{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.IsExported() {
			t.Errorf("OnlineQueryAccessSession must not have exported fields, found: %s", field.Name)
		}
		if tag := field.Tag.Get("json"); tag != "" && tag != "-" {
			t.Errorf("field %s must not have json tag (except '-'), found: %q", field.Name, tag)
		}
	}
	// The constructor returns a pointer, so both the value and the pointer
	// type method sets must stay empty (no getter surface on either).
	for _, sessionType := range []reflect.Type{
		typ,
		reflect.TypeOf((*OnlineQueryAccessSession)(nil)),
	} {
		for i := 0; i < sessionType.NumMethod(); i++ {
			t.Errorf("%v must not expose methods (getters), found: %s", sessionType, sessionType.Method(i).Name)
		}
	}
}

// TestOnlineQueryAccessSession_JSONOpaque proves the session marshals as an
// empty object and reveals no identity, capability, or connection state.
func TestOnlineQueryAccessSession_JSONOpaque(t *testing.T) {
	conn := openOnlineStubDB(t, onlineStubConfig{version: "8.4.10"})
	defer conn.Close()
	sessConn, err := conn.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer sessConn.Close()

	session, err := NewOnlineQueryAccessSessionFromConn(t.Context(), sessConn)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != "{}" {
		t.Fatalf("session must marshal as empty object, got %s", string(data))
	}

	forbidden := []string{
		"conn", "identity", "product", "profile", "capability", "target",
		"mysql", "tidb", "postgresql", "version", "dialect", "8.4", "sql",
	}
	for _, field := range forbidden {
		if strings.Contains(strings.ToLower(string(data)), strings.ToLower(field)) {
			t.Errorf("JSON output must not contain %q, got %s", field, string(data))
		}
	}
}

// ---------------------------------------------------------------------------
// Constructor contract
// ---------------------------------------------------------------------------

// TestOnlineQueryAccessSession_ConstructorUnavailableInputs proves nil context,
// nil connection, failed liveness, and failed identity map to the bounded
// unavailable sentinel.
func TestOnlineQueryAccessSession_ConstructorUnavailableInputs(t *testing.T) {
	cases := []struct {
		name  string
		ctxFn func() context.Context
		conn  func(t *testing.T) *sql.Conn
	}{
		{
			name: "nil_context",
			ctxFn: func() context.Context {
				return nil
			},
			conn: func(t *testing.T) *sql.Conn {
				db := openOnlineStubDB(t, onlineStubConfig{version: "8.4.10"})
				t.Cleanup(func() { db.Close() })
				c, err := db.Conn(context.Background())
				if err != nil {
					t.Fatalf("conn: %v", err)
				}
				return c
			},
		},
		{
			name: "nil_connection",
			ctxFn: func() context.Context {
				return context.Background()
			},
			conn: func(t *testing.T) *sql.Conn {
				return nil
			},
		},
		{
			name: "ping_failure",
			ctxFn: func() context.Context {
				return context.Background()
			},
			conn: func(t *testing.T) *sql.Conn {
				db := openOnlineStubDB(t, onlineStubConfig{version: "8.4.10", pingErr: errors.New("backend down")})
				t.Cleanup(func() { db.Close() })
				c, err := db.Conn(context.Background())
				if err != nil {
					t.Fatalf("conn: %v", err)
				}
				return c
			},
		},
		{
			name: "identity_failure",
			ctxFn: func() context.Context {
				return context.Background()
			},
			conn: func(t *testing.T) *sql.Conn {
				db := openOnlineStubDB(t, onlineStubConfig{version: "mariadb 10.11.2"})
				t.Cleanup(func() { db.Close() })
				c, err := db.Conn(context.Background())
				if err != nil {
					t.Fatalf("conn: %v", err)
				}
				return c
			},
		},
		{
			name: "version_query_failure",
			ctxFn: func() context.Context {
				return context.Background()
			},
			conn: func(t *testing.T) *sql.Conn {
				db := openOnlineStubDB(t, onlineStubConfig{version: "8.4.10", versionErr: errors.New("dial tcp 127.0.0.1:3306: connection reset")})
				t.Cleanup(func() { db.Close() })
				c, err := db.Conn(context.Background())
				if err != nil {
					t.Fatalf("conn: %v", err)
				}
				return c
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := tc.conn(t)
			if conn != nil {
				defer conn.Close()
			}
			session, err := NewOnlineQueryAccessSessionFromConn(tc.ctxFn(), conn)
			if session != nil {
				t.Fatalf("expected nil session, got %+v", session)
			}
			if !errors.Is(err, ErrOnlineQueryAccessSessionUnavailable) {
				t.Fatalf("want ErrOnlineQueryAccessSessionUnavailable, got %v", err)
			}
		})
	}
}

// TestOnlineQueryAccessSession_ConstructorDoesNotLeak proves constructor
// failures never expose identity, version, endpoint, credential, or driver text
// and that each failure class maps to its bounded sentinel.
func TestOnlineQueryAccessSession_ConstructorDoesNotLeak(t *testing.T) {
	// PG17 constructor behavior follows the linked capability seam: it fails
	// with the capability sentinel in the no-tag build and routes (constructs
	// a usable session) in the postgresql-tagged build.
	pg17WantErr := ErrOnlineQueryAccessCapabilityUnsupported
	if queryAccessOnlineCapabilityLinked(online.TargetPG17) {
		pg17WantErr = nil
	}
	cases := []struct {
		name    string
		version string
		pingErr error
		wantErr error
	}{
		{"pg17", "PostgreSQL 17.4", nil, pg17WantErr},
		{"garbage_identity", "some random junk version", nil, ErrOnlineQueryAccessSessionUnavailable},
		{"ping_failure", "8.4.10", errors.New("dial tcp 127.0.0.1:3306: connect: connection refused"), ErrOnlineQueryAccessSessionUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openOnlineStubDB(t, onlineStubConfig{version: tc.version, pingErr: tc.pingErr})
			defer db.Close()
			conn, err := db.Conn(t.Context())
			if err != nil {
				t.Fatalf("conn: %v", err)
			}
			defer conn.Close()

			_, err = NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("want success, got %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
			text := err.Error()
			for _, forbidden := range []string{
				"127.0.0.1", "3306", "password", "dsn", "host=", "user=",
				"PostgreSQL", "postgres", "17.4", "8.4.10", "mariadb",
				"dial tcp", "connection refused", "pgx", "mysql",
			} {
				if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
					t.Errorf("error text must not contain %q, got %q", forbidden, text)
				}
			}
		})
	}
}

// TestOnlineQueryAccessSession_PostgreSQLTargetCapabilityBoundary proves the
// unified constructor follows the linked capability seam for an observed
// PostgreSQL 17 target: capability sentinel in the no-tag build, usable
// session in the postgresql-tagged build.
func TestOnlineQueryAccessSession_PostgreSQLTargetCapabilityBoundary(t *testing.T) {
	db := openOnlineStubDB(t, onlineStubConfig{version: "PostgreSQL 17.4"})
	defer db.Close()
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
	if queryAccessOnlineCapabilityLinked(online.TargetPG17) {
		if err != nil {
			t.Fatalf("tagged build must route PG17, got %v", err)
		}
		if session == nil {
			t.Fatal("expected non-nil session in tagged build")
		}
		return
	}
	if !errors.Is(err, ErrOnlineQueryAccessCapabilityUnsupported) {
		t.Fatalf("want ErrOnlineQueryAccessCapabilityUnsupported, got session=%v err=%v", session, err)
	}
	if session != nil {
		t.Fatal("expected nil session for unsupported capability")
	}
}

// TestOnlineQueryAccessSession_CallerOwnsConnection proves construction and
// analysis never close the caller's connection and the caller keeps control.
func TestOnlineQueryAccessSession_CallerOwnsConnection(t *testing.T) {
	db := openOnlineStubDB(t, onlineStubConfig{version: "8.4.10"})
	defer db.Close()

	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}

	session, err := NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	result, err := AnalyzeOnlineQueryAccessWithSession(t.Context(), session, QueryAccessRequest{
		SQL:           "SELECT id FROM app.users",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
		t.Fatalf("classification=%q admission=%q, want read_only/admissible", result.ReadClassification, result.Admission)
	}
	// The session must never close the caller-owned connection.
	if err := conn.PingContext(t.Context()); err != nil {
		t.Fatalf("caller connection after analysis: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("caller close: %v", err)
	}
}

// TestOnlineQueryAccessSession_RecognizedButUnsupportedVersion proves a
// recognized product with an unsupported version series (MySQL 8.1, TiDB 7.5,
// PostgreSQL 16) maps to the capability sentinel, never leaking the raw
// version, while untrustworthy identities stay unavailable.
func TestOnlineQueryAccessSession_RecognizedButUnsupportedVersion(t *testing.T) {
	cases := []struct {
		name    string
		version string
	}{
		{"mysql81", "8.1.0"},
		{"tidb75", "8.0.11-TiDB-v7.5.4"},
		{"pg16", "PostgreSQL 16.3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openOnlineStubDB(t, onlineStubConfig{version: tc.version})
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
				t.Fatal("expected nil session for unsupported version")
			}
			text := err.Error()
			for _, forbidden := range []string{
				"8.1", "7.5", "16.3", "TiDB", "PostgreSQL", "mysql",
				"password", "dsn", "host=", "user=", "127.0.0.1",
			} {
				if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
					t.Errorf("error text must not contain %q, got %q", forbidden, text)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Fixed validation priority (combined-invalid-input)
// ---------------------------------------------------------------------------

type onlineStubResolver struct{}

func (onlineStubResolver) ResolveRelation(context.Context, string, string, string) (QueryAccessRelationSchema, error) {
	return QueryAccessRelationSchema{}, nil
}

// TestOnlineQueryAccessSession_ValidationPriority pins the fixed order:
// session/context; dialect mismatch; profile; resolver; linked capability;
// existing request validation. Every row combines all later invalids so a
// refactor that reorders validation fails loudly.
func TestOnlineQueryAccessSession_ValidationPriority(t *testing.T) {
	db := openOnlineStubDB(t, onlineStubConfig{version: "8.4.10"})
	defer db.Close()
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	// A never-linked capability target pins the capability-beats-mode row in
	// both builds. PostgreSQL 17 is linked in the postgresql-tagged build since
	// issue #6, so it can no longer serve as the unsupported example here; the
	// package-internal test builds the session directly.
	unsupportedSession := &OnlineQueryAccessSession{conn: conn, target: online.CapabilityTarget("future-unsupported")}

	invalidMode := QueryAccessMode("bogus_mode")
	profile := QueryAccessAnalysisProfileMySQL57
	resolver := onlineStubResolver{}

	cases := []struct {
		name    string
		session *OnlineQueryAccessSession
		req     QueryAccessRequest
		wantErr error
	}{
		{
			name:    "session_unavailable_beats_all",
			session: nil,
			req: QueryAccessRequest{
				Dialect:         DialectTiDB,
				AnalysisProfile: profile,
				SchemaResolver:  resolver,
				Mode:            invalidMode,
			},
			wantErr: ErrOnlineQueryAccessSessionUnavailable,
		},
		{
			name:    "dialect_mismatch_beats_profile_resolver_capability_mode",
			session: session,
			req: QueryAccessRequest{
				Dialect:         DialectTiDB,
				AnalysisProfile: profile,
				SchemaResolver:  resolver,
				Mode:            invalidMode,
			},
			wantErr: ErrOnlineQueryAccessDialectMismatch,
		},
		{
			name:    "profile_beats_resolver_capability_mode",
			session: session,
			req: QueryAccessRequest{
				Dialect:         DialectMySQL,
				AnalysisProfile: profile,
				SchemaResolver:  resolver,
				Mode:            invalidMode,
			},
			wantErr: ErrOnlineQueryAccessProfileNotAllowed,
		},
		{
			name:    "resolver_beats_capability_mode",
			session: session,
			req: QueryAccessRequest{
				Dialect:        DialectMySQL,
				SchemaResolver: resolver,
				Mode:           invalidMode,
			},
			wantErr: ErrOnlineQueryAccessSchemaResolverNotAllowed,
		},
		{
			name:    "capability_beats_mode",
			session: unsupportedSession,
			req: QueryAccessRequest{
				Mode: invalidMode,
			},
			wantErr: ErrOnlineQueryAccessCapabilityUnsupported,
		},
		{
			name:    "existing_mode_validation_last",
			session: session,
			req: QueryAccessRequest{
				Mode: invalidMode,
			},
			wantErr: ErrInvalidQueryAccessMode,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AnalyzeOnlineQueryAccessWithSession(t.Context(), tc.session, tc.req)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MySQL/TiDB equivalence with the existing dialect-specific API
// ---------------------------------------------------------------------------

type onlineEquivCase struct {
	name    string
	version string
	dialect Dialect // dialect-specific API dialect; unified API leaves empty
	sql     string
	wantErr bool // when true both APIs must return an error
}

func onlineEquivalenceCases() []onlineEquivCase {
	return []onlineEquivCase{
		// Admitted: identity-derived proof paths.
		{name: "mysql57_count_star", version: "5.7.44", dialect: DialectMySQL, sql: "SELECT COUNT(*) FROM app.builtin_semantic_facts"},
		{name: "mysql80_count_star", version: "8.0.46", dialect: DialectMySQL, sql: "SELECT COUNT(*) FROM app.builtin_semantic_facts"},
		{name: "mysql84_count_star", version: "8.4.10", dialect: DialectMySQL, sql: "SELECT COUNT(*) FROM app.builtin_semantic_facts"},
		{name: "tidb85_count_star", version: "8.0.11-TiDB-v8.5.7", dialect: DialectTiDB, sql: "SELECT COUNT(*) FROM app.builtin_semantic_facts"},
		{name: "mysql84_sum", version: "8.4.10", dialect: DialectMySQL, sql: "SELECT SUM(amount) FROM app.builtin_semantic_facts"},
		{name: "mysql84_literal", version: "8.4.10", dialect: DialectMySQL, sql: "SELECT LOWER('x') FROM app.builtin_semantic_facts"},
		{name: "mysql84_relationless", version: "8.4.10", dialect: DialectMySQL, sql: "SELECT LOWER('SECRET_LITERAL')"},
		// Indeterminate: unknown function stays fail-closed.
		{name: "mysql84_unknown_function", version: "8.4.10", dialect: DialectMySQL, sql: "SELECT app_specific_rollup(id) FROM app.users"},
		// Rejected: write statement.
		{name: "mysql84_insert", version: "8.4.10", dialect: DialectMySQL, sql: "INSERT INTO app.users (id) VALUES (1)"},
	}
}

// TestOnlineQueryAccessSession_MySQLTiDBSemanticMatrix makes the unified
// entry the direct owner of the MySQL/TiDB semantic rows formerly exercised
// only through the deprecated dialect-specific session.
func TestOnlineQueryAccessSession_MySQLTiDBSemanticMatrix(t *testing.T) {
	type testCase struct {
		name      string
		version   string
		sql       string
		admission QueryAccessAdmission
		wantReqs  []QueryAccessRequirement
	}
	baseTable := []QueryAccessRequirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}}
	cases := []testCase{
		{name: "mysql57_count_star", version: "5.7.44", sql: "SELECT COUNT(*) FROM app.builtin_semantic_facts", admission: QueryAccessAdmissible, wantReqs: baseTable},
		{name: "mysql80_count_star", version: "8.0.46", sql: "SELECT COUNT(*) FROM app.builtin_semantic_facts", admission: QueryAccessAdmissible, wantReqs: baseTable},
		{name: "mysql84_count_star", version: "8.4.10", sql: "SELECT COUNT(*) FROM app.builtin_semantic_facts", admission: QueryAccessAdmissible, wantReqs: baseTable},
		{name: "tidb85_count_star", version: "8.0.11-TiDB-v8.5.7", sql: "SELECT COUNT(*) FROM app.builtin_semantic_facts", admission: QueryAccessAdmissible, wantReqs: baseTable},
		{name: "literal_only_lower", version: "8.4.10", sql: "SELECT LOWER('x') FROM app.builtin_semantic_facts", admission: QueryAccessAdmissible, wantReqs: baseTable},
		{name: "count_literal", version: "8.4.10", sql: "SELECT COUNT(1) FROM app.builtin_semantic_facts", admission: QueryAccessAdmissible, wantReqs: baseTable},
		{name: "reversed_coalesce", version: "8.4.10", sql: "SELECT COALESCE('x', name) FROM app.builtin_semantic_facts", admission: QueryAccessAdmissible, wantReqs: append(append([]QueryAccessRequirement(nil), baseTable...), QueryAccessRequirement{Object: "app.builtin_semantic_facts.name", Privilege: "read_column"})},
		{name: "all_constant_coalesce", version: "8.4.10", sql: "SELECT COALESCE('x', 'y') FROM app.builtin_semantic_facts", admission: QueryAccessAdmissible, wantReqs: baseTable},
		{name: "unknown_function", version: "8.4.10", sql: "SELECT app_specific_rollup(id) FROM app.users", admission: QueryAccessIndeterminateAdmission, wantReqs: []QueryAccessRequirement{{Object: "app.users", Privilege: "read_table"}, {Object: "app.users.id", Privilege: "read_column"}}},
	}
	for _, shape := range onlineRelationlessLiteralShapes() {
		cases = append(cases, testCase{name: "relationless_" + shape.name, version: "8.4.10", sql: shape.sql, admission: QueryAccessAdmissible})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openOnlineStubDB(t, onlineStubConfig{version: tc.version})
			defer db.Close()
			conn, err := db.Conn(t.Context())
			if err != nil {
				t.Fatalf("conn: %v", err)
			}
			defer conn.Close()
			session, err := NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
			if err != nil {
				t.Fatalf("new session: %v", err)
			}
			result, err := AnalyzeOnlineQueryAccessWithSession(t.Context(), session, QueryAccessRequest{SQL: tc.sql, DefaultSchema: "app"})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			if result.Admission != tc.admission {
				t.Fatalf("admission=%q, want %q; result=%+v", result.Admission, tc.admission, result)
			}
			if tc.admission == QueryAccessAdmissible && result.ReadClassification != QueryAccessReadOnly {
				t.Fatalf("classification=%q, want read_only", result.ReadClassification)
			}
			if !reflect.DeepEqual(result.Requirements, tc.wantReqs) {
				t.Fatalf("requirements=%+v, want %+v", result.Requirements, tc.wantReqs)
			}
			if strings.Contains(fmt.Sprintf("%+v", result), "SECRET_LITERAL") {
				t.Fatalf("result leaked literal: %+v", result)
			}
			data, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("marshal result: %v", err)
			}
			if strings.Contains(string(data), "SECRET_LITERAL") {
				t.Fatalf("result JSON leaked literal: %s", data)
			}
		})
	}
}

func onlineRelationlessLiteralShapes() []struct{ name, sql string } {
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

// TestOnlineQueryAccessSession_MatchesDialectSpecificMySQLTiDB runs the same
// request through the unified entry (empty dialect = observed identity) and the
// existing MySQL/TiDB session API on the same caller-owned connection, and
// requires identical results.
func TestOnlineQueryAccessSession_MatchesDialectSpecificMySQLTiDB(t *testing.T) {
	for _, tc := range onlineEquivalenceCases() {
		t.Run(tc.name, func(t *testing.T) {
			db := openOnlineStubDB(t, onlineStubConfig{version: tc.version})
			defer db.Close()

			conn, err := db.Conn(t.Context())
			if err != nil {
				t.Fatalf("conn: %v", err)
			}
			defer conn.Close()

			unifiedSession, err := NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
			if err != nil {
				t.Fatalf("unified new session: %v", err)
			}
			legacySession, err := NewMySQLTiDBQueryAccessSessionFromConn(t.Context(), conn)
			if err != nil {
				t.Fatalf("legacy new session: %v", err)
			}

			unifiedResult, unifiedErr := AnalyzeOnlineQueryAccessWithSession(t.Context(), unifiedSession, QueryAccessRequest{
				SQL:           tc.sql,
				DefaultSchema: "app",
			})
			legacyResult, legacyErr := AnalyzeMySQLTiDBQueryAccessWithSession(t.Context(), legacySession, QueryAccessRequest{
				SQL:           tc.sql,
				Dialect:       tc.dialect,
				DefaultSchema: "app",
			})

			if tc.wantErr {
				if unifiedErr == nil || legacyErr == nil {
					t.Fatalf("both APIs must error: unified=%v legacy=%v", unifiedErr, legacyErr)
				}
				return
			}
			if unifiedErr != nil {
				t.Fatalf("unified analyze: %v", unifiedErr)
			}
			if legacyErr != nil {
				t.Fatalf("legacy analyze: %v", legacyErr)
			}
			if !reflect.DeepEqual(unifiedResult, legacyResult) {
				t.Fatalf("results differ:\nunified=%+v\nlegacy =%+v", unifiedResult, legacyResult)
			}
		})
	}
}

// TestOnlineQueryAccessSession_MatchesDialectSpecificOnFailure proves
// cancellation, closed-connection, and catalog-failure cases stay equivalent
// and bounded across both APIs.
func TestOnlineQueryAccessSession_MatchesDialectSpecificOnFailure(t *testing.T) {
	t.Run("cancellation", func(t *testing.T) {
		db := openOnlineStubDB(t, onlineStubConfig{version: "8.4.10"})
		defer db.Close()
		conn, err := db.Conn(t.Context())
		if err != nil {
			t.Fatalf("conn: %v", err)
		}
		defer conn.Close()

		unifiedSession, err := NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
		if err != nil {
			t.Fatalf("unified new session: %v", err)
		}
		legacySession, err := NewMySQLTiDBQueryAccessSessionFromConn(t.Context(), conn)
		if err != nil {
			t.Fatalf("legacy new session: %v", err)
		}

		cancelled, cancel := context.WithCancel(context.Background())
		cancel()

		_, unifiedErr := AnalyzeOnlineQueryAccessWithSession(cancelled, unifiedSession, QueryAccessRequest{
			SQL:           "SELECT id FROM app.users",
			DefaultSchema: "app",
		})
		_, legacyErr := AnalyzeMySQLTiDBQueryAccessWithSession(cancelled, legacySession, QueryAccessRequest{
			SQL:           "SELECT id FROM app.users",
			Dialect:       DialectMySQL,
			DefaultSchema: "app",
		})
		if unifiedErr == nil || legacyErr == nil {
			t.Fatalf("both APIs must honor cancellation: unified=%v legacy=%v", unifiedErr, legacyErr)
		}
		if unifiedErr.Error() != legacyErr.Error() {
			t.Fatalf("cancellation errors differ: unified=%q legacy=%q", unifiedErr.Error(), legacyErr.Error())
		}
		for _, err := range []error{unifiedErr, legacyErr} {
			text := err.Error()
			for _, forbidden := range []string{"8.4", "password", "dsn", "host=", "user=", "127.0.0.1"} {
				if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
					t.Errorf("error text must not contain %q, got %q", forbidden, text)
				}
			}
		}
	})

	t.Run("closed_connection", func(t *testing.T) {
		db := openOnlineStubDB(t, onlineStubConfig{version: "8.4.10"})
		defer db.Close()
		conn, err := db.Conn(t.Context())
		if err != nil {
			t.Fatalf("conn: %v", err)
		}

		unifiedSession, err := NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
		if err != nil {
			t.Fatalf("unified new session: %v", err)
		}
		legacySession, err := NewMySQLTiDBQueryAccessSessionFromConn(t.Context(), conn)
		if err != nil {
			t.Fatalf("legacy new session: %v", err)
		}

		// The caller closes the connection; the sessions must not own it.
		if err := conn.Close(); err != nil {
			t.Fatalf("caller close: %v", err)
		}

		unifiedResult, unifiedErr := AnalyzeOnlineQueryAccessWithSession(t.Context(), unifiedSession, QueryAccessRequest{
			SQL:           "SELECT id FROM app.users",
			DefaultSchema: "app",
		})
		legacyResult, legacyErr := AnalyzeMySQLTiDBQueryAccessWithSession(t.Context(), legacySession, QueryAccessRequest{
			SQL:           "SELECT id FROM app.users",
			Dialect:       DialectMySQL,
			DefaultSchema: "app",
		})
		if unifiedErr != nil || legacyErr != nil {
			t.Fatalf("closed-connection analysis must complete bounded: unified=%v legacy=%v", unifiedErr, legacyErr)
		}
		if !reflect.DeepEqual(unifiedResult, legacyResult) {
			t.Fatalf("closed-connection results differ:\nunified=%+v\nlegacy =%+v", unifiedResult, legacyResult)
		}
		if unifiedResult.Admission != QueryAccessIndeterminateAdmission {
			t.Fatalf("closed-connection must stay indeterminate, got %q", unifiedResult.Admission)
		}
	})

	t.Run("catalog_failure", func(t *testing.T) {
		db := openOnlineStubDB(t, onlineStubConfig{version: "8.4.10", catalogErr: errors.New("catalog unavailable")})
		defer db.Close()
		conn, err := db.Conn(t.Context())
		if err != nil {
			t.Fatalf("conn: %v", err)
		}
		defer conn.Close()

		unifiedSession, err := NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
		if err != nil {
			t.Fatalf("unified new session: %v", err)
		}
		legacySession, err := NewMySQLTiDBQueryAccessSessionFromConn(t.Context(), conn)
		if err != nil {
			t.Fatalf("legacy new session: %v", err)
		}

		unifiedResult, unifiedErr := AnalyzeOnlineQueryAccessWithSession(t.Context(), unifiedSession, QueryAccessRequest{
			SQL:           "SELECT id FROM app.users",
			DefaultSchema: "app",
		})
		legacyResult, legacyErr := AnalyzeMySQLTiDBQueryAccessWithSession(t.Context(), legacySession, QueryAccessRequest{
			SQL:           "SELECT id FROM app.users",
			Dialect:       DialectMySQL,
			DefaultSchema: "app",
		})
		if unifiedErr != nil || legacyErr != nil {
			t.Fatalf("catalog-failure analysis must complete bounded: unified=%v legacy=%v", unifiedErr, legacyErr)
		}
		if !reflect.DeepEqual(unifiedResult, legacyResult) {
			t.Fatalf("catalog-failure results differ:\nunified=%+v\nlegacy =%+v", unifiedResult, legacyResult)
		}
		if len(unifiedResult.Unresolved) == 0 {
			t.Fatalf("catalog failure must produce unresolved metadata, got %+v", unifiedResult)
		}
	})
}

// ---------------------------------------------------------------------------
// Recording-driver: user SQL never executed; no marker or fact leaks
// ---------------------------------------------------------------------------

// TestOnlineQueryAccessSession_NoExecutionNoLeakMatchesDialectSpecificMySQLTiDB
// compares the unified and dialect-specific APIs directly across every
// supported MySQL/TiDB profile, including their exact recording-driver query
// sequence and bounded public result.
func TestOnlineQueryAccessSession_NoExecutionNoLeakMatchesDialectSpecificMySQLTiDB(t *testing.T) {
	cases := []struct {
		name    string
		version string
		dialect Dialect
	}{
		{name: "mysql57", version: "5.7.44", dialect: DialectMySQL},
		{name: "mysql80", version: "8.0.46", dialect: DialectMySQL},
		{name: "mysql84", version: "8.4.10", dialect: DialectMySQL},
		{name: "tidb85", version: "8.0.11-TiDB-v8.5.7", dialect: DialectTiDB},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			marker := "NO_EXEC_NO_LEAK_" + strings.ToUpper(tc.name)
			sqlText := "SELECT COUNT(*) /* " + marker + " */ FROM app.users"
			run := func(unified bool) (*QueryAccessResult, []string) {
				t.Helper()
				recorder := &onlineRecordingDriver{version: tc.version}
				db := openOnlineRecordingDB(t, recorder)
				defer db.Close()
				conn, err := db.Conn(t.Context())
				if err != nil {
					t.Fatalf("conn: %v", err)
				}
				defer conn.Close()

				var result *QueryAccessResult
				if unified {
					session, err := NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
					if err != nil {
						t.Fatalf("unified new session: %v", err)
					}
					result, err = AnalyzeOnlineQueryAccessWithSession(t.Context(), session, QueryAccessRequest{
						SQL: sqlText, DefaultSchema: "app",
					})
					if err != nil {
						t.Fatalf("unified analyze: %v", err)
					}
				} else {
					session, err := NewMySQLTiDBQueryAccessSessionFromConn(t.Context(), conn)
					if err != nil {
						t.Fatalf("legacy new session: %v", err)
					}
					result, err = AnalyzeMySQLTiDBQueryAccessWithSession(t.Context(), session, QueryAccessRequest{
						SQL: sqlText, Dialect: tc.dialect, DefaultSchema: "app",
					})
					if err != nil {
						t.Fatalf("legacy analyze: %v", err)
					}
				}
				return result, recorder.recorded()
			}

			unifiedResult, unifiedQueries := run(true)
			legacyResult, legacyQueries := run(false)
			if !reflect.DeepEqual(unifiedResult, legacyResult) {
				t.Fatalf("results differ:\nunified=%+v\nlegacy =%+v", unifiedResult, legacyResult)
			}
			if !reflect.DeepEqual(unifiedQueries, legacyQueries) {
				t.Fatalf("recording queries differ:\nunified=%v\nlegacy =%v", unifiedQueries, legacyQueries)
			}
			for _, query := range append(unifiedQueries, legacyQueries...) {
				if strings.Contains(query, marker) {
					t.Fatalf("submitted SQL reached driver: %q", query)
				}
			}
			for _, result := range []*QueryAccessResult{unifiedResult, legacyResult} {
				data, err := json.Marshal(result)
				if err != nil {
					t.Fatalf("marshal result: %v", err)
				}
				for _, forbidden := range []string{marker, tc.version, "information_schema", "password", "dsn", "host=", "user="} {
					if strings.Contains(strings.ToLower(string(data)), strings.ToLower(forbidden)) {
						t.Errorf("result JSON leaked %q: %s", forbidden, data)
					}
				}
			}
		})
	}
}

// TestOnlineQueryAccessSession_DoesNotExecuteUserSQL proves the unified entry
// never sends caller-submitted analysis SQL to the database and only runs the
// bounded identity/catalog probes.
func TestOnlineQueryAccessSession_DoesNotExecuteUserSQL(t *testing.T) {
	const marker = "SQLNOTEXEC_MARKER_7f3a"
	rec := &onlineRecordingDriver{}
	db := openOnlineRecordingDB(t, rec)
	defer db.Close()

	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	result, err := AnalyzeOnlineQueryAccessWithSession(t.Context(), session, QueryAccessRequest{
		SQL:           "SELECT LOWER('" + marker + "') FROM app.users",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
		t.Fatalf("classification=%q admission=%q", result.ReadClassification, result.Admission)
	}

	queries := rec.recorded()
	if len(queries) == 0 {
		t.Fatal("expected bounded identity/catalog probes")
	}
	for _, q := range queries {
		if strings.Contains(q, marker) {
			t.Fatalf("executed user SQL against the database: %q", q)
		}
	}
}

// TestOnlineQueryAccessSession_NoLeak proves results and errors from the
// unified entry never expose credentials, endpoints, raw versions, catalog
// details, driver text, or marker literals.
func TestOnlineQueryAccessSession_NoLeak(t *testing.T) {
	const marker = "SECRET_LITERAL_9f2b"
	rec := &onlineRecordingDriver{}
	db := openOnlineRecordingDB(t, rec)
	defer db.Close()

	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()

	session, err := NewOnlineQueryAccessSessionFromConn(t.Context(), conn)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	result, err := AnalyzeOnlineQueryAccessWithSession(t.Context(), session, QueryAccessRequest{
		SQL:           "SELECT LOWER('" + marker + "')",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{
		marker, "information_schema", "VERSION", "8.4", "password", "dsn",
		"host=", "user=", "127.0.0.1", "go-sql-driver", "driver",
	} {
		if strings.Contains(strings.ToLower(string(data)), strings.ToLower(forbidden)) {
			t.Errorf("result JSON must not contain %q: %s", forbidden, string(data))
		}
	}
}

// ---------------------------------------------------------------------------
// Stub drivers
// ---------------------------------------------------------------------------

type onlineStubConfig struct {
	version    string
	versionErr error
	pingErr    error
	catalogErr error
}

var onlineStubSeq int
var onlineStubSeqMu sync.Mutex

// openOnlineStubDB opens a *sql.DB backed by a configurable stub driver.
func openOnlineStubDB(t *testing.T, cfg onlineStubConfig) *sql.DB {
	t.Helper()
	onlineStubSeqMu.Lock()
	onlineStubSeq++
	name := fmt.Sprintf("deltascope-online-stub-%d", onlineStubSeq)
	onlineStubSeqMu.Unlock()
	sql.Register(name, onlineStubDriver{cfg: cfg})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("open stub db: %v", err)
	}
	return db
}

type onlineStubDriver struct {
	cfg onlineStubConfig
}

func (d onlineStubDriver) Open(string) (driver.Conn, error) {
	return onlineStubConn{cfg: d.cfg}, nil
}

type onlineStubConn struct {
	cfg onlineStubConfig
}

func (onlineStubConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (onlineStubConn) Close() error                        { return nil }
func (onlineStubConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c onlineStubConn) Ping(context.Context) error {
	return c.cfg.pingErr
}

func (c onlineStubConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "information_schema.tables"):
		if c.cfg.catalogErr != nil {
			return nil, c.cfg.catalogErr
		}
		return newOnlineStubRows([]string{"table_type"}, [][]driver.Value{{"BASE TABLE"}}), nil
	case strings.Contains(query, "information_schema.columns"):
		if c.cfg.catalogErr != nil {
			return nil, c.cfg.catalogErr
		}
		return newOnlineStubRows([]string{"column_name", "ordinal_position"}, [][]driver.Value{
			{"id", int64(1)},
			{"dept", int64(2)},
			{"amount", int64(3)},
			{"name", int64(4)},
		}), nil
	case strings.Contains(query, "SELECT VERSION()"):
		if c.cfg.versionErr != nil {
			return nil, c.cfg.versionErr
		}
		return newOnlineStubRows([]string{"version"}, [][]driver.Value{{c.cfg.version}}), nil
	case strings.Contains(query, "SELECT 1"):
		return newOnlineStubRows([]string{"value"}, [][]driver.Value{{int64(1)}}), nil
	default:
		return nil, driver.ErrSkip
	}
}

type onlineStubRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func newOnlineStubRows(columns []string, rows [][]driver.Value) *onlineStubRows {
	return &onlineStubRows{columns: columns, rows: rows}
}

func (r *onlineStubRows) Columns() []string { return r.columns }
func (r *onlineStubRows) Close() error      { return nil }

func (r *onlineStubRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

// onlineRecordingDriver records every query text routed through the
// caller-owned connection so tests can prove the unified entry only runs
// bounded identity/catalog probes.
type onlineRecordingDriver struct {
	mu      sync.Mutex
	queries []string
	version string
}

func (d *onlineRecordingDriver) record(query string) {
	d.mu.Lock()
	d.queries = append(d.queries, query)
	d.mu.Unlock()
}

func (d *onlineRecordingDriver) recorded() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.queries...)
}

func (d *onlineRecordingDriver) Open(string) (driver.Conn, error) {
	return onlineRecordingConn{rec: d}, nil
}

var onlineRecordingSeq int
var onlineRecordingSeqMu sync.Mutex

func openOnlineRecordingDB(t *testing.T, rec *onlineRecordingDriver) *sql.DB {
	t.Helper()
	onlineRecordingSeqMu.Lock()
	onlineRecordingSeq++
	name := fmt.Sprintf("deltascope-online-recording-%d", onlineRecordingSeq)
	onlineRecordingSeqMu.Unlock()
	sql.Register(name, rec)
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("open recording db: %v", err)
	}
	return db
}

type onlineRecordingConn struct {
	rec *onlineRecordingDriver
}

func (onlineRecordingConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (onlineRecordingConn) Close() error                        { return nil }
func (onlineRecordingConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (onlineRecordingConn) Ping(context.Context) error          { return nil }

func (c onlineRecordingConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.rec.record(query)
	switch {
	case strings.Contains(query, "information_schema.tables"):
		return newOnlineStubRows([]string{"table_type"}, [][]driver.Value{{"BASE TABLE"}}), nil
	case strings.Contains(query, "information_schema.columns"):
		return newOnlineStubRows([]string{"column_name", "ordinal_position"}, [][]driver.Value{
			{"id", int64(1)},
			{"dept", int64(2)},
			{"amount", int64(3)},
			{"name", int64(4)},
		}), nil
	case strings.Contains(query, "SELECT VERSION()"):
		version := c.rec.version
		if version == "" {
			version = "8.4.10"
		}
		return newOnlineStubRows([]string{"version"}, [][]driver.Value{{version}}), nil
	case strings.Contains(query, "SELECT 1"):
		return newOnlineStubRows([]string{"value"}, [][]driver.Value{{int64(1)}}), nil
	default:
		return nil, driver.ErrSkip
	}
}

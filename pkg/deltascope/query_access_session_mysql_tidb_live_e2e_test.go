//go:build integration

// Package deltascope verifies the live MySQL/TiDB SDK profile E2E boundary.
// input: caller-owned *sql.Conn against running MySQL 5.7/8.0/8.4 and TiDB 8.5
// output: read_only + admissible for proven entries; indeterminate for every excluded shape
// pos: live SDK session E2E against the four production builtin semantic profiles
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

	_ "github.com/go-sql-driver/mysql"
)

type liveProfileCase struct {
	name          string
	dialect       Dialect
	profile       QueryAccessAnalysisProfile
	dsnEnvHost    string
	dsnEnvPort    string
	dsnEnvUser    string
	dsnEnvPass    string
	defaultPort   int
	exactVersion  string
	versionMatch  func(string, string) bool
	windowSupport bool
}

func liveProfileCases() []liveProfileCase {
	return []liveProfileCase{
		{
			name: "mysql57", dialect: DialectMySQL, profile: QueryAccessAnalysisProfileMySQL57,
			dsnEnvHost: "DELTASCOPE_MYSQL57_HOST", dsnEnvPort: "DELTASCOPE_MYSQL57_PORT",
			dsnEnvUser: "DELTASCOPE_MYSQL57_USER", dsnEnvPass: "DELTASCOPE_MYSQL57_PASSWORD",
			defaultPort: 3507, exactVersion: "5.7.44",
			versionMatch:  func(observed, want string) bool { return observed == want },
			windowSupport: false,
		},
		{
			name: "mysql80", dialect: DialectMySQL, profile: QueryAccessAnalysisProfileMySQL80,
			dsnEnvHost: "DELTASCOPE_MYSQL80_HOST", dsnEnvPort: "DELTASCOPE_MYSQL80_PORT",
			dsnEnvUser: "DELTASCOPE_MYSQL80_USER", dsnEnvPass: "DELTASCOPE_MYSQL80_PASSWORD",
			defaultPort: 3800, exactVersion: "8.0.46",
			versionMatch:  func(observed, want string) bool { return observed == want },
			windowSupport: true,
		},
		{
			name: "mysql84", dialect: DialectMySQL, profile: QueryAccessAnalysisProfileMySQL84,
			dsnEnvHost: "DELTASCOPE_MYSQL84_HOST", dsnEnvPort: "DELTASCOPE_MYSQL84_PORT",
			dsnEnvUser: "DELTASCOPE_MYSQL84_USER", dsnEnvPass: "DELTASCOPE_MYSQL84_PASSWORD",
			defaultPort: 3840, exactVersion: "8.4.10",
			versionMatch:  func(observed, want string) bool { return observed == want },
			windowSupport: true,
		},
		{
			name: "tidb85", dialect: DialectTiDB, profile: QueryAccessAnalysisProfileTiDB85,
			dsnEnvHost: "DELTASCOPE_TIDB85_HOST", dsnEnvPort: "DELTASCOPE_TIDB85_PORT",
			dsnEnvUser: "DELTASCOPE_TIDB85_USER", dsnEnvPass: "DELTASCOPE_TIDB85_PASSWORD",
			defaultPort: 4850, exactVersion: "8.0.11-TiDB-v8.5.7",
			versionMatch:  func(observed, want string) bool { return observed == want },
			windowSupport: true,
		},
	}
}

func TestLiveProfile_AssertsVersionAndAdmitsAggregates(t *testing.T) {
	for _, tc := range liveProfileCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			conn, db := openLiveProfileConn(t, ctx, tc)
			defer conn.Close()
			defer db.Close()

			assertLiveProfileVersion(t, ctx, conn, tc)
			assertLiveProfileAdmitsCountStar(t, ctx, conn, tc)
			assertLiveProfileAdmitsDirectColumnAggregates(t, ctx, conn, tc)
			assertLiveProfileRejectsUnqualified(t, ctx, conn, tc)
			assertLiveProfileRejectsQualifiedCall(t, ctx, conn, tc)
			assertLiveProfileRejectsQuotedCall(t, ctx, conn, tc)
			assertLiveProfileRejectsNoncanonicalSpacing(t, ctx, conn, tc)
			assertLiveProfileRejectsDistinct(t, ctx, conn, tc)
			assertLiveProfileRejectsNestedOperand(t, ctx, conn, tc)
			assertLiveProfileRejectsUnknownFunction(t, ctx, conn, tc)
			assertLiveProfileAdmitsMixedLiteralScalars(t, ctx, conn, tc)
			assertLiveProfileAdmitsLiteralOnlyShapes(t, ctx, conn, tc)
			assertLiveProfileAdmitsReversedOperandShapes(t, ctx, conn, tc)
			assertLiveProfileAdmitsAllConstantShapes(t, ctx, conn, tc)
			assertLiveProfileRejectsMixedLiteralNegatives(t, ctx, conn, tc)
			if tc.windowSupport {
				assertLiveProfileAdmitsRankingWindows(t, ctx, conn, tc)
				assertLiveProfileRejectsExplicitFrame(t, ctx, conn, tc)
				assertLiveProfileRejectsNamedWindow(t, ctx, conn, tc)
				assertLiveProfileRejectsMissingOrder(t, ctx, conn, tc)
			} else {
				assertLiveProfileRejectsRankingWindows(t, ctx, conn, tc)
			}
		})
	}
}

func openLiveProfileConn(t *testing.T, ctx context.Context, tc liveProfileCase) (*sql.Conn, *sql.DB) {
	t.Helper()
	host := envOr(tc.dsnEnvHost, "127.0.0.1")
	port := envIntOr(tc.dsnEnvPort, tc.defaultPort)
	user := envOr(tc.dsnEnvUser, "root")
	pass := envOr(tc.dsnEnvPass, tc.defaultPassword())
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/app?parseTime=true&timeout=10s", user, pass, host, port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("%s service unavailable at configured endpoint (driver error suppressed)", tc.name)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Fatalf("%s service ping unavailable (driver error suppressed)", tc.name)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		db.Close()
		t.Fatalf("%s service connection unavailable (driver error suppressed)", tc.name)
	}
	return conn, db
}

func (tc liveProfileCase) defaultPassword() string {
	if tc.dialect == DialectTiDB {
		return ""
	}
	return "root"
}

func assertLiveProfileVersion(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	var version string
	if err := conn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		t.Fatalf("%s VERSION probe failed (driver error suppressed)", tc.name)
	}
	if !tc.versionMatch(version, tc.exactVersion) {
		t.Fatalf("%s VERSION=%q did not match exact expected %q", tc.name, version, tc.exactVersion)
	}
	t.Logf("%s observed VERSION %q", tc.name, version)
}

func assertLiveProfileAdmitsCountStar(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("%s new session: %v", tc.name, err)
	}
	result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:             "SELECT COUNT(*) FROM app.builtin_semantic_facts",
		Dialect:         tc.dialect,
		AnalysisProfile: QueryAccessAnalysisProfileEmpty,
		DefaultSchema:   "app",
	})
	if err != nil {
		t.Fatalf("%s COUNT(*) analyze: %v", tc.name, err)
	}
	if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
		t.Fatalf("%s COUNT(*) classification=%q admission=%q, want read_only/admissible", tc.name, result.ReadClassification, result.Admission)
	}
}

func assertLiveProfileAdmitsDirectColumnAggregates(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	for _, probe := range []struct {
		name string
		sql  string
	}{
		{"COUNT", "SELECT COUNT(amount) FROM app.builtin_semantic_facts"},
		{"SUM", "SELECT SUM(amount) FROM app.builtin_semantic_facts"},
		{"AVG", "SELECT AVG(amount) FROM app.builtin_semantic_facts"},
		{"MIN", "SELECT MIN(amount) FROM app.builtin_semantic_facts"},
		{"MAX", "SELECT MAX(amount) FROM app.builtin_semantic_facts"},
	} {
		session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
		if err != nil {
			t.Fatalf("%s %s new session: %v", tc.name, probe.name, err)
		}
		result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
			SQL:             probe.sql,
			Dialect:         tc.dialect,
			AnalysisProfile: QueryAccessAnalysisProfileEmpty,
			DefaultSchema:   "app",
		})
		if err != nil {
			t.Fatalf("%s %s analyze: %v", tc.name, probe.name, err)
		}
		if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
			t.Fatalf("%s %s classification=%q admission=%q, want read_only/admissible", tc.name, probe.name, result.ReadClassification, result.Admission)
		}
	}
}

func assertLiveProfileRejectsUnqualified(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("%s new session: %v", tc.name, err)
	}
	result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:             "SELECT COUNT(*) FROM builtin_semantic_facts",
		Dialect:         tc.dialect,
		AnalysisProfile: QueryAccessAnalysisProfileEmpty,
		DefaultSchema:   "app",
	})
	if err != nil {
		t.Fatalf("%s unqualified analyze: %v", tc.name, err)
	}
	if result.ReadClassification == QueryAccessReadOnly && result.Admission == QueryAccessAdmissible {
		t.Fatalf("%s unqualified relation was promoted", tc.name)
	}
}

func assertLiveProfileRejectsQualifiedCall(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("%s new session: %v", tc.name, err)
	}
	result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:             "SELECT app.COUNT(*) FROM app.builtin_semantic_facts",
		Dialect:         tc.dialect,
		AnalysisProfile: QueryAccessAnalysisProfileEmpty,
		DefaultSchema:   "app",
	})
	if err != nil {
		t.Fatalf("%s qualified call analyze: %v", tc.name, err)
	}
	if result.ReadClassification == QueryAccessReadOnly && result.Admission == QueryAccessAdmissible {
		t.Fatalf("%s qualified builtin call was promoted", tc.name)
	}
}

func assertLiveProfileRejectsQuotedCall(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("%s new session: %v", tc.name, err)
	}
	result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:             "SELECT `COUNT`(*) FROM app.builtin_semantic_facts",
		Dialect:         tc.dialect,
		AnalysisProfile: QueryAccessAnalysisProfileEmpty,
		DefaultSchema:   "app",
	})
	if err != nil {
		t.Fatalf("%s quoted call analyze: %v", tc.name, err)
	}
	if result.ReadClassification == QueryAccessReadOnly && result.Admission == QueryAccessAdmissible {
		t.Fatalf("%s quoted builtin call was promoted", tc.name)
	}
}

func assertLiveProfileRejectsNoncanonicalSpacing(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("%s new session: %v", tc.name, err)
	}
	result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:             "SELECT COUNT (id) FROM app.builtin_semantic_facts",
		Dialect:         tc.dialect,
		AnalysisProfile: QueryAccessAnalysisProfileEmpty,
		DefaultSchema:   "app",
	})
	if err != nil {
		t.Fatalf("%s spacing analyze: %v", tc.name, err)
	}
	if result.ReadClassification == QueryAccessReadOnly && result.Admission == QueryAccessAdmissible {
		t.Fatalf("%s noncanonical spacing was promoted", tc.name)
	}
}

func assertLiveProfileRejectsDistinct(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("%s new session: %v", tc.name, err)
	}
	result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:             "SELECT COUNT(DISTINCT amount) FROM app.builtin_semantic_facts",
		Dialect:         tc.dialect,
		AnalysisProfile: QueryAccessAnalysisProfileEmpty,
		DefaultSchema:   "app",
	})
	if err != nil {
		t.Fatalf("%s distinct analyze: %v", tc.name, err)
	}
	if result.ReadClassification == QueryAccessReadOnly && result.Admission == QueryAccessAdmissible {
		t.Fatalf("%s DISTINCT was promoted", tc.name)
	}
}

func assertLiveProfileRejectsNestedOperand(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("%s new session: %v", tc.name, err)
	}
	result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:             "SELECT COUNT(ABS(amount)) FROM app.builtin_semantic_facts",
		Dialect:         tc.dialect,
		AnalysisProfile: QueryAccessAnalysisProfileEmpty,
		DefaultSchema:   "app",
	})
	if err != nil {
		t.Fatalf("%s nested analyze: %v", tc.name, err)
	}
	if result.ReadClassification == QueryAccessReadOnly && result.Admission == QueryAccessAdmissible {
		t.Fatalf("%s nested operand was promoted", tc.name)
	}
}

func assertLiveProfileRejectsUnknownFunction(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("%s new session: %v", tc.name, err)
	}
	result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:             "SELECT app_specific_rollup(amount) FROM app.builtin_semantic_facts",
		Dialect:         tc.dialect,
		AnalysisProfile: QueryAccessAnalysisProfileEmpty,
		DefaultSchema:   "app",
	})
	if err != nil {
		t.Fatalf("%s unknown analyze: %v", tc.name, err)
	}
	if result.ReadClassification == QueryAccessReadOnly && result.Admission == QueryAccessAdmissible {
		t.Fatalf("%s unknown function was promoted", tc.name)
	}
}

func assertLiveProfileAdmitsRankingWindows(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	for _, probe := range []struct {
		name string
		sql  string
	}{
		{"ROW_NUMBER", "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY amount DESC) FROM app.builtin_semantic_facts"},
		{"RANK", "SELECT RANK() OVER (PARTITION BY dept ORDER BY amount DESC) FROM app.builtin_semantic_facts"},
		{"DENSE_RANK", "SELECT DENSE_RANK() OVER (PARTITION BY dept ORDER BY amount DESC) FROM app.builtin_semantic_facts"},
	} {
		session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
		if err != nil {
			t.Fatalf("%s %s new session: %v", tc.name, probe.name, err)
		}
		result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
			SQL:             probe.sql,
			Dialect:         tc.dialect,
			AnalysisProfile: QueryAccessAnalysisProfileEmpty,
			DefaultSchema:   "app",
		})
		if err != nil {
			t.Fatalf("%s %s analyze: %v", tc.name, probe.name, err)
		}
		if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
			t.Fatalf("%s %s classification=%q admission=%q, want read_only/admissible", tc.name, probe.name, result.ReadClassification, result.Admission)
		}
	}
}

func assertLiveProfileRejectsRankingWindows(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("%s new session: %v", tc.name, err)
	}
	result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:             "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY amount DESC) FROM app.builtin_semantic_facts",
		Dialect:         tc.dialect,
		AnalysisProfile: QueryAccessAnalysisProfileEmpty,
		DefaultSchema:   "app",
	})
	if err != nil {
		t.Fatalf("%s deferred window analyze: %v", tc.name, err)
	}
	if result.ReadClassification == QueryAccessReadOnly && result.Admission == QueryAccessAdmissible {
		t.Fatalf("%s ranking window was promoted on a window-deferred profile", tc.name)
	}
}

func assertLiveProfileRejectsExplicitFrame(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("%s new session: %v", tc.name, err)
	}
	result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:             "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY amount ROWS BETWEEN 1 PRECEDING AND CURRENT ROW) FROM app.builtin_semantic_facts",
		Dialect:         tc.dialect,
		AnalysisProfile: QueryAccessAnalysisProfileEmpty,
		DefaultSchema:   "app",
	})
	if err != nil {
		t.Fatalf("%s frame analyze: %v", tc.name, err)
	}
	if result.ReadClassification == QueryAccessReadOnly && result.Admission == QueryAccessAdmissible {
		t.Fatalf("%s explicit frame was promoted", tc.name)
	}
}

func assertLiveProfileRejectsNamedWindow(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("%s new session: %v", tc.name, err)
	}
	result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:             "SELECT ROW_NUMBER() OVER w FROM app.builtin_semantic_facts WINDOW w AS (PARTITION BY dept ORDER BY amount)",
		Dialect:         tc.dialect,
		AnalysisProfile: QueryAccessAnalysisProfileEmpty,
		DefaultSchema:   "app",
	})
	if err != nil {
		t.Fatalf("%s named window analyze: %v", tc.name, err)
	}
	if result.ReadClassification == QueryAccessReadOnly && result.Admission == QueryAccessAdmissible {
		t.Fatalf("%s named window was promoted", tc.name)
	}
}

func assertLiveProfileRejectsMissingOrder(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
	if err != nil {
		t.Fatalf("%s new session: %v", tc.name, err)
	}
	result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
		SQL:             "SELECT ROW_NUMBER() OVER (PARTITION BY dept) FROM app.builtin_semantic_facts",
		Dialect:         tc.dialect,
		AnalysisProfile: QueryAccessAnalysisProfileEmpty,
		DefaultSchema:   "app",
	})
	if err != nil {
		t.Fatalf("%s missing order analyze: %v", tc.name, err)
	}
	if result.ReadClassification == QueryAccessReadOnly && result.Admission == QueryAccessAdmissible {
		t.Fatalf("%s missing ORDER BY was promoted", tc.name)
	}
}

func assertLiveProfileAdmitsMixedLiteralScalars(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	for _, probe := range []struct {
		name string
		sql  string
	}{
		{"COALESCE", "SELECT COALESCE(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts"},
		{"NULLIF", "SELECT NULLIF(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts"},
		{"IFNULL", "SELECT IFNULL(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts"},
	} {
		session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
		if err != nil {
			t.Fatalf("%s %s new session: %v", tc.name, probe.name, err)
		}
		result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
			SQL:           probe.sql,
			Dialect:       tc.dialect,
			DefaultSchema: "app",
		})
		if err != nil {
			t.Fatalf("%s %s analyze: %v", tc.name, probe.name, err)
		}
		if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
			t.Fatalf("%s %s classification=%q admission=%q, want read_only/admissible", tc.name, probe.name, result.ReadClassification, result.Admission)
		}
		// Exact requirement set — no extras, no missing, no literal-derived
		wantReqs := []QueryAccessRequirement{
			{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			{Object: "app.builtin_semantic_facts.name", Privilege: "read_column"},
		}
		if len(result.Requirements) != len(wantReqs) {
			t.Errorf("%s %s requirement count: got %d, want %d; got=%v", tc.name, probe.name, len(result.Requirements), len(wantReqs), result.Requirements)
		}
		for i, got := range result.Requirements {
			if i < len(wantReqs) && got != wantReqs[i] {
				t.Errorf("%s %s requirements[%d]: got %+v, want %+v", tc.name, probe.name, i, got, wantReqs[i])
			}
		}
		// No-leak: SECRET_LITERAL must not appear in struct dump, JSON, or error
		dump := fmt.Sprintf("%+v", result)
		if strings.Contains(dump, "SECRET_LITERAL") {
			t.Errorf("%s %s leaked SECRET_LITERAL in struct dump", tc.name, probe.name)
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("%s %s marshal: %v", tc.name, probe.name, err)
		}
		if strings.Contains(string(data), "SECRET_LITERAL") {
			t.Errorf("%s %s leaked SECRET_LITERAL in JSON", tc.name, probe.name)
		}
	}
}

func assertLiveProfileAdmitsLiteralOnlyShapes(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	for _, probe := range []struct {
		name     string
		sql      string
		wantReqs []QueryAccessRequirement
	}{
		{
			name: "literal_only_lower",
			sql:  "SELECT LOWER('SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			},
		},
		{
			name: "literal_only_upper",
			sql:  "SELECT UPPER('SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			},
		},
		{
			name: "literal_only_length",
			sql:  "SELECT LENGTH('SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			},
		},
		{
			name: "literal_only_char_length",
			sql:  "SELECT CHAR_LENGTH('SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			},
		},
		{
			name: "literal_only_abs",
			sql:  "SELECT ABS(42) FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			},
		},
		{
			name: "literal_only_ceil",
			sql:  "SELECT CEIL(42) FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			},
		},
		{
			name: "literal_only_ceiling",
			sql:  "SELECT CEILING(42) FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			},
		},
		{
			name: "literal_only_floor",
			sql:  "SELECT FLOOR(42) FROM app.builtin_semantic_facts",
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
	} {
		session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
		if err != nil {
			t.Fatalf("%s %s new session: %v", tc.name, probe.name, err)
		}
		result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
			SQL:           probe.sql,
			Dialect:       tc.dialect,
			DefaultSchema: "app",
		})
		if err != nil {
			t.Fatalf("%s %s analyze: %v", tc.name, probe.name, err)
		}
		if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
			t.Fatalf("%s %s classification=%q admission=%q, want read_only/admissible", tc.name, probe.name, result.ReadClassification, result.Admission)
		}
		// Exact requirement set — literal-only produces table-only requirements
		if len(result.Requirements) != len(probe.wantReqs) {
			t.Errorf("%s %s requirement count: got %d, want %d; got=%v", tc.name, probe.name, len(result.Requirements), len(probe.wantReqs), result.Requirements)
		}
		for i, got := range result.Requirements {
			if i < len(probe.wantReqs) && got != probe.wantReqs[i] {
				t.Errorf("%s %s requirements[%d]: got %+v, want %+v", tc.name, probe.name, i, got, probe.wantReqs[i])
			}
		}
		// No-leak: SECRET_LITERAL must not appear in struct dump or JSON
		dump := fmt.Sprintf("%+v", result)
		if strings.Contains(dump, "SECRET_LITERAL") {
			t.Errorf("%s %s leaked SECRET_LITERAL in struct dump", tc.name, probe.name)
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("%s %s marshal: %v", tc.name, probe.name, err)
		}
		if strings.Contains(string(data), "SECRET_LITERAL") {
			t.Errorf("%s %s leaked SECRET_LITERAL in JSON", tc.name, probe.name)
		}
	}
}

func assertLiveProfileAdmitsReversedOperandShapes(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	for _, probe := range []struct {
		name     string
		sql      string
		wantReqs []QueryAccessRequirement
	}{
		{
			name: "reversed_coalesce",
			sql:  "SELECT COALESCE('SECRET_LITERAL', name) FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
				{Object: "app.builtin_semantic_facts.name", Privilege: "read_column"},
			},
		},
		{
			name: "reversed_nullif",
			sql:  "SELECT NULLIF('SECRET_LITERAL', name) FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
				{Object: "app.builtin_semantic_facts.name", Privilege: "read_column"},
			},
		},
		{
			name: "reversed_ifnull",
			sql:  "SELECT IFNULL('SECRET_LITERAL', name) FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
				{Object: "app.builtin_semantic_facts.name", Privilege: "read_column"},
			},
		},
	} {
		session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
		if err != nil {
			t.Fatalf("%s %s new session: %v", tc.name, probe.name, err)
		}
		result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
			SQL:           probe.sql,
			Dialect:       tc.dialect,
			DefaultSchema: "app",
		})
		if err != nil {
			t.Fatalf("%s %s analyze: %v", tc.name, probe.name, err)
		}
		if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
			t.Fatalf("%s %s classification=%q admission=%q, want read_only/admissible", tc.name, probe.name, result.ReadClassification, result.Admission)
		}
		// Exact requirement set — reversed produces table + column requirements
		if len(result.Requirements) != len(probe.wantReqs) {
			t.Errorf("%s %s requirement count: got %d, want %d; got=%v", tc.name, probe.name, len(result.Requirements), len(probe.wantReqs), result.Requirements)
		}
		for i, got := range result.Requirements {
			if i < len(probe.wantReqs) && got != probe.wantReqs[i] {
				t.Errorf("%s %s requirements[%d]: got %+v, want %+v", tc.name, probe.name, i, got, probe.wantReqs[i])
			}
		}
		// No-leak: SECRET_LITERAL must not appear in struct dump or JSON
		dump := fmt.Sprintf("%+v", result)
		if strings.Contains(dump, "SECRET_LITERAL") {
			t.Errorf("%s %s leaked SECRET_LITERAL in struct dump", tc.name, probe.name)
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("%s %s marshal: %v", tc.name, probe.name, err)
		}
		if strings.Contains(string(data), "SECRET_LITERAL") {
			t.Errorf("%s %s leaked SECRET_LITERAL in JSON", tc.name, probe.name)
		}
	}
}

func assertLiveProfileAdmitsAllConstantShapes(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	for _, probe := range []struct {
		name     string
		sql      string
		wantReqs []QueryAccessRequirement
	}{
		{
			name: "all_constant_coalesce",
			sql:  "SELECT COALESCE('SECRET_LITERAL', 'SECRET_LITERAL2') FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			},
		},
		{
			name: "all_constant_nullif",
			sql:  "SELECT NULLIF('SECRET_LITERAL', 'SECRET_LITERAL2') FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			},
		},
		{
			name: "all_constant_ifnull",
			sql:  "SELECT IFNULL('SECRET_LITERAL', 'SECRET_LITERAL2') FROM app.builtin_semantic_facts",
			wantReqs: []QueryAccessRequirement{
				{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
			},
		},
	} {
		session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
		if err != nil {
			t.Fatalf("%s %s new session: %v", tc.name, probe.name, err)
		}
		result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
			SQL:           probe.sql,
			Dialect:       tc.dialect,
			DefaultSchema: "app",
		})
		if err != nil {
			t.Fatalf("%s %s analyze: %v", tc.name, probe.name, err)
		}
		if result.ReadClassification != QueryAccessReadOnly || result.Admission != QueryAccessAdmissible {
			t.Fatalf("%s %s classification=%q admission=%q, want read_only/admissible", tc.name, probe.name, result.ReadClassification, result.Admission)
		}
		// Exact requirement set — all-constant produces table-only requirements
		if len(result.Requirements) != len(probe.wantReqs) {
			t.Errorf("%s %s requirement count: got %d, want %d; got=%v", tc.name, probe.name, len(result.Requirements), len(probe.wantReqs), result.Requirements)
		}
		for i, got := range result.Requirements {
			if i < len(probe.wantReqs) && got != probe.wantReqs[i] {
				t.Errorf("%s %s requirements[%d]: got %+v, want %+v", tc.name, probe.name, i, got, probe.wantReqs[i])
			}
		}
		// No-leak: SECRET_LITERAL and SECRET_LITERAL2 must not appear in struct dump or JSON
		dump := fmt.Sprintf("%+v", result)
		if strings.Contains(dump, "SECRET_LITERAL") {
			t.Errorf("%s %s leaked SECRET_LITERAL in struct dump", tc.name, probe.name)
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("%s %s marshal: %v", tc.name, probe.name, err)
		}
		if strings.Contains(string(data), "SECRET_LITERAL") {
			t.Errorf("%s %s leaked SECRET_LITERAL in JSON", tc.name, probe.name)
		}
	}
}

func assertLiveProfileRejectsMixedLiteralNegatives(t *testing.T, ctx context.Context, conn *sql.Conn, tc liveProfileCase) {
	t.Helper()
	for _, probe := range []struct {
		name string
		sql  string
	}{
		{"relationless", "SELECT COALESCE(0, 1)"},
		{"coalesce_arity_1", "SELECT COALESCE(amount) FROM app.builtin_semantic_facts"},
		{"coalesce_arity_3", "SELECT COALESCE('x', 'y', 'z') FROM app.builtin_semantic_facts"},
		{"nested_expr", "SELECT COALESCE(ABS(amount), name) FROM app.builtin_semantic_facts"},
		{"cast_operand", "SELECT COALESCE(CAST(amount AS CHAR), name) FROM app.builtin_semantic_facts"},
		{"parameter", "SELECT COALESCE(?, name) FROM app.builtin_semantic_facts"},
		{"unknown_func", "SELECT UNKNOWN_FUNC(amount) FROM app.builtin_semantic_facts"},
	} {
		session, err := NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
		if err != nil {
			t.Fatalf("%s %s new session: %v", tc.name, probe.name, err)
		}
		result, err := AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, QueryAccessRequest{
			SQL:           probe.sql,
			Dialect:       tc.dialect,
			DefaultSchema: "app",
		})
		if err != nil {
			t.Fatalf("%s %s analyze: %v", tc.name, probe.name, err)
		}
		if result.ReadClassification == QueryAccessReadOnly && result.Admission == QueryAccessAdmissible {
			t.Errorf("%s %s was promoted but should not be: classification=%q admission=%q", tc.name, probe.name, result.ReadClassification, result.Admission)
		}
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

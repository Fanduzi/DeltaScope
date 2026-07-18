// Package cli adds no-leak regression coverage for the MySQL/TiDB builtin-effect
// identity feasibility probe boundary on the CLI surface. The live Docker
// probes live in internal/infrastructure/metadata/mysql/builtin_effect_identity_live_probes_test.go
// (build tag: integration) and do NOT introduce a new CLI path. This file
// therefore tests the UNCHANGED CLI surface (`query-access analyze`) under
// MySQL/TiDB function-bearing SQL with injected markers, and asserts those
// markers, identity facts, candidates, session/context data, manifest data,
// raw SQL, and `severity` are absent from stdout/stderr/JSON output.
//
// input: CLI invocations with MySQL/TiDB function-bearing SQL carrying injected
//
//	function-name and literal markers on the normal path
//
// output: CLI stdout/stderr/JSON contain none of the injected markers, no
//
//	identity/candidate/manifest/session fields, no raw SQL, no `severity` field;
//	DSN/credential/driver-error injection is covered by the HTTP error boundary
//
// pos: CLI no-leak regression for the MySQL/TiDB probe boundary; no new CLI
//
//	path is introduced or exercised here
//
// note: if this file changes, update this header, the module README.md, and
//
//	the decision record's no-leak evidence section. See
//	docs/decisions/2026-07-17-query-access-mysql-tidb-effect-identity-feasibility.md
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// mysqlTiDBCLINoLeakMarkers is the canonical marker set for the MySQL/TiDB
// builtin-identity probe boundary on the CLI surface. Every marker must be
// absent from stdout and stderr.
var mysqlTiDBCLINoLeakMarkers = []string{
	"my_secret_udf",
	"SECRET_LITERAL",
	"P@ssw0rd",
	"postgres://user:pass@host",
	"mysql://root:root@host:3406",
	"driver: bad conn",
	"severity",
	"identity",
	"candidate",
	"manifest",
	"session",
	"dsn",
}

// TestQueryAccess_MySQLTiDBProbeBoundary_NoLeak verifies the unchanged CLI
// surface (`query-access analyze`) does not leak injected markers, identity
// facts, candidates, session/context, manifest, raw SQL, or severity when
// analyzing MySQL/TiDB function-bearing SQL that references marker function
// names and literals.
//
// This test covers the CLI NORMAL path (successful analysis). It injects
// function-name and literal markers via SQL. DSN/credential/driver-error
// markers are NOT injected through the CLI normal path because the CLI does
// not expose a live-connection error path in offline mode; those markers are
// covered by the HTTP error-boundary test
// (TestHandlerQueryAccess_MySQLTiDBProbeBoundary_DriverErrorNoLeak). The
// marker scan still includes all markers as a defense-in-depth check that no
// hardcoded DSN/credential/driver text appears in CLI output.
func TestQueryAccess_MySQLTiDBProbeBoundary_NoLeak(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		sql     string
		dialect string
	}{
		{name: "mysql_udf_marker_function", sql: "SELECT my_secret_udf(id) FROM users", dialect: "mysql"},
		{name: "tidb_udf_marker_function", sql: "SELECT my_secret_udf(id) FROM users", dialect: "tidb"},
		{name: "mysql_count_with_marker_literal", sql: "SELECT COUNT('SECRET_LITERAL') FROM users", dialect: "mysql"},
		{name: "tidb_count_with_marker_literal", sql: "SELECT COUNT('SECRET_LITERAL') FROM users", dialect: "tidb"},
		{name: "mysql_qualified_marker_function", sql: "SELECT app.my_secret_udf(id) FROM users", dialect: "mysql"},
		{name: "tidb_qualified_marker_function", sql: "SELECT app.my_secret_udf(id) FROM users", dialect: "tidb"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			exitCode := Execute(context.Background(), []string{
				"query-access", "analyze",
				"--sql", tc.sql,
				"--dialect", tc.dialect,
				"--mode", "strict",
			}, &bytes.Buffer{}, &stdout, &stderr)

			// Function-bearing SQL must yield indeterminate admission → exit
			// code 2 (indeterminate), not 0 (admissible) or 1 (rejected).
			if exitCode != 2 {
				t.Fatalf("%s: expected exit 2 for indeterminate function-bearing SQL, got %d: %s", tc.name, exitCode, stdout.String())
			}

			stdoutStr := stdout.String()
			stderrStr := stderr.String()

			// Assert no-leak on stdout and stderr.
			for _, marker := range mysqlTiDBCLINoLeakMarkers {
				if strings.Contains(stdoutStr, marker) {
					t.Errorf("%s: CLI stdout leaked marker %q: %s", tc.name, marker, stdoutStr)
				}
				if strings.Contains(stderrStr, marker) {
					t.Errorf("%s: CLI stderr leaked marker %q: %s", tc.name, marker, stderrStr)
				}
			}

			// The raw SQL must not appear in stdout or stderr. The hard
			// boundary forbids raw-SQL output on any CLI surface.
			if strings.Contains(stdoutStr, tc.sql) {
				t.Errorf("%s: CLI stdout must not embed raw SQL: %s", tc.name, stdoutStr)
			}
			if strings.Contains(stderrStr, tc.sql) {
				t.Errorf("%s: CLI stderr must not embed raw SQL: %s", tc.name, stderrStr)
			}

			// If stdout is JSON, verify no forbidden top-level fields.
			if strings.HasPrefix(strings.TrimSpace(stdoutStr), "{") {
				var payload map[string]any
				if err := json.Unmarshal([]byte(stdoutStr), &payload); err == nil {
					forbiddenTopLevel := []string{
						"severity", "identity", "candidate", "manifest", "session",
						"context", "dsn", "raw_sql", "driver_error",
					}
					for _, field := range forbiddenTopLevel {
						if _, ok := payload[field]; ok {
							t.Errorf("%s: CLI JSON must not carry top-level field %q: %s", tc.name, field, stdoutStr)
						}
					}
				}
			}
		})
	}
}

// TestQueryAccess_MySQLTiDBProbeBoundary_FunctionFreeAdmissibleNoLeak
// verifies the unchanged CLI surface keeps function-free admissible MySQL/TiDB
// SELECTs leak-free under marker injection. This is the non-regression
// baseline.
func TestQueryAccess_MySQLTiDBProbeBoundary_FunctionFreeAdmissibleNoLeak(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		sql     string
		dialect string
	}{
		{name: "mysql_where_marker_literal", sql: "SELECT id FROM users WHERE name = 'SECRET_LITERAL'", dialect: "mysql"},
		{name: "tidb_where_marker_literal", sql: "SELECT id FROM users WHERE name = 'SECRET_LITERAL'", dialect: "tidb"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			exitCode := Execute(context.Background(), []string{
				"query-access", "analyze",
				"--sql", tc.sql,
				"--dialect", tc.dialect,
				"--mode", "strict",
			}, &bytes.Buffer{}, &stdout, &stderr)

			if exitCode != 0 {
				t.Fatalf("%s: expected exit 0 for admissible, got %d: %s", tc.name, exitCode, stderr.String())
			}

			stdoutStr := stdout.String()
			stderrStr := stderr.String()
			for _, marker := range mysqlTiDBCLINoLeakMarkers {
				if strings.Contains(stdoutStr, marker) {
					t.Errorf("%s: CLI stdout leaked marker %q: %s", tc.name, marker, stdoutStr)
				}
				if strings.Contains(stderrStr, marker) {
					t.Errorf("%s: CLI stderr leaked marker %q: %s", tc.name, marker, stderrStr)
				}
			}
		})
	}
}

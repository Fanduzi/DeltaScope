//go:build integration

// Package cli verifies CLI query-access online behavior for mixed-literal
// operands (COALESCE, NULLIF, IFNULL) against real Docker-backed MySQL and TiDB.
// input: CLI invocations with connection flags pointing at running Docker containers
// output: end-to-end proof that mixed-literal scalar operands yield
//
//	read_only + admissible via the CLI online path across all four MySQL/TiDB
//	versions, with no-leak assertions on stdout/stderr
//
// pos: CLI online E2E coverage for the mixed-literal scalar operand feature
// note: if this file changes, update this header and module README.md.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	deltascope "github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestQueryAccessOnline_MixedLiteralScalars(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		port    int
		user    string
		pass    string
		dialect string
	}{
		{"mysql57", "127.0.0.1", 3507, "root", "root", "mysql"},
		{"mysql80", "127.0.0.1", 3800, "root", "root", "mysql"},
		{"mysql84", "127.0.0.1", 3840, "root", "root", "mysql"},
		{"tidb85", "127.0.0.1", 4850, "root", "", "tidb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, probe := range []struct {
				name string
				sql  string
			}{
				{"COALESCE", "SELECT COALESCE(amount, 0) FROM app.builtin_semantic_facts"},
				{"NULLIF", "SELECT NULLIF(amount, 0) FROM app.builtin_semantic_facts"},
				{"IFNULL", "SELECT IFNULL(amount, 0) FROM app.builtin_semantic_facts"},
			} {
				t.Run(probe.name, func(t *testing.T) {
					var stdout, stderr bytes.Buffer
					args := []string{
						"query-access", "analyze",
						"--sql", probe.sql,
						"--host", tc.host,
						"--port", fmt.Sprintf("%d", tc.port),
						"--user", tc.user,
						"--schema", "app",
						"--dialect", tc.dialect,
					}
					if tc.pass != "" {
						t.Setenv("MYSQL_PASSWORD", tc.pass)
						args = append(args, "--password-env", "MYSQL_PASSWORD")
					}
					exitCode := Execute(
						t.Context(),
						args,
						&bytes.Buffer{}, &stdout, &stderr,
					)
					if exitCode != 0 {
						t.Fatalf("exit code: got %d, want 0; stderr: %s", exitCode, stderr.String())
					}
					var result deltascope.QueryAccessResult
					if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
						t.Fatalf("unmarshal: %v", err)
					}
					if result.ReadClassification != deltascope.QueryAccessReadOnly {
						t.Errorf("classification: got %q, want read_only", result.ReadClassification)
					}
					if result.Admission != deltascope.QueryAccessAdmissible {
						t.Errorf("admission: got %q, want admissible", result.Admission)
					}
					// No-leak: stdout and stderr must not contain injected markers.
					if strings.Contains(stdout.String(), "SECRET_LITERAL") {
						t.Errorf("stdout leaked SECRET_LITERAL")
					}
					if strings.Contains(stderr.String(), "SECRET_LITERAL") {
						t.Errorf("stderr leaked SECRET_LITERAL")
					}
				})
			}
		})
	}
}

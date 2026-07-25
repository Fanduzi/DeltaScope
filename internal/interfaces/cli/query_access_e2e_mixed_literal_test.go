//go:build integration

// Package cli verifies CLI query-access online behavior for mixed-literal
// operands (COALESCE, NULLIF, IFNULL) against real Docker-backed MySQL and TiDB.
// input: CLI invocations with connection flags pointing at running Docker containers
// output: end-to-end proof that mixed-literal scalar operands yield
//
//	read_only + admissible via the CLI online path across all four MySQL/TiDB
//	versions, with exact requirement assertions and no-leak guards
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
	probes := []struct {
		name string
		sql  string
	}{
		{"COALESCE", "SELECT COALESCE(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts"},
		{"NULLIF", "SELECT NULLIF(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts"},
		{"IFNULL", "SELECT IFNULL(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts"},
	}

	// Online path: connection flags present.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, probe := range probes {
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

					// Exact requirement assertions.
					wantReqs := []deltascope.QueryAccessRequirement{
						{Object: "app.builtin_semantic_facts", Privilege: "read_table"},
						{Object: "app.builtin_semantic_facts.name", Privilege: "read_column"},
					}
					if len(result.Requirements) != len(wantReqs) {
						t.Fatalf("requirements: got %d items, want %d; got %+v", len(result.Requirements), len(wantReqs), result.Requirements)
					}
					for i, got := range result.Requirements {
						if got != wantReqs[i] {
							t.Errorf("requirements[%d]: got %+v, want %+v", i, got, wantReqs[i])
						}
					}

					// No-leak: stdout, stderr, and deserialized JSON fields.
					if strings.Contains(stdout.String(), "SECRET_LITERAL") {
						t.Errorf("stdout leaked SECRET_LITERAL")
					}
					if strings.Contains(stderr.String(), "SECRET_LITERAL") {
						t.Errorf("stderr leaked SECRET_LITERAL")
					}
					raw, _ := json.Marshal(result)
					if strings.Contains(string(raw), "SECRET_LITERAL") {
						t.Errorf("deserialized result leaked SECRET_LITERAL")
					}
				})
			}
		})
	}

	// Offline regression: no connection flags → indeterminate.
	t.Run("offline_indeterminate", func(t *testing.T) {
		for _, probe := range probes {
			t.Run(probe.name, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				args := []string{
					"query-access", "analyze",
					"--sql", probe.sql,
					"--dialect", "mysql",
				}
				exitCode := Execute(
					t.Context(),
					args,
					&bytes.Buffer{}, &stdout, &stderr,
				)
				if exitCode != exitQueryAccessIndeterminate {
					t.Errorf("offline exit code: got %d, want %d; stderr: %s", exitCode, exitQueryAccessIndeterminate, stderr.String())
				}
				if strings.Contains(stdout.String(), "SECRET_LITERAL") {
					t.Errorf("offline stdout leaked SECRET_LITERAL")
				}
				if strings.Contains(stderr.String(), "SECRET_LITERAL") {
					t.Errorf("offline stderr leaked SECRET_LITERAL")
				}
			})
		}
	})
}

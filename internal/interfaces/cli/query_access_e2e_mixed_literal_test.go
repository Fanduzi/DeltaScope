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
		name              string
		sql               string
		wantExitCode      int
		wantClassification string
		wantAdmission     string
		wantRequirements  []string
	}{
		// Original mixed-literal shapes (column-first operand).
		{
			name:              "COALESCE",
			sql:               "SELECT COALESCE(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantExitCode:      0,
			wantClassification: "read_only",
			wantAdmission:     "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
				"app.builtin_semantic_facts.name=read_column",
			},
		},
		{
			name:              "NULLIF",
			sql:               "SELECT NULLIF(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantExitCode:      0,
			wantClassification: "read_only",
			wantAdmission:     "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
				"app.builtin_semantic_facts.name=read_column",
			},
		},
		{
			name:              "IFNULL",
			sql:               "SELECT IFNULL(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantExitCode:      0,
			wantClassification: "read_only",
			wantAdmission:     "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
				"app.builtin_semantic_facts.name=read_column",
			},
		},
		// Literal-only scalar.
		{
			name:              "LOWER_literal",
			sql:               "SELECT LOWER('SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantExitCode:      0,
			wantClassification: "read_only",
			wantAdmission:     "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		// COUNT with literal argument.
		{
			name:              "COUNT_literal",
			sql:               "SELECT COUNT(1) FROM app.builtin_semantic_facts",
			wantExitCode:      0,
			wantClassification: "read_only",
			wantAdmission:     "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		// Reversed binary operand order (literal first, column second).
		{
			name:              "COALESCE_reversed",
			sql:               "SELECT COALESCE('SECRET_LITERAL', name) FROM app.builtin_semantic_facts",
			wantExitCode:      0,
			wantClassification: "read_only",
			wantAdmission:     "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
				"app.builtin_semantic_facts.name=read_column",
			},
		},
		// All-constant operands (no column reference).
		{
			name:              "COALESCE_all_constant",
			sql:               "SELECT COALESCE('SECRET_LITERAL', 'SECRET_LITERAL2') FROM app.builtin_semantic_facts",
			wantExitCode:      0,
			wantClassification: "read_only",
			wantAdmission:     "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
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
					if exitCode != probe.wantExitCode {
						t.Fatalf("exit code: got %d, want %d; stderr: %s", exitCode, probe.wantExitCode, stderr.String())
					}
					var result deltascope.QueryAccessResult
					if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
						t.Fatalf("unmarshal: %v", err)
					}
					if string(result.ReadClassification) != probe.wantClassification {
						t.Errorf("classification: got %q, want %q", result.ReadClassification, probe.wantClassification)
					}
					if string(result.Admission) != probe.wantAdmission {
						t.Errorf("admission: got %q, want %q", result.Admission, probe.wantAdmission)
					}

					// Exact requirement assertions.
					if len(result.Requirements) != len(probe.wantRequirements) {
						t.Fatalf("requirements: got %d items, want %d; got %+v", len(result.Requirements), len(probe.wantRequirements), result.Requirements)
					}
					for i, got := range result.Requirements {
						want := probe.wantRequirements[i]
						gotStr := got.Object + "=" + got.Privilege
						if gotStr != want {
							t.Errorf("requirements[%d]: got %q, want %q", i, gotStr, want)
						}
					}

					// No-leak: stdout, stderr, and deserialized JSON fields.
					for _, marker := range []string{"SECRET_LITERAL", "SECRET_LITERAL2"} {
						if strings.Contains(stdout.String(), marker) {
							t.Errorf("stdout leaked %s", marker)
						}
						if strings.Contains(stderr.String(), marker) {
							t.Errorf("stderr leaked %s", marker)
						}
						raw, _ := json.Marshal(result)
						if strings.Contains(string(raw), marker) {
							t.Errorf("deserialized result leaked %s", marker)
						}
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
				var offlineResult deltascope.QueryAccessResult
				if err := json.Unmarshal(stdout.Bytes(), &offlineResult); err != nil {
					t.Fatalf("offline unmarshal: %v", err)
				}
				if offlineResult.ReadClassification != deltascope.QueryAccessIndeterminate {
					t.Errorf("offline classification: got %q, want indeterminate", offlineResult.ReadClassification)
				}
				if offlineResult.Admission != deltascope.QueryAccessIndeterminateAdmission {
					t.Errorf("offline admission: got %q, want indeterminate", offlineResult.Admission)
				}
				for _, marker := range []string{"SECRET_LITERAL", "SECRET_LITERAL2"} {
					if strings.Contains(stdout.String(), marker) {
						t.Errorf("offline stdout leaked %s", marker)
					}
					if strings.Contains(stderr.String(), marker) {
						t.Errorf("offline stderr leaked %s", marker)
					}
				}
			})
		}
	})
}

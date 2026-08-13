//go:build integration

// Package cli verifies online Query Access through both the in-process CLI
// adapter and a built real binary across Docker-backed MySQL and TiDB profiles.
// input: CLI invocations and a built deltascope binary pointed at MySQL 5.7/8.0/8.4 and TiDB 8.5 containers
// output: supported-profile routing, mixed/literal admission, exact requirements, offline regression, and marker no-leak evidence
// pos: Docker-backed CLI adapter and real-binary Query Access compatibility coverage
// note: if this file changes, update this header and module README.md.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	deltascope "github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestQueryAccessOnline_BuiltBinarySupportedProfiles(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	binary := filepath.Join(t.TempDir(), "deltascope")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "./cmd/deltascope")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	cases := []struct {
		name, port, dialect string
		password            bool
	}{
		{name: "mysql57", port: "3507", dialect: "mysql", password: true},
		{name: "mysql80", port: "3800", dialect: "mysql", password: true},
		{name: "mysql84", port: "3840", dialect: "mysql", password: true},
		{name: "tidb85", port: "4850", dialect: "tidb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			marker := "CLI_BUILT_BINARY_" + strings.ToUpper(tc.name) + "_MARKER"
			args := []string{
				"query-access", "analyze",
				"--sql", "SELECT COUNT(1) /* " + marker + " */ FROM app.builtin_semantic_facts",
				"--host", "127.0.0.1",
				"--port", tc.port,
				"--user", "root",
				"--schema", "app",
				"--dialect", tc.dialect,
			}
			command := exec.CommandContext(t.Context(), binary, args...)
			if tc.password {
				command.Env = append(os.Environ(), "CLI_BUILT_BINARY_PASSWORD=root")
				command.Args = append(command.Args, "--password-env", "CLI_BUILT_BINARY_PASSWORD")
			}
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			if err := command.Run(); err != nil {
				t.Fatalf("built CLI failed: %v; stdout=%s stderr=%s", err, stdout.String(), stderr.String())
			}
			var result deltascope.QueryAccessResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("decode built CLI output: %v; stdout=%s", err, stdout.String())
			}
			if result.ReadClassification != deltascope.QueryAccessReadOnly || result.Admission != deltascope.QueryAccessAdmissible {
				t.Fatalf("unexpected result: classification=%s admission=%s", result.ReadClassification, result.Admission)
			}
			if strings.Contains(stdout.String()+stderr.String(), marker) {
				t.Fatalf("built CLI leaked SQL marker: stdout=%s stderr=%s", stdout.String(), stderr.String())
			}
		})
	}
}

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
		name               string
		sql                string
		wantExitCode       int
		wantClassification string
		wantAdmission      string
		wantRequirements   []string
		relationless       bool
	}{
		// Original mixed-literal shapes (column-first operand).
		{
			name:               "COALESCE",
			sql:                "SELECT COALESCE(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
				"app.builtin_semantic_facts.name=read_column",
			},
		},
		{
			name:               "NULLIF",
			sql:                "SELECT NULLIF(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
				"app.builtin_semantic_facts.name=read_column",
			},
		},
		{
			name:               "IFNULL",
			sql:                "SELECT IFNULL(name, 'SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
				"app.builtin_semantic_facts.name=read_column",
			},
		},
		// Literal-only scalar.
		{
			name:               "LOWER_literal",
			sql:                "SELECT LOWER('SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		{
			name:               "UPPER_literal",
			sql:                "SELECT UPPER('SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		{
			name:               "LENGTH_literal",
			sql:                "SELECT LENGTH('SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		{
			name:               "CHAR_LENGTH_literal",
			sql:                "SELECT CHAR_LENGTH('SECRET_LITERAL') FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		{
			name:               "ABS_literal",
			sql:                "SELECT ABS(42) FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		{
			name:               "CEIL_literal",
			sql:                "SELECT CEIL(42) FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		{
			name:               "CEILING_literal",
			sql:                "SELECT CEILING(42) FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		{
			name:               "FLOOR_literal",
			sql:                "SELECT FLOOR(42) FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		// COUNT with literal argument.
		{
			name:               "COUNT_literal",
			sql:                "SELECT COUNT(1) FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		// Reversed binary operand order (literal first, column second).
		{
			name:               "COALESCE_reversed",
			sql:                "SELECT COALESCE('SECRET_LITERAL', name) FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
				"app.builtin_semantic_facts.name=read_column",
			},
		},
		{
			name:               "NULLIF_reversed",
			sql:                "SELECT NULLIF('SECRET_LITERAL', name) FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
				"app.builtin_semantic_facts.name=read_column",
			},
		},
		{
			name:               "IFNULL_reversed",
			sql:                "SELECT IFNULL('SECRET_LITERAL', name) FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
				"app.builtin_semantic_facts.name=read_column",
			},
		},
		// All-constant operands (no column reference).
		{
			name:               "COALESCE_all_constant",
			sql:                "SELECT COALESCE('SECRET_LITERAL', 'SECRET_LITERAL2') FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		{
			name:               "NULLIF_all_constant",
			sql:                "SELECT NULLIF('SECRET_LITERAL', 'SECRET_LITERAL2') FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		{
			name:               "IFNULL_all_constant",
			sql:                "SELECT IFNULL('SECRET_LITERAL', 'SECRET_LITERAL2') FROM app.builtin_semantic_facts",
			wantExitCode:       0,
			wantClassification: "read_only",
			wantAdmission:      "admissible",
			wantRequirements: []string{
				"app.builtin_semantic_facts=read_table",
			},
		},
		// Relationless (no FROM) literal-only shapes: nothing is read.
		{name: "relationless_lower", sql: "SELECT LOWER('SECRET_LITERAL')", wantExitCode: 0, wantClassification: "read_only", wantAdmission: "admissible", wantRequirements: nil, relationless: true},
		{name: "relationless_upper", sql: "SELECT UPPER('SECRET_LITERAL')", wantExitCode: 0, wantClassification: "read_only", wantAdmission: "admissible", wantRequirements: nil, relationless: true},
		{name: "relationless_length", sql: "SELECT LENGTH('SECRET_LITERAL')", wantExitCode: 0, wantClassification: "read_only", wantAdmission: "admissible", wantRequirements: nil, relationless: true},
		{name: "relationless_char_length", sql: "SELECT CHAR_LENGTH('SECRET_LITERAL')", wantExitCode: 0, wantClassification: "read_only", wantAdmission: "admissible", wantRequirements: nil, relationless: true},
		{name: "relationless_abs", sql: "SELECT ABS(42)", wantExitCode: 0, wantClassification: "read_only", wantAdmission: "admissible", wantRequirements: nil, relationless: true},
		{name: "relationless_ceil", sql: "SELECT CEIL(42)", wantExitCode: 0, wantClassification: "read_only", wantAdmission: "admissible", wantRequirements: nil, relationless: true},
		{name: "relationless_ceiling", sql: "SELECT CEILING(42)", wantExitCode: 0, wantClassification: "read_only", wantAdmission: "admissible", wantRequirements: nil, relationless: true},
		{name: "relationless_floor", sql: "SELECT FLOOR(42)", wantExitCode: 0, wantClassification: "read_only", wantAdmission: "admissible", wantRequirements: nil, relationless: true},
		{name: "relationless_count_literal", sql: "SELECT COUNT(1)", wantExitCode: 0, wantClassification: "read_only", wantAdmission: "admissible", wantRequirements: nil, relationless: true},
		{name: "relationless_coalesce", sql: "SELECT COALESCE('SECRET_LITERAL', 'SECRET_LITERAL2')", wantExitCode: 0, wantClassification: "read_only", wantAdmission: "admissible", wantRequirements: nil, relationless: true},
		{name: "relationless_nullif", sql: "SELECT NULLIF('SECRET_LITERAL', 'SECRET_LITERAL2')", wantExitCode: 0, wantClassification: "read_only", wantAdmission: "admissible", wantRequirements: nil, relationless: true},
		{name: "relationless_ifnull", sql: "SELECT IFNULL('SECRET_LITERAL', 'SECRET_LITERAL2')", wantExitCode: 0, wantClassification: "read_only", wantAdmission: "admissible", wantRequirements: nil, relationless: true},
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

					// Relationless literal-only proves nothing is read.
					if probe.relationless {
						if len(result.Relations) != 0 {
							t.Errorf("relationless relations: got %d (%v), want 0", len(result.Relations), result.Relations)
						}
						if len(result.ReferencedColumns) != 0 {
							t.Errorf("relationless referenced_columns: got %d (%v), want 0", len(result.ReferencedColumns), result.ReferencedColumns)
						}
						if len(result.Unresolved) != 0 {
							t.Errorf("relationless unresolved: got %d (%v), want 0", len(result.Unresolved), result.Unresolved)
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

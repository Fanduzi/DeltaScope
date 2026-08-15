//go:build integration

// Package cli verifies online Query Access through a built deltascope binary
// against MySQL 8.4 and TiDB 8.5 containers.
// input: built CLI invocations pointed at MySQL 8.4 and TiDB 8.5 containers
// output: real-route admitted and fail-closed CLI result, exit-code, and no-leak evidence
// pos: Docker-backed CLI real-route Query Access smoke coverage
// note: if this file changes, update this header and module README.md.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	deltascope "github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestQueryAccessOnline_BuiltBinaryTransportSmoke(t *testing.T) {
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
		name, port, dialect, sql, requirement, reason string
		password                                      bool
		wantExit                                      int
		wantClassification                            deltascope.QueryAccessReadClassification
		wantAdmission                                 deltascope.QueryAccessAdmission
	}{
		{
			name: "mysql84_admissible", port: "3840", dialect: "mysql",
			sql: "SELECT COUNT(1) FROM app.builtin_semantic_facts", password: true,
			requirement: "app.builtin_semantic_facts=read_table",
			wantExit:    exitQueryAccessAdmissible, wantClassification: deltascope.QueryAccessReadOnly, wantAdmission: deltascope.QueryAccessAdmissible,
		},
		{
			name: "mysql84_unknown_function", port: "3840", dialect: "mysql",
			sql: "SELECT app_specific_rollup(amount) FROM app.builtin_semantic_facts", password: true,
			reason:   "unknown_function_effect",
			wantExit: exitQueryAccessIndeterminate, wantClassification: deltascope.QueryAccessIndeterminate, wantAdmission: deltascope.QueryAccessIndeterminateAdmission,
		},
		{
			name: "tidb85_admissible", port: "4850", dialect: "tidb",
			sql:         "SELECT COUNT(1) FROM app.builtin_semantic_facts",
			requirement: "app.builtin_semantic_facts=read_table",
			wantExit:    exitQueryAccessAdmissible, wantClassification: deltascope.QueryAccessReadOnly, wantAdmission: deltascope.QueryAccessAdmissible,
		},
		{
			name: "tidb85_unknown_function", port: "4850", dialect: "tidb",
			sql:      "SELECT app_specific_rollup(amount) FROM app.builtin_semantic_facts",
			reason:   "unknown_function_effect",
			wantExit: exitQueryAccessIndeterminate, wantClassification: deltascope.QueryAccessIndeterminate, wantAdmission: deltascope.QueryAccessIndeterminateAdmission,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			marker := "CLI_BUILT_BINARY_" + strings.ToUpper(tc.name) + "_MARKER"
			args := []string{
				"query-access", "analyze",
				"--sql", tc.sql + " /* " + marker + " */",
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
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != tc.wantExit {
					t.Fatalf("built CLI: %v; stdout=%s stderr=%s", err, stdout.String(), stderr.String())
				}
			} else if tc.wantExit != exitQueryAccessAdmissible {
				t.Fatalf("built CLI exit code: got 0, want %d; stdout=%s stderr=%s", tc.wantExit, stdout.String(), stderr.String())
			}

			var result deltascope.QueryAccessResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("decode built CLI output: %v; stdout=%s", err, stdout.String())
			}
			if result.ReadClassification != tc.wantClassification || result.Admission != tc.wantAdmission {
				t.Fatalf("result: classification=%s admission=%s, want %s/%s", result.ReadClassification, result.Admission, tc.wantClassification, tc.wantAdmission)
			}
			if tc.requirement != "" {
				if len(result.Requirements) != 1 || result.Requirements[0].Object+"="+result.Requirements[0].Privilege != tc.requirement {
					t.Fatalf("requirements=%+v, want %s", result.Requirements, tc.requirement)
				}
			}
			if tc.reason != "" {
				found := false
				for _, reason := range result.ReasonCodes {
					found = string(reason) == tc.reason
					if found {
						break
					}
				}
				if !found {
					t.Fatalf("reasons=%v, want %q", result.ReasonCodes, tc.reason)
				}
			}
			if strings.Contains(stdout.String()+stderr.String(), marker) {
				t.Fatalf("built CLI leaked SQL marker: stdout=%s stderr=%s", stdout.String(), stderr.String())
			}
		})
	}
}

func TestQueryAccessOnline_DefaultOffline(t *testing.T) {
	const marker = "CLI_DEFAULT_OFFLINE_MARKER"
	var stdout, stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{
		"query-access", "analyze",
		"--sql", "SELECT COUNT(1) /* " + marker + " */ FROM app.builtin_semantic_facts",
		"--dialect", "mysql",
	}, &bytes.Buffer{}, &stdout, &stderr)
	if exitCode != exitQueryAccessIndeterminate {
		t.Fatalf("offline exit code: got %d, want %d; stdout=%s stderr=%s", exitCode, exitQueryAccessIndeterminate, stdout.String(), stderr.String())
	}

	var result deltascope.QueryAccessResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode offline CLI output: %v; stdout=%s", err, stdout.String())
	}
	if result.ReadClassification != deltascope.QueryAccessIndeterminate || result.Admission != deltascope.QueryAccessIndeterminateAdmission {
		t.Fatalf("offline result: classification=%s admission=%s, want indeterminate/indeterminate", result.ReadClassification, result.Admission)
	}
	if strings.Contains(stdout.String()+stderr.String(), marker) {
		t.Fatalf("offline CLI leaked SQL marker: stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

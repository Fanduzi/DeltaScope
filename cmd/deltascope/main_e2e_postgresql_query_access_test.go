//go:build e2e && postgresql

// Package main verifies Docker-backed CLI PostgreSQL query-access behavior.
// input: the PG17 Docker fixture and CLI query-access arguments
// output: end-to-end proof of the PostgreSQL COUNT(1) online surface contract
// pos: slower external verification kept outside the default go test loop
// note: if this file changes, update this header and module README.md.
package main

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	deltascope "github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestCLIQueryAccess_PG17_CountIntegerOne(t *testing.T) {
	result, _, _ := runCLIQueryAccessPG17(t, "SELECT COUNT(1) FROM app.orders", true)
	assertCLIQueryAccessPG17Positive(t, result)
}

func TestCLIQueryAccess_PG17_ExcludedShapes(t *testing.T) {
	queries := []string{
		"SELECT COUNT(NULL) FROM app.orders",
		"SELECT COUNT(2) FROM app.orders",
		"SELECT COUNT(1::integer) FROM app.orders",
		"SELECT COUNT($1) FROM app.orders",
		"SELECT COUNT(1) FILTER (WHERE true) FROM app.orders",
		"SELECT COUNT(1) OVER () FROM app.orders",
		"SELECT COUNT(1) FROM app.orders WHERE true",
		"SELECT COUNT(1) FROM app.orders JOIN app.users ON true",
		"SELECT COUNT(1) FROM app.user_summary",
		"WITH source AS (SELECT id FROM app.orders) SELECT COUNT(1) FROM source",
		"SELECT COUNT(1) FROM (SELECT id FROM app.orders) AS source",
		"SELECT COUNT(1) FROM orders",
		"SELECT COUNT(1), * FROM app.orders",
		"SELECT COUNT(1) FROM app.missing_relation",
	}

	for index, sqlText := range queries {
		sqlText := sqlText + " /* CLI_PG17_EXCLUDED_" + string(rune('A'+index)) + " */"
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			result, stdout, stderr := runCLIQueryAccessPG17(t, sqlText, true, 2)
			assertCLIQueryAccessPG17Indeterminate(t, result)
			assertCLIQueryAccessPG17NoLeak(t, stdout, stderr, sqlText)
		})
	}
}

func TestCLIQueryAccess_PG17_DefaultOffline(t *testing.T) {
	marker := "CLI_PG17_OFFLINE_COUNT1_MARKER"
	result, stdout, stderr := runCLIQueryAccessPG17(t, "SELECT COUNT(1) /* "+marker+" */ FROM app.orders", false)
	assertCLIQueryAccessPG17Indeterminate(t, result)
	assertCLIQueryAccessPG17NoLeak(t, stdout, stderr, marker)
}

func TestCLIQueryAccess_PG17_NoLeak(t *testing.T) {
	marker := "CLI_PG17_NO_LEAK_MARKER"
	password := "root"
	t.Setenv("DELTASCOPE_PG17_E2E_PASSWORD", password)
	result, stdout, stderr := runCLIQueryAccessPG17(t, "SELECT COUNT(1) /* "+marker+" */ FROM app.orders", true)
	assertCLIQueryAccessPG17Positive(t, result)
	assertCLIQueryAccessPG17NoLeak(t, stdout, stderr, marker, password,
		"root",
		"secret_should_not_leak", "sensitive comment text", "pg_catalog", "database_oid",
		"role_oid", "backend_pid", "session_binding", "catalog_sql", "raw_sql", "dsn",
		"candidate", "manifest", "credential", "password")
}

func TestCLIQueryAccess_PG17_ConnectionFailureNoLeak(t *testing.T) {
	marker := "CLI_PG17_REAL_CONNECTION_FAILURE_MARKER"
	password := "CLI_PG17_REAL_CONNECTION_FAILURE_PASSWORD"
	t.Setenv("DELTASCOPE_PG17_E2E_PASSWORD", password)
	port := unreachablePG17Port(t)
	stdout, stderr, _ := runCLIQueryAccessPG17ProcessAtPort(t, "SELECT COUNT(1) /* "+marker+" */ FROM app.orders", true, port, 3)
	if len(stdout) > 2048 || len(stderr) > 2048 {
		t.Fatalf("CLI failure output exceeded bound: stdout=%d stderr=%d", len(stdout), len(stderr))
	}
	assertCLIQueryAccessPG17NoLeak(t, stdout, stderr, marker, password, "127.0.0.1", "root", "app")
}

func runCLIQueryAccessPG17(t *testing.T, sqlText string, online bool, expectedExitCodes ...int) (deltascope.QueryAccessResult, string, string) {
	t.Helper()
	stdoutText, stderrText, _ := runCLIQueryAccessPG17Process(t, sqlText, online, expectedExitCodes...)
	var result deltascope.QueryAccessResult
	if err := json.Unmarshal([]byte(stdoutText), &result); err != nil {
		t.Fatalf("decode query-access result: %v; stdout=%s stderr=%s", err, stdoutText, stderrText)
	}
	return result, stdoutText, stderrText
}

func runCLIQueryAccessPG17Process(t *testing.T, sqlText string, online bool, expectedExitCodes ...int) (string, string, int) {
	return runCLIQueryAccessPG17ProcessAtPort(t, sqlText, online, 5500, expectedExitCodes...)
}

func runCLIQueryAccessPG17ProcessAtPort(t *testing.T, sqlText string, online bool, port int, expectedExitCodes ...int) (string, string, int) {
	t.Helper()
	binaryPath := buildCLIQueryAccessPG17Binary(t)
	args := []string{"query-access", "analyze", "--sql", sqlText, "--dialect", "postgresql"}
	if online {
		if os.Getenv("DELTASCOPE_PG17_E2E_PASSWORD") == "" {
			t.Setenv("DELTASCOPE_PG17_E2E_PASSWORD", "root")
		}
		args = append(args, "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--user", "root", "--password-env", "DELTASCOPE_PG17_E2E_PASSWORD", "--schema", "app")
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(t.Context(), binaryPath, args...)
	cmd.Env = os.Environ()
	cmd.Stdin = strings.NewReader("")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if processErr, ok := err.(*exec.ExitError); ok {
			exitCode = processErr.ExitCode()
		} else {
			t.Fatalf("run CLI binary: %v", err)
		}
	}
	wantExitCode := 2
	if online {
		wantExitCode = 0
	}
	if len(expectedExitCodes) > 0 {
		wantExitCode = expectedExitCodes[0]
	}
	if exitCode != wantExitCode {
		t.Fatalf("exit code: got %d, want %d; stdout=%s stderr=%s", exitCode, wantExitCode, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String(), exitCode
}

func buildCLIQueryAccessPG17Binary(t *testing.T) string {
	t.Helper()
	moduleRoot := findCLIQueryAccessPG17ModuleRoot(t)
	binaryPath := filepath.Join(t.TempDir(), "deltascope")
	cmd := exec.Command("go", "build", "-tags", "postgresql", "-o", binaryPath, "./cmd/deltascope")
	cmd.Dir = moduleRoot
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build CLI binary: %v\n%s", err, string(output))
	}
	return binaryPath
}

func findCLIQueryAccessPG17ModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate module root")
		}
		dir = parent
	}
}

func unreachablePG17Port(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate unreachable port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release unreachable port: %v", err)
	}
	return port
}

func assertCLIQueryAccessPG17Positive(t *testing.T, result deltascope.QueryAccessResult) {
	t.Helper()
	if result.ReadClassification != deltascope.QueryAccessReadOnly || result.Admission != deltascope.QueryAccessAdmissible {
		t.Fatalf("expected read_only/admissible, got classification=%q admission=%q reasons=%v", result.ReadClassification, result.Admission, result.ReasonCodes)
	}
	if len(result.Relations) != 1 || result.Relations[0].Schema != "app" || result.Relations[0].Name != "orders" || result.Relations[0].Kind != "table" {
		t.Fatalf("expected one app.orders table relation, got %+v", result.Relations)
	}
	if len(result.ReferencedColumns) != 0 {
		t.Fatalf("COUNT(1) referenced columns: %+v", result.ReferencedColumns)
	}
	if len(result.Requirements) != 1 || result.Requirements[0].Object != "app.orders" || result.Requirements[0].Privilege != "read_table" {
		t.Fatalf("COUNT(1) requirements: %+v", result.Requirements)
	}
}

func assertCLIQueryAccessPG17Indeterminate(t *testing.T, result deltascope.QueryAccessResult) {
	t.Helper()
	if result.ReadClassification != deltascope.QueryAccessIndeterminate || result.Admission != deltascope.QueryAccessIndeterminateAdmission {
		t.Fatalf("expected indeterminate/indeterminate, got classification=%q admission=%q requirements=%+v reasons=%v", result.ReadClassification, result.Admission, result.Requirements, result.ReasonCodes)
	}
}

func assertCLIQueryAccessPG17NoLeak(t *testing.T, stdout, stderr string, markers ...string) {
	t.Helper()
	if len(stdout) > 2048 || len(stderr) > 2048 {
		t.Fatalf("CLI output exceeded bound: stdout=%d stderr=%d", len(stdout), len(stderr))
	}
	data := stdout + "\n" + stderr
	for _, marker := range markers {
		if strings.Contains(strings.ToLower(data), strings.ToLower(marker)) {
			t.Errorf("CLI output leaked %q: %s", marker, data)
		}
	}
}

// Package cli verifies CLI query access command behavior.
// input: synthetic CLI invocations covering audit-only flag boundaries in both command positions, fixed JSON output, admission exits, explicit-empty/non-EOF stdin, file, unified online-session routing, and the PostgreSQL PG17 version boundary
// output: coverage for query access JSON output, exit codes, input-source validation, bounded connection/authentication/version messages, and close ownership
// pos: CLI adapter behavior and unified online migration coverage for query access
// note: if this file changes, update this header and module README.md.
package cli

import (
	"bytes"
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/online"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestQueryAccessOnlineMapsUnifiedConstructorFailures(t *testing.T) {
	for _, constructorError := range []error{
		deltascope.ErrOnlineQueryAccessSessionUnavailable,
		deltascope.ErrOnlineQueryAccessCapabilityUnsupported,
	} {
		t.Run(constructorError.Error(), func(t *testing.T) {
			previousOpener := openOnlineSession
			previousConstructor := newOnlineQueryAccessSessionFromConn
			t.Cleanup(func() {
				openOnlineSession = previousOpener
				newOnlineQueryAccessSessionFromConn = previousConstructor
			})
			t.Setenv("CLI_UNIFIED_ENTRY_PASSWORD", "secret")

			var closeCalls, constructorCalls int
			openOnlineSession = func(context.Context, online.SessionConfig) (*online.Session, error) {
				return &online.Session{
					Conn: &sql.Conn{},
					Close: func() error {
						closeCalls++
						return nil
					},
				}, nil
			}
			newOnlineQueryAccessSessionFromConn = func(context.Context, *sql.Conn) (*deltascope.OnlineQueryAccessSession, error) {
				constructorCalls++
				return nil, constructorError
			}

			var stdout, stderr bytes.Buffer
			exitCode := Execute(t.Context(), []string{
				"query-access", "analyze",
				"--sql", "SELECT 1",
				"--dialect", "mysql",
				"--host", "recording.invalid",
				"--user", "cli_user",
				"--password-env", "CLI_UNIFIED_ENTRY_PASSWORD",
			}, &bytes.Buffer{}, &stdout, &stderr)

			if exitCode != exitQueryAccessUsageError {
				t.Fatalf("exit code = %d, want %d", exitCode, exitQueryAccessUsageError)
			}
			if stdout.Len() != 0 || stderr.String() != "connection failed\n" {
				t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			if constructorCalls != 1 || closeCalls != 1 {
				t.Fatalf("calls = constructor:%d close:%d, want 1 each", constructorCalls, closeCalls)
			}
		})
	}
}

func TestQueryAccessOnlinePostgreSQL16ReportsVersionRequirement(t *testing.T) {
	t.Setenv("PG16_PASSWORD", "pg16-secret")
	previousOpener := openOnlineSession
	openOnlineSession = func(context.Context, online.SessionConfig) (*online.Session, error) {
		return nil, online.ErrPostgreSQLQueryAccessVersionUnsupported
	}
	t.Cleanup(func() { openOnlineSession = previousOpener })

	var stdout, stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{
		"query-access", "analyze",
		"--sql", "SELECT COUNT(1) /* PG16_SQL_MARKER */ FROM public.users",
		"--dialect", "postgresql",
		"--host", "pg16.invalid",
		"--port", "5432",
		"--user", "pg16_user",
		"--password-env", "PG16_PASSWORD",
		"--database", "pg16_database",
	}, &bytes.Buffer{}, &stdout, &stderr)

	if exitCode != exitQueryAccessUsageError {
		t.Fatalf("exit code = %d, want %d", exitCode, exitQueryAccessUsageError)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no analysis output, got %q", stdout.String())
	}
	if stderr.String() != "online PostgreSQL Query Access requires PostgreSQL 17\n" {
		t.Fatalf("unexpected bounded error: %q", stderr.String())
	}
	for _, forbidden := range []string{"16.3", "PG16_SQL_MARKER", "pg16.invalid", "5432", "pg16_user", "PG16_PASSWORD", "pg16_database"} {
		if strings.Contains(strings.ToLower(stderr.String()), strings.ToLower(forbidden)) {
			t.Fatalf("CLI output leaked %q: %q", forbidden, stderr.String())
		}
	}
}

func TestQueryAccessOnlineAuthenticationFailureRemainsBounded(t *testing.T) {
	t.Setenv("CLI_AUTH_PASSWORD", "cli-auth-secret")
	previousOpener := openOnlineSession
	openOnlineSession = func(context.Context, online.SessionConfig) (*online.Session, error) {
		return nil, online.ErrAuthenticationFailed
	}
	t.Cleanup(func() { openOnlineSession = previousOpener })

	var stdout, stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{
		"query-access", "analyze",
		"--sql", "SELECT 1 /* CLI_AUTH_MARKER */",
		"--dialect", "postgresql",
		"--host", "auth.invalid",
		"--user", "auth_user",
		"--password-env", "CLI_AUTH_PASSWORD",
	}, &bytes.Buffer{}, &stdout, &stderr)

	if exitCode != exitQueryAccessUsageError || stdout.Len() != 0 {
		t.Fatalf("unexpected authentication failure result: exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	if stderr.String() != "authentication failed\n" {
		t.Fatalf("unexpected bounded authentication error: %q", stderr.String())
	}
	for _, forbidden := range []string{"CLI_AUTH_MARKER", "auth.invalid", "auth_user", "CLI_AUTH_PASSWORD", "cli-auth-secret"} {
		if strings.Contains(strings.ToLower(stderr.String()), strings.ToLower(forbidden)) {
			t.Fatalf("CLI output leaked %q: %q", forbidden, stderr.String())
		}
	}
}

func TestQueryAccessOnlineUsesUnifiedEntryWithEmptyRequestDialect(t *testing.T) {
	previousOpener := openOnlineSession
	previousConstructor := newOnlineQueryAccessSessionFromConn
	previousAnalyzer := analyzeOnlineQueryAccessWithSession
	t.Cleanup(func() {
		openOnlineSession = previousOpener
		newOnlineQueryAccessSessionFromConn = previousConstructor
		analyzeOnlineQueryAccessWithSession = previousAnalyzer
	})

	t.Setenv("CLI_UNIFIED_ENTRY_PASSWORD", "secret")

	var constructorCalls, analysisCalls int
	openOnlineSession = func(context.Context, online.SessionConfig) (*online.Session, error) {
		return &online.Session{Conn: &sql.Conn{}, Close: func() error { return nil }}, nil
	}
	newOnlineQueryAccessSessionFromConn = func(context.Context, *sql.Conn) (*deltascope.OnlineQueryAccessSession, error) {
		constructorCalls++
		return nil, nil
	}
	analyzeOnlineQueryAccessWithSession = func(_ context.Context, session *deltascope.OnlineQueryAccessSession, request deltascope.QueryAccessRequest) (*deltascope.QueryAccessResult, error) {
		analysisCalls++
		if session != nil {
			return nil, errors.New("unexpected opaque session")
		}
		if request.Dialect != "" {
			return nil, errors.New("online request dialect must remain empty")
		}
		return &deltascope.QueryAccessResult{
			Dialect:            "mysql",
			Mode:               deltascope.QueryAccessModeStrict,
			ReadClassification: deltascope.QueryAccessReadOnly,
			Admission:          deltascope.QueryAccessAdmissible,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{
		"query-access", "analyze",
		"--sql", "SELECT 1",
		"--dialect", "mysql",
		"--host", "recording.invalid",
		"--user", "cli_user",
		"--password-env", "CLI_UNIFIED_ENTRY_PASSWORD",
	}, &bytes.Buffer{}, &stdout, &stderr)

	if exitCode != exitQueryAccessAdmissible {
		t.Fatalf("exit code = %d, want %d; stderr=%s", exitCode, exitQueryAccessAdmissible, stderr.String())
	}
	if constructorCalls != 1 || analysisCalls != 1 {
		t.Fatalf("unified calls = constructor:%d analysis:%d, want 1 each", constructorCalls, analysisCalls)
	}
}

func TestQueryAccessAnalyzeMySQLSelect(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{"query-access", "analyze", "--sql", "SELECT id, name FROM users WHERE id = 1", "--dialect", "mysql"}, &bytes.Buffer{}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", exitCode, stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if result["dialect"] != "mysql" {
		t.Errorf("expected mysql dialect, got %#v", result["dialect"])
	}
	if result["mode"] != "strict" {
		t.Errorf("expected strict mode, got %#v", result["mode"])
	}
	if result["read_classification"] != "read_only" {
		t.Errorf("expected read_only classification, got %#v", result["read_classification"])
	}
	if result["admission"] != "admissible" {
		t.Errorf("expected admissible admission, got %#v", result["admission"])
	}
}

func TestQueryAccessAnalyzeMySQLDelete(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{"query-access", "analyze", "--sql", "DELETE FROM users WHERE id = 1", "--dialect", "mysql"}, &bytes.Buffer{}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1 (rejected), got %d: %s", exitCode, stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if result["admission"] != "rejected" {
		t.Errorf("expected rejected admission, got %#v", result["admission"])
	}
}

func TestQueryAccessAnalyzeNoAuditFieldLeakage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{"query-access", "analyze", "--sql", "SELECT id FROM users", "--dialect", "mysql"}, &bytes.Buffer{}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", exitCode, stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}

	forbiddenFields := []string{"verdict", "summary", "statements", "global_findings", "findings", "level", "rule_id", "context"}
	for _, field := range forbiddenFields {
		if _, ok := result[field]; ok {
			t.Errorf("forbidden field %q found in query access CLI output", field)
		}
	}
}

func TestQueryAccessAnalyzeProjectionOnlyMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{"query-access", "analyze", "--sql", "SELECT id FROM users WHERE name = 'test'", "--dialect", "mysql", "--mode", "projection_only"}, &bytes.Buffer{}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", exitCode, stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if result["mode"] != "projection_only" {
		t.Errorf("expected projection_only mode, got %#v", result["mode"])
	}
}

func TestQueryAccessAnalyzeHelpNoProfile(t *testing.T) {
	var stdout bytes.Buffer
	exitCode := Execute(t.Context(), []string{"query-access", "analyze", "--help"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for --help, got %d", exitCode)
	}
	if bytes.Contains(stdout.Bytes(), []byte("--profile")) {
		t.Fatalf("--profile flag should not appear in help output:\n%s", stdout.String())
	}
	for _, forbidden := range []string{"--format", "--fail-on"} {
		if bytes.Contains(stdout.Bytes(), []byte(forbidden)) {
			t.Fatalf("%s flag should not appear in help output:\n%s", forbidden, stdout.String())
		}
	}
}

func TestQueryAccessAnalyzeHelpShowsConnectionFlags(t *testing.T) {
	var stdout bytes.Buffer
	exitCode := Execute(t.Context(), []string{"query-access", "analyze", "--help"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for --help, got %d", exitCode)
	}
	requiredFlags := []string{"--host", "--port", "--user", "--password-env", "--password-file", "--ask-password", "--schema", "--database", "--socket", "--metadata-connect-timeout"}
	for _, flag := range requiredFlags {
		if !bytes.Contains(stdout.Bytes(), []byte(flag)) {
			t.Errorf("expected %q in help output", flag)
		}
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Explicit empty or whitespace-only --sql fails with exit 3 without reading stdin.")) {
		t.Fatalf("expected explicit empty SQL input contract in help output:\n%s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("online PostgreSQL Query Access requires PostgreSQL 17")) {
		t.Fatalf("expected PostgreSQL online version requirement in help output:\n%s", stdout.String())
	}
}

func TestQueryAccessAnalyzeRejectsAuditOnlyFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		flag string
	}{
		{name: "format", args: []string{"query-access", "analyze", "--sql", "SELECT 1", "--format", "json"}, flag: "--format"},
		{name: "fail-on", args: []string{"query-access", "analyze", "--sql", "SELECT 1", "--fail-on", "warning"}, flag: "--fail-on"},
		{name: "format after command group", args: []string{"query-access", "--format", "json", "analyze", "--sql", "SELECT 1"}, flag: "--format"},
		{name: "fail-on after command group", args: []string{"query-access", "--fail-on", "warning", "analyze", "--sql", "SELECT 1"}, flag: "--fail-on"},
		{name: "format before command", args: []string{"--format", "json", "query-access", "analyze", "--sql", "SELECT 1"}, flag: "--format"},
		{name: "fail-on before command", args: []string{"--fail-on", "warning", "query-access", "analyze", "--sql", "SELECT 1"}, flag: "--fail-on"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exitCode := Execute(t.Context(), tc.args, &bytes.Buffer{}, &stdout, &stderr)

			if exitCode != exitQueryAccessUsageError {
				t.Fatalf("expected exit code %d, got %d: %s", exitQueryAccessUsageError, exitCode, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no analysis output, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "unknown flag: "+tc.flag) {
				t.Fatalf("expected bounded unknown-flag error, got %q", stderr.String())
			}
		})
	}
}

func TestQueryAccessAnalyzeInvalidMode(t *testing.T) {
	var stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{"query-access", "analyze", "--sql", "SELECT 1", "--dialect", "mysql", "--mode", "invalid"}, &bytes.Buffer{}, &bytes.Buffer{}, &stderr)

	if exitCode != exitQueryAccessUsageError {
		t.Fatalf("expected exit code %d, got %d: %s", exitQueryAccessUsageError, exitCode, stderr.String())
	}
}

func TestQueryAccessAnalyzeEmptyStdin(t *testing.T) {
	var stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{"query-access", "analyze"}, strings.NewReader("  \n"), &bytes.Buffer{}, &stderr)

	if exitCode != exitQueryAccessUsageError {
		t.Fatalf("expected exit code %d, got %d: %s", exitQueryAccessUsageError, exitCode, stderr.String())
	}
	if stderr.String() != "SQL input must not be empty\n" {
		t.Fatalf("expected bounded empty SQL error, got %q", stderr.String())
	}
}

func TestQueryAccessAnalyzeRejectsExplicitEmptySQLWithoutReadingStdin(t *testing.T) {
	for _, args := range [][]string{
		{"query-access", "analyze", "--sql", ""},
		{"query-access", "analyze", "--sql", "   "},
	} {
		t.Run(args[len(args)-1], func(t *testing.T) {
			var stderr bytes.Buffer
			exitCode := Execute(t.Context(), args, unexpectedStdinReader{t: t}, &bytes.Buffer{}, &stderr)

			if exitCode != exitQueryAccessUsageError {
				t.Fatalf("expected exit code %d, got %d: %s", exitQueryAccessUsageError, exitCode, stderr.String())
			}
			if stderr.String() != "SQL input must not be empty\n" {
				t.Fatalf("expected bounded empty SQL error, got %q", stderr.String())
			}
		})
	}
}

func TestQueryAccessAnalyzeMissingFileErrorIsBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.sql")
	var stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{"query-access", "analyze", "--file", path}, strings.NewReader("SELECT 1"), &bytes.Buffer{}, &stderr)

	if exitCode != exitQueryAccessUsageError {
		t.Fatalf("expected exit code %d, got %d: %s", exitQueryAccessUsageError, exitCode, stderr.String())
	}
	if stderr.String() != "cannot read SQL file\n" {
		t.Fatalf("expected bounded missing file error, got %q", stderr.String())
	}
	if bytes.Contains(stderr.Bytes(), []byte(path)) {
		t.Fatalf("missing file path leaked in stderr: %q", stderr.String())
	}
}

func TestQueryAccessAnalyzeSupportsStdinAndFileInput(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "query.sql")
	if err := os.WriteFile(filePath, []byte("SELECT 1"), 0o600); err != nil {
		t.Fatalf("write SQL file: %v", err)
	}

	for _, tc := range []struct {
		name  string
		args  []string
		stdin string
	}{
		{name: "stdin", args: []string{"query-access", "analyze"}, stdin: "SELECT 1"},
		{name: "file", args: []string{"query-access", "analyze", "--file", filePath}, stdin: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exitCode := Execute(t.Context(), tc.args, strings.NewReader(tc.stdin), &stdout, &stderr)

			if exitCode != exitQueryAccessAdmissible {
				t.Fatalf("expected exit code %d, got %d: %s", exitQueryAccessAdmissible, exitCode, stderr.String())
			}
			var result map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("decode output: %v", err)
			}
			if result["admission"] != string(deltascope.QueryAccessAdmissible) {
				t.Fatalf("expected admissible admission, got %#v", result["admission"])
			}
		})
	}
}

func TestQueryAccessAnalyzeJSONFieldNames(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{"query-access", "analyze", "--sql", "SELECT u.id, u.name FROM users u JOIN orders o ON u.id = o.user_id", "--dialect", "mysql"}, &bytes.Buffer{}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", exitCode, stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}

	requiredFields := []string{"dialect", "mode", "read_classification", "admission"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("expected field %q in JSON output", field)
		}
	}
}

func TestExitCodeForQueryAccess_UnknownAdmissionFailsClosed(t *testing.T) {
	t.Parallel()

	result := &deltascope.QueryAccessResult{
		Admission: deltascope.QueryAccessAdmission("bogus"),
	}
	code := exitCodeForQueryAccess(result)
	if code != exitQueryAccessIndeterminate {
		t.Errorf("unknown admission: got exit code %d, want %d (indeterminate)", code, exitQueryAccessIndeterminate)
	}
}

func TestQueryAccessAnalyzeRejectsRemovedPasswordFlag(t *testing.T) {
	var stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{"query-access", "analyze", "--sql", "SELECT 1", "--password", "secret"}, &bytes.Buffer{}, &bytes.Buffer{}, &stderr)

	if exitCode != 3 {
		t.Fatalf("expected usage error exit code 3 for removed --password flag, got %d: %s", exitCode, stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("unknown flag")) {
		t.Fatalf("expected unknown flag error, got %q", stderr.String())
	}
}

func TestQueryAccessOnlineBuildsTLSSessionConfig(t *testing.T) {
	t.Parallel()

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte("-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJAL...\n-----END CERTIFICATE-----"))

	connection := auditConnectionOptions{
		Host:     "db.example.com",
		Port:     5432,
		User:     "admin",
		Password: "secret",
		Database: "mydb",
		Schema:   "app",
		TLSMode:  "enabled",
		CACert:   pool,
	}

	cfg := buildOnlineSessionConfig(connection, spec.DialectPostgreSQL)

	if cfg.TLSMode != "enabled" {
		t.Errorf("TLSMode = %q, want %q", cfg.TLSMode, "enabled")
	}
	if cfg.CACert == nil {
		t.Fatal("CACert should not be nil when connection provides a CA pool")
	}
	if cfg.CACert != pool {
		t.Error("CACert should be the same pool passed via connection")
	}
	if cfg.Database != "mydb" {
		t.Errorf("Database = %q, want %q", cfg.Database, "mydb")
	}
	if cfg.Host != "db.example.com" {
		t.Errorf("Host = %q, want %q", cfg.Host, "db.example.com")
	}
}

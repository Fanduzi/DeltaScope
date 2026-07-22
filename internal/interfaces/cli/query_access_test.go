// Package cli verifies CLI query access command behavior.
// input: synthetic CLI invocations for query access analysis
// output: coverage for query access command JSON output, exit codes, and input validation
// pos: CLI adapter test coverage for query access surface
// note: if this file changes, update this header and module README.md.
package cli

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

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
}

func TestQueryAccessAnalyzeInvalidMode(t *testing.T) {
	var stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{"query-access", "analyze", "--sql", "SELECT 1", "--dialect", "mysql", "--mode", "invalid"}, &bytes.Buffer{}, &bytes.Buffer{}, &stderr)

	if exitCode != exitQueryAccessUsageError {
		t.Fatalf("expected exit code %d, got %d: %s", exitQueryAccessUsageError, exitCode, stderr.String())
	}
}

func TestQueryAccessAnalyzeEmptySQL(t *testing.T) {
	var stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{"query-access", "analyze", "--sql", "  "}, &bytes.Buffer{}, &bytes.Buffer{}, &stderr)

	if exitCode != exitQueryAccessUsageError {
		t.Fatalf("expected exit code %d, got %d: %s", exitQueryAccessUsageError, exitCode, stderr.String())
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

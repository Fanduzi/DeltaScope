// Package cli verifies CLI query access command behavior.
// input: synthetic CLI invocations for query access analysis
// output: coverage for query access command JSON output, exit codes, and input validation
// pos: CLI adapter test coverage for query access surface
// note: if this file changes, update this header and module README.md.
package cli

import (
	"bytes"
	"encoding/json"
	"testing"
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

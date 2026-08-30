//go:build postgresql

// Package cli verifies CLI unsupported-statement result contracts.
// input: Execute audit commands containing structured unsupported PostgreSQL statements
// output: JSON and Markdown review floors with exit 1 and audited-only statement counts
// pos: CLI adapter regression coverage for the unsupported-statement verdict floor
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCLIUnsupportedStatementJSONAppliesReviewFloor(t *testing.T) {
	t.Parallel()

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "SELECT 1", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)
	if code != exitAudit {
		t.Fatalf("expected exit %d for unsupported SQL, got %d", exitAudit, code)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &payload); err != nil {
		t.Fatalf("decode JSON output: %v\noutput was: %s", err, stdout.String())
	}
	if payload["verdict"] != "review" {
		t.Fatalf("expected unsupported review floor, got %#v", payload["verdict"])
	}
	summary, _ := payload["summary"].(map[string]any)
	if summary["statements"] != float64(0) {
		t.Fatalf("expected zero audited statements, got %s", stdout.String())
	}
	if _, ok := payload["statements"]; ok {
		t.Fatalf("expected omitted audited statements, got %s", stdout.String())
	}
	unsupported, _ := payload["unsupported"].([]any)
	if len(unsupported) != 1 {
		t.Fatalf("expected one unsupported detail, got %s", stdout.String())
	}
	item, _ := unsupported[0].(map[string]any)
	if item["feature"] != "select" {
		t.Fatalf("expected unsupported feature select, got %#v", item)
	}
	diagnostics, _ := payload["diagnostics"].([]any)
	if len(diagnostics) != 1 {
		t.Fatalf("expected one unsupported diagnostic, got %s", stdout.String())
	}
	diagnostic, _ := diagnostics[0].(map[string]any)
	if diagnostic["classification"] != "unsupported_statement" || diagnostic["audited"] != false {
		t.Fatalf("expected unaudited unsupported_statement diagnostic, got %#v", diagnostic)
	}
	if strings.Contains(stderr.String(), "SELECT 1") {
		t.Fatalf("CLI stderr leaked SQL text: %q", stderr.String())
	}
}

func TestCLIUnsupportedStatementMarkdownAppliesReviewFloor(t *testing.T) {
	t.Parallel()

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "SELECT 1", "--dialect", "postgresql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)
	if code != exitAudit {
		t.Fatalf("expected exit %d for unsupported SQL, got %d", exitAudit, code)
	}
	if !strings.Contains(stdout.String(), "Verdict: `review`") {
		t.Fatalf("expected unsupported review floor, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "## Unsupported Statements") || !strings.Contains(stdout.String(), "select") {
		t.Fatalf("expected markdown unsupported details, got %s", stdout.String())
	}
}

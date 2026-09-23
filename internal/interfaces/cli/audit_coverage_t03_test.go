// Package cli verifies the T03 incomplete-coverage CLI contract.
// input: recognized-but-unsupported and parser-failure audit invocations
// output: exit-code and JSON coverage assertions for issue #82
// pos: CLI transport contract tests for incomplete audit results
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const t03GoldenSQL = "CREATE SEQUENCE golden_seq START WITH 1; ALTER TABLE t ADD COLUMN c INT;"

// Recognized-but-unsupported statements keep the partial result, mark coverage
// incomplete, and exit 1 even when --fail-on none disables the finding gate.
func TestAuditCommandT03IncompleteCoverageExitsAudit(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", t03GoldenSQL, "--dialect", "mysql", "--format", "json", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected exit %d for incomplete coverage, got %d, stderr=%q", exitAudit, code, stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, stdout.String())
	}
	if decoded["verdict"] != "review" {
		t.Fatalf("expected verdict review, got %#v", decoded["verdict"])
	}
	coverage, ok := decoded["coverage"].(map[string]any)
	if !ok || coverage["status"] != "incomplete" {
		t.Fatalf("expected aggregate coverage incomplete, got %#v", decoded["coverage"])
	}
	stmts, ok := decoded["statements"].([]any)
	if !ok || len(stmts) != 2 {
		t.Fatalf("expected 2 retained statements, got %#v", decoded["statements"])
	}
	first, _ := stmts[0].(map[string]any)
	second, _ := stmts[1].(map[string]any)
	if c, _ := first["coverage"].(map[string]any); c["status"] != "incomplete" {
		t.Fatalf("expected statement 0 coverage incomplete, got %#v", first["coverage"])
	}
	if c, _ := second["coverage"].(map[string]any); c["status"] != "complete" {
		t.Fatalf("expected statement 1 coverage complete, got %#v", second["coverage"])
	}
	unsupported, _ := decoded["unsupported"].([]any)
	if len(unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", decoded["unsupported"])
	}
	if detail, _ := unsupported[0].(map[string]any); detail["feature"] != "create_sequence" {
		t.Fatalf("expected create_sequence feature, got %#v", detail["feature"])
	}
}

// A genuine parser failure keeps the parser-error contract and exit 2; it must
// not be reclassified as a recognized-but-unsupported statement.
func TestAuditCommandT03ParserFailureKeepsExitUser(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE TABLE golden_broken (", "--dialect", "mysql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitUser {
		t.Fatalf("expected parser-error exit %d, got %d", exitUser, code)
	}
}

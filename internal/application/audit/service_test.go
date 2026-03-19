// Package audit verifies the application audit service behavior.
// input: audit service requests with SQL, dialect, and optional config path
// output: end-to-end application audit coverage over policy loading, extraction, and rules
// pos: application audit service test coverage
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLUsesDefaultPolicy(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "delete from users",
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}

	if result.Verdict != report.VerdictReject {
		t.Fatalf("expected reject verdict, got %q", result.Verdict)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %d", len(result.Statements))
	}
	if len(result.Statements[0].Findings) == 0 {
		t.Fatal("expected statement findings from default policy")
	}
}

func TestAuditSQLAppliesConfigOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deltascope.yaml")
	content := []byte(`
rules:
  dml.where.require:
    enabled: false
    level: notice
    params:
      required: false
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "delete from users",
		Dialect:    spec.DialectMySQL,
		ConfigPath: path,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}

	if result.Verdict != report.VerdictPass {
		t.Fatalf("expected pass verdict after disabling WHERE rule, got %q", result.Verdict)
	}
}

func TestAuditSQLReturnsGroupedStatementResults(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "delete from users; update accounts set active = 0",
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}

	if len(result.Statements) != 2 {
		t.Fatalf("expected 2 statement results, got %d", len(result.Statements))
	}
	if result.Summary.Statements != 2 {
		t.Fatalf("expected summary statements=2, got %d", result.Summary.Statements)
	}
}

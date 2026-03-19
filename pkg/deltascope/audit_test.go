// Package deltascope verifies the public library audit API.
// input: inline SQL text, optional YAML policy overrides, and the public request contract
// output: regression coverage for public audit orchestration and stable result mapping
// pos: public API test coverage for pkg/deltascope
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditUsesDefaultPolicy(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "update users set name = 'delta';",
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	if result.Verdict != VerdictReject {
		t.Fatalf("expected reject verdict, got %q", result.Verdict)
	}
	if result.Summary.Blockers != 1 {
		t.Fatalf("expected 1 blocker, got %#v", result.Summary)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	if len(result.Statements[0].Findings) != 1 {
		t.Fatalf("expected 1 statement finding, got %d", len(result.Statements[0].Findings))
	}
	if result.Statements[0].Findings[0].RuleID != "dml.where.require" {
		t.Fatalf("expected where rule finding, got %q", result.Statements[0].Findings[0].RuleID)
	}
}

func TestAuditSupportsConfigOverridePath(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "policy.yaml")
	config := "rules:\n  dml.where.require:\n    enabled: false\n    level: blocker\n    params:\n      required: true\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := Audit(context.Background(), Request{
		SQL:        "update users set name = 'delta';",
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	if result.Verdict != VerdictPass {
		t.Fatalf("expected pass verdict, got %q", result.Verdict)
	}
	if result.Summary.Blockers != 0 || result.Summary.Warnings != 0 || result.Summary.Notices != 0 {
		t.Fatalf("expected empty findings after disabling rule, got %#v", result.Summary)
	}
}

func TestAuditReturnsGroupedMultiStatementResults(t *testing.T) {
	sql := "create table users (id bigint); update users set name = 'delta';"

	result, err := Audit(context.Background(), Request{
		SQL:     sql,
		Dialect: DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	if len(result.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(result.Statements))
	}
	if result.Statements[0].Kind != "ddl" {
		t.Fatalf("expected first statement kind ddl, got %q", result.Statements[0].Kind)
	}
	if result.Statements[1].Kind != "dml" {
		t.Fatalf("expected second statement kind dml, got %q", result.Statements[1].Kind)
	}
	if len(result.Statements[0].Findings) == 0 {
		t.Fatalf("expected first statement findings to be grouped")
	}
	if len(result.Statements[1].Findings) == 0 {
		t.Fatalf("expected second statement findings to be grouped")
	}
	if result.Statements[0].Findings[0].StatementIndex != 0 {
		t.Fatalf("expected first statement finding index 0, got %d", result.Statements[0].Findings[0].StatementIndex)
	}
	if result.Statements[1].Findings[0].StatementIndex != 1 {
		t.Fatalf("expected second statement finding index 1, got %d", result.Statements[1].Findings[0].StatementIndex)
	}
}

func TestAuditRejectsUnsupportedDialect(t *testing.T) {
	_, err := Audit(context.Background(), Request{
		SQL:     "select 1;",
		Dialect: Dialect("postgres"),
	})
	if err == nil {
		t.Fatal("expected unsupported dialect error")
	}
}

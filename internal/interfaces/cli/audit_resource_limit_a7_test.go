// Package cli verifies the T05-A7 ordered-state budget at the CLI seam.
// input: 1025-statement MySQL audit invocations across every fail threshold, plus a trailing parser failure
// output: exit codes and JSON assertions for the retained partial result, bounded resource-limit projection, and parser-error priority
// pos: CLI transport contract tests for the shared ordered-state admission budget (issue #84/T05-A7)
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func assertT05A7BudgetJSON(t *testing.T, decoded map[string]any) {
	t.Helper()
	if decoded["verdict"] != "review" {
		t.Fatalf("verdict = %#v, want review", decoded["verdict"])
	}
	coverage, _ := decoded["coverage"].(map[string]any)
	if coverage["status"] != "incomplete" {
		t.Fatalf("coverage = %#v, want incomplete", decoded["coverage"])
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1025 {
		t.Fatalf("statements = %d, want 1025 retained", len(statements))
	}
	for i := 0; i < 1024; i++ {
		statement, _ := statements[i].(map[string]any)
		sc, _ := statement["coverage"].(map[string]any)
		if statement["index"] != float64(i) || sc["status"] != "complete" {
			t.Fatalf("admitted statement %d = %#v, want complete", i, statement)
		}
		if statement["findings"] != nil || statement["evidence_gaps"] != nil || statement["impact"] != nil {
			t.Fatalf("admitted statement %d must not carry findings, gaps, or impact, got %#v", i, statement)
		}
	}
	if decoded["global_findings"] != nil {
		t.Fatalf("global findings = %#v, want none", decoded["global_findings"])
	}
	if summary, _ := decoded["summary"].(map[string]any); summary["blockers"] != float64(0) || summary["warnings"] != float64(0) || summary["notices"] != float64(0) {
		t.Fatalf("finding counters = %#v, want zero", decoded["summary"])
	}
	last, _ := statements[1024].(map[string]any)
	lc, _ := last["coverage"].(map[string]any)
	normalized, _ := last["normalized_sql"].(string)
	if last["index"] != float64(1024) || last["kind"] != "unknown" || strings.TrimSpace(last["raw_sql"].(string)) != "SELECT 1;" || normalized == "" {
		t.Fatalf("blocked statement identity = %#v, want index 1024 kind unknown raw SELECT 1;", last)
	}
	if lc["status"] != "incomplete" || last["findings"] != nil || last["evidence_gaps"] != nil || last["impact"] != nil {
		t.Fatalf("blocked statement = %#v, want incomplete with no findings, gaps, or impact", last)
	}
	unsupported, _ := decoded["unsupported"].([]any)
	if len(unsupported) != 1 {
		t.Fatalf("unsupported entries = %d, want 1: %#v", len(unsupported), decoded["unsupported"])
	}
	item, _ := unsupported[0].(map[string]any)
	if item["feature"] != "audit.resource_limit" || item["reason"] != "ordered-state statement budget exhausted" {
		t.Fatalf("unsupported entry = %#v, want the audit.resource_limit budget projection", item)
	}
	if item["index"] != float64(1024) || item["sql"] != last["raw_sql"] {
		t.Fatalf("unsupported identity = %#v, want index 1024 bound to the blocked statement", item)
	}
	wantMeta := map[string]any{
		"phase":    "ordered_state",
		"resource": "statements",
		"limit":    float64(1024),
		"consumed": float64(1024),
		"line":     float64(1025),
		"column":   float64(1),
	}
	if metadata, _ := item["metadata"].(map[string]any); !reflect.DeepEqual(metadata, wantMeta) {
		t.Fatalf("resource metadata = %#v, want %#v", item["metadata"], wantMeta)
	}
}

func TestT05A7OrderedStatementBudgetCLI(t *testing.T) {
	sql := strings.Repeat("SELECT 1;\n", 1025)
	for _, mode := range []string{"blocker", "warning", "notice", "none"} {
		t.Run("fail_on_"+mode, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}
			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", sql, "--dialect", "mysql", "--format", "json", "--fail-on", mode},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != exitAudit {
				t.Fatalf("expected exit code %d for the over-budget audit, got %d\nstderr=%s", exitAudit, code, stderr.String())
			}
			assertT05A7BudgetJSON(t, decodeAuditJSON(t, stdout.String()))
		})
	}

	t.Run("parser_error_priority", func(t *testing.T) {
		stdout := &strings.Builder{}
		stderr := &strings.Builder{}
		code := Execute(
			context.Background(),
			[]string{"audit", "--sql", sql + "THIS IS NOT SQL;\n", "--dialect", "mysql", "--format", "json", "--fail-on", "none"},
			strings.NewReader(""),
			stdout,
			stderr,
		)
		if code != exitUser {
			t.Fatalf("expected exit code %d for the parser failure, got %d\nstderr=%s", exitUser, code, stderr.String())
		}
		decoded := decodeAuditJSON(t, stdout.String())
		assertT05A7BudgetJSON(t, decoded)
		diagnostics, _ := decoded["diagnostics"].([]any)
		parserSeen := false
		for _, raw := range diagnostics {
			diagnostic, _ := raw.(map[string]any)
			if diagnostic["classification"] == "parser_error" && diagnostic["line"] == float64(1026) {
				parserSeen = true
			}
		}
		if !parserSeen {
			t.Fatalf("missing parser diagnostic at line 1026, diagnostics=%#v", diagnostics)
		}
	})
}

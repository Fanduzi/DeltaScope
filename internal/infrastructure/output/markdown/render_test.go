// Package markdown verifies Markdown rendering behavior.
// input: representative internal audit results with statement and global findings
// output: regression coverage for deterministic Markdown rendering
// pos: infrastructure output test coverage for the Markdown renderer
// note: if this file changes, update this header and module README.md.
package markdown

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func TestRenderIncludesSummaryAndStatementFindings(t *testing.T) {
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{
			Statements: 1,
			Blockers:   1,
		},
		Statements: []report.StatementResult{{
			Index:         0,
			Kind:          "dml",
			NormalizedSQL: "update users set name = 'delta'",
			Findings: []rule.Finding{{
				RuleID:        "dml.where.require",
				Level:         rule.LevelBlocker,
				Message:       "UPDATE and DELETE statements must include a WHERE clause",
				StatementKind: "dml",
				Suggestion:    "add a WHERE clause that narrows the affected rows",
				Metadata: map[string]any{
					"operation": "update",
				},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	assertContains(t, output, "# DeltaScope Audit Result")
	assertContains(t, output, "Verdict: `reject`")
	assertContains(t, output, "## Statement 1")
	assertContains(t, output, "`dml.where.require`")
	assertContains(t, output, "Suggestion: add a WHERE clause")
}

func TestRenderIncludesGlobalFindings(t *testing.T) {
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReview,
		Summary: report.Summary{
			Statements: 1,
			Warnings:   1,
		},
		GlobalFindings: []rule.Finding{{
			RuleID:  "batch.warning",
			Level:   rule.LevelWarning,
			Message: "cross-statement review is required",
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	assertContains(t, string(rendered), "## Global Findings")
	assertContains(t, string(rendered), "`batch.warning`")
}

func assertContains(t *testing.T, output string, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Fatalf("expected output to contain %q\noutput:\n%s", want, output)
	}
}

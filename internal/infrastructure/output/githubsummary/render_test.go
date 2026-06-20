// Package githubsummary verifies the GitHub job-summary renderer.
// input: synthetic report results, rule findings, and catalog entries
// output: regression coverage for job-summary Markdown rendering and no-leak behavior
// pos: infrastructure verification of the CI job-summary output adapter
// note: if this file changes, update this header and module README.md.
package githubsummary

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestRenderCleanResult(t *testing.T) {
	t.Parallel()

	output, err := Render(report.Result{
		Verdict: report.VerdictPass,
		Summary: report.Summary{Statements: 1},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	for _, want := range []string{
		"## DeltaScope SQL Review",
		"Verdict: PASS",
		"| Statements | 1 |",
		"No findings.",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in output:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "## Action Summary") {
		t.Fatalf("clean result must not include action summary:\n%s", rendered)
	}
}

func TestRenderFindingActionSummary(t *testing.T) {
	t.Parallel()

	output, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Statements: 1, Blockers: 1},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "DELETE must use a WHERE clause.",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	for _, want := range []string{
		"Verdict: REJECT",
		"| Blockers | 1 |",
		"## Action Summary",
		"- [blocker] `dml.where.require`: 1 finding",
		"Explain: deltascope rules explain dml.where.require",
		"Statements: 1",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in output:\n%s", want, rendered)
		}
	}
}

func TestRenderUsesCatalogSummaryAndSuggestion(t *testing.T) {
	t.Parallel()

	output, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Statements: 1, Blockers: 1},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "fallback message should be overridden by the catalog",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	// dml.where.require is a shipped catalog rule, so the renderer must surface catalog
	// summary/suggestion prose rather than the raw finding message.
	if !strings.Contains(rendered, "  Summary: ") {
		t.Fatalf("expected a non-empty Summary line from the catalog, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "  Suggestion: ") {
		t.Fatalf("expected a non-empty Suggestion line from the catalog, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "fallback message should be overridden by the catalog") {
		t.Fatalf("catalog-backed rule must not fall back to raw finding message:\n%s", rendered)
	}
}

func TestRenderGlobalFindingMarksScope(t *testing.T) {
	t.Parallel()

	output, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Notices: 1},
		GlobalFindings: []rule.Finding{{
			RuleID:  "ddl.global.only",
			Level:   rule.LevelNotice,
			Message: "global scope finding",
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	if !strings.Contains(rendered, "ddl.global.only") {
		t.Fatalf("expected global rule id in output:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Scope: global") {
		t.Fatalf("expected global scope marker in output:\n%s", rendered)
	}
	if strings.Contains(rendered, "Statements:") {
		t.Fatalf("global-only finding must not print statement indexes:\n%s", rendered)
	}
}

func TestRenderOmitsRawSQL(t *testing.T) {
	t.Parallel()

	output, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Statements: 1, Blockers: 1},
		Statements: []report.StatementResult{{
			Index:         0,
			Kind:          "dml",
			RawSQL:        "DELETE FROM users",
			NormalizedSQL: "DELETE FROM users",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "DELETE must use a WHERE clause.",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if strings.Contains(string(output), "DELETE FROM users") {
		t.Fatalf("github summary must not include raw or normalized SQL:\n%s", string(output))
	}
}

func TestRenderTruncatesActionSummary(t *testing.T) {
	t.Parallel()

	findings := make([]rule.Finding, 0, 12)
	for i := 0; i < 12; i++ {
		findings = append(findings, rule.Finding{
			RuleID:  "z.rule." + strconv.Itoa(i),
			Level:   rule.LevelWarning,
			Message: "synthetic finding",
		})
	}

	output, err := Render(report.Result{
		Verdict: report.VerdictReview,
		Summary: report.Summary{Statements: 1, Warnings: 12},
		Statements: []report.StatementResult{{
			Index:    0,
			Kind:     "ddl",
			Findings: findings,
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(string(output), "Showing 10 of 12 rule groups.") {
		t.Fatalf("expected truncation notice, got:\n%s", string(output))
	}
}

func TestRenderDeterministic(t *testing.T) {
	t.Parallel()

	result := report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Statements: 2, Blockers: 1, Warnings: 1},
		Statements: []report.StatementResult{
			{
				Index: 0,
				Kind:  "dml",
				Findings: []rule.Finding{{
					RuleID: "dml.where.require", Level: rule.LevelBlocker, Message: "first",
				}},
			},
			{
				Index: 1,
				Kind:  "ddl",
				Findings: []rule.Finding{{
					RuleID: "ddl.index.name.require", Level: rule.LevelWarning, Message: "second",
				}},
			},
		},
	}

	first, err := Render(result)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for run := 0; run < 5; run++ {
		got, err := Render(result)
		if err != nil {
			t.Fatalf("render run %d: %v", run, err)
		}
		if !bytes.Equal(got, first) {
			t.Fatalf("run %d: non-deterministic output\nfirst:\n%s\ngot:\n%s", run, first, got)
		}
	}
}

func TestRenderHasNoSeverityField(t *testing.T) {
	t.Parallel()

	output, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Statements: 1, Blockers: 1},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause is required",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if strings.Contains(strings.ToLower(string(output)), "severity") {
		t.Fatalf("github summary must not mention severity:\n%s", string(output))
	}
}

func TestRenderUnsupportedOnly(t *testing.T) {
	t.Parallel()

	output, err := Render(report.Result{
		Verdict: report.VerdictPass,
		Summary: report.Summary{Statements: 1},
		Unsupported: []spec.UnsupportedDetail{{
			Index:   0,
			Feature: "CREATE TRIGGER",
			Reason:  "unsupported statement",
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	for _, want := range []string{
		"No findings.",
		"Unsupported statements: 1",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in output:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "## Action Summary") {
		t.Fatalf("result without findings must not include action summary:\n%s", rendered)
	}
	// Only the count is surfaced; the unsupported feature label and any raw SQL stay out.
	if strings.Contains(rendered, "CREATE TRIGGER") {
		t.Fatalf("unsupported count line must not leak the feature label:\n%s", rendered)
	}
}

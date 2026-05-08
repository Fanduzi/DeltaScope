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
	t.Parallel()
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
	t.Parallel()
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

func TestRenderIncludesStatementImpact(t *testing.T) {
	t.Parallel()
	estimatedRows := int64(12)
	estimatedRatio := 0.25

	rendered, err := Render(report.Result{
		Verdict: report.VerdictReview,
		Summary: report.Summary{
			Statements: 1,
		},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Impact: &report.Impact{
				EstimatedRows:  &estimatedRows,
				EstimatedRatio: &estimatedRatio,
				RiskLevel:      report.ImpactRiskMedium,
				Confidence:     report.ImpactConfidenceHigh,
				Source:         report.ImpactSourceMetadata,
				ReasonCodes:    []string{"indexed_range"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	assertContains(t, output, "### Impact")
	assertContains(t, output, "`estimated_rows`: `12`")
	assertContains(t, output, "`estimated_ratio`: `0.2500`")
	assertContains(t, output, "`risk_level`: `medium`")
	assertContains(t, output, "`source`: `metadata`")
}

func TestRenderPreservesTinyNonZeroImpactRatioPrecision(t *testing.T) {
	t.Parallel()
	estimatedRatio := 0.000000001

	rendered, err := Render(report.Result{
		Verdict: report.VerdictReview,
		Summary: report.Summary{
			Statements: 1,
		},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Impact: &report.Impact{
				EstimatedRatio: &estimatedRatio,
				RiskLevel:      report.ImpactRiskLow,
				Confidence:     report.ImpactConfidenceHigh,
				Source:         report.ImpactSourceShape,
			},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	assertContains(t, output, "`estimated_ratio`: `1e-09`")
	assertNotContains(t, output, "`estimated_ratio`: `0.0000`")
}

func TestRenderIncludesAggregateExplanations(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{
			Statements: 1,
			Blockers:   1,
		},
		Explanation: &report.Explanation{
			Summary: "Audit produced 1 finding",
			Reasons: []string{"where clause required"},
		},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Explanation: &report.Explanation{
				Summary: "Statement 1 has 1 finding",
				Reasons: []string{"where clause required"},
			},
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause required",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	assertContains(t, output, "## Result Explanation")
	assertContains(t, output, "Audit produced 1 finding")
	assertContains(t, output, "## Statement 1")
	assertContains(t, output, "### Explanation")
	assertContains(t, output, "Statement 1 has 1 finding")
}

func TestRenderUsesSingleSuggestionLineWhenExplanationIncludesSuggestion(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{
			Statements: 1,
			Blockers:   1,
		},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:      "dml.where.require",
				Level:       rule.LevelBlocker,
				Message:     "where clause required",
				Suggestion:  "legacy suggestion",
				Explanation: &rule.FindingExplanation{Suggestion: "explanation suggestion"},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	if strings.Count(output, "Suggestion:") != 1 {
		t.Fatalf("expected exactly one suggestion line, got output:\n%s", output)
	}
	assertContains(t, output, "Suggestion: explanation suggestion")
	assertNotContains(t, output, "Suggestion: legacy suggestion")
}

func TestRenderFallsBackToLegacySuggestionWhenExplanationSuggestionIsEmpty(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{
			Statements: 1,
			Blockers:   1,
		},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:     "dml.where.require",
				Level:      rule.LevelBlocker,
				Message:    "where clause required",
				Suggestion: "legacy suggestion",
				Explanation: &rule.FindingExplanation{
					Why: "predicate required",
				},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	if strings.Count(output, "Suggestion:") != 1 {
		t.Fatalf("expected exactly one suggestion line, got output:\n%s", output)
	}
	assertContains(t, output, "Suggestion: legacy suggestion")
}

func TestRenderUsesLongerCodeSpanDelimiterWhenSQLContainsBackticks(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{
			Statements: 1,
			Blockers:   1,
		},
		Statements: []report.StatementResult{{
			Index:         0,
			Kind:          "dml",
			NormalizedSQL: "delete from `users` where `id` = 1",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "UPDATE and DELETE statements must include a WHERE clause",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	assertContains(t, output, "- SQL: ``delete from `users` where `id` = 1``")
	assertNotContains(t, output, "- SQL: `delete from `users` where `id` = 1`")
	assertNotContains(t, output, "\\`")
}

func TestRenderUsesDelimiterLongerThanLongestBacktickRun(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{
			Statements: 1,
			Blockers:   1,
		},
		Statements: []report.StatementResult{{
			Index:         0,
			Kind:          "dml",
			NormalizedSQL: "select ``value`` from audit_log",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "UPDATE and DELETE statements must include a WHERE clause",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	assertContains(t, output, "- SQL: ```select ``value`` from audit_log```")
}

func TestRenderPadsCodeSpanWhenSQLStartsWithBacktick(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{
			Statements: 1,
			Blockers:   1,
		},
		Statements: []report.StatementResult{{
			Index:         0,
			Kind:          "dml",
			NormalizedSQL: "`users`",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "UPDATE and DELETE statements must include a WHERE clause",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	assertContains(t, output, "- SQL: `` `users` ``")
}

func TestRenderPadsCodeSpanWhenSQLEndsWithBacktick(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{
			Statements: 1,
			Blockers:   1,
		},
		Statements: []report.StatementResult{{
			Index:         0,
			Kind:          "dml",
			NormalizedSQL: "users`",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "UPDATE and DELETE statements must include a WHERE clause",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	assertContains(t, output, "- SQL: `` users` ``")
}

func TestRenderPadsCodeSpanWhenSQLStartsAndEndsWithBacktick(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{
			Statements: 1,
			Blockers:   1,
		},
		Statements: []report.StatementResult{{
			Index:         0,
			Kind:          "dml",
			NormalizedSQL: "`users`",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "UPDATE and DELETE statements must include a WHERE clause",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	assertContains(t, output, "- SQL: `` `users` ``")
}

func assertContains(t *testing.T, output string, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Fatalf("expected output to contain %q\noutput:\n%s", want, output)
	}
}

func assertNotContains(t *testing.T, output string, want string) {
	t.Helper()
	if strings.Contains(output, want) {
		t.Fatalf("expected output not to contain %q\noutput:\n%s", want, output)
	}
}

func TestMarkdownRenderIncludesRuleSummary(t *testing.T) {
	t.Parallel()
	loaded := 147
	applicable := 103
	rendered, err := Render(report.Result{
		Verdict: report.VerdictPass,
		Summary: report.Summary{Statements: 1},
		RuleSummary: &report.RuleSummary{
			Loaded:     loaded,
			Applicable: applicable,
			Skipped: []rule.SkippedRule{{
				RuleID: "ddl.pg.table.engine.allowlist",
				Reason: rule.SkipReasonDialectMismatch,
			}},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	assertContains(t, output, "## Rule Summary")
	assertContains(t, output, "- Loaded: 147")
	assertContains(t, output, "- Applicable: 103")
	assertContains(t, output, "- Skipped: 1")
}

func TestMarkdownRenderIncludesSkippedRulesSection(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictPass,
		Summary: report.Summary{Statements: 1},
		RuleSummary: &report.RuleSummary{
			Loaded:     10,
			Applicable: 8,
			Skipped: []rule.SkippedRule{
				{RuleID: "ddl.pg.table.engine.allowlist", Reason: rule.SkipReasonDialectMismatch},
				{RuleID: "ddl.pg.index.concurrent.require", Reason: rule.SkipReasonDialectMismatch},
			},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	assertContains(t, output, "## Skipped Rules")
	assertContains(t, output, "`ddl.pg.index.concurrent.require`: not applicable to current dialect")
	assertContains(t, output, "`ddl.pg.table.engine.allowlist`: not applicable to current dialect")
}

func TestMarkdownRenderOmitsSkippedSectionWhenEmpty(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictPass,
		Summary: report.Summary{Statements: 1},
		RuleSummary: &report.RuleSummary{
			Loaded:     10,
			Applicable: 10,
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	assertContains(t, output, "## Rule Summary")
	assertNotContains(t, output, "## Skipped Rules")
}

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
	// The derived Action Summary now contributes its own Suggestion line, so the
	// dedup assertion is scoped to the per-statement finding detail.
	statementView := statementSection(output)
	if strings.Count(statementView, "Suggestion:") != 1 {
		t.Fatalf("expected exactly one suggestion line in the statement finding detail, got output:\n%s", output)
	}
	assertContains(t, statementView, "Suggestion: explanation suggestion")
	assertNotContains(t, statementView, "Suggestion: legacy suggestion")
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
	// The derived Action Summary now contributes its own Suggestion line, so the
	// dedup assertion is scoped to the per-statement finding detail.
	statementView := statementSection(output)
	if strings.Count(statementView, "Suggestion:") != 1 {
		t.Fatalf("expected exactly one suggestion line in the statement finding detail, got output:\n%s", output)
	}
	assertContains(t, statementView, "Suggestion: legacy suggestion")
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

// statementSection returns the substring of output starting at the first per-statement
// heading. The derived Action Summary renders before statement sections and carries its
// own Suggestion line, so scoping here isolates the per-statement finding detail.
func statementSection(output string) string {
	if idx := strings.Index(output, "## Statement"); idx >= 0 {
		return output[idx:]
	}
	return output
}

// actionSummarySection returns the body of the "## Action Summary" section up to the next
// top-level heading, or "" when the result has no findings and omits the section.
func actionSummarySection(output string) string {
	lines := strings.Split(output, "\n")
	inSection := false
	var collected []string
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if inSection {
				break
			}
			inSection = line == "## Action Summary"
			continue
		}
		if inSection {
			collected = append(collected, line)
		}
	}
	return strings.Join(collected, "\n")
}

// actionSummaryItems returns the Markdown list item lines within the Action Summary section.
func actionSummaryItems(output string) []string {
	lines := strings.Split(output, "\n")
	inSection := false
	var items []string
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			inSection = line == "## Action Summary"
			continue
		}
		if inSection && strings.HasPrefix(line, "- [") {
			items = append(items, line)
		}
	}
	return items
}

func TestRenderIncludesActionSummaryForBlocker(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Statements: 1, Blockers: 1},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
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
	assertContains(t, output, "## Action Summary")
	// Action Summary must sit after the summary counts and before the statement sections.
	noticesIdx := strings.Index(output, "- Notices:")
	actionIdx := strings.Index(output, "## Action Summary")
	if noticesIdx < 0 || actionIdx < 0 || actionIdx < noticesIdx {
		t.Fatalf("expected Action Summary after the summary counts, got output:\n%s", output)
	}
	assertContains(t, output, "- [blocker] `dml.where.require`: 1 finding")
}

func TestRenderOmitsActionSummaryForCleanResult(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictPass,
		Summary: report.Summary{Statements: 1},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	assertNotContains(t, string(rendered), "## Action Summary")
}

func TestRenderActionSummaryIncludesExplainCommand(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Statements: 1, Blockers: 1},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
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

	assertContains(t, string(rendered), "Explain: deltascope rules explain dml.where.require")
}

func TestRenderActionSummaryIncludesOneBasedStatementIndexes(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Statements: 3, Blockers: 2},
		Statements: []report.StatementResult{
			{Index: 0, Kind: "dml", Findings: []rule.Finding{{
				RuleID: "dml.where.require", Level: rule.LevelBlocker, Message: "where clause required",
			}}},
			{Index: 1, Kind: "ddl"},
			{Index: 2, Kind: "dml", Findings: []rule.Finding{{
				RuleID: "dml.where.require", Level: rule.LevelBlocker, Message: "where clause required",
			}}},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	assertContains(t, string(rendered), "Statements: 1, 3")
}

func TestRenderActionSummaryGroupsDuplicateRuleFindings(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Statements: 2, Blockers: 2},
		Statements: []report.StatementResult{
			{Index: 0, Kind: "dml", Findings: []rule.Finding{{
				RuleID: "dml.where.require", Level: rule.LevelBlocker, Message: "where clause required",
			}}},
			{Index: 1, Kind: "dml", Findings: []rule.Finding{{
				RuleID: "dml.where.require", Level: rule.LevelBlocker, Message: "where clause required",
			}}},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	items := actionSummaryItems(string(rendered))
	if len(items) != 1 {
		t.Fatalf("expected one action summary rule group, got %d: %v", len(items), items)
	}
	assertContains(t, items[0], "`dml.where.require`")
	assertContains(t, items[0], "2 findings")
}

func TestRenderActionSummaryOrdersBlockerBeforeWarningAndNotice(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Statements: 3, Blockers: 1, Warnings: 1, Notices: 1},
		Statements: []report.StatementResult{
			{Index: 0, Kind: "ddl", Findings: []rule.Finding{{
				RuleID: "ddl.database.create.notice", Level: rule.LevelNotice, Message: "database creation is informational",
			}}},
			{Index: 1, Kind: "dml", Findings: []rule.Finding{{
				RuleID: "dml.where.require", Level: rule.LevelBlocker, Message: "where clause required",
			}}},
			{Index: 2, Kind: "dml", Findings: []rule.Finding{{
				RuleID: "dml.limit.forbid", Level: rule.LevelWarning, Message: "limit forbidden",
			}}},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	items := actionSummaryItems(string(rendered))
	if len(items) != 3 {
		t.Fatalf("expected three action summary rule groups, got %d: %v", len(items), items)
	}
	if !strings.HasPrefix(items[0], "- [blocker]") {
		t.Fatalf("expected blocker first, got %q", items[0])
	}
	if !strings.HasPrefix(items[1], "- [warning]") {
		t.Fatalf("expected warning second, got %q", items[1])
	}
	if !strings.HasPrefix(items[2], "- [notice]") {
		t.Fatalf("expected notice third, got %q", items[2])
	}
}

func TestRenderActionSummaryTruncatesAfterTenRuleGroups(t *testing.T) {
	t.Parallel()
	ruleIDs := []string{
		"dml.where.require",
		"dml.limit.forbid",
		"dml.insert.select.forbid",
		"dml.join.on.require",
		"dml.replace.forbid",
		"dml.subquery.forbid",
		"dml.table.denylist.forbid",
		"ddl.table.drop.forbid",
		"ddl.table.truncate.forbid",
		"ddl.view.create.forbid",
		"ddl.view.drop.forbid",
		"ddl.column.bit.forbid",
	}
	statements := make([]report.StatementResult, 0, len(ruleIDs))
	for i, id := range ruleIDs {
		statements = append(statements, report.StatementResult{
			Index: i,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  id,
				Level:   rule.LevelBlocker,
				Message: "blocked",
			}},
		})
	}

	rendered, err := Render(report.Result{
		Verdict:    report.VerdictReject,
		Summary:    report.Summary{Statements: len(ruleIDs), Blockers: len(ruleIDs)},
		Statements: statements,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	output := string(rendered)
	items := actionSummaryItems(output)
	if len(items) != 10 {
		t.Fatalf("expected 10 truncated action summary groups, got %d: %v", len(items), items)
	}
	assertContains(t, output, "Showing 10 of 12 rule groups.")
}

func TestRenderActionSummaryMarksGlobalScope(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReview,
		Summary: report.Summary{Warnings: 1},
		GlobalFindings: []rule.Finding{{
			RuleID:  "batch.warning",
			Level:   rule.LevelWarning,
			Message: "cross-statement review is required",
		}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	section := actionSummarySection(string(rendered))
	if section == "" {
		t.Fatalf("expected an Action Summary section, got output:\n%s", string(rendered))
	}
	assertContains(t, section, "Scope: global")
}

func TestRenderActionSummaryHasNoSeverityField(t *testing.T) {
	t.Parallel()
	rendered, err := Render(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Statements: 1, Blockers: 1},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
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

	assertNotContains(t, string(rendered), "severity")
}

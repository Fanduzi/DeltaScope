// Package report verifies audit result aggregation behavior.
// input: domain report types and synthetic findings for test scenarios
// output: test coverage for verdict and summary correctness
// pos: domain verification of reporting behavior
// note: if this file changes, update this header and module README.md.
package report

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func TestVerdictPass(t *testing.T) {
	result := Aggregate(nil, nil)

	if result.Verdict != VerdictPass {
		t.Fatalf("expected verdict %q, got %q", VerdictPass, result.Verdict)
	}
	if result.Summary.Blockers != 0 || result.Summary.Warnings != 0 || result.Summary.Notices != 0 {
		t.Fatalf("expected empty summary, got %+v", result.Summary)
	}
}

func TestVerdictReview(t *testing.T) {
	result := Aggregate(nil, []rule.Finding{
		{RuleID: "ddl.table.comment.require", Level: rule.LevelWarning, Message: "table comment is recommended"},
	})

	if result.Verdict != VerdictReview {
		t.Fatalf("expected verdict %q, got %q", VerdictReview, result.Verdict)
	}
	if result.Summary.Warnings != 1 {
		t.Fatalf("expected 1 warning, got %+v", result.Summary)
	}
	if len(result.GlobalFindings) != 1 {
		t.Fatalf("expected 1 global finding, got %+v", result.GlobalFindings)
	}
}

func TestVerdictReject(t *testing.T) {
	result := Aggregate(nil, []rule.Finding{
		{RuleID: "dml.where.require", Level: rule.LevelBlocker, Message: "where clause is required"},
		{RuleID: "ddl.table.comment.require", Level: rule.LevelWarning, Message: "table comment is recommended"},
	})

	if result.Verdict != VerdictReject {
		t.Fatalf("expected verdict %q, got %q", VerdictReject, result.Verdict)
	}
	if result.Summary.Blockers != 1 || result.Summary.Warnings != 1 {
		t.Fatalf("expected blocker and warning counts, got %+v", result.Summary)
	}
	if len(result.GlobalFindings) != 2 {
		t.Fatalf("expected 2 global findings, got %+v", result.GlobalFindings)
	}
}

func TestAggregatePreservesStatementExplanation(t *testing.T) {
	result := Aggregate([]StatementResult{{
		Index: 0,
		Kind:  "dml",
		Findings: []rule.Finding{{
			RuleID:  "dml.where.require",
			Level:   rule.LevelWarning,
			Message: "where clause is recommended",
		}},
		Explanation: &Explanation{
			Summary: "statement explanation",
			Reasons: []string{"missing predicate"},
		},
	}}, nil)

	if result.Statements[0].Explanation == nil {
		t.Fatal("expected statement explanation to be preserved")
	}
	if result.Statements[0].Explanation.Summary != "statement explanation" {
		t.Fatalf("expected statement explanation summary to be preserved, got %#v", result.Statements[0].Explanation)
	}
	if result.Verdict != VerdictReview {
		t.Fatalf("expected verdict %q, got %q", VerdictReview, result.Verdict)
	}
	if result.Summary.Warnings != 1 {
		t.Fatalf("expected warning count to stay based on findings, got %+v", result.Summary)
	}
}

func TestAggregatePreservesStatementImpact(t *testing.T) {
	result := Aggregate([]StatementResult{{
		Index: 0,
		Kind:  "dml",
		Impact: &Impact{
			EstimatedRows:  ptrInt64(1),
			EstimatedRatio: ptrFloat64(0.01),
			RiskLevel:      ImpactRiskLow,
			Confidence:     ImpactConfidenceHigh,
			Source:         ImpactSourceShape,
			ReasonCodes:    []string{"pk_equality"},
		},
	}}, nil)

	if result.Statements[0].Impact == nil {
		t.Fatal("expected statement impact to be preserved")
	}
	if result.Statements[0].Impact.RiskLevel != ImpactRiskLow {
		t.Fatalf("expected statement impact risk level to be preserved, got %#v", result.Statements[0].Impact)
	}
	if result.Verdict != VerdictPass {
		t.Fatalf("expected verdict %q, got %q", VerdictPass, result.Verdict)
	}
}

func TestAggregatePreservesVerdictSemanticsWithExplanation(t *testing.T) {
	result := Aggregate([]StatementResult{{
		Index: 0,
		Kind:  "ddl",
		Findings: []rule.Finding{{
			RuleID:  "ddl.table.comment.require",
			Level:   rule.LevelWarning,
			Message: "table comment is recommended",
		}},
		Explanation: &Explanation{
			Summary: "statement explanation",
			Reasons: []string{"missing table comment"},
		},
	}}, []rule.Finding{{
		RuleID:  "dml.where.require",
		Level:   rule.LevelBlocker,
		Message: "where clause is required",
	}})

	if result.Verdict != VerdictReject {
		t.Fatalf("expected verdict %q, got %q", VerdictReject, result.Verdict)
	}
	if result.Summary.Blockers != 1 || result.Summary.Warnings != 1 {
		t.Fatalf("expected blocker and warning counts to remain unchanged, got %+v", result.Summary)
	}
	if result.Statements[0].Explanation == nil {
		t.Fatal("expected statement explanation to be preserved")
	}
}

func TestAggregateBuildsStatementExplanationFromFindings(t *testing.T) {
	result := Aggregate([]StatementResult{{
		Index: 0,
		Kind:  "dml",
		Findings: []rule.Finding{{
			RuleID:  "dml.where.require",
			Level:   rule.LevelBlocker,
			Message: "where clause is required",
		}, {
			RuleID:  "dml.limit.forbid",
			Level:   rule.LevelWarning,
			Message: "limit is discouraged",
		}},
	}}, nil)

	if result.Statements[0].Explanation == nil {
		t.Fatal("expected generated statement explanation")
	}
	if result.Statements[0].Explanation.Summary == "" {
		t.Fatalf("expected statement explanation summary, got %#v", result.Statements[0].Explanation)
	}
	if len(result.Statements[0].Explanation.Reasons) != 2 {
		t.Fatalf("expected statement explanation reasons from findings, got %#v", result.Statements[0].Explanation)
	}
	if result.Summary.Blockers != 1 || result.Summary.Warnings != 1 {
		t.Fatalf("expected summary counts to remain based on findings, got %+v", result.Summary)
	}
}

func TestAggregateBuildsResultExplanationFromAggregateFindings(t *testing.T) {
	result := Aggregate([]StatementResult{{
		Index: 0,
		Kind:  "dml",
		Findings: []rule.Finding{{
			RuleID:  "dml.where.require",
			Level:   rule.LevelBlocker,
			Message: "where clause is required",
		}},
	}}, []rule.Finding{{
		RuleID:  "ddl.alter.merge.mysql.require",
		Level:   rule.LevelWarning,
		Message: "merge repeated alters",
	}})

	if result.Explanation == nil {
		t.Fatal("expected generated result explanation")
	}
	if result.Explanation.Summary == "" {
		t.Fatalf("expected result explanation summary, got %#v", result.Explanation)
	}
	if len(result.Explanation.Reasons) != 2 {
		t.Fatalf("expected result explanation reasons from all findings, got %#v", result.Explanation)
	}
	if result.Verdict != VerdictReject {
		t.Fatalf("expected verdict %q, got %q", VerdictReject, result.Verdict)
	}
}

func TestAggregateKeepsDistinctReasonsWhenMessagesMatch(t *testing.T) {
	result := Aggregate([]StatementResult{{
		Index: 0,
		Kind:  "ddl",
		Findings: []rule.Finding{{
			RuleID:  "ddl.rule.one",
			Level:   rule.LevelWarning,
			Message: "duplicate message",
		}, {
			RuleID:  "ddl.rule.two",
			Level:   rule.LevelWarning,
			Message: "duplicate message",
		}},
	}}, nil)

	if result.Explanation == nil {
		t.Fatal("expected generated result explanation")
	}
	if len(result.Explanation.Reasons) != 2 {
		t.Fatalf("expected one reason per finding even when messages match, got %#v", result.Explanation)
	}
}

func TestAggregatePreservesRuleSummary(t *testing.T) {
	summary := &RuleSummary{
		Loaded:     50,
		Applicable: 46,
		Skipped: []rule.SkippedRule{
			{RuleID: "ddl.pg.create_index.concurrently.require", Reason: rule.SkipReasonDialectMismatch},
		},
	}

	result := Aggregate(nil, nil)
	result.RuleSummary = summary

	if result.RuleSummary == nil {
		t.Fatal("expected rule summary to be preserved")
	}
	if result.RuleSummary.Loaded != 50 {
		t.Fatalf("expected loaded 50, got %d", result.RuleSummary.Loaded)
	}
	if result.RuleSummary.Applicable != 46 {
		t.Fatalf("expected applicable 46, got %d", result.RuleSummary.Applicable)
	}
	if len(result.RuleSummary.Skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(result.RuleSummary.Skipped))
	}
	if result.RuleSummary.Skipped[0].Reason != rule.SkipReasonDialectMismatch {
		t.Fatalf("expected dialect_mismatch, got %s", result.RuleSummary.Skipped[0].Reason)
	}
}

func ptrInt64(value int64) *int64 {
	return &value
}

func ptrFloat64(value float64) *float64 {
	return &value
}

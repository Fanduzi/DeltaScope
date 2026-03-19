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
}

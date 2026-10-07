// Package cli verifies the T06-A4 fail-on exit contract on the A-path result
// shape: review/unverified with one evidence gap and zero findings exits 0
// at blocker/none and 1 at warning/notice. The result shape itself is proven
// against the real provider in the shared auditmeta tests; this file pins
// only the exit mapping on that shape — no duplicated database case.
// input: a constructed report.Result matching the frozen A-path shape
// output: exitOK/exitAudit per --fail-on level; fail_on_triggered false at blocker
// pos: CLI exit-table contract test for issue #85 T06-A4
// note: if this file changes, update this header and module README.md.
package cli

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

// t06a4AResultShape mirrors the frozen A outcome: statement 0 complete,
// statement 1 unverified with exactly one unknown_table_state gap and no
// findings, aggregate review/unverified.
func t06a4AResultShape() report.Result {
	return report.Result{
		Verdict:  report.VerdictReview,
		Coverage: report.Coverage{Status: report.CoverageUnverified},
		Statements: []report.StatementResult{
			{Index: 0, Coverage: report.Coverage{Status: report.CoverageComplete}},
			{
				Index:    1,
				Coverage: report.Coverage{Status: report.CoverageUnverified},
				EvidenceGaps: []rule.EvidenceGap{{
					RuleID:        "ddl.create_index.columns.exists.require",
					ReasonCode:    "unknown_table_state",
					RequiredFacts: []string{"target_table.columns", "target_table.existence"},
				}},
			},
		},
	}
}

func TestT06A4FailOnExitTable(t *testing.T) {
	for _, tc := range []struct {
		level string
		want  int
	}{
		{"blocker", exitOK},
		{"none", exitOK},
		{"warning", exitAudit},
		{"notice", exitAudit},
	} {
		t.Run(tc.level, func(t *testing.T) {
			if got := exitCodeForResult(t06a4AResultShape(), tc.level); got != tc.want {
				t.Fatalf("exitCodeForResult(--fail-on %s) = %d, want %d", tc.level, got, tc.want)
			}
		})
	}
}

// TestT06A4FailOnTriggeredStaysFalseAtBlocker pins the JSON-side flag: with
// --fail-on blocker the gap does not trigger, so recorded runs keep
// fail_on_triggered=false beside the review verdict.
func TestT06A4FailOnTriggeredStaysFalseAtBlocker(t *testing.T) {
	if failOnTriggered(t06a4AResultShape(), "blocker") {
		t.Fatal("fail_on_triggered = true at blocker, want false")
	}
	if !failOnTriggered(t06a4AResultShape(), "warning") {
		t.Fatal("fail_on_triggered = false at warning, want true (evidence-gap weight)")
	}
}

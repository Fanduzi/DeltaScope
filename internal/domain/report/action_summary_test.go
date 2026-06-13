// Package report verifies audit result aggregation behavior.
// This file verifies the derived action summary that groups findings by rule.
// input: synthetic report results, rule findings, and catalog entries
// output: regression coverage for action summary grouping, ordering, and fallbacks
// pos: domain verification of derived human-report behavior
// note: if this file changes, update this header and module README.md.
package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
)

func TestBuildActionSummaryEmptyResult(t *testing.T) {
	t.Parallel()

	summary := BuildActionSummary(Result{}, nil, ActionSummaryOptions{})

	if summary.Items == nil {
		t.Fatal("expected non-nil Items for empty result, got nil")
	}
	if len(summary.Items) != 0 {
		t.Fatalf("expected zero items, got %d", len(summary.Items))
	}
	if summary.TotalItems != 0 {
		t.Fatalf("expected total 0, got %d", summary.TotalItems)
	}
}

func TestBuildActionSummaryGroupsByRuleID(t *testing.T) {
	t.Parallel()

	result := Result{Statements: []StatementResult{
		{Index: 0, Findings: []rule.Finding{
			{RuleID: "dml.where.require", Level: rule.LevelBlocker, Message: "where required"},
		}},
		{Index: 1, Findings: []rule.Finding{
			{RuleID: "dml.where.require", Level: rule.LevelBlocker, Message: "where required again"},
		}},
		{Index: 2, Findings: []rule.Finding{
			{RuleID: "ddl.index.name.require", Level: rule.LevelWarning, Message: "index name"},
		}},
	}}

	summary := BuildActionSummary(result, nil, ActionSummaryOptions{})

	if len(summary.Items) != 2 {
		t.Fatalf("expected 2 groups, got %d (%+v)", len(summary.Items), summary.Items)
	}
	if summary.TotalItems != 2 {
		t.Fatalf("expected total 2, got %d", summary.TotalItems)
	}

	var blocker ActionItem
	for _, item := range summary.Items {
		if item.RuleID == "dml.where.require" {
			blocker = item
		}
	}
	if blocker.RuleID == "" {
		t.Fatalf("expected dml.where.require group, got %+v", summary.Items)
	}
	if blocker.Count != 2 {
		t.Fatalf("expected count 2 for grouped rule, got %d", blocker.Count)
	}
	if len(blocker.StatementIndexes) != 2 {
		t.Fatalf("expected 2 statement indexes, got %+v", blocker.StatementIndexes)
	}
}

func TestBuildActionSummaryIncludesGlobalFindings(t *testing.T) {
	t.Parallel()

	result := Result{
		Statements: []StatementResult{
			{Index: 0, Findings: []rule.Finding{
				{RuleID: "ddl.table.comment.require", Level: rule.LevelWarning, Message: "comment"},
			}},
		},
		GlobalFindings: []rule.Finding{
			{RuleID: "ddl.global.rule", Level: rule.LevelNotice, Message: "global only"},
			{RuleID: "ddl.table.comment.require", Level: rule.LevelWarning, Message: "global comment"},
		},
	}

	summary := BuildActionSummary(result, nil, ActionSummaryOptions{})

	byID := map[string]ActionItem{}
	for _, item := range summary.Items {
		byID[item.RuleID] = item
	}

	comment, ok := byID["ddl.table.comment.require"]
	if !ok {
		t.Fatal("expected ddl.table.comment.require group")
	}
	if !comment.HasGlobalFindings {
		t.Error("expected HasGlobalFindings true for rule with a global finding")
	}
	if comment.Count != 2 {
		t.Fatalf("expected count 2 (statement + global), got %d", comment.Count)
	}

	globalOnly, ok := byID["ddl.global.rule"]
	if !ok {
		t.Fatal("expected ddl.global.rule group from global findings")
	}
	if !globalOnly.HasGlobalFindings {
		t.Error("expected HasGlobalFindings true for global-only rule")
	}
	if len(globalOnly.StatementIndexes) != 0 {
		t.Fatalf("expected no statement indexes for global-only rule, got %+v", globalOnly.StatementIndexes)
	}
}

func TestBuildActionSummaryStatementIndexesOneBased(t *testing.T) {
	t.Parallel()

	result := Result{Statements: []StatementResult{
		{Index: 0, Findings: []rule.Finding{
			{RuleID: "ddl.first", Level: rule.LevelNotice, Message: "first"},
		}},
		{Index: 1, Findings: []rule.Finding{
			{RuleID: "ddl.second", Level: rule.LevelNotice, Message: "second"},
		}},
		{Index: 2, Findings: []rule.Finding{
			{RuleID: "ddl.third", Level: rule.LevelNotice, Message: "third"},
		}},
	}}

	summary := BuildActionSummary(result, nil, ActionSummaryOptions{})

	byID := map[string]ActionItem{}
	for _, item := range summary.Items {
		byID[item.RuleID] = item
	}

	if got := byID["ddl.first"].StatementIndexes; len(got) != 1 || got[0] != 1 {
		t.Fatalf("expected first statement index [1], got %+v", got)
	}
	if got := byID["ddl.third"].StatementIndexes; len(got) != 1 || got[0] != 3 {
		t.Fatalf("expected third statement index [3], got %+v", got)
	}
}

func TestBuildActionSummaryStatementIndexesDeduplicated(t *testing.T) {
	t.Parallel()

	result := Result{Statements: []StatementResult{
		{Index: 0, Findings: []rule.Finding{
			{RuleID: "ddl.dup", Level: rule.LevelNotice, Message: "one"},
			{RuleID: "ddl.dup", Level: rule.LevelNotice, Message: "two"},
			{RuleID: "ddl.dup", Level: rule.LevelNotice, Message: "three"},
		}},
		{Index: 1, Findings: []rule.Finding{
			{RuleID: "ddl.dup", Level: rule.LevelNotice, Message: "four"},
		}},
	}}

	summary := BuildActionSummary(result, nil, ActionSummaryOptions{})

	if len(summary.Items) != 1 {
		t.Fatalf("expected 1 group, got %d", len(summary.Items))
	}
	item := summary.Items[0]
	if item.Count != 4 {
		t.Fatalf("expected count 4 across findings, got %d", item.Count)
	}
	if got := item.StatementIndexes; len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("expected deduped indexes [1 2], got %+v", got)
	}
}

func TestBuildActionSummaryLevelHighestPriority(t *testing.T) {
	t.Parallel()

	result := Result{
		Statements: []StatementResult{
			{Index: 0, Findings: []rule.Finding{
				{RuleID: "ddl.mix", Level: rule.LevelNotice, Message: "notice"},
			}},
			{Index: 1, Findings: []rule.Finding{
				{RuleID: "ddl.mix", Level: rule.LevelWarning, Message: "warning"},
			}},
		},
		GlobalFindings: []rule.Finding{
			{RuleID: "ddl.mix", Level: rule.LevelBlocker, Message: "blocker"},
		},
	}

	summary := BuildActionSummary(result, nil, ActionSummaryOptions{})

	if len(summary.Items) != 1 {
		t.Fatalf("expected 1 group, got %d", len(summary.Items))
	}
	if summary.Items[0].Level != rule.LevelBlocker {
		t.Fatalf("expected highest priority blocker, got %q", summary.Items[0].Level)
	}
}

func TestBuildActionSummaryBlockerSortsBeforeWarningNotice(t *testing.T) {
	t.Parallel()

	result := Result{Statements: []StatementResult{
		{Index: 0, Findings: []rule.Finding{
			{RuleID: "ddl.notice", Level: rule.LevelNotice, Message: "n"},
			{RuleID: "ddl.warning", Level: rule.LevelWarning, Message: "w"},
			{RuleID: "ddl.blocker", Level: rule.LevelBlocker, Message: "b"},
		}},
	}}

	summary := BuildActionSummary(result, nil, ActionSummaryOptions{})

	if len(summary.Items) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(summary.Items))
	}
	got := []string{
		string(summary.Items[0].Level),
		string(summary.Items[1].Level),
		string(summary.Items[2].Level),
	}
	want := []string{"blocker", "warning", "notice"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: expected level %q, got %q (full order %v)", i, want[i], got[i], got)
		}
	}
}

func TestBuildActionSummaryCountDescendingTieBreak(t *testing.T) {
	t.Parallel()

	result := Result{Statements: []StatementResult{
		{Index: 0, Findings: []rule.Finding{
			{RuleID: "ddl.one", Level: rule.LevelWarning, Message: "a"},
		}},
		{Index: 1, Findings: []rule.Finding{
			{RuleID: "ddl.two", Level: rule.LevelWarning, Message: "b"},
			{RuleID: "ddl.two", Level: rule.LevelWarning, Message: "c"},
		}},
	}}

	summary := BuildActionSummary(result, nil, ActionSummaryOptions{})

	if len(summary.Items) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(summary.Items))
	}
	if summary.Items[0].RuleID != "ddl.two" || summary.Items[0].Count != 2 {
		t.Fatalf("expected higher-count rule first, got %+v", summary.Items[0])
	}
	if summary.Items[1].RuleID != "ddl.one" || summary.Items[1].Count != 1 {
		t.Fatalf("expected lower-count rule second, got %+v", summary.Items[1])
	}
}

func TestBuildActionSummaryRuleIDAscendingTieBreak(t *testing.T) {
	t.Parallel()

	// Same level and same count: rule ID ascending wins.
	result := Result{Statements: []StatementResult{
		{Index: 0, Findings: []rule.Finding{
			{RuleID: "ddl.zeta", Level: rule.LevelNotice, Message: "z"},
			{RuleID: "ddl.alpha", Level: rule.LevelNotice, Message: "a"},
			{RuleID: "ddl.mid", Level: rule.LevelNotice, Message: "m"},
		}},
	}}

	summary := BuildActionSummary(result, nil, ActionSummaryOptions{})

	got := []string{
		summary.Items[0].RuleID,
		summary.Items[1].RuleID,
		summary.Items[2].RuleID,
	}
	want := []string{"ddl.alpha", "ddl.mid", "ddl.zeta"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: expected %q, got %q (full order %v)", i, want[i], got[i], got)
		}
	}
}

func TestBuildActionSummaryCatalogSummarySuggestionPreferred(t *testing.T) {
	t.Parallel()

	result := Result{Statements: []StatementResult{
		{Index: 0, Findings: []rule.Finding{
			{RuleID: "dml.where.require", Level: rule.LevelBlocker, Message: "raw finding message"},
		}},
	}}

	entries := []catalog.Entry{{
		RuleID:     "dml.where.require",
		Summary:    "Catalog summary text",
		Suggestion: "Catalog suggestion text",
	}}

	summary := BuildActionSummary(result, entries, ActionSummaryOptions{})

	if len(summary.Items) != 1 {
		t.Fatalf("expected 1 group, got %d", len(summary.Items))
	}
	item := summary.Items[0]
	if item.Summary != "Catalog summary text" {
		t.Fatalf("expected catalog summary, got %q", item.Summary)
	}
	if item.Suggestion != "Catalog suggestion text" {
		t.Fatalf("expected catalog suggestion, got %q", item.Suggestion)
	}
}

func TestBuildActionSummaryFallbackForUnknownCatalogRule(t *testing.T) {
	t.Parallel()

	result := Result{Statements: []StatementResult{
		{Index: 0, Findings: []rule.Finding{
			{
				RuleID:     "custom.unknown.rule",
				Level:      rule.LevelWarning,
				Message:    "first finding message",
				Suggestion: "fix it like this",
			},
		}},
	}}

	// No matching catalog entry: fallback to finding text.
	summary := BuildActionSummary(result, []catalog.Entry{{
		RuleID:     "different.rule",
		Summary:    "Unrelated",
		Suggestion: "Unrelated",
	}}, ActionSummaryOptions{})

	if len(summary.Items) != 1 {
		t.Fatalf("expected 1 group, got %d", len(summary.Items))
	}
	item := summary.Items[0]
	if item.Summary != "first finding message" {
		t.Fatalf("expected fallback summary from finding message, got %q", item.Summary)
	}
	if item.Suggestion != "fix it like this" {
		t.Fatalf("expected fallback suggestion from finding, got %q", item.Suggestion)
	}
	if item.ExplainCommand != "deltascope rules explain custom.unknown.rule" {
		t.Fatalf("unexpected explain command %q", item.ExplainCommand)
	}
}

func TestBuildActionSummaryFallbackSuggestionFirstNonEmpty(t *testing.T) {
	t.Parallel()

	result := Result{Statements: []StatementResult{
		{Index: 0, Findings: []rule.Finding{
			{RuleID: "custom.rule", Level: rule.LevelWarning, Message: "m1", Suggestion: ""},
			{RuleID: "custom.rule", Level: rule.LevelWarning, Message: "m2", Suggestion: "   "},
			{RuleID: "custom.rule", Level: rule.LevelWarning, Message: "m3", Suggestion: "real suggestion"},
			{RuleID: "custom.rule", Level: rule.LevelWarning, Message: "m4", Suggestion: "later suggestion"},
		}},
	}}

	summary := BuildActionSummary(result, nil, ActionSummaryOptions{})

	if len(summary.Items) != 1 {
		t.Fatalf("expected 1 group, got %d", len(summary.Items))
	}
	item := summary.Items[0]
	if item.Summary != "m1" {
		t.Fatalf("expected fallback summary from first finding message, got %q", item.Summary)
	}
	if item.Suggestion != "real suggestion" {
		t.Fatalf("expected first non-empty suggestion, got %q", item.Suggestion)
	}
	if item.Count != 4 {
		t.Fatalf("expected count 4, got %d", item.Count)
	}
}

func TestBuildActionSummaryLimitTruncates(t *testing.T) {
	t.Parallel()

	result := Result{Statements: []StatementResult{
		{Index: 0, Findings: []rule.Finding{
			{RuleID: "ddl.one", Level: rule.LevelBlocker, Message: "1"},
			{RuleID: "ddl.two", Level: rule.LevelBlocker, Message: "2"},
			{RuleID: "ddl.three", Level: rule.LevelBlocker, Message: "3"},
			{RuleID: "ddl.four", Level: rule.LevelBlocker, Message: "4"},
		}},
	}}

	summary := BuildActionSummary(result, nil, ActionSummaryOptions{Limit: 2})

	if len(summary.Items) != 2 {
		t.Fatalf("expected truncated items length 2, got %d", len(summary.Items))
	}
	if summary.TotalItems != 4 {
		t.Fatalf("expected total to preserve pre-truncation count 4, got %d", summary.TotalItems)
	}
	// Truncation keeps the first two in sorted order (blocker, count tie, rule ID ascending).
	if summary.Items[0].RuleID != "ddl.four" || summary.Items[1].RuleID != "ddl.one" {
		t.Fatalf("expected ddl.four, ddl.one order, got %s, %s", summary.Items[0].RuleID, summary.Items[1].RuleID)
	}
}

func TestBuildActionSummaryLimitZeroReturnsAll(t *testing.T) {
	t.Parallel()

	result := Result{Statements: []StatementResult{
		{Index: 0, Findings: []rule.Finding{
			{RuleID: "ddl.one", Level: rule.LevelBlocker, Message: "1"},
			{RuleID: "ddl.two", Level: rule.LevelBlocker, Message: "2"},
			{RuleID: "ddl.three", Level: rule.LevelBlocker, Message: "3"},
		}},
	}}

	summary := BuildActionSummary(result, nil, ActionSummaryOptions{Limit: 0})

	if len(summary.Items) != 3 {
		t.Fatalf("expected all 3 items, got %d", len(summary.Items))
	}
	if summary.TotalItems != 3 {
		t.Fatalf("expected total 3, got %d", summary.TotalItems)
	}
}

func TestBuildActionSummaryDeterministic(t *testing.T) {
	t.Parallel()

	result := Result{Statements: []StatementResult{
		{Index: 0, Findings: []rule.Finding{
			{RuleID: "ddl.b", Level: rule.LevelWarning, Message: "b"},
			{RuleID: "ddl.a", Level: rule.LevelWarning, Message: "a"},
			{RuleID: "ddl.c", Level: rule.LevelBlocker, Message: "c"},
		}},
		{Index: 1, Findings: []rule.Finding{
			{RuleID: "ddl.a", Level: rule.LevelWarning, Message: "a2"},
		}},
	}}

	var first []string
	for run := 0; run < 5; run++ {
		summary := BuildActionSummary(result, nil, ActionSummaryOptions{})
		got := make([]string, len(summary.Items))
		for i, item := range summary.Items {
			got[i] = item.RuleID
		}
		if first == nil {
			first = got
			continue
		}
		if len(got) != len(first) {
			t.Fatalf("run %d: expected %d items, got %d", run, len(first), len(got))
		}
		for i := range first {
			if got[i] != first[i] {
				t.Fatalf("run %d: non-deterministic order, expected %v, got %v", run, first, got)
			}
		}
	}

	// Sanity check the deterministic order: blocker first, then warning by count desc then id asc.
	want := []string{"ddl.c", "ddl.a", "ddl.b"}
	for i := range want {
		if first[i] != want[i] {
			t.Fatalf("expected deterministic order %v, got %v", want, first)
		}
	}
}

func TestBuildActionSummaryNoSeverityInJSON(t *testing.T) {
	t.Parallel()

	result := Result{
		Verdict: VerdictReject,
		Summary: Summary{Statements: 1, Blockers: 1},
		Statements: []StatementResult{
			{Index: 0, Findings: []rule.Finding{
				{RuleID: "dml.where.require", Level: rule.LevelBlocker, Message: "where"},
			}},
		},
	}

	summary := BuildActionSummary(result, []catalog.Entry{{
		RuleID:     "dml.where.require",
		Summary:    "summary",
		Suggestion: "suggestion",
	}}, ActionSummaryOptions{})

	if encoded, err := json.Marshal(summary); err != nil {
		t.Fatalf("failed to marshal ActionSummary: %v", err)
	} else if strings.Contains(string(encoded), "severity") {
		t.Fatalf("ActionSummary JSON must not contain severity, got %s", encoded)
	}

	if len(summary.Items) == 0 {
		t.Fatal("expected at least one item to marshal")
	}
	if encoded, err := json.Marshal(summary.Items[0]); err != nil {
		t.Fatalf("failed to marshal ActionItem: %v", err)
	} else if strings.Contains(string(encoded), "severity") {
		t.Fatalf("ActionItem JSON must not contain severity, got %s", encoded)
	}

	if encoded, err := json.Marshal(result); err != nil {
		t.Fatalf("failed to marshal Result: %v", err)
	} else if strings.Contains(string(encoded), "severity") {
		t.Fatalf("Result JSON must not contain severity, got %s", encoded)
	}
}

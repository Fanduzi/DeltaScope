// Package catalog verifies rule catalog metadata coverage.
// input: built-in policy defaults plus catalog lookups and search queries, including default-disabled opt-in rules
// output: regression coverage for catalog completeness, lookup stability, and metadata-aware flags
// pos: explanation-oriented rule catalog test coverage
// note: if this file changes, update this header and module README.md.
package catalog

import (
	"testing"

	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func TestAllCoversEveryDefaultRule(t *testing.T) {
	t.Parallel()
	defaults := domainpolicy.Default()
	items := All()

	if len(items) < len(defaults.Rules) {
		t.Fatalf("expected catalog to cover %d default rules, got %d", len(defaults.Rules), len(items))
	}
	seen := make(map[string]bool, len(items))
	for _, entry := range items {
		if entry.RuleID == "" || entry.Summary == "" || entry.Description == "" {
			t.Fatalf("expected non-empty core metadata, got %#v", entry)
		}
		if entry.TriggerExample == "" || entry.ValidExample == "" || entry.ConfigExample == "" || entry.RemediationHint == "" {
			t.Fatalf("expected examples and remediation for %s", entry.RuleID)
		}
		seen[entry.RuleID] = true
	}
	for ruleID := range defaults.Rules {
		if !seen[ruleID] {
			t.Fatalf("expected catalog to include default rule %q", ruleID)
		}
	}
}

func TestLookupReturnsKnownRuleMetadata(t *testing.T) {
	t.Parallel()
	entry, ok := Lookup("dml.where.require")
	if !ok {
		t.Fatalf("expected dml.where.require to exist")
	}
	if entry.DefaultLevel == "" || len(entry.StatementKinds) == 0 {
		t.Fatalf("expected default level and kinds, got %#v", entry)
	}
	if entry.Why == "" || entry.Risk == "" || entry.Suggestion == "" {
		t.Fatalf("expected explanation-oriented metadata, got %#v", entry)
	}
	if len(entry.ConfigHints) == 0 {
		t.Fatalf("expected config hints, got %#v", entry)
	}
	if entry.MetadataAware {
		t.Fatalf("expected dml.where.require to stay offline-safe")
	}
}

func TestLookupMarksMetadataAwareRules(t *testing.T) {
	t.Parallel()
	for _, ruleID := range []string{"ddl.table.exists.create.forbid", "dml.table.exists.require"} {
		entry, ok := Lookup(ruleID)
		if !ok {
			t.Fatalf("expected metadata-aware rule %q to exist", ruleID)
		}
		if !entry.MetadataAware {
			t.Fatalf("expected %s to be metadata-aware", ruleID)
		}
		if entry.MetadataNotes == nil {
			t.Fatalf("expected %s to expose metadata notes", ruleID)
		}
		if entry.MetadataNotes.Required == "" || entry.MetadataNotes.Missing == "" {
			t.Fatalf("expected metadata notes to explain required and missing states, got %#v", entry.MetadataNotes)
		}
	}

	entry, ok := Lookup("ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory")
	if !ok || !entry.MetadataAware {
		t.Fatalf("expected unknown-prior-state nullability advisory to be metadata-aware, got %#v", entry)
	}
	if len(entry.Dialects) != 2 || entry.Dialects[0] != "mysql" || entry.Dialects[1] != "tidb" {
		t.Fatalf("expected advisory dialects [mysql tidb], got %#v", entry.Dialects)
	}
}

func TestSearchMatchesRuleIDAndSummaryText(t *testing.T) {
	t.Parallel()
	results := Search("where")
	if len(results) == 0 {
		t.Fatalf("expected search results")
	}
	found := false
	for _, entry := range results {
		if entry.RuleID == "dml.where.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected dml.where.require in search results: %#v", results)
	}
}

func TestAllIncludesExplanationMetadataForShippedRules(t *testing.T) {
	t.Parallel()
	for _, entry := range All() {
		if entry.Why == "" || entry.Risk == "" || entry.Suggestion == "" {
			t.Fatalf("expected explanation metadata for %s, got %#v", entry.RuleID, entry)
		}
		if len(entry.ConfigHints) == 0 {
			t.Fatalf("expected config hints for %s, got %#v", entry.RuleID, entry)
		}
	}
}

func TestLookupReturnsIndependentCopies(t *testing.T) {
	t.Parallel()
	entry, ok := Lookup("dml.where.require")
	if !ok {
		t.Fatalf("expected dml.where.require to exist")
	}
	entry.StatementKinds[0] = "mutated"
	entry.ConfigHints[0] = "mutated"
	entry.DefaultParams["required"] = false

	again, ok := Lookup("dml.where.require")
	if !ok {
		t.Fatalf("expected dml.where.require to exist on second lookup")
	}
	if again.StatementKinds[0] == "mutated" {
		t.Fatalf("expected statement kinds to be defensive copies, got %#v", again.StatementKinds)
	}
	if again.ConfigHints[0] == "mutated" {
		t.Fatalf("expected config hints to be defensive copies, got %#v", again.ConfigHints)
	}
	if value, ok := again.DefaultParams["required"]; ok && value == false {
		t.Fatalf("expected default params to be defensive copies, got %#v", again.DefaultParams)
	}
}

func TestSearchReturnsIndependentCopies(t *testing.T) {
	t.Parallel()
	results := Search("where")
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	results[0].StatementKinds[0] = "mutated"
	results[0].ConfigHints[0] = "mutated"

	again := Search("where")
	if len(again) == 0 {
		t.Fatal("expected search results on second search")
	}
	if again[0].StatementKinds[0] == "mutated" {
		t.Fatalf("expected search statement kinds to be defensive copies, got %#v", again[0].StatementKinds)
	}
	if again[0].ConfigHints[0] == "mutated" {
		t.Fatalf("expected search config hints to be defensive copies, got %#v", again[0].ConfigHints)
	}
}

func TestLookupDeepCopiesSliceDefaultParams(t *testing.T) {
	t.Parallel()
	entry, ok := Lookup("ddl.table.charset.allowlist")
	if !ok {
		t.Fatalf("expected ddl.table.charset.allowlist to exist")
	}
	values, ok := entry.DefaultParams["values"].([]string)
	if !ok || len(values) == 0 {
		t.Fatalf("expected values slice param, got %#v", entry.DefaultParams["values"])
	}
	values[0] = "mutated"

	again, ok := Lookup("ddl.table.charset.allowlist")
	if !ok {
		t.Fatalf("expected ddl.table.charset.allowlist to exist on second lookup")
	}
	againValues, ok := again.DefaultParams["values"].([]string)
	if !ok || len(againValues) == 0 {
		t.Fatalf("expected values slice param on second lookup, got %#v", again.DefaultParams["values"])
	}
	if againValues[0] == "mutated" {
		t.Fatalf("expected slice params to be defensive copies, got %#v", againValues)
	}
}

func TestFormatYAMLScalarQuotesStringLists(t *testing.T) {
	t.Parallel()
	got := formatYAMLScalar([]string{"utf8mb4", "needs:quote", "two words"})
	want := `["utf8mb4", "needs:quote", "two words"]`
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestLookupReturnsDefaultDisabledImpactRules(t *testing.T) {
	t.Parallel()
	defaults := domainpolicy.Default()
	for _, tc := range []struct {
		ruleID string
		level  string
	}{
		{ruleID: "dml.impact.estimate", level: "notice"},
		{ruleID: "dml.impact.rows.max_count", level: "warning"},
		{ruleID: "dml.impact.ratio.max_percent", level: "warning"},
	} {
		if _, ok := defaults.Rules[tc.ruleID]; ok {
			t.Fatalf("Default Policy must not include %q", tc.ruleID)
		}
		entry, ok := Lookup(tc.ruleID)
		if !ok {
			t.Fatalf("expected catalog to include %q", tc.ruleID)
		}
		if entry.DefaultEnabled {
			t.Fatalf("expected %s to be default-disabled", tc.ruleID)
		}
		if string(entry.DefaultLevel) != tc.level {
			t.Fatalf("expected %s default level %s, got %s", tc.ruleID, tc.level, entry.DefaultLevel)
		}
		if entry.ConfigKey != tc.ruleID {
			t.Fatalf("expected config key %s, got %s", tc.ruleID, entry.ConfigKey)
		}
	}
}

func TestSearchFindsDefaultDisabledImpactRules(t *testing.T) {
	t.Parallel()
	results := Search("dml.impact")
	found := map[string]bool{}
	for _, entry := range results {
		if entry.DefaultEnabled {
			t.Fatalf("expected search hit %s to stay default-disabled", entry.RuleID)
		}
		found[entry.RuleID] = true
	}
	for _, ruleID := range []string{
		"dml.impact.estimate",
		"dml.impact.rows.max_count",
		"dml.impact.ratio.max_percent",
	} {
		if !found[ruleID] {
			t.Fatalf("expected Search(dml.impact) to include %q", ruleID)
		}
	}
}

func TestCatalogIncludesShippedRulesOutsideDefaultPolicy(t *testing.T) {
	t.Parallel()
	defaults := domainpolicy.Default()
	items := All()
	if len(items) <= len(defaults.Rules) {
		t.Fatalf("catalog generation must not equal Default Policy keys only: catalog=%d policy=%d", len(items), len(defaults.Rules))
	}
}

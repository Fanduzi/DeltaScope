// Package catalog verifies rule catalog metadata coverage.
// input: built-in policy defaults plus catalog lookups and search queries
// output: regression coverage for catalog completeness, lookup stability, and metadata-aware flags
// pos: explanation-oriented rule catalog test coverage
// note: if this file changes, update this header and module README.md.
package catalog

import (
	"testing"

	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func TestAllCoversEveryDefaultRule(t *testing.T) {
	defaults := domainpolicy.Default()
	items := All()

	if len(items) != len(defaults.Rules) {
		t.Fatalf("expected %d catalog entries, got %d", len(defaults.Rules), len(items))
	}
	for _, entry := range items {
		if entry.RuleID == "" || entry.Summary == "" || entry.Description == "" {
			t.Fatalf("expected non-empty core metadata, got %#v", entry)
		}
		if entry.TriggerExample == "" || entry.ValidExample == "" || entry.ConfigExample == "" || entry.RemediationHint == "" {
			t.Fatalf("expected examples and remediation for %s", entry.RuleID)
		}
	}
}

func TestLookupReturnsKnownRuleMetadata(t *testing.T) {
	entry, ok := Lookup("dml.where.require")
	if !ok {
		t.Fatalf("expected dml.where.require to exist")
	}
	if entry.DefaultLevel == "" || len(entry.StatementKinds) == 0 {
		t.Fatalf("expected default level and kinds, got %#v", entry)
	}
	if entry.MetadataAware {
		t.Fatalf("expected dml.where.require to stay offline-safe")
	}
}

func TestLookupMarksMetadataAwareRules(t *testing.T) {
	entry, ok := Lookup("ddl.table.exists.create.forbid")
	if !ok {
		t.Fatalf("expected metadata-aware rule to exist")
	}
	if !entry.MetadataAware {
		t.Fatalf("expected ddl.table.exists.create.forbid to be metadata-aware")
	}
}

func TestSearchMatchesRuleIDAndSummaryText(t *testing.T) {
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

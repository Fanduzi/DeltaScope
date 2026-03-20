// Package ddl verifies create-table index rule behavior.
// input: synthetic create-table statements with typed index metadata and rule-specific policy overrides
// output: focused coverage for index count, prefix, and duplicate-index findings
// pos: domain DDL rule test coverage for index governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestIndexTotalMaxCountRuleFindsOversizedIndexCatalog(t *testing.T) {
	statement := statementWithIndexes(
		spec.Index{Name: "idx_a", Kind: spec.IndexKindSecondary, Columns: []string{"a"}},
		spec.Index{Name: "idx_b", Kind: spec.IndexKindSecondary, Columns: []string{"b"}},
		spec.Index{Name: "uniq_c", Kind: spec.IndexKindUnique, Columns: []string{"c"}},
	)

	statementRule, err := newIndexTotalMaxCountRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"limit": 2},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestIndexColumnsMaxCountRuleFindsWideIndexes(t *testing.T) {
	statement := statementWithIndexes(spec.Index{
		Name:    "idx_wide",
		Kind:    spec.IndexKindSecondary,
		Columns: []string{"a", "b", "c"},
	})

	statementRule, err := newIndexColumnsMaxCountRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"limit": 2},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestUniquePrefixRuleFindsBadNames(t *testing.T) {
	statement := statementWithIndexes(spec.Index{Name: "user_email", Kind: spec.IndexKindUnique, Columns: []string{"email"}})
	statementRule, err := newIndexPrefixRequiredRule(ruleIDIndexUniquePrefixRequire, spec.IndexKindUnique, "uniq_", rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"required": true, "prefix": "uniq_"},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestSecondaryPrefixRuleFindsBadNames(t *testing.T) {
	statement := statementWithIndexes(spec.Index{Name: "name_lookup", Kind: spec.IndexKindSecondary, Columns: []string{"name"}})
	statementRule, err := newIndexPrefixRequiredRule(ruleIDIndexSecondaryPrefixRequire, spec.IndexKindSecondary, "idx_", rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"required": true, "prefix": "idx_"},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestFulltextPrefixRuleFindsBadNames(t *testing.T) {
	statement := statementWithIndexes(spec.Index{Name: "search_body", Kind: spec.IndexKindFulltext, Columns: []string{"body"}})
	statementRule, err := newIndexPrefixRequiredRule(ruleIDIndexFulltextPrefixRequire, spec.IndexKindFulltext, "full_", rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"required": true, "prefix": "full_"},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestDuplicateIndexRuleFindsExactDuplicateIndexes(t *testing.T) {
	statement := statementWithIndexes(
		spec.Index{Name: "idx_name", Kind: spec.IndexKindSecondary, Columns: []string{"name"}},
		spec.Index{Name: "idx_name_copy", Kind: spec.IndexKindSecondary, Columns: []string{"name"}},
		spec.Index{Name: "uniq_name", Kind: spec.IndexKindUnique, Columns: []string{"name"}},
	)

	statementRule, err := newDuplicateIndexForbiddenRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["duplicate"] != "idx_name" {
		t.Fatalf("expected duplicate metadata to point at idx_name, got %+v", findings[0].Metadata)
	}
}

func statementWithIndexes(indexes ...spec.Index) spec.Statement {
	return spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table:   &spec.Table{Name: "users"},
			Columns: []spec.Column{{Name: "id", Type: "bigint", Comment: "'id'", NotNull: true, HasDefault: true}},
			Indexes: indexes,
		},
	}
}

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

func TestIndexSuffixRuleFindsBadNames(t *testing.T) {
	tests := []struct {
		name      string
		ruleID    string
		kind      spec.IndexKind
		suffix    string
		indexName string
	}{
		{
			name:      "unique",
			ruleID:    ruleIDIndexUniqueSuffixRequire,
			kind:      spec.IndexKindUnique,
			suffix:    "_uniq",
			indexName: "uniq_user_email",
		},
		{
			name:      "secondary",
			ruleID:    ruleIDIndexSecondarySuffixRequire,
			kind:      spec.IndexKindSecondary,
			suffix:    "_idx",
			indexName: "idx_user_email",
		},
		{
			name:      "fulltext",
			ruleID:    ruleIDIndexFulltextSuffixRequire,
			kind:      spec.IndexKindFulltext,
			suffix:    "_fts",
			indexName: "full_user_email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement := statementWithIndexes(spec.Index{Name: tt.indexName, Kind: tt.kind, Columns: []string{"email"}})
			statementRule, err := newIndexSuffixRequiredRule(tt.ruleID, tt.kind, rule.LevelWarning, policy.RulePolicy{
				Enabled: true,
				Level:   rule.LevelWarning,
				Params:  map[string]any{"suffix": tt.suffix},
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
			if findings[0].RuleID != tt.ruleID {
				t.Fatalf("expected rule id %q, got %q", tt.ruleID, findings[0].RuleID)
			}
			if got := findings[0].Metadata["suffix"]; got != tt.suffix {
				t.Fatalf("expected suffix metadata %q, got %#v", tt.suffix, got)
			}
		})
	}
}

func TestIndexContainsRuleUsesORSemantics(t *testing.T) {
	tests := []struct {
		name      string
		ruleID    string
		kind      spec.IndexKind
		indexes   []spec.Index
		contains  []string
		wantCount int
	}{
		{
			name:   "unique",
			ruleID: ruleIDIndexUniqueContainsRequire,
			kind:   spec.IndexKindUnique,
			indexes: []spec.Index{
				{Name: "uniq_user_email", Kind: spec.IndexKindUnique, Columns: []string{"email"}},
				{Name: "uniq_login", Kind: spec.IndexKindUnique, Columns: []string{"login"}},
			},
			contains:  []string{"user", "account"},
			wantCount: 1,
		},
		{
			name:   "secondary",
			ruleID: ruleIDIndexSecondaryContainsRequire,
			kind:   spec.IndexKindSecondary,
			indexes: []spec.Index{
				{Name: "idx_order_status", Kind: spec.IndexKindSecondary, Columns: []string{"status"}},
				{Name: "idx_lookup", Kind: spec.IndexKindSecondary, Columns: []string{"status"}},
			},
			contains:  []string{"order", "account"},
			wantCount: 1,
		},
		{
			name:   "fulltext",
			ruleID: ruleIDIndexFulltextContainsRequire,
			kind:   spec.IndexKindFulltext,
			indexes: []spec.Index{
				{Name: "full_search_body", Kind: spec.IndexKindFulltext, Columns: []string{"body"}},
				{Name: "full_lookup", Kind: spec.IndexKindFulltext, Columns: []string{"body"}},
			},
			contains:  []string{"search", "terms"},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statementRule, err := newIndexContainsRequiredRule(tt.ruleID, tt.kind, rule.LevelWarning, policy.RulePolicy{
				Enabled: true,
				Level:   rule.LevelWarning,
				Params:  map[string]any{"contains": tt.contains},
			})
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}

			findings, err := statementRule.Evaluate(statementWithIndexes(tt.indexes...))
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != tt.wantCount {
				t.Fatalf("expected %d findings, got %d", tt.wantCount, len(findings))
			}
			if tt.wantCount == 0 {
				return
			}
			if findings[0].RuleID != tt.ruleID {
				t.Fatalf("expected rule id %q, got %q", tt.ruleID, findings[0].RuleID)
			}
		})
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

func TestRedundantLeftPrefixIndexRuleFindsShorterSecondaryIndex(t *testing.T) {
	statement := statementWithIndexes(
		spec.Index{Name: "idx_name", Kind: spec.IndexKindSecondary, Columns: []string{"name"}},
		spec.Index{Name: "idx_name_status", Kind: spec.IndexKindSecondary, Columns: []string{"name", "status"}},
	)

	statementRule, err := newRedundantLeftPrefixIndexRule(policy.RulePolicy{
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
	if findings[0].Metadata["redundant"] != "idx_name_status" {
		t.Fatalf("expected covering index metadata to point at idx_name_status, got %+v", findings[0].Metadata)
	}
}

func TestRedundantUniqueOverlapIndexRuleFindsSecondaryShadowingUnique(t *testing.T) {
	statement := statementWithIndexes(
		spec.Index{Name: "idx_email", Kind: spec.IndexKindSecondary, Columns: []string{"email"}},
		spec.Index{Name: "uniq_email", Kind: spec.IndexKindUnique, Columns: []string{"email"}},
	)

	statementRule, err := newRedundantUniqueOverlapIndexRule(policy.RulePolicy{
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
	if findings[0].Metadata["redundant"] != "uniq_email" {
		t.Fatalf("expected unique overlap metadata to point at uniq_email, got %+v", findings[0].Metadata)
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

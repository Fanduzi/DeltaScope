// Package ddl verifies semantic alter rule behavior.
// input: synthetic alter-table statements with rename, add-index, target-type, and explicit-change semantic payloads plus policy overrides
// output: focused coverage for semantic alter rename, alter-added index governance, and target-type-family rules
// pos: domain DDL semantic alter rule and registration-path test coverage
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAlterTargetTypeFamilyAllowlistRuleBlocksDisallowedFamilies(t *testing.T) {
	t.Parallel()
	statementRule, err := newAlterTargetTypeFamilyRule(
		ruleIDAlterModifyColumnTargetTypeFamilyAllowlist,
		"modify_column",
		"modify column",
		rule.LevelBlocker,
		[]string{"integer", "decimal", "string", "binary", "time"},
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required":              true,
				"allowed_type_families": []string{"integer", "decimal", "string", "binary", "time"},
			},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "modify_column",
			Name:   "payload",
			Column: &spec.AlterColumn{
				Definition: &spec.Column{
					Name: "payload",
					Type: "json",
				},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDAlterModifyColumnTargetTypeFamilyAllowlist {
		t.Fatalf("expected rule id %q, got %q", ruleIDAlterModifyColumnTargetTypeFamilyAllowlist, findings[0].RuleID)
	}
	if got := findings[0].Metadata["type_family"]; got != "text" {
		t.Fatalf("expected type_family metadata text, got %#v", got)
	}
}

func TestAlterTargetTypeFamilyAllowlistRuleAllowsConfiguredFamilies(t *testing.T) {
	t.Parallel()
	statementRule, err := newAlterTargetTypeFamilyRule(
		ruleIDAlterChangeColumnTargetTypeFamilyAllowlist,
		"change_column",
		"change column",
		rule.LevelBlocker,
		[]string{"integer", "decimal", "string", "binary", "time"},
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required":              true,
				"allowed_type_families": []string{"integer", "decimal", "string", "binary", "time"},
			},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "change_column",
			Name:   "age",
			Column: &spec.AlterColumn{
				OldName: "age",
				Definition: &spec.Column{
					Name:     "age",
					Type:     "bigint(20) unsigned",
					Unsigned: true,
				},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestAlterColumnTransitionRuleBlocksConfiguredChanges(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		ruleID     string
		action     string
		label      string
		predicate  func(spec.Alter) bool
		changeKind string
		alter      spec.Alter
	}{
		{
			name:       "modify nullability",
			ruleID:     ruleIDAlterModifyColumnExplicitNullabilityChangeForbid,
			action:     "modify_column",
			label:      "modify column",
			predicate:  alterTouchesExplicitNullability,
			changeKind: "explicit_nullability_change",
			alter: spec.Alter{
				Action: "modify_column",
				Name:   "age",
				Column: &spec.AlterColumn{
					Definition: &spec.Column{Name: "age", Type: "bigint(20)"},
					Change:     &spec.AlterColumnChange{TouchesNullability: true},
				},
			},
		},
		{
			name:       "change default",
			ruleID:     ruleIDAlterChangeColumnExplicitDefaultChangeForbid,
			action:     "change_column",
			label:      "change column",
			predicate:  alterTouchesExplicitDefault,
			changeKind: "explicit_default_change",
			alter: spec.Alter{
				Action: "change_column",
				Name:   "legacy_name",
				Column: &spec.AlterColumn{
					OldName:    "legacy_name",
					Definition: &spec.Column{Name: "name", Type: "varchar(32)"},
					Change:     &spec.AlterColumnChange{TouchesDefault: true},
				},
			},
		},
		{
			name:       "change auto_increment",
			ruleID:     ruleIDAlterChangeColumnExplicitAutoIncrementChangeForbid,
			action:     "change_column",
			label:      "change column",
			predicate:  alterTouchesExplicitAutoIncrement,
			changeKind: "explicit_auto_increment_change",
			alter: spec.Alter{
				Action: "change_column",
				Name:   "legacy_id",
				Column: &spec.AlterColumn{
					OldName:    "legacy_id",
					Definition: &spec.Column{Name: "id", Type: "bigint(20)", AutoIncrement: true},
					Change:     &spec.AlterColumnChange{TouchesAutoIncrement: true},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
				tt.ruleID,
				tt.action,
				tt.label,
				tt.changeKind,
				rule.LevelBlocker,
				tt.predicate,
				policy.RulePolicy{
					Enabled: true,
					Level:   rule.LevelBlocker,
					Params: map[string]any{
						"forbid": true,
					},
				},
			)
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}

			findings, err := statementRule.Evaluate(context.Background(), alterStatement(tt.alter))
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d", len(findings))
			}
			if findings[0].RuleID != tt.ruleID {
				t.Fatalf("expected rule id %q, got %q", tt.ruleID, findings[0].RuleID)
			}
		})
	}
}

func TestAlterColumnTransitionRuleAllowsUntouchedChanges(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
		ruleIDAlterModifyColumnExplicitDefaultChangeForbid,
		"modify_column",
		"modify column",
		"explicit_default_change",
		rule.LevelBlocker,
		alterTouchesExplicitDefault,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "modify_column",
			Name:   "age",
			Column: &spec.AlterColumn{
				Definition: &spec.Column{Name: "age", Type: "bigint(20)"},
				Change:     &spec.AlterColumnChange{TouchesNullability: true},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestAlterAddedRedundantIndexRulesReuseLifecycleSnapshot(t *testing.T) {
	t.Parallel()
	statement := alterStatement(
		spec.Alter{
			Action: "add_index",
			Name:   "idx_email_created",
			Index: &spec.AlterIndex{
				Definition: &spec.Index{
					Name:    "idx_email_created",
					Kind:    spec.IndexKindSecondary,
					Columns: []string{"email", "created_at"},
				},
			},
		},
		spec.Alter{
			Action: "add_constraint",
			Name:   "uniq_email_status",
			Index: &spec.AlterIndex{
				Definition: &spec.Index{
					Name:    "uniq_email_status",
					Kind:    spec.IndexKindUnique,
					Columns: []string{"email", "status"},
				},
			},
		},
	)
	statement.Metadata = &spec.Metadata{
		TargetTable: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
			Indexes: []spec.Index{
				{Name: "idx_email", Kind: spec.IndexKindSecondary, Columns: []string{"email"}},
				{Name: "uniq_email", Kind: spec.IndexKindUnique, Columns: []string{"email"}},
			},
		},
	}

	leftPrefixRule, err := newAlterAddedRedundantLeftPrefixRule(rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new left-prefix rule: %v", err)
	}
	uniqueOverlapRule, err := newAlterAddedRedundantUniqueOverlapRule(rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new unique-overlap rule: %v", err)
	}

	leftFindings, err := leftPrefixRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate left-prefix rule: %v", err)
	}
	if len(leftFindings) != 1 || leftFindings[0].RuleID != ruleIDAlterAddIndexRedundantLeftPrefixForbid {
		t.Fatalf("expected left-prefix lifecycle finding, got %+v", leftFindings)
	}

	uniqueFindings, err := uniqueOverlapRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate unique-overlap rule: %v", err)
	}
	if len(uniqueFindings) != 1 || uniqueFindings[0].RuleID != ruleIDAlterAddIndexRedundantUniqueOverlapForbid {
		t.Fatalf("expected unique-overlap lifecycle finding, got %+v", uniqueFindings)
	}
}

func TestAlterRenameIndexRuleFindsForbiddenRename(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenAlterRenameRule(
		ruleIDAlterRenameIndexForbid,
		"rename_index",
		"rename index",
		rule.LevelBlocker,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "rename_index",
			Name:   "idx_old",
			Index: &spec.AlterIndex{
				OldName: "idx_old",
				Definition: &spec.Index{
					Name: "idx_new",
				},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDAlterRenameIndexForbid {
		t.Fatalf("expected rule id %q, got %q", ruleIDAlterRenameIndexForbid, findings[0].RuleID)
	}
	if got := findings[0].Metadata["new_name"]; got != "idx_new" {
		t.Fatalf("expected new_name metadata idx_new, got %#v", got)
	}
}

func TestAlterRenameIndexRuleFindsStandaloneForbiddenRename(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenAlterRenameRule(
		ruleIDAlterRenameIndexForbid,
		"rename_index",
		"rename index",
		rule.LevelBlocker,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Alter:     []spec.Alter{{Action: "rename_index", Name: "idx_old", Options: map[string]string{"new_name": "idx_new"}}},
		},
	}
	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDAlterRenameIndexForbid {
		t.Fatalf("expected rule id %q, got %q", ruleIDAlterRenameIndexForbid, findings[0].RuleID)
	}
	if got := findings[0].Metadata["old_name"]; got != "idx_old" {
		t.Fatalf("expected old_name metadata idx_old, got %#v", got)
	}
	if got := findings[0].Metadata["new_name"]; got != "idx_new" {
		t.Fatalf("expected new_name metadata idx_new, got %#v", got)
	}
	if _, ok := findings[0].Metadata["table"]; ok {
		t.Fatalf("expected standalone rename metadata to omit table, got %#v", findings[0].Metadata)
	}
	if findings[0].Message != "DDL rename index from \"idx_old\" to \"idx_new\" is forbidden" {
		t.Fatalf("expected standalone rename message, got %q", findings[0].Message)
	}
}

func TestAlterAddedIndexPrefixRuleFindsBadPrefixes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		ruleID     string
		kind       spec.IndexKind
		fallback   string
		indexName  string
		wantRuleID string
		wantPrefix string
	}{
		{
			name:       "unique",
			ruleID:     ruleIDAlterAddIndexUniquePrefixRequire,
			kind:       spec.IndexKindUnique,
			fallback:   "uniq_",
			indexName:  "user_email",
			wantRuleID: ruleIDAlterAddIndexUniquePrefixRequire,
			wantPrefix: "uniq_",
		},
		{
			name:       "secondary",
			ruleID:     ruleIDAlterAddIndexSecondaryPrefixRequire,
			kind:       spec.IndexKindSecondary,
			fallback:   "idx_",
			indexName:  "name_lookup",
			wantRuleID: ruleIDAlterAddIndexSecondaryPrefixRequire,
			wantPrefix: "idx_",
		},
		{
			name:       "fulltext",
			ruleID:     ruleIDAlterAddIndexFulltextPrefixRequire,
			kind:       spec.IndexKindFulltext,
			fallback:   "full_",
			indexName:  "search_body",
			wantRuleID: ruleIDAlterAddIndexFulltextPrefixRequire,
			wantPrefix: "full_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			statementRule, err := newAlterAddedIndexPrefixRule(tt.ruleID, tt.kind, tt.fallback, rule.LevelWarning, policy.RulePolicy{
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"required": true,
					"prefix":   tt.fallback,
				},
			})
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}

			findings, err := statementRule.Evaluate(context.Background(), alterStatement(
				spec.Alter{
					Action: "add_constraint",
					Name:   tt.indexName,
					Index: &spec.AlterIndex{
						Definition: &spec.Index{
							Name:    tt.indexName,
							Kind:    tt.kind,
							Columns: []string{"payload"},
						},
					},
				},
			))
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d", len(findings))
			}
			if findings[0].RuleID != tt.wantRuleID {
				t.Fatalf("expected rule id %q, got %q", tt.wantRuleID, findings[0].RuleID)
			}
			if got := findings[0].Metadata["prefix"]; got != tt.wantPrefix {
				t.Fatalf("expected prefix metadata %q, got %#v", tt.wantPrefix, got)
			}
		})
	}
}

func TestAlterAddedIndexPrefixRuleIgnoresNonAddConstraintAlters(t *testing.T) {
	t.Parallel()
	statementRule, err := newAlterAddedIndexPrefixRule(
		ruleIDAlterAddIndexSecondaryPrefixRequire,
		spec.IndexKindSecondary,
		"idx_",
		rule.LevelWarning,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"required": true,
				"prefix":   "idx_",
			},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "rename_index",
			Name:   "legacy_idx",
			Index: &spec.AlterIndex{
				OldName: "legacy_idx",
				Definition: &spec.Index{
					Name: "idx_legacy_idx",
					Kind: spec.IndexKindSecondary,
				},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestAlterAddedIndexSuffixRuleFindsBadNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		ruleID    string
		kind      spec.IndexKind
		suffix    string
		indexName string
	}{
		{
			name:      "unique",
			ruleID:    ruleIDAlterAddIndexUniqueSuffixRequire,
			kind:      spec.IndexKindUnique,
			suffix:    "_uniq",
			indexName: "uniq_email",
		},
		{
			name:      "secondary",
			ruleID:    ruleIDAlterAddIndexSecondarySuffixRequire,
			kind:      spec.IndexKindSecondary,
			suffix:    "_idx",
			indexName: "idx_email",
		},
		{
			name:      "fulltext",
			ruleID:    ruleIDAlterAddIndexFulltextSuffixRequire,
			kind:      spec.IndexKindFulltext,
			suffix:    "_fts",
			indexName: "full_email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			statementRule, err := newAlterAddedIndexSuffixRule(tt.ruleID, tt.kind, rule.LevelWarning, policy.RulePolicy{
				Enabled: true,
				Level:   rule.LevelWarning,
				Params:  map[string]any{"suffix": tt.suffix},
			})
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}

			findings, err := statementRule.Evaluate(context.Background(), alterStatement(
				spec.Alter{
					Action: "add_constraint",
					Name:   tt.indexName,
					Index: &spec.AlterIndex{
						Definition: &spec.Index{
							Name:    tt.indexName,
							Kind:    tt.kind,
							Columns: []string{"email"},
						},
					},
				},
			))
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

func TestAlterAddedIndexContainsRuleUsesORSemantics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		ruleID    string
		kind      spec.IndexKind
		indexName string
		contains  []string
	}{
		{
			name:      "unique",
			ruleID:    ruleIDAlterAddIndexUniqueContainsRequire,
			kind:      spec.IndexKindUnique,
			indexName: "uniq_login",
			contains:  []string{"user", "account"},
		},
		{
			name:      "secondary",
			ruleID:    ruleIDAlterAddIndexSecondaryContainsRequire,
			kind:      spec.IndexKindSecondary,
			indexName: "idx_lookup",
			contains:  []string{"order", "account"},
		},
		{
			name:      "fulltext",
			ruleID:    ruleIDAlterAddIndexFulltextContainsRequire,
			kind:      spec.IndexKindFulltext,
			indexName: "full_lookup",
			contains:  []string{"search", "terms"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			statementRule, err := newAlterAddedIndexContainsRule(tt.ruleID, tt.kind, rule.LevelWarning, policy.RulePolicy{
				Enabled: true,
				Level:   rule.LevelWarning,
				Params:  map[string]any{"contains": tt.contains},
			})
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}

			findings, err := statementRule.Evaluate(context.Background(), alterStatement(
				spec.Alter{
					Action: "add_constraint",
					Name:   tt.indexName,
					Index: &spec.AlterIndex{
						Definition: &spec.Index{
							Name:    tt.indexName,
							Kind:    tt.kind,
							Columns: []string{"email"},
						},
					},
				},
			))
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d", len(findings))
			}
			if findings[0].RuleID != tt.ruleID {
				t.Fatalf("expected rule id %q, got %q", tt.ruleID, findings[0].RuleID)
			}
		})
	}
}

func TestAlterAddedIndexColumnsMaxCountRuleBlocksWideAlterAddedIndexes(t *testing.T) {
	t.Parallel()
	statementRule, err := newAlterAddedIndexColumnsMaxCountRule(2, rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"limit": 2,
		},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "add_constraint",
			Name:   "idx_wide_payload",
			Index: &spec.AlterIndex{
				Definition: &spec.Index{
					Name:    "idx_wide_payload",
					Kind:    spec.IndexKindSecondary,
					Columns: []string{"a", "b", "c"},
				},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDAlterAddIndexColumnsMaxCount {
		t.Fatalf("expected rule id %q, got %q", ruleIDAlterAddIndexColumnsMaxCount, findings[0].RuleID)
	}
	if got := findings[0].Metadata["actual"]; got != 3 {
		t.Fatalf("expected actual metadata 3, got %#v", got)
	}
}

func TestAlterAddedDuplicateIndexRuleBlocksDuplicateAlterAddedIndexes(t *testing.T) {
	t.Parallel()
	statementRule, err := newAlterAddedDuplicateIndexForbiddenRule(rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"forbid": true,
		},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "add_constraint",
			Name:   "idx_email",
			Index: &spec.AlterIndex{
				Definition: &spec.Index{
					Name:    "idx_email",
					Kind:    spec.IndexKindSecondary,
					Columns: []string{"email"},
				},
			},
		},
		spec.Alter{
			Action: "add_constraint",
			Name:   "idx_email_lookup",
			Index: &spec.AlterIndex{
				Definition: &spec.Index{
					Name:    "idx_email_lookup",
					Kind:    spec.IndexKindSecondary,
					Columns: []string{"email"},
				},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDAlterAddIndexDuplicateForbid {
		t.Fatalf("expected rule id %q, got %q", ruleIDAlterAddIndexDuplicateForbid, findings[0].RuleID)
	}
}

func TestAlterRegisterAddsEnabledSemanticRulesInDeterministicOrder(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules[ruleIDAlterChangeColumnForbid] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"forbid": false,
		},
	}
	cfg.Rules[ruleIDAlterRenameIndexForbid] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"forbid": true,
		},
	}
	cfg.Rules[ruleIDAlterModifyColumnTargetTypeFamilyAllowlist] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"required":              true,
			"allowed_type_families": []string{"integer", "decimal", "string", "binary", "time"},
		},
	}
	cfg.Rules[ruleIDAlterChangeColumnTargetTypeFamilyAllowlist] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"required":              true,
			"allowed_type_families": []string{"integer", "decimal", "string", "binary", "time"},
		},
	}
	cfg.Rules[ruleIDAlterModifyColumnExplicitNullabilityChangeForbid] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"forbid": true,
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(context.Background(), alterStatement(
		spec.Alter{
			Action: "rename_index",
			Name:   "idx_old",
			Index: &spec.AlterIndex{
				OldName: "idx_old",
				Definition: &spec.Index{
					Name: "idx_new",
				},
			},
		},
		spec.Alter{
			Action: "modify_column",
			Name:   "payload",
			Column: &spec.AlterColumn{
				Definition: &spec.Column{
					Name: "payload",
					Type: "json",
				},
				Change: &spec.AlterColumnChange{
					TouchesNullability: true,
				},
			},
		},
		spec.Alter{
			Action: "change_column",
			Name:   "body",
			Column: &spec.AlterColumn{
				OldName: "body",
				Definition: &spec.Column{
					Name: "body",
					Type: "json",
				},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	wantIDs := []string{
		ruleIDAlterRenameIndexForbid,
		ruleIDAlterModifyColumnTargetTypeFamilyAllowlist,
		ruleIDAlterChangeColumnTargetTypeFamilyAllowlist,
		ruleIDAlterModifyColumnExplicitNullabilityChangeForbid,
	}
	if len(findings) != len(wantIDs) {
		t.Fatalf("expected %d findings, got %d", len(wantIDs), len(findings))
	}
	for i, want := range wantIDs {
		if findings[i].RuleID != want {
			t.Fatalf("expected finding %d to use rule %q, got %q", i, want, findings[i].RuleID)
		}
	}
}

func TestAlterRegisterAddsAlterAddedIndexPrefixRulesFromDefaultPolicy(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(context.Background(), alterStatement(
		spec.Alter{
			Action: "add_constraint",
			Name:   "user_email",
			Index: &spec.AlterIndex{
				Definition: &spec.Index{
					Name:    "user_email",
					Kind:    spec.IndexKindUnique,
					Columns: []string{"email"},
				},
			},
		},
		spec.Alter{
			Action: "add_constraint",
			Name:   "name_lookup",
			Index: &spec.AlterIndex{
				Definition: &spec.Index{
					Name:    "name_lookup",
					Kind:    spec.IndexKindSecondary,
					Columns: []string{"name"},
				},
			},
		},
		spec.Alter{
			Action: "add_constraint",
			Name:   "search_body",
			Index: &spec.AlterIndex{
				Definition: &spec.Index{
					Name:    "search_body",
					Kind:    spec.IndexKindFulltext,
					Columns: []string{"body"},
				},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	wantIDs := []string{
		ruleIDAlterAddIndexUniquePrefixRequire,
		ruleIDAlterAddIndexSecondaryPrefixRequire,
		ruleIDAlterAddIndexFulltextPrefixRequire,
	}
	if len(findings) != len(wantIDs) {
		t.Fatalf("expected %d findings, got %d", len(wantIDs), len(findings))
	}
	for i, want := range wantIDs {
		if findings[i].RuleID != want {
			t.Fatalf("expected finding %d to use rule %q, got %q", i, want, findings[i].RuleID)
		}
	}
}

// --- Phase 5 Slice E: PG explicit default change semantic rules ---

func TestPGSetDefaultExplicitDefaultChangeRule_FiresWhenTouchesDefault(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
		ruleIDAlterSetDefaultExplicitDefaultChangeForbid,
		"set_default",
		"set default",
		"explicit_default_change",
		rule.LevelBlocker,
		alterTouchesExplicitDefault,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{"forbid": true},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "set_default",
			Name:   "status",
			Column: &spec.AlterColumn{
				OldName: "status",
				Change:  &spec.AlterColumnChange{TouchesDefault: true},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDAlterSetDefaultExplicitDefaultChangeForbid {
		t.Fatalf("expected rule id %q, got %q", ruleIDAlterSetDefaultExplicitDefaultChangeForbid, findings[0].RuleID)
	}
}

func TestPGDropDefaultExplicitDefaultChangeRule_FiresWhenTouchesDefault(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
		ruleIDAlterDropDefaultExplicitDefaultChangeForbid,
		"drop_default",
		"drop default",
		"explicit_default_change",
		rule.LevelBlocker,
		alterTouchesExplicitDefault,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{"forbid": true},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "drop_default",
			Name:   "email",
			Column: &spec.AlterColumn{
				OldName: "email",
				Change:  &spec.AlterColumnChange{TouchesDefault: true},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDAlterDropDefaultExplicitDefaultChangeForbid {
		t.Fatalf("expected rule id %q, got %q", ruleIDAlterDropDefaultExplicitDefaultChangeForbid, findings[0].RuleID)
	}
}

func TestPGSetDefaultExplicitDefaultChangeRule_SkipsWhenTouchesDefaultFalse(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
		ruleIDAlterSetDefaultExplicitDefaultChangeForbid,
		"set_default",
		"set default",
		"explicit_default_change",
		rule.LevelBlocker,
		alterTouchesExplicitDefault,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{"forbid": true},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "set_default",
			Name:   "status",
			Column: &spec.AlterColumn{
				OldName: "status",
				Change:  &spec.AlterColumnChange{TouchesDefault: false},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when TouchesDefault=false, got %d", len(findings))
	}
}

func TestPGDropDefaultExplicitDefaultChangeRule_SkipsWhenTouchesDefaultFalse(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
		ruleIDAlterDropDefaultExplicitDefaultChangeForbid,
		"drop_default",
		"drop default",
		"explicit_default_change",
		rule.LevelBlocker,
		alterTouchesExplicitDefault,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{"forbid": true},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "drop_default",
			Name:   "email",
			Column: &spec.AlterColumn{
				OldName: "email",
				Change:  &spec.AlterColumnChange{TouchesDefault: false},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when TouchesDefault=false, got %d", len(findings))
	}
}

func TestPGExplicitDefaultChangeRule_SkipsWhenForbidFalse(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
		ruleIDAlterSetDefaultExplicitDefaultChangeForbid,
		"set_default",
		"set default",
		"explicit_default_change",
		rule.LevelBlocker,
		alterTouchesExplicitDefault,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{"forbid": false},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "set_default",
			Name:   "status",
			Column: &spec.AlterColumn{
				OldName: "status",
				Change:  &spec.AlterColumnChange{TouchesDefault: true},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when forbid:false, got %d", len(findings))
	}
}

func TestPGExplicitDefaultChangeRule_MySQLModifyColumnDoesNotTrigger(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
		ruleIDAlterSetDefaultExplicitDefaultChangeForbid,
		"set_default",
		"set default",
		"explicit_default_change",
		rule.LevelBlocker,
		alterTouchesExplicitDefault,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{"forbid": true},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	// MySQL modify_column action — set_default rule must not match
	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "modify_column",
			Name:   "status",
			Column: &spec.AlterColumn{
				OldName: "status",
				Change:  &spec.AlterColumnChange{TouchesDefault: true},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for MySQL modify_column action, got %d", len(findings))
	}
}

func TestAlterRegisterAddsAlterAddedIndexLifecycleRulesWhenEnabled(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules[ruleIDAlterAddIndexColumnsMaxCount] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"limit": 1,
		},
	}
	cfg.Rules[ruleIDAlterAddIndexDuplicateForbid] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"forbid": true,
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(context.Background(), alterStatement(
		spec.Alter{
			Action: "add_constraint",
			Name:   "idx_email",
			Index: &spec.AlterIndex{
				Definition: &spec.Index{
					Name:    "idx_email",
					Kind:    spec.IndexKindSecondary,
					Columns: []string{"email", "tenant_id"},
				},
			},
		},
		spec.Alter{
			Action: "add_constraint",
			Name:   "idx_email_dup",
			Index: &spec.AlterIndex{
				Definition: &spec.Index{
					Name:    "idx_email_dup",
					Kind:    spec.IndexKindSecondary,
					Columns: []string{"email", "tenant_id"},
				},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	wantIDs := []string{
		ruleIDAlterAddIndexColumnsMaxCount,
		ruleIDAlterAddIndexColumnsMaxCount,
		ruleIDAlterAddIndexDuplicateForbid,
	}
	if len(findings) != len(wantIDs) {
		t.Fatalf("expected %d findings, got %d", len(wantIDs), len(findings))
	}
	for i, want := range wantIDs {
		if findings[i].RuleID != want {
			t.Fatalf("expected finding %d to use rule %q, got %q", i, want, findings[i].RuleID)
		}
	}
}

// --- Phase 5 Slice F: PG explicit nullability change semantic rules ---

func TestPGSetNotNullExplicitNullabilityChangeRule_FiresWhenTouchesNullability(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
		ruleIDAlterSetNotNullExplicitNullabilityChangeForbid,
		"set_not_null",
		"set not null",
		"explicit_nullability_change",
		rule.LevelBlocker,
		alterTouchesExplicitNullability,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{"forbid": true},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "set_not_null",
			Name:   "email",
			Column: &spec.AlterColumn{
				OldName: "email",
				Change:  &spec.AlterColumnChange{TouchesNullability: true},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDAlterSetNotNullExplicitNullabilityChangeForbid {
		t.Fatalf("expected rule id %q, got %q", ruleIDAlterSetNotNullExplicitNullabilityChangeForbid, findings[0].RuleID)
	}
}

func TestPGDropNotNullExplicitNullabilityChangeRule_FiresWhenTouchesNullability(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
		ruleIDAlterDropNotNullExplicitNullabilityChangeForbid,
		"drop_not_null",
		"drop not null",
		"explicit_nullability_change",
		rule.LevelBlocker,
		alterTouchesExplicitNullability,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{"forbid": true},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "drop_not_null",
			Name:   "phone",
			Column: &spec.AlterColumn{
				OldName: "phone",
				Change:  &spec.AlterColumnChange{TouchesNullability: true},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDAlterDropNotNullExplicitNullabilityChangeForbid {
		t.Fatalf("expected rule id %q, got %q", ruleIDAlterDropNotNullExplicitNullabilityChangeForbid, findings[0].RuleID)
	}
}

func TestPGSetNotNullExplicitNullabilityChangeRule_SkipsWhenTouchesNullabilityFalse(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
		ruleIDAlterSetNotNullExplicitNullabilityChangeForbid,
		"set_not_null",
		"set not null",
		"explicit_nullability_change",
		rule.LevelBlocker,
		alterTouchesExplicitNullability,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{"forbid": true},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "set_not_null",
			Name:   "email",
			Column: &spec.AlterColumn{
				OldName: "email",
				Change:  &spec.AlterColumnChange{TouchesNullability: false},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when TouchesNullability=false, got %d", len(findings))
	}
}

func TestPGDropNotNullExplicitNullabilityChangeRule_SkipsWhenTouchesNullabilityFalse(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
		ruleIDAlterDropNotNullExplicitNullabilityChangeForbid,
		"drop_not_null",
		"drop not null",
		"explicit_nullability_change",
		rule.LevelBlocker,
		alterTouchesExplicitNullability,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{"forbid": true},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "drop_not_null",
			Name:   "phone",
			Column: &spec.AlterColumn{
				OldName: "phone",
				Change:  &spec.AlterColumnChange{TouchesNullability: false},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when TouchesNullability=false, got %d", len(findings))
	}
}

func TestPGExplicitNullabilityChangeRule_SkipsWhenForbidFalse(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
		ruleIDAlterSetNotNullExplicitNullabilityChangeForbid,
		"set_not_null",
		"set not null",
		"explicit_nullability_change",
		rule.LevelBlocker,
		alterTouchesExplicitNullability,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{"forbid": false},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "set_not_null",
			Name:   "email",
			Column: &spec.AlterColumn{
				OldName: "email",
				Change:  &spec.AlterColumnChange{TouchesNullability: true},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when forbid:false, got %d", len(findings))
	}
}

func TestPGExplicitNullabilityChangeRule_MySQLModifyColumnDoesNotTrigger(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenExplicitAlterColumnChangeRule(
		ruleIDAlterSetNotNullExplicitNullabilityChangeForbid,
		"set_not_null",
		"set not null",
		"explicit_nullability_change",
		rule.LevelBlocker,
		alterTouchesExplicitNullability,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{"forbid": true},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	// MySQL modify_column action — set_not_null rule must not match
	findings, err := statementRule.Evaluate(context.Background(), alterStatement(
		spec.Alter{
			Action: "modify_column",
			Name:   "email",
			Column: &spec.AlterColumn{
				OldName: "email",
				Change:  &spec.AlterColumnChange{TouchesNullability: true},
			},
		},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for MySQL modify_column action, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// v0.39.0 Task 1: Rule applicability — PostgreSQL ALTER TABLE ADD CONSTRAINT
// ---------------------------------------------------------------------------

// TestPostgreSQLAlterTableAddUniquePrefixRuleCoverage proves that the unique
// index prefix rule fires on the PostgreSQL ALTER TABLE ADD CONSTRAINT shape
// after Task 2 constraint option projection.
func TestPostgreSQLAlterTableAddUniquePrefixRuleCoverage(t *testing.T) {
	t.Parallel()
	statementRule, err := newAlterAddedIndexPrefixRule(
		ruleIDAlterAddIndexUniquePrefixRequire,
		spec.IndexKindUnique,
		"uniq_",
		rule.LevelWarning,
		policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"required": true,
				"prefix":   "uniq_",
			},
		},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{{
				Action: "add_constraint",
				Name:   "bad_email_key",
				Options: map[string]string{
					"constraint_type": "unique",
					"columns":         "email",
				},
			}},
		},
	}

	if !statementRule.AppliesTo(statement) {
		t.Fatal("expected rule to apply to ALTER TABLE ADD CONSTRAINT UNIQUE")
	}

	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != ruleIDAlterAddIndexUniquePrefixRequire {
		t.Fatalf("expected rule_id %s, got %s", ruleIDAlterAddIndexUniquePrefixRequire, findings[0].RuleID)
	}
}

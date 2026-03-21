// Package ddl verifies semantic alter rule behavior.
// input: synthetic alter-table statements with rename, add-index, target-type, and explicit-change semantic payloads plus policy overrides
// output: focused coverage for semantic alter rename, alter-added index governance, and target-type-family rules
// pos: domain DDL semantic alter rule and registration-path test coverage
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAlterTargetTypeFamilyAllowlistRuleBlocksDisallowedFamilies(t *testing.T) {
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

	findings, err := statementRule.Evaluate(alterStatement(
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

	findings, err := statementRule.Evaluate(alterStatement(
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

			findings, err := statementRule.Evaluate(alterStatement(tt.alter))
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

	findings, err := statementRule.Evaluate(alterStatement(
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

func TestAlterRenameIndexRuleFindsForbiddenRename(t *testing.T) {
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

	findings, err := statementRule.Evaluate(alterStatement(
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

func TestAlterAddedIndexPrefixRuleFindsBadPrefixes(t *testing.T) {
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

			findings, err := statementRule.Evaluate(alterStatement(
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

	findings, err := statementRule.Evaluate(alterStatement(
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

func TestAlterAddedIndexColumnsMaxCountRuleBlocksWideAlterAddedIndexes(t *testing.T) {
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

	findings, err := statementRule.Evaluate(alterStatement(
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

	findings, err := statementRule.Evaluate(alterStatement(
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

	findings, err := registry.EvaluateStatement(alterStatement(
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
	registry := rule.NewRegistry()
	cfg := policy.Default()

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(alterStatement(
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

func TestAlterRegisterAddsAlterAddedIndexLifecycleRulesWhenEnabled(t *testing.T) {
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

	findings, err := registry.EvaluateStatement(alterStatement(
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

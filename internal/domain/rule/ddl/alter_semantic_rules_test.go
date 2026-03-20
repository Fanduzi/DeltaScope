// Package ddl verifies semantic alter rule behavior.
// input: synthetic alter-table statements with rename and target-type semantic payloads plus policy overrides
// output: focused coverage for semantic alter rename and target-type-family governance
// pos: domain DDL semantic alter rule test coverage
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAlterTargetTypeFamilyRuleBlocksDisallowedFamilies(t *testing.T) {
	statementRule, err := newAlterTargetTypeFamilyRule(
		ruleIDAlterModifyColumnCompatibleRequire,
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
	if findings[0].RuleID != ruleIDAlterModifyColumnCompatibleRequire {
		t.Fatalf("expected rule id %q, got %q", ruleIDAlterModifyColumnCompatibleRequire, findings[0].RuleID)
	}
	if got := findings[0].Metadata["type_family"]; got != "text" {
		t.Fatalf("expected type_family metadata text, got %#v", got)
	}
}

func TestAlterTargetTypeFamilyRuleAllowsConfiguredFamilies(t *testing.T) {
	statementRule, err := newAlterTargetTypeFamilyRule(
		ruleIDAlterChangeColumnCompatibleRequire,
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

func TestRegisterAddsEnabledAlterSemanticRulesInDeterministicOrder(t *testing.T) {
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
	cfg.Rules[ruleIDAlterModifyColumnCompatibleRequire] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"required":              true,
			"allowed_type_families": []string{"integer", "decimal", "string", "binary", "time"},
		},
	}
	cfg.Rules[ruleIDAlterChangeColumnCompatibleRequire] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"required":              true,
			"allowed_type_families": []string{"integer", "decimal", "string", "binary", "time"},
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
		ruleIDAlterModifyColumnCompatibleRequire,
		ruleIDAlterChangeColumnCompatibleRequire,
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

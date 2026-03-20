// Package ddl verifies alter-action restriction behavior.
// input: synthetic alter-table statements with normalized alter actions and rule-specific policy overrides
// output: focused coverage for action-level ALTER TABLE restrictions
// pos: domain DDL rule test coverage for alter governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestForbiddenAlterActionRuleFindsForbiddenActions(t *testing.T) {
	statement := alterStatement(
		spec.Alter{Action: "drop_column", Name: "old_name"},
		spec.Alter{Action: "rename_table", Name: "users_archive"},
	)

	statementRule, err := newForbiddenAlterActionRule(ruleIDAlterDropColumnForbid, "drop_column", "drop column", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
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
}

func TestForbiddenAlterActionRuleSkipsAllowedPolicies(t *testing.T) {
	statement := alterStatement(spec.Alter{Action: "rename_column", Name: "legacy_name"})

	statementRule, err := newForbiddenAlterActionRule(ruleIDAlterRenameColumnForbid, "rename_column", "rename column", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": false},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func alterStatement(alters ...spec.Alter) spec.Statement {
	return spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: alters,
		},
	}
}

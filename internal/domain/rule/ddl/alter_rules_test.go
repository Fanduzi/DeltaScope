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

func TestForbiddenAlterActionRuleHandlesStandaloneDropIndexWithoutTable(t *testing.T) {
	statementRule, err := newForbiddenAlterActionRule(ruleIDAlterDropIndexForbid, "drop_index", "drop index", rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationDropIndex,
			Alter:     []spec.Alter{{Action: "drop_index", Name: "idx_users_email"}},
		},
	}

	if !statementRule.AppliesTo(statement) {
		t.Fatal("expected standalone drop index statement to apply")
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["table"] != nil {
		t.Fatalf("expected standalone finding metadata not to fake table, got %+v", findings[0].Metadata)
	}
	if findings[0].Metadata["name"] != "idx_users_email" {
		t.Fatalf("expected standalone finding metadata name idx_users_email, got %+v", findings[0].Metadata)
	}
}

func TestForbiddenAlterActionRuleMapsPostgreSQLDropConstraintPrimaryKeySemantically(t *testing.T) {
	statementRule, err := newForbiddenAlterActionRule(ruleIDAlterDropPrimaryKeyForbid, "drop_primary_key", "drop primary key", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: []spec.Alter{{Action: "drop_constraint", Name: "users_pkey"}},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists:      true,
				Table:       &spec.Table{Name: "users"},
				PrimaryKey:  &spec.Index{Name: "users_primary_idx", Kind: spec.IndexKindPrimary},
				Constraints: []spec.Constraint{{Type: "primary_key", Name: "users_pkey"}},
			},
		},
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %#v", findings)
	}
	if findings[0].Metadata["action"] != "drop_primary_key" {
		t.Fatalf("expected semantic primary-key action metadata, got %+v", findings[0].Metadata)
	}
	if findings[0].Metadata["name"] != "users_pkey" {
		t.Fatalf("expected preserved constraint name, got %+v", findings[0].Metadata)
	}
}

func TestForbiddenAlterActionRuleDoesNotMapNonPrimaryConstraintToDropPrimaryKey(t *testing.T) {
	statementRule, err := newForbiddenAlterActionRule(ruleIDAlterDropPrimaryKeyForbid, "drop_primary_key", "drop primary key", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: []spec.Alter{{Action: "drop_constraint", Name: "users_pkey"}},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists:      true,
				Table:       &spec.Table{Name: "users"},
				PrimaryKey:  &spec.Index{Name: "users_primary_idx", Kind: spec.IndexKindPrimary},
				Constraints: []spec.Constraint{{Type: "unique", Name: "users_pkey"}},
			},
		},
	}

	findings, err := statementRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings for non-primary constraint drop, got %#v", findings)
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

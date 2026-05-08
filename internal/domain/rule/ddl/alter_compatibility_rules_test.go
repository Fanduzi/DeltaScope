// Package ddl verifies source-aware alter compatibility rules.
// input: metadata-enriched alter-table statements with source and target column shapes
// output: coverage for source-to-target compatibility findings on change/modify column
// pos: DDL alter compatibility test coverage
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAlterColumnCompatibilityRuleFindsBreakingTransitions(t *testing.T) {
	ruleUnderTest, err := newAlterColumnCompatibilityRule(ruleIDAlterModifyColumnCompatibilityRequire, "modify_column", "modify column", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("new compatibility rule: %v", err)
	}

	statement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{
					Action: "modify_column",
					Name:   "email",
					Column: &spec.AlterColumn{
						Definition: &spec.Column{Name: "email", Type: "varchar", Length: 64, Unsigned: false, NotNull: true},
					},
				},
			},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists: true,
				Table:  &spec.Table{Name: "users"},
				Columns: []spec.Column{
					{Name: "email", Type: "varchar", Length: 255, Unsigned: true, NotNull: false, AutoIncrement: true},
				},
			},
		},
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate compatibility rule: %v", err)
	}
	if len(findings) != 4 {
		t.Fatalf("expected 4 compatibility findings, got %d", len(findings))
	}
}

func TestAlterColumnCompatibilityRuleFindsFamilyChanges(t *testing.T) {
	ruleUnderTest, err := newAlterColumnCompatibilityRule(ruleIDAlterChangeColumnCompatibilityRequire, "change_column", "change column", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("new compatibility rule: %v", err)
	}

	statement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{
					Action: "change_column",
					Name:   "score",
					Column: &spec.AlterColumn{
						OldName:    "score",
						Definition: &spec.Column{Name: "score", Type: "varchar", Length: 32},
					},
				},
			},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists: true,
				Table:  &spec.Table{Name: "users"},
				Columns: []spec.Column{
					{Name: "score", Type: "int", Length: 11},
				},
			},
		},
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate compatibility rule: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 family-change finding, got %d", len(findings))
	}
}

func TestAlterColumnCompatibilityRuleSkipsOfflineMode(t *testing.T) {
	ruleUnderTest, err := newAlterColumnCompatibilityRule(ruleIDAlterModifyColumnCompatibilityRequire, "modify_column", "modify column", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("new compatibility rule: %v", err)
	}

	statement := alterStatement(spec.Alter{
		Action: "modify_column",
		Name:   "email",
		Column: &spec.AlterColumn{
			Definition: &spec.Column{Name: "email", Type: "varchar", Length: 64},
		},
	})

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate compatibility rule: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings without metadata, got %d", len(findings))
	}
}

func TestAlterTableOptionCompatibilityRuleFlagsMetadataBackedChanges(t *testing.T) {
	ruleUnderTest, err := newAlterTableOptionCompatibilityRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"required": true, "requires_metadata": true},
	})
	if err != nil {
		t.Fatalf("new option compatibility rule: %v", err)
	}

	statement := alterStatement(spec.Alter{
		Action:  "table_option",
		Options: map[string]string{"engine": "MyISAM", "charset": "utf8", "row_format": "COMPACT", "auto_increment": "42"},
	})
	statement.Metadata = &spec.Metadata{
		Schema: "app",
		TargetTable: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
			Options: map[string]string{
				"engine":         "InnoDB",
				"charset":        "utf8mb4",
				"collation":      "utf8mb4_general_ci",
				"row_format":     "DYNAMIC",
				"auto_increment": "100",
			},
		},
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate option compatibility rule: %v", err)
	}
	if len(findings) != 4 {
		t.Fatalf("expected 4 option-compatibility findings, got %d", len(findings))
	}
}

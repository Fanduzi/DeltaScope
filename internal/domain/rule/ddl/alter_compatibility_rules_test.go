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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestAlterColumnCompatibilityEvidenceGapsIgnoreOptInParam pins the inert
// requires_metadata contract: no param value may hide the missing-fact gap of
// an enabled, required, applicable rule; required=false and non-applicable
// input stay silent independently.
func TestAlterColumnCompatibilityEvidenceGapsIgnoreOptInParam(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		params map[string]any
		want   int
	}{
		{name: "requires_metadata true", params: map[string]any{"required": true, "requires_metadata": true}, want: 1},
		{name: "requires_metadata absent", params: map[string]any{"required": true}, want: 1},
		{name: "requires_metadata false", params: map[string]any{"required": true, "requires_metadata": false}, want: 1},
		{name: "required false", params: map[string]any{"required": false, "requires_metadata": true}, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ruleUnderTest, err := newAlterColumnCompatibilityRule(ruleIDAlterModifyColumnCompatibilityRequire, "modify_column", "modify column", rule.LevelBlocker, policy.RulePolicy{
				Enabled: true,
				Params:  tc.params,
			})
			if err != nil {
				t.Fatalf("new compatibility rule: %v", err)
			}
			reporter, ok := ruleUnderTest.(rule.EvidenceReporter)
			if !ok {
				t.Fatal("alter column compatibility rule must implement rule.EvidenceReporter")
			}
			gaps := reporter.EvidenceGaps(alterStatement(spec.Alter{
				Action: "modify_column",
				Name:   "email",
				Column: &spec.AlterColumn{
					Definition: &spec.Column{Name: "email", Type: "varchar", Length: 64},
				},
			}))
			if len(gaps) != tc.want {
				t.Fatalf("expected %d gaps, got %#v", tc.want, gaps)
			}
			if tc.want == 1 {
				if gaps[0].ReasonCode != "missing_source_column" {
					t.Fatalf("expected missing_source_column, got %q", gaps[0].ReasonCode)
				}
				if len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "source_column.definition" {
					t.Fatalf("expected [source_column.definition], got %#v", gaps[0].RequiredFacts)
				}
			}
		})
	}
}

// TestAlterColumnCompatibilityEvidenceGapsCoexistWithFindings proves partial
// facts do not suppress proven violations: one alter with a full source row
// yields a blocker while another with no source row yields the gap.
func TestAlterColumnCompatibilityEvidenceGapsCoexistWithFindings(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newAlterColumnCompatibilityRule(ruleIDAlterModifyColumnCompatibilityRequire, "modify_column", "modify column", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"required": true, "requires_metadata": true},
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
					Column: &spec.AlterColumn{Definition: &spec.Column{Name: "email", Type: "varchar", Length: 20}},
				},
				{
					Action: "modify_column",
					Name:   "nickname",
					Column: &spec.AlterColumn{Definition: &spec.Column{Name: "nickname", Type: "varchar", Length: 20}},
				},
			},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists:  true,
				Table:   &spec.Table{Name: "users"},
				Columns: []spec.Column{{Name: "email", Type: "varchar", Length: 200}},
			},
		},
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 narrowing finding for the known column, got %#v", findings)
	}
	reporter := ruleUnderTest.(rule.EvidenceReporter)
	gaps := reporter.EvidenceGaps(statement)
	if len(gaps) != 1 || gaps[0].ReasonCode != "missing_source_column" {
		t.Fatalf("expected 1 missing_source_column gap for the unknown column, got %#v", gaps)
	}
}

// TestAlterChangeColumnEvidenceGapsUsesOldName is the shared-implementation
// regression for CHANGE COLUMN: the source lookup uses the pre-rename column
// name, matching Evaluate.
func TestAlterChangeColumnEvidenceGapsUsesOldName(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newAlterColumnCompatibilityRule(ruleIDAlterChangeColumnCompatibilityRequire, "change_column", "change column", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"required": true, "requires_metadata": true},
	})
	if err != nil {
		t.Fatalf("new compatibility rule: %v", err)
	}
	reporter := ruleUnderTest.(rule.EvidenceReporter)

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
						Definition: &spec.Column{Name: "total", Type: "varchar", Length: 32},
					},
				},
			},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists:  true,
				Table:   &spec.Table{Name: "users"},
				Columns: []spec.Column{{Name: "score", Type: "int", Length: 11}},
			},
		},
	}

	if gaps := reporter.EvidenceGaps(statement); len(gaps) != 0 {
		t.Fatalf("expected no gaps when old-name source column is present, got %#v", gaps)
	}
	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 family-change finding, got %#v", findings)
	}

	statement.Metadata.TargetTable.Columns[0].Name = "other"
	if gaps := reporter.EvidenceGaps(statement); len(gaps) != 1 || gaps[0].ReasonCode != "missing_source_column" {
		t.Fatalf("expected missing_source_column when old-name column is absent, got %#v", gaps)
	}
}

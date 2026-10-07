// Package ddl verifies the T06-A5 declared-length policy contract at the rule
// layer: limit parameter bounds, shipped default levels, explicit-level
// overrides, type applicability, and the CREATE-only scope of the two
// column-length rules.
// input: synthetic column/type shapes plus policy overrides through the real rule constructors
// output: constructor validation, default-level, applicability, and scope pinning for char/varchar length rules
// pos: domain DDL rule test coverage for issue #85 T06-A5
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t06a5LengthRule builds the char or varchar max-length rule with the given
// policy entry so constructor and evaluation behavior stay observable.
func t06a5LengthRule(t *testing.T, charRule bool, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	var (
		statementRule rule.StatementRule
		err           error
	)
	if charRule {
		statementRule, err = newColumnCharMaxLengthRule(cfg)
	} else {
		statementRule, err = newColumnVarcharMaxLengthRule(cfg)
	}
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return statementRule
}

// TestT06A5LengthLimitBounds pins the constructor contract shared by both
// rules: limit>=1 is accepted, limit<=0 is rejected with a rule-bound error,
// and no database maximum leaks into the policy upper bound.
func TestT06A5LengthLimitBounds(t *testing.T) {
	for _, charRule := range []bool{true, false} {
		name := "varchar"
		ruleID := ruleIDColumnVarcharMaxLength
		if charRule {
			name = "char"
			ruleID = ruleIDColumnCharMaxLength
		}
		for _, limit := range []int{1, 8} {
			if _, err := t06a5LengthRuleOK(charRule, policy.RulePolicy{
				Enabled: true, Level: rule.LevelBlocker,
				Params: map[string]any{"limit": limit},
			}); err != nil {
				t.Fatalf("%s limit=%d rejected: %v", name, limit, err)
			}
		}
		for _, limit := range []int{0, -1} {
			_, err := t06a5LengthRuleOK(charRule, policy.RulePolicy{
				Enabled: true, Level: rule.LevelBlocker,
				Params: map[string]any{"limit": limit},
			})
			if err == nil || !strings.Contains(err.Error(), ruleID) {
				t.Fatalf("%s limit=%d error = %v, want a %s-bound rejection", name, limit, err, ruleID)
			}
		}
	}
}

func t06a5LengthRuleOK(charRule bool, cfg policy.RulePolicy) (rule.StatementRule, error) {
	if charRule {
		return newColumnCharMaxLengthRule(cfg)
	}
	return newColumnVarcharMaxLengthRule(cfg)
}

// TestT06A5LengthDefaultLevelsAndOverride pins the shipped levels — char
// warning, varchar blocker — and proves an explicit level=blocker on the
// char rule overrides cleanly (the isolated golden profile relies on it).
func TestT06A5LengthDefaultLevelsAndOverride(t *testing.T) {
	oversized := []struct {
		name    string
		char    bool
		column  spec.Column
		message string
	}{
		{"char", true, spec.Column{Name: "c", Type: "char(9)", Length: 9}, `char column "c" must not exceed 8 characters`},
		{"varchar", false, spec.Column{Name: "c", Type: "varchar(9)", Length: 9}, `varchar column "c" must not exceed 8 characters`},
	}
	wantDefault := map[bool]rule.Level{true: rule.LevelWarning, false: rule.LevelBlocker}
	for _, tc := range oversized {
		t.Run(tc.name+"_default", func(t *testing.T) {
			statementRule := t06a5LengthRule(t, tc.char, policy.RulePolicy{
				Enabled: true, Params: map[string]any{"limit": 8},
			})
			findings, err := statementRule.Evaluate(context.Background(), createTableWithColumns("t", tc.column))
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 || findings[0].Level != wantDefault[tc.char] {
				t.Fatalf("findings = %#v, want one %s finding", findings, wantDefault[tc.char])
			}
		})
	}
	// Explicit blocker override on the char rule (the A5 isolated profile).
	statementRule := t06a5LengthRule(t, true, policy.RulePolicy{
		Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"limit": 8},
	})
	findings, err := statementRule.Evaluate(context.Background(), createTableWithColumns("t", spec.Column{Name: "c", Type: "char(9)", Length: 9}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Level != rule.LevelBlocker {
		t.Fatalf("override findings = %#v, want one blocker", findings)
	}
}

// TestT06A5LengthTypeApplicability pins the type gate at the rule layer:
// each rule only fires on its own base type and never on the other string
// type nor on binary/varbinary/int/text shapes.
func TestT06A5LengthTypeApplicability(t *testing.T) {
	charRule := t06a5LengthRule(t, true, policy.RulePolicy{
		Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"limit": 8},
	})
	varcharRule := t06a5LengthRule(t, false, policy.RulePolicy{
		Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"limit": 8},
	})
	cases := []struct {
		column    spec.Column
		wantChar  int
		wantVchar int
	}{
		{spec.Column{Name: "c", Type: "char(9)", Length: 9}, 1, 0},
		{spec.Column{Name: "c", Type: "varchar(9)", Length: 9}, 0, 1},
		{spec.Column{Name: "c", Type: "binary(9)", Length: 9}, 0, 0},
		{spec.Column{Name: "c", Type: "varbinary(9)", Length: 9}, 0, 0},
		{spec.Column{Name: "c", Type: "int", Length: 0}, 0, 0},
		{spec.Column{Name: "c", Type: "text", Length: 0}, 0, 0},
	}
	for _, tc := range cases {
		statement := createTableWithColumns("t", tc.column)
		charFindings, err := charRule.Evaluate(context.Background(), statement)
		if err != nil {
			t.Fatalf("char evaluate %s: %v", tc.column.Type, err)
		}
		varcharFindings, err := varcharRule.Evaluate(context.Background(), statement)
		if err != nil {
			t.Fatalf("varchar evaluate %s: %v", tc.column.Type, err)
		}
		if len(charFindings) != tc.wantChar || len(varcharFindings) != tc.wantVchar {
			t.Fatalf("type %s: char=%d varchar=%d, want %d/%d",
				tc.column.Type, len(charFindings), len(varcharFindings), tc.wantChar, tc.wantVchar)
		}
	}
}

// TestT06A5LengthRulesIgnoreAlterActions pins the CREATE-only scope at the
// rule-method layer: ALTER ADD/MODIFY/CHANGE column statements are never
// applicable to either length rule. ALTER-side length policy ownership stays
// with #87/T08 — this test asserts current scope, not a new contract.
func TestT06A5LengthRulesIgnoreAlterActions(t *testing.T) {
	column := spec.Column{Name: "c", Type: "varchar(9)", Length: 9}
	alters := []spec.Statement{
		{Kind: spec.KindDDL, DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "t"},
			Alter: []spec.Alter{{Action: "add_column", Name: "c", Column: &spec.AlterColumn{Definition: &column}}},
		}},
		{Kind: spec.KindDDL, DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "t"},
			Alter: []spec.Alter{{Action: "modify_column", Name: "c", Column: &spec.AlterColumn{Definition: &column}}},
		}},
	}
	for _, charRule := range []bool{true, false} {
		statementRule := t06a5LengthRule(t, charRule, policy.RulePolicy{
			Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"limit": 8},
		})
		for _, statement := range alters {
			if statementRule.AppliesTo(statement) {
				t.Fatalf("%s rule applied to %s", statementRule.ID(), statement.DDL.Alter[0].Action)
			}
			findings, err := statementRule.Evaluate(context.Background(), statement)
			if err != nil || len(findings) != 0 {
				t.Fatalf("%s on alter: findings=%#v err=%v, want silence", statementRule.ID(), findings, err)
			}
		}
	}
}

// Package dml verifies Tier-1 DML insert-rule behavior.
// input: synthetic insert Statement specs and rule policy overrides
// output: deterministic findings for insert row-count, replace, insert-select, and on-duplicate checks
// pos: domain DML rule test coverage for insert-family guardrails
// note: if this file changes, update this header and module README.md.
package dml

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestInsertRowsMaxCountRuleFindsOversizedBatch(t *testing.T) {
	ruleUnderTest, err := newInsertRowsMaxCountRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"limit": 2},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(),insertStatement(3, false, false, false))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Message != "INSERT statements must not include more than 2 row(s)" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestReplaceForbiddenRuleFindsReplace(t *testing.T) {
	ruleUnderTest, err := newReplaceForbiddenRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(),insertStatement(1, true, false, false))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Message != "REPLACE statements are forbidden" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestInsertSelectForbiddenRuleFindsInsertSelect(t *testing.T) {
	ruleUnderTest, err := newInsertSelectForbiddenRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(),insertStatement(0, false, true, false))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Message != "INSERT ... SELECT statements are forbidden" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestOnDuplicateForbiddenRuleFindsOnDuplicate(t *testing.T) {
	ruleUnderTest, err := newOnDuplicateForbiddenRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(),insertStatement(1, false, false, true))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Message != "INSERT ... ON DUPLICATE KEY UPDATE is forbidden" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestInsertSelectForbiddenRuleIgnoresPostgreSQLOnConflictShape(t *testing.T) {
	ruleUnderTest, err := newInsertSelectForbiddenRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(),spec.Statement{
		Kind:    spec.KindDML,
		Dialect: spec.DialectPostgreSQL,
		DML: &spec.DML{
			Operation:      spec.DMLOperationInsert,
			IsInsertSelect: false,
			HasOnDuplicate: false,
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestOnDuplicateForbiddenRuleIgnoresPostgreSQLOnConflictShape(t *testing.T) {
	ruleUnderTest, err := newOnDuplicateForbiddenRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(),spec.Statement{
		Kind:    spec.KindDML,
		Dialect: spec.DialectPostgreSQL,
		DML: &spec.DML{
			Operation:      spec.DMLOperationInsert,
			IsInsertSelect: false,
			HasOnDuplicate: false,
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

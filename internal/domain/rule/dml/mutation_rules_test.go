// Package dml verifies Tier-1 DML mutation-rule behavior.
// input: synthetic update/delete Statement specs and rule policy overrides
// output: deterministic findings for WHERE, LIMIT, ORDER BY, subquery, and JOIN ... ON checks
// pos: domain DML rule test coverage for update/delete guardrails
// note: if this file changes, update this header and module README.md.
package dml

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestWhereRequiredRuleFindsMissingWhere(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newWhereRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), updateStatement(false, false, false, false, false))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Message != "UPDATE and DELETE statements must include a WHERE clause" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestLimitForbiddenRuleFindsLimit(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newLimitForbiddenRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), updateStatement(true, true, false, false, false))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Message != "UPDATE and DELETE statements must not include LIMIT" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestOrderByForbiddenRuleFindsOrderBy(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newOrderByForbiddenRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), deleteStatement(true, false, true, false, false))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Message != "UPDATE and DELETE statements must not include ORDER BY" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestSubqueryForbiddenRuleFindsSubquery(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newSubqueryForbiddenRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), updateStatement(true, false, false, true, false))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Message != "UPDATE and DELETE statements must not include subqueries" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestJoinOnRequiredRuleFindsJoinWithoutOn(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newJoinOnRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), updateStatement(true, false, false, false, true))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Message != "UPDATE and DELETE joins must include an ON clause" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestMutationRulesIgnoreInsertStatements(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newWhereRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), insertStatement(2, false, false, false))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected insert statements to be ignored, got %d findings", len(findings))
	}
}

func updateStatement(hasWhere, hasLimit, hasOrderBy, hasSubquery, joinWithoutOn bool) spec.Statement {
	return spec.Statement{
		Kind: spec.KindDML,
		DML: &spec.DML{
			Operation:   spec.DMLOperationUpdate,
			HasWhere:    hasWhere,
			HasLimit:    hasLimit,
			HasOrderBy:  hasOrderBy,
			HasSubquery: hasSubquery,
			HasJoin:     joinWithoutOn,
			HasJoinOn:   false,
		},
	}
}

func deleteStatement(hasWhere, hasLimit, hasOrderBy, hasSubquery, joinWithoutOn bool) spec.Statement {
	return spec.Statement{
		Kind: spec.KindDML,
		DML: &spec.DML{
			Operation:   spec.DMLOperationDelete,
			HasWhere:    hasWhere,
			HasLimit:    hasLimit,
			HasOrderBy:  hasOrderBy,
			HasSubquery: hasSubquery,
			HasJoin:     joinWithoutOn,
			HasJoinOn:   false,
		},
	}
}

func insertStatement(rows int, isReplace, isInsertSelect, hasOnDuplicate bool) spec.Statement {
	return spec.Statement{
		Kind: spec.KindDML,
		DML: &spec.DML{
			Operation:      spec.DMLOperationInsert,
			InsertRows:     rows,
			IsReplace:      isReplace,
			IsInsertSelect: isInsertSelect,
			HasOnDuplicate: hasOnDuplicate,
		},
	}
}

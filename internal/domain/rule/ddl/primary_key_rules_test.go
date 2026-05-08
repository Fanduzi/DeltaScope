// Package ddl verifies Tier-1 DDL primary-key rule behavior.
// input: create-table Statement specs and rule policy overrides
// output: deterministic findings for primary-key presence and shape rules
// pos: domain DDL rule test coverage for primary-key constraints
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func TestPrimaryKeyRequiredRuleFindsMissingPrimaryKey(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newPrimaryKeyRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"required": true,
		},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), createTableStatement("users", "user table", nil))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelBlocker {
		t.Fatalf("expected blocker level, got %q", findings[0].Level)
	}
	if findings[0].Message != "primary key is required" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestPrimaryKeyRequiredRuleAcceptsPresentPrimaryKey(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newPrimaryKeyRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"required": true,
		},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), createTableStatement("users", "user table", []string{"id"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestPrimaryKeyColumnCountRuleFindsCompositePrimaryKeyWhenLimitIsOne(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newPrimaryKeyColumnCountRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"limit": 1,
		},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), createTableStatement("user_roles", "user roles", []string{"user_id", "role_id"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", findings[0].Level)
	}
	if findings[0].Message != "primary key must not contain more than 1 columns" {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
	if findings[0].Metadata["actual"] != 2 {
		t.Fatalf("expected actual column count metadata, got %#v", findings[0].Metadata["actual"])
	}
}

func TestPrimaryKeyColumnCountRuleIgnoresMissingPrimaryKey(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newPrimaryKeyColumnCountRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"limit": 1,
		},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), createTableStatement("users", "user table", nil))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings without primary key metadata, got %d", len(findings))
	}
}

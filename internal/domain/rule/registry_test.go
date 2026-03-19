// Package rule_test verifies rule registration and evaluation behavior.
// input: synthetic statements and test rule implementations
// output: test coverage for deterministic rule execution, ID enforcement, and finding collection
// pos: domain rule engine test coverage
// note: if this file changes, update this header and module README.md.
package rule_test

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type testStatementRule struct {
	id         string
	kind       spec.Kind
	level      rule.Level
	message    string
	evaluated  *int
}

func (r testStatementRule) ID() string {
	return r.id
}

func (r testStatementRule) AppliesTo(statement spec.Statement) bool {
	return statement.Kind == r.kind
}

func (r testStatementRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if r.evaluated != nil {
		*r.evaluated++
	}
	return []rule.Finding{{
		RuleID:        r.id,
		Level:         r.level,
		Message:       r.message,
		StatementKind: statement.Kind.String(),
	}}, nil
}

type testGlobalRule struct {
	id      string
	level   rule.Level
	message string
}

func (r testGlobalRule) ID() string {
	return r.id
}

func (r testGlobalRule) EvaluateAll(statements []spec.Statement) ([]rule.Finding, error) {
	return []rule.Finding{{
		RuleID:  r.id,
		Level:   r.level,
		Message: r.message,
		Metadata: map[string]any{
			"statements": len(statements),
		},
	}}, nil
}

func TestRegistryEvaluatesStatementRulesDeterministically(t *testing.T) {
	registry := rule.NewRegistry()
	evaluated := 0

	if err := registry.RegisterStatement(testStatementRule{
		id:        "ddl.table.comment.require",
		kind:      spec.KindDDL,
		level:     rule.LevelWarning,
		message:   "table comment missing",
		evaluated: &evaluated,
	}); err != nil {
		t.Fatalf("register first statement rule: %v", err)
	}
	if err := registry.RegisterStatement(testStatementRule{
		id:      "dml.where.require",
		kind:    spec.KindDML,
		level:   rule.LevelBlocker,
		message: "where clause required",
	}); err != nil {
		t.Fatalf("register second statement rule: %v", err)
	}

	findings, err := registry.EvaluateStatement(spec.Statement{Kind: spec.KindDDL})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if evaluated != 1 {
		t.Fatalf("expected 1 statement rule evaluation, got %d", evaluated)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "ddl.table.comment.require" {
		t.Fatalf("expected first finding from ddl rule, got %+v", findings[0])
	}
}

func TestRegistryCollectsGlobalRuleFindings(t *testing.T) {
	registry := rule.NewRegistry()
	if err := registry.RegisterGlobal(testGlobalRule{
		id:      "audit.batch.notice",
		level:   rule.LevelNotice,
		message: "batch processed",
	}); err != nil {
		t.Fatalf("register global rule: %v", err)
	}

	findings, err := registry.EvaluateGlobal([]spec.Statement{
		{Kind: spec.KindDDL},
		{Kind: spec.KindDML},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 global finding, got %d", len(findings))
	}
	if findings[0].Metadata["statements"] != 2 {
		t.Fatalf("expected statements metadata to equal 2, got %#v", findings[0].Metadata["statements"])
	}
}

func TestRegistryRejectsEmptyRuleID(t *testing.T) {
	registry := rule.NewRegistry()
	err := registry.RegisterStatement(testStatementRule{
		id:      "",
		kind:    spec.KindDML,
		level:   rule.LevelBlocker,
		message: "where clause required",
	})
	if err == nil {
		t.Fatal("expected empty statement rule ID to be rejected")
	}
}

func TestRegistryRejectsDuplicateRuleIDs(t *testing.T) {
	registry := rule.NewRegistry()
	if err := registry.RegisterStatement(testStatementRule{
		id:      "dml.where.require",
		kind:    spec.KindDML,
		level:   rule.LevelBlocker,
		message: "where clause required",
	}); err != nil {
		t.Fatalf("register first statement rule: %v", err)
	}

	err := registry.RegisterStatement(testStatementRule{
		id:      "dml.where.require",
		kind:    spec.KindDML,
		level:   rule.LevelBlocker,
		message: "duplicate",
	})
	if err == nil {
		t.Fatal("expected duplicate statement rule ID to be rejected")
	}
}

func TestRegistryStampsEmptyFindingRuleID(t *testing.T) {
	registry := rule.NewRegistry()
	if err := registry.RegisterStatement(testStatementRule{
		id:      "dml.where.require",
		kind:    spec.KindDML,
		level:   rule.LevelBlocker,
		message: "where clause required",
	}); err != nil {
		t.Fatalf("register statement rule: %v", err)
	}

	findings, err := registry.EvaluateStatement(spec.Statement{Kind: spec.KindDML})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "dml.where.require" {
		t.Fatalf("expected rule ID to be stamped from registry, got %+v", findings[0])
	}
}

func TestRegistryRejectsMismatchedFindingRuleID(t *testing.T) {
	registry := rule.NewRegistry()
	err := registry.RegisterStatement(testStatementRule{
		id:      "dml.where.require",
		kind:    spec.KindDML,
		level:   rule.LevelBlocker,
		message: "where clause required",
	})
	if err != nil {
		t.Fatalf("register statement rule: %v", err)
	}

	badRule := testStatementRule{
		id:      "ddl.table.comment.require",
		kind:    spec.KindDDL,
		level:   rule.LevelWarning,
		message: "table comment missing",
	}
	registry = rule.NewRegistry()
	if err := registry.RegisterStatement(badMismatchedRule{inner: badRule, findingRuleID: "wrong.id"}); err != nil {
		t.Fatalf("register mismatched rule: %v", err)
	}

	_, err = registry.EvaluateStatement(spec.Statement{Kind: spec.KindDDL})
	if err == nil {
		t.Fatal("expected mismatched finding rule ID to fail evaluation")
	}
}

type badMismatchedRule struct {
	inner         testStatementRule
	findingRuleID string
}

func (r badMismatchedRule) ID() string {
	return r.inner.ID()
}

func (r badMismatchedRule) AppliesTo(statement spec.Statement) bool {
	return r.inner.AppliesTo(statement)
}

func (r badMismatchedRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	return []rule.Finding{{
		RuleID:  r.findingRuleID,
		Level:   r.inner.level,
		Message: r.inner.message,
	}}, nil
}

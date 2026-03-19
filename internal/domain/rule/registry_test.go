// Package rule_test verifies rule registration and evaluation behavior.
// input: synthetic statements, test rules, and report aggregation scenarios
// output: test coverage for deterministic rule execution and finding collection
// pos: domain rule engine test coverage
// note: if this file changes, update this header and module README.md.
package rule_test

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
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

	registry.RegisterStatement(testStatementRule{
		id:        "ddl.table.comment.require",
		kind:      spec.KindDDL,
		level:     rule.LevelWarning,
		message:   "table comment missing",
		evaluated: &evaluated,
	})
	registry.RegisterStatement(testStatementRule{
		id:      "dml.where.require",
		kind:    spec.KindDML,
		level:   rule.LevelBlocker,
		message: "where clause required",
	})

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
	registry.RegisterGlobal(testGlobalRule{
		id:      "audit.batch.notice",
		level:   rule.LevelNotice,
		message: "batch processed",
	})

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

func TestEvaluateProducesReportFlowOutput(t *testing.T) {
	registry := rule.NewRegistry()
	registry.RegisterStatement(testStatementRule{
		id:      "dml.where.require",
		kind:    spec.KindDML,
		level:   rule.LevelBlocker,
		message: "where clause required",
	})
	registry.RegisterGlobal(testGlobalRule{
		id:      "audit.batch.notice",
		level:   rule.LevelNotice,
		message: "batch processed",
	})

	result, err := audit.EvaluateStatements(registry, []spec.Statement{
		{
			Kind:          spec.KindDML,
			RawSQL:        "delete from users",
			NormalizedSQL: "delete from users",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Verdict != report.VerdictReject {
		t.Fatalf("expected reject verdict, got %q", result.Verdict)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %d", len(result.Statements))
	}
	if len(result.Statements[0].Findings) != 1 {
		t.Fatalf("expected 1 statement finding, got %d", len(result.Statements[0].Findings))
	}
	if len(result.GlobalFindings) != 1 {
		t.Fatalf("expected 1 global finding, got %d", len(result.GlobalFindings))
	}
}

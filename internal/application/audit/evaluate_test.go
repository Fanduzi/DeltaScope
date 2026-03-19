// Package audit verifies report-flow integration for audit evaluation.
// input: extracted statements and registered rules
// output: test coverage for application-owned evaluation and report aggregation
// pos: application audit integration test coverage
// note: if this file changes, update this header and module README.md.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type testStatementRule struct {
	id      string
	kind    spec.Kind
	level   rule.Level
	message string
}

func (r testStatementRule) ID() string {
	return r.id
}

func (r testStatementRule) AppliesTo(statement spec.Statement) bool {
	return statement.Kind == r.kind
}

func (r testStatementRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	return []rule.Finding{{
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
		Level:   r.level,
		Message: r.message,
		Metadata: map[string]any{
			"statements": len(statements),
		},
	}}, nil
}

func TestEvaluateProducesReportFlowOutput(t *testing.T) {
	registry := rule.NewRegistry()
	if err := registry.RegisterStatement(testStatementRule{
		id:      "dml.where.require",
		kind:    spec.KindDML,
		level:   rule.LevelBlocker,
		message: "where clause required",
	}); err != nil {
		t.Fatalf("register statement rule: %v", err)
	}
	if err := registry.RegisterGlobal(testGlobalRule{
		id:      "audit.batch.notice",
		level:   rule.LevelNotice,
		message: "batch processed",
	}); err != nil {
		t.Fatalf("register global rule: %v", err)
	}

	result, err := EvaluateStatements(registry, []spec.Statement{
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
	if result.Statements[0].Findings[0].RuleID != "dml.where.require" {
		t.Fatalf("expected statement finding rule ID to be enforced, got %+v", result.Statements[0].Findings[0])
	}
	if len(result.GlobalFindings) != 1 {
		t.Fatalf("expected 1 global finding, got %d", len(result.GlobalFindings))
	}
	if result.GlobalFindings[0].RuleID != "audit.batch.notice" {
		t.Fatalf("expected global finding rule ID to be enforced, got %+v", result.GlobalFindings[0])
	}
}

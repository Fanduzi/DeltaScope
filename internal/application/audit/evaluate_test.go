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
	id         string
	kind       spec.Kind
	level      rule.Level
	message    string
	suggestion string
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
		Suggestion:    r.suggestion,
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

func TestEvaluateStatementsEnrichesFindingFromCatalogMetadata(t *testing.T) {
	registry := rule.NewRegistry()
	if err := registry.RegisterStatement(testStatementRule{
		id:         "dml.where.require",
		kind:       spec.KindDML,
		level:      rule.LevelBlocker,
		message:    "where clause required",
		suggestion: "add a WHERE clause that narrows the affected rows",
	}); err != nil {
		t.Fatalf("register statement rule: %v", err)
	}

	result, err := EvaluateStatements(registry, []spec.Statement{{
		Kind:          spec.KindDML,
		RawSQL:        "delete from users",
		NormalizedSQL: "delete from users",
	}})
	if err != nil {
		t.Fatalf("evaluate statements: %v", err)
	}

	finding := result.Statements[0].Findings[0]
	if finding.Explanation == nil {
		t.Fatal("expected finding explanation from catalog metadata")
	}
	if finding.Explanation.Why == "" || finding.Explanation.Risk == "" || finding.Explanation.Summary == "" {
		t.Fatalf("expected explanation metadata to be populated, got %#v", finding.Explanation)
	}
	if finding.Explanation.Suggestion != "add a WHERE clause that narrows the affected rows" {
		t.Fatalf("expected rule-authored suggestion to win, got %#v", finding.Explanation)
	}
}

func TestEvaluateStatementsGracefullyHandlesMissingCatalogMetadata(t *testing.T) {
	registry := rule.NewRegistry()
	if err := registry.RegisterGlobal(testGlobalRule{
		id:      "audit.batch.notice",
		level:   rule.LevelNotice,
		message: "batch processed",
	}); err != nil {
		t.Fatalf("register global rule: %v", err)
	}

	result, err := EvaluateStatements(registry, []spec.Statement{{
		Kind:          spec.KindDML,
		RawSQL:        "delete from users",
		NormalizedSQL: "delete from users",
	}})
	if err != nil {
		t.Fatalf("evaluate statements: %v", err)
	}
	if result.Verdict != report.VerdictPass {
		t.Fatalf("expected verdict semantics to remain unchanged, got %q", result.Verdict)
	}
	if len(result.GlobalFindings) != 1 {
		t.Fatalf("expected one global finding, got %d", len(result.GlobalFindings))
	}
	if result.GlobalFindings[0].Explanation == nil {
		t.Fatal("expected minimal explanation for uncatalogued finding")
	}
	if result.GlobalFindings[0].Explanation.Summary == "" {
		t.Fatalf("expected minimal explanation summary, got %#v", result.GlobalFindings[0].Explanation)
	}
}

func TestEvaluateStatementsBuildsAggregateExplanations(t *testing.T) {
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

	result, err := EvaluateStatements(registry, []spec.Statement{{
		Kind:          spec.KindDML,
		RawSQL:        "delete from users",
		NormalizedSQL: "delete from users",
	}})
	if err != nil {
		t.Fatalf("evaluate statements: %v", err)
	}
	if result.Statements[0].Explanation == nil {
		t.Fatal("expected statement explanation to be generated")
	}
	if result.Statements[0].Explanation.Summary == "" || len(result.Statements[0].Explanation.Reasons) == 0 {
		t.Fatalf("expected statement explanation content, got %#v", result.Statements[0].Explanation)
	}
	if result.Explanation == nil {
		t.Fatal("expected result explanation to be generated")
	}
	if result.Explanation.Summary == "" || len(result.Explanation.Reasons) != 2 {
		t.Fatalf("expected result explanation content from statement and global findings, got %#v", result.Explanation)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("expected verdict semantics to remain unchanged, got %q", result.Verdict)
	}
}

func TestEvaluateStatementsAddsMetadataLimitedNoteForMetadataAwareRuleWithoutMetadata(t *testing.T) {
	registry := rule.NewRegistry()
	if err := registry.RegisterStatement(testStatementRule{
		id:      "ddl.table.exists.create.forbid",
		kind:    spec.KindDDL,
		level:   rule.LevelBlocker,
		message: "table already exists",
	}); err != nil {
		t.Fatalf("register statement rule: %v", err)
	}

	result, err := EvaluateStatements(registry, []spec.Statement{{
		Kind:          spec.KindDDL,
		RawSQL:        "create table users (id bigint)",
		NormalizedSQL: "create table users (id bigint)",
	}})
	if err != nil {
		t.Fatalf("evaluate statements: %v", err)
	}

	finding := result.Statements[0].Findings[0]
	if finding.Explanation == nil || finding.Explanation.Metadata == nil {
		t.Fatalf("expected metadata note on explanation, got %#v", finding.Explanation)
	}
	if finding.Explanation.Metadata.Status != "limited" {
		t.Fatalf("expected limited metadata status, got %#v", finding.Explanation.Metadata)
	}
	if finding.Explanation.Metadata.Note == "" {
		t.Fatalf("expected missing-metadata note, got %#v", finding.Explanation.Metadata)
	}
}

func TestEvaluateStatementsAddsMetadataAvailableNoteForMetadataAwareRuleWithMetadata(t *testing.T) {
	registry := rule.NewRegistry()
	if err := registry.RegisterStatement(testStatementRule{
		id:      "ddl.table.exists.create.forbid",
		kind:    spec.KindDDL,
		level:   rule.LevelBlocker,
		message: "table already exists",
	}); err != nil {
		t.Fatalf("register statement rule: %v", err)
	}

	result, err := EvaluateStatements(registry, []spec.Statement{{
		Kind:          spec.KindDDL,
		RawSQL:        "create table users (id bigint)",
		NormalizedSQL: "create table users (id bigint)",
		Metadata: &spec.Metadata{
			Schema: "app",
			TargetTable: &spec.TableSnapshot{Exists: true},
		},
	}})
	if err != nil {
		t.Fatalf("evaluate statements: %v", err)
	}

	finding := result.Statements[0].Findings[0]
	if finding.Explanation == nil || finding.Explanation.Metadata == nil {
		t.Fatalf("expected metadata note on explanation, got %#v", finding.Explanation)
	}
	if finding.Explanation.Metadata.Status != "available" {
		t.Fatalf("expected available metadata status, got %#v", finding.Explanation.Metadata)
	}
	if finding.Explanation.Metadata.Note == "" {
		t.Fatalf("expected available-metadata note, got %#v", finding.Explanation.Metadata)
	}
}

func TestEvaluateStatementsTreatsSchemaAsSufficientForSchemaAwareRules(t *testing.T) {
	registry := rule.NewRegistry()
	if err := registry.RegisterStatement(testStatementRule{
		id:      "dml.table.denylist.forbid",
		kind:    spec.KindDML,
		level:   rule.LevelWarning,
		message: "table is denylisted",
	}); err != nil {
		t.Fatalf("register statement rule: %v", err)
	}

	result, err := EvaluateStatements(registry, []spec.Statement{{
		Kind:          spec.KindDML,
		RawSQL:        "delete from users",
		NormalizedSQL: "delete from users",
		Metadata: &spec.Metadata{
			Schema: "app",
		},
	}})
	if err != nil {
		t.Fatalf("evaluate statements: %v", err)
	}

	finding := result.Statements[0].Findings[0]
	if finding.Explanation == nil || finding.Explanation.Metadata == nil {
		t.Fatalf("expected metadata note on explanation, got %#v", finding.Explanation)
	}
	if finding.Explanation.Metadata.Status != "available" {
		t.Fatalf("expected schema-aware rule to treat schema-only metadata as available, got %#v", finding.Explanation.Metadata)
	}
}

func TestEvaluateStatementsTreatsTargetTableAsInsufficientForInstanceAwareRules(t *testing.T) {
	registry := rule.NewRegistry()
	if err := registry.RegisterStatement(testStatementRule{
		id:      "ddl.table.drop.adaptive_hash.warn",
		kind:    spec.KindDDL,
		level:   rule.LevelNotice,
		message: "adaptive hash caution",
	}); err != nil {
		t.Fatalf("register statement rule: %v", err)
	}

	result, err := EvaluateStatements(registry, []spec.Statement{{
		Kind:          spec.KindDDL,
		RawSQL:        "drop table users",
		NormalizedSQL: "drop table users",
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{Exists: true},
		},
	}})
	if err != nil {
		t.Fatalf("evaluate statements: %v", err)
	}

	finding := result.Statements[0].Findings[0]
	if finding.Explanation == nil || finding.Explanation.Metadata == nil {
		t.Fatalf("expected metadata note on explanation, got %#v", finding.Explanation)
	}
	if finding.Explanation.Metadata.Status != "limited" {
		t.Fatalf("expected instance-aware rule to stay limited without instance facts, got %#v", finding.Explanation.Metadata)
	}
}

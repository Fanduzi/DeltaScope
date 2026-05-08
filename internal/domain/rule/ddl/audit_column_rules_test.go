// Package ddl verifies audit-column DDL rule behavior.
// input: create-table Statement specs with synthetic timestamp metadata and rule policy overrides
// output: deterministic findings for missing created/updated audit timestamp columns
// pos: domain DDL rule test coverage for audit-column governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditColumnsRequiredRuleFindsMissingCreatedAndUpdatedColumns(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newTableAuditColumnsRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), createTableWithColumns("users", spec.Column{Name: "name", Type: "varchar(32)", Length: 32, Comment: "'name'"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
}

func TestAuditColumnsRequiredRuleAcceptsCreatedAndUpdatedPatterns(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newTableAuditColumnsRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), createTableWithColumns(
		"users",
		spec.Column{Name: "created_at", Type: "datetime", Comment: "'created'", DefaultIsCurrentTimestamp: true, HasDefault: true, DefaultValue: "current_timestamp", NotNull: true},
		spec.Column{Name: "updated_at", Type: "datetime", Comment: "'updated'", DefaultIsCurrentTimestamp: true, OnUpdateCurrentTimestamp: true, HasDefault: true, DefaultValue: "current_timestamp", NotNull: true},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestAuditColumnsRequiredRulePostgreSQLOnlyRequiresCreatedPattern(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newTableAuditColumnsRequiredRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	statement := createTableWithColumns(
		"users",
		spec.Column{Name: "created_at", Type: "timestamp", Comment: "'created'", DefaultIsCurrentTimestamp: true, HasDefault: true, DefaultValue: "current_timestamp", NotNull: true},
	)
	statement.Dialect = spec.DialectPostgreSQL

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

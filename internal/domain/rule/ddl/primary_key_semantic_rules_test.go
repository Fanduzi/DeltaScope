// Package ddl verifies primary-key semantic rule behavior.
// input: synthetic create-table statements with primary-key declarations and enriched column metadata
// output: focused coverage for bigint, unsigned, auto-increment, and not-null primary-key findings
// pos: domain DDL rule test coverage for primary-key semantic governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPrimaryKeyBigintRuleFindsNonBigintPrimaryKey(t *testing.T) {
	statementRule, err := newSinglePrimaryKeyColumnRule(ruleIDPrimaryKeyBigintRequire, rule.LevelBlocker, "must use bigint", "change the primary key column type to bigint", func(column spec.Column) bool {
		return baseType(column) == "bigint"
	}, policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"required": true}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := statementRule.Evaluate(context.Background(), primaryKeyStatement(spec.Column{Name: "id", Type: "int", NotNull: true, Unsigned: true, AutoIncrement: true}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestPrimaryKeyUnsignedRuleFindsSignedPrimaryKey(t *testing.T) {
	statementRule, err := newSinglePrimaryKeyColumnRule(ruleIDPrimaryKeyUnsignedRequire, rule.LevelBlocker, "must be unsigned", "mark the primary key column as UNSIGNED", func(column spec.Column) bool {
		return column.Unsigned
	}, policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"required": true}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := statementRule.Evaluate(context.Background(), primaryKeyStatement(spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoIncrement: true}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestPrimaryKeyAutoIncrementRuleFindsMissingAutoIncrement(t *testing.T) {
	statementRule, err := newSinglePrimaryKeyColumnRule(ruleIDPrimaryKeyAutoIncrementRequire, rule.LevelBlocker, "must use auto_increment", "add AUTO_INCREMENT to the primary key column", func(column spec.Column) bool {
		return column.AutoIncrement
	}, policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"required": true}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := statementRule.Evaluate(context.Background(), primaryKeyStatement(spec.Column{Name: "id", Type: "bigint", NotNull: true, Unsigned: true}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestPrimaryKeyNotNullRuleFindsNullablePrimaryKey(t *testing.T) {
	statementRule, err := newPrimaryKeyNotNullRule(policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"required": true}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := statementRule.Evaluate(context.Background(), primaryKeyStatement(spec.Column{Name: "id", Type: "bigint", Unsigned: true, AutoIncrement: true}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestPrimaryKeySemanticRulesSkipCompositePrimaryKeys(t *testing.T) {
	statementRule, err := newSinglePrimaryKeyColumnRule(ruleIDPrimaryKeyBigintRequire, rule.LevelBlocker, "must use bigint", "change the primary key column type to bigint", func(column spec.Column) bool {
		return baseType(column) == "bigint"
	}, policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"required": true}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Columns: []spec.Column{
				{Name: "tenant_id", Type: "bigint"},
				{Name: "id", Type: "int"},
			},
			PrimaryKey: &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"tenant_id", "id"}},
		},
	}
	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestPrimaryKeyBigintRuleAcceptsPostgreSQLBigIntAliases(t *testing.T) {
	statementRule, err := newSinglePrimaryKeyColumnRule(ruleIDPrimaryKeyBigintRequire, rule.LevelBlocker, "must use bigint", "change the primary key column type to bigint", func(column spec.Column) bool {
		return baseType(column) == "bigint"
	}, policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"required": true}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	for _, columnType := range []string{"int8", "pg_catalog.int8"} {
		statement := primaryKeyStatement(spec.Column{Name: "id", Type: columnType, NotNull: true})
		statement.Dialect = spec.DialectPostgreSQL
		findings, err := statementRule.Evaluate(context.Background(), statement)
		if err != nil {
			t.Fatalf("evaluate %s: %v", columnType, err)
		}
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for %s, got %d", columnType, len(findings))
		}
	}
}

func primaryKeyStatement(column spec.Column) spec.Statement {
	return spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table:      &spec.Table{Name: "users"},
			Columns:    []spec.Column{column},
			PrimaryKey: &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{column.Name}},
		},
	}
}

// ---------------------------------------------------------------------------
// v0.39.0 Task 1: Rule applicability — PostgreSQL ALTER TABLE ADD CONSTRAINT
// ---------------------------------------------------------------------------

// TestPostgreSQLAlterTableAddPrimaryKeyBigintRuleCoverage proves that the
// primary-key bigint rule fires on the PostgreSQL ALTER TABLE ADD CONSTRAINT
// shape after Task 2 primary-key projection.
func TestPostgreSQLAlterTableAddPrimaryKeyBigintRuleCoverage(t *testing.T) {
	statementRule, err := newSinglePrimaryKeyColumnRule(
		ruleIDPrimaryKeyBigintRequire,
		rule.LevelBlocker,
		"must use bigint",
		"change the primary key column type to bigint",
		func(column spec.Column) bool {
			return baseType(column) == "bigint"
		},
		policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"required": true}},
	)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{{
				Action: "add_constraint",
				Name:   "users_pkey",
				Options: map[string]string{
					"constraint_type": "primary_key",
					"columns":         "id",
				},
			}},
		},
	}

	if !statementRule.AppliesTo(statement) {
		t.Fatal("expected rule to apply to ALTER TABLE ADD CONSTRAINT PRIMARY KEY")
	}

	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelBlocker {
		t.Fatalf("expected blocker level, got %s", findings[0].Level)
	}
}

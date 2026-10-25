// Package ddl verifies primary-key semantic rule behavior.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPrimaryKeyBigintRuleFindsNonBigintPrimaryKey(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
// shape when the member's column facts arrive through the target-table
// snapshot — the declared name resolves to a proven INT member and the
// predicate answers. The previous oracle asserted the same finding from a
// name-only stub (no type facts at all), which fabricated the violation;
// the corrected control adds the metadata path the rule now requires and
// keeps a no-facts control asserting silence.
func TestPostgreSQLAlterTableAddPrimaryKeyBigintRuleCoverage(t *testing.T) {
	t.Parallel()
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

	newStatement := func(metadata *spec.Metadata) spec.Statement {
		return spec.Statement{
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
			Metadata: metadata,
		}
	}

	statement := newStatement(&spec.Metadata{TargetTable: &spec.TableSnapshot{
		Exists:  true,
		Columns: []spec.Column{{Name: "id", Type: "int"}},
	}})

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

	// No proven column facts → no provable answer → silence, never a
	// violation computed from the zero-value stub.
	factsWithheld, err := statementRule.Evaluate(context.Background(), newStatement(nil))
	if err != nil {
		t.Fatalf("evaluate withheld: %v", err)
	}
	if len(factsWithheld) != 0 {
		t.Fatalf("withheld column facts must not fabricate a violation, got %v", factsWithheld)
	}
}

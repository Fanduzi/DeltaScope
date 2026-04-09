// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL DDL operations
// output: findings for PostgreSQL migration-safety risks (lock/rewrite/constraint validation)
// pos: PostgreSQL-specific migration-safety rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// ---------------------------------------------------------------------------
// Rule 1: ddl.pg.create_index.concurrently.require
// ---------------------------------------------------------------------------

type createIndexConcurrentlyRequiredRule struct {
	level rule.Level
}

func newCreateIndexConcurrentlyRequiredRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return createIndexConcurrentlyRequiredRule{
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r createIndexConcurrentlyRequiredRule) ID() string {
	return ruleIDPGCreateIndexConcurrentlyRequire
}

func (r createIndexConcurrentlyRequiredRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == spec.DDLOperationCreateIndex
}

func (r createIndexConcurrentlyRequiredRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if statement.DDL.Options["concurrently"] == "true" {
		return nil, nil
	}

	indexName := ""
	if len(statement.DDL.Indexes) > 0 {
		indexName = statement.DDL.Indexes[0].Name
	}
	tableName := ""
	if statement.DDL.Table != nil {
		tableName = statement.DDL.Table.Name
	}

	message := "CREATE INDEX without CONCURRENTLY can block writes on PostgreSQL"
	if indexName != "" {
		message = fmt.Sprintf("CREATE INDEX %q without CONCURRENTLY can block writes on PostgreSQL", indexName)
	}

	return []rule.Finding{{
		Level:   r.level,
		Message: message,
		Explanation: &rule.FindingExplanation{
			Why:        "PostgreSQL builds a non-concurrent index while holding stronger locks than most online migration workflows can tolerate.",
			Risk:       "Application writes to the indexed table can block during the entire index build window.",
			Suggestion: "Use CREATE INDEX CONCURRENTLY in a dedicated migration step when your PostgreSQL rollout policy allows it.",
		},
		Metadata: map[string]any{
			"operation":    "create_index",
			"index":        indexName,
			"table":        tableName,
			"concurrently": false,
		},
	}}, nil
}

// ---------------------------------------------------------------------------
// Rule 2: ddl.pg.alter.add_column.non_null_default.rewrite.warn
// ---------------------------------------------------------------------------

type addColumnNonNullDefaultRewriteWarnRule struct {
	level rule.Level
}

func newAddColumnNonNullDefaultRewriteWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return addColumnNonNullDefaultRewriteWarnRule{
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r addColumnNonNullDefaultRewriteWarnRule) ID() string {
	return ruleIDPGAlterAddColumnNonNullDefaultRewriteWarn
}

func (r addColumnNonNullDefaultRewriteWarnRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		appliesToAlterActions(statement, "add_column")
}

func (r addColumnNonNullDefaultRewriteWarnRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "add_column") {
		col, ok := alterColumnDefinition(alter)
		if !ok || !col.NotNull || !col.HasDefault {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: fmt.Sprintf("Adding NOT NULL column %q with a default may trigger a table rewrite on PostgreSQL", alter.Name),
			Explanation: &rule.FindingExplanation{
				Why:        "PostgreSQL must rewrite every row to evaluate and store the default value when a new column is both NOT NULL and has a default.",
				Risk:       "Large tables may experience significant downtime during the rewrite, blocking concurrent reads and writes.",
				Suggestion: "Split the migration: add the column as nullable, backfill data, then enforce NOT NULL in a separate step.",
			},
			Metadata: map[string]any{
				"action": "add_column",
				"column": alter.Name,
				"table":  statement.DDL.Table.Name,
			},
		})
	}
	return findings, nil
}

// ---------------------------------------------------------------------------
// Rule 3: ddl.pg.alter.add_check.not_valid.require
// ---------------------------------------------------------------------------

type addCheckNotValidRequiredRule struct {
	level rule.Level
}

func newAddCheckNotValidRequiredRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return addCheckNotValidRequiredRule{
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r addCheckNotValidRequiredRule) ID() string {
	return ruleIDPGAlterAddCheckNotValidRequire
}

func (r addCheckNotValidRequiredRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		appliesToAlterActions(statement, "add_constraint")
}

func (r addCheckNotValidRequiredRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "add_constraint") {
		if alter.Options["constraint_type"] != "check" {
			continue
		}
		if alter.Options["not_valid"] == "true" {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: fmt.Sprintf("CHECK constraint %q without NOT VALID validates all existing rows immediately on PostgreSQL", alter.Name),
			Explanation: &rule.FindingExplanation{
				Why:        "PostgreSQL validates a CHECK constraint against all existing rows at ADD CONSTRAINT time when NOT VALID is absent.",
				Risk:       "On large tables the validation scan can hold an ACCESS EXCLUSIVE lock, blocking reads and writes until it completes.",
				Suggestion: "Add the constraint with NOT VALID, then run VALIDATE CONSTRAINT in a separate step to spread the lock impact.",
			},
			Metadata: map[string]any{
				"action":           "add_constraint",
				"constraint":       alter.Name,
				"constraint_type":  "check",
				"not_valid":        false,
				"table":            statement.DDL.Table.Name,
			},
		})
	}
	return findings, nil
}

// ---------------------------------------------------------------------------
// Rule 4: ddl.pg.alter.set_data_type.rewrite.warn
// ---------------------------------------------------------------------------

type setDataTypeRewriteWarnRule struct {
	level rule.Level
}

func newSetDataTypeRewriteWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return setDataTypeRewriteWarnRule{
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r setDataTypeRewriteWarnRule) ID() string {
	return ruleIDPGAlterSetDataTypeRewriteWarn
}

func (r setDataTypeRewriteWarnRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		appliesToAlterActions(statement, "set_data_type")
}

func (r setDataTypeRewriteWarnRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "set_data_type") {
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: fmt.Sprintf("ALTER COLUMN %q SET DATA TYPE carries table rewrite risk on PostgreSQL", alter.Name),
			Explanation: &rule.FindingExplanation{
				Why:        "PostgreSQL ALTER TABLE ... ALTER COLUMN ... SET DATA TYPE typically rewrites the entire table to convert existing row data.",
				Risk:       "The rewrite holds an ACCESS EXCLUSIVE lock, blocking all concurrent access for the duration of the conversion.",
				Suggestion: "Review the rollout strategy: consider shadow-column or backfill approaches to avoid a single-statement rewrite window.",
			},
			Metadata: map[string]any{
				"action": "set_data_type",
				"column": alter.Name,
				"table":  statement.DDL.Table.Name,
			},
		})
	}
	return findings, nil
}

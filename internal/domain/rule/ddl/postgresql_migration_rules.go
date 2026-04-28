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
			Suggestion: "Use CREATE INDEX CONCURRENTLY to build the index without blocking writes. Note that CONCURRENTLY cannot run inside a transaction; run it as a standalone migration step.",
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
				Suggestion: "Split into safe steps: 1) ADD COLUMN as nullable with no default. 2) Backfill existing rows. 3) SET DEFAULT for new rows. 4) SET NOT NULL once all rows are populated.",
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
				Suggestion: "Use a two-step approach: 1) ADD CONSTRAINT ... NOT VALID to register the constraint without scanning existing rows. 2) VALIDATE CONSTRAINT in a separate step — it holds only a SHARE UPDATE EXCLUSIVE lock.",
			},
			Metadata: map[string]any{
				"action":          "add_constraint",
				"constraint":      alter.Name,
				"constraint_type": "check",
				"not_valid":       false,
				"table":           statement.DDL.Table.Name,
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
				Suggestion: "Assess table size and lock impact first. For large tables, use a phased migration: add a shadow column with the new type, backfill in batches, switch application reads, then drop the old column.",
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

// ---------------------------------------------------------------------------
// Rule 5: ddl.pg.drop_index.advisory
// ---------------------------------------------------------------------------

type dropIndexAdvisoryRule struct {
	level rule.Level
}

func newDropIndexAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return dropIndexAdvisoryRule{
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r dropIndexAdvisoryRule) ID() string { return ruleIDPGDropIndexAdvisory }

func (r dropIndexAdvisoryRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == spec.DDLOperationDropIndex
}

func (r dropIndexAdvisoryRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	indexName := ""
	for _, a := range statement.DDL.Alter {
		if a.Action == "drop_index" && a.Name != "" {
			indexName = a.Name
		}
	}

	message := "DROP INDEX acquires an ACCESS EXCLUSIVE lock on PostgreSQL"
	if indexName != "" {
		message = fmt.Sprintf("DROP INDEX %q acquires an ACCESS EXCLUSIVE lock on PostgreSQL", indexName)
	}

	return []rule.Finding{{
		Level:   r.level,
		Message: message,
		Explanation: &rule.FindingExplanation{
			Why:        "PostgreSQL DROP INDEX takes an ACCESS EXCLUSIVE lock, blocking all reads and writes on the indexed table until the index metadata is removed.",
			Risk:       "Concurrent queries that reference the indexed columns may stall until the lock is released.",
			Suggestion: "For production migrations, schedule DROP INDEX during a maintenance window or use a two-step approach: 1) CONCURRENTLY CREATE a replacement index if restructuring. 2) DROP the old index during low-traffic period.",
		},
		Metadata: map[string]any{
			"operation": "drop_index",
			"index":     indexName,
		},
	}}, nil
}

// ---------------------------------------------------------------------------
// Rule 6: ddl.pg.alter.add_column.non_null_no_default.warn
// ---------------------------------------------------------------------------

type addColumnNonNullNoDefaultWarnRule struct {
	level rule.Level
}

func newAddColumnNonNullNoDefaultWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return addColumnNonNullNoDefaultWarnRule{
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r addColumnNonNullNoDefaultWarnRule) ID() string {
	return ruleIDPGAlterAddColumnNonNullNoDefaultWarn
}

func (r addColumnNonNullNoDefaultWarnRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		appliesToAlterActions(statement, "add_column")
}

func (r addColumnNonNullNoDefaultWarnRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "add_column") {
		col, ok := alterColumnDefinition(alter)
		if !ok || !col.NotNull || col.HasDefault {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: fmt.Sprintf("Adding NOT NULL column %q without a DEFAULT fails on existing rows in PostgreSQL", alter.Name),
			Explanation: &rule.FindingExplanation{
				Why:        "PostgreSQL must evaluate the NOT NULL constraint against every existing row immediately. Without a DEFAULT, all existing rows violate the constraint and the statement aborts.",
				Risk:       "The ALTER TABLE fails outright if the table contains any rows, blocking the migration and leaving the schema unchanged.",
				Suggestion: "Use a phased migration: 1) ADD COLUMN as nullable. 2) Backfill values in batches. 3) SET DEFAULT for future inserts. 4) SET NOT NULL once all rows are populated.",
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
// Rule 7: ddl.pg.alter.add_unique_constraint.concurrent_index.advisory
// ---------------------------------------------------------------------------

type addUniqueConstraintAdvisoryRule struct {
	level rule.Level
}

func newAddUniqueConstraintAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return addUniqueConstraintAdvisoryRule{
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r addUniqueConstraintAdvisoryRule) ID() string {
	return ruleIDPGAlterAddUniqueConstraintConcurrentIndexAdvisory
}

func (r addUniqueConstraintAdvisoryRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		appliesToAlterActions(statement, "add_constraint")
}

func (r addUniqueConstraintAdvisoryRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "add_constraint") {
		if alter.Options["constraint_type"] != "unique" {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: fmt.Sprintf("ADD UNIQUE CONSTRAINT %q builds a unique index — consider CREATE UNIQUE INDEX CONCURRENTLY instead", alter.Name),
			Explanation: &rule.FindingExplanation{
				Why:        "PostgreSQL ADD CONSTRAINT ... UNIQUE acquires a SHARE ROW EXCLUSIVE lock while building the underlying unique index, which can block writes on the table.",
				Risk:       "On large tables, the index build can take significant time, blocking concurrent writes throughout.",
				Suggestion: "Prefer a two-step approach: 1) CREATE UNIQUE INDEX CONCURRENTLY to build the index without blocking writes. 2) ADD CONSTRAINT ... USING INDEX to attach the constraint to the pre-built index.",
			},
			Metadata: map[string]any{
				"action":          "add_constraint",
				"constraint":      alter.Name,
				"constraint_type": "unique",
				"table":           statement.DDL.Table.Name,
			},
		})
	}
	return findings, nil
}

// ---------------------------------------------------------------------------
// Rule 8: ddl.pg.alter.drop_constraint.advisory
// ---------------------------------------------------------------------------

type dropConstraintAdvisoryRule struct {
	level rule.Level
}

func newDropConstraintAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return dropConstraintAdvisoryRule{
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r dropConstraintAdvisoryRule) ID() string { return ruleIDPGAlterDropConstraintAdvisory }

func (r dropConstraintAdvisoryRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		appliesToAlterActions(statement, "drop_constraint")
}

func (r dropConstraintAdvisoryRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "drop_constraint") {
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: fmt.Sprintf("DROP CONSTRAINT %q on PostgreSQL may break application assumptions", alter.Name),
			Explanation: &rule.FindingExplanation{
				Why:        "Dropping a constraint removes a data integrity guarantee that application logic or downstream consumers may depend on.",
				Risk:       "Data that previously satisfied the constraint is no longer validated, potentially leading to data quality issues that are difficult to detect and remediate.",
				Suggestion: "Before dropping: 1) Verify no application code depends on the constraint guarantee. 2) Assess whether a softer deprecation (e.g., NOT VALID for CHECK) is safer. 3) Schedule the change during a maintenance window with monitoring.",
			},
			Metadata: map[string]any{
				"action":     "drop_constraint",
				"constraint": alter.Name,
				"table":      statement.DDL.Table.Name,
			},
		})
	}
	return findings, nil
}

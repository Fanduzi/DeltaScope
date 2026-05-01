// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL ALTER TABLE operations
// output: advisory and notice findings for PG-only alter table gap coverage
// pos: PostgreSQL-specific alter table advisory rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// ---------------------------------------------------------------------------
// Generic PG-only alter action advisory rule
// Covers: drop_column (warning), validate_constraint (notice)
// ---------------------------------------------------------------------------

type pgAlterActionAdvisoryRule struct {
	id         string
	level      rule.Level
	action     string
	object     string
	message    string
	why        string
	risk       string
	suggestion string
}

func newPGAlterActionAdvisoryRule(id string, level rule.Level, action string, object string, message string, why string, risk string, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgAlterActionAdvisoryRule{
		id:         id,
		level:      configuredLevel(cfg, level),
		action:     action,
		object:     object,
		message:    message,
		why:        why,
		risk:       risk,
		suggestion: suggestion,
	}, nil
}

func (r pgAlterActionAdvisoryRule) ID() string { return r.id }

func (r pgAlterActionAdvisoryRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		appliesToAlterActions(statement, r.action)
}

func (r pgAlterActionAdvisoryRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.action) {
		message := r.message
		if alter.Name != "" {
			message = fmt.Sprintf(r.message, alter.Name)
		}
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: message,
			Explanation: &rule.FindingExplanation{
				Why:        r.why,
				Risk:       r.risk,
				Suggestion: r.suggestion,
			},
			Metadata: map[string]any{
				"operation": "alter_table",
				"action":    r.action,
				"table":     statement.DDL.Table.Name,
				"target":    alter.Name,
			},
		})
	}
	return findings, nil
}

// ---------------------------------------------------------------------------
// Rule: ddl.pg.alter.add_column.nullable.notice
// ---------------------------------------------------------------------------

type pgAlterAddColumnNullableNoticeRule struct {
	level rule.Level
}

func newAddColumnNullableNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgAlterAddColumnNullableNoticeRule{
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r pgAlterAddColumnNullableNoticeRule) ID() string {
	return ruleIDPGAlterAddColumnNullableNotice
}

func (r pgAlterAddColumnNullableNoticeRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		appliesToAlterActions(statement, "add_column")
}

func (r pgAlterAddColumnNullableNoticeRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "add_column") {
		col, ok := alterColumnDefinition(alter)
		if !ok {
			continue
		}
		// Skip NOT NULL (covered by non_null_no_default.warn and non_null_default.rewrite.warn)
		if col.NotNull {
			continue
		}
		// Skip has-default (covered by non_null_default.rewrite.warn for NOT NULL;
		// nullable-with-default is schema expansion but not risky enough to notice)
		if col.HasDefault {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: fmt.Sprintf("Nullable column %q added without DEFAULT — schema expands silently on PostgreSQL", alter.Name),
			Explanation: &rule.FindingExplanation{
				Why:        "A nullable column without a DEFAULT is added to every existing row as NULL. While this is safe in PostgreSQL (no table rewrite), the schema change should be visible in review.",
				Risk:       "Application code that does not handle NULL for this column may encounter unexpected nil values.",
				Suggestion: "Consider whether a DEFAULT value would make the schema change self-documenting. If NULL is intentional, no action is needed — this notice is informational.",
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
// Constructors for register.go
// ---------------------------------------------------------------------------

func newDropColumnAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterDropColumnAdvisory, rule.LevelWarning, "drop_column", "column",
		"DROP COLUMN %q removes data permanently on PostgreSQL",
		"Dropping a column in PostgreSQL marks it as logically dead. Existing rows retain the data on disk until VACUUM reclaims space, but the column is immediately invisible to queries.",
		"Any application code, views, or stored procedures referencing this column will break. Data in the dropped column is permanently lost and cannot be recovered without a backup.",
		"Verify no application code, views, or stored procedures reference this column before dropping. Consider a two-step deprecation: 1) Mark the column as deprecated in documentation. 2) Drop in a subsequent release after all references are removed.",
		cfg,
	)
}

func newValidateConstraintAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterValidateConstraintAdvisory, rule.LevelNotice, "validate_constraint", "constraint",
		"VALIDATE CONSTRAINT %q scans existing rows on PostgreSQL",
		"VALIDATE CONSTRAINT checks all existing rows against the constraint definition. It holds a SHARE UPDATE EXCLUSIVE lock, which is less restrictive than ACCESS EXCLUSIVE but still blocks certain DDL.",
		"On large tables the validation scan can take significant time, though it does not block reads or writes.",
		"Schedule VALIDATE CONSTRAINT during low-traffic periods for large tables. Ensure the constraint was added with NOT VALID first for safe phased deployment.",
		cfg,
	)
}

func newSetSchemaAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterSetSchemaAdvisory, rule.LevelNotice, "set_schema", "schema",
		"ALTER TABLE SET SCHEMA moves table %q to a different schema on PostgreSQL",
		"SET SCHEMA changes the schema containing the table. Dependent objects (views, foreign keys, functions referencing the table by schema-qualified name) may break after the move.",
		"Applications that reference the table by its old schema-qualified name will fail. Existing prepared statements may be invalidated.",
		"Verify all schema-qualified references are updated after the move. Consider whether renaming or a synonym would be less disruptive.",
		cfg,
	)
}

func newOwnerAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterOwnerAdvisory, rule.LevelNotice, "change_owner", "owner",
		"ALTER TABLE OWNER TO changes ownership of table %q on PostgreSQL",
		"OWNER TO transfers table ownership to a different role. Permissions, default privileges, and role-based access policies may be affected.",
		"The new owner gains full control over the table. Scripts or policies expecting the previous owner may behave differently.",
		"Verify the new role has appropriate permissions and that no automation depends on the current owner.",
		cfg,
	)
}

func newEnableTriggerNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterEnableTriggerNotice, rule.LevelNotice, "enable_trigger", "trigger",
		"Trigger %q enabled on PostgreSQL — verify intent",
		"Enabling a trigger re-activates its firing logic for all subsequent DML operations on the table. This may change data mutation behavior immediately.",
		"Application code that relied on the trigger being disabled will now observe side effects from the trigger.",
		"Confirm the trigger logic is compatible with current data and application expectations before enabling.",
		cfg,
	)
}

func newDisableTriggerWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterDisableTriggerWarn, rule.LevelWarning, "disable_trigger", "trigger",
		"Trigger %q disabled on PostgreSQL — data integrity may be affected",
		"Disabling a trigger stops its firing logic for all subsequent DML operations. Audit trails, data validation, or business-logic triggers will no longer execute.",
		"Data integrity constraints enforced by the trigger are no longer active. Rows modified while the trigger is disabled will bypass its checks.",
		"Re-enable the trigger as soon as the maintenance window ends. Document the reason for disabling and verify data consistency after re-enabling.",
		cfg,
	)
}

func newAttachPartitionAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterAttachPartitionAdvisory, rule.LevelNotice, "attach_partition", "partition",
		"Partition %q attached on PostgreSQL — verify boundary and data",
		"ATTACH PARTITION makes an existing table a partition of the parent table. PostgreSQL validates that existing data satisfies the partition bound, which may trigger a full table scan.",
		"On large tables the validation scan holds a SHARE UPDATE EXCLUSIVE lock. Incorrect boundaries can cause future DML to route to the wrong partition.",
		"Verify the partition bound covers exactly the intended data range. Schedule during low-traffic periods for large tables.",
		cfg,
	)
}

func newDetachPartitionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterDetachPartitionWarn, rule.LevelWarning, "detach_partition", "partition",
		"Partition %q detached on PostgreSQL — queries targeting parent may lose coverage",
		"DETACH PARTITION removes a partition from the parent table. Queries against the parent table will no longer include data from the detached partition.",
		"Applications expecting the detached partition's data to appear in parent-table queries will see missing rows. Referential integrity constraints referencing the detached data may be affected.",
		"Verify no queries depend on the detached partition's data being visible through the parent. Consider whether archiving or dropping the detached table is appropriate.",
		cfg,
	)
}

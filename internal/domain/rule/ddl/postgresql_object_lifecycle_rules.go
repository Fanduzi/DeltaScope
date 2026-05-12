// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL object lifecycle DDL operations
// output: findings for PostgreSQL schema, sequence, and materialized view lifecycle risks
// pos: PostgreSQL-specific object lifecycle rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgObjectLifecycleRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	option     string
	action     string
	object     string
	objectType string
	message    string
	why        string
	risk       string
	suggestion string
}

func newPGObjectLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, option string, object string, message string, why string, risk string, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgObjectLifecycleRule{
		id:         id,
		level:      configuredLevel(cfg, level),
		operation:  operation,
		option:     option,
		object:     object,
		message:    message,
		why:        why,
		risk:       risk,
		suggestion: suggestion,
	}, nil
}

func newPGObjectLifecycleRuleWithType(id string, level rule.Level, operation spec.DDLOperation, option string, object string, objectType string, message string, why string, risk string, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgObjectLifecycleRule{
		id:         id,
		level:      configuredLevel(cfg, level),
		operation:  operation,
		option:     option,
		object:     object,
		objectType: objectType,
		message:    message,
		why:        why,
		risk:       risk,
		suggestion: suggestion,
	}, nil
}

func newPGObjectLifecycleRuleWithAction(id string, level rule.Level, operation spec.DDLOperation, action string, object string, objectType string, message string, why string, risk string, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgObjectLifecycleRule{
		id:         id,
		level:      configuredLevel(cfg, level),
		operation:  operation,
		action:     action,
		object:     object,
		objectType: objectType,
		message:    message,
		why:        why,
		risk:       risk,
		suggestion: suggestion,
	}, nil
}

func (r pgObjectLifecycleRule) ID() string { return r.id }

func (r pgObjectLifecycleRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectPostgreSQL ||
		statement.Kind != spec.KindDDL ||
		statement.DDL == nil ||
		statement.DDL.Operation != r.operation {
		return false
	}
	if r.objectType != "" && statement.DDL.ObjectType != r.objectType {
		return false
	}
	return true
}

func (r pgObjectLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if r.option != "" && statement.DDL.Options[r.option] != "true" {
		return nil, nil
	}
	if r.action != "" && statement.DDL.Options["action"] != r.action {
		return nil, nil
	}

	objectName := statement.DDL.ObjectName
	var message string
	if objectName != "" {
		message = fmt.Sprintf(r.message, objectName)
	} else {
		message = fmt.Sprintf("%s on PostgreSQL", r.object)
	}

	return []rule.Finding{{
		Level:   r.level,
		Message: message,
		Explanation: &rule.FindingExplanation{
			Why:        r.why,
			Risk:       r.risk,
			Suggestion: r.suggestion,
		},
		Metadata: map[string]any{
			"operation":   string(r.operation),
			"object_type": r.object,
			"object_name": objectName,
		},
	}}, nil
}

func newCreateSchemaNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithType(
		ruleIDPGCreateSchemaNotice, rule.LevelNotice, spec.DDLOperationCreateSchema, "",
		"schema", "schema",
		"CREATE SCHEMA %q changes namespace and security boundaries on PostgreSQL",
		"Creating a schema adds a new namespace that affects object resolution, search_path behavior, and security boundaries.",
		"New schemas change how PostgreSQL resolves unqualified object names and may expose or hide objects unexpectedly depending on search_path configuration.",
		"Review whether the schema name follows naming conventions and whether GRANTs are needed to control access.",
		cfg,
	)
}

func newDropSchemaAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRule(
		ruleIDPGDropSchemaAdvisory, rule.LevelNotice, spec.DDLOperationDropSchema, "",
		"schema",
		"DROP SCHEMA %q removes all contained objects on PostgreSQL",
		"DROP SCHEMA irreversibly removes the schema and every object it contains (tables, views, sequences, functions).",
		"All dependent objects are destroyed. Downstream queries and application code referencing objects in this schema will break.",
		"Verify no application or service references objects in this schema before dropping. Consider a two-step approach: 1) Move needed objects to another schema. 2) Drop the empty schema during a maintenance window.",
		cfg,
	)
}

func newDropSchemaCascadeWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRule(
		ruleIDPGDropSchemaCascadeWarn, rule.LevelWarning, spec.DDLOperationDropSchema, "cascade",
		"schema",
		"DROP SCHEMA %q CASCADE automatically drops all dependent objects on PostgreSQL",
		"CASCADE forces PostgreSQL to drop every object that depends on the schema, including objects in other schemas that reference it.",
		"Cascading drops can silently destroy objects beyond the target schema, including foreign-key-referenced tables and views, causing unpredictable runtime failures.",
		"Avoid CASCADE in production migrations. Explicitly drop dependent objects first so the blast radius is visible and reviewed.",
		cfg,
	)
}

func newCreateSequenceCycleWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRule(
		ruleIDPGCreateSequenceCycleWarn, rule.LevelWarning, spec.DDLOperationCreateSequence, "cycle",
		"sequence",
		"CREATE SEQUENCE %q with CYCLE may produce duplicate values on PostgreSQL",
		"A CYCLIC sequence wraps around to its minimum value after reaching the maximum, reusing values that may already exist in generated or referenced data.",
		"Downstream consumers that assume uniqueness (primary keys, surrogate references) can encounter duplicate values, leading to constraint violations or silent data corruption.",
		"Only use CYCLE for intentionally repeating sequences (e.g., round-robin partition keys). For identity or primary-key sequences, omit CYCLE.",
		cfg,
	)
}

func newAlterSequenceRestartWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRule(
		ruleIDPGAlterSequenceRestartWarn, rule.LevelWarning, spec.DDLOperationAlterSequence, "restart",
		"sequence",
		"ALTER SEQUENCE %q RESTART changes the current value on PostgreSQL",
		"RESTART resets the sequence counter, causing subsequent calls to nextval() to return values starting from the restart point.",
		"Restarting a sequence can produce values that collide with existing rows if the restart value is not set above the current maximum key in referencing tables.",
		"Before restarting, verify the new start value is above the maximum existing key in any table that uses this sequence for primary keys. Use: SELECT MAX(id) FROM referencing_table.",
		cfg,
	)
}

func newAlterSequenceCycleWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRule(
		ruleIDPGAlterSequenceCycleWarn, rule.LevelWarning, spec.DDLOperationAlterSequence, "cycle",
		"sequence",
		"ALTER SEQUENCE %q with CYCLE may produce duplicate values on PostgreSQL",
		"Enabling CYCLE on an existing sequence changes its wrap behavior, reusing values that downstream consumers may already treat as unique.",
		"Downstream consumers that assume uniqueness (primary keys, surrogate references) can encounter duplicate values, leading to constraint violations or silent data corruption.",
		"Only enable CYCLE for intentionally repeating sequences. For identity or primary-key sequences, keep CYCLE off.",
		cfg,
	)
}

func newDropSequenceAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRule(
		ruleIDPGDropSequenceAdvisory, rule.LevelNotice, spec.DDLOperationDropSequence, "",
		"sequence",
		"DROP SEQUENCE %q removes the sequence generator on PostgreSQL",
		"Dropping a sequence that is still referenced by a column DEFAULT or an identity column will cause subsequent inserts to fail.",
		"Any table column using this sequence as a default or identity source will raise an error on next INSERT.",
		"Before dropping: 1) Find columns referencing this sequence via pg_depend. 2) Alter or drop those column defaults first. 3) Drop the sequence once no dependencies remain.",
		cfg,
	)
}

func newDropSequenceCascadeWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRule(
		ruleIDPGDropSequenceCascadeWarn, rule.LevelWarning, spec.DDLOperationDropSequence, "cascade",
		"sequence",
		"DROP SEQUENCE %q CASCADE silently removes dependent column defaults on PostgreSQL",
		"CASCADE forces PostgreSQL to drop the sequence and silently remove any column DEFAULT or identity linkage that references it.",
		"Dependent columns lose their default values without warning, causing inserts to fail or insert NULL where a generated value was expected.",
		"Avoid CASCADE. Explicitly ALTER columns to remove DEFAULT or identity linkage first so the impact is visible and reviewed.",
		cfg,
	)
}

func newDropMaterializedViewAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRule(
		ruleIDPGDropMaterializedViewAdvisory, rule.LevelNotice, spec.DDLOperationDropMaterializedView, "",
		"materialized view",
		"DROP MATERIALIZED VIEW %q removes cached query results on PostgreSQL",
		"Dropping a materialized view removes precomputed query results that may be used by reporting pipelines or downstream consumers.",
		"Queries, dashboards, or ETL steps that depend on this materialized view will fail after it is dropped.",
		"Before dropping: 1) Check for downstream queries referencing this materialized view. 2) Consider whether a scheduled REFRESH is sufficient instead. 3) Drop during a maintenance window with dependent consumers notified.",
		cfg,
	)
}

func newDropMaterializedViewCascadeWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRule(
		ruleIDPGDropMaterializedViewCascadeWarn, rule.LevelWarning, spec.DDLOperationDropMaterializedView, "cascade",
		"materialized view",
		"DROP MATERIALIZED VIEW %q CASCADE automatically drops dependent objects on PostgreSQL",
		"CASCADE forces PostgreSQL to drop any views or other objects that depend on this materialized view.",
		"Cascading drops can silently destroy dependent views, breaking downstream queries and reporting pipelines beyond the intended target.",
		"Avoid CASCADE in production migrations. Explicitly drop dependent views first so the full blast radius is visible and reviewed.",
		cfg,
	)
}

func newAlterSchemaRenameNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithAction(
		ruleIDPGAlterSchemaRenameNotice, rule.LevelNotice, spec.DDLOperationAlterSchema, "rename",
		"schema", "schema",
		"ALTER SCHEMA %q RENAME changes namespace resolution on PostgreSQL",
		"Renaming a schema changes how PostgreSQL resolves unqualified object names for every object it contains.",
		"Downstream queries, views, and application code that reference the old schema name will break until updated.",
		"Before renaming, search for all references to the old schema name across application code, views, and functions.",
		cfg,
	)
}

func newAlterSchemaOwnerNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithAction(
		ruleIDPGAlterSchemaOwnerNotice, rule.LevelNotice, spec.DDLOperationAlterSchema, "set_owner",
		"schema", "schema",
		"ALTER SCHEMA %q OWNER TO changes schema ownership on PostgreSQL",
		"Changing schema ownership transfers all privileges and default permissions associated with the schema.",
		"The new owner gains full control over the schema and can drop or modify contained objects.",
		"Verify the new owner role has appropriate permissions and that no access control policies are violated.",
		cfg,
	)
}

func newAlterIndexRenameNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithAction(
		ruleIDPGAlterIndexRenameNotice, rule.LevelNotice, spec.DDLOperationAlterIndex, "rename",
		"index", "index",
		"ALTER INDEX %q RENAME changes index identification on PostgreSQL",
		"Renaming an index changes how it is identified in query plans and monitoring dashboards.",
		"Application code or monitoring that references the old index name will need updating.",
		"Update any monitoring alerts or documentation that reference the old index name.",
		cfg,
	)
}

func newAlterIndexSetTablespaceNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithAction(
		ruleIDPGAlterIndexSetTablespaceNotice, rule.LevelNotice, spec.DDLOperationAlterIndex, "set_tablespace",
		"index", "index",
		"ALTER INDEX %q SET TABLESPACE moves index storage on PostgreSQL",
		"Moving an index to a different tablespace changes its physical storage location.",
		"The operation rewrites the index to the new tablespace, which can be I/O-intensive on large indexes.",
		"Schedule tablespace moves during low-traffic windows and verify the target tablespace has sufficient space.",
		cfg,
	)
}

func newAlterMaterializedViewRenameNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithAction(
		ruleIDPGAlterMaterializedViewRenameNotice, rule.LevelNotice, spec.DDLOperationAlterMaterializedView, "rename",
		"materialized view", "materialized_view",
		"ALTER MATERIALIZED VIEW %q RENAME changes materialized view identification on PostgreSQL",
		"Renaming a materialized view changes how downstream queries and reporting pipelines reference it.",
		"Queries, dashboards, or ETL steps that reference the old materialized view name will fail after renaming.",
		"Update all downstream references before renaming. Consider creating a view with the old name as an alias during transition.",
		cfg,
	)
}

func newAlterMaterializedViewSetSchemaNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithAction(
		ruleIDPGAlterMaterializedViewSetSchemaNotice, rule.LevelNotice, spec.DDLOperationAlterMaterializedView, "set_schema",
		"materialized view", "materialized_view",
		"ALTER MATERIALIZED VIEW %q SET SCHEMA changes namespace resolution on PostgreSQL",
		"Moving a materialized view to a different schema changes how PostgreSQL resolves unqualified references to it.",
		"Downstream queries and reporting pipelines that reference the materialized view without a schema qualifier may resolve to a different object or fail.",
		"Update all schema-qualified references before moving. Verify no unqualified references depend on the current schema.",
		cfg,
	)
}

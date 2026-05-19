// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL ALTER TABLE operations
// output: advisory and notice findings for PG-only alter table gap coverage
// pos: PostgreSQL-specific alter table advisory rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
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

func (r pgAlterActionAdvisoryRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.action) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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

func (r pgAlterAddColumnNullableNoticeRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "add_column") {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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

// ---------------------------------------------------------------------------
// Constructors for logged-state rules
// ---------------------------------------------------------------------------

func newSetLoggedNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterLoggedNotice, rule.LevelNotice, "set_logged", "table",
		"ALTER TABLE SET LOGGED on table %q — table will be crash-safe on PostgreSQL",
		"SET LOGGED marks the table as logged, meaning writes are written to the write-ahead log and survive crashes. This is the default for permanent tables.",
		"Temporary or unlogged tables switched to logged will begin generating WAL. On large tables this may increase WAL volume significantly.",
		"Verify that the increased WAL volume is acceptable for the replication and backup infrastructure.",
		cfg,
	)
}

func newSetUnloggedNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterUnloggedNotice, rule.LevelNotice, "set_unlogged", "table",
		"ALTER TABLE SET UNLOGGED on table %q — table will NOT survive crashes on PostgreSQL",
		"SET UNLOGGED marks the table as unlogged, meaning writes are not written to the write-ahead log. The table is automatically truncated after a crash or unclean shutdown.",
		"All data in this table will be lost after a crash, power failure, or unclean restart. Replication streams will not include changes to this table.",
		"Ensure no business-critical data is stored in this table. Use unlogged tables only for ephemeral data (caches, session stores, temporary computation results) that can be rebuilt.",
		cfg,
	)
}

// ---------------------------------------------------------------------------
// Storage layout rules: SET TABLESPACE, SET ACCESS METHOD
// ---------------------------------------------------------------------------

type pgAlterStorageLayoutRule struct {
	id         string
	level      rule.Level
	action     string
	optionKey  string
	message    string
	why        string
	risk       string
	suggestion string
}

func newPGAlterStorageLayoutRule(id string, level rule.Level, action, optionKey, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgAlterStorageLayoutRule{
		id:         id,
		level:      configuredLevel(cfg, level),
		action:     action,
		optionKey:  optionKey,
		message:    message,
		why:        why,
		risk:       risk,
		suggestion: suggestion,
	}, nil
}

func (r pgAlterStorageLayoutRule) ID() string { return r.id }

func (r pgAlterStorageLayoutRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		appliesToAlterActions(statement, r.action)
}

func (r pgAlterStorageLayoutRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.action) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		metadata := map[string]any{
			"operation": "alter_table",
			"action":    r.action,
			"table":     statement.DDL.Table.Name,
		}
		for k, v := range alter.Options {
			metadata[k] = v
		}
		value := alter.Options[r.optionKey]
		message := fmt.Sprintf(r.message, value)
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: message,
			Explanation: &rule.FindingExplanation{
				Why:        r.why,
				Risk:       r.risk,
				Suggestion: r.suggestion,
			},
			Metadata: metadata,
		})
	}
	return findings, nil
}

func newSetTablespaceNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterStorageLayoutRule(
		ruleIDPGAlterSetTablespaceNotice, rule.LevelNotice, "set_tablespace", "tablespace",
		"ALTER TABLE SET TABLESPACE %s on PostgreSQL — table files will be moved",
		"SET TABLESPACE moves all data files for the table to the named tablespace. This requires an ACCESS EXCLUSIVE lock and copies all data to the new location.",
		"The table is inaccessible during the move. On large tables this can cause significant downtime. The target tablespace must have enough disk space.",
		"Schedule during a maintenance window. Verify the target tablespace exists and has sufficient capacity. Consider whether the move is necessary or if the current tablespace is adequate.",
		cfg,
	)
}

func newSetAccessMethodWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterStorageLayoutRule(
		ruleIDPGAlterSetAccessMethodWarn, rule.LevelWarning, "set_access_method", "access_method",
		"ALTER TABLE SET ACCESS METHOD %s on PostgreSQL — table will be rewritten",
		"SET ACCESS METHOD changes how the table's data is stored on disk. PostgreSQL rewrites the entire table using the new access method, requiring an ACCESS EXCLUSIVE lock.",
		"The table is inaccessible during the rewrite. All data is copied, which can be expensive for large tables. Changing to an incompatible access method may cause data loss or corruption.",
		"Verify the target access method is installed and compatible with the table's schema. Schedule during a maintenance window and test on a non-production copy first.",
		cfg,
	)
}

func newSetReloptionsWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterStorageLayoutRule(
		ruleIDPGAlterSetReloptionsWarn, rule.LevelWarning, "set_reloptions", "option_count",
		"ALTER TABLE SET (...) on PostgreSQL — %s storage option(s) changed",
		"SET (reloptions) changes table storage parameters such as fillfactor or autovacuum settings. These affect how PostgreSQL stores and maintains the table on disk.",
		"Incorrect storage parameters can degrade performance or cause unexpected behavior. Some options require a table rewrite (e.g. fillfactor) or take effect only after the next VACUUM.",
		"Review each storage parameter change against the table's workload profile. Apply fillfactor changes during a maintenance window if the table is large.",
		cfg,
	)
}

func newResetReloptionsNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterStorageLayoutRule(
		ruleIDPGAlterResetReloptionsNotice, rule.LevelNotice, "reset_reloptions", "reset_count",
		"ALTER TABLE RESET (...) on PostgreSQL — %s storage option(s) reset to default",
		"RESET (reloptions) reverts table storage parameters to their PostgreSQL defaults. The table continues to use previously set values until the next VACUUM or ANALYZE for some parameters.",
		"Resetting parameters that were tuned for a specific workload may cause performance regression if the defaults are suboptimal for the table's access patterns.",
		"Verify the default values are acceptable for the table's workload. Monitor query performance after resetting tuned parameters.",
		cfg,
	)
}

// ---------------------------------------------------------------------------
// Constructors for trigger mode rules
// ---------------------------------------------------------------------------

func newEnableReplicaTriggerNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterStorageLayoutRule(
		ruleIDPGAlterEnableReplicaTriggerNotice, rule.LevelNotice, "enable_replica_trigger", "trigger",
		"Trigger %s enabled in REPLICA mode on PostgreSQL — replication firing mode changed",
		"ENABLE REPLICA TRIGGER configures the trigger to fire only in replica mode. The trigger will fire on standby servers that are in hot standby mode but not on the primary.",
		"Applications expecting the trigger to fire on the primary will not observe side effects. Replication behavior depends on the session_replication_role setting.",
		"Verify the trigger firing mode matches the intended replication topology. This mode is typically used for trigger-based replication setups.",
		cfg,
	)
}

func newEnableAlwaysTriggerNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterStorageLayoutRule(
		ruleIDPGAlterEnableAlwaysTriggerNotice, rule.LevelNotice, "enable_always_trigger", "trigger",
		"Trigger %s enabled in ALWAYS mode on PostgreSQL — trigger fires regardless of replication role",
		"ENABLE ALWAYS TRIGGER configures the trigger to fire regardless of the current session_replication_role setting. The trigger fires on the primary, on replicas, and during recovery.",
		"The trigger will execute in all contexts, which may cause unexpected behavior if the trigger logic assumes it is running only on the primary.",
		"Confirm the trigger logic is safe to execute in all replication contexts. This mode is typically used for audit or data-synchronization triggers that must fire unconditionally.",
		cfg,
	)
}

// ---------------------------------------------------------------------------
// Constructors for rule mode rules
// ---------------------------------------------------------------------------

func newEnableRuleNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterStorageLayoutRule(
		ruleIDPGAlterEnableRuleNotice, rule.LevelNotice, "enable_rule", "rule",
		"Rule %s enabled on PostgreSQL — query rewriting re-activated",
		"ENABLE RULE re-activates the named rewrite rule for all subsequent queries targeting the table. Query routing or transformation logic will resume immediately.",
		"Applications that relied on the rule being disabled will now observe query rewriting behavior that was previously absent.",
		"Confirm the rule logic is compatible with current query patterns and application expectations before enabling.",
		cfg,
	)
}

func newDisableRuleWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterStorageLayoutRule(
		ruleIDPGAlterDisableRuleWarn, rule.LevelWarning, "disable_rule", "rule",
		"Rule %s disabled on PostgreSQL — query rewriting may be affected",
		"DISABLE RULE deactivates the named rewrite rule for all subsequent queries targeting the table. Query routing or transformation logic will no longer apply.",
		"Applications expecting the rule to rewrite queries will observe unmodified query behavior. Views or application logic that depend on rule-based routing may return different results.",
		"Re-enable the rule as soon as the maintenance window ends. Document the reason for disabling and verify query behavior after re-enabling.",
		cfg,
	)
}

func newEnableReplicaRuleNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterStorageLayoutRule(
		ruleIDPGAlterEnableReplicaRuleNotice, rule.LevelNotice, "enable_replica_rule", "rule",
		"Rule %s enabled in REPLICA mode on PostgreSQL — replication rule mode changed",
		"ENABLE REPLICA RULE configures the rewrite rule to fire only in replica mode. The rule applies on standby servers that are in hot standby mode but not on the primary.",
		"Applications expecting the rule to rewrite queries on the primary will not observe transformation behavior. Replication behavior depends on the session_replication_role setting.",
		"Verify the rule mode matches the intended replication topology. This mode is typically used for rule-based replication setups.",
		cfg,
	)
}

func newEnableAlwaysRuleNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterStorageLayoutRule(
		ruleIDPGAlterEnableAlwaysRuleNotice, rule.LevelNotice, "enable_always_rule", "rule",
		"Rule %s enabled in ALWAYS mode on PostgreSQL — rule fires regardless of replication role",
		"ENABLE ALWAYS RULE configures the rewrite rule to fire regardless of the current session_replication_role setting. The rule applies on the primary, on replicas, and during recovery.",
		"The rule will execute in all contexts, which may cause unexpected query rewriting if the rule logic assumes it is running only on the primary.",
		"Confirm the rule logic is safe to execute in all replication contexts. This mode is typically used for audit or data-transformation rules that must fire unconditionally.",
		cfg,
	)
}

// ---------------------------------------------------------------------------
// Generic PG-only alter action+option rule
// Covers: replica_identity_full, replica_identity_nothing, replica_identity_using_index
// ---------------------------------------------------------------------------

type pgAlterActionOptionRule struct {
	id          string
	level       rule.Level
	action      string
	optionKey   string
	optionValue string
	message     string
	why         string
	risk        string
	suggestion  string
}

func newPGAlterActionOptionRule(id string, level rule.Level, action, optionKey, optionValue, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgAlterActionOptionRule{
		id:          id,
		level:       configuredLevel(cfg, level),
		action:      action,
		optionKey:   optionKey,
		optionValue: optionValue,
		message:     message,
		why:         why,
		risk:        risk,
		suggestion:  suggestion,
	}, nil
}

func (r pgAlterActionOptionRule) ID() string { return r.id }

func (r pgAlterActionOptionRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		appliesToAlterActions(statement, r.action)
}

func (r pgAlterActionOptionRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.action) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if alter.Options[r.optionKey] != r.optionValue {
			continue
		}
		metadata := map[string]any{
			"operation": "alter_table",
			"action":    r.action,
			r.optionKey: r.optionValue,
		}
		if statement.DDL != nil && statement.DDL.Table != nil {
			metadata["table"] = statement.DDL.Table.Name
		}
		if idx := alter.Options["index"]; idx != "" {
			metadata["index"] = idx
		}
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: r.message,
			Explanation: &rule.FindingExplanation{
				Why:        r.why,
				Risk:       r.risk,
				Suggestion: r.suggestion,
			},
			Metadata: metadata,
		})
	}
	return findings, nil
}

// ---------------------------------------------------------------------------
// Constructors for replica identity rules
// ---------------------------------------------------------------------------

func newReplicaIdentityFullWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionOptionRule(
		ruleIDPGAlterReplicaIdentityFullWarn, rule.LevelWarning,
		"replica_identity", "identity", "full",
		"REPLICA IDENTITY FULL enabled on PostgreSQL — logical replication may emit full row images",
		"REPLICA IDENTITY FULL causes PostgreSQL to use the entire old row as replica identity for UPDATE and DELETE when no suitable key is available.",
		"Logical replication or CDC streams can grow significantly for wide or high-churn tables.",
		"Prefer DEFAULT or USING INDEX when a stable key exists. Use FULL only when downstream consumers require it and the write volume is acceptable.",
		cfg,
	)
}

func newReplicaIdentityNothingWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionOptionRule(
		ruleIDPGAlterReplicaIdentityNothingWarn, rule.LevelWarning,
		"replica_identity", "identity", "nothing",
		"REPLICA IDENTITY NOTHING enabled on PostgreSQL — UPDATE/DELETE row identity may be unavailable",
		"REPLICA IDENTITY NOTHING records no row identity for logical decoding of UPDATE and DELETE operations.",
		"Downstream CDC or logical replication consumers may be unable to identify changed or deleted rows.",
		"Use DEFAULT or USING INDEX unless downstream systems explicitly do not require UPDATE/DELETE identity.",
		cfg,
	)
}

func newReplicaIdentityUsingIndexNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionOptionRule(
		ruleIDPGAlterReplicaIdentityUsingIndexNotice, rule.LevelNotice,
		"replica_identity", "identity", "using_index",
		"REPLICA IDENTITY USING INDEX configured on PostgreSQL — verify the selected index",
		"USING INDEX makes a specific index the replica identity for logical decoding.",
		"DeltaScope audits offline and cannot verify whether the named index exists, is valid, unique, not partial, and suitable for replica identity.",
		"Confirm the index is valid and intentionally chosen before relying on this replica identity configuration.",
		cfg,
	)
}

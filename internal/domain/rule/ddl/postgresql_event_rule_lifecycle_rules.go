// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL event trigger and rewrite rule lifecycle DDL operations
// output: findings for PostgreSQL event trigger and rule lifecycle events
// pos: PostgreSQL-specific event trigger and rewrite rule lifecycle rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgEventRuleLifecycleRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	action     string // "" for any action, "disable" for disable-specific, "rename" for rename-specific
	message    string
	why        string
	risk       string
	suggestion string
}

func newPGEventRuleLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, action string, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgEventRuleLifecycleRule{
		id:         id,
		level:      configuredLevel(cfg, level),
		operation:  operation,
		action:     action,
		message:    message,
		why:        why,
		risk:       risk,
		suggestion: suggestion,
	}, nil
}

func (r pgEventRuleLifecycleRule) ID() string { return r.id }

func (r pgEventRuleLifecycleRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectPostgreSQL ||
		statement.Kind != spec.KindDDL ||
		statement.DDL == nil ||
		statement.DDL.Operation != r.operation {
		return false
	}
	if r.action == "" {
		return true
	}
	if r.action == "non_disable" {
		actualAction := statement.DDL.Options["action"]
		return actualAction != "" && actualAction != "disable"
	}
	actualAction := statement.DDL.Options["action"]
	return actualAction == r.action
}

func (r pgEventRuleLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	objectName := statement.DDL.ObjectName
	message := fmt.Sprintf(r.message, objectName)

	metadata := map[string]any{
		"operation":   string(statement.DDL.Operation),
		"object_type": statement.DDL.ObjectType,
		"object_name": objectName,
	}
	for _, key := range []string{"action", "event", "function", "new_name", "table", "instead", "replace", "if_exists", "cascade"} {
		if val, ok := statement.DDL.Options[key]; ok && val != "" {
			metadata[key] = val
		}
	}

	return []rule.Finding{{
		Level:   r.level,
		Message: message,
		Explanation: &rule.FindingExplanation{
			Why:        r.why,
			Risk:       r.risk,
			Suggestion: r.suggestion,
		},
		Metadata: metadata,
	}}, nil
}

func newCreateEventTriggerNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGEventRuleLifecycleRule(
		ruleIDPGCreateEventTriggerNotice, rule.LevelNotice, spec.DDLOperationCreateEventTrigger, "",
		"PostgreSQL event trigger %q created — DDL event hook registered",
		"Event triggers fire on DDL events such as ddl_command_start and ddl_command_end. They execute a user-defined function when the event occurs.",
		"Event triggers can intercept, log, or block DDL operations. Misconfigured event triggers may disrupt schema changes or introduce unexpected side effects.",
		"Verify that the event trigger function is reviewed and the event scope is intentional.",
		cfg,
	)
}

func newAlterEventTriggerNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGEventRuleLifecycleRule(
		ruleIDPGAlterEventTriggerNotice, rule.LevelNotice, spec.DDLOperationAlterEventTrigger, "non_disable",
		"PostgreSQL event trigger %q altered — DDL event hook configuration changed",
		"ALTER EVENT TRIGGER changes the configuration of a DDL event trigger, such as enabling, renaming, or adjusting replica/always modes.",
		"Changing event trigger configuration may affect DDL audit logging, governance hooks, or monitoring expectations.",
		"Review the change to ensure event trigger governance remains effective.",
		cfg,
	)
}

func newAlterEventTriggerDisableWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGEventRuleLifecycleRule(
		ruleIDPGAlterEventTriggerDisableWarn, rule.LevelWarning, spec.DDLOperationAlterEventTrigger, "disable",
		"PostgreSQL event trigger %q disabled — DDL event hook deactivated",
		"ALTER EVENT TRIGGER DISABLE deactivates an event trigger, removing its DDL event interception.",
		"Disabling event triggers may bypass audit logging or DDL governance that depends on the trigger. This can mask unauthorized schema changes.",
		"Verify that disabling the event trigger does not compromise DDL audit or governance requirements.",
		cfg,
	)
}

func newDropEventTriggerWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGEventRuleLifecycleRule(
		ruleIDPGDropEventTriggerWarn, rule.LevelWarning, spec.DDLOperationDropEventTrigger, "",
		"PostgreSQL event trigger %q dropped — DDL event hook removed",
		"DROP EVENT TRIGGER permanently removes a DDL event trigger from the database.",
		"Dropping event triggers removes DDL audit or governance hooks. This can allow unmonitored schema changes.",
		"Ensure the event trigger is no longer needed for DDL governance before dropping.",
		cfg,
	)
}

func newCreateRuleNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGEventRuleLifecycleRule(
		ruleIDPGCreateRuleNotice, rule.LevelNotice, spec.DDLOperationCreateRule, "",
		"PostgreSQL rewrite rule %q created — query rewriting hook registered",
		"CREATE RULE defines a rewriting rule that intercepts queries on a target table and replaces or augments them with alternative actions.",
		"Rewrite rules can silently alter query behavior, making debugging difficult. They are legacy features superseded by triggers in most cases.",
		"Consider using triggers instead of rules. If rules are required, document their behavior clearly.",
		cfg,
	)
}

func newAlterRuleNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGEventRuleLifecycleRule(
		ruleIDPGAlterRuleNotice, rule.LevelNotice, spec.DDLOperationAlterRule, "rename",
		"PostgreSQL rewrite rule %q renamed — query rewriting hook identifier changed",
		"ALTER RULE RENAME changes the name of a rewrite rule without affecting its behavior.",
		"Renaming rules may break scripts or monitoring that reference the rule by name.",
		"Update any references to the old rule name in scripts and monitoring.",
		cfg,
	)
}

func newDropRuleWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGEventRuleLifecycleRule(
		ruleIDPGDropRuleWarn, rule.LevelWarning, spec.DDLOperationDropRule, "",
		"PostgreSQL rewrite rule %q dropped — query rewriting hook removed",
		"DROP RULE permanently removes a rewrite rule from the database.",
		"Dropping rules changes how queries are processed on the target table. Applications that relied on the rule's behavior may be affected.",
		"Verify that no application logic depends on the rule's query rewriting before dropping.",
		cfg,
	)
}

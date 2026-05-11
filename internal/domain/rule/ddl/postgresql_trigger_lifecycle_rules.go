package ddl

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func newCreateTriggerNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithType(
		ruleIDPGCreateTriggerNotice, rule.LevelNotice, spec.DDLOperationCreateTrigger, "",
		"trigger", "trigger",
		"CREATE TRIGGER %q defines a trigger on PostgreSQL",
		"Creating a trigger attaches procedural logic that fires on table events.",
		"Triggers add implicit behavior that can be hard to trace and debug across table interactions.",
		"Review the trigger name and verify the event timing and function match intended behavior.",
		cfg,
	)
}

func newCreateConstraintTriggerWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithType(
		ruleIDPGCreateConstraintTriggerWarn, rule.LevelWarning, spec.DDLOperationCreateTrigger, "constraint",
		"trigger", "trigger",
		"CREATE CONSTRAINT TRIGGER %q defines a constraint trigger on PostgreSQL",
		"Constraint triggers enforce referential integrity with deferrable semantics.",
		"Constraint triggers interact with transaction commit ordering and can cause subtle timing failures.",
		"Verify deferrable behavior and referenced table match the intended constraint semantics.",
		cfg,
	)
}

func newDropTriggerAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithType(
		ruleIDPGDropTriggerAdvisory, rule.LevelNotice, spec.DDLOperationDropTrigger, "",
		"trigger", "trigger",
		"DROP TRIGGER %q removes a trigger on PostgreSQL",
		"Dropping a trigger removes procedural logic that may be relied upon by application workflows.",
		"Removing a trigger silently disables event-driven behavior without downstream notification.",
		"Verify no application workflows depend on this trigger before dropping.",
		cfg,
	)
}

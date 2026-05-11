// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL policy and RLS lifecycle DDL operations
// output: findings for PostgreSQL policy creation, alteration, removal, and row-level security changes
// pos: PostgreSQL-specific policy lifecycle rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func newCreatePolicyNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithType(
		ruleIDPGCreatePolicyNotice, rule.LevelNotice, spec.DDLOperationCreatePolicy, "",
		"policy", "policy",
		"CREATE POLICY %q defines row-level security policy on PostgreSQL",
		"Creating a policy adds a row-level security rule that controls data access at the row level.",
		"Policies control which rows users can see or modify. Misconfigured policies may silently expose or restrict data.",
		"Review the policy name and ensure the USING/WITH CHECK expressions match intended access patterns.",
		cfg,
	)
}

func newAlterPolicyNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithType(
		ruleIDPGAlterPolicyNotice, rule.LevelNotice, spec.DDLOperationAlterPolicy, "",
		"policy", "policy",
		"ALTER POLICY %q modifies row-level security policy on PostgreSQL",
		"Altering a policy changes the row-level security rule controlling data access.",
		"Policy changes may silently alter data visibility or write permissions without downstream notification.",
		"Verify the new USING/WITH CHECK expressions maintain intended access control.",
		cfg,
	)
}

func newDropPolicyWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithType(
		ruleIDPGDropPolicyWarn, rule.LevelWarning, spec.DDLOperationDropPolicy, "",
		"policy", "policy",
		"DROP POLICY %q removes row-level security policy on PostgreSQL",
		"Dropping a policy removes row-level access control. If RLS is still enabled, the table may become inaccessible to non-owners.",
		"Removing a policy while RLS is enabled can lock out non-owner users from the table entirely.",
		"Ensure RLS is disabled or another policy covers the same access patterns before dropping.",
		cfg,
	)
}

func newEnableRLSNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterEnableRLSNotice, rule.LevelNotice, "enable_rls", "table",
		"ALTER TABLE %q ENABLE ROW LEVEL SECURITY activates row-level security on PostgreSQL",
		"Enabling RLS requires policies to be defined, otherwise the table may become inaccessible to non-owners.",
		"Without policies, RLS blocks all non-owner and non-superuser access to the table.",
		"Ensure at least one policy exists before or immediately after enabling RLS.",
		cfg,
	)
}

func newDisableRLSWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterDisableRLSWarn, rule.LevelWarning, "disable_rls", "table",
		"ALTER TABLE %q DISABLE ROW LEVEL SECURITY deactivates row-level security on PostgreSQL",
		"Disabling RLS removes row-level access control, potentially exposing data that was previously restricted.",
		"Data previously protected by RLS policies becomes visible to all roles with table-level SELECT permission.",
		"Verify that no active application workflows depend on RLS protections before disabling.",
		cfg,
	)
}

func newForceRLSNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterForceRLSNotice, rule.LevelNotice, "force_rls", "table",
		"ALTER TABLE %q FORCE ROW LEVEL SECURITY applies RLS to table owner on PostgreSQL",
		"Force RLS makes policies apply to the table owner, overriding default owner exemptions.",
		"The table owner may lose access to their own data if policies do not explicitly allow owner access.",
		"Ensure owner-access policies are in place before forcing RLS.",
		cfg,
	)
}

func newNoForceRLSNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAlterActionAdvisoryRule(
		ruleIDPGAlterNoForceRLSNotice, rule.LevelNotice, "no_force_rls", "table",
		"ALTER TABLE %q NO FORCE ROW LEVEL SECURITY exempts table owner from RLS on PostgreSQL",
		"No force RLS exempts the table owner from row-level security policies.",
		"The table owner regains unrestricted access to all rows, bypassing policy filters.",
		"Confirm this is intentional; owner exemption may mask policy gaps during development.",
		cfg,
	)
}

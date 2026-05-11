// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL advanced view lifecycle DDL operations
// output: findings for PostgreSQL CREATE OR REPLACE VIEW, TEMP VIEW, CHECK OPTION, ALTER VIEW, and DROP VIEW CASCADE risks
// pos: PostgreSQL-specific advanced view lifecycle rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func newCreateOrReplaceViewAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRule(
		ruleIDPGCreateOrReplaceViewAdvisory, rule.LevelNotice, spec.DDLOperationCreateView, "replace",
		"view",
		"CREATE OR REPLACE VIEW %q silently overwrites the existing view definition on PostgreSQL",
		"OR REPLACE modifies the view's query without dropping it, preserving column ACLs and dependencies but changing what the view returns.",
		"Downstream queries will immediately see the new result shape. If column types or positions change, applications relying on the view may break silently.",
		"Prefer DROP + CREATE with explicit review over OR REPLACE in production migrations. If using OR REPLACE, verify column compatibility first.",
		cfg,
	)
}

func newCreateTempViewNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRule(
		ruleIDPGCreateTempViewNotice, rule.LevelNotice, spec.DDLOperationCreateView, "temporary",
		"view",
		"CREATE TEMP VIEW %q creates a session-scoped view on PostgreSQL",
		"Temporary views exist only for the duration of the current database session and are invisible to other sessions.",
		"Code that depends on a temporary view will fail in new sessions or concurrent connections that do not create the same view.",
		"Use temporary views only for ad-hoc session-local queries. For persistent transformations, use regular views or materialized views.",
		cfg,
	)
}

func newCreateViewCheckOptionNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRule(
		ruleIDPGCreateViewCheckOptionNotice, rule.LevelNotice, spec.DDLOperationCreateView, "check_option",
		"view",
		"CREATE VIEW %q with CHECK OPTION constrains INSERT/UPDATE through the view on PostgreSQL",
		"CHECK OPTION ensures that rows inserted or updated through the view remain visible within the view's WHERE clause.",
		"Applications that insert or update via this view may receive unexpected constraint violations if values fall outside the view's filter.",
		"Document CHECK OPTION behavior for downstream consumers. Verify that application INSERT/UPDATE paths are compatible with the view's filter.",
		cfg,
	)
}

func newAlterViewRenameNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithType(
		ruleIDPGAlterViewRenameNotice, rule.LevelNotice, spec.DDLOperationAlterView, "",
		"view", "view",
		"ALTER VIEW %q RENAME TO changes the view identifier on PostgreSQL",
		"Renaming a view changes its qualified name, breaking any query or application code that references the old name.",
		"Downstream queries, stored procedures, and application code that reference the old view name will fail after rename.",
		"Before renaming: 1) Search for all references to the current view name. 2) Update or alias dependent queries. 3) Perform rename during a maintenance window.",
		cfg,
	)
}

func newAlterViewSetSchemaNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRuleWithType(
		ruleIDPGAlterViewSetSchemaNotice, rule.LevelNotice, spec.DDLOperationAlterView, "",
		"view", "view",
		"ALTER VIEW %q SET SCHEMA changes the view's namespace on PostgreSQL",
		"Moving a view to a different schema changes its fully qualified name and may affect search_path resolution.",
		"Downstream queries using unqualified or schema-qualified references to this view will break after the schema change.",
		"Before changing schema: 1) Identify all queries referencing this view. 2) Update qualified references. 3) Verify the target schema exists and has appropriate permissions.",
		cfg,
	)
}

func newDropViewCascadeWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGObjectLifecycleRule(
		ruleIDPGDropViewCascadeWarn, rule.LevelWarning, spec.DDLOperationDropView, "cascade",
		"view",
		"DROP VIEW %q CASCADE automatically drops dependent objects on PostgreSQL",
		"CASCADE forces PostgreSQL to drop any views or other objects that depend on this view.",
		"Cascading drops can silently destroy dependent views, breaking downstream queries and reporting pipelines beyond the intended target.",
		"Avoid CASCADE in production migrations. Explicitly drop dependent views first so the full blast radius is visible and reviewed.",
		cfg,
	)
}

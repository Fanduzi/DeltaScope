// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL domain lifecycle DDL operations
// output: findings for PostgreSQL domain creation, alteration, and drop risks
// pos: PostgreSQL-specific domain lifecycle rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func newCreateDomainNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGDomainLifecycleRule(
		ruleIDPGCreateDomainNotice, rule.LevelNotice, spec.DDLOperationCreateDomain,
		"", "", false, nil, "domain",
		"PostgreSQL domain %q created — review reusable type constraint boundary",
		"CREATE DOMAIN introduces a reusable type constraint that application code and table columns may depend on.",
		"Domain constraints are schema-level contracts; changing them later affects all columns using the domain.",
		"Confirm the domain name, base type, and constraint set are stable before deployment.",
		cfg,
	)
}

func newAlterDomainConstraintNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGDomainLifecycleRule(
		ruleIDPGAlterDomainConstraintNotice, rule.LevelNotice, spec.DDLOperationAlterDomain,
		"", "", false, []string{"add_constraint", "drop_constraint", "validate_constraint"}, "domain",
		"PostgreSQL domain %q constraint modified — review constraint boundary change",
		"ALTER DOMAIN constraint changes (add, drop, validate) modify the type contract that all dependent columns inherit.",
		"Constraint changes on a domain propagate to every column using it, potentially affecting inserts and updates application-wide.",
		"Confirm the constraint change is compatible with existing data and application expectations.",
		cfg,
	)
}

func newAlterDomainDefaultNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGDomainLifecycleRule(
		ruleIDPGAlterDomainDefaultNotice, rule.LevelNotice, spec.DDLOperationAlterDomain,
		"", "", false, []string{"set_default", "drop_default"}, "domain",
		"PostgreSQL domain %q default changed — review default state transition",
		"ALTER DOMAIN SET/DROP DEFAULT changes the implicit value for columns using this domain that lack an explicit column-level default.",
		"Changing the domain default does not affect existing rows but changes the implicit value for future inserts on all dependent columns.",
		"Confirm the default transition is intentional and compatible with application insert logic.",
		cfg,
	)
}

func newAlterDomainNotNullNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGDomainLifecycleRule(
		ruleIDPGAlterDomainNotNullNotice, rule.LevelNotice, spec.DDLOperationAlterDomain,
		"", "", false, []string{"set_not_null", "drop_not_null"}, "domain",
		"PostgreSQL domain %q nullability changed — review NOT NULL constraint transition",
		"ALTER DOMAIN SET/DROP NOT NULL changes the nullability contract for all columns using this domain.",
		"Adding NOT NULL may reject existing nulls in dependent columns; dropping NOT NULL weakens a schema contract that applications may rely on.",
		"Verify existing data satisfies the new nullability state before deployment.",
		cfg,
	)
}

func newAlterDomainRenameNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGDomainLifecycleRule(
		ruleIDPGAlterDomainRenameNotice, rule.LevelNotice, spec.DDLOperationAlterDomain,
		"action", "rename", true, nil, "domain",
		"PostgreSQL domain %q renamed — review schema reference impact",
		"ALTER DOMAIN RENAME changes the domain name that dependent columns, application code, and schema scripts reference.",
		"Existing column definitions continue using the domain, but application code, migration scripts, and documentation referencing the old name will break.",
		"Coordinate the rename with all downstream references before deployment.",
		cfg,
	)
}

func newDropDomainAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGDomainLifecycleRule(
		ruleIDPGDropDomainAdvisory, rule.LevelWarning, spec.DDLOperationDropDomain,
		"", "", false, nil, "domain",
		"PostgreSQL domain %q dropped — dependent columns may break",
		"DROP DOMAIN removes a type that table columns may depend on, breaking those column definitions.",
		"Columns using the domain lose their constraint boundary immediately, and restoring the domain state may require coordinated schema recovery.",
		"Verify no table columns or application code depend on this domain before dropping it.",
		cfg,
	)
}

func newDropDomainCascadeWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGDomainLifecycleRule(
		ruleIDPGDropDomainCascadeWarn, rule.LevelWarning, spec.DDLOperationDropDomain,
		"cascade", "true", true, nil, "domain",
		"DROP DOMAIN %q CASCADE may remove dependent objects on PostgreSQL",
		"CASCADE tells PostgreSQL to drop objects that depend on the domain, which may include table columns.",
		"Tables, views, functions, or application contracts depending on the domain may be removed unexpectedly.",
		"Prefer RESTRICT during review. If CASCADE is intentional, enumerate dependent objects and confirm the blast radius.",
		cfg,
	)
}

type pgDomainLifecycleRule struct {
	id            string
	level         rule.Level
	operation     spec.DDLOperation
	optionKey     string
	optionValue   string
	requireOption bool
	matchActions  []string
	object        string
	message       string
	why           string
	risk          string
	suggestion    string
}

func newPGDomainLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, optionKey, optionValue string, requireOption bool, matchActions []string, object, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgDomainLifecycleRule{
		id:            id,
		level:         configuredLevel(cfg, level),
		operation:     operation,
		optionKey:     optionKey,
		optionValue:   optionValue,
		requireOption: requireOption,
		matchActions:  matchActions,
		object:        object,
		message:       message,
		why:           why,
		risk:          risk,
		suggestion:    suggestion,
	}, nil
}

func (r pgDomainLifecycleRule) ID() string { return r.id }

func (r pgDomainLifecycleRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation
}

func (r pgDomainLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if r.requireOption && statement.DDL.Options[r.optionKey] != r.optionValue {
		return nil, nil
	}
	if !r.requireOption && r.optionKey != "" && statement.DDL.Options[r.optionKey] != r.optionValue {
		return nil, nil
	}
	if len(r.matchActions) > 0 {
		action := statement.DDL.Options["action"]
		if !containsString(r.matchActions, action) {
			return nil, nil
		}
	}

	objectName := statement.DDL.ObjectName
	message := fmt.Sprintf(r.message, objectName)

	metadata := map[string]any{
		"operation":   string(statement.DDL.Operation),
		"object_type": statement.DDL.ObjectType,
		"object_name": objectName,
	}
	for _, key := range []string{"base_type", "not_null", "has_default", "has_check", "constraint", "action", "new_name", "cascade", "if_exists"} {
		if val := statement.DDL.Options[key]; val != "" {
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

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

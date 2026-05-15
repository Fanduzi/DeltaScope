package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgSemanticObjectLifecycleRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	action     string
	message    string
	why        string
	risk       string
	suggestion string
}

func newPGSemanticObjectLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, action string, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgSemanticObjectLifecycleRule{
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

func (r pgSemanticObjectLifecycleRule) ID() string { return r.id }

func (r pgSemanticObjectLifecycleRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectPostgreSQL ||
		statement.Kind != spec.KindDDL ||
		statement.DDL == nil ||
		statement.DDL.Operation != r.operation {
		return false
	}
	if r.action == "" {
		return true
	}
	return statement.DDL.Options["action"] == r.action
}

func (r pgSemanticObjectLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
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
	for _, key := range []string{"action", "new_name", "new_schema", "owner", "if_exists", "cascade"} {
		if val, ok := statement.DDL.Options[key]; ok && val != "" {
			metadata[key] = val
		}
	}
	for k, v := range projectObjectMetadata(statement) {
		metadata[k] = v
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

func newCreateAggregateNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGSemanticObjectLifecycleRule(
		ruleIDPGCreateAggregateNotice, rule.LevelNotice, spec.DDLOperationCreateAggregate, "",
		"PostgreSQL aggregate %q created — custom aggregation function defined",
		"CREATE AGGREGATE defines a custom aggregate function that extends SQL with domain-specific computation.",
		"Custom aggregates affect query results and application behavior. Removing or changing them breaks dependent queries.",
		"Verify that the aggregate name, input types, and transition logic match application expectations.",
		cfg,
	)
}

func newAlterAggregateNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGSemanticObjectLifecycleRule(
		ruleIDPGAlterAggregateNotice, rule.LevelNotice, spec.DDLOperationAlterAggregate, "",
		"PostgreSQL aggregate %q altered — aggregation configuration changed",
		"ALTER AGGREGATE changes the configuration of an existing aggregate, such as renaming it, changing its owner, or moving it to a different schema.",
		"Changing aggregate configuration may affect queries and application behavior that depends on the aggregate.",
		"Review the change to ensure dependent queries remain correct.",
		cfg,
	)
}

func newDropAggregateWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGSemanticObjectLifecycleRule(
		ruleIDPGDropAggregateWarn, rule.LevelWarning, spec.DDLOperationDropAggregate, "",
		"PostgreSQL aggregate %q dropped — custom aggregation removed",
		"DROP AGGREGATE permanently removes a custom aggregate function from the database.",
		"Dropping an aggregate invalidates any queries that use it. Dependent queries will fail until the aggregate is restored or rewritten.",
		"Ensure no queries depend on this aggregate before dropping.",
		cfg,
	)
}

func newCreateOperatorNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGSemanticObjectLifecycleRule(
		ruleIDPGCreateOperatorNotice, rule.LevelNotice, spec.DDLOperationCreateOperator, "",
		"PostgreSQL operator %q created — custom operator defined",
		"CREATE OPERATOR defines a custom operator that extends SQL expression evaluation with domain-specific semantics.",
		"Custom operators affect query results, index usage, and application behavior. Removing or changing them breaks dependent queries.",
		"Verify that the operator name, argument types, and implementation match application expectations.",
		cfg,
	)
}

func newAlterOperatorNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGSemanticObjectLifecycleRule(
		ruleIDPGAlterOperatorNotice, rule.LevelNotice, spec.DDLOperationAlterOperator, "",
		"PostgreSQL operator %q altered — operator configuration changed",
		"ALTER OPERATOR changes the configuration of an existing operator, such as changing its owner or moving it to a different schema.",
		"Changing operator configuration may affect queries and application behavior that depends on the operator.",
		"Review the change to ensure dependent queries remain correct.",
		cfg,
	)
}

func newDropOperatorWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGSemanticObjectLifecycleRule(
		ruleIDPGDropOperatorWarn, rule.LevelWarning, spec.DDLOperationDropOperator, "",
		"PostgreSQL operator %q dropped — custom operator removed",
		"DROP OPERATOR permanently removes a custom operator from the database.",
		"Dropping an operator invalidates any queries that use it. Dependent queries will fail until the operator is restored.",
		"Ensure no queries depend on this operator before dropping.",
		cfg,
	)
}

func newCreateConversionNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGSemanticObjectLifecycleRule(
		ruleIDPGCreateConversionNotice, rule.LevelNotice, spec.DDLOperationCreateConversion, "",
		"PostgreSQL conversion %q created — encoding conversion defined",
		"CREATE CONVERSION defines a custom encoding conversion that transforms text between character sets.",
		"Custom conversions affect how text data is converted between encodings. Removing or changing them breaks dependent conversion operations.",
		"Verify that the conversion source and target encodings match application expectations.",
		cfg,
	)
}

func newAlterConversionNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGSemanticObjectLifecycleRule(
		ruleIDPGAlterConversionNotice, rule.LevelNotice, spec.DDLOperationAlterConversion, "",
		"PostgreSQL conversion %q altered — encoding conversion configuration changed",
		"ALTER CONVERSION changes the configuration of an existing conversion, such as renaming it, changing its owner, or moving it to a different schema.",
		"Changing conversion configuration may affect text encoding behavior that depends on the conversion.",
		"Review the change to ensure encoding behavior remains correct for dependent operations.",
		cfg,
	)
}

func newDropConversionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGSemanticObjectLifecycleRule(
		ruleIDPGDropConversionWarn, rule.LevelWarning, spec.DDLOperationDropConversion, "",
		"PostgreSQL conversion %q dropped — encoding conversion removed",
		"DROP CONVERSION permanently removes a custom encoding conversion from the database.",
		"Dropping a conversion invalidates any operations that reference it. Dependent operations will fail until the conversion is restored.",
		"Ensure no operations depend on this conversion before dropping.",
		cfg,
	)
}

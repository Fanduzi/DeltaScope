package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgOperatorFamilyLifecycleRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	action     string
	message    string
	why        string
	risk       string
	suggestion string
}

func newPGOperatorFamilyLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, action string, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgOperatorFamilyLifecycleRule{
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

func (r pgOperatorFamilyLifecycleRule) ID() string { return r.id }

func (r pgOperatorFamilyLifecycleRule) AppliesTo(statement spec.Statement) bool {
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

func (r pgOperatorFamilyLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
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
	for _, key := range []string{"action", "new_name", "new_schema", "owner", "access_method", "is_default", "if_exists", "cascade"} {
		if val, ok := statement.DDL.Options[key]; ok && val != "" {
			metadata[key] = val
		}
	}
	for k, v := range projectObjectMetadata(statement) {
		metadata[k] = v
	}

	return []rule.Finding{{
		RuleID:  r.id,
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

func newCreateOperatorFamilyNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGOperatorFamilyLifecycleRule(
		ruleIDPGCreateOperatorFamilyNotice, rule.LevelNotice, spec.DDLOperationCreateOperatorFamily, "",
		"PostgreSQL operator family %q created — index method operator group defined",
		"CREATE OPERATOR FAMILY defines a named group of operators and support procedures for a specific index access method.",
		"Operator families affect how indexes are built and how queries use operators with indexes. Removing or changing them breaks index behavior.",
		"Verify that the operator family name and access method match index design expectations.",
		cfg,
	)
}

func newAlterOperatorFamilyNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGOperatorFamilyLifecycleRule(
		ruleIDPGAlterOperatorFamilyNotice, rule.LevelNotice, spec.DDLOperationAlterOperatorFamily, "",
		"PostgreSQL operator family %q altered — operator family configuration changed",
		"ALTER OPERATOR FAMILY changes the configuration of an existing operator family, such as renaming it, changing its owner, or moving it to a different schema.",
		"Changing operator family configuration may affect index behavior and query planning that depends on the family.",
		"Review the change to ensure index behavior remains correct.",
		cfg,
	)
}

func newDropOperatorFamilyWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGOperatorFamilyLifecycleRule(
		ruleIDPGDropOperatorFamilyWarn, rule.LevelWarning, spec.DDLOperationDropOperatorFamily, "",
		"PostgreSQL operator family %q dropped — index method operator group removed",
		"DROP OPERATOR FAMILY permanently removes an operator family from the database.",
		"Dropping an operator family invalidates any indexes that depend on it. Dependent indexes will become unusable until restored.",
		"Ensure no indexes depend on this operator family before dropping.",
		cfg,
	)
}

func newCreateOperatorClassNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGOperatorFamilyLifecycleRule(
		ruleIDPGCreateOperatorClassNotice, rule.LevelNotice, spec.DDLOperationCreateOperatorClass, "",
		"PostgreSQL operator class %q created — index method operator class defined",
		"CREATE OPERATOR CLASS defines a named set of operators and support functions for a specific data type and index access method.",
		"Operator classes control how indexes handle specific data types. Removing or changing them breaks index operations for the associated data type.",
		"Verify that the operator class name, data type, and access method match index design expectations.",
		cfg,
	)
}

func newAlterOperatorClassNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGOperatorFamilyLifecycleRule(
		ruleIDPGAlterOperatorClassNotice, rule.LevelNotice, spec.DDLOperationAlterOperatorClass, "",
		"PostgreSQL operator class %q altered — operator class configuration changed",
		"ALTER OPERATOR CLASS changes the configuration of an existing operator class, such as renaming it, changing its owner, or moving it to a different schema.",
		"Changing operator class configuration may affect index behavior and query planning that depends on the class.",
		"Review the change to ensure index behavior remains correct.",
		cfg,
	)
}

func newDropOperatorClassWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGOperatorFamilyLifecycleRule(
		ruleIDPGDropOperatorClassWarn, rule.LevelWarning, spec.DDLOperationDropOperatorClass, "",
		"PostgreSQL operator class %q dropped — index method operator class removed",
		"DROP OPERATOR CLASS permanently removes an operator class from the database.",
		"Dropping an operator class invalidates any indexes that depend on it. Dependent indexes will become unusable until restored.",
		"Ensure no indexes depend on this operator class before dropping.",
		cfg,
	)
}

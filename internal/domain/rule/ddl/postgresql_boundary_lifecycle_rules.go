package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgBoundaryLifecycleRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	message    string
	why        string
	risk       string
	suggestion string
}

func newPGBoundaryLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgBoundaryLifecycleRule{
		id:         id,
		level:      configuredLevel(cfg, level),
		operation:  operation,
		message:    message,
		why:        why,
		risk:       risk,
		suggestion: suggestion,
	}, nil
}

func (r pgBoundaryLifecycleRule) ID() string { return r.id }

func (r pgBoundaryLifecycleRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation
}

func (r pgBoundaryLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
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

func newDropTransformWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGBoundaryLifecycleRule(
		ruleIDPGDropTransformWarn, rule.LevelWarning, spec.DDLOperationDropTransform,
		"PostgreSQL transform %q dropped",
		"DROP TRANSFORM permanently removes a transform that maps between a data type and a procedural language.",
		"Dropping a transform breaks any PL functions that rely on the type-language mapping for data conversion.",
		"Ensure no PL functions depend on this type-language transform before dropping.",
		cfg,
	)
}

func newDropAccessMethodWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGBoundaryLifecycleRule(
		ruleIDPGDropAccessMethodWarn, rule.LevelWarning, spec.DDLOperationDropAccessMethod,
		"PostgreSQL access method %q dropped",
		"DROP ACCESS METHOD permanently removes a table access method that defines how tables are stored and accessed.",
		"Dropping an access method breaks any tables that use it for storage. Existing tables become inaccessible.",
		"Ensure no tables use this access method before dropping.",
		cfg,
	)
}

func newAlterLargeObjectOwnerNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGBoundaryLifecycleRule(
		ruleIDPGAlterLargeObjectOwnerNotice, rule.LevelNotice, spec.DDLOperationAlterLargeObject,
		"PostgreSQL large object %q owner changed",
		"ALTER LARGE OBJECT changes the owner of a large object (OID). Large objects store binary data outside normal tables.",
		"Changing the owner of a large object affects which roles can read, write, or manage it.",
		"Verify the new owner is the correct role for managing this large object's lifecycle.",
		cfg,
	)
}

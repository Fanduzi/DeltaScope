// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL annotation lifecycle DDL operations
// output: findings for PostgreSQL COMMENT ON and SECURITY LABEL lifecycle events
// pos: PostgreSQL-specific annotation lifecycle rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgAnnotationLifecycleRule struct {
	id         string
	level      rule.Level
	operation  spec.DDLOperation
	isNull     string // "true", "false", or "" for either
	message    string
	why        string
	risk       string
	suggestion string
}

func newPGAnnotationLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, isNull string, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgAnnotationLifecycleRule{
		id:         id,
		level:      configuredLevel(cfg, level),
		operation:  operation,
		isNull:     isNull,
		message:    message,
		why:        why,
		risk:       risk,
		suggestion: suggestion,
	}, nil
}

func (r pgAnnotationLifecycleRule) ID() string { return r.id }

func (r pgAnnotationLifecycleRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation &&
		(r.isNull == "" || statement.DDL.Options["is_null"] == r.isNull)
}

func (r pgAnnotationLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
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
	for _, key := range []string{"target_type", "target_name", "is_null", "provider"} {
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

func newCommentOnNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAnnotationLifecycleRule(
		ruleIDPGCommentOnNotice, rule.LevelNotice, spec.DDLOperationCommentOn, "false",
		"PostgreSQL comment set on %q — metadata annotation added",
		"COMMENT ON attaches descriptive metadata to a database object. Comments are visible in information_schema and psql \\d+ output.",
		"Comments may contain sensitive information or assumptions about schema design that could mislead future developers.",
		"Review that the comment does not contain sensitive information and accurately describes the object.",
		cfg,
	)
}

func newCommentOnRemoveNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAnnotationLifecycleRule(
		ruleIDPGCommentOnRemoveNotice, rule.LevelNotice, spec.DDLOperationCommentOn, "true",
		"PostgreSQL comment removed from %q — metadata annotation cleared",
		"COMMENT ON ... IS NULL removes the descriptive metadata from a database object.",
		"Removing comments may reduce documentation coverage for the schema.",
		"Verify that the comment removal is intentional and not accidental.",
		cfg,
	)
}

func newSecurityLabelNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAnnotationLifecycleRule(
		ruleIDPGSecurityLabelNotice, rule.LevelNotice, spec.DDLOperationSecurityLabel, "false",
		"PostgreSQL security label set on %q — mandatory access control annotation applied",
		"SECURITY LABEL assigns a security classification to a database object for use with mandatory access control frameworks such as SELinux.",
		"Security labels control data access through external security frameworks. Misapplied labels may restrict or expose data unintentionally.",
		"Confirm the security label is appropriate for the object's sensitivity level and the target provider policy.",
		cfg,
	)
}

func newSecurityLabelRemoveNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGAnnotationLifecycleRule(
		ruleIDPGSecurityLabelRemoveNotice, rule.LevelNotice, spec.DDLOperationSecurityLabel, "true",
		"PostgreSQL security label removed from %q — mandatory access control annotation cleared",
		"SECURITY LABEL ... IS NULL removes the security classification from a database object.",
		"Removing security labels may expose data that was previously protected by mandatory access control policies.",
		"Verify that the label removal is intentional and the object no longer requires MAC protection.",
		cfg,
	)
}

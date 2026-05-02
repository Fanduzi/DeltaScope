// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL type lifecycle DDL operations
// output: findings for PostgreSQL enum type creation, value addition, and type drop risks
// pos: PostgreSQL-specific type lifecycle rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgTypeLifecycleRule struct {
	id           string
	level        rule.Level
	operation    spec.DDLOperation
	optionKey    string
	optionValue  string
	requireOption bool
	object       string
	message      string
	why          string
	risk         string
	suggestion   string
}

func newPGTypeLifecycleRule(id string, level rule.Level, operation spec.DDLOperation, optionKey, optionValue string, requireOption bool, object, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgTypeLifecycleRule{
		id:            id,
		level:         configuredLevel(cfg, level),
		operation:     operation,
		optionKey:     optionKey,
		optionValue:   optionValue,
		requireOption: requireOption,
		object:        object,
		message:       message,
		why:           why,
		risk:          risk,
		suggestion:    suggestion,
	}, nil
}

func (r pgTypeLifecycleRule) ID() string { return r.id }

func (r pgTypeLifecycleRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation
}

func (r pgTypeLifecycleRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if r.requireOption && statement.DDL.Options[r.optionKey] != r.optionValue {
		return nil, nil
	}
	if !r.requireOption && r.optionKey != "" && statement.DDL.Options[r.optionKey] != r.optionValue {
		return nil, nil
	}

	objectName := statement.DDL.ObjectName
	message := fmt.Sprintf(r.message, objectName)

	metadata := map[string]any{
		"operation":   string(statement.DDL.Operation),
		"object_type": statement.DDL.ObjectType,
		"object_name": objectName,
	}
	for _, key := range []string{"type_kind", "action", "value", "placement", "neighbor", "cascade", "if_exists"} {
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

// pgAlterTypeAddValuePositionNoticeRule fires only when ALTER TYPE ADD VALUE
// includes an explicit BEFORE/AFTER placement.
type pgAlterTypeAddValuePositionNoticeRule struct {
	level      rule.Level
	message    string
	why        string
	risk       string
	suggestion string
}

func newAlterTypeAddValuePositionNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgAlterTypeAddValuePositionNoticeRule{
		level:      configuredLevel(cfg, rule.LevelNotice),
		message:    "PostgreSQL enum value %q added with explicit ordering — comparison semantics may change",
		why:        "BEFORE/AFTER placement affects the sort/comparison order of enum labels, which can change query results or application logic that depends on label ordering.",
		risk:       "Downstream comparisons or ORDER BY clauses that rely on enum sort order may produce different results after a positioned value insertion.",
		suggestion: "Confirm that the new enum label's position is compatible with existing application assumptions about sort order. Test queries that sort or compare enum values after deployment.",
	}, nil
}

func (r pgAlterTypeAddValuePositionNoticeRule) ID() string {
	return ruleIDPGAlterTypeAddValuePositionNotice
}

func (r pgAlterTypeAddValuePositionNoticeRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectPostgreSQL || statement.Kind != spec.KindDDL || statement.DDL == nil {
		return false
	}
	if statement.DDL.Operation != spec.DDLOperationAlterType {
		return false
	}
	if statement.DDL.Options["action"] != "add_value" {
		return false
	}
	p := statement.DDL.Options["placement"]
	return p == "before" || p == "after"
}

func (r pgAlterTypeAddValuePositionNoticeRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
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
	for _, key := range []string{"type_kind", "action", "value", "placement", "neighbor"} {
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

func newCreateTypeEnumNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTypeLifecycleRule(
		ruleIDPGCreateTypeEnumNotice, rule.LevelNotice, spec.DDLOperationCreateType,
		"type_kind", "enum", true, "type",
		"PostgreSQL enum type %q created — review schema contract change",
		"CREATE TYPE ... AS ENUM introduces a shared schema contract that application code and table columns may depend on.",
		"Enum labels are harder to evolve than ordinary lookup-table rows, and future changes may require coordinated deploys.",
		"Confirm the enum is preferable to a lookup table and that label names are stable before deployment.",
		cfg,
	)
}

func newAlterTypeAddValueAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTypeLifecycleRule(
		ruleIDPGAlterTypeAddValueAdvisory, rule.LevelWarning, spec.DDLOperationAlterType,
		"action", "add_value", true, "type",
		"PostgreSQL enum value added to %q — rollback may be difficult",
		"ALTER TYPE ... ADD VALUE extends an enum's allowed values. Once application data uses the new label, rolling back code or schema can be difficult.",
		"Older application versions may not understand the new enum value, and removing enum labels is not a simple reversible migration.",
		"Deploy enum additions in a forward-compatible sequence and ensure older application versions tolerate the new value.",
		cfg,
	)
}

func newDropTypeAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTypeLifecycleRule(
		ruleIDPGDropTypeAdvisory, rule.LevelWarning, spec.DDLOperationDropType,
		"", "", false, "type",
		"PostgreSQL type %q dropped — dependent schema objects may break",
		"DROP TYPE removes a type that tables, functions, casts, views, or application contracts may depend on.",
		"Dependent objects can fail immediately, and restoring dropped type state may require coordinated schema recovery.",
		"Verify no schema objects or application code depend on this type before dropping it.",
		cfg,
	)
}

func newDropTypeCascadeWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTypeLifecycleRule(
		ruleIDPGDropTypeCascadeWarn, rule.LevelWarning, spec.DDLOperationDropType,
		"cascade", "true", true, "type",
		"DROP TYPE %q CASCADE may remove dependent objects on PostgreSQL",
		"CASCADE tells PostgreSQL to drop objects that depend on the type.",
		"Tables, columns, functions, views, casts, or constraints depending on the type may be removed unexpectedly.",
		"Prefer RESTRICT during review. If CASCADE is intentional, enumerate dependent objects and confirm the blast radius.",
		cfg,
	)
}

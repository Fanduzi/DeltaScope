// Package ddl defines Tier-1 DDL rules.
// input: create-table Statement specs with table options, constraints, and shape flags plus per-rule policy values
// output: findings for table options and object-shape restrictions
// pos: DDL rule implementations for create-table option governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"
	"strconv"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type tableCommentMaxLengthRule struct {
	limit int
	level rule.Level
}

func newTableCommentMaxLengthRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	limit, err := intParam(ruleIDTableCommentMaxLength, cfg, "limit", 128)
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, fmt.Errorf("rule %s param %q must be >= 1, got %d", ruleIDTableCommentMaxLength, "limit", limit)
	}
	return tableCommentMaxLengthRule{limit: limit, level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r tableCommentMaxLengthRule) ID() string { return ruleIDTableCommentMaxLength }

func (r tableCommentMaxLengthRule) AppliesTo(statement spec.Statement) bool {
	return appliesToCreateTable(statement)
}

func (r tableCommentMaxLengthRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	actual := len(statement.DDL.Table.Comment)
	if actual == 0 || actual <= r.limit {
		return nil, nil
	}
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("table comment must not exceed %d characters", r.limit),
		Suggestion: fmt.Sprintf("shorten the table comment to %d characters or fewer", r.limit),
		Metadata: map[string]any{
			"table":  statement.DDL.Table.Name,
			"limit":  r.limit,
			"actual": actual,
		},
	}}, nil
}

type tableOptionAllowlistRule struct {
	ruleID          string
	optionKey       string
	label           string
	allowed         []string
	level           rule.Level
	requireExplicit bool
}

func newTableOptionAllowlistRule(ruleID, optionKey, label string, fallback []string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	allowed, err := stringSliceParam(ruleID, cfg, "values", fallback)
	if err != nil {
		return nil, err
	}
	requireExplicit, err := boolParam(ruleID, cfg, "require_explicit", true)
	if err != nil {
		return nil, err
	}
	return tableOptionAllowlistRule{
		ruleID:          ruleID,
		optionKey:       optionKey,
		label:           label,
		allowed:         allowed,
		level:           configuredLevel(cfg, fallbackLevel),
		requireExplicit: requireExplicit,
	}, nil
}

func (r tableOptionAllowlistRule) ID() string { return r.ruleID }

func (r tableOptionAllowlistRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect == spec.DialectPostgreSQL && (r.ruleID == ruleIDTableEngineAllowlist || r.ruleID == ruleIDTableRowFormatAllowlist) {
		return false
	}
	return appliesToCreateTable(statement)
}

func (r tableOptionAllowlistRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	actual := statement.DDL.Options[r.optionKey]
	if actual == "" && !r.requireExplicit {
		return nil, nil
	}
	if actual != "" && containsFold(r.allowed, actual) {
		return nil, nil
	}
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("table %s must be one of %v", r.label, r.allowed),
		Suggestion: fmt.Sprintf("set an explicit %s from the allowed list", r.label),
		Metadata: map[string]any{
			"table":   statement.DDL.Table.Name,
			"option":  r.optionKey,
			"actual":  actual,
			"allowed": append([]string(nil), r.allowed...),
		},
	}}, nil
}

type tableBooleanShapeRule struct {
	ruleID    string
	label     string
	level     rule.Level
	forbid    bool
	predicate func(*spec.DDL) bool
}

func newTableBooleanShapeRule(ruleID, label string, fallbackLevel rule.Level, predicate func(*spec.DDL) bool, cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleID, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}
	return tableBooleanShapeRule{
		ruleID:    ruleID,
		label:     label,
		level:     configuredLevel(cfg, fallbackLevel),
		forbid:    forbid,
		predicate: predicate,
	}, nil
}

func (r tableBooleanShapeRule) ID() string { return r.ruleID }

func (r tableBooleanShapeRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToCreateTable(statement)
}

func (r tableBooleanShapeRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) || !r.predicate(statement.DDL) {
		return nil, nil
	}
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("create table %s is forbidden", r.label),
		Suggestion: fmt.Sprintf("avoid %s in this statement or relax the policy intentionally", r.label),
		Metadata: map[string]any{
			"table": statement.DDL.Table.Name,
			"rule":  r.ruleID,
		},
	}}, nil
}

type tableForeignKeyForbidRule struct {
	forbid bool
	level  rule.Level
}

type tableAutoIncrementInitValueRule struct {
	requiredValue int
	level         rule.Level
}

func newTableAutoIncrementInitValueRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	requiredValue, err := boundedIntParam(ruleIDTableAutoIncrementInitValueRequire, cfg, "value", 1, 1)
	if err != nil {
		return nil, err
	}
	return tableAutoIncrementInitValueRule{requiredValue: requiredValue, level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r tableAutoIncrementInitValueRule) ID() string { return ruleIDTableAutoIncrementInitValueRequire }

func (r tableAutoIncrementInitValueRule) AppliesTo(statement spec.Statement) bool {
	return appliesToCreateTable(statement)
}

func (r tableAutoIncrementInitValueRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	actualText := statement.DDL.Options["auto_increment"]
	if actualText == "" {
		return nil, nil
	}
	actual, err := strconv.Atoi(actualText)
	if err != nil {
		return nil, fmt.Errorf("table option %q must parse as integer, got %q", "auto_increment", actualText)
	}
	if actual == r.requiredValue {
		return nil, nil
	}
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("table auto_increment init value must be %d", r.requiredValue),
		Suggestion: fmt.Sprintf("set AUTO_INCREMENT=%d or omit the explicit init value", r.requiredValue),
		Metadata: map[string]any{
			"table":          statement.DDL.Table.Name,
			"required_value": r.requiredValue,
			"actual_value":   actual,
		},
	}}, nil
}

func newTableForeignKeyForbidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleIDTableForeignKeyForbid, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}
	return tableForeignKeyForbidRule{forbid: forbid, level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r tableForeignKeyForbidRule) ID() string { return ruleIDTableForeignKeyForbid }

func (r tableForeignKeyForbidRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToCreateTable(statement)
}

func (r tableForeignKeyForbidRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, constraint := range statement.DDL.Constraints {
		if constraint.Type != "foreign_key" {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("foreign key constraint %q is forbidden", constraint.Name),
			Suggestion: "remove the foreign key constraint or disable the policy intentionally",
			Metadata: func() map[string]any {
				m := map[string]any{
					"table":      statement.DDL.Table.Name,
					"constraint": constraint.Name,
					"columns":    append([]string(nil), constraint.Columns...),
				}
				if constraint.ReferencedSchema != "" {
					m["referenced_schema"] = constraint.ReferencedSchema
				}
				if constraint.ReferencedTable != "" {
					m["referenced_table"] = constraint.ReferencedTable
				}
				if len(constraint.ReferencedColumns) > 0 {
					m["referenced_columns"] = append([]string(nil), constraint.ReferencedColumns...)
				}
				return m
			}(),
		})
	}
	return findings, nil
}

// ---------------------------------------------------------------------------
// Rule: ddl.pg.table.foreign_key.cross_schema.advisory
// ---------------------------------------------------------------------------

type tableForeignKeyCrossSchemaAdvisoryRule struct {
	level rule.Level
}

func newTableForeignKeyCrossSchemaAdvisoryRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tableForeignKeyCrossSchemaAdvisoryRule{
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r tableForeignKeyCrossSchemaAdvisoryRule) ID() string {
	return ruleIDPGTableForeignKeyCrossSchemaAdvisory
}

func (r tableForeignKeyCrossSchemaAdvisoryRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL && appliesToCreateTable(statement)
}

func (r tableForeignKeyCrossSchemaAdvisoryRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, constraint := range statement.DDL.Constraints {
		if constraint.Type != "foreign_key" {
			continue
		}
		if statement.DDL.Table.Schema == "" || constraint.ReferencedSchema == "" {
			continue
		}
		if statement.DDL.Table.Schema == constraint.ReferencedSchema {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: fmt.Sprintf("foreign key constraint %q references a different schema (%s.%s → %s.%s)", constraint.Name, statement.DDL.Table.Schema, statement.DDL.Table.Name, constraint.ReferencedSchema, constraint.ReferencedTable),
			Metadata: map[string]any{
				"table":              statement.DDL.Table.Name,
				"table_schema":       statement.DDL.Table.Schema,
				"constraint":         constraint.Name,
				"columns":            append([]string(nil), constraint.Columns...),
				"referenced_schema":  constraint.ReferencedSchema,
				"referenced_table":   constraint.ReferencedTable,
				"referenced_columns": append([]string(nil), constraint.ReferencedColumns...),
			},
		})
	}
	return findings, nil
}

// Package ddl defines Tier-1 DDL rules.
// input: create-table Statement specs with enriched column metadata and per-rule policy values
// output: column-level findings for naming, comments, defaults, nullability, and types
// pos: DDL rule implementations for column governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type tableColumnsMinCountRule struct {
	limit int
	level rule.Level
}

func newTableColumnsMinCountRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	limit, err := intParam(ruleIDTableColumnsMinCount, cfg, "limit", 1)
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, fmt.Errorf("rule %s param %q must be >= 1, got %d", ruleIDTableColumnsMinCount, "limit", limit)
	}

	return tableColumnsMinCountRule{
		limit: limit,
		level: configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r tableColumnsMinCountRule) ID() string { return ruleIDTableColumnsMinCount }

func (r tableColumnsMinCountRule) AppliesTo(statement spec.Statement) bool {
	return appliesToCreateTableColumns(statement)
}

func (r tableColumnsMinCountRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) || len(statement.DDL.Columns) >= r.limit {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("table must define at least %d column(s)", r.limit),
		Suggestion: "add at least one application column before creating the table",
		Metadata: map[string]any{
			"table":  statement.DDL.Table.Name,
			"limit":  r.limit,
			"actual": len(statement.DDL.Columns),
		},
	}}, nil
}

type columnCommentRequiredRule struct {
	required bool
	level    rule.Level
}

func newColumnCommentRequiredRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDColumnCommentRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}

	return columnCommentRequiredRule{
		required: required,
		level:    configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r columnCommentRequiredRule) ID() string { return ruleIDColumnCommentRequire }

func (r columnCommentRequiredRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToCreateTableColumns(statement)
}

func (r columnCommentRequiredRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, column := range statement.DDL.Columns {
		if strings.TrimSpace(column.Comment) != "" {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("column %q must include a comment", column.Name),
			Suggestion: "add a COMMENT clause describing the column meaning",
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"column": column.Name,
			},
		})
	}
	return findings, nil
}

type columnNameMaxLengthRule struct {
	limit int
	level rule.Level
}

func newColumnNameMaxLengthRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	limit, err := intParam(ruleIDColumnNameMaxLength, cfg, "limit", 64)
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, fmt.Errorf("rule %s param %q must be >= 1, got %d", ruleIDColumnNameMaxLength, "limit", limit)
	}

	return columnNameMaxLengthRule{
		limit: limit,
		level: configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r columnNameMaxLengthRule) ID() string { return ruleIDColumnNameMaxLength }

func (r columnNameMaxLengthRule) AppliesTo(statement spec.Statement) bool {
	return appliesToCreateTableColumns(statement)
}

func (r columnNameMaxLengthRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, column := range statement.DDL.Columns {
		actual := len(column.Name)
		if actual <= r.limit {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("column name %q must not exceed %d characters", column.Name, r.limit),
			Suggestion: fmt.Sprintf("rename the column to fit within %d characters", r.limit),
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"column": column.Name,
				"limit":  r.limit,
				"actual": actual,
			},
		})
	}
	return findings, nil
}

type columnVarcharMaxLengthRule struct {
	limit int
	level rule.Level
}

func newColumnVarcharMaxLengthRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	limit, err := intParam(ruleIDColumnVarcharMaxLength, cfg, "limit", 16383)
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, fmt.Errorf("rule %s param %q must be >= 1, got %d", ruleIDColumnVarcharMaxLength, "limit", limit)
	}

	return columnVarcharMaxLengthRule{
		limit: limit,
		level: configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r columnVarcharMaxLengthRule) ID() string { return ruleIDColumnVarcharMaxLength }

func (r columnVarcharMaxLengthRule) AppliesTo(statement spec.Statement) bool {
	return appliesToCreateTableColumns(statement)
}

func (r columnVarcharMaxLengthRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, column := range statement.DDL.Columns {
		if baseType(column) != "varchar" || column.Length == 0 || column.Length <= r.limit {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("varchar column %q must not exceed %d characters", column.Name, r.limit),
			Suggestion: "reduce the varchar length or switch to a text/blob family type intentionally",
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"column": column.Name,
				"limit":  r.limit,
				"actual": column.Length,
			},
		})
	}
	return findings, nil
}

type columnDefaultRequiredRule struct {
	required bool
	level    rule.Level
}

func newColumnDefaultRequiredRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDColumnDefaultRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}

	return columnDefaultRequiredRule{
		required: required,
		level:    configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r columnDefaultRequiredRule) ID() string { return ruleIDColumnDefaultRequire }

func (r columnDefaultRequiredRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToCreateTableColumns(statement)
}

func (r columnDefaultRequiredRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, column := range statement.DDL.Columns {
		if column.HasDefault || isBlobTextLike(column) {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("column %q should define a default value", column.Name),
			Suggestion: "add an explicit DEFAULT clause or disable the rule for this policy",
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"column": column.Name,
				"type":   column.Type,
			},
		})
	}
	return findings, nil
}

type columnNotNullRequiredRule struct {
	required      bool
	allowTimeNull bool
	level         rule.Level
}

func newColumnNotNullRequiredRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDColumnNotNullRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	allowTimeNull, err := boolParam(ruleIDColumnNotNullRequire, cfg, "allow_time_null", true)
	if err != nil {
		return nil, err
	}

	return columnNotNullRequiredRule{
		required:      required,
		allowTimeNull: allowTimeNull,
		level:         configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r columnNotNullRequiredRule) ID() string { return ruleIDColumnNotNullRequire }

func (r columnNotNullRequiredRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToCreateTableColumns(statement)
}

func (r columnNotNullRequiredRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, column := range statement.DDL.Columns {
		if column.NotNull || isBlobTextLike(column) || (r.allowTimeNull && isTimeLike(column)) {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("column %q should be declared NOT NULL", column.Name),
			Suggestion: "mark the column as NOT NULL or relax the policy for allowed nullable types",
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"column": column.Name,
				"type":   column.Type,
			},
		})
	}
	return findings, nil
}

type columnFloatDoubleForbiddenRule struct {
	forbid bool
	level  rule.Level
}

func newColumnFloatDoubleForbiddenRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleIDColumnFloatDoubleForbid, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}

	return columnFloatDoubleForbiddenRule{
		forbid: forbid,
		level:  configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r columnFloatDoubleForbiddenRule) ID() string { return ruleIDColumnFloatDoubleForbid }

func (r columnFloatDoubleForbiddenRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToCreateTableColumns(statement)
}

func (r columnFloatDoubleForbiddenRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, column := range statement.DDL.Columns {
		switch baseType(column) {
		case "float", "double":
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("column %q should not use %s", column.Name, baseType(column)),
				Suggestion: "prefer int, bigint, or decimal depending on the data semantics",
				Metadata: map[string]any{
					"table":  statement.DDL.Table.Name,
					"column": column.Name,
					"type":   column.Type,
				},
			})
		}
	}
	return findings, nil
}

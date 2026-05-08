// Package ddl defines Tier-1 DDL rules.
// input: create-table Statement specs with enriched column type and charset metadata plus policy values
// output: findings for type-family, char-length, and charset/collation governance
// pos: DDL rule implementations for create-table type-family breadth checks
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type columnTypeForbiddenRule struct {
	ruleID      string
	label       string
	level       rule.Level
	forbid      bool
	suggestion  string
	matchesType func(spec.Column) bool
}

func newColumnTypeForbiddenRule(ruleID, label string, fallbackLevel rule.Level, suggestion string, matches func(spec.Column) bool, cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleID, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}
	return columnTypeForbiddenRule{
		ruleID:      ruleID,
		label:       label,
		level:       configuredLevel(cfg, fallbackLevel),
		forbid:      forbid,
		suggestion:  suggestion,
		matchesType: matches,
	}, nil
}

func (r columnTypeForbiddenRule) ID() string { return r.ruleID }

func (r columnTypeForbiddenRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToCreateTableColumns(statement)
}

func (r columnTypeForbiddenRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, column := range statement.DDL.Columns {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !r.matchesType(column) {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("column %q must not use the %s type family", column.Name, r.label),
			Suggestion: r.suggestion,
			Metadata: map[string]any{
				"table":   statement.DDL.Table.Name,
				"column":  column.Name,
				"type":    column.Type,
				"family":  r.label,
				"charset": column.Charset,
			},
		})
	}
	return findings, nil
}

type columnCharMaxLengthRule struct {
	limit int
	level rule.Level
}

func newColumnCharMaxLengthRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	limit, err := boundedIntParam(ruleIDColumnCharMaxLength, cfg, "limit", 64, 1)
	if err != nil {
		return nil, err
	}
	return columnCharMaxLengthRule{limit: limit, level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r columnCharMaxLengthRule) ID() string { return ruleIDColumnCharMaxLength }

func (r columnCharMaxLengthRule) AppliesTo(statement spec.Statement) bool {
	return appliesToCreateTableColumns(statement)
}

func (r columnCharMaxLengthRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, column := range statement.DDL.Columns {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if baseType(column) != "char" || column.Length == 0 || column.Length <= r.limit {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("char column %q must not exceed %d characters", column.Name, r.limit),
			Suggestion: "switch the column to varchar or lower the char length",
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

type columnCharsetAllowlistRule struct {
	ruleID  string
	field   string
	level   rule.Level
	allowed map[string]struct{}
}

func newColumnValueAllowlistRule(ruleID, field string, fallback []string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	allowed, err := normalizedStringSetParam(ruleID, cfg, "values", fallback)
	if err != nil {
		return nil, err
	}
	return columnCharsetAllowlistRule{
		ruleID:  ruleID,
		field:   field,
		level:   configuredLevel(cfg, fallbackLevel),
		allowed: allowed,
	}, nil
}

func (r columnCharsetAllowlistRule) ID() string { return r.ruleID }

func (r columnCharsetAllowlistRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect == spec.DialectPostgreSQL {
		return false
	}
	return len(r.allowed) > 0 && appliesToCreateTableColumns(statement)
}

func (r columnCharsetAllowlistRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, column := range statement.DDL.Columns {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		value := strings.ToLower(strings.TrimSpace(column.Charset))
		if r.field == "collation" {
			value = strings.ToLower(strings.TrimSpace(column.Collation))
		}
		if value == "" {
			continue
		}
		if _, ok := r.allowed[value]; ok {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("column %q uses unsupported %s %q", column.Name, r.field, value),
			Suggestion: fmt.Sprintf("choose an allowed %s or remove the explicit column-level %s override", r.field, r.field),
			Metadata: map[string]any{
				"table":   statement.DDL.Table.Name,
				"column":  column.Name,
				"field":   r.field,
				"value":   value,
				"allowed": sortedSet(r.allowed),
			},
		})
	}
	return findings, nil
}

type columnCharsetCollationMatchRule struct {
	required bool
	level    rule.Level
}

func newColumnCharsetCollationMatchRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDColumnCharsetCollationMatchRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return columnCharsetCollationMatchRule{required: required, level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r columnCharsetCollationMatchRule) ID() string { return ruleIDColumnCharsetCollationMatchRequire }

func (r columnCharsetCollationMatchRule) AppliesTo(statement spec.Statement) bool {
	return r.required && statement.Dialect != spec.DialectPostgreSQL && appliesToCreateTableColumns(statement)
}

func (r columnCharsetCollationMatchRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, column := range statement.DDL.Columns {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		charset := strings.ToLower(strings.TrimSpace(column.Charset))
		collation := strings.ToLower(strings.TrimSpace(column.Collation))
		if charset == "" && collation == "" {
			continue
		}
		if charset == "" || collation == "" {
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("column %q must specify charset and collation together", column.Name),
				Suggestion: "set both CHARACTER SET and COLLATE explicitly, or remove the partial override",
				Metadata: map[string]any{
					"table":     statement.DDL.Table.Name,
					"column":    column.Name,
					"charset":   charset,
					"collation": collation,
				},
			})
			continue
		}
		if !strings.HasPrefix(collation, charset+"_") {
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("column %q collation %q must match charset %q", column.Name, collation, charset),
				Suggestion: "choose a collation whose prefix matches the configured charset",
				Metadata: map[string]any{
					"table":     statement.DDL.Table.Name,
					"column":    column.Name,
					"charset":   charset,
					"collation": collation,
				},
			})
		}
	}
	return findings, nil
}

func sortedSet(items map[string]struct{}) []string {
	out := make([]string, 0, len(items))
	for item := range items {
		out = append(out, item)
	}
	// Small local sort to keep metadata deterministic without adding another helper package seam.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

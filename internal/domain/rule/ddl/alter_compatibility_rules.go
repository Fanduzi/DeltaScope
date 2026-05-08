// Package ddl defines Tier-1 DDL rules.
// input: metadata-enriched alter-table Statement specs with current and target column shapes
// output: source-aware alter compatibility findings for change/modify column operations
// pos: DDL compatibility rules layered above metadata-backed existence checks
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type alterColumnCompatibilityRule struct {
	ruleID   string
	action   string
	label    string
	required bool
	level    rule.Level
}

type alterTableOptionCompatibilityRule struct {
	required bool
	level    rule.Level
}

func newAlterColumnCompatibilityRule(ruleID, action, label string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleID, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return alterColumnCompatibilityRule{
		ruleID:   ruleID,
		action:   action,
		label:    label,
		required: required,
		level:    configuredLevel(cfg, fallbackLevel),
	}, nil
}

func newAlterTableOptionCompatibilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDAlterTableOptionCompatibilityRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return alterTableOptionCompatibilityRule{
		required: required,
		level:    configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r alterColumnCompatibilityRule) ID() string { return r.ruleID }

func (r alterTableOptionCompatibilityRule) ID() string {
	return ruleIDAlterTableOptionCompatibilityRequire
}

func (r alterColumnCompatibilityRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToAlterActions(statement, r.action)
}

func (r alterTableOptionCompatibilityRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToAlterActions(statement, "table_option")
}

func (r alterColumnCompatibilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || !snapshot.Exists {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.action) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		source := snapshot.FindColumn(alter.Name)
		target, ok := alterColumnDefinition(alter)
		if source == nil || !ok {
			continue
		}
		findings = append(findings, compatibilityFindings(r.ruleID, r.level, statement.DDL.Table.Name, alter, *source, *target)...)
	}
	return findings, nil
}

func (r alterTableOptionCompatibilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || !snapshot.Exists {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "table_option") {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		findings = append(findings, optionCompatibilityFindings(r.level, statement.DDL.Table.Name, snapshot, alter)...)
	}
	return findings, nil
}

func compatibilityFindings(ruleID string, level rule.Level, tableName string, alter spec.Alter, source spec.Column, target spec.Column) []rule.Finding {
	columnName := target.Name
	if columnName == "" {
		columnName = alter.Name
	}
	findings := make([]rule.Finding, 0)

	sourceFamily := columnTypeFamily(source)
	targetFamily := columnTypeFamily(target)
	if sourceFamily != targetFamily {
		findings = append(findings, newCompatibilityFinding(ruleID, level, tableName, alter, columnName,
			fmt.Sprintf("column %q changes type family from %q to %q", alter.Name, sourceFamily, targetFamily),
			"keep the column in the same type family or split the change into a reviewed migration",
			map[string]any{"source_type": source.Type, "target_type": target.Type, "source_family": sourceFamily, "target_family": targetFamily},
		))
		return findings
	}

	if sourceFamily == "integer" && integerTypeRank(target) < integerTypeRank(source) {
		findings = append(findings, newCompatibilityFinding(ruleID, level, tableName, alter, columnName,
			fmt.Sprintf("column %q narrows integer width from %q to %q", alter.Name, source.Type, target.Type),
			"widen the target integer type or migrate data before shrinking integer width",
			map[string]any{"source_type": source.Type, "target_type": target.Type},
		))
	}

	if sourceFamily == "string" && target.Length > 0 && source.Length > 0 && target.Length < source.Length {
		findings = append(findings, newCompatibilityFinding(ruleID, level, tableName, alter, columnName,
			fmt.Sprintf("column %q shrinks string length from %d to %d", alter.Name, source.Length, target.Length),
			"keep the new length at or above the current size or validate existing data before shrinking",
			map[string]any{"source_length": source.Length, "target_length": target.Length},
		))
	}

	if source.Unsigned != target.Unsigned {
		findings = append(findings, newCompatibilityFinding(ruleID, level, tableName, alter, columnName,
			fmt.Sprintf("column %q changes unsigned mode from %t to %t", alter.Name, source.Unsigned, target.Unsigned),
			"keep signedness unchanged unless the current data range is fully reviewed",
			map[string]any{"source_unsigned": source.Unsigned, "target_unsigned": target.Unsigned},
		))
	}

	if !source.NotNull && target.NotNull {
		findings = append(findings, newCompatibilityFinding(ruleID, level, tableName, alter, columnName,
			fmt.Sprintf("column %q tightens nullability from nullable to not null", alter.Name),
			"clean existing NULL values before making the column NOT NULL",
			map[string]any{"source_not_null": source.NotNull, "target_not_null": target.NotNull},
		))
	}

	if source.AutoIncrement && !target.AutoIncrement {
		findings = append(findings, newCompatibilityFinding(ruleID, level, tableName, alter, columnName,
			fmt.Sprintf("column %q removes auto_increment", alter.Name),
			"keep auto_increment unchanged or perform a reviewed multi-step migration",
			map[string]any{"source_auto_increment": source.AutoIncrement, "target_auto_increment": target.AutoIncrement},
		))
	}

	return findings
}

func newCompatibilityFinding(ruleID string, level rule.Level, tableName string, alter spec.Alter, columnName string, message string, suggestion string, metadata map[string]any) rule.Finding {
	payload := map[string]any{
		"table":       tableName,
		"action":      alter.Action,
		"name":        alter.Name,
		"column_name": columnName,
	}
	for key, value := range metadata {
		payload[key] = value
	}
	return rule.Finding{
		RuleID:     ruleID,
		Level:      level,
		Message:    message,
		Suggestion: suggestion,
		Metadata:   payload,
	}
}

func optionCompatibilityFindings(level rule.Level, tableName string, snapshot *spec.TableSnapshot, alter spec.Alter) []rule.Finding {
	findings := make([]rule.Finding, 0)
	for _, optionKey := range []string{"engine", "charset", "collation", "row_format", "auto_increment"} {
		target, ok := alter.Options[optionKey]
		if !ok || strings.TrimSpace(target) == "" {
			continue
		}
		source := strings.TrimSpace(snapshot.Options[optionKey])
		if source == "" {
			continue
		}
		if optionKey == "auto_increment" {
			if finding, ok := autoIncrementCompatibilityFinding(level, tableName, alter, source, target); ok {
				findings = append(findings, finding)
			}
			continue
		}
		if strings.EqualFold(source, target) {
			continue
		}
		findings = append(findings, rule.Finding{
			RuleID:     ruleIDAlterTableOptionCompatibilityRequire,
			Level:      level,
			Message:    fmt.Sprintf("table option %q changes from %q to %q", optionKey, source, target),
			Suggestion: fmt.Sprintf("keep %s aligned with the current table setting or review the compatibility impact explicitly", optionKey),
			Metadata: map[string]any{
				"table":        tableName,
				"action":       alter.Action,
				"option":       optionKey,
				"source_value": source,
				"target_value": target,
			},
		})
	}
	return findings
}

func autoIncrementCompatibilityFinding(level rule.Level, tableName string, alter spec.Alter, source, target string) (rule.Finding, bool) {
	sourceValue, sourceErr := strconv.ParseInt(source, 10, 64)
	targetValue, targetErr := strconv.ParseInt(target, 10, 64)
	if sourceErr == nil && targetErr == nil {
		if targetValue >= sourceValue {
			return rule.Finding{}, false
		}
		return rule.Finding{
			RuleID:     ruleIDAlterTableOptionCompatibilityRequire,
			Level:      level,
			Message:    fmt.Sprintf("table auto_increment seed shrinks from %d to %d", sourceValue, targetValue),
			Suggestion: "keep AUTO_INCREMENT at or above the current seed unless the reset is fully reviewed",
			Metadata: map[string]any{
				"table":        tableName,
				"action":       alter.Action,
				"option":       "auto_increment",
				"source_value": sourceValue,
				"target_value": targetValue,
			},
		}, true
	}
	if strings.EqualFold(source, target) {
		return rule.Finding{}, false
	}
	return rule.Finding{
		RuleID:     ruleIDAlterTableOptionCompatibilityRequire,
		Level:      level,
		Message:    fmt.Sprintf("table option %q changes from %q to %q", "auto_increment", source, target),
		Suggestion: "keep AUTO_INCREMENT aligned with the current seed or review the reset explicitly",
		Metadata: map[string]any{
			"table":        tableName,
			"action":       alter.Action,
			"option":       "auto_increment",
			"source_value": source,
			"target_value": target,
		},
	}, true
}

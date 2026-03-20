// Package ddl defines Tier-1 DDL rules.
// input: parser-neutral alter-table Statement specs with richer rename and target-type detail
// output: findings for semantic alter rename and conservative target-type-family checks
// pos: DDL semantic alter rule implementations layered above action-level forbids
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

var defaultConservativeAlterTypeFamilies = []string{"integer", "decimal", "string", "binary", "time"}

type forbiddenAlterRenameRule struct {
	ruleID string
	action string
	label  string
	level  rule.Level
	forbid bool
}

func newForbiddenAlterRenameRule(ruleID, action, label string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleID, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}

	return forbiddenAlterRenameRule{
		ruleID: ruleID,
		action: action,
		label:  label,
		level:  configuredLevel(cfg, fallbackLevel),
		forbid: forbid,
	}, nil
}

func (r forbiddenAlterRenameRule) ID() string { return r.ruleID }

func (r forbiddenAlterRenameRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToAlterActions(statement, r.action)
}

func (r forbiddenAlterRenameRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.action) {
		oldName, newName, ok := alterRenameNames(alter)
		if !ok {
			continue
		}
		findings = append(findings, rule.Finding{
			RuleID:     r.ruleID,
			Level:      r.level,
			Message:    fmt.Sprintf("ALTER TABLE %s from %q to %q is forbidden", r.label, oldName, newName),
			Suggestion: fmt.Sprintf("keep the existing %s name or relax the policy intentionally", r.label),
			Metadata: map[string]any{
				"table":    statement.DDL.Table.Name,
				"action":   alter.Action,
				"name":     alter.Name,
				"old_name": oldName,
				"new_name": newName,
			},
		})
	}
	return findings, nil
}

type alterTargetTypeFamilyRule struct {
	ruleID          string
	action          string
	label           string
	level           rule.Level
	required        bool
	allowedFamilies map[string]struct{}
}

func newAlterTargetTypeFamilyRule(ruleID, action, label string, fallbackLevel rule.Level, fallbackFamilies []string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleID, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	allowedFamilies, err := normalizedStringSetParam(ruleID, cfg, "allowed_type_families", fallbackFamilies)
	if err != nil {
		return nil, err
	}

	return alterTargetTypeFamilyRule{
		ruleID:          ruleID,
		action:          action,
		label:           label,
		level:           configuredLevel(cfg, fallbackLevel),
		required:        required,
		allowedFamilies: allowedFamilies,
	}, nil
}

func (r alterTargetTypeFamilyRule) ID() string { return r.ruleID }

func (r alterTargetTypeFamilyRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToAlterActions(statement, r.action)
}

func (r alterTargetTypeFamilyRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.action) {
		column, ok := alterColumnDefinition(alter)
		if !ok {
			continue
		}

		family := columnTypeFamily(*column)
		if _, allowed := r.allowedFamilies[family]; allowed {
			continue
		}

		targetName := column.Name
		if targetName == "" {
			targetName = alter.Name
		}

		findings = append(findings, rule.Finding{
			RuleID: r.ruleID,
			Level:  r.level,
			Message: fmt.Sprintf(
				"ALTER TABLE %s target type %q uses conservative type family %q for %q, which is outside the allowed offline-safe families",
				r.label,
				column.Type,
				family,
				targetName,
			),
			Suggestion: fmt.Sprintf(
				"use a conservatively allowed target type family for %q or relax the policy intentionally after manual review",
				targetName,
			),
			Metadata: map[string]any{
				"table":       statement.DDL.Table.Name,
				"action":      alter.Action,
				"name":        alter.Name,
				"column_name": targetName,
				"target_type": column.Type,
				"type_family": family,
			},
		})
	}
	return findings, nil
}

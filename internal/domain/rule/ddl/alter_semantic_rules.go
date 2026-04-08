// Package ddl defines Tier-1 DDL rules.
// input: parser-neutral alter Statement specs with richer rename, add-index, target-type, and explicit-change detail
// output: findings for semantic alter rename, standalone rename-index forbids, alter-added index lifecycle, and conservative target-type-family checks
// pos: DDL semantic alter rule implementations layered above action-level forbids and shared rename semantics
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
	return r.forbid && len(matchingRenameActions(statement, r.action)) > 0
}

func (r forbiddenAlterRenameRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingRenameActions(statement, r.action) {
		oldName, newName, ok := alterRenameNames(alter)
		if !ok {
			continue
		}
		metadata := map[string]any{
			"action":   alter.Action,
			"name":     alter.Name,
			"old_name": oldName,
			"new_name": newName,
		}
		if statement.DDL != nil && statement.DDL.Table != nil && statement.DDL.Table.Name != "" {
			metadata["table"] = statement.DDL.Table.Name
		}
		message := fmt.Sprintf("ALTER TABLE %s from %q to %q is forbidden", r.label, oldName, newName)
		if statement.DDL == nil || statement.DDL.Table == nil {
			message = fmt.Sprintf("DDL %s from %q to %q is forbidden", r.label, oldName, newName)
		}
		findings = append(findings, rule.Finding{
			RuleID:     r.ruleID,
			Level:      r.level,
			Message:    message,
			Suggestion: fmt.Sprintf("keep the existing %s name or relax the policy intentionally", r.label),
			Metadata:   metadata,
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

type forbiddenExplicitAlterColumnChangeRule struct {
	ruleID     string
	action     string
	label      string
	changeKind string
	level      rule.Level
	forbid     bool
	predicate  func(spec.Alter) bool
}

func newForbiddenExplicitAlterColumnChangeRule(ruleID, action, label, changeKind string, fallbackLevel rule.Level, predicate func(spec.Alter) bool, cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleID, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}

	return forbiddenExplicitAlterColumnChangeRule{
		ruleID:     ruleID,
		action:     action,
		label:      label,
		changeKind: changeKind,
		level:      configuredLevel(cfg, fallbackLevel),
		forbid:     forbid,
		predicate:  predicate,
	}, nil
}

func (r forbiddenExplicitAlterColumnChangeRule) ID() string { return r.ruleID }

func (r forbiddenExplicitAlterColumnChangeRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToAlterActions(statement, r.action)
}

func (r forbiddenExplicitAlterColumnChangeRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.action) {
		if !r.predicate(alter) {
			continue
		}

		targetName := alter.Name
		if column, ok := alterColumnDefinition(alter); ok && column.Name != "" {
			targetName = column.Name
		}

		findings = append(findings, rule.Finding{
			RuleID:  r.ruleID,
			Level:   r.level,
			Message: fmt.Sprintf("ALTER TABLE %s explicitly changes %s for %q, which this policy forbids", r.label, humanizeExplicitChangeKind(r.changeKind), targetName),
			Suggestion: fmt.Sprintf(
				"keep %s unchanged for %q or relax the policy intentionally after review",
				humanizeExplicitChangeKind(r.changeKind),
				targetName,
			),
			Metadata: map[string]any{
				"table":       statement.DDL.Table.Name,
				"action":      alter.Action,
				"name":        alter.Name,
				"column_name": targetName,
				"change_kind": r.changeKind,
			},
		})
	}
	return findings, nil
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

type alterAddedIndexPrefixRule struct {
	inner rule.StatementRule
	kind  spec.IndexKind
}

type alterAddedIndexSuffixRule struct {
	inner rule.StatementRule
	kind  spec.IndexKind
}

type alterAddedIndexContainsRule struct {
	inner rule.StatementRule
	kind  spec.IndexKind
}

type alterAddedIndexRule struct {
	ruleID  string
	inner   rule.StatementRule
	indexes func(spec.Statement) []spec.Index
}

func newAlterAddedIndexPrefixRule(ruleID string, kind spec.IndexKind, fallbackPrefix string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	inner, err := newIndexPrefixRequiredRule(ruleID, kind, fallbackPrefix, fallbackLevel, cfg)
	if err != nil {
		return nil, err
	}
	return alterAddedIndexPrefixRule{
		inner: inner,
		kind:  kind,
	}, nil
}

func (r alterAddedIndexPrefixRule) ID() string { return r.inner.ID() }

func (r alterAddedIndexPrefixRule) AppliesTo(statement spec.Statement) bool {
	return len(alterAddedIndexesByKind(statement, r.kind)) > 0
}

func (r alterAddedIndexPrefixRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings, err := evaluateProjectedAlterIndexRule(
		r.inner,
		r.ID(),
		projectedAlterIndexesStatement(statement, alterAddedIndexesByKind(statement, r.kind)),
	)
	if err != nil {
		return nil, err
	}
	return findings, nil
}

func newAlterAddedIndexSuffixRule(ruleID string, kind spec.IndexKind, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	inner, err := newIndexSuffixRequiredRule(ruleID, kind, fallbackLevel, cfg)
	if err != nil {
		return nil, err
	}
	return alterAddedIndexSuffixRule{
		inner: inner,
		kind:  kind,
	}, nil
}

func (r alterAddedIndexSuffixRule) ID() string { return r.inner.ID() }

func (r alterAddedIndexSuffixRule) AppliesTo(statement spec.Statement) bool {
	return len(alterAddedIndexesByKind(statement, r.kind)) > 0
}

func (r alterAddedIndexSuffixRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings, err := evaluateProjectedAlterIndexRule(
		r.inner,
		r.ID(),
		projectedAlterIndexesStatement(statement, alterAddedIndexesByKind(statement, r.kind)),
	)
	if err != nil {
		return nil, err
	}
	return findings, nil
}

func newAlterAddedIndexContainsRule(ruleID string, kind spec.IndexKind, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	inner, err := newIndexContainsRequiredRule(ruleID, kind, fallbackLevel, cfg)
	if err != nil {
		return nil, err
	}
	return alterAddedIndexContainsRule{
		inner: inner,
		kind:  kind,
	}, nil
}

func (r alterAddedIndexContainsRule) ID() string { return r.inner.ID() }

func (r alterAddedIndexContainsRule) AppliesTo(statement spec.Statement) bool {
	return len(alterAddedIndexesByKind(statement, r.kind)) > 0
}

func (r alterAddedIndexContainsRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings, err := evaluateProjectedAlterIndexRule(
		r.inner,
		r.ID(),
		projectedAlterIndexesStatement(statement, alterAddedIndexesByKind(statement, r.kind)),
	)
	if err != nil {
		return nil, err
	}
	return findings, nil
}

func newAlterAddedIndexColumnsMaxCountRule(fallbackLimit int, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	inner, err := newIndexColumnsMaxCountRule(policy.RulePolicy{
		Enabled: cfg.Enabled,
		Level:   configuredLevel(cfg, fallbackLevel),
		Params:  cfg.Params,
	})
	if err != nil {
		return nil, err
	}
	return alterAddedIndexRule{
		ruleID:  ruleIDAlterAddIndexColumnsMaxCount,
		inner:   inner,
		indexes: allAlterAddedIndexes,
	}, nil
}

func newAlterAddedDuplicateIndexForbiddenRule(fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	inner, err := newDuplicateIndexForbiddenRule(policy.RulePolicy{
		Enabled: cfg.Enabled,
		Level:   configuredLevel(cfg, fallbackLevel),
		Params:  cfg.Params,
	})
	if err != nil {
		return nil, err
	}
	return alterAddedIndexRule{
		ruleID:  ruleIDAlterAddIndexDuplicateForbid,
		inner:   inner,
		indexes: allAlterAddedIndexes,
	}, nil
}

func newAlterAddedRedundantLeftPrefixRule(fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	inner, err := newRedundantLeftPrefixIndexRule(policy.RulePolicy{
		Enabled: cfg.Enabled,
		Level:   configuredLevel(cfg, fallbackLevel),
		Params:  cfg.Params,
	})
	if err != nil {
		return nil, err
	}
	return alterAddedIndexRule{
		ruleID:  ruleIDAlterAddIndexRedundantLeftPrefixForbid,
		inner:   inner,
		indexes: alterLifecycleIndexes,
	}, nil
}

func newAlterAddedRedundantUniqueOverlapRule(fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	inner, err := newRedundantUniqueOverlapIndexRule(policy.RulePolicy{
		Enabled: cfg.Enabled,
		Level:   configuredLevel(cfg, fallbackLevel),
		Params:  cfg.Params,
	})
	if err != nil {
		return nil, err
	}
	return alterAddedIndexRule{
		ruleID:  ruleIDAlterAddIndexRedundantUniqueOverlapForbid,
		inner:   inner,
		indexes: alterLifecycleIndexes,
	}, nil
}

func (r alterAddedIndexRule) ID() string { return r.ruleID }

func (r alterAddedIndexRule) AppliesTo(statement spec.Statement) bool {
	return len(r.indexes(statement)) > 0
}

func (r alterAddedIndexRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	findings, err := evaluateProjectedAlterIndexRule(
		r.inner,
		r.ruleID,
		projectedAlterIndexesStatement(statement, r.indexes(statement)),
	)
	if err != nil {
		return nil, err
	}
	return findings, nil
}

func evaluateProjectedAlterIndexRule(inner rule.StatementRule, ruleID string, projected spec.Statement) ([]rule.Finding, error) {
	findings, err := inner.Evaluate(projected)
	if err != nil {
		return nil, err
	}
	for i := range findings {
		findings[i].RuleID = ruleID
	}
	return findings, nil
}

func allAlterAddedIndexes(statement spec.Statement) []spec.Index {
	if !appliesToAlterActions(statement, "add_constraint") {
		return nil
	}

	indexes := make([]spec.Index, 0)
	for _, alter := range matchingAlterActions(statement, "add_constraint") {
		index, ok := alterIndexDefinition(alter)
		if !ok {
			continue
		}
		indexes = append(indexes, *index)
	}
	return indexes
}

func alterLifecycleIndexes(statement spec.Statement) []spec.Index {
	indexes := make([]spec.Index, 0)
	if snapshot, ok := targetTableSnapshot(statement); ok && snapshot.Exists {
		indexes = append(indexes, snapshot.Indexes...)
	}
	indexes = append(indexes, allAlterAddedIndexes(statement)...)
	return indexes
}

func humanizeExplicitChangeKind(changeKind string) string {
	switch changeKind {
	case "explicit_nullability_change":
		return "nullability"
	case "explicit_default_change":
		return "default value"
	case "explicit_auto_increment_change":
		return "auto_increment"
	default:
		return changeKind
	}
}

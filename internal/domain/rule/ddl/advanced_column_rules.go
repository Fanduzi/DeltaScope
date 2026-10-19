// Package ddl defines Tier-1 DDL rules.
// input: advanced-column shared assessments (dependency effects, compatibility issues/gaps) and policy config
// output: the column-dependency and advanced-compatibility statement rules with contract-frozen finding/gap shapes
// pos: DDL rule registration for T07 generated/expression-default/visibility checks
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

// columnDependencyRule blocks column drops and identity changes whose source
// column is referenced by generated or expression-default facts on sibling
// columns.
type columnDependencyRule struct {
	ruleID   string
	required bool
	level    rule.Level
}

func newColumnDependencyRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDAlterColumnDependencyRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return columnDependencyRule{
		ruleID:   ruleIDAlterColumnDependencyRequire,
		required: required,
		level:    configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r columnDependencyRule) ID() string { return r.ruleID }

func (r columnDependencyRule) AppliesTo(statement spec.Statement) bool {
	if !r.required {
		return false
	}
	return len(spec.AssessAdvancedColumns(statement).DependencyEffects) > 0
}

func (r columnDependencyRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	assessment := spec.AssessAdvancedColumns(statement)
	snapshot, _ := targetTableSnapshot(statement)
	table := metadataTargetTableName(statement, snapshot)
	var findings []rule.Finding
	for _, effect := range assessment.DependencyEffects {
		for _, dependent := range effect.Dependents {
			verb := "rename"
			if effect.Action == "drop_column" {
				verb = "drop"
			}
			label := strings.ReplaceAll(dependent.Kind, "_", "-")
			findings = append(findings, rule.Finding{
				Level:   r.level,
				Message: fmt.Sprintf("cannot %s column %q: %s column %q depends on it", verb, effect.Column, label, dependent.Column),
				Metadata: map[string]any{
					"table":            table,
					"column":           effect.Column,
					"dependent_column": dependent.Column,
					"dependency_kind":  dependent.Kind,
					"dependency":       dependent.Dependency,
					"action":           effect.Action,
				},
			})
		}
	}
	return findings, nil
}

// EvidenceGaps projects the dependency-side unknowns: a withheld column set
// and an unprovable dependency graph are bounded gaps, never silent passes.
func (r columnDependencyRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	assessment := spec.AssessAdvancedColumns(statement)
	gaps := make([]rule.EvidenceGap, 0, len(assessment.DependencyGaps))
	for _, gap := range assessment.DependencyGaps {
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    gap.ReasonCode,
			RequiredFacts: append([]string(nil), gap.RequiredFacts...),
		})
	}
	return gaps
}

// advancedCompatibilityRule enforces the frozen advanced-column
// product/version responsibility table for generated columns, expression
// defaults, and visibility.
type advancedCompatibilityRule struct {
	ruleID   string
	required bool
	level    rule.Level
	features map[string]bool
}

var advancedCompatibilityFeatures = map[string]bool{
	spec.AdvancedFeatureGenerated:         true,
	spec.AdvancedFeatureExpressionDefault: true,
	spec.AdvancedFeatureVisibility:        true,
}

func newAdvancedCompatibilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDColumnAdvancedCompatibilityRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	features := map[string]bool{
		spec.AdvancedFeatureGenerated:         true,
		spec.AdvancedFeatureExpressionDefault: true,
		spec.AdvancedFeatureVisibility:        true,
	}
	if raw, ok := cfg.Params["features"]; ok {
		features = map[string]bool{}
		var names []string
		switch list := raw.(type) {
		case []string:
			names = list
		case []any:
			for _, item := range list {
				name, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("rule %s param %q entries must be strings, got %T", ruleIDColumnAdvancedCompatibilityRequire, "features", item)
				}
				names = append(names, name)
			}
		default:
			return nil, fmt.Errorf("rule %s param %q must be a list of feature names, got %T", ruleIDColumnAdvancedCompatibilityRequire, "features", raw)
		}
		for _, name := range names {
			if !advancedCompatibilityFeatures[name] {
				return nil, fmt.Errorf("rule %s param %q has unknown feature %q", ruleIDColumnAdvancedCompatibilityRequire, "features", name)
			}
			features[name] = true
		}
	}
	return advancedCompatibilityRule{
		ruleID:   ruleIDColumnAdvancedCompatibilityRequire,
		required: required,
		level:    configuredLevel(cfg, rule.LevelBlocker),
		features: features,
	}, nil
}

func (r advancedCompatibilityRule) ID() string { return r.ruleID }

func (r advancedCompatibilityRule) featureEnabled(feature string) bool {
	return r.features[feature]
}

func (r advancedCompatibilityRule) AppliesTo(statement spec.Statement) bool {
	if !r.required || len(r.features) == 0 {
		return false
	}
	assessment := spec.AssessAdvancedColumns(statement)
	for feature := range assessment.Features {
		if r.featureEnabled(feature) {
			return true
		}
	}
	return false
}

func (r advancedCompatibilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	assessment := spec.AssessAdvancedColumns(statement)
	snapshot, _ := targetTableSnapshot(statement)
	table := metadataTargetTableName(statement, snapshot)
	var version *spec.VersionIdentity
	if statement.Metadata != nil {
		version = statement.Metadata.Version
	}
	targetVersion := ""
	if version != nil {
		targetVersion = version.Version
	}
	var findings []rule.Finding
	for _, issue := range assessment.Issues {
		if !r.featureEnabled(issue.Feature) {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: fmt.Sprintf("column %q advanced definition is not supported for the target (%s)", issue.Column, issue.ReasonCode),
			Metadata: map[string]any{
				"table":           table,
				"column":          issue.Column,
				"feature":         issue.Feature,
				"action":          issue.Action,
				"reason_code":     issue.ReasonCode,
				"product":         spec.VersionProductForDialect(statement.Dialect),
				"target_version":  targetVersion,
				"minimum_version": issue.MinimumVersion,
			},
		})
	}
	return findings, nil
}

// EvidenceGaps projects feature-scoped compatibility gaps: version gaps apply
// only when an enabled feature consults the version, and feature-specific
// gaps stay inside their enabled family.
func (r advancedCompatibilityRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	assessment := spec.AssessAdvancedColumns(statement)
	gated := false
	for feature := range assessment.VersionGated {
		if r.featureEnabled(feature) {
			gated = true
			break
		}
	}
	var gaps []rule.EvidenceGap
	for _, gap := range assessment.Gaps {
		versionGap := gap.ReasonCode == spec.AdvancedReasonMissingTargetVersion ||
			gap.ReasonCode == spec.AdvancedReasonOutOfValidatedRange
		if versionGap && !gated {
			continue
		}
		if gap.Feature != "" && !r.featureEnabled(gap.Feature) {
			continue
		}
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    gap.ReasonCode,
			RequiredFacts: append([]string(nil), gap.RequiredFacts...),
		})
	}
	return gaps
}

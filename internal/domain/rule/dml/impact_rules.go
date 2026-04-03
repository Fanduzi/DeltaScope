// Package dml defines Tier-1 DML rules.
// input: DML impact estimates and policy-backed threshold values
// output: conservative affected-row findings for mutation statements
// pos: DML impact estimation rule implementations
// note: if this file changes, update this header and module README.md.
package dml

import (
	"fmt"
	"math"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	ruleIDImpactEstimate        = "dml.impact.estimate"
	ruleIDImpactRowsMaxCount    = "dml.impact.rows.max_count"
	ruleIDImpactRatioMaxPercent = "dml.impact.ratio.max_percent"
)

type impactEstimateRule struct{}

func newImpactEstimateRule(policy.RulePolicy) (rule.StatementRule, error) {
	return impactEstimateRule{}, nil
}

func (r impactEstimateRule) ID() string { return ruleIDImpactEstimate }

func (r impactEstimateRule) AppliesTo(statement spec.Statement) bool {
	return appliesToMutation(statement)
}

func (r impactEstimateRule) Evaluate(spec.Statement) ([]rule.Finding, error) {
	return nil, nil
}

type impactRowsMaxCountRule struct {
	limit int
	level rule.Level
}

func newImpactRowsMaxCountRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	limit, err := intParam(ruleIDImpactRowsMaxCount, cfg, "value", 1000)
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, fmt.Errorf("rule %s param %q must be >= 1, got %d", ruleIDImpactRowsMaxCount, "value", limit)
	}

	return impactRowsMaxCountRule{
		limit: limit,
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r impactRowsMaxCountRule) ID() string { return ruleIDImpactRowsMaxCount }

func (r impactRowsMaxCountRule) AppliesTo(statement spec.Statement) bool {
	return appliesToMutation(statement) &&
		statement.DML.Impact != nil &&
		statement.DML.Impact.EstimatedRows != nil
}

func (r impactRowsMaxCountRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	estimatedRows := *statement.DML.Impact.EstimatedRows
	if estimatedRows <= int64(r.limit) {
		return nil, nil
	}

	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("Estimated affected rows %d exceed configured threshold %d", estimatedRows, r.limit),
		Metadata: map[string]any{
			"estimated_rows": estimatedRows,
			"threshold":      r.limit,
			"impact_source":  statement.DML.Impact.Source,
		},
	}}, nil
}

type impactRatioMaxPercentRule struct {
	limit float64
	level rule.Level
}

func newImpactRatioMaxPercentRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	limit, err := floatParam(ruleIDImpactRatioMaxPercent, cfg, "value", 10)
	if err != nil {
		return nil, err
	}
	if math.IsNaN(limit) || math.IsInf(limit, 0) || limit < 0 || limit > 100 {
		return nil, fmt.Errorf("rule %s param %q must be finite and between 0 and 100, got %v", ruleIDImpactRatioMaxPercent, "value", limit)
	}

	return impactRatioMaxPercentRule{
		limit: limit,
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r impactRatioMaxPercentRule) ID() string { return ruleIDImpactRatioMaxPercent }

func (r impactRatioMaxPercentRule) AppliesTo(statement spec.Statement) bool {
	return appliesToMutation(statement) &&
		statement.DML.Impact != nil &&
		statement.DML.Impact.EstimatedRatio != nil
}

func (r impactRatioMaxPercentRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	estimatedRatio := *statement.DML.Impact.EstimatedRatio
	estimatedPercent := estimatedRatio * 100
	if estimatedPercent <= r.limit {
		return nil, nil
	}

	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("Estimated affected row ratio %.2f%% exceed configured threshold %.2f%%", estimatedPercent, r.limit),
		Metadata: map[string]any{
			"estimated_ratio":   estimatedRatio,
			"estimated_percent": estimatedPercent,
			"threshold":         r.limit,
			"impact_source":     statement.DML.Impact.Source,
		},
	}}, nil
}

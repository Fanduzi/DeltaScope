// Package dml verifies Tier-1 DML impact-rule behavior.
// input: synthetic update/delete Statement specs and rule policy overrides
// output: deterministic findings for impact estimate guardrails
// pos: domain DML rule test coverage for affected-row threshold rules
// note: if this file changes, update this header and module README.md.
package dml

import (
	"context"
	"math"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestImpactRowsMaxCountRuleFindsLargeEstimate(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newImpactRowsMaxCountRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"value": 1000},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), statementWithImpact(&spec.ImpactEstimate{
		EstimatedRows: ptrInt64(5000),
		RiskLevel:     spec.ImpactRiskHigh,
		Confidence:    spec.ImpactConfidenceMedium,
		Source:        spec.ImpactSourceMetadata,
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(findings))
	}
	if findings[0].Message == "" {
		t.Fatal("expected a descriptive finding message")
	}
	if got := findings[0].Metadata["estimated_rows"]; got != int64(5000) {
		t.Fatalf("expected estimated rows metadata, got %#v", got)
	}
}

func TestImpactRowsMaxCountRuleRejectsThresholdBelowOne(t *testing.T) {
	t.Parallel()
	_, err := newImpactRowsMaxCountRule(policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"value": 0},
	})
	if err == nil {
		t.Fatal("expected rows threshold below one to be rejected")
	}
}

func TestImpactRowsMaxCountRuleSkipsUnknownEstimate(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newImpactRowsMaxCountRule(policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"value": 1000},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), statementWithImpact(nil))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 0 {
		t.Fatalf("expected no findings for missing impact estimate, got %d", len(findings))
	}
}

func TestImpactRatioMaxPercentRuleFindsLargeEstimate(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newImpactRatioMaxPercentRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"value": 10},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), statementWithImpact(&spec.ImpactEstimate{
		EstimatedRatio: ptrFloat64(0.25),
		RiskLevel:      spec.ImpactRiskHigh,
		Confidence:     spec.ImpactConfidenceHigh,
		Source:         spec.ImpactSourceShape,
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(findings))
	}
	if findings[0].Message == "" {
		t.Fatal("expected a descriptive finding message")
	}
	if got := findings[0].Metadata["estimated_ratio"]; got != 0.25 {
		t.Fatalf("expected estimated ratio metadata, got %#v", got)
	}
}

func TestImpactRatioMaxPercentRuleAcceptsNumericVariants(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		value any
	}{
		{name: "float64", value: float64(42)},
		{name: "float32", value: float32(42)},
		{name: "int", value: int(42)},
		{name: "int8", value: int8(42)},
		{name: "int16", value: int16(42)},
		{name: "int32", value: int32(42)},
		{name: "int64", value: int64(42)},
		{name: "uint", value: uint(42)},
		{name: "uint8", value: uint8(42)},
		{name: "uint16", value: uint16(42)},
		{name: "uint32", value: uint32(42)},
		{name: "uint64", value: uint64(42)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := newImpactRatioMaxPercentRule(policy.RulePolicy{
				Enabled: true,
				Params:  map[string]any{"value": tc.value},
			})
			if err != nil {
				t.Fatalf("construct rule: %v", err)
			}
		})
	}
}

func TestImpactRatioMaxPercentRuleSkipsUnknownEstimate(t *testing.T) {
	t.Parallel()
	ruleUnderTest, err := newImpactRatioMaxPercentRule(policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"value": 10},
	})
	if err != nil {
		t.Fatalf("construct rule: %v", err)
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), statementWithImpact(&spec.ImpactEstimate{
		EstimatedRows: ptrInt64(5000),
		RiskLevel:     spec.ImpactRiskHigh,
		Confidence:    spec.ImpactConfidenceMedium,
		Source:        spec.ImpactSourceMetadata,
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 0 {
		t.Fatalf("expected no findings for missing ratio estimate, got %d", len(findings))
	}
}

func TestImpactRatioMaxPercentRuleRejectsNegativeThreshold(t *testing.T) {
	t.Parallel()
	_, err := newImpactRatioMaxPercentRule(policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"value": -1},
	})
	if err == nil {
		t.Fatal("expected negative ratio threshold to be rejected")
	}
}

func TestImpactRatioMaxPercentRuleRejectsThresholdAboveHundred(t *testing.T) {
	t.Parallel()
	_, err := newImpactRatioMaxPercentRule(policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"value": 100.01},
	})
	if err == nil {
		t.Fatal("expected ratio threshold above 100 to be rejected")
	}
}

func TestImpactRatioMaxPercentRuleRejectsNonFiniteThreshold(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		value float64
	}{
		{name: "nan", value: math.NaN()},
		{name: "posinf", value: math.Inf(1)},
		{name: "neginf", value: math.Inf(-1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := newImpactRatioMaxPercentRule(policy.RulePolicy{
				Enabled: true,
				Params:  map[string]any{"value": tc.value},
			})
			if err == nil {
				t.Fatalf("expected %s threshold to be rejected", tc.name)
			}
		})
	}
}

func TestImpactRatioMaxPercentRuleRejectsInvalidConfig(t *testing.T) {
	t.Parallel()
	_, err := newImpactRatioMaxPercentRule(policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"value": "not-a-number"},
	})
	if err == nil {
		t.Fatal("expected invalid ratio config to be rejected")
	}
}

func statementWithImpact(impact *spec.ImpactEstimate) spec.Statement {
	return spec.Statement{
		Kind: spec.KindDML,
		DML: &spec.DML{
			Operation: spec.DMLOperationUpdate,
			Impact:    impact,
		},
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}

func ptrFloat64(v float64) *float64 {
	return &v
}

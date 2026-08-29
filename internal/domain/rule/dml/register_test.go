// Package dml verifies DML rule registration behavior.
// input: policy objects, synthetic DML statements, and the shared registry
// output: deterministic registration and evaluation coverage for the shipped DML rule batch
// pos: domain DML rule integration tests across policy-backed registry assembly
// note: if this file changes, update this header and module README.md.
package dml

import (
	"context"
	"reflect"
	"testing"
	"unsafe"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestRegisterAddsEnabledDMLRulesInDeterministicOrder(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["dml.insert.rows.max_count"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"limit": 1},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(context.Background(), insertStatement(2, true, true, true))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	wantIDs := []string{
		"dml.insert.rows.max_count",
		"dml.replace.forbid",
		"dml.insert.select.forbid",
		"dml.insert.on_duplicate.forbid",
	}
	if len(findings) != len(wantIDs) {
		t.Fatalf("expected %d findings, got %d", len(wantIDs), len(findings))
	}
	for i, want := range wantIDs {
		if findings[i].RuleID != want {
			t.Fatalf("expected finding %d to use rule %q, got %q", i, want, findings[i].RuleID)
		}
	}
}

func TestRegisterAddsImpactRulesInDeterministicOrder(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["dml.where.require"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": true},
	}
	cfg.Rules["dml.impact.estimate"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelNotice,
	}
	cfg.Rules["dml.impact.rows.max_count"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"value": 1000},
	}
	cfg.Rules["dml.impact.ratio.max_percent"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"value": 10},
	}
	cfg.Rules["dml.limit.forbid"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"forbid": true},
	}
	cfg.Rules["dml.order_by.forbid"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"forbid": true},
	}
	cfg.Rules["dml.subquery.forbid"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	}
	cfg.Rules["dml.join.on.require"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": true},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	gotIDs := registeredStatementRuleIDs(t, registry)
	wantIDs := []string{
		"dml.table.denylist.forbid",
		"dml.table.exists.require",
		"dml.where.require",
		"dml.impact.estimate",
		"dml.impact.rows.max_count",
		"dml.impact.ratio.max_percent",
		"dml.limit.forbid",
		"dml.order_by.forbid",
		"dml.subquery.forbid",
		"dml.join.on.require",
	}
	if len(gotIDs) < len(wantIDs) {
		t.Fatalf("expected at least %d registered rules, got %d", len(wantIDs), len(gotIDs))
	}
	for i, want := range wantIDs {
		if gotIDs[i] != want {
			t.Fatalf("expected rule %d to be %q, got %q", i, want, gotIDs[i])
		}
	}

	findings, err := registry.EvaluateStatement(context.Background(), spec.Statement{
		Kind: spec.KindDML,
		DML: &spec.DML{
			Operation:   spec.DMLOperationUpdate,
			HasLimit:    true,
			HasOrderBy:  true,
			HasSubquery: true,
			HasJoin:     true,
			Impact: &spec.ImpactEstimate{
				EstimatedRows:  ptrInt64(5000),
				EstimatedRatio: ptrFloat64(0.25),
				RiskLevel:      spec.ImpactRiskHigh,
				Confidence:     spec.ImpactConfidenceHigh,
				Source:         spec.ImpactSourceMetadata,
			},
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 7 {
		t.Fatalf("expected 7 findings, got %d", len(findings))
	}
	if got := findings[0].RuleID; got != "dml.where.require" {
		t.Fatalf("expected first finding from WHERE rule, got %q", got)
	}
	if got := findings[1].RuleID; got != "dml.impact.rows.max_count" {
		t.Fatalf("expected second finding from impact rows rule, got %q", got)
	}
	if got := findings[2].RuleID; got != "dml.impact.ratio.max_percent" {
		t.Fatalf("expected third finding from impact ratio rule, got %q", got)
	}
	if got := findings[3].RuleID; got != "dml.limit.forbid" {
		t.Fatalf("expected fourth finding from limit rule, got %q", got)
	}
	if got := findings[4].RuleID; got != "dml.order_by.forbid" {
		t.Fatalf("expected fifth finding from order-by rule, got %q", got)
	}
	if got := findings[5].RuleID; got != "dml.subquery.forbid" {
		t.Fatalf("expected sixth finding from subquery rule, got %q", got)
	}
	if got := findings[6].RuleID; got != "dml.join.on.require" {
		t.Fatalf("expected seventh finding from join rule, got %q", got)
	}
}

func TestRegisterSkipsDisabledDMLRules(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["dml.where.require"] = policy.RulePolicy{
		Enabled: false,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": true},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(context.Background(), updateStatement(false, false, false, false, false))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	for _, finding := range findings {
		if finding.RuleID == "dml.where.require" {
			t.Fatalf("expected disabled WHERE rule not to run, got %+v", finding)
		}
	}
}

func registeredStatementRuleIDs(t *testing.T, registry *rule.Registry) []string {
	t.Helper()

	value := reflect.ValueOf(registry).Elem().FieldByName("statementRules")
	if !value.IsValid() {
		t.Fatal("registry does not expose statementRules")
	}
	value = reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem()

	ids := make([]string, 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		ruleValue := value.Index(i)
		ruleValue = reflect.NewAt(ruleValue.Type(), unsafe.Pointer(ruleValue.UnsafeAddr())).Elem()
		statementRule, ok := ruleValue.Interface().(rule.StatementRule)
		if !ok {
			t.Fatalf("registry entry %d does not implement StatementRule", i)
		}
		ids = append(ids, statementRule.ID())
	}
	return ids
}

func TestRegisterRejectsInvalidDMLRuleConfig(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["dml.insert.rows.max_count"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"limit": "not-an-int"},
	}

	if err := Register(registry, cfg); err == nil {
		t.Fatal("expected invalid config to be rejected")
	}
}

func TestRegisterAddsDisabledTableGovernanceRule(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["dml.table.denylist.forbid"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"tables":  []string{"users"},
			"schemas": []string{"app"},
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(context.Background(), spec.Statement{
		Kind: spec.KindDML,
		DML: &spec.DML{
			Operation: spec.DMLOperationDelete,
			Tables:    []spec.Table{{Name: "users"}},
		},
		Metadata: &spec.Metadata{Schema: "app"},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	found := false
	for _, finding := range findings {
		if finding.RuleID == "dml.table.denylist.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected disabled-table finding, got %+v", findings)
	}
}

// Package dml verifies DML rule registration behavior.
// input: policy objects, synthetic DML statements, and the shared registry
// output: deterministic registration and evaluation coverage for the first DML rule batch
// pos: domain DML rule integration tests across policy-backed registry assembly
// note: if this file changes, update this header and module README.md.
package dml

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func TestRegisterAddsEnabledDMLRulesInDeterministicOrder(t *testing.T) {
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

	findings, err := registry.EvaluateStatement(insertStatement(2, true, true, true))
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

func TestRegisterSkipsDisabledDMLRules(t *testing.T) {
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

	findings, err := registry.EvaluateStatement(updateStatement(false, false, false, false, false))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	for _, finding := range findings {
		if finding.RuleID == "dml.where.require" {
			t.Fatalf("expected disabled WHERE rule not to run, got %+v", finding)
		}
	}
}

func TestRegisterRejectsInvalidDMLRuleConfig(t *testing.T) {
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

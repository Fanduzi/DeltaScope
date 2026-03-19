// Package ddl verifies DDL rule registration behavior.
// input: policy objects, synthetic create-table statements, and the shared registry
// output: deterministic registration and evaluation coverage for the first DDL rule batch
// pos: domain DDL rule integration tests across policy-backed registry assembly
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func TestRegisterAddsEnabledDDLRulesInDeterministicOrder(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["ddl.table.name.max_length"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"limit": 5,
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(createTableStatement("orders_archive", "", nil))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(findings))
	}

	wantIDs := []string{
		"ddl.table.comment.require",
		"ddl.table.name.max_length",
		"ddl.table.primary_key.require",
	}
	for i, want := range wantIDs {
		if findings[i].RuleID != want {
			t.Fatalf("expected finding %d to use rule %q, got %q", i, want, findings[i].RuleID)
		}
	}
}

func TestRegisterSkipsDisabledDDLRules(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["ddl.table.comment.require"] = policy.RulePolicy{
		Enabled: false,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"required": true,
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(createTableStatement("users", "", []string{"id"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	for _, finding := range findings {
		if finding.RuleID == "ddl.table.comment.require" {
			t.Fatalf("expected disabled comment rule not to run, got %+v", finding)
		}
	}
}

func TestRegisterRejectsInvalidDDLRuleConfig(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["ddl.table.name.max_length"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"limit": "not-an-int",
		},
	}

	if err := Register(registry, cfg); err == nil {
		t.Fatal("expected invalid config to be rejected")
	}
}

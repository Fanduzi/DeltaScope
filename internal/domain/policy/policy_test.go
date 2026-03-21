// Package policy defines audit policy configuration in domain terms.
// input: rule policy values and validation scenarios
// output: policy behavior coverage for broader parameter shapes
// pos: domain policy test coverage
// note: if this file changes, update this header and module README.md.
package policy

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func TestRulePolicyParams(t *testing.T) {
	p := RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"threshold": 64,
			"enabled":   true,
			"labels":    []string{"a", "b"},
		},
	}

	if got := p.Params["threshold"]; got != 64 {
		t.Fatalf("expected threshold to be 64, got %v", got)
	}
	if got := p.Params["enabled"]; got != true {
		t.Fatalf("expected enabled to be true, got %v", got)
	}
}

func TestDefaultMetadataExistenceRulesAdvertiseMetadataRequirement(t *testing.T) {
	p := Default()

	for _, ruleID := range []string{
		"ddl.table.exists.create.forbid",
		"ddl.table.exists.alter.require",
		"ddl.alter.add_column.exists.forbid",
		"ddl.alter.drop_column.exists.require",
		"ddl.alter.modify_column.exists.require",
		"ddl.alter.change_column.exists.require",
		"ddl.alter.rename_column.exists.require",
		"ddl.alter.add_index.exists.forbid",
		"ddl.alter.drop_index.exists.require",
		"ddl.alter.rename_index.exists.require",
		"ddl.alter.drop_primary_key.exists.require",
	} {
		ruleCfg, ok := p.Rules[ruleID]
		if !ok {
			t.Fatalf("missing default rule config for %s", ruleID)
		}
		if _, hasLegacyRequired := ruleCfg.Params["required"]; hasLegacyRequired {
			t.Fatalf("expected %s to avoid legacy required param, got %#v", ruleID, ruleCfg.Params)
		}
		if got := ruleCfg.Params["requires_metadata"]; got != true {
			t.Fatalf("expected %s to advertise requires_metadata=true, got %#v", ruleID, ruleCfg.Params)
		}
	}
}

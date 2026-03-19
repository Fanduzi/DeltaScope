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

// Package policy defines audit policy configuration in domain terms.
package policy

import "github.com/Fanduzi/DeltaScope/internal/domain/rule"

// RulePolicy defines how one rule is configured.
type RulePolicy struct {
	Enabled bool           `json:"enabled" yaml:"enabled"`
	Level   rule.Level     `json:"level,omitempty" yaml:"level,omitempty"`
	Params  map[string]any `json:"params,omitempty" yaml:"params,omitempty"`
}

// AppPolicy stores top-level audit settings and rule policies.
type Policy struct {
	Rules map[string]RulePolicy `json:"rules,omitempty" yaml:"rules,omitempty"`
}

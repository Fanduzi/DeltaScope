// Package policy defines audit policy configuration in domain terms.
// input: rule identifiers, severity choices, and per-rule parameter values
// output: policy objects consumed by application services and rule evaluation
// pos: domain policy model for configuring audit behavior
// note: if this file changes, update this header and module README.md.
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

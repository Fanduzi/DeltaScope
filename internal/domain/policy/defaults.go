// Package policy defines audit policy configuration in domain terms.
// input: built-in rule identifiers and default severity/parameter choices
// output: baseline policy values used when no external config is supplied
// pos: domain default policy factory for v1 audit behavior
// note: if this file changes, update this header and module README.md.
package policy

// Default returns the built-in v1 policy baseline.
func Default() Policy {
	rules := make(map[string]RulePolicy, len(ddlRules())+len(dmlRules()))
	for k, v := range ddlRules() {
		rules[k] = v
	}
	for k, v := range dmlRules() {
		rules[k] = v
	}
	return Policy{
		Rules: rules,
	}
}

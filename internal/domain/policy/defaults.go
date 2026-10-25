// Package policy defines audit policy configuration in domain terms.
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

package configlint

import (
	"fmt"
	"sort"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

// deriveWarnings returns deterministic rule-level replacement warnings for the
// mentioned rules in raw. Warnings are ordered by rule_id ascending, then by
// field: enabled, level, params.<key> alphabetical. It reads policy.Default()
// for the comparison baseline and never mutates raw or the default policy.
//
// Unknown rules are skipped defensively; validateRaw already rejects them with
// an error before this runs.
func deriveWarnings(raw rawConfigFile) []Warning {
	defaults := policy.Default()
	var warnings []Warning
	for _, ruleID := range mapStringKeys(raw.Rules) {
		def, ok := defaults.Rules[ruleID]
		if !ok {
			continue
		}
		warnings = append(warnings, deriveRuleWarnings(ruleID, def, raw.Rules[ruleID])...)
	}
	return warnings
}

// deriveRuleWarnings emits the four v0.320.0 replacement-hazard cases for one
// mentioned rule, already in canonical field order: enabled, level, params.
func deriveRuleWarnings(ruleID string, def policy.RulePolicy, raw rawRuleConfig) []Warning {
	var warnings []Warning

	// Case 1: default enabled is true but the override omits enabled, so the
	// replaced policy zeroes it to false and the rule goes OFF.
	if raw.Enabled == nil && def.Enabled {
		warnings = append(warnings, Warning{
			RuleID: ruleID,
			Field:  "enabled",
			Message: fmt.Sprintf(
				`rule %q is mentioned without "enabled"; the rule policy is replaced, not partially merged, so omitted "enabled" becomes false and the rule is OFF`,
				ruleID,
			),
		})
	}

	// Case 2: the default level is non-empty but the override omits it (or sets
	// it to empty), so the replaced policy zeroes it to empty.
	levelSet := raw.Level != nil && *raw.Level != ""
	if !levelSet && def.Level != "" {
		warnings = append(warnings, Warning{
			RuleID: ruleID,
			Field:  "level",
			Message: fmt.Sprintf(
				`rule %q is mentioned without "level"; the rule policy is replaced, not partially merged, so omitted "level" becomes empty, replacing the default %q`,
				ruleID, def.Level,
			),
		})
	}

	// Cases 3 and 4 only apply when the default rule ships params.
	if len(def.Params) > 0 {
		if raw.Params == nil {
			// Case 3: the whole params map is omitted, so the replaced policy
			// drops every default param.
			warnings = append(warnings, Warning{
				RuleID: ruleID,
				Field:  "params",
				Message: fmt.Sprintf(
					`rule %q is mentioned without "params"; the rule policy is replaced, not partially merged, so omitted "params" become empty, removing the default params`,
					ruleID,
				),
			})
		} else {
			// Case 4: a present-but-incomplete params map drops each omitted
			// default param, reported one per key in alphabetical order.
			for _, key := range mapStringKeys(def.Params) {
				if _, present := raw.Params[key]; present {
					continue
				}
				warnings = append(warnings, Warning{
					RuleID: ruleID,
					Field:  "params." + key,
					Message: fmt.Sprintf(
						`rule %q omits default params; the rule policy is replaced, not partially merged, so omitted "params.%s" removes the default value`,
						ruleID, key,
					),
				})
			}
		}
	}

	return warnings
}

// mapStringKeys returns the keys of a string-keyed map in sorted order so all
// warning output is deterministic regardless of map iteration order.
func mapStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

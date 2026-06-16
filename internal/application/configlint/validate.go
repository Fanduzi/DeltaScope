package configlint

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

// validateRaw mirrors the semantic validation performed by the CLI `config lint`
// command and by internal/application/configstatus: it rejects unknown rules,
// invalid levels, unknown params, and param type mismatches so lint never
// silently accepts a malformed config. Rule IDs are visited in sorted order for
// deterministic error reporting. This deliberately stays in sync with both
// existing validators; if those rules diverge, update all three together.
func validateRaw(raw rawConfigFile) error {
	defaults := policy.Default()
	ruleIDs := make([]string, 0, len(raw.Rules))
	for ruleID := range raw.Rules {
		ruleIDs = append(ruleIDs, ruleID)
	}
	sort.Strings(ruleIDs)

	for _, ruleID := range ruleIDs {
		rawRule := raw.Rules[ruleID]
		defaultRule, ok := defaults.Rules[ruleID]
		if !ok {
			return fmt.Errorf("unknown rule %q", ruleID)
		}
		if rawRule.Level != nil && *rawRule.Level != "" {
			level := *rawRule.Level
			if level != string(rule.LevelBlocker) && level != string(rule.LevelWarning) && level != string(rule.LevelNotice) {
				return fmt.Errorf("invalid level %q for rule %q", level, ruleID)
			}
		}
		if err := validateRuleParams(ruleID, rawRule.Params, defaultRule.Params); err != nil {
			return err
		}
	}
	return nil
}

func validateRuleParams(ruleID string, rawParams map[string]any, defaultParams map[string]any) error {
	keys := make([]string, 0, len(rawParams))
	for key := range rawParams {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		rawValue := rawParams[key]
		defaultValue, ok := defaultParams[key]
		if !ok {
			return fmt.Errorf("unknown param %q for rule %q", key, ruleID)
		}
		if !paramTypeMatches(rawValue, defaultValue) {
			return fmt.Errorf("invalid type for %s.%s: got %T, want %T", ruleID, key, rawValue, defaultValue)
		}
	}
	return nil
}

// paramTypeMatches mirrors the CLI lint param type check exactly.
func paramTypeMatches(rawValue any, defaultValue any) bool {
	switch defaultValue.(type) {
	case []string:
		items, ok := rawValue.([]any)
		if !ok {
			if typed, ok := rawValue.([]string); ok {
				return len(typed) >= 0
			}
			return false
		}
		for _, item := range items {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	case int:
		switch rawValue.(type) {
		case int, int64, uint64:
			return true
		default:
			return false
		}
	case bool:
		_, ok := rawValue.(bool)
		return ok
	case string:
		_, ok := rawValue.(string)
		return ok
	default:
		return reflect.TypeOf(rawValue) == reflect.TypeOf(defaultValue)
	}
}

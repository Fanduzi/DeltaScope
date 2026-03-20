// Package ddl defines Tier-1 DDL rules.
// input: policy-backed rule configuration values
// output: normalized per-rule settings for DDL rule constructors
// pos: DDL rule configuration parsing helpers
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func configuredLevel(cfg policy.RulePolicy, fallback rule.Level) rule.Level {
	if cfg.Level == "" {
		return fallback
	}
	return cfg.Level
}

func boolParam(ruleID string, cfg policy.RulePolicy, key string, fallback bool) (bool, error) {
	value, ok := cfg.Params[key]
	if !ok {
		return fallback, nil
	}

	typed, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("rule %s param %q must be bool, got %T", ruleID, key, value)
	}
	return typed, nil
}

func intParam(ruleID string, cfg policy.RulePolicy, key string, fallback int) (int, error) {
	value, ok := cfg.Params[key]
	if !ok {
		return fallback, nil
	}

	switch typed := value.(type) {
	case int:
		return typed, nil
	case int8:
		return int(typed), nil
	case int16:
		return int(typed), nil
	case int32:
		return int(typed), nil
	case int64:
		return int(typed), nil
	case uint:
		return int(typed), nil
	case uint8:
		return int(typed), nil
	case uint16:
		return int(typed), nil
	case uint32:
		return int(typed), nil
	case uint64:
		return int(typed), nil
	default:
		return 0, fmt.Errorf("rule %s param %q must be integer, got %T", ruleID, key, value)
	}
}

func stringParam(ruleID string, cfg policy.RulePolicy, key string, fallback string) (string, error) {
	value, ok := cfg.Params[key]
	if !ok {
		return fallback, nil
	}

	typed, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("rule %s param %q must be string, got %T", ruleID, key, value)
	}
	return typed, nil
}

func stringSliceParam(ruleID string, cfg policy.RulePolicy, key string, fallback []string) ([]string, error) {
	value, ok := cfg.Params[key]
	if !ok {
		return append([]string(nil), fallback...), nil
	}

	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), nil
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("rule %s param %q must contain only strings, got %T", ruleID, key, item)
			}
			out = append(out, text)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("rule %s param %q must be a string list, got %T", ruleID, key, value)
	}
}

// Package dml defines Tier-1 DML rules.
// input: policy-backed rule configuration values
// output: normalized per-rule settings for DML rule constructors
// pos: DML rule configuration parsing helpers
// note: if this file changes, update this header and module README.md.
package dml

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

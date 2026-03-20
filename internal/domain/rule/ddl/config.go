// Package ddl defines Tier-1 DDL rules.
// input: policy-backed rule configuration values
// output: normalized per-rule settings for DDL rule constructors
// pos: DDL rule configuration parsing helpers
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"
	"strings"

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

func boundedIntParam(ruleID string, cfg policy.RulePolicy, key string, fallback, min int) (int, error) {
	value, err := intParam(ruleID, cfg, key, fallback)
	if err != nil {
		return 0, err
	}
	if value < min {
		return 0, fmt.Errorf("rule %s param %q must be >= %d, got %d", ruleID, key, min, value)
	}
	return value, nil
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

func normalizedStringSliceParam(ruleID string, cfg policy.RulePolicy, key string, fallback []string) ([]string, error) {
	items, err := stringSliceParam(ruleID, cfg, key, fallback)
	if err != nil {
		return nil, err
	}

	normalized := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func normalizedStringSetParam(ruleID string, cfg policy.RulePolicy, key string, fallback []string) (map[string]struct{}, error) {
	items, err := normalizedStringSliceParam(ruleID, cfg, key, fallback)
	if err != nil {
		return nil, err
	}

	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item] = struct{}{}
	}
	return set, nil
}

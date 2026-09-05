// Package configstatus derives the effective status of one shipped rule under the
// default policy and an optional YAML config file.
// input: a rule ID, an optional config path, built-in default policy, the rule catalog,
//
//	and the existing Viper-backed policy loader
//
// output: a stable Result describing whether the rule is ON or OFF, whether it is Loaded,
//
//	its effective level, default versus current policy snapshot, FK-forbid suppression,
//	and what the user's config changed
//
// pos: application use case for single-rule config status, below the CLI surface
// note: if this file changes, update this header and module README.md.
package configstatus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	viperconfig "github.com/Fanduzi/DeltaScope/internal/infrastructure/config/viper"
	"go.yaml.in/yaml/v3"
)

// Request selects one shipped rule and an optional config file.
type Request struct {
	RuleID     string
	ConfigPath string
}

// Result is the derived status of one rule. It never includes a "severity" field.
type Result struct {
	RuleID             string             `json:"rule_id"`
	Status             RuleStatus         `json:"status"`
	Default            RulePolicySnapshot `json:"default"`
	Current            RulePolicySnapshot `json:"current"`
	ConfigEffect       ConfigEffect       `json:"config_effect"`
	Suppression        *Suppression       `json:"suppression,omitempty"`
	RuleDetailsCommand string             `json:"rule_details_command"`
}

// RuleStatus reports the effective runtime state of the rule.
// Enabled/State follow Default Policy (or the caller's config). Loaded is the
// distinct registration fact: a rule can be enabled and still not Loaded.
type RuleStatus struct {
	Enabled bool       `json:"enabled"`
	Level   rule.Level `json:"level"`
	State   string     `json:"state"`
	Loaded  bool       `json:"loaded"`
}

// Suppression names why an enabled Default Policy rule is not Loaded.
type Suppression struct {
	Reason string `json:"reason"`
	By     string `json:"by"`
}

// RulePolicySnapshot captures the enabled flag, level, and params for one policy source.
// Params is always a non-nil cloned map.
type RulePolicySnapshot struct {
	Enabled bool           `json:"enabled"`
	Level   rule.Level     `json:"level"`
	Params  map[string]any `json:"params"`
}

// ConfigEffect summarizes how the supplied config affected this rule.
type ConfigEffect struct {
	HasConfig     bool     `json:"has_config"`
	HasOverride   bool     `json:"has_override"`
	ChangedFields []string `json:"changed_fields"`
	Messages      []string `json:"messages"`
}

// Inspect derives the effective status of one shipped rule under the default policy
// and an optional YAML config file. It does not run an audit, parse SQL, or touch a
// database. ctx is reserved for future cancellation and is currently unused.
func Inspect(ctx context.Context, req Request) (Result, error) {
	_ = ctx

	if strings.TrimSpace(req.RuleID) == "" {
		return Result{}, errors.New("rule id must not be empty")
	}

	entry, ok := catalog.Lookup(req.RuleID)
	if !ok {
		return Result{}, fmt.Errorf("rule %q not found", req.RuleID)
	}

	defaults := policy.Default()
	defaultRule := shippedRulePolicy(defaults, entry)

	currentRule := defaultRule
	currentPolicy := defaults
	effect := ConfigEffect{}

	if req.ConfigPath == "" {
		effect.HasConfig = false
		effect.HasOverride = false
		effect.ChangedFields = []string{}
		effect.Messages = []string{"No config supplied. This rule uses the default policy."}
	} else {
		effective, err := viperconfig.LoadPolicy(req.ConfigPath)
		if err != nil {
			return Result{}, fmt.Errorf("load policy config %q: %w", req.ConfigPath, err)
		}
		currentPolicy = effective
		loaded, ok := effective.Rules[req.RuleID]
		if !ok {
			// Catalog-only opt-in rules are absent from Default Policy, so LoadPolicy
			// only includes them when the caller mentions the rule.
			currentRule = defaultRule
		} else {
			currentRule = loaded
		}

		raw, err := readRawConfig(req.ConfigPath)
		if err != nil {
			return Result{}, fmt.Errorf("read policy config %q: %w", req.ConfigPath, err)
		}
		if err := validateRawConfig(raw); err != nil {
			return Result{}, err
		}

		effect.HasConfig = true
		rawRule, mentioned := raw.Rules[req.RuleID]
		effect.HasOverride = mentioned
		if !mentioned {
			// The rule is absent from the config file. LoadPolicy preserves the default
			// values for rules it never mentions, so there is no override to report.
			effect.ChangedFields = []string{}
			effect.Messages = []string{"No override found. This rule uses the default policy."}
		} else {
			// Mentioning a rule in YAML replaces its whole policy: viper/mapstructure
			// decodes a fresh RulePolicy for any mentioned rule, so omitted fields become
			// their zero values (enabled=false, level="", params=nil). This matches what
			// the audit path actually applies, so config status never hides that effect.
			changes := diffRulePolicy(defaultRule, currentRule, rawRule)
			effect.ChangedFields = changeFields(changes)
			effect.Messages = append(
				[]string{"Your config mentions this rule, so it replaces the default rule policy."},
				changeMessages(changes)...,
			)
			if len(changes) == 0 {
				effect.Messages = append(effect.Messages, "The effective values match the default policy.")
			}
		}
	}

	state := "off"
	if currentRule.Enabled {
		state = "on"
	}
	suppression := foreignKeyNamingSuppression(currentPolicy, req.RuleID)

	return Result{
		RuleID: req.RuleID,
		Status: RuleStatus{
			Enabled: currentRule.Enabled,
			Level:   currentRule.Level,
			State:   state,
			Loaded:  currentRule.Enabled && suppression == nil,
		},
		Default:            snapshotRulePolicy(defaultRule),
		Current:            snapshotRulePolicy(currentRule),
		ConfigEffect:       effect,
		Suppression:        suppression,
		RuleDetailsCommand: fmt.Sprintf("deltascope rules explain %s", req.RuleID),
	}, nil
}

func foreignKeyNamingSuppression(cfg policy.Policy, ruleID string) *Suppression {
	if !policy.SuppressesForeignKeyNaming(cfg, ruleID) {
		return nil
	}
	return &Suppression{
		Reason: policy.ForeignKeyNamingSuppressionReason,
		By:     policy.ForeignKeyForbidRuleID,
	}
}

// snapshotRulePolicy clones a RulePolicy into an immutable snapshot.
func snapshotRulePolicy(in policy.RulePolicy) RulePolicySnapshot {
	return RulePolicySnapshot{
		Enabled: in.Enabled,
		Level:   in.Level,
		Params:  cloneParams(in.Params),
	}
}

// cloneParams returns a deep-enough copy of a params map. Nested slices are copied so
// callers cannot mutate the source policy through the returned snapshot.
func cloneParams(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneParamValue(value)
	}
	return out
}

func cloneParamValue(value any) any {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		return append([]any(nil), typed...)
	default:
		return value
	}
}

// change describes one field that differs between default and current policy, plus the
// human-readable message for that difference. Changes are produced in a canonical order.
type change struct {
	field   string
	message string
}

// diffRulePolicy compares default and current policy for one mentioned rule and returns
// changes in canonical order: enabled, then level, then params.<key> alphabetically by
// key. raw distinguishes "omitted" (zeroed by the loader) from "explicitly set", which
// drives the warning wording for omitted fields.
func diffRulePolicy(def, cur policy.RulePolicy, raw rawRuleConfig) []change {
	var changes []change

	if raw.Enabled == nil {
		if def.Enabled != cur.Enabled {
			changes = append(changes, change{field: "enabled", message: "`enabled` is omitted, so the effective value is false."})
		}
	} else if def.Enabled != cur.Enabled {
		if cur.Enabled {
			changes = append(changes, change{field: "enabled", message: "`enabled` is set to true; your config enables this rule."})
		} else {
			changes = append(changes, change{field: "enabled", message: "`enabled` is set to false; your config disables this rule."})
		}
	}

	levelSet := raw.Level != nil && *raw.Level != ""
	if !levelSet {
		if def.Level != cur.Level {
			changes = append(changes, change{field: "level", message: "`level` is omitted, so the effective level is empty."})
		}
	} else if def.Level != cur.Level {
		changes = append(changes, change{field: "level", message: fmt.Sprintf("`level` changes from %s to %s.", def.Level, cur.Level)})
	}

	for _, key := range unionParamKeys(def.Params, cur.Params) {
		dv, dOk := def.Params[key]
		cv, cOk := cur.Params[key]
		switch {
		case dOk && !cOk:
			changes = append(changes, change{field: "params." + key, message: fmt.Sprintf("`params.%s` is removed.", key)})
		case !dOk && cOk:
			changes = append(changes, change{field: "params." + key, message: fmt.Sprintf("`params.%s` is set to %s.", key, formatParamValue(cv))})
		case !equalParamValue(dv, cv):
			changes = append(changes, change{field: "params." + key, message: fmt.Sprintf("`params.%s` changes from %s to %s.", key, formatParamValue(dv), formatParamValue(cv))})
		}
	}

	return changes
}

func changeFields(changes []change) []string {
	out := make([]string, 0, len(changes))
	for _, c := range changes {
		out = append(out, c.field)
	}
	return out
}

func changeMessages(changes []change) []string {
	out := make([]string, 0, len(changes))
	for _, c := range changes {
		out = append(out, c.message)
	}
	return out
}

func unionParamKeys(a, b map[string]any) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	keys := make([]string, 0, len(a)+len(b))
	for k := range a {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	for k := range b {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// equalParamValue compares two param values across YAML/Go type drift for scalars and
// string lists. It normalizes integer widths and []string versus []any string lists so
// equivalent values are not reported as changed.
func equalParamValue(a, b any) bool {
	if na, nb, ok := asInts(a, b); ok {
		return na == nb
	}
	if la, oka := toStringList(a); oka {
		if lb, okb := toStringList(b); okb {
			return reflect.DeepEqual(la, lb)
		}
	}
	return reflect.DeepEqual(a, b)
}

func asInts(a, b any) (int64, int64, bool) {
	na, oka := toInt64(a)
	nb, okb := toInt64(b)
	if !oka || !okb {
		return 0, 0, false
	}
	return na, nb, true
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case uint64:
		return int64(n), true
	case int32:
		return int64(n), true
	case uint32:
		return int64(n), true
	}
	return 0, false
}

// toStringList reports whether v is a list of strings, returning a normalized []string.
func toStringList(v any) ([]string, bool) {
	switch typed := v.(type) {
	case []string:
		return append([]string(nil), typed...), true
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	}
	return nil, false
}

func formatParamValue(v any) string {
	if list, ok := toStringList(v); ok {
		return "[" + strings.Join(list, ", ") + "]"
	}
	return fmt.Sprintf("%v", v)
}

// rawConfigFile mirrors the on-disk YAML closely enough to detect which keys a user wrote
// explicitly. Pointers distinguish "absent" from "empty/zero".
type rawConfigFile struct {
	Rules map[string]rawRuleConfig `yaml:"rules"`
}

type rawRuleConfig struct {
	Enabled *bool          `yaml:"enabled"`
	Level   *string        `yaml:"level"`
	Params  map[string]any `yaml:"params"`
}

func readRawConfig(path string) (rawConfigFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return rawConfigFile{}, err
	}
	var raw rawConfigFile
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return rawConfigFile{}, err
	}
	if raw.Rules == nil {
		raw.Rules = map[string]rawRuleConfig{}
	}
	return raw, nil
}

// validateRawConfig mirrors the semantic validation performed by the CLI `config lint`
// command. It rejects unknown rules, invalid levels, unknown params, and param type
// mismatches so `config status` never silently accepts a malformed config. This stays in
// sync with internal/interfaces/cli lintConfigFile intentionally; if those rules diverge,
// update both together.
func validateRawConfig(raw rawConfigFile) error {
	defaults := policy.Default()
	// Iterate rule IDs in sorted order for deterministic error reporting.
	ruleIDs := make([]string, 0, len(raw.Rules))
	for ruleID := range raw.Rules {
		ruleIDs = append(ruleIDs, ruleID)
	}
	sort.Strings(ruleIDs)

	for _, ruleID := range ruleIDs {
		rawRule := raw.Rules[ruleID]
		entry, ok := catalog.Lookup(ruleID)
		if !ok {
			return fmt.Errorf("unknown rule %q", ruleID)
		}
		defaultRule := shippedRulePolicy(defaults, entry)
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

func shippedRulePolicy(defaults policy.Policy, entry catalog.Entry) policy.RulePolicy {
	if ruleCfg, ok := defaults.Rules[entry.RuleID]; ok {
		return ruleCfg
	}
	return policy.RulePolicy{
		Enabled: entry.DefaultEnabled,
		Level:   entry.DefaultLevel,
		Params:  cloneParams(entry.DefaultParams),
	}
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

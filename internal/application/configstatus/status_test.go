// Package configstatus verifies rule config status derivation.
// input: temporary YAML config files, rule IDs, and built-in default policy metadata
// output: test coverage for the config status derivation layer
// pos: application config status adapter test coverage
// note: if this file changes, update this header and module README.md.
package configstatus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "deltascope.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// hasMessage reports whether any message line contains the given substring.
func hasMessage(res Result, substr string) bool {
	for _, m := range res.ConfigEffect.Messages {
		if strings.Contains(m, substr) {
			return true
		}
	}
	return false
}

func TestInspect_DefaultOnlyRule(t *testing.T) {
	res, err := Inspect(t.Context(), Request{RuleID: "dml.where.require"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if res.RuleID != "dml.where.require" {
		t.Fatalf("rule id = %q", res.RuleID)
	}
	if !res.Status.Enabled || res.Status.State != "on" {
		t.Fatalf("expected status ON, got enabled=%v state=%q", res.Status.Enabled, res.Status.State)
	}
	if res.Status.Level != rule.LevelBlocker {
		t.Fatalf("expected level blocker, got %q", res.Status.Level)
	}

	if !res.Default.Enabled || res.Default.Level != rule.LevelBlocker {
		t.Fatalf("unexpected default snapshot: %+v", res.Default)
	}
	if !res.Current.Enabled || res.Current.Level != rule.LevelBlocker {
		t.Fatalf("unexpected current snapshot: %+v", res.Current)
	}

	if res.ConfigEffect.HasConfig {
		t.Fatalf("expected has_config=false")
	}
	if res.ConfigEffect.HasOverride {
		t.Fatalf("expected has_override=false")
	}
	if len(res.ConfigEffect.ChangedFields) != 0 {
		t.Fatalf("expected no changed fields, got %v", res.ConfigEffect.ChangedFields)
	}
	if len(res.ConfigEffect.Messages) != 1 || !strings.Contains(res.ConfigEffect.Messages[0], "No config supplied") {
		t.Fatalf("expected one no-config-supplied message, got %v", res.ConfigEffect.Messages)
	}

	const want = "deltascope rules explain dml.where.require"
	if res.RuleDetailsCommand != want {
		t.Fatalf("rule details command = %q, want %q", res.RuleDetailsCommand, want)
	}
}

func TestInspect_ConfigPathWithoutMatchingRule(t *testing.T) {
	path := writeConfig(t, `
rules:
  ddl.table.name.max_length:
    enabled: true
    level: blocker
    params:
      limit: 64
`)

	res, err := Inspect(t.Context(), Request{RuleID: "dml.where.require", ConfigPath: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !res.ConfigEffect.HasConfig {
		t.Fatalf("expected has_config=true")
	}
	if res.ConfigEffect.HasOverride {
		t.Fatalf("expected has_override=false")
	}
	if len(res.ConfigEffect.ChangedFields) != 0 {
		t.Fatalf("expected no changed fields, got %v", res.ConfigEffect.ChangedFields)
	}
	if !hasMessage(res, "No override found") || !hasMessage(res, "default") {
		t.Fatalf("expected no-override message, got %v", res.ConfigEffect.Messages)
	}
	if res.Status.State != "on" || res.Status.Level != rule.LevelBlocker {
		t.Fatalf("expected default ON blocker, got state=%q level=%q", res.Status.State, res.Status.Level)
	}
}

// TestInspect_PartialLevelOverrideReplacesEntireRulePolicy documents the verbatim
// replacement semantics: writing only `level` mentions the rule, so the loader zeroes
// the omitted `enabled` (to false) and `params` (to empty). The rule ends up OFF even
// though the user only intended to change the level.
func TestInspect_PartialLevelOverrideReplacesEntireRulePolicy(t *testing.T) {
	path := writeConfig(t, `
rules:
  dml.where.require:
    level: warning
`)

	res, err := Inspect(t.Context(), Request{RuleID: "dml.where.require", ConfigPath: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if res.Status.Enabled || res.Status.State != "off" {
		t.Fatalf("expected status OFF because enabled is zeroed, got enabled=%v state=%q", res.Status.Enabled, res.Status.State)
	}
	if res.Status.Level != rule.LevelWarning {
		t.Fatalf("expected effective level warning, got %q", res.Status.Level)
	}
	if len(res.Current.Params) != 0 {
		t.Fatalf("expected current params emptied by replacement, got %#v", res.Current.Params)
	}

	if !res.ConfigEffect.HasOverride {
		t.Fatalf("expected has_override=true (rule is mentioned)")
	}
	wantFields := []string{"enabled", "level", "params.required"}
	if len(res.ConfigEffect.ChangedFields) != len(wantFields) {
		t.Fatalf("expected changed_fields=%v, got %v", wantFields, res.ConfigEffect.ChangedFields)
	}
	for i, want := range wantFields {
		if res.ConfigEffect.ChangedFields[i] != want {
			t.Fatalf("changed_fields[%d] = %q, want %q", i, res.ConfigEffect.ChangedFields[i], want)
		}
	}

	if len(res.ConfigEffect.Messages) == 0 || res.ConfigEffect.Messages[0] != "Your config mentions this rule, so it replaces the default rule policy." {
		t.Fatalf("expected framing message first, got %v", res.ConfigEffect.Messages)
	}
	if !hasMessage(res, "`enabled` is omitted, so the effective value is false.") {
		t.Fatalf("expected omitted-enabled warning, got %v", res.ConfigEffect.Messages)
	}
	if !hasMessage(res, "`level` changes from blocker to warning.") {
		t.Fatalf("expected level change message, got %v", res.ConfigEffect.Messages)
	}
	if !hasMessage(res, "`params.required` is removed.") {
		t.Fatalf("expected params.required removed message, got %v", res.ConfigEffect.Messages)
	}

	// Defaults must remain untouched.
	if !res.Default.Enabled || res.Default.Level != rule.LevelBlocker {
		t.Fatalf("default snapshot changed unexpectedly: %+v", res.Default)
	}
}

// TestInspect_ExplicitDisableWithFullSpec shows the intended way to disable a rule:
// specify every field so the replacement leaves level and params intact.
func TestInspect_ExplicitDisableWithFullSpec(t *testing.T) {
	path := writeConfig(t, `
rules:
  ddl.table.audit_columns.require:
    enabled: false
    level: warning
    params:
      required: true
`)

	res, err := Inspect(t.Context(), Request{RuleID: "ddl.table.audit_columns.require", ConfigPath: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if res.Status.Enabled || res.Status.State != "off" {
		t.Fatalf("expected status OFF, got enabled=%v state=%q", res.Status.Enabled, res.Status.State)
	}
	if res.Status.Level != rule.LevelWarning {
		t.Fatalf("expected effective level warning, got %q", res.Status.Level)
	}
	if len(res.ConfigEffect.ChangedFields) != 1 || res.ConfigEffect.ChangedFields[0] != "enabled" {
		t.Fatalf("expected changed_fields=[enabled], got %v", res.ConfigEffect.ChangedFields)
	}
	if !hasMessage(res, "`enabled` is set to false") || !hasMessage(res, "disables this rule") {
		t.Fatalf("expected explicit-disable message, got %v", res.ConfigEffect.Messages)
	}
	if !res.Default.Enabled {
		t.Fatalf("default enabled should stay true")
	}
}

// TestInspect_PartialParamOverrideReplacesEntireRulePolicy mirrors the level case for
// params: writing only `params` zeroes `enabled` (to false) and `level` (to empty).
func TestInspect_PartialParamOverrideReplacesEntireRulePolicy(t *testing.T) {
	path := writeConfig(t, `
rules:
  ddl.table.name.max_length:
    params:
      limit: 48
`)

	res, err := Inspect(t.Context(), Request{RuleID: "ddl.table.name.max_length", ConfigPath: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if res.Status.Enabled || res.Status.State != "off" {
		t.Fatalf("expected status OFF because enabled is zeroed, got enabled=%v state=%q", res.Status.Enabled, res.Status.State)
	}
	if res.Status.Level != "" {
		t.Fatalf("expected effective level emptied by replacement, got %q", res.Status.Level)
	}
	if cur, _ := res.Current.Params["limit"].(int); cur != 48 {
		t.Fatalf("expected current limit 48, got %#v", res.Current.Params["limit"])
	}

	wantFields := []string{"enabled", "level", "params.limit"}
	if len(res.ConfigEffect.ChangedFields) != len(wantFields) {
		t.Fatalf("expected changed_fields=%v, got %v", wantFields, res.ConfigEffect.ChangedFields)
	}
	for i, want := range wantFields {
		if res.ConfigEffect.ChangedFields[i] != want {
			t.Fatalf("changed_fields[%d] = %q, want %q", i, res.ConfigEffect.ChangedFields[i], want)
		}
	}
	if !hasMessage(res, "`enabled` is omitted, so the effective value is false.") {
		t.Fatalf("expected omitted-enabled warning, got %v", res.ConfigEffect.Messages)
	}
	if !hasMessage(res, "`level` is omitted, so the effective level is empty.") {
		t.Fatalf("expected omitted-level warning, got %v", res.ConfigEffect.Messages)
	}
	if !hasMessage(res, "`params.limit` changes from 64 to 48.") {
		t.Fatalf("expected param change message, got %v", res.ConfigEffect.Messages)
	}
}

func TestInspect_MultipleOverridesDeterministic(t *testing.T) {
	path := writeConfig(t, `
rules:
  ddl.table.name.max_length:
    enabled: false
    level: notice
    params:
      limit: 48
`)

	first, err := Inspect(t.Context(), Request{RuleID: "ddl.table.name.max_length", ConfigPath: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wantFields := []string{"enabled", "level", "params.limit"}
	if len(first.ConfigEffect.ChangedFields) != len(wantFields) {
		t.Fatalf("expected %d changed fields, got %v", len(wantFields), first.ConfigEffect.ChangedFields)
	}
	for i, want := range wantFields {
		if first.ConfigEffect.ChangedFields[i] != want {
			t.Fatalf("changed_fields[%d] = %q, want %q", i, first.ConfigEffect.ChangedFields[i], want)
		}
	}

	// Message order must mirror changed_fields order, framed by the replacement notice.
	if len(first.ConfigEffect.Messages) != len(wantFields)+1 {
		t.Fatalf("expected framing + %d field messages, got %v", len(wantFields), first.ConfigEffect.Messages)
	}
	if first.ConfigEffect.Messages[0] != "Your config mentions this rule, so it replaces the default rule policy." {
		t.Fatalf("unexpected framing message: %q", first.ConfigEffect.Messages[0])
	}
	wantSubstrings := []string{
		"`enabled` is set to false",
		"`level` changes from blocker to notice",
		"`params.limit` changes from 64 to 48",
	}
	for i, want := range wantSubstrings {
		if !strings.Contains(first.ConfigEffect.Messages[i+1], want) {
			t.Fatalf("messages[%d] = %q, want substring %q", i+1, first.ConfigEffect.Messages[i+1], want)
		}
	}

	for range 3 {
		again, err := Inspect(t.Context(), Request{RuleID: "ddl.table.name.max_length", ConfigPath: path})
		if err != nil {
			t.Fatalf("repeat call errored: %v", err)
		}
		if !equalResult(first, again) {
			t.Fatalf("repeat call produced a different result")
		}
	}
}

// TestInspect_MentionedRuleThatMatchesDefault confirms that mentioning a rule still
// counts as an override (its policy is replaced), even when the values happen to match.
func TestInspect_MentionedRuleThatMatchesDefault(t *testing.T) {
	path := writeConfig(t, `
rules:
  dml.where.require:
    enabled: true
    level: blocker
    params:
      required: true
`)

	res, err := Inspect(t.Context(), Request{RuleID: "dml.where.require", ConfigPath: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !res.ConfigEffect.HasConfig || !res.ConfigEffect.HasOverride {
		t.Fatalf("expected has_config=true and has_override=true for a mentioned rule")
	}
	if len(res.ConfigEffect.ChangedFields) != 0 {
		t.Fatalf("expected no changed fields when values match, got %v", res.ConfigEffect.ChangedFields)
	}
	if !hasMessage(res, "replaces the default rule policy") {
		t.Fatalf("expected replacement framing, got %v", res.ConfigEffect.Messages)
	}
	if !hasMessage(res, "match the default policy") {
		t.Fatalf("expected match-default note, got %v", res.ConfigEffect.Messages)
	}
	if res.Status.State != "on" || res.Status.Level != rule.LevelBlocker {
		t.Fatalf("expected ON blocker, got state=%q level=%q", res.Status.State, res.Status.Level)
	}
}

func TestInspect_UnknownRule(t *testing.T) {
	_, err := Inspect(t.Context(), Request{RuleID: "not.real.rule"})
	if err == nil {
		t.Fatalf("expected error for unknown rule")
	}
	if !strings.Contains(err.Error(), "not.real.rule") {
		t.Fatalf("expected error to name rule id, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %q", err.Error())
	}
}

func TestInspect_EmptyRuleID(t *testing.T) {
	_, err := Inspect(t.Context(), Request{RuleID: ""})
	if err == nil {
		t.Fatalf("expected error for empty rule id")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "empty") {
		t.Fatalf("expected empty-id error, got %q", err.Error())
	}
}

func TestInspect_InvalidConfigPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	_, err := Inspect(t.Context(), Request{RuleID: "dml.where.require", ConfigPath: missing})
	if err == nil {
		t.Fatalf("expected error for missing config file")
	}
}

func TestInspect_InvalidConfig(t *testing.T) {
	t.Run("unknown rule in config", func(t *testing.T) {
		path := writeConfig(t, `
rules:
  totally.unknown.rule:
    enabled: true
`)
		_, err := Inspect(t.Context(), Request{RuleID: "dml.where.require", ConfigPath: path})
		if err == nil {
			t.Fatalf("expected error for config with unknown rule")
		}
	})

	t.Run("invalid level", func(t *testing.T) {
		path := writeConfig(t, `
rules:
  dml.where.require:
    level: bogus
`)
		_, err := Inspect(t.Context(), Request{RuleID: "dml.where.require", ConfigPath: path})
		if err == nil {
			t.Fatalf("expected error for invalid level")
		}
	})

	t.Run("invalid param type", func(t *testing.T) {
		path := writeConfig(t, `
rules:
  ddl.table.name.max_length:
    params:
      limit: not-a-number
`)
		_, err := Inspect(t.Context(), Request{RuleID: "ddl.table.name.max_length", ConfigPath: path})
		if err == nil {
			t.Fatalf("expected error for invalid param type")
		}
	})

	t.Run("unknown param", func(t *testing.T) {
		path := writeConfig(t, `
rules:
  dml.where.require:
    params:
      mystery: true
`)
		_, err := Inspect(t.Context(), Request{RuleID: "dml.where.require", ConfigPath: path})
		if err == nil {
			t.Fatalf("expected error for unknown param")
		}
	})
}

func TestInspect_ClonesParams(t *testing.T) {
	path := writeConfig(t, `
rules:
  ddl.table.name.max_length:
    enabled: true
    level: blocker
    params:
      limit: 48
`)

	first, err := Inspect(t.Context(), Request{RuleID: "ddl.table.name.max_length", ConfigPath: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Mutate the returned params maps. Subsequent calls must be unaffected.
	first.Current.Params["limit"] = 9999
	first.Default.Params["limit"] = 9999
	delete(first.Current.Params, "limit")
	delete(first.Default.Params, "limit")

	second, err := Inspect(t.Context(), Request{RuleID: "ddl.table.name.max_length", ConfigPath: path})
	if err != nil {
		t.Fatalf("expected no error on repeat call, got %v", err)
	}
	if def, _ := second.Default.Params["limit"].(int); def != 64 {
		t.Fatalf("default params leaked mutation: limit = %#v", second.Default.Params["limit"])
	}
	if cur, _ := second.Current.Params["limit"].(int); cur != 48 {
		t.Fatalf("current params leaked mutation: limit = %#v", second.Current.Params["limit"])
	}

	// policy.Default() must also remain unaffected by the mutation above.
	res, err := Inspect(t.Context(), Request{RuleID: "ddl.table.name.max_length"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if def, _ := res.Default.Params["limit"].(int); def != 64 {
		t.Fatalf("policy.Default() leaked mutation: limit = %#v", res.Default.Params["limit"])
	}
}

func TestInspect_JSONHasNoSeverity(t *testing.T) {
	res, err := Inspect(t.Context(), Request{RuleID: "dml.where.require"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(data), "severity") {
		t.Fatalf("result JSON must not include severity: %s", data)
	}

	// Spot-check the wrapper fields round-trip and use level, not severity.
	var round Result
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if round.RuleID != "dml.where.require" {
		t.Fatalf("round-trip rule id = %q", round.RuleID)
	}
	if round.Status.Level != rule.LevelBlocker {
		t.Fatalf("round-trip level = %q", round.Status.Level)
	}
}

// equalResult compares two results for the determinism check.
func equalResult(a, b Result) bool {
	if a.RuleID != b.RuleID || a.Status != b.Status {
		return false
	}
	if !sameSnapshot(a.Default, b.Default) || !sameSnapshot(a.Current, b.Current) {
		return false
	}
	if a.ConfigEffect.HasConfig != b.ConfigEffect.HasConfig ||
		a.ConfigEffect.HasOverride != b.ConfigEffect.HasOverride {
		return false
	}
	if len(a.ConfigEffect.ChangedFields) != len(b.ConfigEffect.ChangedFields) {
		return false
	}
	for i := range a.ConfigEffect.ChangedFields {
		if a.ConfigEffect.ChangedFields[i] != b.ConfigEffect.ChangedFields[i] {
			return false
		}
	}
	if len(a.ConfigEffect.Messages) != len(b.ConfigEffect.Messages) {
		return false
	}
	for i := range a.ConfigEffect.Messages {
		if a.ConfigEffect.Messages[i] != b.ConfigEffect.Messages[i] {
			return false
		}
	}
	return a.RuleDetailsCommand == b.RuleDetailsCommand
}

func sameSnapshot(a, b RulePolicySnapshot) bool {
	if a.Enabled != b.Enabled || a.Level != b.Level {
		return false
	}
	if len(a.Params) != len(b.Params) {
		return false
	}
	for k, av := range a.Params {
		bv, ok := b.Params[k]
		if !ok || !equalParamValue(av, bv) {
			return false
		}
	}
	return true
}

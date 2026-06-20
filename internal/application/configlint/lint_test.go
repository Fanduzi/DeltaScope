// Package configlint verifies config lint replacement warning derivation.
// input: temporary YAML config files and built-in default policy metadata
// output: test coverage for the config lint warning core
// pos: application config lint adapter test coverage
// note: if this file changes, update this header and module README.md.
package configlint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

// writeConfig writes body to a temp YAML file and returns its path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "deltascope.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestInspect_EmptyPathReturnsError(t *testing.T) {
	_, err := Inspect(t.Context(), Request{Path: ""})
	if err == nil {
		t.Fatalf("expected error for empty path")
	}
}

func TestInspect_MissingFileReturnsError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	_, err := Inspect(t.Context(), Request{Path: missing})
	if err == nil {
		t.Fatalf("expected error for missing config file")
	}
	if !strings.Contains(err.Error(), "read config file") {
		t.Fatalf("expected read-config error, got %q", err.Error())
	}
}

func TestInspect_MalformedYAMLReturnsError(t *testing.T) {
	path := writeConfig(t, "rules: [this is not, valid yaml\n  - oops: [unclosed")
	_, err := Inspect(t.Context(), Request{Path: path})
	if err == nil {
		t.Fatalf("expected error for malformed YAML")
	}
	if !strings.Contains(err.Error(), "parse yaml") {
		t.Fatalf("expected parse-yaml error, got %q", err.Error())
	}
}

func TestInspect_ValidationErrors(t *testing.T) {
	t.Run("unknown rule is an error not a warning", func(t *testing.T) {
		path := writeConfig(t, `
rules:
  totally.unknown.rule:
    enabled: true
`)
		res, err := Inspect(t.Context(), Request{Path: path})
		if err == nil {
			t.Fatalf("expected error for unknown rule")
		}
		if !strings.Contains(err.Error(), "unknown rule") {
			t.Fatalf("expected unknown-rule error, got %q", err.Error())
		}
		if len(res.Warnings) != 0 {
			t.Fatalf("expected no warnings on validation error, got %v", res.Warnings)
		}
	})

	t.Run("invalid level is an error", func(t *testing.T) {
		path := writeConfig(t, `
rules:
  dml.where.require:
    level: bogus
`)
		_, err := Inspect(t.Context(), Request{Path: path})
		if err == nil {
			t.Fatalf("expected error for invalid level")
		}
		if !strings.Contains(err.Error(), "invalid level") {
			t.Fatalf("expected invalid-level error, got %q", err.Error())
		}
	})

	t.Run("unknown param is an error", func(t *testing.T) {
		path := writeConfig(t, `
rules:
  dml.where.require:
    enabled: true
    level: blocker
    params:
      mystery: true
`)
		_, err := Inspect(t.Context(), Request{Path: path})
		if err == nil {
			t.Fatalf("expected error for unknown param")
		}
		if !strings.Contains(err.Error(), "unknown param") {
			t.Fatalf("expected unknown-param error, got %q", err.Error())
		}
	})

	t.Run("param type mismatch is an error", func(t *testing.T) {
		path := writeConfig(t, `
rules:
  ddl.table.name.max_length:
    enabled: true
    level: blocker
    params:
      limit: not-a-number
`)
		_, err := Inspect(t.Context(), Request{Path: path})
		if err == nil {
			t.Fatalf("expected error for param type mismatch")
		}
		if !strings.Contains(err.Error(), "invalid type") {
			t.Fatalf("expected invalid-type error, got %q", err.Error())
		}
	})
}

// TestInspect_FullExplicitPolicyHasNoWarnings confirms that mentioning a rule
// with every default field present produces no warnings, even though the policy
// is replaced — the effective values match the defaults, so no hazard exists.
func TestInspect_FullExplicitPolicyHasNoWarnings(t *testing.T) {
	path := writeConfig(t, `
rules:
  dml.where.require:
    enabled: true
    level: blocker
    params:
      required: true
`)
	res, err := Inspect(t.Context(), Request{Path: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("expected no warnings for full explicit policy, got %v", res.Warnings)
	}
}

// assertWarning checks the stable substrings every lint warning must carry:
// both halves of the replacement framing, the omitted field name, and an
// effective consequence keyword supplied by the caller.
func assertWarning(t *testing.T, w Warning, ruleID, field, consequence string) {
	t.Helper()
	if w.RuleID != ruleID {
		t.Errorf("rule id = %q, want %q", w.RuleID, ruleID)
	}
	if w.Field != field {
		t.Errorf("field = %q, want %q", w.Field, field)
	}
	if !strings.Contains(w.Message, "replaces the whole rule policy") {
		t.Errorf("message %q missing 'replaces the whole rule policy'", w.Message)
	}
	if !strings.Contains(w.Message, "does not merge with defaults") {
		t.Errorf("message %q missing 'does not merge with defaults'", w.Message)
	}
	if !strings.Contains(w.Message, field) {
		t.Errorf("message %q missing field name %q", w.Message, field)
	}
	if consequence != "" && !strings.Contains(w.Message, consequence) {
		t.Errorf("message %q missing consequence %q", w.Message, consequence)
	}
}

// TestInspect_LevelOnlyOverrideWarnsEnabledAndParams is the canonical footgun:
// mentioning dml.where.require with only level:warning replaces the whole
// policy, so omitted enabled turns the rule OFF and omitted params drop the
// default {required: true}. Level itself is supplied, so no level warning.
func TestInspect_LevelOnlyOverrideWarnsEnabledAndParams(t *testing.T) {
	path := writeConfig(t, `
rules:
  dml.where.require:
    level: warning
`)
	res, err := Inspect(t.Context(), Request{Path: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res.Warnings) != 2 {
		t.Fatalf("expected 2 warnings (enabled, params), got %d: %v", len(res.Warnings), res.Warnings)
	}
	assertWarning(t, res.Warnings[0], "dml.where.require", "enabled", "is OFF")
	assertWarning(t, res.Warnings[1], "dml.where.require", "params", "removes default params")
}

// TestInspect_OmittedLevelWarns covers case 2: enabled and params are present,
// only level is omitted, and the default level is non-empty (blocker).
func TestInspect_OmittedLevelWarns(t *testing.T) {
	path := writeConfig(t, `
rules:
  dml.where.require:
    enabled: true
    params:
      required: true
`)
	res, err := Inspect(t.Context(), Request{Path: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("expected 1 warning (level), got %d: %v", len(res.Warnings), res.Warnings)
	}
	assertWarning(t, res.Warnings[0], "dml.where.require", "level", "no effective level")
	if !strings.Contains(res.Warnings[0].Message, `"level" is omitted`) {
		t.Fatalf("expected level warning to state the omitted field, got %q", res.Warnings[0].Message)
	}
}

// TestInspect_OmittedParamsWarns covers case 3: enabled and level are present,
// the whole params map is omitted, and the default rule has params.
func TestInspect_OmittedParamsWarns(t *testing.T) {
	path := writeConfig(t, `
rules:
  dml.where.require:
    enabled: true
    level: blocker
`)
	res, err := Inspect(t.Context(), Request{Path: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("expected 1 warning (params), got %d: %v", len(res.Warnings), res.Warnings)
	}
	assertWarning(t, res.Warnings[0], "dml.where.require", "params", "removes default params")
}

// TestInspect_PartialParamsWarnsOmittedKeys covers case 4: pattern.require has
// default params {required, pattern}; supplying only required omits pattern.
// enabled and level are also omitted, so the full ordered set is enabled, level,
// params.pattern.
func TestInspect_PartialParamsWarnsOmittedKeys(t *testing.T) {
	path := writeConfig(t, `
rules:
  ddl.table.name.pattern.require:
    params:
      required: false
`)
	res, err := Inspect(t.Context(), Request{Path: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	wantFields := []string{"enabled", "level", "params.pattern"}
	if len(res.Warnings) != len(wantFields) {
		t.Fatalf("expected %d warnings, got %d: %v", len(wantFields), len(res.Warnings), res.Warnings)
	}
	for i, field := range wantFields {
		if res.Warnings[i].Field != field {
			t.Fatalf("warnings[%d].Field = %q, want %q (order: %v)", i, res.Warnings[i].Field, field, wantFields)
		}
		if res.Warnings[i].RuleID != "ddl.table.name.pattern.require" {
			t.Fatalf("warnings[%d].RuleID = %q", i, res.Warnings[i].RuleID)
		}
	}
	assertWarning(t, res.Warnings[2], "ddl.table.name.pattern.require", "params.pattern", "removes default")
}

// TestInspect_WarningCopyLocksReplacementHazardPhrasing locks the human-readable wording
// for all four replacement-hazard cases: each warning names the omitted field with an
// "is omitted" token, states the effective consequence, and carries both the "replaces the
// whole rule policy" and "does not merge with defaults" framing. No warning mentions severity.
func TestInspect_WarningCopyLocksReplacementHazardPhrasing(t *testing.T) {
	t.Run("omitted enabled", func(t *testing.T) {
		path := writeConfig(t, `
rules:
  dml.where.require:
    level: blocker
    params:
      required: true
`)
		res, err := Inspect(t.Context(), Request{Path: path})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(res.Warnings) != 1 || res.Warnings[0].Field != "enabled" {
			t.Fatalf("expected one enabled warning, got %v", res.Warnings)
		}
		msg := res.Warnings[0].Message
		for _, want := range []string{
			"dml.where.require",
			"is OFF",
			`"enabled" is omitted`,
			"replaces the whole rule policy",
			"does not merge with defaults",
		} {
			if !strings.Contains(msg, want) {
				t.Errorf("enabled warning %q missing %q", msg, want)
			}
		}
	})

	t.Run("omitted level", func(t *testing.T) {
		path := writeConfig(t, `
rules:
  dml.where.require:
    enabled: true
    params:
      required: true
`)
		res, err := Inspect(t.Context(), Request{Path: path})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(res.Warnings) != 1 || res.Warnings[0].Field != "level" {
			t.Fatalf("expected one level warning, got %v", res.Warnings)
		}
		msg := res.Warnings[0].Message
		for _, want := range []string{
			"no effective level",
			`"level" is omitted`,
			"replaces the whole rule policy",
			"does not merge with defaults",
		} {
			if !strings.Contains(msg, want) {
				t.Errorf("level warning %q missing %q", msg, want)
			}
		}
	})

	t.Run("omitted whole params", func(t *testing.T) {
		path := writeConfig(t, `
rules:
  dml.where.require:
    enabled: true
    level: blocker
`)
		res, err := Inspect(t.Context(), Request{Path: path})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(res.Warnings) != 1 || res.Warnings[0].Field != "params" {
			t.Fatalf("expected one params warning, got %v", res.Warnings)
		}
		msg := res.Warnings[0].Message
		for _, want := range []string{
			"removes default params",
			`"params" is omitted`,
			"replaces the whole rule policy",
			"does not merge with defaults",
		} {
			if !strings.Contains(msg, want) {
				t.Errorf("params warning %q missing %q", msg, want)
			}
		}
	})

	t.Run("omitted params key", func(t *testing.T) {
		// ddl.table.name.pattern.require defaults to params {required, pattern}; supplying
		// only required omits pattern. enabled and level are supplied, so the only warning is
		// params.pattern.
		path := writeConfig(t, `
rules:
  ddl.table.name.pattern.require:
    enabled: true
    level: blocker
    params:
      required: false
`)
		res, err := Inspect(t.Context(), Request{Path: path})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		var msg string
		for _, w := range res.Warnings {
			if w.Field == "params.pattern" {
				msg = w.Message
			}
		}
		if msg == "" {
			t.Fatalf("expected a params.pattern warning, got %v", res.Warnings)
		}
		for _, want := range []string{
			"removes default",
			`"params.pattern"`,
			"replaces the whole rule policy",
			"does not merge with defaults",
		} {
			if !strings.Contains(msg, want) {
				t.Errorf("params.pattern warning %q missing %q", msg, want)
			}
		}
		if strings.Contains(msg, "severity") {
			t.Errorf("warning must not introduce severity: %q", msg)
		}
	})
}

// TestInspect_WarningsOrderedByRuleIDThenField verifies determinism across
// multiple mentioned rules: rule_id ascending, then enabled, level,
// params.<key> alphabetical within each rule.
func TestInspect_WarningsOrderedByRuleIDThenField(t *testing.T) {
	path := writeConfig(t, `
rules:
  dml.where.require:
    level: warning
  ddl.table.name.max_length:
    level: notice
`)
	res, err := Inspect(t.Context(), Request{Path: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	wantFields := []string{
		"ddl.table.name.max_length/enabled",
		"ddl.table.name.max_length/params",
		"dml.where.require/enabled",
		"dml.where.require/params",
	}
	if len(res.Warnings) != len(wantFields) {
		t.Fatalf("expected %d warnings, got %d: %v", len(wantFields), len(res.Warnings), res.Warnings)
	}
	for i, want := range wantFields {
		got := res.Warnings[i].RuleID + "/" + res.Warnings[i].Field
		if got != want {
			t.Fatalf("warnings[%d] = %q, want %q", i, got, want)
		}
	}
}

// TestInspect_DeterministicOverRepeatedRuns confirms repeated calls on the same
// file produce byte-identical warning slices, including messages.
func TestInspect_DeterministicOverRepeatedRuns(t *testing.T) {
	path := writeConfig(t, `
rules:
  ddl.table.name.pattern.require:
    enabled: true
    level: blocker
    params:
      required: false
  dml.where.require:
    level: warning
`)
	first, err := Inspect(t.Context(), Request{Path: path})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(first.Warnings) == 0 {
		t.Fatalf("expected warnings to exercise determinism")
	}
	for range 4 {
		again, err := Inspect(t.Context(), Request{Path: path})
		if err != nil {
			t.Fatalf("repeat call errored: %v", err)
		}
		if !warningsEqual(first.Warnings, again.Warnings) {
			t.Fatalf("repeat call produced different warnings:\nfirst:  %v\nagain:  %v", first.Warnings, again.Warnings)
		}
	}
}

// TestInspect_ResultJSONHasNoSeverity locks the public contract: Result and
// Warning marshal with no "severity" field; level remains the priority field.
func TestInspect_ResultJSONHasNoSeverity(t *testing.T) {
	path := writeConfig(t, `
rules:
  dml.where.require:
    level: warning
`)
	res, err := Inspect(t.Context(), Request{Path: path})
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
	// Spot-check the stable field names round-trip.
	var round Result
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(round.Warnings) != len(res.Warnings) {
		t.Fatalf("round-trip warning count = %d, want %d", len(round.Warnings), len(res.Warnings))
	}
	if round.Warnings[0].Field != "enabled" {
		t.Fatalf("round-trip first field = %q", round.Warnings[0].Field)
	}
}

// TestInspect_DoesNotMutateDefaults confirms Inspect leaves policy.Default()
// untouched: it only reads the defaults and never writes to default or parsed
// maps.
func TestInspect_DoesNotMutateDefaults(t *testing.T) {
	beforeLimit := policy.Default().Rules["ddl.table.name.max_length"].Params["limit"]

	path := writeConfig(t, `
rules:
  ddl.table.name.max_length:
    enabled: true
    level: blocker
    params:
      limit: 48
`)
	if _, err := Inspect(t.Context(), Request{Path: path}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	afterLimit := policy.Default().Rules["ddl.table.name.max_length"].Params["limit"]
	if afterLimit != beforeLimit {
		t.Fatalf("policy.Default() mutated by Inspect: before=%v after=%v", beforeLimit, afterLimit)
	}
	if afterLimit != 64 {
		t.Fatalf("default limit changed to %v, want 64", afterLimit)
	}
}

func warningsEqual(a, b []Warning) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].RuleID != b[i].RuleID || a[i].Field != b[i].Field || a[i].Message != b[i].Message {
			return false
		}
	}
	return true
}

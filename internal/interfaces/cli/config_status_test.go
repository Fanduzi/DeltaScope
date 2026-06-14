// Package cli verifies the config status command.
// input: command-line args, temporary YAML config files, and the config status application service
// output: end-to-end CLI behavior coverage for config status exit codes, text rendering, and JSON shape
// pos: interface-layer CLI test coverage for config status
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/configstatus"
)

func writeConfigStatusYAML(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestConfigStatus_DefaultRuleText(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"config", "status", "dml.where.require"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	output := stdout.String()
	for _, expected := range []string{
		"Rule: dml.where.require",
		"Current status:",
		"ON",
		"Findings from this rule fail as: blocker.",
		"No config supplied. This rule uses the default policy.",
		"Default:",
		"Current:",
		"deltascope rules explain dml.where.require",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "severity") {
		t.Fatalf("text output must not mention severity:\n%s", output)
	}
}

func TestConfigStatus_FullSpecLevelOverrideText(t *testing.T) {
	path := writeConfigStatusYAML(t, "rules:\n  dml.where.require:\n    enabled: true\n    level: warning\n    params:\n      required: true\n")

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"--config", path, "config", "status", "dml.where.require"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	output := stdout.String()
	for _, expected := range []string{
		"ON",
		"Findings from this rule fail as: warning.",
		// The core message is "`level` changes from blocker to warning."; assert it verbatim so the
		// CLI stays faithful to what configstatus.Inspect emits.
		"`level` changes from blocker to warning.",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "severity") {
		t.Fatalf("text output must not mention severity:\n%s", output)
	}
}

// TestConfigStatus_PartialOverrideReplacementText locks the audit-faithful replacement semantics:
// writing only `level` mentions the rule, so the loader zeroes the omitted `enabled` to false and
// the rule ends up OFF even though the user only intended to change the level.
func TestConfigStatus_PartialOverrideReplacementText(t *testing.T) {
	path := writeConfigStatusYAML(t, "rules:\n  dml.where.require:\n    level: warning\n")

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"--config", path, "config", "status", "dml.where.require"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	output := stdout.String()
	for _, expected := range []string{
		"OFF",
		"replaces the default rule policy",
		"`enabled` is omitted",
		"This rule is OFF.",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestConfigStatus_DisabledRuleText(t *testing.T) {
	path := writeConfigStatusYAML(t, "rules:\n  ddl.table.audit_columns.require:\n    enabled: false\n    level: warning\n    params:\n      required: true\n")

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"--config", path, "config", "status", "ddl.table.audit_columns.require"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	output := stdout.String()
	for _, expected := range []string{
		"OFF",
		"This rule will not produce findings.",
		"disables this rule",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestConfigStatus_JSON(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"config", "status", "dml.where.require", "--format", "json"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	raw := stdout.String()
	if !strings.Contains(raw, `"version"`) {
		t.Fatalf("JSON output must include a top-level version field: %s", raw)
	}
	if strings.Contains(raw, "severity") {
		t.Fatalf("JSON output must not include severity: %s", raw)
	}

	var result configstatus.Result
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal config status JSON: %v\nraw: %s", err, raw)
	}
	if result.RuleID != "dml.where.require" {
		t.Fatalf("rule_id = %q, want dml.where.require", result.RuleID)
	}
	if result.Status.State != "on" {
		t.Fatalf("status.state = %q, want on", result.Status.State)
	}
	if string(result.Status.Level) != "blocker" {
		t.Fatalf("status.level = %q, want blocker", result.Status.Level)
	}
	if result.RuleDetailsCommand == "" {
		t.Fatalf("expected rule_details_command to be set")
	}
}

func TestConfigStatus_JSONPartialReplacement(t *testing.T) {
	path := writeConfigStatusYAML(t, "rules:\n  dml.where.require:\n    level: warning\n")

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"--config", path, "config", "status", "dml.where.require", "--format", "json"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	raw := stdout.String()
	if !strings.Contains(raw, `"version"`) {
		t.Fatalf("JSON output must include a top-level version field: %s", raw)
	}
	if strings.Contains(raw, "severity") {
		t.Fatalf("JSON output must not include severity: %s", raw)
	}

	var result configstatus.Result
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal config status JSON: %v\nraw: %s", err, raw)
	}
	if result.Status.State != "off" {
		t.Fatalf("status.state = %q, want off", result.Status.State)
	}

	changed := make(map[string]bool, len(result.ConfigEffect.ChangedFields))
	for _, field := range result.ConfigEffect.ChangedFields {
		changed[field] = true
	}
	if !changed["enabled"] || !changed["level"] {
		t.Fatalf("expected changed_fields to include enabled and level, got %v", result.ConfigEffect.ChangedFields)
	}

	foundReplacement := false
	for _, message := range result.ConfigEffect.Messages {
		if strings.Contains(message, "replaces the default rule policy") {
			foundReplacement = true
			break
		}
	}
	if !foundReplacement {
		t.Fatalf("expected replacement warning in messages, got %v", result.ConfigEffect.Messages)
	}
}

func TestConfigStatus_MissingRuleID(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"config", "status"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code == 0 {
		t.Fatalf("expected non-zero exit code, got %d", code)
	}
}

func TestConfigStatus_UnknownRule(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"config", "status", "not.real.rule"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code == 0 {
		t.Fatalf("expected non-zero exit code, got %d", code)
	}
	combined := stderr.String()
	if !strings.Contains(combined, "not.real.rule") {
		t.Fatalf("expected stderr to mention rule id, got %q", combined)
	}
	if !strings.Contains(combined, "not found") {
		t.Fatalf("expected stderr to mention 'not found', got %q", combined)
	}
}

func TestConfigStatus_InvalidFormat(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"config", "status", "dml.where.require", "--format", "xml"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code == 0 {
		t.Fatalf("expected non-zero exit code, got %d", code)
	}
	combined := stderr.String()
	if !strings.Contains(combined, "format") {
		t.Fatalf("expected stderr to mention format, got %q", combined)
	}
	if !strings.Contains(combined, "xml") {
		t.Fatalf("expected stderr to mention xml, got %q", combined)
	}
}

func TestConfigStatus_InvalidConfig(t *testing.T) {
	path := writeConfigStatusYAML(t, "rules:\n  dml.where.require:\n    level: bogus\n")

	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"--config", path, "config", "status", "dml.where.require"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code == 0 {
		t.Fatalf("expected non-zero exit code, got %d", code)
	}
	if stderr.Len() == 0 {
		t.Fatalf("expected a useful error message on stderr")
	}
}

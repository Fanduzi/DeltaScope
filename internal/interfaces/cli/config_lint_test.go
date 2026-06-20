// Package cli verifies the Cobra CLI adapter behavior.
// input: config lint command invocations and temporary YAML config files
// output: focused CLI coverage for config lint warnings, --strict, error precedence, and ordering
// pos: interface-layer CLI config lint test coverage
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLintConfig writes body to a temp YAML config file and returns its path.
func writeLintConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deltascope.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// runConfigLint runs `deltascope config lint` with the supplied args against an empty stdin
// and returns the exit code plus captured stdout/stderr.
func runConfigLint(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(context.Background(), args, strings.NewReader(""), stdout, stderr)
	return code, stdout.String(), stderr.String()
}

// TestConfigLintCleanConfigPrintsConfigOK locks the no-error/no-warning case: a fully
// explicit rule override whose effective values match the defaults lints clean, so the
// command prints only "Config OK", exits 0, and emits no warning block and no severity.
func TestConfigLintCleanConfigPrintsConfigOK(t *testing.T) {
	path := writeLintConfig(t, `
rules:
  dml.where.require:
    enabled: true
    level: blocker
    params:
      required: true
`)
	code, stdout, stderr := runConfigLint(t, "config", "lint", "--file", path)

	if code != 0 {
		t.Fatalf("expected exit 0 for clean config, got %d\nstdout=%q\nstderr=%q", code, stdout, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "Config OK" {
		t.Fatalf("expected stdout to be exactly 'Config OK', got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for clean config, got %q", stderr)
	}
}

// TestConfigLintRequiresFile locks the existing missing-file contract at the Execute level:
// without --file the command exits 2 with the same missing-file message semantics.
func TestConfigLintRequiresFile(t *testing.T) {
	code, stdout, stderr := runConfigLint(t, "config", "lint")

	if code != exitUser {
		t.Fatalf("expected exit %d for missing --file, got %d", exitUser, code)
	}
	if !strings.Contains(stderr, "config lint requires --file") {
		t.Fatalf("expected missing-file error on stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "Config OK") {
		t.Fatalf("expected no success output for missing --file, got %q", stdout)
	}
}

// TestConfigLintWarnsOnLevelOnlyOverride locks the canonical footgun: mentioning
// dml.where.require with only level:warning replaces the whole policy, so omitted enabled
// turns the rule OFF. The output begins with "Config OK with warnings", lists the warnings,
// never introduces a severity field, and exits 0 by default.
func TestConfigLintWarnsOnLevelOnlyOverride(t *testing.T) {
	path := writeLintConfig(t, `
rules:
  dml.where.require:
    level: warning
`)
	code, stdout, stderr := runConfigLint(t, "config", "lint", "--file", path)

	if code != 0 {
		t.Fatalf("expected exit 0 for warnings-only config by default, got %d\nstdout=%q\nstderr=%q", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "Config OK with warnings") {
		t.Fatalf("expected output to begin with 'Config OK with warnings', got %q", stdout)
	}
	for _, want := range []string{
		"Warnings:",
		"dml.where.require",
		"enabled",
		"is OFF",
		"replaces the whole rule policy",
		"does not merge with defaults",
		"Inspect effective rule status:",
		"deltascope config status dml.where.require --config " + path,
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected warnings output to contain %q, got:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "Next:") {
		t.Fatalf("warnings output must not use the 'Next:' handoff, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "severity") {
		t.Fatalf("warnings output must not introduce a severity field, got:\n%s", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for warnings-only config, got %q", stderr)
	}
}

// TestConfigLintStrictPromotesWarningsToExitTwo locks --strict: the same warnings-only
// config prints the identical warning text but exits 2, with nothing on stderr.
func TestConfigLintStrictPromotesWarningsToExitTwo(t *testing.T) {
	path := writeLintConfig(t, `
rules:
  dml.where.require:
    level: warning
`)
	defaultCode, defaultOut, _ := runConfigLint(t, "config", "lint", "--file", path)
	if defaultCode != 0 {
		t.Fatalf("expected default exit 0, got %d", defaultCode)
	}

	strictCode, strictOut, strictErr := runConfigLint(t, "config", "lint", "--file", path, "--strict")
	if strictCode != exitUser {
		t.Fatalf("expected --strict exit %d for warnings-only config, got %d", exitUser, strictCode)
	}
	if strictOut != defaultOut {
		t.Fatalf("--strict must print the same warnings as default mode.\ndefault:\n%s\nstrict:\n%s", defaultOut, strictOut)
	}
	if strictErr != "" {
		t.Fatalf("--strict warnings-only must print nothing to stderr, got %q", strictErr)
	}
}

// TestConfigLintErrorsTakePrecedenceOverWarnings locks error precedence: a config that also
// mentions an unknown rule fails with the unknown-rule error and exits 2, and never prints
// "Config OK with warnings" even though a known rule carries replacement hazards.
func TestConfigLintErrorsTakePrecedenceOverWarnings(t *testing.T) {
	path := writeLintConfig(t, `
rules:
  dml.where.require:
    level: warning
  totally.unknown.rule:
    enabled: true
`)
	code, stdout, stderr := runConfigLint(t, "config", "lint", "--file", path)

	if code != exitUser {
		t.Fatalf("expected exit %d for unknown rule, got %d", exitUser, code)
	}
	if !strings.Contains(stderr, "unknown rule") {
		t.Fatalf("expected unknown-rule error on stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "Config OK with warnings") {
		t.Fatalf("error result must not print 'Config OK with warnings', got %q", stdout)
	}
}

// TestConfigLintWarningOrderIsDeterministic locks the rendered warning order within one
// rule: enabled, then level, then params.<key>, matching the core order.
func TestConfigLintWarningOrderIsDeterministic(t *testing.T) {
	// ddl.table.name.pattern.require defaults to enabled, a non-empty level, and params
	// {required, pattern}. Supplying only params.required omits enabled and level and the
	// pattern param, producing warnings in canonical order: enabled, level, params.pattern.
	path := writeLintConfig(t, `
rules:
  ddl.table.name.pattern.require:
    params:
      required: false
`)
	code, stdout, _ := runConfigLint(t, "config", "lint", "--file", path)
	if code != 0 {
		t.Fatalf("expected exit 0 for warnings-only config, got %d", code)
	}

	// Each phrase appears exactly once; assert strictly increasing positions.
	enabledAt := strings.Index(stdout, "is OFF")
	levelAt := strings.Index(stdout, "has no effective level")
	paramsAt := strings.Index(stdout, "removes default")
	for _, pos := range []int{enabledAt, levelAt, paramsAt} {
		if pos < 0 {
			t.Fatalf("missing an expected warning in output:\n%s", stdout)
		}
	}
	if !(enabledAt < levelAt && levelAt < paramsAt) {
		t.Fatalf("expected warning order enabled(%d) < level(%d) < params.pattern(%d) in:\n%s", enabledAt, levelAt, paramsAt, stdout)
	}
}

// TestConfigLintWarningOrderAcrossRules locks rule_id ascending: warnings for the
// alphabetically-earlier rule render first.
func TestConfigLintWarningOrderAcrossRules(t *testing.T) {
	path := writeLintConfig(t, `
rules:
  dml.where.require:
    level: warning
  ddl.table.name.max_length:
    level: notice
`)
	code, stdout, _ := runConfigLint(t, "config", "lint", "--file", path)
	if code != 0 {
		t.Fatalf("expected exit 0 for warnings-only config, got %d", code)
	}

	ddlAt := strings.Index(stdout, "ddl.table.name.max_length")
	dmlAt := strings.Index(stdout, "dml.where.require")
	if ddlAt < 0 || dmlAt < 0 {
		t.Fatalf("expected both rule ids in output:\n%s", stdout)
	}
	if !(ddlAt < dmlAt) {
		t.Fatalf("expected rule_id ascending: ddl.table.name.max_length(%d) before dml.where.require(%d) in:\n%s", ddlAt, dmlAt, stdout)
	}
}

// TestConfigLintStillRejectsInvalidValues locks that existing error categories still fail
// with exit 2 after the lint path moved onto configlint: invalid level, unknown param, and
// param type mismatch.
func TestConfigLintStillRejectsInvalidValues(t *testing.T) {
	t.Run("invalid level", func(t *testing.T) {
		path := writeLintConfig(t, `
rules:
  dml.where.require:
    level: bogus
`)
		code, _, stderr := runConfigLint(t, "config", "lint", "--file", path)
		if code != exitUser {
			t.Fatalf("expected exit %d for invalid level, got %d", exitUser, code)
		}
		if !strings.Contains(stderr, "invalid level") {
			t.Fatalf("expected invalid-level error, got %q", stderr)
		}
	})

	t.Run("unknown param", func(t *testing.T) {
		path := writeLintConfig(t, `
rules:
  dml.where.require:
    enabled: true
    level: blocker
    params:
      mystery: true
`)
		code, _, stderr := runConfigLint(t, "config", "lint", "--file", path)
		if code != exitUser {
			t.Fatalf("expected exit %d for unknown param, got %d", exitUser, code)
		}
		if !strings.Contains(stderr, "unknown param") {
			t.Fatalf("expected unknown-param error, got %q", stderr)
		}
	})

	t.Run("param type mismatch", func(t *testing.T) {
		path := writeLintConfig(t, `
rules:
  ddl.table.name.max_length:
    enabled: true
    level: blocker
    params:
      limit: not-a-number
`)
		code, _, stderr := runConfigLint(t, "config", "lint", "--file", path)
		if code != exitUser {
			t.Fatalf("expected exit %d for param type mismatch, got %d", exitUser, code)
		}
		if !strings.Contains(stderr, "invalid type") {
			t.Fatalf("expected invalid-type error, got %q", stderr)
		}
	})
}

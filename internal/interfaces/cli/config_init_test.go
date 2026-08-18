// Package cli verifies the Cobra CLI adapter behavior.
// input: config init/show-default invocations, shipped example YAML, handwritten full-spec overrides, and default-policy audit JSON
// output: focused CLI coverage for quoted empty-string encoding, lint-clean templates, and default vs generated-config finding equality
// pos: interface-layer CLI config generation test coverage
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func runConfigCommand(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(context.Background(), args, strings.NewReader(""), stdout, stderr)
	return code, stdout.String(), stderr.String()
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deltascope.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func shippedExampleConfigPath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "configs", "deltascope.example.yaml")
}

func assertConfigLintsClean(t *testing.T, path string) {
	t.Helper()
	code, stdout, stderr := runConfigCommand(t, "config", "lint", "--file", path)
	if code != 0 {
		t.Fatalf("expected lint exit 0, got %d\nstdout=%q\nstderr=%q", code, stdout, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "Config OK" {
		t.Fatalf("expected stdout to be exactly 'Config OK', got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for clean generated config, got %q", stderr)
	}
}

// TestConfigInitYAMLLintsClean is the golden path for issue #20: the file
// `config init` writes must pass `config lint` with no type errors and no
// replacement-hazard warnings.
func TestConfigInitYAMLLintsClean(t *testing.T) {
	code, yamlOut, stderr := runConfigCommand(t, "config", "init")
	if code != 0 {
		t.Fatalf("expected config init exit 0, got %d\nstderr=%q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr from config init, got %q", stderr)
	}
	if !strings.Contains(yamlOut, "suffix: \"\"") && !strings.Contains(yamlOut, "suffix: ''") {
		t.Fatalf("expected empty string params to be encoded as quoted empty YAML strings, got no quoted suffix")
	}
	if strings.Contains(yamlOut, "\n      suffix:\n") {
		t.Fatalf("empty suffix params must not be encoded as YAML null")
	}

	path := writeTempConfig(t, yamlOut)
	assertConfigLintsClean(t, path)

	strictCode, strictOut, strictErr := runConfigCommand(t, "config", "lint", "--file", path, "--strict")
	if strictCode != 0 {
		t.Fatalf("expected generated config to pass lint --strict, got %d\nstdout=%q\nstderr=%q", strictCode, strictOut, strictErr)
	}
}

// TestConfigShowDefaultMatchesInitAndLintsClean locks that show-default is the
// same YAML as config init and also lints clean.
func TestConfigShowDefaultMatchesInitAndLintsClean(t *testing.T) {
	initCode, initOut, initErr := runConfigCommand(t, "config", "init")
	if initCode != 0 {
		t.Fatalf("expected config init exit 0, got %d\nstderr=%q", initCode, initErr)
	}
	showCode, showOut, showErr := runConfigCommand(t, "config", "show-default")
	if showCode != 0 {
		t.Fatalf("expected config show-default exit 0, got %d\nstderr=%q", showCode, showErr)
	}
	if showOut != initOut {
		t.Fatalf("config show-default must match config init byte-for-byte")
	}
	assertConfigLintsClean(t, writeTempConfig(t, showOut))
}

// TestShippedExampleYAMLLintsClean locks the committed example policy. Empty
// string params must stay quoted so lint does not treat them as YAML null.
func TestShippedExampleYAMLLintsClean(t *testing.T) {
	path := shippedExampleConfigPath(t)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shipped example: %v", err)
	}
	text := string(content)
	if strings.Contains(text, "\n      suffix:\n") || strings.Contains(text, "\n      prefix:\n") {
		t.Fatalf("shipped example must not encode empty string params as YAML null")
	}
	if !strings.Contains(text, "suffix: \"\"") {
		t.Fatalf("shipped example must encode empty suffix params as quoted empty strings")
	}
	assertConfigLintsClean(t, path)
}

// TestHandWrittenFullSpecOverrideStillLintsClean pins the already-supported
// full-spec override path so the encoding fix does not change lint of a
// complete hand-written rule policy.
func TestHandWrittenFullSpecOverrideStillLintsClean(t *testing.T) {
	path := writeTempConfig(t, `
rules:
  dml.where.require:
    enabled: true
    level: blocker
    params:
      required: true
`)
	assertConfigLintsClean(t, path)
}

// TestGeneratedConfigPreservesDefaultAuditFindings compares a default-policy
// audit with the same SQL audited under the generated config file.
func TestGeneratedConfigPreservesDefaultAuditFindings(t *testing.T) {
	initCode, initOut, initErr := runConfigCommand(t, "config", "init")
	if initCode != 0 {
		t.Fatalf("expected config init exit 0, got %d\nstderr=%q", initCode, initErr)
	}
	configPath := writeTempConfig(t, initOut)

	sql := "CREATE TABLE users (id bigint, name varchar(10)); DELETE FROM users;"
	defaultCode, defaultOut, defaultErr := runConfigCommand(t, "audit", "--sql", sql, "--format", "json", "--fail-on", "none")
	if defaultErr != "" {
		t.Fatalf("default audit stderr: %q", defaultErr)
	}
	generatedCode, generatedOut, generatedErr := runConfigCommand(t, "audit", "--sql", sql, "--config", configPath, "--format", "json", "--fail-on", "none")
	if generatedErr != "" {
		t.Fatalf("generated-config audit stderr: %q", generatedErr)
	}
	if defaultCode != generatedCode {
		t.Fatalf("audit exit codes diverged: default=%d generated=%d", defaultCode, generatedCode)
	}
	if got, want := findingFingerprint(t, generatedOut), findingFingerprint(t, defaultOut); got != want {
		t.Fatalf("default-policy audit findings changed under generated config\ndefault=%s\ngenerated=%s", want, got)
	}
}

type findingNote struct {
	RuleID  string `json:"rule_id"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

func findingFingerprint(t *testing.T, raw string) string {
	t.Helper()
	var decoded struct {
		Verdict    string `json:"verdict"`
		Summary    any    `json:"summary"`
		Statements []struct {
			Findings []findingNote `json:"findings"`
		} `json:"statements"`
	}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("unmarshal audit json: %v\noutput=%s", err, raw)
	}
	var b strings.Builder
	fmtFingerprint := func(v any) {
		encoded, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal fingerprint: %v", err)
		}
		b.Write(encoded)
		b.WriteByte('\n')
	}
	fmtFingerprint(decoded.Verdict)
	fmtFingerprint(decoded.Summary)
	for _, statement := range decoded.Statements {
		fmtFingerprint(statement.Findings)
	}
	return b.String()
}

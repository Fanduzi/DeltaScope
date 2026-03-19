// Package cli verifies the Cobra CLI adapter behavior.
// input: command-line args, stdin/file SQL sources, and config-init/version requests
// output: end-to-end CLI behavior coverage for exit codes and rendered output
// pos: interface-layer CLI test coverage
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	viperconfig "github.com/Fanduzi/DeltaScope/internal/infrastructure/config/viper"
)

func TestAuditCommandSupportsSQLJSONOutput(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 1 {
		t.Fatalf("expected exit code 1 for blocker findings, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output for audit findings, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	if decoded["verdict"] != "reject" {
		t.Fatalf("expected verdict reject, got %#v", decoded["verdict"])
	}
}

func TestAuditCommandSupportsFileInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "change.sql")
	if err := os.WriteFile(path, []byte("delete from users"), 0o644); err != nil {
		t.Fatalf("write sql file: %v", err)
	}

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--file", path},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected exit code 1 for blocker findings, got %d", code)
	}
	if !strings.Contains(stdout.String(), "# DeltaScope Audit Result") {
		t.Fatalf("expected markdown output, got %s", stdout.String())
	}
}

func TestAuditCommandSupportsStdinInput(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit"},
		strings.NewReader("delete from users"),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected exit code 1 for blocker findings, got %d", code)
	}
}

func TestAuditCommandHonorsFailOnThreshold(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "create table users (id bigint, primary key (id))", "--fail-on", "warning"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 1 {
		t.Fatalf("expected exit code 1 when warning threshold is reached, got %d", code)
	}
	if !strings.Contains(stdout.String(), "table comment is required") {
		t.Fatalf("expected warning finding in output, got %s", stdout.String())
	}
}

func TestAuditCommandSupportsConfigOverride(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "policy.yaml")
	config := "rules:\n  dml.where.require:\n    enabled: false\n    level: blocker\n    params:\n      required: true\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--config", configPath},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0 after disabling where rule, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Verdict: `pass`") {
		t.Fatalf("expected pass verdict, got %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestAuditCommandRejectsUnknownFailOnValue(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--fail-on", "fatal"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected usage exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unsupported fail threshold") {
		t.Fatalf("expected threshold validation error, got %q", stderr.String())
	}
}

func TestConfigInitWritesUsableYAML(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"config", "init"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "dml.where.require") {
		t.Fatalf("expected rendered config template, got %s", stdout.String())
	}

	configPath := filepath.Join(t.TempDir(), "generated.yaml")
	if err := os.WriteFile(configPath, []byte(stdout.String()), 0o600); err != nil {
		t.Fatalf("write generated config: %v", err)
	}

	cfg, err := viperconfig.LoadPolicy(configPath)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	if !cfg.Rules["dml.where.require"].Enabled {
		t.Fatalf("expected generated config to preserve dml.where.require")
	}
}

func TestVersionCommandPrintsVersion(t *testing.T) {
	stdout := &strings.Builder{}
	previous := Version
	Version = "test-build"
	t.Cleanup(func() { Version = previous })

	code := Execute(
		context.Background(),
		[]string{"version"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.TrimSpace(stdout.String()) != "test-build" {
		t.Fatalf("expected version output test-build, got %q", stdout.String())
	}
}

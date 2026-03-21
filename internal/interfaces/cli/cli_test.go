// Package cli verifies the Cobra CLI adapter behavior.
// input: command-line args, stdin/file SQL sources, password-prompt doubles, and config-init/version requests
// output: end-to-end CLI behavior coverage for exit codes, rendered output, and connection-flag validation
// pos: interface-layer CLI test coverage
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
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

func TestAuditCommandTreatsMissingConfigAsUserError(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--config", filepath.Join(t.TempDir(), "missing.yaml")},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected exit code 2 for unreadable config, got %d", code)
	}
	if !strings.Contains(stderr.String(), "load policy:") {
		t.Fatalf("expected config loader error on stderr, got %q", stderr.String())
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

func TestAuditCommandQuietMarkdownOmitsFullReportWrapper(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--quiet"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected exit code 1 for blocker findings, got %d", code)
	}
	if strings.Contains(stdout.String(), "# DeltaScope Audit Result") {
		t.Fatalf("expected quiet output to omit markdown wrapper, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "dml.where.require") {
		t.Fatalf("expected quiet output to keep finding identity, got %q", stdout.String())
	}
}

func TestAuditCommandAcceptsMySQLStyleConnectionFlagsWithoutChangingOfflineBehavior(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": {"app"}},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "-h", "127.0.0.1", "-P", "3307", "-u", "root", "-p", "secret", "-D", "app"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected blocker exit code 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "# DeltaScope Audit Result") {
		t.Fatalf("expected standard offline report output, got %q", stdout.String())
	}
}

func TestAuditCommandRejectsAskPasswordWithPassword(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "-u", "root", "-p", "secret", "--ask-password"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected user error exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "mutually exclusive") {
		t.Fatalf("expected mutual exclusion error, got %q", stderr.String())
	}
}

func TestAuditCommandRejectsSocketWithHostOrPort(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--socket", "/tmp/mysql.sock", "--host", "127.0.0.1"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected user error exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "socket") || !strings.Contains(stderr.String(), "host/port") {
		t.Fatalf("expected socket conflict error, got %q", stderr.String())
	}
}

func TestAuditCommandPromptsForPasswordWhenAskPasswordIsSet(t *testing.T) {
	previousClientFactory := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": {"app"}},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previousClientFactory })

	previous := passwordPrompt
	called := 0
	passwordPrompt = func(_ io.Reader, out io.Writer) (string, error) {
		called++
		_, _ = out.Write([]byte("Password: "))
		return "secret", nil
	}
	t.Cleanup(func() { passwordPrompt = previous })

	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "-u", "root", "--ask-password"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 1 {
		t.Fatalf("expected blocker exit code 1, got %d", code)
	}
	if called != 1 {
		t.Fatalf("expected password prompt to be called once, got %d", called)
	}
	if !strings.Contains(stderr.String(), "Password:") {
		t.Fatalf("expected password prompt text on stderr, got %q", stderr.String())
	}
}

func TestAuditCommandReturnsUserErrorWhenPasswordPromptFails(t *testing.T) {
	previous := passwordPrompt
	passwordPrompt = func(_ io.Reader, _ io.Writer) (string, error) {
		return "", errors.New("prompt failed")
	}
	t.Cleanup(func() { passwordPrompt = previous })

	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--ask-password"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected user error exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "prompt password") {
		t.Fatalf("expected prompt failure on stderr, got %q", stderr.String())
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

func TestConfigLintAcceptsValidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.yaml")
	content := "rules:\n  dml.where.require:\n    enabled: true\n    level: blocker\n    params:\n      required: true\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"config", "lint", "--file", path},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Config OK") {
		t.Fatalf("expected success output, got %q", stdout.String())
	}
}

func TestConfigLintRejectsUnknownRuleAndInvalidValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.yaml")
	content := "rules:\n  made.up.rule:\n    enabled: true\n    level: fatal\n    params:\n      required: not-a-bool\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"config", "lint", "--file", path},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown rule") {
		t.Fatalf("expected unknown rule lint failure, got %q", stderr.String())
	}
}

func TestConfigShowDefaultPrintsBuiltInConfig(t *testing.T) {
	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"config", "show-default"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "dml.where.require") {
		t.Fatalf("expected default config output, got %q", stdout.String())
	}
}

func TestRulesListPrintsShippedRules(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"rules", "list"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "dml.where.require") {
		t.Fatalf("expected shipped rule in list output, got %q", stdout.String())
	}
}

func TestRulesListSupportsKindLevelAndEnabledFilters(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"rules", "list", "--kind", "dml", "--level", "blocker", "--enabled-only"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "dml.where.require") {
		t.Fatalf("expected blocker dml rule in filtered output, got %q", output)
	}
	if strings.Contains(output, "ddl.table.comment.require") {
		t.Fatalf("expected ddl rule to be filtered out, got %q", output)
	}
}

func TestRulesShowPrintsExamplesConfigAndRemediation(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"rules", "show", "dml.where.require"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "Trigger Example") || !strings.Contains(output, "Config Example") || !strings.Contains(output, "Remediation") {
		t.Fatalf("expected detailed rule sections, got %q", output)
	}
}

func TestRulesSearchMatchesByKeyword(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"rules", "search", "where"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "dml.where.require") {
		t.Fatalf("expected where rule in search output, got %q", stdout.String())
	}
}

func TestCapabilitiesPrintsStableSummary(t *testing.T) {
	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"capabilities"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	output := stdout.String()
	for _, expected := range []string{"dialects:", "mysql", "tidb", "modes:", "offline", "metadata-aware", "surfaces:", "cli", "http"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected capabilities output to contain %q, got %q", expected, output)
		}
	}
}

func TestVersionCommandPrintsLogoAndVersion(t *testing.T) {
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
	if !strings.Contains(stdout.String(), "____") {
		t.Fatalf("expected logo output, got %q", stdout.String())
	}
	if !strings.HasSuffix(strings.TrimSpace(stdout.String()), "test-build") {
		t.Fatalf("expected version suffix test-build, got %q", stdout.String())
	}
}

func TestRootVersionFlagPrintsVersionOnly(t *testing.T) {
	stdout := &strings.Builder{}
	previous := Version
	Version = "test-build"
	t.Cleanup(func() { Version = previous })

	code := Execute(
		context.Background(),
		[]string{"--version"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.TrimSpace(stdout.String()) != "test-build" {
		t.Fatalf("expected plain version output test-build, got %q", stdout.String())
	}
}

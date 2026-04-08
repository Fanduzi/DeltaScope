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

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	viperconfig "github.com/Fanduzi/DeltaScope/internal/infrastructure/config/viper"
)

type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}

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
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) == 0 {
		t.Fatalf("expected statements array, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first statement object, got %#v", statements[0])
	}
	impact, ok := statement["impact"].(map[string]any)
	if !ok {
		t.Fatalf("expected statement impact object, got %#v", statement["impact"])
	}
	if impact["risk_level"] != "high" {
		t.Fatalf("expected high risk level, got %#v", impact["risk_level"])
	}
	if impact["confidence"] != "high" {
		t.Fatalf("expected high confidence, got %#v", impact["confidence"])
	}
	if impact["source"] != "shape" {
		t.Fatalf("expected shape source, got %#v", impact["source"])
	}
	if ratio, ok := impact["estimated_ratio"].(float64); !ok || ratio != 1 {
		t.Fatalf("expected estimated_ratio 1, got %#v", impact["estimated_ratio"])
	}
	reasonCodes, ok := impact["reason_codes"].([]any)
	if !ok || len(reasonCodes) != 1 || reasonCodes[0] != "missing_where" {
		t.Fatalf("expected missing_where reason code, got %#v", impact["reason_codes"])
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

func TestResolveAuditSQLPrintsInteractiveStdinHint(t *testing.T) {
	stderr := &strings.Builder{}

	sql, err := resolveAuditSQL(context.Background(), strings.NewReader("delete from users"), "", "", stderr, true)
	if err != nil {
		t.Fatalf("resolve audit sql: %v", err)
	}
	if sql != "delete from users" {
		t.Fatalf("expected stdin sql, got %q", sql)
	}
	if !strings.Contains(stderr.String(), "Waiting for SQL from stdin. Press Ctrl+D to finish.") {
		t.Fatalf("expected stdin waiting hint, got %q", stderr.String())
	}
}

func TestResolveAuditSQLRejectsConflictingOrEmptyInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.sql")
	if err := os.WriteFile(path, []byte("   \n"), 0o644); err != nil {
		t.Fatalf("write sql file: %v", err)
	}

	if _, err := resolveAuditSQL(context.Background(), strings.NewReader(""), "delete from users", path, io.Discard, false); err == nil {
		t.Fatal("expected conflict error when both --sql and --file are provided")
	}
	if _, err := resolveAuditSQL(context.Background(), strings.NewReader(""), "", path, io.Discard, false); err == nil {
		t.Fatal("expected empty file input to be rejected")
	}
	if _, err := resolveAuditSQL(context.Background(), strings.NewReader("   "), "", "", io.Discard, false); err == nil {
		t.Fatal("expected empty stdin input to be rejected")
	}
}

func TestParseDialectNormalizesKnownAndUnknownValues(t *testing.T) {
	tests := map[string]spec.Dialect{
		"":            spec.DialectMySQL,
		"mysql":       spec.DialectMySQL,
		" TIDB ":      spec.DialectTiDB,
		"PostgreSQL":  spec.DialectPostgreSQL,
		"ClickHouse":  spec.Dialect("clickhouse"),
	}

	for input, want := range tests {
		if got := parseDialect(input); got != want {
			t.Fatalf("parseDialect(%q) = %q, want %q", input, got, want)
		}
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

func TestAuditCommandRendersNamingGovernanceFinding(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "policy.yaml")
	config := "rules:\n  ddl.table.name.prefix.require:\n    enabled: true\n    level: warning\n    params:\n      prefix: tbl_\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	Execute(
		context.Background(),
		[]string{"audit", "--sql", "create table users (id bigint, primary key (id)) comment='users'", "--config", configPath, "--quiet"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	output := stdout.String()
	if !strings.Contains(output, "ddl.table.name.prefix.require") {
		t.Fatalf("expected naming rule id in user-visible output, got %q", output)
	}
	if !strings.Contains(output, `table name "users" must start with "tbl_"`) {
		t.Fatalf("expected naming message in user-visible output, got %q", output)
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

func TestAuditCommandMySQLPGMismatchShowsDialectHintOnStderr(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "insert into users (name) values ('alice') returning id;", "--dialect", "mysql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected exit code 2 for user-facing parse error, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output on parse error, got %q", stdout.String())
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "dialect mismatch") {
		t.Fatalf("expected dialect mismatch hint on stderr, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "postgresql") {
		t.Fatalf("expected postgresql hint on stderr, got %q", stderr.String())
	}
}

func TestAuditCommandPostgreSQLPathDoesNotShowMismatchHint(t *testing.T) {
	stderr := &strings.Builder{}

	Execute(
		context.Background(),
		[]string{"audit", "--sql", "insert into users (name) values ('alice') returning id;", "--dialect", "postgresql"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if strings.Contains(strings.ToLower(stderr.String()), "dialect mismatch") {
		t.Fatalf("did not expect dialect mismatch hint on postgresql path, got %q", stderr.String())
	}
}

func TestAuditCommandMySQLParseablePGMismatchShowsAdvisoryFinding(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "create table users (id serial primary key);", "--dialect", "mysql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 1 {
		t.Fatalf("expected notice-level audit failure via existing fail-on behavior, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected success-path advisory to stay on stdout, got %q", stderr.String())
	}
	if !strings.Contains(strings.ToLower(stdout.String()), "dialect mismatch") {
		t.Fatalf("expected advisory finding in stdout, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "postgresql") {
		t.Fatalf("expected postgresql target in advisory output, got %q", stdout.String())
	}
}

func TestAuditCommandPostgreSQLParseablePGSyntaxDoesNotShowMismatchAdvisory(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "create table users (id serial primary key);", "--dialect", "postgresql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code == 0 {
		t.Fatalf("expected current postgresql path to remain non-success in this lane")
	}
	if strings.Contains(strings.ToLower(stdout.String()), "dialect mismatch") || strings.Contains(strings.ToLower(stderr.String()), "dialect mismatch") {
		t.Fatalf("did not expect mismatch advisory on postgresql path, stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestAuditCommandMySQLNormalSQLDoesNotShowMismatchAdvisory(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "create table users (id bigint primary key) comment='users';", "--dialect", "mysql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code == 2 {
		t.Fatalf("did not expect parse-error path, stderr=%q", stderr.String())
	}
	if strings.Contains(strings.ToLower(stdout.String()), "dialect mismatch") {
		t.Fatalf("did not expect advisory for normal mysql sql, got %q", stdout.String())
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

func TestRenderMarkdownResultIncludesFindingAndAggregateExplanationDetails(t *testing.T) {
	output, err := renderMarkdownResult(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{
			Statements: 1,
			Blockers:   1,
		},
		Explanation: &report.Explanation{
			Summary: "Audit produced 1 finding",
			Reasons: []string{"where clause required"},
		},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Explanation: &report.Explanation{
				Summary: "Statement 1 has 1 finding",
				Reasons: []string{"where clause required"},
			},
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause required",
				Explanation: &rule.FindingExplanation{
					Why:        "The shipped policy requires a predicate.",
					Risk:       "Without a predicate the statement can affect every row.",
					Suggestion: "Add a WHERE clause that narrows the target rows.",
					Metadata: &rule.ExplanationMetadata{
						Status: "limited",
						Note:   "metadata unavailable",
					},
				},
			}},
		}},
	}, nil)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	rendered := string(output)
	for _, want := range []string{
		"## Result Explanation",
		"Audit produced 1 finding",
		"- where clause required",
		"### Explanation",
		"Statement 1 has 1 finding",
		"Why: The shipped policy requires a predicate.",
		"Risk: Without a predicate the statement can affect every row.",
		"Suggestion: Add a WHERE clause that narrows the target rows.",
		"Metadata status: `limited`",
		"Metadata note: metadata unavailable",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected markdown output to contain %q\noutput:\n%s", want, rendered)
		}
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
	passwordPrompt = func(out io.Writer) (string, error) {
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

func TestAuditCommandResolvesPasswordFromEnv(t *testing.T) {
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

	t.Setenv("DELTASCOPE_DB_PASSWORD", "secret-from-env")

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "-u", "root", "--password-env", "DELTASCOPE_DB_PASSWORD"},
		strings.NewReader(""),
		&strings.Builder{},
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected blocker exit code 1, got %d", code)
	}
	if client.options.Password != "secret-from-env" {
		t.Fatalf("expected env password to reach metadata client, got %q", client.options.Password)
	}
}

func TestAuditCommandResolvesPasswordFromFile(t *testing.T) {
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

	passwordPath := filepath.Join(t.TempDir(), "db-password.txt")
	if err := os.WriteFile(passwordPath, []byte("secret-from-file\n"), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "-u", "root", "--password-file", passwordPath},
		strings.NewReader(""),
		&strings.Builder{},
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected blocker exit code 1, got %d", code)
	}
	if client.options.Password != "secret-from-file" {
		t.Fatalf("expected file password to be trimmed before reaching metadata client, got %q", client.options.Password)
	}
}

func TestAuditCommandRejectsAskPasswordWithPasswordEnv(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "-u", "root", "--password-env", "DELTASCOPE_DB_PASSWORD", "--ask-password"},
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

func TestAuditCommandCanReadSQLFromStdinWhilePromptingPasswordSeparately(t *testing.T) {
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
	passwordPrompt = func(out io.Writer) (string, error) {
		called++
		_, _ = out.Write([]byte("Password: "))
		return "secret", nil
	}
	t.Cleanup(func() { passwordPrompt = previous })

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--ask-password", "-u", "root"},
		strings.NewReader("delete from users"),
		stdout,
		stderr,
	)

	if code != 1 {
		t.Fatalf("expected blocker exit code 1, got %d", code)
	}
	if called != 1 {
		t.Fatalf("expected password prompt to be called once, got %d", called)
	}
	if len(client.instanceCalls) != 1 || client.instanceCalls[0] != "app" {
		t.Fatalf("expected metadata path to infer app schema, got %#v", client.instanceCalls)
	}
}

func TestAuditCommandReturnsUserErrorWhenPasswordPromptFails(t *testing.T) {
	previous := passwordPrompt
	passwordPrompt = func(_ io.Writer) (string, error) {
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
	output := stdout.String()
	for _, expected := range []string{"RULE ID", "LEVEL", "KIND", "SUMMARY", "dml.where.require"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected table output to contain %q, got %q", expected, output)
		}
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
	for _, expected := range []string{"dialects:", "mysql", "tidb", "modes:", "offline", "metadata-aware", "surfaces:", "cli", "http", "mcp", "go-api"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected capabilities output to contain %q, got %q", expected, output)
		}
	}
}

func TestAuditHelpIncludesOfflineAndMetadataAwareExamples(t *testing.T) {
	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--help"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	output := stdout.String()
	for _, expected := range []string{
		"Offline example",
		"Metadata-aware example",
		"--host",
		"--ask-password",
		"--schema",
		"PostgreSQL is currently offline-only",
		"MySQL/TiDB-compatible instances",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected help output to contain %q, got %q", expected, output)
		}
	}
}

func TestMetadataAwareJSONIncludesAuditContext(t *testing.T) {
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
		[]string{"audit", "--sql", "delete from users", "--host", "127.0.0.1", "--user", "root", "--format", "json"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit exit code 1, got %d", code)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	contextValue, ok := decoded["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context object, got %#v", decoded["context"])
	}
	if contextValue["mode"] != "metadata-aware" || contextValue["schema"] != "app" || contextValue["dialect"] != "mysql" {
		t.Fatalf("expected metadata-aware context, got %#v", contextValue)
	}
}

func TestRenderJSONResultIncludesFindingExplanationFields(t *testing.T) {
	output, err := renderJSONResult(report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause required",
				Explanation: &rule.FindingExplanation{
					Why:        "The shipped policy requires a predicate.",
					Risk:       "Without a predicate the statement can affect every row.",
					Suggestion: "Add a WHERE clause that narrows the target rows.",
					Metadata: &rule.ExplanationMetadata{
						Status: "limited",
						Note:   "metadata unavailable",
					},
				},
			}},
		}},
	}, nil)
	if err != nil {
		t.Fatalf("render json: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("unmarshal rendered json: %v\noutput=%s", err, string(output))
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) != 1 {
		t.Fatalf("expected one finding, got %#v", statement["findings"])
	}
	finding, ok := findings[0].(map[string]any)
	if !ok {
		t.Fatalf("expected finding object, got %#v", findings[0])
	}
	explanation, ok := finding["explanation"].(map[string]any)
	if !ok {
		t.Fatalf("expected explanation object, got %#v", finding["explanation"])
	}
	if explanation["why"] != "The shipped policy requires a predicate." || explanation["risk"] != "Without a predicate the statement can affect every row." || explanation["suggestion"] != "Add a WHERE clause that narrows the target rows." {
		t.Fatalf("expected why/risk/suggestion explanation fields, got %#v", explanation)
	}
	metadata, ok := explanation["metadata"].(map[string]any)
	if !ok || metadata["status"] != "limited" || metadata["note"] != "metadata unavailable" {
		t.Fatalf("expected explanation metadata fields, got %#v", explanation["metadata"])
	}
}

func TestAuditCommandJSONMetadataContextUsesFlagSchemaSource(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{detectDialect: spec.DialectMySQL}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--host", "127.0.0.1", "--user", "root", "--schema", "app", "--format", "json"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit exit code 1, got %d", code)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	contextValue, ok := decoded["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context object, got %#v", decoded["context"])
	}
	if contextValue["schema_source"] != "flag" {
		t.Fatalf("expected schema_source flag, got %#v", contextValue["schema_source"])
	}
}

func TestRenderJSONResultIncludesContextAndAggregateExplanations(t *testing.T) {
	output, err := renderJSONResult(report.Result{
		Explanation: &report.Explanation{
			Summary: "Audit produced 1 finding",
			Reasons: []string{"where clause required"},
		},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Explanation: &report.Explanation{
				Summary: "Statement 1 has 1 finding",
				Reasons: []string{"where clause required"},
			},
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause required",
			}},
		}},
	}, &auditRunContext{Mode: "offline", Dialect: "mysql", DialectSource: "default"})
	if err != nil {
		t.Fatalf("render json: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("unmarshal rendered json: %v\noutput=%s", err, string(output))
	}
	contextValue, ok := decoded["context"].(map[string]any)
	if !ok || contextValue["mode"] != "offline" {
		t.Fatalf("expected context object, got %#v", decoded["context"])
	}
	resultExplanation, ok := decoded["explanation"].(map[string]any)
	if !ok || resultExplanation["summary"] != "Audit produced 1 finding" {
		t.Fatalf("expected result explanation object, got %#v", decoded["explanation"])
	}
	resultReasons, ok := resultExplanation["reasons"].([]any)
	if !ok || len(resultReasons) != 1 || resultReasons[0] != "where clause required" {
		t.Fatalf("expected result explanation reasons, got %#v", resultExplanation["reasons"])
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	statementExplanation, ok := statement["explanation"].(map[string]any)
	if !ok || statementExplanation["summary"] != "Statement 1 has 1 finding" {
		t.Fatalf("expected statement explanation object, got %#v", statement["explanation"])
	}
	statementReasons, ok := statementExplanation["reasons"].([]any)
	if !ok || len(statementReasons) != 1 || statementReasons[0] != "where clause required" {
		t.Fatalf("expected statement explanation reasons, got %#v", statementExplanation["reasons"])
	}
}

func TestRenderJSONResultIncludesStatementImpact(t *testing.T) {
	estimatedRows := int64(12)
	estimatedRatio := 0.25

	output, err := renderJSONResult(report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Impact: &report.Impact{
				EstimatedRows:  &estimatedRows,
				EstimatedRatio: &estimatedRatio,
				RiskLevel:      report.ImpactRiskMedium,
				Confidence:     report.ImpactConfidenceHigh,
				Source:         report.ImpactSourceMetadata,
				ReasonCodes:    []string{"indexed_range"},
			},
		}},
	}, nil)
	if err != nil {
		t.Fatalf("render json: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("unmarshal rendered json: %v\noutput=%s", err, string(output))
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	impact, ok := statement["impact"].(map[string]any)
	if !ok || impact["risk_level"] != "medium" || impact["confidence"] != "high" || impact["source"] != "metadata" {
		t.Fatalf("expected statement impact object, got %#v", statement["impact"])
	}
	if rows, ok := impact["estimated_rows"].(float64); !ok || rows != 12 {
		t.Fatalf("expected estimated_rows 12, got %#v", impact["estimated_rows"])
	}
	reasonCodes, ok := impact["reason_codes"].([]any)
	if !ok || len(reasonCodes) != 1 || reasonCodes[0] != "indexed_range" {
		t.Fatalf("expected indexed_range reason code, got %#v", impact["reason_codes"])
	}
}

func TestRenderMarkdownResultIncludesUnsupportedStatements(t *testing.T) {
	output, err := renderMarkdownResult(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Statements: 2},
		Unsupported: []spec.UnsupportedDetail{{
			Index:   1,
			Feature: "select",
			Reason:  "postgresql statement type is not in the approved v1 subset",
		}},
	}, nil)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}
	if !strings.Contains(string(output), "## Unsupported Statements") {
		t.Fatalf("expected unsupported markdown section, got %q", string(output))
	}
	if !strings.Contains(string(output), "Statement 2") || !strings.Contains(string(output), "select") {
		t.Fatalf("expected unsupported statement details, got %q", string(output))
	}
}

func TestRenderJSONResultIncludesUnsupportedStatements(t *testing.T) {
	output, err := renderJSONResult(report.Result{
		Statements: []report.StatementResult{{Index: 0, Kind: "dml"}},
		Unsupported: []spec.UnsupportedDetail{{
			Index:   1,
			Feature: "select",
			SQL:     "select 1",
			Reason:  "postgresql statement type is not in the approved v1 subset",
		}},
	}, nil)
	if err != nil {
		t.Fatalf("render json: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("unmarshal rendered json: %v\noutput=%s", err, string(output))
	}
	unsupported, ok := decoded["unsupported"].([]any)
	if !ok || len(unsupported) != 1 {
		t.Fatalf("expected one unsupported item, got %#v", decoded["unsupported"])
	}
	item, ok := unsupported[0].(map[string]any)
	if !ok {
		t.Fatalf("expected unsupported object, got %#v", unsupported[0])
	}
	if item["feature"] != "select" || item["reason"] == "" {
		t.Fatalf("expected unsupported feature and reason, got %#v", item)
	}
}

func TestRenderResultUsesQuietAndJSONBranches(t *testing.T) {
	result := report.Result{
		GlobalFindings: []rule.Finding{{
			RuleID:  "global.rule",
			Level:   rule.LevelWarning,
			Message: "pay attention",
		}},
	}

	quietOutput, err := renderResult("markdown", true, result, nil)
	if err != nil {
		t.Fatalf("render quiet result: %v", err)
	}
	if string(quietOutput) != "[warning] global.rule: pay attention" {
		t.Fatalf("expected quiet finding output, got %q", string(quietOutput))
	}

	jsonOutput, err := renderResult("json", false, result, &auditRunContext{Mode: "offline", Dialect: "postgresql", DialectSource: "flag"})
	if err != nil {
		t.Fatalf("render json result: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(jsonOutput, &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v", err)
	}
	contextPayload, ok := decoded["context"].(map[string]any)
	if !ok || contextPayload["dialect"] != "postgresql" {
		t.Fatalf("expected json context payload, got %#v", decoded["context"])
	}

	markdownOutput, err := renderResult("yaml", false, report.Result{Verdict: report.VerdictPass}, nil)
	if err != nil {
		t.Fatalf("render fallback markdown result: %v", err)
	}
	if !strings.Contains(string(markdownOutput), "# DeltaScope Audit Result") {
		t.Fatalf("expected unknown format to fall back to markdown, got %q", string(markdownOutput))
	}
}

func TestExitCodeForResultReturnsAuditFailureWhenUnsupportedExists(t *testing.T) {
	code := exitCodeForResult(report.Result{
		Verdict:     report.VerdictPass,
		Summary:     report.Summary{Statements: 1},
		Unsupported: []spec.UnsupportedDetail{{Feature: "select", Reason: "not supported"}},
	}, "blocker")
	if code != exitAudit {
		t.Fatalf("expected unsupported statements to force audit exit code, got %d", code)
	}
}

func TestExitCodeForResultRespectsThresholds(t *testing.T) {
	tests := []struct {
		name      string
		threshold string
		summary    report.Summary
		want      int
	}{
		{name: "none ignores findings", threshold: "none", summary: report.Summary{Notices: 1, Warnings: 1, Blockers: 1}, want: exitOK},
		{name: "notice trips on notice", threshold: "notice", summary: report.Summary{Notices: 1}, want: exitAudit},
		{name: "warning trips on warning", threshold: "warning", summary: report.Summary{Warnings: 1}, want: exitAudit},
		{name: "blocker trips on blocker", threshold: "blocker", summary: report.Summary{Blockers: 1}, want: exitAudit},
		{name: "blocker ignores warning", threshold: "blocker", summary: report.Summary{Warnings: 1}, want: exitOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCodeForResult(report.Result{Summary: tc.summary}, tc.threshold); got != tc.want {
				t.Fatalf("exitCodeForResult(%+v, %q) = %d, want %d", tc.summary, tc.threshold, got, tc.want)
			}
		})
	}
}

func TestMapAuditErrorClassifiesKnownCases(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "user error", err: newUserError("bad input"), want: exitUser},
		{name: "empty sql", err: appaudit.ErrEmptySQL, want: exitUser},
		{name: "unknown dialect", err: appaudit.ErrUnknownDialect, want: exitUser},
		{name: "unsupported statement", err: appaudit.ErrUnsupportedStatement, want: exitUser},
		{name: "parse sql string match", err: errors.New("parse sql: syntax error"), want: exitUser},
		{name: "pg capable build hint", err: errors.New("requires PG-capable build"), want: exitUser},
		{name: "load policy string match", err: errors.New("load policy: bad config"), want: exitUser},
		{name: "context canceled", err: context.Canceled, want: exitInternal},
		{name: "generic error", err: errors.New("boom"), want: exitInternal},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code := 0
			if got := mapAuditError(&code, tc.err); !errors.Is(got, tc.err) && got.Error() != tc.err.Error() {
				t.Fatalf("expected original error to be returned, got %v want %v", got, tc.err)
			}
			if code != tc.want {
				t.Fatalf("expected exit code %d, got %d", tc.want, code)
			}
		})
	}
}

func TestCapabilitiesCommandReturnsInternalOnWriteFailure(t *testing.T) {
	exitCode := 0
	cmd := newCapabilitiesCmd(&exitCode)
	cmd.SetOut(failingWriter{})
	cmd.SetArgs(nil)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected capabilities command to surface writer error")
	}
	if exitCode != exitInternal {
		t.Fatalf("expected internal exit code on writer failure, got %d", exitCode)
	}
}

func TestConfigAndRuleCommandHelpersHandleValidationAndWriteFailures(t *testing.T) {
	t.Run("config lint requires file", func(t *testing.T) {
		exitCode := 0
		cmd := newConfigLintCmd(&exitCode)
		cmd.SetArgs(nil)

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "requires --file") {
			t.Fatalf("expected missing --file error, got %v", err)
		}
		if exitCode != exitUser {
			t.Fatalf("expected user exit code, got %d", exitCode)
		}
	})

	t.Run("config show-default write failure", func(t *testing.T) {
		exitCode := 0
		cmd := newConfigShowDefaultCmd(&exitCode)
		cmd.SetOut(failingWriter{})
		cmd.SetArgs(nil)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected write failure from config show-default")
		}
		if exitCode != exitInternal {
			t.Fatalf("expected internal exit code, got %d", exitCode)
		}
	})

	t.Run("config init write failure", func(t *testing.T) {
		exitCode := 0
		cmd := newConfigInitCmd(&exitCode)
		cmd.SetOut(failingWriter{})
		cmd.SetArgs(nil)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected write failure from config init")
		}
		if exitCode != exitInternal {
			t.Fatalf("expected internal exit code, got %d", exitCode)
		}
	})

	t.Run("rules show unknown rule", func(t *testing.T) {
		exitCode := 0
		cmd := newRulesShowCmd(&exitCode)
		cmd.SetArgs([]string{"made.up.rule"})

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "unknown rule") {
			t.Fatalf("expected unknown rule error, got %v", err)
		}
		if exitCode != exitUser {
			t.Fatalf("expected user exit code, got %d", exitCode)
		}
	})

	t.Run("rules list invalid filters", func(t *testing.T) {
		exitCode := 0
		cmd := newRulesListCmd(&exitCode)
		cmd.SetArgs([]string{"--kind", "bad"})

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--kind") {
			t.Fatalf("expected invalid kind error, got %v", err)
		}
		if exitCode != exitUser {
			t.Fatalf("expected user exit code, got %d", exitCode)
		}
	})
}

func TestParamTypeMatchesCoversSupportedShapes(t *testing.T) {
	tests := []struct {
		name    string
		raw     any
		def     any
		matches bool
	}{
		{name: "string slice from any slice", raw: []any{"a", "b"}, def: []string{}, matches: true},
		{name: "string slice from typed slice", raw: []string{"a"}, def: []string{}, matches: true},
		{name: "string slice rejects mixed items", raw: []any{"a", 1}, def: []string{}, matches: false},
		{name: "int accepts int64", raw: int64(1), def: int(0), matches: true},
		{name: "int rejects string", raw: "1", def: int(0), matches: false},
		{name: "bool accepts bool", raw: true, def: false, matches: true},
		{name: "string accepts string", raw: "x", def: "", matches: true},
		{name: "default reflect path", raw: 1.5, def: 0.0, matches: true},
		{name: "default reflect path mismatch", raw: 1, def: 0.0, matches: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := paramTypeMatches(tc.raw, tc.def); got != tc.matches {
				t.Fatalf("paramTypeMatches(%T, %T) = %t, want %t", tc.raw, tc.def, got, tc.matches)
			}
		})
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
	output := stdout.String()
	if !strings.Contains(output, "____") {
		t.Fatalf("expected logo output, got %q", output)
	}
	for _, expected := range []string{"deltascope", "test-build", "mysql", "tidb"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected version output to contain %q, got %q", expected, output)
		}
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
	output := strings.TrimSpace(stdout.String())
	for _, expected := range []string{"deltascope", "test-build", "mysql", "tidb"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected root version output to contain %q, got %q", expected, output)
		}
	}
}

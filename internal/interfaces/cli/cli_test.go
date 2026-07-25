// Package cli verifies the Cobra CLI adapter behavior.
// input: command-line args, stdin/file SQL sources, password-prompt doubles, and config-init/version requests
// output: end-to-end CLI behavior coverage for exit codes, rendered output, and connection-flag validation
// pos: interface-layer CLI test coverage
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	viperconfig "github.com/Fanduzi/DeltaScope/internal/infrastructure/config/viper"
)

const postgreSQLSyntaxNoticeRuleID = "dialect.postgresql.syntax.detected.notice"
const mysqlReturningUnsupportedNoticeRuleID = "dialect.mysql.returning.unsupported.notice"

// pgCapabilityBoundaryIsRealParser returns true when the PG-capable parser is
// linked in (CGO_ENABLED=1 -tags postgresql). It returns false when the stub
// build is active and parsePostgreSQL always returns PostgreSQLCapabilityBoundaryError.
func pgCapabilityBoundaryIsRealParser() bool {
	_, err := appaudit.Parse(context.Background(), "SELECT 1", spec.DialectPostgreSQL)
	return err == nil
}

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
		"":           spec.DialectMySQL,
		"mysql":      spec.DialectMySQL,
		" TIDB ":     spec.DialectTiDB,
		"PostgreSQL": spec.DialectPostgreSQL,
		"ClickHouse": spec.Dialect("clickhouse"),
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

func TestAuditCommandMarkdownIncludesOfflineTrustContext(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected blocker exit code 1, got %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "## Audit Context") {
		t.Fatalf("expected audit context section, got %q", output)
	}
	if !strings.Contains(output, "- Mode: `offline`") {
		t.Fatalf("expected offline mode in audit context, got %q", output)
	}
	if !strings.Contains(output, "- Dialect: `mysql` (default)") {
		t.Fatalf("expected mysql default dialect context, got %q", output)
	}
}

// TestAuditCommandMarkdownActionSummaryContract locks the user-facing Action
// Summary block that the default markdown audit path renders for a blocker
// finding. It asserts the section appears, names the rule id and its explain
// command, surfaces the 1-based statement index, and never introduces a
// severity field.
func TestAuditCommandMarkdownActionSummaryContract(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "DELETE FROM users"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit-failure exit code 1 for blocker finding, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "## Action Summary") {
		t.Fatalf("expected action summary section in default markdown output, got:\n%s", output)
	}
	for _, want := range []string{
		"`dml.where.require`",
		"deltascope rules explain dml.where.require",
		"Statements: 1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected default markdown output to contain %q, got:\n%s", want, output)
		}
	}
	if strings.Contains(output, "severity") {
		t.Fatalf("default markdown action summary must not introduce a severity field, got:\n%s", output)
	}
}

// TestAuditCommandExplicitMarkdownActionSummaryContract confirms that an
// explicit --format markdown request renders the same Action Summary block as
// the default markdown path, including the rule explain command.
func TestAuditCommandExplicitMarkdownActionSummaryContract(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--format", "markdown", "--sql", "DELETE FROM users"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit-failure exit code 1 for blocker finding, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "## Action Summary") {
		t.Fatalf("expected action summary section in explicit markdown output, got:\n%s", output)
	}
	if !strings.Contains(output, "deltascope rules explain dml.where.require") {
		t.Fatalf("expected rule explain command in explicit markdown output, got:\n%s", output)
	}
}

// TestAuditCommandJSONActionSummaryOmittedContract locks the JSON contract: the
// derived Action Summary is a markdown-only surface, so JSON output must not
// carry an action_summary key, must keep findings on the existing level field
// (blocker), and must not introduce a severity field.
func TestAuditCommandJSONActionSummaryOmittedContract(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--format", "json", "--sql", "DELETE FROM users"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit-failure exit code 1 for blocker finding, got %d", code)
	}

	raw := stdout.String()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, raw)
	}
	if _, ok := decoded["action_summary"]; ok {
		t.Fatalf("JSON output must not expose action_summary, got:\n%s", raw)
	}
	if strings.Contains(raw, "severity") {
		t.Fatalf("JSON output must not introduce a severity field, got:\n%s", raw)
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
	if !ok || len(findings) == 0 {
		t.Fatalf("expected at least one finding, got %#v", statement["findings"])
	}
	finding, ok := findings[0].(map[string]any)
	if !ok {
		t.Fatalf("expected finding object, got %#v", findings[0])
	}
	if finding["level"] != "blocker" {
		t.Fatalf("expected finding level blocker, got %#v", finding["level"])
	}
}

// TestAuditCommandQuietOmitsActionSummaryContract locks the quiet contract:
// quiet output keeps the single-line finding identity but never renders the
// markdown Action Summary block. Action Summary must not change quiet behavior.
func TestAuditCommandQuietOmitsActionSummaryContract(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--quiet", "--sql", "DELETE FROM users"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected audit-failure exit code 1 for blocker finding, got %d", code)
	}

	output := stdout.String()
	if strings.Contains(output, "## Action Summary") {
		t.Fatalf("quiet output must omit the action summary block, got:\n%s", output)
	}
	if !strings.Contains(output, "dml.where.require") {
		t.Fatalf("quiet output must keep the finding rule identity, got:\n%s", output)
	}
}

// TestAuditCommandCleanResultOmitsActionSummaryContract locks the clean-result
// contract: an audit with no findings must report a pass and omit the Action
// Summary section entirely. SELECT 1 is a stable clean input (no rules apply).
func TestAuditCommandCleanResultOmitsActionSummaryContract(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "SELECT 1"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected clean exit code 0, got %d\nstdout=%s", code, stdout.String())
	}

	if strings.Contains(stdout.String(), "## Action Summary") {
		t.Fatalf("clean result must omit the action summary section, got:\n%s", stdout.String())
	}
}

func TestAuditCommandShowsPostgreSQLSyntaxNoticeOnParseErrorStdout(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "insert into users(id) values (1) on conflict (id) do nothing;", "--dialect", "mysql", "--fail-on", "notice"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected user-facing parse error exit code 2, got %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, postgreSQLSyntaxNoticeRuleID) {
		t.Fatalf("expected rule id in stdout, got %q", output)
	}
	if !strings.Contains(strings.ToLower(output), "sql looks like postgresql") {
		t.Fatalf("expected PostgreSQL syntax notice in stdout, got %q", output)
	}
	if !strings.Contains(output, "--dialect postgresql") {
		t.Fatalf("expected explicit dialect guidance in stdout, got %q", output)
	}
	if strings.Contains(output, "Verdict: `pass`") {
		t.Fatalf("did not expect parse-error output to report pass, got %q", output)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected parse error on stderr")
	}
	if strings.Contains(strings.ToLower(stderr.String()), "dialect mismatch") {
		t.Fatalf("did not expect legacy dialect mismatch prose on stderr, got %q", stderr.String())
	}
}

func TestAuditCommandParseErrorJSONDoesNotReportPassVerdict(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "insert into users(id) values (1) on conflict (id) do nothing;", "--dialect", "mysql", "--fail-on", "notice", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected user-facing parse error exit code 2, got %d", code)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	if decoded["verdict"] == "pass" {
		t.Fatalf("did not expect parse-error json to report pass, got %#v", decoded)
	}
	findings, ok := decoded["global_findings"].([]any)
	if !ok || len(findings) != 1 {
		t.Fatalf("expected one global finding, got %#v", decoded["global_findings"])
	}
	finding, ok := findings[0].(map[string]any)
	if !ok || finding["rule_id"] != postgreSQLSyntaxNoticeRuleID {
		t.Fatalf("expected PostgreSQL syntax notice finding, got %#v", decoded["global_findings"])
	}
	if stderr.Len() == 0 {
		t.Fatal("expected parse error on stderr")
	}
}

func TestAuditCommandParseErrorNoticeDoesNotImplyDialectSwitch(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "insert into users(id) values (1) on conflict (id) do nothing;"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected parse-error exit code 2, got %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "- Dialect: `mysql` (default)") {
		t.Fatalf("expected mysql default dialect context, got %q", output)
	}
	if !strings.Contains(output, "DeltaScope did not auto-switch dialect") {
		t.Fatalf("expected trust note about no automatic dialect switch, got %q", output)
	}
	if strings.Contains(output, "- Dialect: `postgresql`") {
		t.Fatalf("did not expect markdown output to imply a postgresql switch, got %q", output)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected parse error on stderr")
	}
}

func TestAuditCommandMySQLReturningEmitsUnsupportedNoticeJSON(t *testing.T) {
	t.Parallel()
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "insert into users(id) values (1) returning id;", "--dialect", "mysql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitOK {
		t.Fatalf("expected successful audit exit code 0 for mysql returning, got %d\nstdout=%s", code, stdout.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	findings, ok := decoded["global_findings"].([]any)
	if !ok {
		t.Fatalf("expected global_findings array, got %#v", decoded["global_findings"])
	}
	var hasMySQLReturning, hasPostgreSQL bool
	var notice map[string]any
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch finding["rule_id"] {
		case mysqlReturningUnsupportedNoticeRuleID:
			hasMySQLReturning = true
			notice = finding
		case postgreSQLSyntaxNoticeRuleID:
			hasPostgreSQL = true
		}
	}
	if !hasMySQLReturning {
		t.Fatalf("expected mysql returning unsupported notice, got %#v", findings)
	}
	if hasPostgreSQL {
		t.Fatalf("did not expect postgresql syntax notice for mysql returning, got %#v", findings)
	}
	// no-leak: the finding payload (message + metadata + explanation) must not echo
	// raw SQL, returned column names/expressions, parser fragments, or a severity field.
	rawSQLFragments := []string{"values (1)", "insert into", "returning id"}
	assertNoRawSQLInString := func(label, value string) {
		t.Helper()
		for _, fragment := range rawSQLFragments {
			if strings.Contains(strings.ToLower(value), fragment) {
				t.Fatalf("finding %q leaked raw sql fragment %q: %q", label, fragment, value)
			}
		}
	}
	if msg, ok := notice["message"].(string); ok {
		assertNoRawSQLInString("message", msg)
	}
	if suggestion, ok := notice["suggestion"].(string); ok {
		assertNoRawSQLInString("suggestion", suggestion)
	}
	if explanation, ok := notice["explanation"].(map[string]any); ok {
		for key, value := range explanation {
			if s, ok := value.(string); ok {
				assertNoRawSQLInString("explanation."+key, s)
			}
		}
	}
	if metadata, ok := notice["metadata"].(map[string]any); ok {
		for key, value := range metadata {
			if s, ok := value.(string); ok {
				assertNoRawSQLInString("metadata."+key, s)
			}
		}
	}
	if _, ok := notice["severity"]; ok {
		t.Fatalf("did not expect severity field on mysql returning notice, got %#v", notice)
	}
	_ = stderr
}

func TestAuditCommandShowsPostgreSQLSyntaxNoticeOnStdout(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users alter column score type bigint using abs(score);", "--dialect", "mysql", "--fail-on", "notice"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected parse-error exit code 2 when notice threshold is enabled, got %d", code)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected parse error on stderr")
	}
	output := stdout.String()
	if !strings.Contains(output, postgreSQLSyntaxNoticeRuleID) {
		t.Fatalf("expected rule id in stdout, got %q", output)
	}
	if !strings.Contains(strings.ToLower(output), "sql looks like postgresql") {
		t.Fatalf("expected PostgreSQL syntax notice in stdout, got %q", output)
	}
	if !strings.Contains(output, "ALTER COLUMN TYPE USING") {
		t.Fatalf("expected ALTER COLUMN TYPE USING token in stdout, got %q", output)
	}
	if !strings.Contains(output, "--dialect postgresql") {
		t.Fatalf("expected explicit dialect guidance in stdout, got %q", output)
	}
}

func TestAuditCommandDoesNotShowPostgreSQLSyntaxNoticeForMySQLSQL(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "create table users (id bigint primary key) comment='users';", "--dialect", "mysql", "--fail-on", "notice"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code == 2 {
		t.Fatalf("did not expect parse-error path, stderr=%q", stderr.String())
	}
	if strings.Contains(stdout.String(), postgreSQLSyntaxNoticeRuleID) {
		t.Fatalf("did not expect PostgreSQL syntax notice for normal mysql sql, got %q", stdout.String())
	}
}

func TestAuditCommandDoesNotShowPostgreSQLSyntaxNoticeForTokenInsideStringLiteral(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "select 'returning' as note;", "--dialect", "mysql", "--fail-on", "notice"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code == 2 {
		t.Fatalf("did not expect parse-error exit code 2, stderr=%q", stderr.String())
	}
	output := stdout.String()
	if strings.Contains(output, postgreSQLSyntaxNoticeRuleID) {
		t.Fatalf("did not expect PostgreSQL syntax notice for token inside string literal, got %q", output)
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

func TestAuditCommandQuietIncludesContextSummary(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--quiet"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected blocker exit code 1, got %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "[context] mode=offline dialect=mysql dialect_source=default") {
		t.Fatalf("expected offline context line in quiet output, got %q", output)
	}
	if !strings.Contains(output, "[summary] loaded=") {
		t.Fatalf("expected existing summary line to remain, got %q", output)
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

	t.Setenv("TEST_DB_PASSWORD", "secret")

	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "-h", "127.0.0.1", "-P", "3307", "-u", "root", "--password-env", "TEST_DB_PASSWORD", "-D", "app"},
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
	for _, expected := range []string{"RULE ID", "LEVEL", "DIALECT", "KIND", "CATEGORY", "dml.where.require"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected table output to contain %q, got %q", expected, output)
		}
	}
}

func TestRulesListSupportsKindAndLevelFilters(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"rules", "list", "--kind", "dml", "--level", "blocker"},
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

func TestRulesExplainPrintsDetailSections(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"rules", "explain", "dml.where.require"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "Trigger Example") || !strings.Contains(output, "Default policy") || !strings.Contains(output, "Safe override example") || !strings.Contains(output, "Inspect effective rule status") || !strings.Contains(output, "Suggestion") {
		t.Fatalf("expected detailed rule sections, got %q", output)
	}
}

func TestRulesListSearchMatchesByKeyword(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"rules", "list", "--search", "where"},
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

func TestRootAndAuditHelpAdvertiseGitLabCodeQualityFormat(t *testing.T) {
	for _, args := range [][]string{
		{"--help"},
		{"audit", "--help"},
	} {
		stdout := &strings.Builder{}
		code := Execute(
			context.Background(),
			args,
			strings.NewReader(""),
			stdout,
			&strings.Builder{},
		)

		if code != 0 {
			t.Fatalf("args=%v: expected exit code 0, got %d", args, code)
		}
		if output := stdout.String(); !strings.Contains(output, "gitlab-codequality") {
			t.Fatalf("args=%v: expected help output to advertise gitlab-codequality, got %q", args, output)
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
		"postgresql connections",
		"auto-detects the dialect",
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

func TestAuditCommandOfflineJSONIncludesAuditContext(t *testing.T) {
	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--format", "json"},
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
	if contextValue["mode"] != "offline" || contextValue["dialect"] != "mysql" || contextValue["dialect_source"] != "default" {
		t.Fatalf("expected offline audit context, got %#v", contextValue)
	}
}

func TestAuditCommandRejectsExplicitPostgreSQLMetadataAwareRequestWithCapabilityBoundaryError(t *testing.T) {
	if pgCapabilityBoundaryIsRealParser() {
		t.Skip("skipping: real PG parser available, capability boundary test requires stub build")
	}
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect:  spec.DialectPostgreSQL,
		schemasByTable: map[string][]string{"users": {"public"}},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "insert into users(id) values (1) returning id;", "--host", "127.0.0.1", "--user", "root", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected user error exit code 2, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout result on capability-boundary error, got %q", stdout.String())
	}
	message := stderr.String()
	if !strings.Contains(message, "PG-capable") {
		t.Fatalf("expected capability-boundary wording, got %q", message)
	}
	if strings.Contains(strings.ToLower(message), "resolve schema targets:") {
		t.Fatalf("did not expect metadata parse wrapper wording, got %q", message)
	}
	if strings.Contains(strings.ToLower(message), "possible dialect mismatch") {
		t.Fatalf("did not expect mismatch wording, got %q", message)
	}
	if strings.Contains(strings.ToLower(message), "if you are auditing postgresql") {
		t.Fatalf("did not expect heuristic suggestion wording, got %q", message)
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

func TestRenderJSONResultOmitsAggregateExplanations(t *testing.T) {
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
				Explanation: &rule.FindingExplanation{
					Why:        "The shipped policy requires a predicate.",
					Risk:       "Without a predicate the statement can affect every row.",
					Suggestion: "Add a WHERE clause that narrows the target rows.",
				},
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
	if _, ok := decoded["explanation"]; ok {
		t.Fatalf("did not expect duplicate result explanation, got %#v", decoded["explanation"])
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	if _, ok := statement["explanation"]; ok {
		t.Fatalf("did not expect duplicate statement explanation, got %#v", statement["explanation"])
	}
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) != 1 {
		t.Fatalf("expected one finding, got %#v", statement["findings"])
	}
	finding, ok := findings[0].(map[string]any)
	if !ok {
		t.Fatalf("expected finding object, got %#v", findings[0])
	}
	findingExplanation, ok := finding["explanation"].(map[string]any)
	if !ok || findingExplanation["why"] != "The shipped policy requires a predicate." {
		t.Fatalf("expected finding explanation to remain, got %#v", finding["explanation"])
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

	quietOutput, err := renderResult("markdown", true, result, nil, "")
	if err != nil {
		t.Fatalf("render quiet result: %v", err)
	}
	if string(quietOutput) != "[warning] global.rule: pay attention" {
		t.Fatalf("expected quiet finding output, got %q", string(quietOutput))
	}

	jsonOutput, err := renderResult("json", false, result, &auditRunContext{Mode: "offline", Dialect: "postgresql", DialectSource: "flag"}, "")
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

	markdownOutput, err := renderResult("yaml", false, report.Result{Verdict: report.VerdictPass}, nil, "")
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
		summary   report.Summary
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
		{name: "typed pg capability boundary", err: &appaudit.PostgreSQLCapabilityBoundaryError{Message: "requires PG-capable build"}, want: exitUser},
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

	t.Run("rules explain unknown rule", func(t *testing.T) {
		exitCode := 0
		cmd := newRulesExplainCmd(&exitCode)
		cmd.SetArgs([]string{"made.up.rule"})

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected not found error, got %v", err)
		}
		if exitCode != exitUser {
			t.Fatalf("expected user exit code, got %d", exitCode)
		}
	})

	t.Run("rules list invalid kind", func(t *testing.T) {
		exitCode := 0
		cmd := newRulesListCmd(&exitCode)
		cmd.SetArgs([]string{"--kind", "bad"})

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "kind") {
			t.Fatalf("expected invalid kind error, got %v", err)
		}
		if exitCode != exitUser {
			t.Fatalf("expected user exit code, got %d", exitCode)
		}
	})
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

func TestAuditCommandSupportsGitHubActionsFormat(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--format", "github-actions"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 1 {
		t.Fatalf("expected exit code 1 for blocker findings, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr for github-actions output, got %q", stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "::error") {
		t.Fatalf("expected ::error annotation in github-actions output, got %q", output)
	}
	if !strings.Contains(output, "dml.where.require") {
		t.Fatalf("expected rule ID in github-actions output, got %q", output)
	}
}

// TestAuditGitHubSummaryOutput locks the job-summary contract that
// --format github-summary emits for a blocker finding: the fixed title,
// canonical REJECT verdict, the derived Action Summary with the rule id and
// its explain command, and no raw SQL or severity wording.
func TestAuditGitHubSummaryOutput(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--format", "github-summary", "--fail-on", "none"},
		nil,
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr for github-summary output, got %q", stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"## DeltaScope SQL Review",
		"Verdict: REJECT",
		"## Action Summary",
		"`dml.where.require`",
		"Explain: deltascope rules explain dml.where.require",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected github-summary output to contain %q, got:\n%s", want, output)
		}
	}
	for _, banned := range []string{
		"delete from users",
		"DELETE FROM users",
		"severity",
	} {
		if strings.Contains(output, banned) {
			t.Fatalf("github-summary output must not contain %q, got:\n%s", banned, output)
		}
	}
}

// TestAuditGitHubSummaryCleanOutput locks the clean-result contract for the
// github-summary format: a pass verdict, the "No findings." line, and no
// Action Summary section.
func TestAuditGitHubSummaryCleanOutput(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "SELECT 1", "--format", "github-summary", "--fail-on", "none"},
		nil,
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0 for clean github-summary result, got %d\nstdout=%s", code, stdout.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"Verdict: PASS",
		"No findings.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected clean github-summary output to contain %q, got:\n%s", want, output)
		}
	}
	if strings.Contains(output, "## Action Summary") {
		t.Fatalf("clean github-summary output must not include action summary, got:\n%s", output)
	}
}

// TestRootAndAuditHelpAdvertiseGitHubSummaryFormat verifies that both the root
// and audit help text advertise the github-summary output format.
func TestRootAndAuditHelpAdvertiseGitHubSummaryFormat(t *testing.T) {
	for _, args := range [][]string{
		{"--help"},
		{"audit", "--help"},
	} {
		stdout := &strings.Builder{}
		code := Execute(
			context.Background(),
			args,
			strings.NewReader(""),
			stdout,
			&strings.Builder{},
		)

		if code != 0 {
			t.Fatalf("args=%v: expected exit code 0, got %d", args, code)
		}
		if output := stdout.String(); !strings.Contains(output, "github-summary") {
			t.Fatalf("args=%v: expected help output to advertise github-summary, got %q", args, output)
		}
	}
}

// TestUnsupportedFormatMessageMentionsGitHubSummary verifies that an invalid
// --format value produces a user error whose message lists github-summary.
func TestUnsupportedFormatMessageMentionsGitHubSummary(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--format", "bad-format"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != exitUser {
		t.Fatalf("expected user error exit code %d for bad format, got %d", exitUser, code)
	}
	if !strings.Contains(stderr.String(), "github-summary") {
		t.Fatalf("expected unsupported-format error to mention github-summary, got %q", stderr.String())
	}
}

func TestAuditCommandSupportsSARIFFormat(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--format", "sarif"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 1 {
		t.Fatalf("expected exit code 1 for blocker findings, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr for sarif output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal sarif output: %v\noutput=%s", err, stdout.String())
	}
	if decoded["version"] != "2.1.0" {
		t.Fatalf("expected SARIF version 2.1.0, got %v", decoded["version"])
	}
	runs, ok := decoded["runs"].([]any)
	if !ok || len(runs) == 0 {
		t.Fatalf("expected runs array in SARIF output, got %v", decoded["runs"])
	}
}

func TestAuditCommandSupportsGitLabCodeQualityFormatWithSQL(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--format", "gitlab-codequality", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &issues); err != nil {
		t.Fatalf("unmarshal gitlab-codequality output: %v\noutput=%s", err, stdout.String())
	}
	if len(issues) == 0 {
		t.Fatal("expected at least one issue for DELETE without WHERE")
	}

	issue := issues[0]
	if issue["check_name"] != "dml.where.require" {
		t.Errorf("check_name = %v, want dml.where.require", issue["check_name"])
	}
	loc, ok := issue["location"].(map[string]any)
	if !ok {
		t.Fatal("location is not an object")
	}
	if loc["path"] != "deltascope.sql" {
		t.Errorf("location.path = %v, want deltascope.sql", loc["path"])
	}
	lines, ok := loc["lines"].(map[string]any)
	if !ok {
		t.Fatal("location.lines is not an object")
	}
	if lines["begin"] == nil {
		t.Error("location.lines.begin is missing")
	}
	if issue["severity"] == "" {
		t.Error("severity is empty")
	}
	if issue["fingerprint"] == "" {
		t.Error("fingerprint is empty")
	}
}

func TestAuditCommandGitLabCodeQualityFormatWithFile(t *testing.T) {
	dir := t.TempDir()
	filePath := dir + "/migrations.sql"
	if err := os.WriteFile(filePath, []byte("CREATE TABLE t (id INT);"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--file", filePath, "--format", "gitlab-codequality", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d, stderr=%s", code, stderr.String())
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &issues); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, stdout.String())
	}
	if len(issues) == 0 {
		t.Fatal("expected at least one issue")
	}

	loc := issues[0]["location"].(map[string]any)
	path := loc["path"].(string)
	if path == "" {
		t.Error("location.path should not be empty")
	}
	if path == "deltascope.sql" {
		t.Errorf("location.path should use file path, not synthetic, got %s", path)
	}
}

func TestAuditCommandGitLabCodeQualityRendersDDLFinding(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE TABLE t (id INT);", "--format", "gitlab-codequality", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d, stderr=%s", code, stderr.String())
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &issues); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, stdout.String())
	}
	if len(issues) == 0 {
		t.Fatal("expected at least one DDL issue for bare CREATE TABLE")
	}

	hasDDL := false
	for _, issue := range issues {
		cn := issue["check_name"].(string)
		if strings.HasPrefix(cn, "ddl.") {
			hasDDL = true
			break
		}
	}
	if !hasDDL {
		t.Error("expected at least one issue with check_name starting with ddl.")
	}
}

func TestAuditCommandGitLabCodeQualityRelativeFilePath(t *testing.T) {
	dir := t.TempDir()
	sub := dir + "/migrations"
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	filePath := sub + "/gitlab.sql"
	if err := os.WriteFile(filePath, []byte("CREATE TABLE t (id INT);"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Run from a working directory inside the temp dir so the path is relative.
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--file", "migrations/gitlab.sql", "--format", "gitlab-codequality", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &issues); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(issues) == 0 {
		t.Fatal("expected at least one issue")
	}

	loc := issues[0]["location"].(map[string]any)
	path := loc["path"].(string)
	if path != "migrations/gitlab.sql" {
		t.Errorf("location.path = %q, want migrations/gitlab.sql", path)
	}
}

func TestAuditCommandJSONFormatDoesNotRegress(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--format", "json"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 1 {
		t.Fatalf("expected exit code 1 for blocker findings, got %d", code)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	if decoded["verdict"] != "reject" {
		t.Fatalf("expected verdict reject, got %v", decoded["verdict"])
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

func TestRenderJSONResultIncludesRuleSummary(t *testing.T) {
	output, err := renderJSONResult(report.Result{
		Verdict: report.VerdictPass,
		Summary: report.Summary{Statements: 1},
		RuleSummary: &report.RuleSummary{
			Loaded:     147,
			Applicable: 103,
			Skipped: []rule.SkippedRule{{
				RuleID: "ddl.pg.table.engine.allowlist",
				Reason: rule.SkipReasonDialectMismatch,
			}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("render json: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, string(output))
	}
	summary, ok := decoded["rule_summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected rule_summary object, got %#v", decoded["rule_summary"])
	}
	if loaded, _ := summary["loaded"].(float64); loaded != 147 {
		t.Fatalf("expected loaded=147, got %v", summary["loaded"])
	}
	if applicable, _ := summary["applicable"].(float64); applicable != 103 {
		t.Fatalf("expected applicable=103, got %v", summary["applicable"])
	}
	skipped, ok := summary["skipped"].([]any)
	if !ok || len(skipped) != 1 {
		t.Fatalf("expected 1 skipped rule, got %#v", summary["skipped"])
	}
	first, _ := skipped[0].(map[string]any)
	if first["rule_id"] != "ddl.pg.table.engine.allowlist" {
		t.Fatalf("expected skipped rule_id, got %#v", first)
	}
}

func TestQuietOutputIncludesRuleSummary(t *testing.T) {
	output := renderQuietResult(report.Result{
		Verdict: report.VerdictPass,
		Summary: report.Summary{Statements: 1},
		RuleSummary: &report.RuleSummary{
			Loaded:     147,
			Applicable: 103,
			Skipped: []rule.SkippedRule{{
				RuleID: "ddl.pg.table.engine.allowlist",
				Reason: rule.SkipReasonDialectMismatch,
			}},
		},
	}, nil)

	rendered := string(output)
	if !strings.Contains(rendered, "[summary] loaded=147 applicable=103 skipped=1") {
		t.Fatalf("expected summary line in quiet output, got %q", rendered)
	}
}

func TestQuietOutputWithRuleSummaryButNoFindingsShowsSummary(t *testing.T) {
	output := renderQuietResult(report.Result{
		Verdict: report.VerdictPass,
		Summary: report.Summary{Statements: 1},
		RuleSummary: &report.RuleSummary{
			Loaded:     50,
			Applicable: 50,
		},
	}, nil)

	rendered := string(output)
	if rendered == "pass" {
		t.Fatalf("expected summary line, not just 'pass', got %q", rendered)
	}
	if !strings.Contains(rendered, "[summary] loaded=50 applicable=50 skipped=0") {
		t.Fatalf("expected summary line when no findings but rule summary present, got %q", rendered)
	}
}

func TestQuietOutputKeepsPassWhenContextIsOnlyRenderableContent(t *testing.T) {
	output := renderQuietResult(report.Result{}, &auditRunContext{Mode: "offline", Dialect: "mysql", DialectSource: "default"})

	if string(output) != "pass" {
		t.Fatalf("expected quiet success path to remain pass, got %q", string(output))
	}
}

func TestMarkdownPathWithRuleSummaryDoesNotRegress(t *testing.T) {
	output, err := renderMarkdownResult(report.Result{
		Verdict: report.VerdictReject,
		Summary: report.Summary{Statements: 1, Blockers: 1},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause required",
			}},
		}},
		RuleSummary: &report.RuleSummary{
			Loaded:     10,
			Applicable: 9,
			Skipped: []rule.SkippedRule{{
				RuleID: "ddl.pg.table.engine.allowlist",
				Reason: rule.SkipReasonDialectMismatch,
			}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	rendered := string(output)
	if !strings.Contains(rendered, "# DeltaScope Audit Result") {
		t.Fatalf("expected markdown header, got %q", rendered)
	}
	if !strings.Contains(rendered, "dml.where.require") {
		t.Fatalf("expected finding in markdown, got %q", rendered)
	}
	if !strings.Contains(rendered, "## Rule Summary") {
		t.Fatalf("expected rule summary section, got %q", rendered)
	}
}

func TestAuditCommandDefaultPolicyDialectHygieneMySQLExcludesPostgreSQLRules(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE TABLE smoke_users (id bigint unsigned NOT NULL AUTO_INCREMENT, name varchar(64) NOT NULL DEFAULT '', created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='smoke users';", "--dialect", "mysql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)
	_ = code
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	assertCLIPayloadNoPGRuleIDs(t, decoded)
}

func TestAuditCommandDefaultPolicyDialectHygieneTiDBExcludesPostgreSQLRules(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE TABLE smoke_users (id bigint unsigned NOT NULL AUTO_INCREMENT, name varchar(64) NOT NULL DEFAULT '', created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='smoke users';", "--dialect", "tidb", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)
	_ = code
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	assertCLIPayloadNoPGRuleIDs(t, decoded)
}

func assertCLIPayloadNoPGRuleIDs(t *testing.T, decoded map[string]any) {
	t.Helper()
	statements, ok := decoded["statements"].([]any)
	if !ok {
		return
	}
	for _, rawStmt := range statements {
		stmt, ok := rawStmt.(map[string]any)
		if !ok {
			continue
		}
		findings, ok := stmt["findings"].([]any)
		if !ok {
			continue
		}
		for _, rawFinding := range findings {
			finding, ok := rawFinding.(map[string]any)
			if !ok {
				continue
			}
			if ruleID, _ := finding["rule_id"].(string); strings.HasPrefix(ruleID, "ddl.pg.") {
				t.Errorf("MySQL/TiDB default CLI audit should not emit PG-only rule %q", ruleID)
			}
		}
	}
	globalFindings, ok := decoded["global_findings"].([]any)
	if !ok {
		return
	}
	for _, rawFinding := range globalFindings {
		finding, ok := rawFinding.(map[string]any)
		if !ok {
			continue
		}
		if ruleID, _ := finding["rule_id"].(string); strings.HasPrefix(ruleID, "ddl.pg.") {
			t.Errorf("MySQL/TiDB default CLI audit should not emit PG-only rule %q in global findings", ruleID)
		}
	}
}

const locationFidelityMultiStmtSQL = `create table ok_users (
  id bigint unsigned not null auto_increment comment 'id',
  name varchar(32) not null default '' comment 'name',
  created_at datetime not null default current_timestamp comment 'created',
  updated_at datetime not null default current_timestamp on update current_timestamp comment 'updated',
  primary key (id)
) comment='ok users';

delete from users;`

// TestLocationFidelityGitHubActionsFileAndLine verifies that --format github-actions
// with --file outputs the file path and real statement-start line number.
func TestLocationFidelityGitHubActionsFileAndLine(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "migrations.sql")
	if err := os.WriteFile(sqlPath, []byte(locationFidelityMultiStmtSQL), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--file", sqlPath, "--format", "github-actions", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "dml.where.require") {
		t.Fatalf("expected dml.where.require in output, got: %s", output)
	}

	if !strings.Contains(output, "file="+filepath.ToSlash(sqlPath)) {
		t.Errorf("expected file=%s in annotation, got: %s", filepath.ToSlash(sqlPath), output)
	}
	if !strings.Contains(output, "line=9") {
		t.Errorf("expected line=9 (delete statement start) in annotation, got: %s", output)
	}
}

// TestLocationFidelitySARIFArtifactURIAndLine verifies that --format sarif with
// --file outputs artifactLocation.uri and real statement-start line numbers.
func TestLocationFidelitySARIFArtifactURIAndLine(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "migrations.sql")
	if err := os.WriteFile(sqlPath, []byte(locationFidelityMultiStmtSQL), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--file", sqlPath, "--format", "sarif", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d", code)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal sarif: %v\noutput=%s", err, stdout.String())
	}

	runs, ok := decoded["runs"].([]any)
	if !ok || len(runs) == 0 {
		t.Fatal("expected runs array in SARIF output")
	}

	run, _ := runs[0].(map[string]any)
	results, _ := run["results"].([]any)

	var whereResult map[string]any
	for _, r := range results {
		result, _ := r.(map[string]any)
		if result["ruleId"] == "dml.where.require" {
			whereResult = result
			break
		}
	}
	if whereResult == nil {
		t.Fatal("expected dml.where.require result in SARIF")
	}

	locations, _ := whereResult["locations"].([]any)
	if len(locations) == 0 {
		t.Fatal("expected locations array in dml.where.require result")
	}

	loc, _ := locations[0].(map[string]any)
	phys, _ := loc["physicalLocation"].(map[string]any)
	if phys == nil {
		t.Fatal("expected physicalLocation in SARIF location")
	}

	artifact, _ := phys["artifactLocation"].(map[string]any)
	if artifact == nil {
		t.Fatal("expected artifactLocation in SARIF physicalLocation")
	}
	uri, _ := artifact["uri"].(string)
	if uri == "" {
		t.Error("expected artifactLocation.uri to be populated")
	}

	region, _ := phys["region"].(map[string]any)
	startLine, _ := region["startLine"].(float64)
	if startLine != 9 {
		t.Errorf("expected startLine=9 (delete statement start), got %v", startLine)
	}
}

// TestLocationFidelityGitLabCodeQualityLineReal verifies that --format
// gitlab-codequality with --file preserves location.path and uses real
// statement-start line numbers instead of statementIndex+1 fallback.
func TestLocationFidelityGitLabCodeQualityLineReal(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "migrations.sql")
	if err := os.WriteFile(sqlPath, []byte(locationFidelityMultiStmtSQL), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--file", sqlPath, "--format", "gitlab-codequality", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d", code)
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &issues); err != nil {
		t.Fatalf("unmarshal gitlab-codequality: %v\noutput=%s", err, stdout.String())
	}

	var whereIssue map[string]any
	for _, issue := range issues {
		if issue["check_name"] == "dml.where.require" {
			whereIssue = issue
			break
		}
	}
	if whereIssue == nil {
		t.Fatal("expected dml.where.require issue")
	}

	loc, _ := whereIssue["location"].(map[string]any)
	path, _ := loc["path"].(string)
	lines, _ := loc["lines"].(map[string]any)
	begin := lines["begin"]

	if path == "" {
		t.Fatal("expected location.path to be populated from --file")
	}

	beginFloat, _ := begin.(float64)
	if beginFloat != 9 {
		t.Errorf("expected lines.begin=9 (delete statement start line), got %v", begin)
	}
}

// TestLocationFidelityTiDBGitLabCodeQualityLineReal verifies that explicit
// --dialect tidb with --format gitlab-codequality produces the same correct
// statement-start line numbers as the default MySQL dialect.
func TestLocationFidelityTiDBGitLabCodeQualityLineReal(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "migrations.sql")
	if err := os.WriteFile(sqlPath, []byte(locationFidelityMultiStmtSQL), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--file", sqlPath, "--dialect", "tidb", "--format", "gitlab-codequality", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d", code)
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &issues); err != nil {
		t.Fatalf("unmarshal gitlab-codequality: %v\noutput=%s", err, stdout.String())
	}

	var whereIssue map[string]any
	for _, issue := range issues {
		if issue["check_name"] == "dml.where.require" {
			whereIssue = issue
			break
		}
	}
	if whereIssue == nil {
		t.Fatal("expected dml.where.require issue")
	}

	loc, _ := whereIssue["location"].(map[string]any)
	lines, _ := loc["lines"].(map[string]any)
	begin := lines["begin"]

	beginFloat, _ := begin.(float64)
	if beginFloat != 9 {
		t.Errorf("TiDB: expected lines.begin=9 (delete statement start line), got %v", begin)
	}
}

// TestLocationFidelityTiDBSARIFArtifactURIAndLine verifies that explicit
// --dialect tidb with --format sarif produces correct artifactLocation.uri
// and statement-start line numbers, locking non-default dialect SARIF output.
func TestLocationFidelityTiDBSARIFArtifactURIAndLine(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "migrations.sql")
	if err := os.WriteFile(sqlPath, []byte(locationFidelityMultiStmtSQL), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--dialect", "tidb", "--file", sqlPath, "--format", "sarif", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d", code)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal sarif: %v\noutput=%s", err, stdout.String())
	}

	runs, ok := decoded["runs"].([]any)
	if !ok || len(runs) == 0 {
		t.Fatal("expected runs array in SARIF output")
	}

	run, _ := runs[0].(map[string]any)
	results, _ := run["results"].([]any)

	var whereResult map[string]any
	for _, r := range results {
		result, _ := r.(map[string]any)
		if result["ruleId"] == "dml.where.require" {
			whereResult = result
			break
		}
	}
	if whereResult == nil {
		t.Fatal("expected dml.where.require result in SARIF")
	}

	locations, _ := whereResult["locations"].([]any)
	if len(locations) == 0 {
		t.Fatal("expected locations array in dml.where.require result")
	}

	loc, _ := locations[0].(map[string]any)
	phys, _ := loc["physicalLocation"].(map[string]any)
	if phys == nil {
		t.Fatal("expected physicalLocation in SARIF location")
	}

	artifact, _ := phys["artifactLocation"].(map[string]any)
	if artifact == nil {
		t.Fatal("expected artifactLocation in SARIF physicalLocation")
	}
	uri, _ := artifact["uri"].(string)
	if uri == "" {
		t.Error("expected artifactLocation.uri to be populated from --file")
	}

	region, _ := phys["region"].(map[string]any)
	startLine, _ := region["startLine"].(float64)
	if startLine != 9 {
		t.Errorf("TiDB: expected startLine=9 (delete statement start), got %v", startLine)
	}
}

func TestAuditCommandDatabaseSchemaLifecycleRules(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		dialect    string
		wantRuleID string
	}{
		{
			name:       "mysql_create_database_notice",
			sql:        "CREATE DATABASE app;",
			dialect:    "mysql",
			wantRuleID: "ddl.database.create.notice",
		},
		{
			name:       "mysql_drop_database_warn",
			sql:        "DROP DATABASE app;",
			dialect:    "mysql",
			wantRuleID: "ddl.database.drop.warn",
		},
		{
			name:       "tidb_create_database_notice",
			sql:        "CREATE DATABASE app;",
			dialect:    "tidb",
			wantRuleID: "ddl.database.create.notice",
		},
		{
			name:       "tidb_drop_database_warn",
			sql:        "DROP DATABASE app;",
			dialect:    "tidb",
			wantRuleID: "ddl.database.drop.warn",
		},
		{
			name:       "tidb_create_schema_synonym_notice",
			sql:        "CREATE SCHEMA app;",
			dialect:    "tidb",
			wantRuleID: "ddl.database.create.notice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}
			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", tt.dialect, "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			_ = code

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
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
			if !ok || len(findings) < 1 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}

			found := false
			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if finding["rule_id"] == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected rule_id %q, got %#v", tt.wantRuleID, findings)
			}

			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if !ok {
					continue
				}
				ruleID, _ := finding["rule_id"].(string)
				if strings.HasPrefix(ruleID, "ddl.pg.") {
					t.Fatalf("MySQL/TiDB CLI audit must not emit PG rule %q", ruleID)
				}
			}
		})
	}
}

func TestAuditCommandRejectsRemovedPasswordFlag(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--password", "secret"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 3 {
		t.Fatalf("expected exit code 3 for removed --password flag, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown flag") {
		t.Fatalf("expected unknown flag error, got %q", stderr.String())
	}
}

func TestAuditCommandRejectsUnknownTLSMode(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--tls-mode", "invalid"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected exit code 2 for invalid --tls-mode, got %d", code)
	}
	if !strings.Contains(stderr.String(), "tls-mode") {
		t.Fatalf("expected tls-mode error message, got %q", stderr.String())
	}
}

func TestAuditCommandDefaultsTLSModeToDisabled(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "select 1", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0 for offline audit, got %d: %s", code, stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	contextObj, ok := decoded["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context in JSON output, got %#v", decoded["context"])
	}
	if contextObj["mode"] != "offline" {
		t.Fatalf("expected offline mode (TLS defaults to disabled, no connection), got %#v", contextObj["mode"])
	}
}

func TestAuditCommandRejectsTLSCAFileWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caPath, []byte("not-used"), 0o644); err != nil {
		t.Fatal(err)
	}

	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--tls-ca-file", caPath},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected exit code 2 for --tls-ca-file without --tls-mode=enabled, got %d", code)
	}
	if !strings.Contains(stderr.String(), "tls-ca-file") || !strings.Contains(stderr.String(), "tls-mode=enabled") {
		t.Fatalf("expected tls-ca-file requires tls-mode=enabled error, got %q", stderr.String())
	}
}

func TestAuditCommandRejectsTLSWithoutHost(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--tls-mode", "enabled", "--user", "root"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected exit code 2 for TLS without --host, got %d", code)
	}
	if !strings.Contains(stderr.String(), "tls-mode=enabled") || !strings.Contains(stderr.String(), "--host") {
		t.Fatalf("expected tls-mode=enabled requires --host error, got %q", stderr.String())
	}
}

func TestAuditCommandRejectsTLSWithoutUser(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--tls-mode", "enabled", "--host", "127.0.0.1"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected exit code 2 for TLS without --user, got %d", code)
	}
	if !strings.Contains(stderr.String(), "tls-mode=enabled") || !strings.Contains(stderr.String(), "--user") {
		t.Fatalf("expected tls-mode=enabled requires --user error, got %q", stderr.String())
	}
}

func TestAuditCommandRejectsTLSSocket(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--tls-mode", "enabled", "--host", "127.0.0.1", "--user", "root", "--socket", "/tmp/mysql.sock"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected exit code 2 for TLS with --socket, got %d", code)
	}
	if !strings.Contains(stderr.String(), "socket") {
		t.Fatalf("expected socket conflict error, got %q", stderr.String())
	}
}

func TestAuditCommandLoadsTLSCAFile(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")

	caPEM := generateTestCAPEM(t)
	if err := os.WriteFile(caPath, caPEM, 0o644); err != nil {
		t.Fatal(err)
	}

	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--tls-mode", "enabled", "--host", "127.0.0.1", "--user", "root", "--tls-ca-file", caPath},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected exit code 2 (connection will fail since no real DB), got %d: %s", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "cannot read TLS CA file") {
		t.Fatalf("CA file should have been readable, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "invalid TLS CA certificate") {
		t.Fatalf("CA file should have been valid PEM, got %q", stderr.String())
	}
}

func TestAuditCommandRejectsInvalidCAFile(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(caPath, []byte("not a PEM file"), 0o644); err != nil {
		t.Fatal(err)
	}

	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--tls-mode", "enabled", "--host", "127.0.0.1", "--user", "root", "--tls-ca-file", caPath},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected exit code 2 for invalid CA file, got %d", code)
	}
	if !strings.Contains(stderr.String(), "invalid TLS CA certificate") {
		t.Fatalf("expected invalid TLS CA certificate error, got %q", stderr.String())
	}
}

func TestAuditCommandRejectsMissingCAFile(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--tls-mode", "enabled", "--host", "127.0.0.1", "--user", "root", "--tls-ca-file", "/nonexistent/ca.pem"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 2 {
		t.Fatalf("expected exit code 2 for missing CA file, got %d", code)
	}
	if !strings.Contains(stderr.String(), "cannot read TLS CA file") {
		t.Fatalf("expected cannot read TLS CA file error, got %q", stderr.String())
	}
}

func TestCLIMapsOnlineErrorToBoundedMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{name: "connection refused", input: "dial tcp 127.0.0.1:3306: connection refused", wantMsg: "connection failed"},
		{name: "certificate error", input: "x509: certificate signed by unknown authority", wantMsg: "TLS handshake failed"},
		{name: "timeout", input: "dial tcp 127.0.0.1:3306: i/o timeout", wantMsg: "connection timed out"},
		{name: "context canceled", input: "context canceled", wantMsg: "request canceled"},
		{name: "unknown error", input: "some unexpected driver error with details", wantMsg: "connection failed"},
		{name: "tls handshake failure", input: "tls: handshake failure", wantMsg: "TLS handshake failed"},
		{name: "x509 unknown authority", input: "x509: certificate signed by unknown authority", wantMsg: "TLS handshake failed"},
		{name: "pgpass missing", input: "pgpass file not found", wantMsg: "connection failed"},
		{name: "connection failed", input: "connection failed: host unreachable", wantMsg: "connection failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapOnlineCLIBoundaryError(errors.New(tt.input))
			if err.Error() != tt.wantMsg {
				t.Fatalf("expected %q, got %q", tt.wantMsg, err.Error())
			}
		})
	}
}

func TestAuditOnlineErrorsNeverBreachCLIBoundary(t *testing.T) {
	adversarialInputs := []string{
		"permission denied for table users",
		"unknown database driver error: panic at 0xdeadbeef",
		"pg_catalog read failed: relation does not exist",
		"mysql: 1045 access denied for user 'root'@'localhost'",
		"unexpected notice: server closed the connection unexpectedly",
	}
	for _, input := range adversarialInputs {
		err := mapOnlineCLIBoundaryError(errors.New(input))
		if err == nil || err.Error() == input {
			t.Fatalf("raw error breached CLI boundary: %q", input)
		}
	}
}

func TestAuditMetaConnectionOpenErrorDoesNotLeakSensitiveData(t *testing.T) {
	sensitiveCases := []struct {
		name string
		err  *auditmeta.Error
	}{
		{
			name: "connection_open with DSN",
			err: &auditmeta.Error{
				Kind:    auditmeta.ErrorConnectionOpen,
				Message: "open metadata connection: dial tcp 10.0.0.1:3306: connect: connection refused",
			},
		},
		{
			name: "schema_lookup with table name and driver error",
			err: &auditmeta.Error{
				Kind:    auditmeta.ErrorSchemaLookupFailed,
				Message: `resolve schema for table "users": pq: password authentication failed`,
			},
		},
		{
			name: "dialect_detect with version string",
			err: &auditmeta.Error{
				Kind:    auditmeta.ErrorDialectDetect,
				Message: "detect dialect: unexpected version string 8.0.36-custom",
			},
		},
		{
			name: "invalid_sql with parser error",
			err: &auditmeta.Error{
				Kind:    auditmeta.ErrorInvalidSQL,
				Message: "resolve schema targets: syntax error at position 42",
			},
		},
	}

	sensitiveTokens := []string{
		"10.0.0.1", "3306", "password", "users", "8.0.36", "position 42",
		"dial tcp", "pq:", "connection refused",
	}

	for _, tc := range sensitiveCases {
		t.Run(tc.name, func(t *testing.T) {
			bounded := mapAuditMetaErrorToBounded(tc.err)
			for _, token := range sensitiveTokens {
				if strings.Contains(strings.ToLower(bounded.Error()), strings.ToLower(token)) {
					t.Fatalf("bounded message %q leaked sensitive token %q", bounded.Error(), token)
				}
			}
		})
	}
}

func TestAuditMetaSafeErrorsPassThrough(t *testing.T) {
	dialectMismatch := &auditmeta.Error{
		Kind:    auditmeta.ErrorDialectMismatch,
		Message: `detected dialect "mysql" does not match requested dialect "postgresql"`,
	}
	if !isBoundedApplicationError(dialectMismatch) {
		t.Fatal("dialect mismatch should be bounded")
	}

	schemaHint := &auditmeta.Error{
		Kind:    auditmeta.ErrorSchemaHintRequired,
		Message: "ambiguous schema for table orders",
	}
	if !isBoundedApplicationError(schemaHint) {
		t.Fatal("schema hint required should be bounded")
	}

	connOpen := &auditmeta.Error{
		Kind:    auditmeta.ErrorConnectionOpen,
		Message: "open metadata connection: some driver error",
	}
	if isBoundedApplicationError(connOpen) {
		t.Fatal("connection open should NOT be bounded (must be mapped)")
	}
}

func TestQueryAccessAnalyzeHelpShowsTLSFlags(t *testing.T) {
	stdout := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"query-access", "analyze", "--help"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0 for help, got %d", code)
	}
	help := stdout.String()
	if !strings.Contains(help, "--tls-mode") {
		t.Fatalf("expected --tls-mode in help output, got %q", help)
	}
	if !strings.Contains(help, "--tls-ca-file") {
		t.Fatalf("expected --tls-ca-file in help output, got %q", help)
	}
}

func TestQueryAccessAnalyzeRejectsUnknownTLSMode(t *testing.T) {
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"query-access", "analyze", "--sql", "select 1", "--tls-mode", "badmode"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != 3 {
		t.Fatalf("expected exit code 3 for invalid --tls-mode on query-access, got %d", code)
	}
	if !strings.Contains(stderr.String(), "tls-mode") {
		t.Fatalf("expected tls-mode error message, got %q", stderr.String())
	}
}

func generateTestCAPEM(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}

	var buf strings.Builder
	if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		t.Fatalf("encode CA cert PEM: %v", err)
	}
	return []byte(buf.String())
}

func TestPasswordSourceErrorsAreBounded(t *testing.T) {
	t.Run("unset env var does not leak var name", func(t *testing.T) {
		stderr := &strings.Builder{}
		code := Execute(
			context.Background(),
			[]string{"audit", "--sql", "delete from users", "-u", "root", "--password-env", "MY_SECRET_VAR_NAME"},
			strings.NewReader(""),
			&strings.Builder{},
			stderr,
		)
		if code != 2 {
			t.Fatalf("expected exit code 2, got %d", code)
		}
		if strings.Contains(stderr.String(), "MY_SECRET_VAR_NAME") {
			t.Fatalf("stderr must not leak env var name, got %q", stderr.String())
		}
		if !strings.Contains(stderr.String(), "invalid password source") {
			t.Fatalf("expected bounded error message, got %q", stderr.String())
		}
	})

	t.Run("missing file does not leak path", func(t *testing.T) {
		stderr := &strings.Builder{}
		code := Execute(
			context.Background(),
			[]string{"audit", "--sql", "delete from users", "-u", "root", "--password-file", "/tmp/super-secret-path.txt"},
			strings.NewReader(""),
			&strings.Builder{},
			stderr,
		)
		if code != 2 {
			t.Fatalf("expected exit code 2, got %d", code)
		}
		if strings.Contains(stderr.String(), "/tmp/super-secret-path.txt") {
			t.Fatalf("stderr must not leak file path, got %q", stderr.String())
		}
		if !strings.Contains(stderr.String(), "invalid password source") {
			t.Fatalf("expected bounded error message, got %q", stderr.String())
		}
	})
}

func TestTLSModeIsCaseSensitive(t *testing.T) {
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--tls-mode", "Enabled"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)
	if code != 2 {
		t.Fatalf("expected exit code 2 for case-sensitive tls-mode, got %d", code)
	}
	if !strings.Contains(stderr.String(), "tls-mode") {
		t.Fatalf("expected tls-mode error, got %q", stderr.String())
	}
}

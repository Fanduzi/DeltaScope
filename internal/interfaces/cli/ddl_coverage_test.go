package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const testDDLCoverageCatalogPath = "../../../docs/reference/ddl-coverage-catalog.json"

func TestDDLCoverage_MySQLParserUpgradeCandidate(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"ddl-coverage", "--catalog", testDDLCoverageCatalogPath, "--dialect", "mysql", "--classification", "parser_error", "--guidance-code", "parser_upgrade_candidate"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "ALTER VIEW") {
		t.Error("output should contain ALTER VIEW")
	}
	if !strings.Contains(output, "parser_upgrade_candidate") {
		t.Error("output should contain parser_upgrade_candidate")
	}
}

func TestDDLCoverage_PostgreSQLDropSubscriptionJSON(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"ddl-coverage", "--catalog", testDDLCoverageCatalogPath, "--dialect", "postgresql", "--search", "drop subscription", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}

	var result ddlCoverageJSONOutput
	if err := json.Unmarshal([]byte(stdout.String()), &result); err != nil {
		t.Fatalf("JSON parse error: %v\noutput=%s", err, stdout.String())
	}

	found := false
	for _, e := range result.Entries {
		if e.Form == "DROP SUBSCRIPTION" {
			found = true
			break
		}
	}
	if !found {
		t.Error("entries should contain DROP SUBSCRIPTION")
	}
	if result.Summary.Returned == 0 {
		t.Error("summary.returned should be non-zero")
	}
}

func TestDDLCoverage_TiDBAllEntries(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"ddl-coverage", "--catalog", testDDLCoverageCatalogPath, "--dialect", "tidb", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}

	var result ddlCoverageJSONOutput
	if err := json.Unmarshal([]byte(stdout.String()), &result); err != nil {
		t.Fatalf("JSON parse error: %v\noutput=%s", err, stdout.String())
	}

	if len(result.Entries) == 0 {
		t.Fatal("expected TiDB entries")
	}
	for _, e := range result.Entries {
		if e.Dialect != "tidb" {
			t.Errorf("got dialect %q, want tidb", e.Dialect)
		}
	}
}

func TestDDLCoverage_EmptyResultJSON(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"ddl-coverage", "--catalog", testDDLCoverageCatalogPath, "--search", "definitely-not-present", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}

	var result ddlCoverageJSONOutput
	if err := json.Unmarshal([]byte(stdout.String()), &result); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}
	if len(result.Entries) != 0 {
		t.Errorf("entries should be empty, got %d", len(result.Entries))
	}
}

func TestDDLCoverage_InvalidDialect(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"ddl-coverage", "--catalog", testDDLCoverageCatalogPath, "--dialect", "oracle"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code == 0 {
		t.Fatal("expected non-zero exit for invalid dialect")
	}
	errText := stderr.String()
	if !strings.Contains(errText, "dialect") || !strings.Contains(errText, "oracle") {
		t.Errorf("error should mention dialect and oracle: %s", errText)
	}
}

func TestDDLCoverage_InvalidClassification(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"ddl-coverage", "--catalog", testDDLCoverageCatalogPath, "--classification", "bogus"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code == 0 {
		t.Fatal("expected non-zero exit for invalid classification")
	}
	errText := stderr.String()
	if !strings.Contains(errText, "classification") || !strings.Contains(errText, "bogus") {
		t.Errorf("error should mention classification and bogus: %s", errText)
	}
}

func TestDDLCoverage_InvalidGuidanceCode(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"ddl-coverage", "--catalog", testDDLCoverageCatalogPath, "--guidance-code", "nonexistent"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code == 0 {
		t.Fatal("expected non-zero exit for invalid guidance code")
	}
	errText := stderr.String()
	if !strings.Contains(errText, "guidance") || !strings.Contains(errText, "nonexistent") {
		t.Errorf("error should mention guidance and nonexistent: %s", errText)
	}
}

func TestDDLCoverage_InvalidFormat(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"ddl-coverage", "--catalog", testDDLCoverageCatalogPath, "--format", "xml"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code == 0 {
		t.Fatal("expected non-zero exit for invalid format")
	}
	errText := stderr.String()
	if !strings.Contains(errText, "format") {
		t.Errorf("error should mention format: %s", errText)
	}
}

func TestDDLCoverage_NegativeLimit(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"ddl-coverage", "--catalog", testDDLCoverageCatalogPath, "--limit", "-1"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code == 0 {
		t.Fatal("expected non-zero exit for negative limit")
	}
	errText := stderr.String()
	if !strings.Contains(errText, "limit") {
		t.Errorf("error should mention limit: %s", errText)
	}
}

func TestDDLCoverage_TextOutput(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"ddl-coverage", "--catalog", testDDLCoverageCatalogPath, "--dialect", "mysql", "--classification", "parser_error"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}

	output := stdout.String()
	for _, header := range []string{"DIALECT", "CLASSIFICATION", "FAMILY", "FORM", "GUIDANCE"} {
		if !strings.Contains(output, header) {
			t.Errorf("text output should contain column header %q", header)
		}
	}
	if !strings.Contains(output, "parser_error") {
		t.Error("text output should contain a classification value")
	}
	if !strings.Contains(output, "mysql") {
		t.Error("text output should contain a dialect value")
	}
}

func TestDDLCoverage_JSONNoLeakSanity(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"ddl-coverage", "--catalog", testDDLCoverageCatalogPath, "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}

	output := stdout.String()
	forbidden := []string{
		"near \"",
		"near `",
		"Syntax error",
		"syntax error",
		"$$",
	}
	for _, f := range forbidden {
		if strings.Contains(output, f) {
			t.Errorf("no-leak violation: output contains %q", f)
		}
	}
}

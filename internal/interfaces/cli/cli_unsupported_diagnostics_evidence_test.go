// Package cli verifies CLI unsupported-diagnostics evidence rendering.
// input: parser-error SQL through Execute and diagnostic render helpers
// output: JSON/text/markdown/quiet diagnostic field and no-leak coverage
// pos: interface-layer diagnostics evidence tests
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestUnsupportedDiagnosticsEvidenceCLIParserErrorJSON(t *testing.T) {
	t.Parallel()

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE FUNCTION hello() RETURNS VARCHAR(20) RETURN 'secret_body_value'", "--dialect", "mysql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitUser {
		t.Fatalf("expected exit %d for parser-error SQL, got %d", exitUser, code)
	}

	output := stdout.String()
	combined := strings.ToLower(output + stderr.String())

	if strings.Contains(combined, "secret_body_value") {
		t.Fatalf("CLI JSON output leaked forbidden payload in stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(combined, "near ") {
		t.Fatalf("CLI JSON output leaked raw parser fragment in stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("decode JSON output: %v\noutput was: %s", err, output)
	}

	diagnostics, ok := payload["diagnostics"].([]any)
	if !ok || len(diagnostics) == 0 {
		t.Fatalf("expected non-empty diagnostics array in JSON output, got %#v", payload["diagnostics"])
	}

	first, ok := diagnostics[0].(map[string]any)
	if !ok {
		t.Fatalf("expected diagnostic object, got %T", diagnostics[0])
	}

	classification, _ := first["classification"].(string)
	if classification != "parser_error" {
		t.Fatalf("expected classification parser_error, got %q", classification)
	}

	reason, _ := first["reason"].(string)
	if !strings.Contains(strings.ToLower(reason), "not audited") {
		t.Fatalf("expected reason containing 'not audited', got %q", reason)
	}

	actionHint, _ := first["action_hint"].(string)
	if !strings.Contains(strings.ToLower(actionHint), "verify the selected dialect") {
		t.Fatalf("expected action_hint containing 'verify the selected dialect', got %q", actionHint)
	}

	audited, _ := first["audited"].(bool)
	if audited {
		t.Fatal("expected audited=false in diagnostic")
	}

	dialect, _ := first["dialect"].(string)
	if dialect != "mysql" {
		t.Fatalf("expected dialect mysql, got %q", dialect)
	}

	if _, ok := payload["statements"].([]any); ok && len(payload["statements"].([]any)) != 0 {
		t.Fatalf("parser-error SQL must not produce statement results in JSON")
	}
}

func TestCLIParserErrorJSONPreservesPartialAuditResult(t *testing.T) {
	t.Parallel()

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "ALTER TABLE users ADD COLUMN x INT;\nCREATE INDEX CONCURRENTLY idx_x ON users (x);\nDELETE FROM users;", "--dialect", "mysql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)
	if code != exitUser {
		t.Fatalf("expected exit %d for partial parser-error SQL, got %d", exitUser, code)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &payload); err != nil {
		t.Fatalf("decode JSON output: %v\noutput was: %s", err, stdout.String())
	}
	summary, _ := payload["summary"].(map[string]any)
	if summary["statements"] != float64(2) {
		t.Fatalf("expected two audited statements, got %s", stdout.String())
	}
	statements, _ := payload["statements"].([]any)
	if len(statements) != 2 || !strings.Contains(stdout.String(), "dml.where.require") {
		t.Fatalf("expected valid statements and DELETE finding, got %s", stdout.String())
	}
	diagnostics, _ := payload["diagnostics"].([]any)
	if len(diagnostics) != 1 || diagnostics[0].(map[string]any)["line"] != float64(2) {
		t.Fatalf("expected one line-2 parser diagnostic, got %s", stdout.String())
	}
	if strings.Contains(stdout.String()+stderr.String(), "idx_x") {
		t.Fatalf("CLI diagnostic output leaked invalid SQL text: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestUnsupportedDiagnosticsEvidenceCLIParserErrorText(t *testing.T) {
	t.Parallel()

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE FUNCTION hello() RETURNS VARCHAR(20) RETURN 'secret_body_value'", "--dialect", "mysql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitUser {
		t.Fatalf("expected exit %d for parser-error SQL, got %d", exitUser, code)
	}

	combined := strings.ToLower(stdout.String() + stderr.String())

	if strings.Contains(combined, "secret_body_value") {
		t.Fatalf("CLI text output leaked forbidden payload in stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(combined, "near ") {
		t.Fatalf("CLI text output leaked raw parser fragment in stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	if !strings.Contains(combined, "parser_error") {
		t.Fatalf("expected 'parser_error' classification in text output, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(combined, "action_hint") || !strings.Contains(combined, "not audited") {
		t.Fatalf("expected action_hint and 'not audited' in text output, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(combined, "guidance_code") {
		t.Fatalf("expected 'guidance_code' in text output for parser-upgrade candidate, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(combined, "parser_upgrade_candidate") {
		t.Fatalf("expected 'parser_upgrade_candidate' guidance_code value in text output, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(combined, "evidence_ref") {
		t.Fatalf("expected 'evidence_ref' in text output for parser-upgrade candidate, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(combined, "https://github.com/fanduzi/deltascope/") {
		t.Fatalf("expected GitHub evidence_ref URL in text output, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestUnsupportedDiagnosticsGuidanceMarkdownRender(t *testing.T) {
	t.Parallel()

	output, err := renderMarkdownResult(report.Result{
		Verdict: report.VerdictPass,
		Diagnostics: []spec.Diagnostic{
			{
				Classification: "parser_error",
				Reason:         "statement was not audited",
				ActionHint:     "verify the selected dialect",
				Audited:        false,
				Dialect:        "mysql",
				GuidanceCode:   "parser_upgrade_candidate",
				EvidenceRef:    "https://github.com/Fanduzi/DeltaScope/blob/main/docs/reference/cli.md#parser-upgrade-candidate-evidence-v02500",
				Line:           2,
				Column:         3,
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	rendered := string(output)
	if !strings.Contains(rendered, "guidance_code: parser_upgrade_candidate") {
		t.Fatalf("expected guidance_code in markdown diagnostics, got %q", rendered)
	}
	if !strings.Contains(rendered, "evidence_ref: https://github.com/Fanduzi/DeltaScope/") {
		t.Fatalf("expected evidence_ref URL in markdown diagnostics, got %q", rendered)
	}
	if !strings.Contains(rendered, "line: 2") || !strings.Contains(rendered, "column: 3") {
		t.Fatalf("expected diagnostic location in markdown output, got %q", rendered)
	}
}

func TestUnsupportedDiagnosticsGuidanceQuietRender(t *testing.T) {
	t.Parallel()

	output := renderQuietResult(report.Result{
		Verdict: report.VerdictPass,
		Diagnostics: []spec.Diagnostic{
			{
				Classification: "parser_error",
				Reason:         "statement was not audited",
				ActionHint:     "verify the selected dialect",
				Audited:        false,
				Dialect:        "mysql",
				GuidanceCode:   "parser_upgrade_candidate",
				EvidenceRef:    "https://github.com/Fanduzi/DeltaScope/blob/main/docs/reference/cli.md#parser-upgrade-candidate-evidence-v02500",
				Line:           2,
				Column:         3,
			},
		},
	}, nil)

	rendered := string(output)
	if !strings.Contains(rendered, "guidance_code=parser_upgrade_candidate") {
		t.Fatalf("expected guidance_code in quiet output, got %q", rendered)
	}
	if !strings.Contains(rendered, "evidence_ref=https://github.com/Fanduzi/DeltaScope/") {
		t.Fatalf("expected evidence_ref URL in quiet output, got %q", rendered)
	}
	if !strings.Contains(rendered, "line=2") || !strings.Contains(rendered, "column=3") {
		t.Fatalf("expected diagnostic location in quiet output, got %q", rendered)
	}
}

func TestUnsupportedDiagnosticsNoGuidanceOmitsFields(t *testing.T) {
	t.Parallel()

	output := renderQuietResult(report.Result{
		Verdict: report.VerdictPass,
		Diagnostics: []spec.Diagnostic{
			{
				Classification: "parser_error",
				Reason:         "statement was not audited",
				ActionHint:     "verify the selected dialect",
				Audited:        false,
				Dialect:        "mysql",
			},
		},
	}, nil)

	rendered := string(output)
	if strings.Contains(rendered, "guidance_code=") {
		t.Fatalf("expected no guidance_code for diagnostic without guidance, got %q", rendered)
	}
	if strings.Contains(rendered, "evidence_ref=") {
		t.Fatalf("expected no evidence_ref for diagnostic without guidance, got %q", rendered)
	}
}

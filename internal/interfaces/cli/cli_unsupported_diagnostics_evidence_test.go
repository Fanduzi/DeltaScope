package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
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

	if code == 0 {
		t.Fatalf("expected non-zero exit code for parser-error SQL, got %d", code)
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

	if code == 0 {
		t.Fatalf("expected non-zero exit code for parser-error SQL, got %d", code)
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
}

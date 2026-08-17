// Package cli verifies CLI process exit mapping for bad user input.
// input: Execute args for unknown flags and unparseable SQL
// output: exit-code, diagnostic classification, and no-leak coverage
// pos: interface-layer contract tests for issue #24
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditUnknownFlagSqllExitsUser(t *testing.T) {
	t.Parallel()

	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sqll", "delete from users"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != exitUser {
		t.Fatalf("expected exit %d for unknown flag --sqll, got %d (stderr=%q)", exitUser, code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unknown flag: --sqll") {
		t.Fatalf("expected unknown flag --sqll on stderr, got %q", stderr.String())
	}
}

func TestAuditUnparseableSQLJSONExitsUser(t *testing.T) {
	t.Parallel()

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "this is not sql !!!", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitUser {
		t.Fatalf("expected exit %d for unparseable SQL, got %d (stderr=%q)", exitUser, code, stderr.String())
	}

	combined := strings.ToLower(stdout.String() + stderr.String())
	if strings.Contains(combined, "this is not sql") {
		t.Fatalf("CLI leaked SQL payload in stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(combined, "near ") {
		t.Fatalf("CLI leaked raw parser fragment in stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &payload); err != nil {
		t.Fatalf("decode JSON output: %v\nstdout was: %s", err, stdout.String())
	}

	verdict, _ := payload["verdict"].(string)
	if verdict != "" {
		t.Fatalf("parser-error JSON verdict must stay empty, got %q", verdict)
	}

	diagnostics, ok := payload["diagnostics"].([]any)
	if !ok || len(diagnostics) == 0 {
		t.Fatalf("expected parser-error diagnostics, got %#v", payload["diagnostics"])
	}
	first, ok := diagnostics[0].(map[string]any)
	if !ok {
		t.Fatalf("expected diagnostic object, got %T", diagnostics[0])
	}
	classification, _ := first["classification"].(string)
	if classification != "parser_error" {
		t.Fatalf("expected classification parser_error, got %q", classification)
	}
}

func TestAuditUnparseableSQLMarkdownExitsUser(t *testing.T) {
	t.Parallel()

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "this is not sql !!!"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitUser {
		t.Fatalf("expected exit %d for unparseable SQL, got %d (stderr=%q)", exitUser, code, stderr.String())
	}

	combined := strings.ToLower(stdout.String() + stderr.String())
	if strings.Contains(combined, "this is not sql") {
		t.Fatalf("CLI leaked SQL payload in stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(combined, "near ") {
		t.Fatalf("CLI leaked raw parser fragment in stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(combined, "parser_error") && !strings.Contains(combined, "not audited") {
		t.Fatalf("expected parser-error diagnostic, stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestAuditExistingUserErrorsStayExitUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "invalid format", args: []string{"audit", "--sql", "delete from users", "--format", "nope"}},
		{name: "unsupported dialect", args: []string{"audit", "--sql", "delete from users", "--dialect", "oracle"}},
		{name: "missing file", args: []string{"audit", "--file", "/missing.sql"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stderr := &strings.Builder{}
			code := Execute(context.Background(), tc.args, strings.NewReader(""), &strings.Builder{}, stderr)
			if code != exitUser {
				t.Fatalf("expected exit %d for %s, got %d (stderr=%q)", exitUser, tc.name, code, stderr.String())
			}
		})
	}
}

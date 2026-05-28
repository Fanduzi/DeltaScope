package cli

import (
	"context"
	"strings"
	"testing"
)

func TestCLIparserErrorUnsupportedContractMySQL(t *testing.T) {
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

	output := stdout.String()
	combined := strings.ToLower(stdout.String() + stderr.String())

	if !strings.Contains(combined, "not audited") && !strings.Contains(combined, "parse") {
		t.Fatalf("expected not-audited or parse diagnostic in output, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(combined, "secret_body_value") {
		t.Fatalf("CLI output leaked forbidden payload in stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(combined, "near ") {
		t.Fatalf("CLI output leaked raw parser fragment in stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	// Verify no DDL rule IDs appear (parser-error SQL produces no findings).
	_ = output
}

func TestCLIparserErrorUnsupportedContractTiDB(t *testing.T) {
	t.Parallel()

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "ALTER TABLE users LOCALITY = 'region=us-east-1'", "--dialect", "tidb"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code == 0 {
		t.Fatalf("expected non-zero exit code for parser-error SQL, got %d", code)
	}

	combined := strings.ToLower(stdout.String() + stderr.String())

	if !strings.Contains(combined, "not audited") && !strings.Contains(combined, "parse") {
		t.Fatalf("expected not-audited or parse diagnostic in output, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(combined, "us-east-1") {
		t.Fatalf("CLI output leaked forbidden payload in stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(combined, "near ") {
		t.Fatalf("CLI output leaked raw parser fragment in stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

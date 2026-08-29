//go:build postgresql

// Package cli verifies PostgreSQL offline impact rendering.
// input: CLI audit arguments selecting PostgreSQL JSON output
// output: public impact fields rendered in the CLI result
// pos: PostgreSQL-tagged CLI adapter regression coverage
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditCommandPostgreSQLOfflineIDEqualityImpact(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users where id = 42", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d\nstdout=%q\nstderr=%q", exitOK, code, stdout.String(), stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("decode JSON: %v\noutput=%s", err, stdout.String())
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
	if !ok {
		t.Fatalf("expected impact object, got %#v", statement["impact"])
	}
	if impact["estimated_rows"] != float64(1) || impact["risk_level"] != "low" || impact["confidence"] != "high" || impact["source"] != "shape" {
		t.Fatalf("unexpected impact object: %#v", impact)
	}
	reasonCodes, ok := impact["reason_codes"].([]any)
	if !ok || len(reasonCodes) != 1 || reasonCodes[0] != "pk_equality" {
		t.Fatalf("reason codes = %#v, want [pk_equality]", impact["reason_codes"])
	}
}

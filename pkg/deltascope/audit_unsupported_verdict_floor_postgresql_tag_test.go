//go:build postgresql

// Package deltascope verifies public SDK unsupported-statement result contracts.
// input: public Audit requests containing structured unsupported PostgreSQL statements
// output: SDK errors that preserve the review-floored unsupported result contract
// pos: public package regression coverage for the unsupported-statement verdict floor
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAuditUnsupportedStatementFloorsPassVerdictToReview(t *testing.T) {
	t.Parallel()

	result, err := Audit(context.Background(), Request{
		SQL:     "SELECT 1",
		Dialect: DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if result.Verdict != VerdictReview {
		t.Fatalf("expected unsupported completeness floor to review, got %q", result.Verdict)
	}
	if len(result.Statements) != 0 || result.Summary.Statements != 0 {
		t.Fatalf("expected zero audited statements, got statements=%#v summary=%+v", result.Statements, result.Summary)
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Feature != "select" {
		t.Fatalf("expected one select unsupported detail, got %#v", result.Unsupported)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Classification != "unsupported_statement" || result.Diagnostics[0].Audited {
		t.Fatalf("expected one unaudited unsupported diagnostic, got %#v", result.Diagnostics)
	}
	combined := result.Diagnostics[0].Reason + result.Diagnostics[0].ActionHint + err.Error()
	if strings.Contains(combined, "SELECT 1") {
		t.Fatalf("SDK diagnostic leaked SQL text: %q", combined)
	}
}

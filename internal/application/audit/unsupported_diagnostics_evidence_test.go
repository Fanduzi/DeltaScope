package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestParserErrorDiagnosticEvidence(t *testing.T) {
	t.Parallel()

	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE FUNCTION hello() RETURNS VARCHAR(20) RETURN 'secret_body_value'",
		Dialect: spec.DialectMySQL,
	})
	if err == nil {
		t.Fatal("expected parser-error diagnostic")
	}
	if len(result.Statements) != 0 {
		t.Fatalf("expected no statement results, got %d", len(result.Statements))
	}
	if len(result.GlobalFindings) != 0 {
		t.Fatalf("expected no findings, got %#v", result.GlobalFindings)
	}

	diagnostics := result.Diagnostics
	if len(diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %d: %+v", len(diagnostics), diagnostics)
	}
	got := diagnostics[0]
	if got.Classification != "parser_error" {
		t.Fatalf("classification: expected parser_error, got %q", got.Classification)
	}
	if got.Audited {
		t.Fatal("parser-error diagnostic must mark audited=false")
	}
	if !strings.Contains(got.Reason, "not audited") {
		t.Fatalf("expected not-audited reason, got %q", got.Reason)
	}
	if !strings.Contains(got.ActionHint, "verify the selected dialect") {
		t.Fatalf("expected action hint containing 'verify the selected dialect', got %q", got.ActionHint)
	}
	if got.Dialect != "mysql" {
		t.Fatalf("expected dialect mysql, got %q", got.Dialect)
	}
	combined := got.Reason + got.ActionHint + err.Error()
	if strings.Contains(combined, "secret_body_value") {
		t.Fatal("diagnostic leaked function body")
	}
}

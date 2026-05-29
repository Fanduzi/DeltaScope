package deltascope

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestUnsupportedDiagnosticsEvidenceSDKParserError(t *testing.T) {
	t.Parallel()

	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE FUNCTION hello() RETURNS VARCHAR(20) RETURN 'secret_body_value'",
		Dialect: DialectMySQL,
	})
	if err == nil {
		t.Fatalf("expected parser-error diagnostic, got nil error and result=%#v", result)
	}

	if len(result.Diagnostics) == 0 {
		t.Fatalf("expected at least one diagnostic, got none")
	}

	var pe *spec.Diagnostic
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Classification == "parser_error" {
			pe = &result.Diagnostics[i]
			break
		}
	}
	if pe == nil {
		t.Fatalf("expected parser_error diagnostic, got %#v", result.Diagnostics)
	}

	if !strings.Contains(pe.Reason, "not audited") {
		t.Fatalf("expected reason containing 'not audited', got %q", pe.Reason)
	}
	if !strings.Contains(pe.ActionHint, "verify the selected dialect") {
		t.Fatalf("expected action_hint containing 'verify the selected dialect', got %q", pe.ActionHint)
	}
	if pe.Audited {
		t.Fatal("parser-error diagnostic must mark audited=false")
	}
	if pe.Dialect != "mysql" {
		t.Fatalf("expected dialect mysql, got %q", pe.Dialect)
	}

	if len(result.Statements) != 0 {
		t.Fatalf("parser-error SQL must not produce statement results: %#v", result.Statements)
	}
	if len(result.GlobalFindings) != 0 {
		t.Fatalf("parser-error SQL must not produce global findings: %#v", result.GlobalFindings)
	}

	combined := pe.Reason + pe.ActionHint + pe.Dialect + err.Error()
	if strings.Contains(combined, "secret_body_value") {
		t.Fatalf("SDK diagnostic leaked forbidden payload: %q", combined)
	}
	if strings.Contains(strings.ToLower(combined), "near ") {
		t.Fatalf("SDK diagnostic leaked raw parser fragment: %q", combined)
	}

	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "not audited") {
		t.Fatalf("v0.220.0 error contract: expected 'not audited', got %q", err.Error())
	}
}

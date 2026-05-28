package deltascope

import (
	"context"
	"strings"
	"testing"
)

func TestAuditParserErrorUnsupportedContractMySQL(t *testing.T) {
	t.Parallel()

	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE FUNCTION hello() RETURNS VARCHAR(20) RETURN 'secret_body_value'",
		Dialect: DialectMySQL,
	})
	if err == nil {
		t.Fatalf("expected parser-error diagnostic, got nil error and result=%#v", result)
	}
	if len(result.Statements) != 0 {
		t.Fatalf("parser-error SQL must not produce statement results: %#v", result.Statements)
	}
	if len(result.GlobalFindings) != 0 {
		t.Fatalf("parser-error SQL must not produce global findings: %#v", result.GlobalFindings)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("parser-error SQL must not fabricate unsupported details: %#v", result.Unsupported)
	}

	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "not audited") {
		t.Fatalf("expected error to mention %q, got %q", "not audited", err.Error())
	}
	if !strings.Contains(msg, "audit") {
		t.Fatalf("expected error to mention %q, got %q", "audit", err.Error())
	}
	if strings.Contains(err.Error(), "secret_body_value") {
		t.Fatalf("parser-error diagnostic leaked forbidden payload in %q", err.Error())
	}
}

func TestAuditParserErrorUnsupportedContractTiDB(t *testing.T) {
	t.Parallel()

	result, err := Audit(context.Background(), Request{
		SQL:     "ALTER TABLE users LOCALITY = 'region=us-east-1'",
		Dialect: DialectTiDB,
	})
	if err == nil {
		t.Fatalf("expected parser-error diagnostic, got nil error and result=%#v", result)
	}
	if len(result.Statements) != 0 {
		t.Fatalf("parser-error SQL must not produce statement results: %#v", result.Statements)
	}
	if len(result.GlobalFindings) != 0 {
		t.Fatalf("parser-error SQL must not produce global findings: %#v", result.GlobalFindings)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("parser-error SQL must not fabricate unsupported details: %#v", result.Unsupported)
	}

	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "not audited") {
		t.Fatalf("expected error to mention %q, got %q", "not audited", err.Error())
	}
	if !strings.Contains(msg, "audit") {
		t.Fatalf("expected error to mention %q, got %q", "audit", err.Error())
	}
	if strings.Contains(err.Error(), "us-east-1") {
		t.Fatalf("parser-error diagnostic leaked forbidden payload in %q", err.Error())
	}
}

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
		SQL:     "CREATE FUNCTION hello() RETURNS VARCHAR(20) BEGIN SELECT 'secret_body_value'",
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

// The ENGINE-led ALTER TABLESPACE form graduated out of the MySQL
// parser-upgrade candidate list (T57/#140): the bounded tablespace
// recognizer consumes `ALTER TABLESPACE name ENGINE [=] engine`, so the
// statement is audited — never a parser-error diagnostic.
func TestMySQLAlterTablespaceEngineGraduated(t *testing.T) {
	t.Parallel()

	result, err := AuditSQL(context.Background(), Request{
		SQL:     "ALTER TABLESPACE ts1 ENGINE=INNODB",
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("engine-led ALTER TABLESPACE must audit, got error: %v", err)
	}
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Classification == "parser_error" {
			t.Fatalf("graduated form must not emit parser_error, got %+v", result.Diagnostics[i])
		}
	}
	var sawWarn bool
	for _, st := range result.Statements {
		for _, f := range st.Findings {
			if f.RuleID == "ddl.mysql.tablespace.alter.warn" {
				sawWarn = true
			}
		}
	}
	if !sawWarn {
		t.Fatalf("expected ddl.mysql.tablespace.alter.warn finding, got %+v", result.Statements)
	}
}

func diagnosticClassifications(ds []spec.Diagnostic) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Classification
	}
	return out
}

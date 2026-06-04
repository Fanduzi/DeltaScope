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

func TestParserUpgradeCandidateGuidanceCodeMySQL(t *testing.T) {
	t.Parallel()

	result, err := AuditSQL(context.Background(), Request{
		SQL:     "ALTER VIEW v_users AS SELECT id, name FROM users",
		Dialect: spec.DialectMySQL,
	})
	if err == nil {
		t.Fatal("expected parser-error diagnostic for ALTER VIEW")
	}

	diagnostics := result.Diagnostics
	if len(diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic")
	}

	var pe *spec.Diagnostic
	for i := range diagnostics {
		if diagnostics[i].Classification == "parser_error" {
			pe = &diagnostics[i]
			break
		}
	}
	if pe == nil {
		t.Fatalf("expected parser_error diagnostic, got classifications: %+v", diagnosticClassifications(diagnostics))
	}

	if pe.Audited {
		t.Fatal("parser-error diagnostic must mark audited=false")
	}
	if pe.Dialect != "mysql" {
		t.Fatalf("expected dialect mysql, got %q", pe.Dialect)
	}

	if pe.GuidanceCode != "parser_upgrade_candidate" {
		t.Fatalf("expected guidance_code parser_upgrade_candidate, got %q", pe.GuidanceCode)
	}

	const expectedRef = "https://github.com/Fanduzi/DeltaScope/blob/main/docs/reference/cli.md#parser-upgrade-candidate-evidence-v02500"
	if pe.EvidenceRef != expectedRef {
		t.Fatalf("expected evidence_ref %q, got %q", expectedRef, pe.EvidenceRef)
	}
	if !strings.HasPrefix(pe.EvidenceRef, "https://github.com/Fanduzi/DeltaScope/") {
		t.Fatalf("evidence_ref must start with GitHub base URL, got %q", pe.EvidenceRef)
	}

	if strings.Contains(pe.EvidenceRef, "v_users") {
		t.Fatal("evidence_ref must not contain raw SQL object names")
	}
	if strings.Contains(pe.GuidanceCode+pe.EvidenceRef+pe.Reason+pe.ActionHint, "v_users") {
		t.Fatal("diagnostic fields must not contain raw SQL object names")
	}
	if strings.Contains(strings.ToLower(pe.GuidanceCode+pe.EvidenceRef+pe.Reason+pe.ActionHint+err.Error()), "near ") {
		t.Fatal("diagnostic must not contain raw parser near-text fragments")
	}
}

func diagnosticClassifications(ds []spec.Diagnostic) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Classification
	}
	return out
}

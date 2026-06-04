//go:build postgresql

package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestUnsupportedStatementDiagnosticEvidencePostgreSQL(t *testing.T) {
	t.Parallel()

	result, err := AuditSQL(context.Background(), Request{
		SQL:     "alter table users rename column old_name to new_name; select 1;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err == nil {
		t.Fatal("expected unsupported statement error")
	}
	if len(result.Unsupported) == 0 {
		t.Fatalf("expected unsupported details, got result=%#v", result)
	}

	if len(result.Diagnostics) == 0 {
		t.Fatalf("expected unsupported diagnostic evidence, got result.Diagnostics empty")
	}
	found := false
	for _, d := range result.Diagnostics {
		if d.Classification == "unsupported_statement" {
			found = true
			if d.Audited {
				t.Fatal("unsupported_statement diagnostic must mark audited=false")
			}
			if !strings.Contains(d.ActionHint, "review it manually") {
				t.Fatalf("expected manual-review action hint, got %q", d.ActionHint)
			}
			if d.Dialect != "postgresql" {
				t.Fatalf("expected dialect postgresql, got %q", d.Dialect)
			}
			if strings.Contains(d.Reason+d.ActionHint, "old_name") {
				t.Fatal("unsupported diagnostic leaked raw SQL column name")
			}
		}
	}
	if !found {
		t.Fatalf("expected unsupported_statement diagnostic, got classifications: %+v",
			classificationsOf(result.Diagnostics))
	}
}

func TestParserUpgradeCandidateGuidanceCodePostgreSQL(t *testing.T) {
	t.Parallel()

	result, err := AuditSQL(context.Background(), Request{
		SQL:     "DROP SUBSCRIPTION sub1 WITH (drop_slot = true)",
		Dialect: spec.DialectPostgreSQL,
	})
	if err == nil {
		t.Fatal("expected parser-error diagnostic for DROP SUBSCRIPTION")
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
		t.Fatalf("expected parser_error diagnostic, got classifications: %+v", classificationsOf(diagnostics))
	}

	if pe.Audited {
		t.Fatal("parser-error diagnostic must mark audited=false")
	}
	if pe.Dialect != "postgresql" {
		t.Fatalf("expected dialect postgresql, got %q", pe.Dialect)
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

	if strings.Contains(pe.EvidenceRef, "sub1") {
		t.Fatal("evidence_ref must not contain raw SQL object names")
	}
	if strings.Contains(pe.GuidanceCode+pe.EvidenceRef+pe.Reason+pe.ActionHint, "sub1") {
		t.Fatal("diagnostic fields must not contain raw SQL object names")
	}
	if strings.Contains(strings.ToLower(pe.GuidanceCode+pe.EvidenceRef+pe.Reason+pe.ActionHint+err.Error()), "near ") {
		t.Fatal("diagnostic must not contain raw parser near-text fragments")
	}
}

func classificationsOf(ds []spec.Diagnostic) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Classification
	}
	return out
}

//go:build postgresql

package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestParserUpgradeCandidateGuidanceCoveragePostgreSQL(t *testing.T) {
	t.Parallel()

	const expectedRef = "https://github.com/Fanduzi/DeltaScope/blob/main/docs/reference/cli.md#parser-upgrade-candidate-evidence-v02500"

	tests := []struct {
		name           string
		sql            string
		forbiddenNames []string
	}{
		{
			name:           "PostgreSQL DROP SUBSCRIPTION WITH",
			sql:            "DROP SUBSCRIPTION sub1 WITH (drop_slot = true)",
			forbiddenNames: []string{"sub1"},
		},
		{
			name:           "PostgreSQL NOT NULL NOT VALID",
			sql:            "ALTER TABLE accounts ALTER COLUMN email SET NOT NULL NOT VALID",
			forbiddenNames: []string{"accounts_email_key"},
		},
		{
			name:           "PostgreSQL ALTER CONSTRAINT NOT ENFORCED",
			sql:            "ALTER TABLE accounts ALTER CONSTRAINT accounts_email_key NOT ENFORCED",
			forbiddenNames: []string{"accounts_email_key"},
		},
		{
			name:           "PostgreSQL ALTER CONSTRAINT INHERIT",
			sql:            "ALTER TABLE accounts ALTER CONSTRAINT accounts_email_key INHERIT",
			forbiddenNames: []string{"accounts_email_key"},
		},
		{
			name:           "PostgreSQL ALTER CONSTRAINT NO INHERIT",
			sql:            "ALTER TABLE accounts ALTER CONSTRAINT accounts_email_key NO INHERIT",
			forbiddenNames: []string{"accounts_email_key"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := AuditSQL(context.Background(), Request{
				SQL:     tc.sql,
				Dialect: spec.DialectPostgreSQL,
			})
			if err == nil {
				t.Fatal("expected parser-error diagnostic")
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
				t.Error("parser-error diagnostic must mark audited=false")
			}
			if pe.Dialect != "postgresql" {
				t.Errorf("expected dialect postgresql, got %q", pe.Dialect)
			}
			if pe.GuidanceCode != "parser_upgrade_candidate" {
				t.Errorf("expected guidance_code parser_upgrade_candidate, got %q", pe.GuidanceCode)
			}
			if pe.EvidenceRef != expectedRef {
				t.Errorf("expected evidence_ref %q, got %q", expectedRef, pe.EvidenceRef)
			}

			combined := pe.GuidanceCode + pe.EvidenceRef + pe.Reason + pe.ActionHint
			for _, name := range tc.forbiddenNames {
				if strings.Contains(combined, name) {
					t.Errorf("diagnostic fields must not contain raw SQL object name %q", name)
				}
			}

			if strings.Contains(strings.ToLower(combined+err.Error()), "near ") {
				t.Error("diagnostic must not contain raw parser near-text fragments")
			}
		})
	}
}

package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestParserUpgradeCandidateGuidanceCoverage(t *testing.T) {
	t.Parallel()

	const expectedRef = "https://github.com/Fanduzi/DeltaScope/blob/main/docs/reference/cli.md#parser-upgrade-candidate-evidence-v02500"

	tests := []struct {
		name           string
		sql            string
		dialect        spec.Dialect
		forbiddenNames []string
	}{
		// MySQL candidates (5)
		{
			name:           "MySQL ALTER VIEW",
			sql:            "ALTER VIEW v_users AS SELECT id, name FROM users",
			dialect:        spec.DialectMySQL,
			forbiddenNames: []string{"v_users"},
		},
		{
			name:           "MySQL ALTER PROCEDURE",
			sql:            "ALTER PROCEDURE p1 COMMENT 'x'",
			dialect:        spec.DialectMySQL,
			forbiddenNames: []string{"p1"},
		},
		{
			name:           "MySQL CREATE FUNCTION",
			sql:            "CREATE FUNCTION f1() RETURNS INT RETURN 1",
			dialect:        spec.DialectMySQL,
			forbiddenNames: []string{"f1"},
		},
		{
			name:           "MySQL ALTER FUNCTION",
			sql:            "ALTER FUNCTION f1 COMMENT 'x'",
			dialect:        spec.DialectMySQL,
			forbiddenNames: []string{"f1"},
		},
		{
			name:           "MySQL DROP FUNCTION",
			sql:            "DROP FUNCTION f1",
			dialect:        spec.DialectMySQL,
			forbiddenNames: []string{"f1"},
		},
		// PostgreSQL candidates (5) are in the postgresql-tag counterpart.
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := AuditSQL(context.Background(), Request{
				SQL:     tc.sql,
				Dialect: tc.dialect,
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
			if pe.Dialect != string(tc.dialect) {
				t.Errorf("expected dialect %q, got %q", tc.dialect, pe.Dialect)
			}
			if pe.GuidanceCode != "parser_upgrade_candidate" {
				t.Errorf("expected guidance_code parser_upgrade_candidate, got %q", pe.GuidanceCode)
			}
			if pe.EvidenceRef != expectedRef {
				t.Errorf("expected evidence_ref %q, got %q", expectedRef, pe.EvidenceRef)
			}

			// No-leak: raw object names must not appear in any diagnostic field.
			combined := pe.GuidanceCode + pe.EvidenceRef + pe.Reason + pe.ActionHint
			for _, name := range tc.forbiddenNames {
				if strings.Contains(combined, name) {
					t.Errorf("diagnostic fields must not contain raw SQL object name %q", name)
				}
			}

			// No-leak: parser near-text must not appear.
			if strings.Contains(strings.ToLower(combined+err.Error()), "near ") {
				t.Error("diagnostic must not contain raw parser near-text fragments")
			}
		})
	}
}

func TestParserUpgradeCandidateNegativeCases(t *testing.T) {
	t.Parallel()

	t.Run("garbage SQL is not parser_upgrade_candidate", func(t *testing.T) {
		t.Parallel()

		result, err := AuditSQL(context.Background(), Request{
			SQL:     "GARBAGE NOT SQL @#$%",
			Dialect: spec.DialectMySQL,
		})
		if err == nil {
			t.Fatal("expected parser-error diagnostic")
		}

		if len(result.Diagnostics) == 0 {
			t.Fatal("expected at least one diagnostic")
		}

		var pe *spec.Diagnostic
		for i := range result.Diagnostics {
			if result.Diagnostics[i].Classification == "parser_error" {
				pe = &result.Diagnostics[i]
				break
			}
		}
		if pe == nil {
			t.Fatalf("expected parser_error diagnostic, got classifications: %+v", diagnosticClassifications(result.Diagnostics))
		}

		if pe.GuidanceCode == "parser_upgrade_candidate" {
			t.Error("garbage SQL must not be classified as parser_upgrade_candidate")
		}
		if pe.EvidenceRef != "" {
			t.Errorf("garbage SQL must not have evidence_ref, got %q", pe.EvidenceRef)
		}
	})

	t.Run("non-candidate parser_error has no guidance", func(t *testing.T) {
		t.Parallel()

		// CREATE TRIGGER is a known parser_error but NOT in the parser_upgrade_candidate list.
		result, err := AuditSQL(context.Background(), Request{
			SQL:     "CREATE TRIGGER trg_users_bi BEFORE INSERT ON users FOR EACH ROW SET NEW.created_at = NOW()",
			Dialect: spec.DialectMySQL,
		})
		if err == nil {
			t.Fatal("expected parser-error diagnostic")
		}

		if len(result.Diagnostics) == 0 {
			t.Fatal("expected at least one diagnostic")
		}

		var pe *spec.Diagnostic
		for i := range result.Diagnostics {
			if result.Diagnostics[i].Classification == "parser_error" {
				pe = &result.Diagnostics[i]
				break
			}
		}
		if pe == nil {
			t.Fatalf("expected parser_error diagnostic, got classifications: %+v", diagnosticClassifications(result.Diagnostics))
		}

		if pe.GuidanceCode == "parser_upgrade_candidate" {
			t.Error("non-candidate parser_error must not be classified as parser_upgrade_candidate")
		}
		if pe.EvidenceRef != "" {
			t.Errorf("non-candidate parser_error must not have evidence_ref, got %q", pe.EvidenceRef)
		}
	})
}

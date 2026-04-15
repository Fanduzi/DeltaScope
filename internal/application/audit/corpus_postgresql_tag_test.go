//go:build postgresql

package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// TestSQLCorpusPostgreSQL runs every .expected.yaml under
// testdata/sql-corpus/postgresql through the full audit pipeline and
// asserts expected behaviour.
func TestSQLCorpusPostgreSQL(t *testing.T) {
	corpusRoot := filepath.Join("..", "..", "..", "testdata", "sql-corpus", "postgresql")

	entries, err := corpusExpectedFiles(corpusRoot)
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no .expected.yaml files found under testdata/sql-corpus/postgresql")
	}

	for _, expPath := range entries {
		rel, _ := filepath.Rel(corpusRoot, expPath)
		t.Run(rel, func(t *testing.T) {
			// Read sibling .sql.
			sqlPath := expPath[:len(expPath)-len(".expected.yaml")] + ".sql"
			sqlBytes, err := os.ReadFile(sqlPath)
			if err != nil {
				t.Fatalf("read sql: %v", err)
			}

			// Parse expected YAML.
			raw, err := os.ReadFile(expPath)
			if err != nil {
				t.Fatalf("read expected: %v", err)
			}
			var tc corpusExpected
			if err := yaml.Unmarshal(raw, &tc); err != nil {
				t.Fatalf("parse expected yaml: %v", err)
			}

			// Run audit pipeline.
			result, auditErr := AuditSQL(context.Background(), Request{
				SQL:     string(sqlBytes),
				Dialect: spec.DialectPostgreSQL,
			})

			// Determine whether parsing succeeded.
			// parse_ok=true means the statement was parsed; unsupported entries are okay.
			// parse_ok=false means a hard parse error.
			isUnsupportedErr := auditErr != nil && errors.Is(auditErr, ErrUnsupportedStatement)
			if tc.Expect.ParseOK != nil {
				if *tc.Expect.ParseOK {
					if auditErr != nil && !isUnsupportedErr {
						t.Fatalf("expected parse_ok=true, got non-unsupported error: %v", auditErr)
					}
				} else {
					if auditErr == nil || isUnsupportedErr {
						t.Fatal("expected parse_ok=false, got nil error or unsupported-only error")
					}
					return
				}
			} else if auditErr != nil && !isUnsupportedErr {
				t.Fatalf("audit error: %v", auditErr)
			}

			// Assert unsupported.count.
			if tc.Expect.Unsupported != nil && tc.Expect.Unsupported.Count != nil {
				got := len(result.Unsupported)
				if got != *tc.Expect.Unsupported.Count {
					t.Errorf("unsupported.count: expected %d, got %d (details: %+v)", *tc.Expect.Unsupported.Count, got, result.Unsupported)
				}
			}

			// Assert unsupported.include.
			if tc.Expect.Unsupported != nil {
				for _, feat := range tc.Expect.Unsupported.Include {
					found := false
					for _, u := range result.Unsupported {
						if u.Feature == feat {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("unsupported.include: expected feature %q not found (actual: %+v)", feat, unsupportedFeatures(result.Unsupported))
					}

				// Assert unsupported.metadata.
				if tc.Expect.Unsupported != nil && len(tc.Expect.Unsupported.Metadata) > 0 {
					corpusAssertUnsupportedMetadata(t, result.Unsupported, tc.Expect.Unsupported.Metadata)
				}
				}
			}

			// Assert statement_kind.
			if tc.Expect.StatementKind != "" {
				if len(result.Statements) == 0 {
					t.Fatal("expected statements but got none")
				}
				got := result.Statements[0].Kind
				if got != tc.Expect.StatementKind {
					t.Errorf("statement_kind: expected %q, got %q", tc.Expect.StatementKind, got)
				}
			}

			// Collect all finding rule IDs across statements + global.
			allRuleIDs := make(map[string]struct{})
			for _, sr := range result.Statements {
				for _, f := range sr.Findings {
					allRuleIDs[f.RuleID] = struct{}{}
				}
			}
			for _, f := range result.GlobalFindings {
				allRuleIDs[f.RuleID] = struct{}{}
			}

			// Assert findings.include.
			if tc.Expect.Findings != nil {
				for _, id := range tc.Expect.Findings.Include {
					if _, found := allRuleIDs[id]; !found {
						t.Errorf("findings.include: expected rule %q present, not found (present: %v)", id, sortedKeys(allRuleIDs))
					}
				}
				// Assert findings.exclude.
				for _, id := range tc.Expect.Findings.Exclude {
					if _, found := allRuleIDs[id]; found {
						t.Errorf("findings.exclude: expected rule %q absent, but found", id)
					}
				}
			}

			// Semantic assertions: operation and facts via parse/extract path.
			if len(result.Statements) > 0 {
				if tc.Expect.Operation != "" || (tc.Facts != nil && len(tc.Facts.Constraints) > 0) {
					corpusAssertSemantic(t, string(sqlBytes), spec.DialectPostgreSQL, tc)
				}
			}

			t.Logf("OK: %s (%s/%s) verdict=%s findings=%v unsupported=%d audit_err=%v",
				tc.Name, tc.Dialect, tc.Category, result.Verdict, sortedKeys(allRuleIDs), len(result.Unsupported), auditErr)
		})
	}
}

// unsupportedFeatures returns a summary of unsupported detail features for error messages.
func unsupportedFeatures(details []spec.UnsupportedDetail) []string {
	features := make([]string, 0, len(details))
	for _, d := range details {
		features = append(features, d.Feature)
	}
	return features
}

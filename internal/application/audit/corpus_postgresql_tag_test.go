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

			// Assert facts.constraints if present in expected.
			if tc.Facts != nil && len(tc.Facts.Constraints) > 0 {
				// report.Result does not expose DDL constraints directly.
				// Use parse+extract to get spec.Statement for fact checking.
				assertConstraintFacts(t, string(sqlBytes), tc.Facts.Constraints)
			}

			t.Logf("OK: %s (%s/%s) verdict=%s findings=%v unsupported=%d audit_err=%v",
				tc.Name, tc.Dialect, tc.Category, result.Verdict, sortedKeys(allRuleIDs), len(result.Unsupported), auditErr)
		})
	}
}

// assertConstraintFacts parses and extracts the SQL to verify expected constraint facts.
// This is test-only — it uses the internal parse+extract path to access spec.Statement.DDL.Constraints,
// which report.Result does not expose.
func assertConstraintFacts(t *testing.T, sql string, expected []corpusFactConstraint) {
	t.Helper()

	parsed, err := Parse(sql, spec.DialectPostgreSQL)
	if err != nil {
		t.Fatalf("constraint facts: parse failed: %v", err)
	}
	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("constraint facts: extract failed: %v", err)
	}

	// Filter to supported statements only.
	var supported *spec.Statement
	for i := range statements {
		if statements[i].Unsupported == nil {
			supported = &statements[i]
			break
		}
	}
	if supported == nil || supported.DDL == nil {
		t.Fatal("constraint facts: no supported DDL statement found")
	}

	actualConstraints := supported.DDL.Constraints
	for _, exp := range expected {
		found := false
		for _, actual := range actualConstraints {
			if actual.Type != exp.Type {
				continue
			}
			if exp.Type == "foreign_key" {
				if actual.ReferencedTable == exp.ReferencedTable &&
					stringSlicesEqual(actual.ReferencedColumns, exp.ReferencedColumns) {
					found = true
					break
				}
			} else {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("facts.constraints: expected %s constraint with referenced_table=%q referenced_columns=%v, not found in %+v",
				exp.Type, exp.ReferencedTable, exp.ReferencedColumns, actualConstraints)
		}
	}
}

// stringSlicesEqual compares two string slices for equality, ignoring order.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int)
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
		if seen[s] < 0 {
			return false
		}
	}
	return true
}

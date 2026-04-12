package audit

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// corpusDialectMap translates expected YAML dialect strings to spec.Dialect values.
var corpusDialectMap = map[string]spec.Dialect{
	"mysql": spec.DialectMySQL,
	"tidb":  spec.DialectTiDB,
}

// corpusDialects lists the dialects this runner exercises.
var corpusDialects = []string{"mysql", "tidb"}

// corpusWalkDialects walks only the dialect subdirectories listed in corpusDialects.
func corpusWalkDialects(corpusRoot string) ([]string, error) {
	var files []string
	for _, dialect := range corpusDialects {
		dialectDir := filepath.Join(corpusRoot, dialect)
		if _, err := os.Stat(dialectDir); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(dialectDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".expected.yaml") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func TestSQLCorpusMySQLAndTiDB(t *testing.T) {
	corpusRoot := filepath.Join("..", "..", "..", "testdata", "sql-corpus")

	entries, err := corpusWalkDialects(corpusRoot)
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no .expected.yaml files found for mysql/tidb")
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

			// Resolve dialect.
			dialect, ok := corpusDialectMap[tc.Dialect]
			if !ok {
				t.Fatalf("dialect %q not supported by corpus runner", tc.Dialect)
			}

			// Run audit pipeline.
			result, err := AuditSQL(context.Background(), Request{
				SQL:     string(sqlBytes),
				Dialect: dialect,
			})

			// Assert parse_ok.
			if tc.Expect.ParseOK != nil {
				if *tc.Expect.ParseOK {
					if err != nil {
						t.Fatalf("expected parse_ok=true, got error: %v", err)
					}
				} else {
					if err == nil {
						t.Fatal("expected parse_ok=false, got nil error")
					}
					return // no further assertions on parse failure
				}
			} else if err != nil {
				t.Fatalf("audit error: %v", err)
			}

			// Assert unsupported.count.
			if tc.Expect.Unsupported != nil && tc.Expect.Unsupported.Count != nil {
				got := len(result.Unsupported)
				if got != *tc.Expect.Unsupported.Count {
					t.Errorf("unsupported.count: expected %d, got %d", *tc.Expect.Unsupported.Count, got)
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
					corpusAssertSemantic(t, string(sqlBytes), dialect, tc)
				}
			}

			t.Logf("OK: %s (%s/%s) verdict=%s findings=%v unsupported=%d",
				tc.Name, tc.Dialect, tc.Category, result.Verdict, sortedKeys(allRuleIDs), len(result.Unsupported))
		})
	}
}

func sortedKeys(m map[string]struct{}) []string {
	k := make([]string, 0, len(m))
	for key := range m {
		k = append(k, key)
	}
	sort.Strings(k)
	return k
}

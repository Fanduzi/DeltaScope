package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// corpusExpected is the schema each .expected.yaml file must satisfy.
type corpusExpected struct {
	Name     string `yaml:"name"`
	Dialect  string `yaml:"dialect"`
	Category string `yaml:"category"`
	Expect   struct {
		ParseOK        *bool    `yaml:"parse_ok"`
		StatementKind  string   `yaml:"statement_kind"`
		Operation      string   `yaml:"operation"`
		Unsupported    *struct {
			Count *int `yaml:"count"`
		} `yaml:"unsupported"`
		Findings *struct {
			Include []string `yaml:"include"`
			Exclude []string `yaml:"exclude"`
		} `yaml:"findings"`
	} `yaml:"expect"`
	Facts *corpusFacts `yaml:"facts"`
}

// corpusFacts carries expected structural facts for deeper assertions.
type corpusFacts struct {
	Constraints []corpusFactConstraint `yaml:"constraints"`
}

// corpusFactConstraint is one expected constraint fact.
type corpusFactConstraint struct {
	Type              string   `yaml:"type"`
	ReferencedTable   string   `yaml:"referenced_table,omitempty"`
	ReferencedColumns []string `yaml:"referenced_columns,omitempty"`
}

var validDialects = map[string]bool{
	"mysql":      true,
	"tidb":       true,
	"postgresql": true,
}

var validCategories = map[string]bool{
	"ddl": true,
	"dml": true,
}

// corpusExpectedFiles walks corpusRoot recursively and returns all
// .expected.yaml paths.
func corpusExpectedFiles(corpusRoot string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(corpusRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".expected.yaml") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func TestSQLCorpusExpectedFilesAreWellFormed(t *testing.T) {
	corpusRoot := filepath.Join("..", "..", "..", "testdata", "sql-corpus")

	entries, err := corpusExpectedFiles(corpusRoot)
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no .expected.yaml files found under testdata/sql-corpus; the corpus loader has nothing to validate")
	}

	for _, expPath := range entries {
		name := filepath.Base(expPath)
		t.Run(name, func(t *testing.T) {
			// 1. Sibling .sql must exist.
			sqlPath := expPath[:len(expPath)-len(".expected.yaml")] + ".sql"
			if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
				t.Fatalf("missing sibling .sql file: %s", sqlPath)
			}

			// 2. Parse the YAML.
			raw, err := os.ReadFile(expPath)
			if err != nil {
				t.Fatalf("read error: %v", err)
			}
			var tc corpusExpected
			if err := yaml.Unmarshal(raw, &tc); err != nil {
				t.Fatalf("YAML parse error: %v", err)
			}

			// 3. Validate required top-level fields.
			if tc.Name == "" {
				t.Fatal("missing required field: name")
			}
			if !validDialects[tc.Dialect] {
				t.Fatalf("invalid dialect %q; must be one of mysql, tidb, postgresql", tc.Dialect)
			}
			if !validCategories[tc.Category] {
				t.Fatalf("invalid category %q; must be one of ddl, dml", tc.Category)
			}

			// 4. Validate unsupported.count if present.
			if tc.Expect.Unsupported != nil && tc.Expect.Unsupported.Count != nil {
				if *tc.Expect.Unsupported.Count < 0 {
					t.Fatalf("unsupported.count must be >= 0, got %d", *tc.Expect.Unsupported.Count)
				}
			}

			// 5. Validate findings lists if present.
			if tc.Expect.Findings != nil {
				for _, id := range tc.Expect.Findings.Include {
					if id == "" {
						t.Fatal("findings.include must not contain empty strings")
					}
				}
				for _, id := range tc.Expect.Findings.Exclude {
					if id == "" {
						t.Fatal("findings.exclude must not contain empty strings")
					}
				}
			}

			// 6. Validate facts.constraints if present.
			if tc.Facts != nil {
				for _, c := range tc.Facts.Constraints {
					if c.Type == "" {
						t.Fatal("facts.constraints[].type must not be empty")
					}
				}
			}

			t.Logf("OK: %s (%s/%s)", tc.Name, tc.Dialect, tc.Category)
		})
	}
}

// TestSQLCorpusWalkerFindsActualCases is a sanity check that the walker
// discovers the sample fixture added in Task 1.
func TestSQLCorpusWalkerFindsActualCases(t *testing.T) {
	corpusRoot := filepath.Join("..", "..", "..", "testdata", "sql-corpus")
	matches, err := corpusExpectedFiles(corpusRoot)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal(fmt.Sprintf("walk found no .expected.yaml files under %s; check directory layout", corpusRoot))
	}
	t.Logf("found %d expected file(s)", len(matches))
}

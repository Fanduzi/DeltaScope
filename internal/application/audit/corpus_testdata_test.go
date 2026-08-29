// Package audit defines and validates shared SQL corpus expectations.
// input: expected YAML files and application audit results
// output: validated corpus schemas plus reusable partial-result assertions
// pos: shared corpus test-data contract for tagged and untagged audit runners
// note: if this file changes, update this header and module README.md.
package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
)

// corpusExpected is the schema each .expected.yaml file must satisfy.
type corpusExpected struct {
	Name     string `yaml:"name"`
	Dialect  string `yaml:"dialect"`
	Category string `yaml:"category"`
	Expect   struct {
		ParseOK       *bool  `yaml:"parse_ok"`
		StatementKind string `yaml:"statement_kind"`
		Operation     string `yaml:"operation"`
		Unsupported   *struct {
			Count    *int                `yaml:"count"`
			Include  []string            `yaml:"include,omitempty"`
			Metadata []corpusUnsupported `yaml:"metadata,omitempty"`
		} `yaml:"unsupported"`
		Findings *struct {
			Include []string `yaml:"include"`
			Exclude []string `yaml:"exclude"`
		} `yaml:"findings"`
		Statements *struct {
			Count *int `yaml:"count"`
		} `yaml:"statements"`
		Diagnostics *struct {
			ParserErrorCount *int  `yaml:"parser_error_count"`
			Lines            []int `yaml:"lines"`
			Columns          []int `yaml:"columns"`
		} `yaml:"diagnostics"`
	} `yaml:"expect"`
	Facts    *corpusFacts    `yaml:"facts"`
	Config   map[string]any  `yaml:"config,omitempty"`
	Metadata *corpusMetadata `yaml:"metadata,omitempty"`
}

func corpusHasPartialAssertions(tc corpusExpected) bool {
	return tc.Expect.Statements != nil || tc.Expect.Diagnostics != nil
}

func corpusAssertPartialResult(t *testing.T, result report.Result, tc corpusExpected) {
	t.Helper()
	if tc.Expect.Statements != nil && tc.Expect.Statements.Count != nil && len(result.Statements) != *tc.Expect.Statements.Count {
		t.Errorf("statements.count: expected %d, got %d", *tc.Expect.Statements.Count, len(result.Statements))
	}
	if tc.Expect.Diagnostics == nil {
		return
	}
	parserDiagnostics := make([]struct{ line, column int }, 0)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Classification == "parser_error" {
			parserDiagnostics = append(parserDiagnostics, struct{ line, column int }{diagnostic.Line, diagnostic.Column})
		}
	}
	if tc.Expect.Diagnostics.ParserErrorCount != nil && len(parserDiagnostics) != *tc.Expect.Diagnostics.ParserErrorCount {
		t.Errorf("diagnostics.parser_error_count: expected %d, got %d", *tc.Expect.Diagnostics.ParserErrorCount, len(parserDiagnostics))
	}
	for i, line := range tc.Expect.Diagnostics.Lines {
		if i >= len(parserDiagnostics) || parserDiagnostics[i].line != line {
			t.Errorf("diagnostics.lines[%d]: expected %d, got %+v", i, line, parserDiagnostics)
		}
	}
	for i, column := range tc.Expect.Diagnostics.Columns {
		if i >= len(parserDiagnostics) || parserDiagnostics[i].column != column {
			t.Errorf("diagnostics.columns[%d]: expected %d, got %+v", i, column, parserDiagnostics)
		}
	}
}

// corpusFacts carries expected structural facts for deeper assertions.
type corpusFacts struct {
	Constraints []corpusFactConstraint `yaml:"constraints"`
}

// corpusMetadata carries optional metadata-aware audit fixtures.
type corpusMetadata struct {
	Schema      string                         `yaml:"schema,omitempty"`
	Instance    *corpusInstanceFacts           `yaml:"instance,omitempty"`
	Tables      map[string]corpusTableSnapshot `yaml:"tables,omitempty"`
	IndexOwners map[string]string              `yaml:"index_owners,omitempty"`
}

type corpusInstanceFacts struct {
	Version                   string `yaml:"version,omitempty"`
	DefaultCharset            string `yaml:"default_charset,omitempty"`
	InnoDBLargePrefixEnabled  bool   `yaml:"innodb_large_prefix_enabled,omitempty"`
	InnoDBDefaultRowFormat    string `yaml:"innodb_default_row_format,omitempty"`
	InnoDBAdaptiveHashEnabled bool   `yaml:"innodb_adaptive_hash_enabled,omitempty"`
}

type corpusTableSnapshot struct {
	Schema      string             `yaml:"schema,omitempty"`
	Exists      bool               `yaml:"exists"`
	Table       *corpusTable       `yaml:"table,omitempty"`
	Columns     []corpusColumn     `yaml:"columns,omitempty"`
	PrimaryKey  *corpusIndex       `yaml:"primary_key,omitempty"`
	Indexes     []corpusIndex      `yaml:"indexes,omitempty"`
	Constraints []corpusConstraint `yaml:"constraints,omitempty"`
	Options     map[string]string  `yaml:"options,omitempty"`
}

type corpusTable struct {
	Schema  string `yaml:"schema,omitempty"`
	Name    string `yaml:"name"`
	Comment string `yaml:"comment,omitempty"`
}

type corpusColumn struct {
	Name                      string         `yaml:"name"`
	Type                      string         `yaml:"type,omitempty"`
	Length                    int            `yaml:"length,omitempty"`
	Charset                   string         `yaml:"charset,omitempty"`
	Collation                 string         `yaml:"collation,omitempty"`
	Comment                   string         `yaml:"comment,omitempty"`
	Unsigned                  bool           `yaml:"unsigned,omitempty"`
	NotNull                   bool           `yaml:"not_null,omitempty"`
	AutoIncrement             bool           `yaml:"auto_increment,omitempty"`
	HasDefault                bool           `yaml:"has_default,omitempty"`
	DefaultValue              string         `yaml:"default_value,omitempty"`
	DefaultIsNull             bool           `yaml:"default_is_null,omitempty"`
	DefaultIsCurrentTimestamp bool           `yaml:"default_is_current_timestamp,omitempty"`
	OnUpdateCurrentTimestamp  bool           `yaml:"on_update_current_timestamp,omitempty"`
	GeneratedWhen             string         `yaml:"generated_when,omitempty"`
	IsIdentity                bool           `yaml:"is_identity,omitempty"`
	IdentityOptions           map[string]any `yaml:"identity_options,omitempty"`
}

type corpusIndex struct {
	Name        string   `yaml:"name"`
	Kind        string   `yaml:"kind,omitempty"`
	Columns     []string `yaml:"columns,omitempty"`
	Cardinality *int64   `yaml:"cardinality,omitempty"`
}

type corpusConstraint struct {
	Type              string   `yaml:"type"`
	Name              string   `yaml:"name,omitempty"`
	Columns           []string `yaml:"columns,omitempty"`
	ReferencedSchema  string   `yaml:"referenced_schema,omitempty"`
	ReferencedTable   string   `yaml:"referenced_table,omitempty"`
	ReferencedColumns []string `yaml:"referenced_columns,omitempty"`
}

// corpusFactConstraint is one expected constraint fact.
type corpusFactConstraint struct {
	Type              string   `yaml:"type"`
	Name              string   `yaml:"name,omitempty"`
	Columns           []string `yaml:"columns,omitempty"`
	ReferencedSchema  string   `yaml:"referenced_schema,omitempty"`
	ReferencedTable   string   `yaml:"referenced_table,omitempty"`
	ReferencedColumns []string `yaml:"referenced_columns,omitempty"`
}

// corpusUnsupported is one expected unsupported-detail metadata entry.
type corpusUnsupported struct {
	Feature  string         `yaml:"feature"`
	Metadata map[string]any `yaml:"metadata,omitempty"`
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

var validStatementKinds = map[string]bool{
	"ddl":     true,
	"dml":     true,
	"unknown": true,
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
	t.Parallel()
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
			t.Parallel()
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

			// 4. Validate unsupported fields if present.
			if tc.Expect.Unsupported != nil {
				if tc.Expect.Unsupported.Count != nil && *tc.Expect.Unsupported.Count < 0 {
					t.Fatalf("unsupported.count must be >= 0, got %d", *tc.Expect.Unsupported.Count)
				}
				for _, feat := range tc.Expect.Unsupported.Include {
					if feat == "" {
						t.Fatal("unsupported.include must not contain empty strings")
					}
				}
				for _, m := range tc.Expect.Unsupported.Metadata {
					if m.Feature == "" {
						t.Fatal("unsupported.metadata[].feature must not be empty")
					}
				}
			}
			if tc.Expect.Statements != nil && tc.Expect.Statements.Count != nil && *tc.Expect.Statements.Count < 0 {
				t.Fatalf("statements.count must be >= 0, got %d", *tc.Expect.Statements.Count)
			}
			if tc.Expect.Diagnostics != nil && tc.Expect.Diagnostics.ParserErrorCount != nil && *tc.Expect.Diagnostics.ParserErrorCount < 0 {
				t.Fatalf("diagnostics.parser_error_count must be >= 0, got %d", *tc.Expect.Diagnostics.ParserErrorCount)
			}

			// 5. Validate operation if present — must not be empty.
			if tc.Expect.Operation != "" { //nolint:staticcheck // non-empty is valid
				// non-empty is valid
			}

			// 6. Validate statement_kind if present — must be a known enum.
			if tc.Expect.StatementKind != "" {
				if !validStatementKinds[tc.Expect.StatementKind] {
					t.Fatalf("invalid statement_kind %q; must be one of ddl, dml, unknown", tc.Expect.StatementKind)
				}
			}

			// 7. Validate findings lists if present.
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

			// 8. Validate facts.constraints if present.
			if tc.Facts != nil {
				for _, c := range tc.Facts.Constraints {
					if c.Type == "" {
						t.Fatal("facts.constraints[].type must not be empty")
					}
					for _, col := range c.Columns {
						if col == "" {
							t.Fatal("facts.constraints[].columns must not contain empty strings")
						}
					}
					for _, col := range c.ReferencedColumns {
						if col == "" {
							t.Fatal("facts.constraints[].referenced_columns must not contain empty strings")
						}
					}
				}
			}

			// 9. Validate optional inline config shape if present.
			if tc.Config != nil {
				if _, ok := tc.Config["rules"]; !ok {
					t.Fatal("config must contain rules when present")
				}
			}

			// 10. Validate optional metadata fixture shape if present.
			if tc.Metadata != nil {
				for tableName := range tc.Metadata.Tables {
					if tableName == "" {
						t.Fatal("metadata.tables must not contain an empty table key")
					}
				}
				for indexName, tableName := range tc.Metadata.IndexOwners {
					if indexName == "" || tableName == "" {
						t.Fatal("metadata.index_owners must not contain empty index or table names")
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
	t.Parallel()
	corpusRoot := filepath.Join("..", "..", "..", "testdata", "sql-corpus")
	matches, err := corpusExpectedFiles(corpusRoot)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("walk found no .expected.yaml files under %s; check directory layout", corpusRoot)
	}
	t.Logf("found %d expected file(s)", len(matches))
}

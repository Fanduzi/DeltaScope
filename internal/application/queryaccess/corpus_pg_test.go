//go:build postgresql

package queryaccess_test

import (
	"path/filepath"
	"testing"
)

func TestQueryAccessCorpusPostgreSQL(t *testing.T) {
	t.Parallel()
	corpusRoot := filepath.Join("..", "..", "..", "testdata", "query-access")

	entries, err := queryAccessWalkDialects(corpusRoot, []string{"postgresql"})
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no .expected.yaml files found for postgresql query access corpus")
	}

	for _, expPath := range entries {
		rel, _ := filepath.Rel(corpusRoot, expPath)
		t.Run(rel, func(t *testing.T) {
			t.Parallel()
			runQueryAccessCorpusCase(t, expPath)
		})
	}
}

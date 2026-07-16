// Package tidbparser verifies dependency completeness for common pure-effect holders.
// input: derived queries, ordering/grouping/join expressions, and window clauses
// output: fail-closed classifications, bounded function reasons, and semantic usages
// pos: parser-level regression coverage; no admission promotion
package tidbparser

import (
	"context"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestDependencyCompleteness_TiDB_PropagatesDerivedFunctionEffects(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}

	facts, err := extractor.ExtractQueryAccess(context.Background(),
		"SELECT id FROM (SELECT COUNT(amount) AS c, id FROM orders GROUP BY id) t",
		"mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want indeterminate", facts.ReadClassification)
	}
	if !hasParserReason(facts.ReasonCodes, "function_call") {
		t.Errorf("reason codes: got %v, want function_call", facts.ReasonCodes)
	}
}

func TestDependencyCompleteness_TiDB_InspectsFunctionEffectsInHolders(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sql  string
	}{
		{name: "order_by_count", sql: "SELECT id FROM users ORDER BY COUNT(id)"},
		{name: "order_by_now", sql: "SELECT id FROM users ORDER BY NOW()"},
		{name: "group_by_function", sql: "SELECT id FROM users GROUP BY LOWER(id)"},
		{name: "join_on_function", sql: "SELECT u.id FROM users u JOIN orders o ON LOWER(u.name) = LOWER(o.name)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(), tc.sql, "mysql", "")
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if facts.ReadClassification != string(domain.Indeterminate) {
				t.Errorf("classification: got %q, want indeterminate", facts.ReadClassification)
			}
			if !hasParserReason(facts.ReasonCodes, "function_call") {
				t.Errorf("reason codes: got %v, want function_call", facts.ReasonCodes)
			}
		})
	}
}

func TestDependencyCompleteness_TiDB_PreservesWindowDependencyUsages(t *testing.T) {
	t.Parallel()
	facts, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(),
		"SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary) FROM employees",
		"mysql", "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	assertColumnUsage(t, facts.ColumnReferences, "dept", string(domain.UsageWindow))
	assertColumnUsage(t, facts.ColumnReferences, "salary", string(domain.UsageOrdering))
}

func TestDependencyCompleteness_TiDB_ExistingFailClosedAndAdmissibleFormsRemainStable(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	cases := []struct {
		name           string
		sql            string
		classification string
	}{
		{name: "having_function", sql: "SELECT dept FROM employees GROUP BY dept HAVING COUNT(*) > 1", classification: string(domain.Indeterminate)},
		{name: "operator_where", sql: "SELECT id FROM users WHERE id = 1", classification: string(domain.ReadOnly)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts, err := extractor.ExtractQueryAccess(context.Background(), tc.sql, "mysql", "")
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if facts.ReadClassification != tc.classification {
				t.Errorf("classification: got %q, want %q", facts.ReadClassification, tc.classification)
			}
		})
	}
}

func assertColumnUsage(t *testing.T, columns []ColumnFact, column, usage string) {
	t.Helper()
	for _, reference := range columns {
		if reference.Column != column {
			continue
		}
		assertUsageContains(t, reference.Usages, usage)
		return
	}
	t.Errorf("column %q not found in %+v", column, columns)
}

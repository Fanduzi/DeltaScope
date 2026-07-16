// Package tidbparser characterizes TiDB extraction of common pure-effect forms.
// input: aggregate, grouping, and window SELECT forms
// output: function-call reason and current dependency usages
// pos: parser-level characterization; no admission promotion
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestCommonPureEffects_TiDBParser_FunctionsIndeterminate(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	for _, sql := range []string{
		"SELECT COUNT(*) FROM users",
		"SELECT SUM(amount) FROM orders",
		"SELECT AVG(amount) FROM orders",
		"SELECT MIN(amount) FROM orders",
		"SELECT MAX(amount) FROM orders",
		"SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY id) FROM employees",
		"SELECT RANK() OVER (PARTITION BY dept ORDER BY id) FROM employees",
		"SELECT DENSE_RANK() OVER (PARTITION BY dept ORDER BY id) FROM employees",
	} {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			facts, err := extractor.ExtractQueryAccess(context.Background(), sql, "mysql", "")
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

func TestCommonPureEffects_TiDBParser_GroupingAndWindowUsages(t *testing.T) {
	t.Parallel()
	extractor := &QueryAccessExtractor{}
	cases := []struct {
		name string
		sql  string
		want map[string]string
	}{
		{name: "grouping", sql: "SELECT dept, COUNT(*) FROM employees GROUP BY dept", want: map[string]string{"dept": string(domain.UsageGrouping)}},
		{name: "window", sql: "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY id) FROM employees", want: map[string]string{"dept": string(domain.UsageWindow), "id": string(domain.UsageOrdering)}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts, err := extractor.ExtractQueryAccess(context.Background(), tc.sql, "mysql", "")
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			for column, usage := range tc.want {
				found := false
				for _, reference := range facts.ColumnReferences {
					if reference.Column == column {
						found = true
						assertUsageContains(t, reference.Usages, usage)
					}
				}
				if !found {
					t.Errorf("column %q not found in %+v", column, facts.ColumnReferences)
				}
			}
		})
	}
}

func hasParserReason(codes []string, want string) bool {
	for _, code := range codes {
		if code == want {
			return true
		}
	}
	return false
}

//go:build postgresql

// Package postgresql records PostgreSQL's complete traversal of common effect holders.
// input: derived queries, ordering expressions, and join predicates
// output: fail-closed classification with bounded unproven-function reasons
// pos: parser-level characterization; no admission promotion
package postgresql

import (
	"context"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestDependencyCompleteness_PostgreSQL_FunctionHoldersAlreadyFailClosed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sql  string
	}{
		{name: "derived_count", sql: "SELECT id FROM (SELECT COUNT(amount) AS c, id FROM orders GROUP BY id) t"},
		{name: "order_by_count", sql: "SELECT id FROM users ORDER BY COUNT(id)"},
		{name: "order_by_now", sql: "SELECT id FROM users ORDER BY NOW()"},
		{name: "join_on_function", sql: "SELECT u.id FROM users u JOIN orders o ON LOWER(u.name) = LOWER(o.name)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(), tc.sql, "postgresql", "public")
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if facts.ReadClassification != string(domain.Indeterminate) {
				t.Errorf("classification: got %q, want indeterminate", facts.ReadClassification)
			}
			if !hasParserReason(facts.ReasonCodes, string(domain.ReasonUnprovenFunctionEffect)) {
				t.Errorf("reason codes: got %v, want unproven_function_effect", facts.ReasonCodes)
			}
		})
	}
}

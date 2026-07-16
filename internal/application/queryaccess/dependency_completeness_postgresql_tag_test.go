//go:build postgresql

// Package queryaccess records PostgreSQL's complete application-level effect traversal.
// input: derived queries and ORDER BY function expressions
// output: indeterminate admission with bounded unproven-function reasons
// pos: application-level characterization; no trust-policy changes
package queryaccess

import (
	"testing"
)

func TestDependencyCompleteness_PostgreSQL_FunctionHoldersAlreadyFailClosed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		sql  string
	}{
		{name: "derived_count", sql: "SELECT id FROM (SELECT COUNT(amount) AS c, id FROM orders GROUP BY id) t"},
		{name: "order_by_count", sql: "SELECT id FROM users ORDER BY COUNT(id)"},
		{name: "order_by_now", sql: "SELECT id FROM users ORDER BY NOW()"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertPostgreSQLDefaultIndeterminate(t, analyzeCommonPostgreSQL(t, tc.sql))
		})
	}
}

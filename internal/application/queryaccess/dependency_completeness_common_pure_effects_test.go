// Package queryaccess verifies application-level dependency completeness for MySQL/TiDB.
// input: common pure-effect holders and an operator-bearing control query
// output: indeterminate admission for unproven functions without promotion
// pos: application-level regression coverage; no trust-policy changes
package queryaccess

import (
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestDependencyCompleteness_CommonPureEffects_MySQLTiDB_FunctionHoldersIndeterminate(t *testing.T) {
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
			for _, dialect := range []string{"mysql", "tidb"} {
				t.Run(dialect, func(t *testing.T) {
					result := analyzeCommonMySQL(t, dialect, tc.sql, "strict")
					assertCommonFunctionIndeterminate(t, result)
				})
			}
		})
	}
}

func TestDependencyCompleteness_CommonPureEffects_MySQLTiDB_WindowUsages(t *testing.T) {
	t.Parallel()
	for _, dialect := range []string{"mysql", "tidb"} {
		t.Run(dialect, func(t *testing.T) {
			result := analyzeCommonMySQL(t, dialect,
				"SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary) FROM employees", "strict")
			assertCommonFunctionIndeterminate(t, result)
			assertCommonColumn(t, result, "employees", "dept", domain.UsageWindow)
			assertCommonColumn(t, result, "employees", "salary", domain.UsageOrdering)
		})
	}
}

func TestDependencyCompleteness_CommonPureEffects_MySQLTiDB_ExistingBoundariesRemainStable(t *testing.T) {
	t.Parallel()
	for _, dialect := range []string{"mysql", "tidb"} {
		dialect := dialect
		t.Run(dialect, func(t *testing.T) {
			having := analyzeCommonMySQL(t, dialect,
				"SELECT dept FROM employees GROUP BY dept HAVING COUNT(*) > 1", "strict")
			assertCommonFunctionIndeterminate(t, having)

			operator := analyzeCommonMySQL(t, dialect, "SELECT id FROM users WHERE id = 1", "strict")
			if operator.ReadClassification != domain.ReadOnly || operator.Admission != domain.Admissible {
				t.Errorf("operator control: got %q/%q, want read_only/admissible", operator.ReadClassification, operator.Admission)
			}
		})
	}
}

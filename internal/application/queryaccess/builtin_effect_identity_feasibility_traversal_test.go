// Package queryaccess_test locks the hidden-function fail-closed boundary
// for MySQL/TiDB (Task 2). A function hidden in a derived table, subquery,
// ORDER BY, HAVING, JOIN subquery, CASE WHEN, or nested expression position
// must never be silently admitted on either dialect.
//
// input: function-bearing SQL with the function hidden in non-projection
//
//	positions (derived table, WHERE subquery, ORDER BY, HAVING, JOIN
//	subquery, CASE WHEN, nested function call, nested derived table)
//
// output: every case remains indeterminate with unknown_function_effect
// pos: application-level negative regression; no promotion
package queryaccess_test

import (
	"context"
	"testing"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestFeasibilityT2_HiddenFunctionFailClosed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sql  string
	}{
		{name: "derived_count_star", sql: "SELECT t.c FROM (SELECT COUNT(*) AS c FROM orders) t"},
		{name: "where_subquery_max", sql: "SELECT id FROM users WHERE id IN (SELECT MAX(id) FROM orders)"},
		{name: "having_subquery_max", sql: "SELECT dept FROM employees GROUP BY dept HAVING dept IN (SELECT MAX(x) FROM t2)"},
		{name: "case_when_count", sql: "SELECT CASE WHEN COUNT(*) > 0 THEN 'a' ELSE 'b' END FROM users"},
		{name: "nested_function_abs", sql: "SELECT SUM(ABS(id)) FROM users"},
		{name: "join_subquery_max", sql: "SELECT u.id FROM users u JOIN (SELECT MAX(id) AS mid FROM orders) j ON u.id = j.mid"},
		{name: "nested_derived_function", sql: "SELECT x.v FROM (SELECT SUM(v) AS v FROM (SELECT COUNT(*) AS v FROM users) y) x"},
		{name: "orderby_function", sql: "SELECT id FROM users ORDER BY LENGTH(id)"},
		{name: "groupby_function", sql: "SELECT id FROM users GROUP BY LOWER(id)"},
		{name: "join_on_function", sql: "SELECT u.id FROM users u JOIN orders o ON LOWER(u.name) = LOWER(o.name)"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, dialect := range []string{"mysql", "tidb"} {
				dialect := dialect
				t.Run(dialect, func(t *testing.T) {
					t.Parallel()
					res, err := (&appqa.Service{}).Analyze(context.Background(), appqa.QueryAccessRequest{
						SQL: tc.sql, Dialect: dialect, Mode: "strict",
					})
					if err != nil {
						t.Fatalf("analyze: %v", err)
					}
					dr := res.DomainResult
					if dr.ReadClassification != domain.Indeterminate {
						t.Errorf("classification: got %q, want %q", dr.ReadClassification, domain.Indeterminate)
					}
					if dr.Admission != domain.IndeterminateAdmission {
						t.Errorf("admission: got %q, want %q", dr.Admission, domain.IndeterminateAdmission)
					}
					foundFn := false
					for _, r := range dr.ReasonCodes {
						if r == domain.ReasonFunctionEffect {
							foundFn = true
						}
					}
					if !foundFn {
						t.Errorf("expected %q in %v", domain.ReasonFunctionEffect, dr.ReasonCodes)
					}
					if len(res.EffectCandidates) == 0 {
						t.Errorf("hidden function must retain internal effect candidates")
					}
				})
			}
		})
	}
}

// TestFeasibilityT2_ProjectionOnlyHiddenFunctionFailClosed verifies projection_only
// mode does not silently lift a hidden function to admissible. The function
// effect boundary is independent of mode.
func TestFeasibilityT2_ProjectionOnlyHiddenFunctionFailClosed(t *testing.T) {
	t.Parallel()
	for _, dialect := range []string{"mysql", "tidb"} {
		dialect := dialect
		t.Run(dialect, func(t *testing.T) {
			t.Parallel()
			res, err := (&appqa.Service{}).Analyze(context.Background(), appqa.QueryAccessRequest{
				SQL:     "SELECT t.c FROM (SELECT COUNT(*) AS c FROM orders) t",
				Dialect: dialect,
				Mode:    "projection_only",
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			dr := res.DomainResult
			if dr.ReadClassification != domain.Indeterminate {
				t.Errorf("classification: got %q, want %q", dr.ReadClassification, domain.Indeterminate)
			}
			if dr.Admission != domain.IndeterminateAdmission {
				t.Errorf("admission: got %q, want %q", dr.Admission, domain.IndeterminateAdmission)
			}
		})
	}
}

// TestFeasibilityT2_FunctionFreeAdmissibleNoRegression verifies function-free
// queries with the same structural shapes (derived, subquery, JOIN) remain
// admissible. This is the non-regression baseline: the hidden-function lock
// must not narrow the existing admissible set.
func TestFeasibilityT2_FunctionFreeAdmissibleNoRegression(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sql  string
	}{
		{name: "derived_no_function", sql: "SELECT t.c FROM (SELECT id AS c FROM orders) t"},
		{name: "where_subquery_no_function", sql: "SELECT id FROM users WHERE id IN (SELECT id FROM orders)"},
		{name: "join_subquery_no_function", sql: "SELECT u.id FROM users u JOIN (SELECT id AS mid FROM orders) j ON u.id = j.mid"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, dialect := range []string{"mysql", "tidb"} {
				dialect := dialect
				t.Run(dialect, func(t *testing.T) {
					t.Parallel()
					res, err := (&appqa.Service{}).Analyze(context.Background(), appqa.QueryAccessRequest{
						SQL: tc.sql, Dialect: dialect, Mode: "strict",
					})
					if err != nil {
						t.Fatalf("analyze: %v", err)
					}
					dr := res.DomainResult
					if dr.ReadClassification != domain.ReadOnly {
						t.Errorf("classification: got %q, want %q", dr.ReadClassification, domain.ReadOnly)
					}
					if dr.Admission != domain.Admissible {
						t.Errorf("admission: got %q, want %q", dr.Admission, domain.Admissible)
					}
				})
			}
		})
	}
}

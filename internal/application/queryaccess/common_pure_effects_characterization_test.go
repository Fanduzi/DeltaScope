// Package queryaccess characterizes the current MySQL/TiDB pure-effect boundary.
// input: aggregate, grouping, window, and adjacent SELECT forms
// output: current indeterminate classification, requirements, and bounded reasons
// pos: application-level characterization; no admission or trust promotion
package queryaccess

import (
	"context"
	"fmt"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func analyzeCommonMySQL(t *testing.T, dialect, sql, mode string) domain.Result {
	t.Helper()
	res, err := (&Service{}).Analyze(context.Background(), QueryAccessRequest{
		SQL: sql, Dialect: dialect, Mode: mode,
	})
	if err != nil {
		t.Fatalf("analyze %s %q: %v", dialect, sql, err)
	}
	return res.DomainResult
}

func assertCommonFunctionIndeterminate(t *testing.T, result domain.Result) {
	t.Helper()
	if result.ReadClassification != domain.Indeterminate {
		t.Errorf("classification: got %q, want %q", result.ReadClassification, domain.Indeterminate)
	}
	if result.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission: got %q, want %q", result.Admission, domain.IndeterminateAdmission)
	}
	assertReasonsContain(t, result.ReasonCodes, domain.ReasonFunctionEffect)
	if hasReason(result.ReasonCodes, domain.ReasonUnprovenFunctionEffect) {
		t.Errorf("MySQL/TiDB must not emit %q: %v", domain.ReasonUnprovenFunctionEffect, result.ReasonCodes)
	}
}

func assertCommonRequirement(t *testing.T, result domain.Result, object, privilege string) {
	t.Helper()
	for _, requirement := range result.Requirements {
		if requirement.Object == object && requirement.Privilege == privilege {
			return
		}
	}
	t.Errorf("missing requirement %s/%s in %+v", object, privilege, result.Requirements)
}

func assertCommonColumn(t *testing.T, result domain.Result, table, column string, usages ...domain.UsageContext) {
	t.Helper()
	for _, reference := range result.ReferencedColumns {
		if reference.Table != table || reference.Column != column {
			continue
		}
		for _, want := range usages {
			found := false
			for _, got := range reference.Usages {
				if got == want {
					found = true
				}
			}
			if !found {
				t.Errorf("column %s.%s missing usage %q: %v", table, column, want, reference.Usages)
			}
		}
		return
	}
	t.Errorf("missing column %s.%s in %+v", table, column, result.ReferencedColumns)
}

func TestCommonPureEffects_MySQLTiDB_AggregatesIndeterminate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		sql        string
		relation   string
		column     string
		wantColumn bool
	}{
		{name: "count_star", sql: "SELECT COUNT(*) FROM users", relation: "users"},
		{name: "count_column", sql: "SELECT COUNT(id) FROM users", relation: "users", column: "id", wantColumn: true},
		{name: "sum", sql: "SELECT SUM(amount) FROM orders", relation: "orders", column: "amount", wantColumn: true},
		{name: "avg", sql: "SELECT AVG(amount) FROM orders", relation: "orders", column: "amount", wantColumn: true},
		{name: "min", sql: "SELECT MIN(amount) FROM orders", relation: "orders", column: "amount", wantColumn: true},
		{name: "max", sql: "SELECT MAX(amount) FROM orders", relation: "orders", column: "amount", wantColumn: true},
		{name: "groupby_count", sql: "SELECT dept, COUNT(*) FROM employees GROUP BY dept", relation: "employees", column: "dept", wantColumn: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, dialect := range []string{"mysql", "tidb"} {
				result := analyzeCommonMySQL(t, dialect, tc.sql, "strict")
				assertCommonFunctionIndeterminate(t, result)
				assertCommonRequirement(t, result, tc.relation, "read_table")
				if tc.name == "count_star" {
					projectionOnly := analyzeCommonMySQL(t, dialect, tc.sql, "projection_only")
					assertCommonFunctionIndeterminate(t, projectionOnly)
					if hasReason(projectionOnly.ReasonCodes, domain.ReasonSchemaUnavailable) {
						t.Errorf("projection-only COUNT(*) unexpectedly needs schema expansion: %v", projectionOnly.ReasonCodes)
					}
				}
				if tc.wantColumn {
					assertCommonColumn(t, result, tc.relation, tc.column, domain.UsageProjection)
					assertCommonRequirement(t, result, tc.relation+"."+tc.column, "read_column")
				}
			}
		})
	}
}

func TestCommonPureEffects_MySQLTiDB_WindowsIndeterminate(t *testing.T) {
	t.Parallel()
	for _, function := range []string{"ROW_NUMBER", "RANK", "DENSE_RANK"} {
		function := function
		t.Run(strings.ToLower(function), func(t *testing.T) {
			t.Parallel()
			for _, dialect := range []string{"mysql", "tidb"} {
				result := analyzeCommonMySQL(t, dialect,
					fmt.Sprintf("SELECT %s() OVER (PARTITION BY dept ORDER BY id) FROM employees", function), "strict")
				assertCommonFunctionIndeterminate(t, result)
				assertCommonRequirement(t, result, "employees", "read_table")
				assertCommonColumn(t, result, "employees", "dept", domain.UsageWindow)
				assertCommonColumn(t, result, "employees", "id", domain.UsageOrdering)
				assertCommonRequirement(t, result, "employees.dept", "read_column")
				assertCommonRequirement(t, result, "employees.id", "read_column")
			}
		})
	}
}

func TestCommonPureEffects_MySQLTiDB_NegativesIndeterminate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		sql          string
		functionCall bool
	}{
		{name: "count_distinct", sql: "SELECT COUNT(DISTINCT id) FROM users", functionCall: true},
		{name: "udf", sql: "SELECT my_udf(id) FROM users", functionCall: true},
		{name: "wildcard", sql: "SELECT * FROM users", functionCall: false},
		{name: "ambiguous_join", sql: "SELECT id FROM users JOIN orders ON users.id = orders.user_id", functionCall: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, dialect := range []string{"mysql", "tidb"} {
				result := analyzeCommonMySQL(t, dialect, tc.sql, "strict")
				if tc.functionCall {
					assertCommonFunctionIndeterminate(t, result)
					continue
				}
				if result.ReadClassification != domain.Indeterminate || result.Admission != domain.IndeterminateAdmission {
					t.Errorf("classification/admission: got %q/%q, want indeterminate/indeterminate", result.ReadClassification, result.Admission)
				}
			}
		})
	}
}

func TestCommonPureEffects_MySQLTiDB_OperatorBearingStillAdmissible(t *testing.T) {
	t.Parallel()
	for _, dialect := range []string{"mysql", "tidb"} {
		dialect := dialect
		t.Run(dialect, func(t *testing.T) {
			t.Parallel()
			result := analyzeCommonMySQL(t, dialect, "SELECT id FROM users WHERE id = 1", "strict")
			if result.ReadClassification != domain.ReadOnly {
				t.Errorf("classification: got %q, want %q", result.ReadClassification, domain.ReadOnly)
			}
			if result.Admission != domain.Admissible {
				t.Errorf("admission: got %q, want %q", result.Admission, domain.Admissible)
			}
		})
	}
	for _, dialect := range []string{"mysql", "tidb"} {
		dialect := dialect
		t.Run(dialect+"_cast_adjacent", func(t *testing.T) {
			t.Parallel()
			result := analyzeCommonMySQL(t, dialect, "SELECT CAST(id AS CHAR) FROM users", "strict")
			if result.ReadClassification != domain.ReadOnly || result.Admission != domain.Admissible {
				t.Errorf("cast classification/admission: got %q/%q, want read_only/admissible", result.ReadClassification, result.Admission)
			}
		})
	}
}

func TestCommonPureEffects_MySQLTiDB_NoLeak(t *testing.T) {
	t.Parallel()
	result := analyzeCommonMySQL(t, "mysql", "SELECT my_udf(id) FROM users", "strict")
	dump := fmt.Sprintf("%+v", result)
	for _, forbidden := range []string{"severity", "my_udf", "SELECT", "password", "postgres://"} {
		if strings.Contains(dump, forbidden) {
			t.Errorf("result leaked %q: %s", forbidden, dump)
		}
	}
	if len(result.ReasonCodes) == 0 {
		t.Fatal("expected bounded reason codes")
	}
	if len((&QueryAccessResult{}).EffectCandidates) != 0 {
		t.Fatal("zero-value EffectCandidates sanity check failed")
	}
	for _, dialect := range []string{"mysql", "tidb"} {
		res, err := (&Service{}).Analyze(context.Background(), QueryAccessRequest{
			SQL: "SELECT COUNT(*) FROM users", Dialect: dialect, Mode: "strict",
		})
		if err != nil {
			t.Fatalf("analyze %s: %v", dialect, err)
		}
		if len(res.EffectCandidates) == 0 {
			t.Errorf("%s COUNT(*) effect candidates must be retained", dialect)
		}
	}
}

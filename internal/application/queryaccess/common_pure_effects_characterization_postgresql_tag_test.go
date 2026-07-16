//go:build postgresql

// Package queryaccess characterizes the default PostgreSQL pure-effect boundary.
// input: schema-qualified and unqualified aggregate/window SELECT forms
// output: unproven function effects, requirements, and fail-closed admission
// pos: application-level characterization; no trusted-bundle promotion
package queryaccess

import (
	"context"
	"fmt"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func analyzeCommonPostgreSQL(t *testing.T, sql string) QueryAccessResult {
	t.Helper()
	result, err := (&Service{}).Analyze(context.Background(), QueryAccessRequest{
		SQL: sql, Dialect: "postgresql", Mode: "strict",
	})
	if err != nil {
		t.Fatalf("analyze %q: %v", sql, err)
	}
	return result
}

func assertPostgreSQLDefaultIndeterminate(t *testing.T, result QueryAccessResult) {
	t.Helper()
	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification: got %q, want %q", result.DomainResult.ReadClassification, domain.Indeterminate)
	}
	if result.DomainResult.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission: got %q, want %q", result.DomainResult.Admission, domain.IndeterminateAdmission)
	}
	if !hasReason(result.DomainResult.ReasonCodes, domain.ReasonUnprovenFunctionEffect) {
		t.Errorf("expected %q in %v", domain.ReasonUnprovenFunctionEffect, result.DomainResult.ReasonCodes)
	}
	if hasReason(result.DomainResult.ReasonCodes, domain.ReasonFunctionEffect) {
		t.Errorf("PostgreSQL must not emit %q: %v", domain.ReasonFunctionEffect, result.DomainResult.ReasonCodes)
	}
	if len(result.EffectCandidates) < 1 {
		t.Errorf("expected internal effect candidates, got %v", result.EffectCandidates)
	}
}

func TestCommonPureEffects_PostgreSQLDefault_AggregatesIndeterminate(t *testing.T) {
	t.Parallel()
	for _, function := range []string{"COUNT(*)", "SUM(amount)", "AVG(amount)", "MIN(amount)", "MAX(amount)"} {
		function := function
		t.Run(function, func(t *testing.T) {
			t.Parallel()
			table := "public.users"
			if function != "COUNT(*)" {
				table = "public.orders"
			}
			result := analyzeCommonPostgreSQL(t, fmt.Sprintf("SELECT %s FROM %s", function, table))
			assertPostgreSQLDefaultIndeterminate(t, result)
			if len(result.DomainResult.Requirements) == 0 {
				t.Error("expected physical requirements for qualified relation")
			}
		})
	}
}

func TestCommonPureEffects_PostgreSQLDefault_WindowsIndeterminate(t *testing.T) {
	t.Parallel()
	for _, function := range []string{"ROW_NUMBER", "RANK", "DENSE_RANK"} {
		function := function
		t.Run(strings.ToLower(function), func(t *testing.T) {
			t.Parallel()
			result := analyzeCommonPostgreSQL(t, fmt.Sprintf(
				"SELECT %s() OVER (PARTITION BY dept ORDER BY id) FROM public.employees", function))
			assertPostgreSQLDefaultIndeterminate(t, result)
		})
	}
}

func TestCommonPureEffects_PostgreSQLDefault_UnqualifiedBlocked(t *testing.T) {
	t.Parallel()
	result := analyzeCommonPostgreSQL(t, "SELECT COUNT(*) FROM users")
	assertPostgreSQLDefaultIndeterminate(t, result)
	if !hasReason(result.DomainResult.ReasonCodes, domain.ReasonUnqualifiedRelationBlocked) {
		t.Errorf("expected unqualified relation reason: %v", result.DomainResult.ReasonCodes)
	}
}

func TestCommonPureEffects_PostgreSQLDefault_NoLeak(t *testing.T) {
	t.Parallel()
	result := analyzeCommonPostgreSQL(t, "SELECT my_udf(id) FROM public.users")
	dump := fmt.Sprintf("%+v", result.DomainResult)
	for _, forbidden := range []string{"severity", "my_udf", "SELECT", "password", "postgres://"} {
		if strings.Contains(dump, forbidden) {
			t.Errorf("result leaked %q: %s", forbidden, dump)
		}
	}
}

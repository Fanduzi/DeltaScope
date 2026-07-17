//go:build postgresql

// Package queryaccess verifies PostgreSQL candidate mapping and public-result privacy.
// input: PostgreSQL aggregate and window queries
// output: mapped internal candidates with unchanged indeterminate admission
// pos: application coverage for parser candidate facts without promotion
package queryaccess

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestAnalyzePostgreSQL_PureEffectCandidates_MapFactsWithoutPublicLeak(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sql  string
		want func(*testing.T, []EffectCandidate)
	}{
		{
			name: "count star",
			sql:  "SELECT count(*) FROM public.users",
			want: func(t *testing.T, candidates []EffectCandidate) {
				t.Helper()
				candidate := functionCandidate(t, candidates, "count")
				if candidate.Arity != 0 || len(candidate.OperandKinds) != 1 || candidate.OperandKinds[0] != "star" {
					t.Fatalf("count candidate: %+v", candidate)
				}
			},
		},
		{
			name: "sum column",
			sql:  "SELECT sum(amount) FROM public.orders",
			want: func(t *testing.T, candidates []EffectCandidate) {
				t.Helper()
				candidate := functionCandidate(t, candidates, "sum")
				if candidate.OperandKinds[0] != "column" || len(candidate.OperandColumnRefs) != 1 || candidate.OperandColumnRefs[0].Column != "amount" {
					t.Fatalf("sum candidate: %+v", candidate)
				}
			},
		},
		{
			name: "window",
			sql:  "SELECT row_number() OVER (PARTITION BY dept ORDER BY id) FROM public.employees",
			want: func(t *testing.T, candidates []EffectCandidate) {
				t.Helper()
				candidate := functionCandidate(t, candidates, "row_number")
				if candidate.Arity != 0 || !candidate.HasWindow {
					t.Fatalf("window candidate: %+v", candidate)
				}
				if candidate.HasFrame {
					t.Fatalf("default OVER must not set HasFrame: %+v", candidate)
				}
			},
		},
		{
			name: "window explicit frame",
			sql:  "SELECT row_number() OVER (PARTITION BY dept ORDER BY id ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) FROM public.employees",
			want: func(t *testing.T, candidates []EffectCandidate) {
				t.Helper()
				candidate := functionCandidate(t, candidates, "row_number")
				if !candidate.HasWindow || !candidate.HasFrame {
					t.Fatalf("explicit frame must set HasFrame: %+v", candidate)
				}
			},
		},
		{
			name: "filter",
			sql:  "SELECT count(*) FILTER (WHERE true) FROM public.users",
			want: func(t *testing.T, candidates []EffectCandidate) {
				t.Helper()
				if !functionCandidate(t, candidates, "count").HasFilter {
					t.Fatal("expected HasFilter")
				}
			},
		},
		{
			name: "distinct",
			sql:  "SELECT count(DISTINCT id) FROM public.users",
			want: func(t *testing.T, candidates []EffectCandidate) {
				t.Helper()
				if !functionCandidate(t, candidates, "count").HasDistinct {
					t.Fatal("expected HasDistinct")
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
				SQL: tc.sql, Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			if result.DomainResult.ReadClassification != domain.Indeterminate || result.DomainResult.Admission != domain.IndeterminateAdmission {
				t.Fatalf("admission changed: classification=%q admission=%q", result.DomainResult.ReadClassification, result.DomainResult.Admission)
			}
			if !hasReason(result.DomainResult.ReasonCodes, domain.ReasonUnprovenFunctionEffect) {
				t.Fatalf("missing function reason: %v", result.DomainResult.ReasonCodes)
			}
			tc.want(t, result.EffectCandidates)

			data, err := json.Marshal(result.DomainResult)
			if err != nil {
				t.Fatalf("marshal domain result: %v", err)
			}
			if strings.Contains(string(data), "effect_candidates") || strings.Contains(string(data), "OperandColumnRefs") {
				t.Fatalf("candidate facts leaked into domain JSON: %s", data)
			}
		})
	}
}

func TestAnalyzePostgreSQL_PureEffectCandidates_OrderedAggregateFunctions(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL: "SELECT avg(amount), min(amount), max(amount) FROM public.orders", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var names []string
	for _, candidate := range result.EffectCandidates {
		if candidate.Kind == EffectCandidateFunction {
			names = append(names, candidate.NamePath[len(candidate.NamePath)-1])
		}
	}
	if got, want := strings.Join(names, ","), "avg,min,max"; got != want {
		t.Fatalf("function order: got %q, want %q", got, want)
	}
}

func functionCandidate(t *testing.T, candidates []EffectCandidate, name string) EffectCandidate {
	t.Helper()
	for _, candidate := range candidates {
		if candidate.Kind == EffectCandidateFunction && len(candidate.NamePath) > 0 && strings.EqualFold(candidate.NamePath[len(candidate.NamePath)-1], name) {
			return candidate
		}
	}
	t.Fatalf("missing function candidate %q: %+v", name, candidates)
	return EffectCandidate{}
}

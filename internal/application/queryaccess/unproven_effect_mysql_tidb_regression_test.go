// Package queryaccess regression coverage for MySQL/TiDB under unproven-effect reasons (T4).
// input: MySQL/TiDB operator-bearing SELECT forms that are currently admissible
// output: classification/admission must not regress; unproven_* PG codes must not appear
// pos: dialect non-alignment guard for Query Access pure-read reason work
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func hasReason(codes []domain.ReasonCode, want domain.ReasonCode) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

func assertReasonsContain(t *testing.T, codes []domain.ReasonCode, want ...domain.ReasonCode) {
	t.Helper()
	for _, w := range want {
		if !hasReason(codes, w) {
			t.Errorf("expected reason %q in %v", w, codes)
		}
	}
}

func assertReasonsOnlyMachineIDs(t *testing.T, codes []domain.ReasonCode) {
	t.Helper()
	for _, c := range codes {
		s := string(c)
		if s == "" {
			t.Error("empty reason code")
			continue
		}
		if strings.ContainsAny(s, " \t\n'\"();=<>:") {
			t.Errorf("reason %q must be a machine identifier", c)
		}
	}
}

func TestUnprovenEffectReasons_MySQLTiDBOperatorBearingNoRegression(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		dialect string
		sql     string
	}{
		{name: "mysql_where_eq", dialect: "mysql", sql: "SELECT id FROM users WHERE id = 1"},
		{name: "mysql_where_gt", dialect: "mysql", sql: "SELECT id FROM users WHERE salary > 50000"},
		{name: "tidb_where_eq", dialect: "tidb", sql: "SELECT id FROM users WHERE id = 1"},
		{name: "tidb_join_on", dialect: "tidb", sql: "SELECT u.id FROM users u JOIN orders o ON u.id = o.user_id"},
	}

	svc := &Service{}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, err := svc.Analyze(context.Background(), QueryAccessRequest{
				SQL: tc.sql, Dialect: tc.dialect, Mode: "strict",
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
			// PostgreSQL unproven_* codes must not appear on MySQL/TiDB operator-bearing admissible cases.
			for _, bad := range []domain.ReasonCode{
				domain.ReasonUnprovenOperatorEffect,
				domain.ReasonUnprovenFunctionEffect,
				domain.ReasonUnprovenCastEffect,
				domain.ReasonIdentityLookupFailed,
				domain.ReasonIdentityUnknown,
				domain.ReasonIdentityAmbiguous,
				domain.ReasonIdentityCoercionGap,
				domain.ReasonIdentityResolverUnavailable,
			} {
				if hasReason(dr.ReasonCodes, bad) {
					t.Errorf("MySQL/TiDB must not emit PG unproven/identity reason %q; got %v", bad, dr.ReasonCodes)
				}
			}
		})
	}
}

func TestEffectCandidates_MySQLTiDBOrdered(t *testing.T) {
	t.Parallel()
	svc := &Service{}
	for _, dialect := range []string{"mysql", "tidb"} {
		dialect := dialect
		t.Run(dialect, func(t *testing.T) {
			t.Parallel()
			res, err := svc.Analyze(context.Background(), QueryAccessRequest{
				SQL: "SELECT id FROM users WHERE id = 1", Dialect: dialect, Mode: "strict",
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			if len(res.EffectCandidates) != 0 {
				t.Errorf("%s function-free query must not invent effect candidates: %+v", dialect, res.EffectCandidates)
			}
			functionResult, err := svc.Analyze(context.Background(), QueryAccessRequest{
				SQL: "SELECT COUNT(*) FROM users", Dialect: dialect, Mode: "strict",
			})
			if err != nil {
				t.Fatalf("analyze function: %v", err)
			}
			if len(functionResult.EffectCandidates) == 0 {
				t.Fatalf("%s function-bearing query must extract internal effect candidates", dialect)
			}
			for i, candidate := range functionResult.EffectCandidates {
				if candidate.Ordinal != i {
					t.Errorf("%s candidate ordinal: got %d at index %d", dialect, candidate.Ordinal, i)
				}
			}
			if res.DomainResult.Admission != domain.Admissible {
				t.Errorf("admission: got %q, want admissible", res.DomainResult.Admission)
			}
		})
	}
}

func TestUnprovenEffectReasons_MySQLFunctionStillUnknownEffect(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT NOW() FROM users", Dialect: "mysql", Mode: "strict",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	dr := res.DomainResult
	if dr.ReadClassification != domain.Indeterminate {
		t.Errorf("classification: got %q, want %q", dr.ReadClassification, domain.Indeterminate)
	}
	if !hasReason(dr.ReasonCodes, domain.ReasonFunctionEffect) {
		t.Errorf("expected %q in %v", domain.ReasonFunctionEffect, dr.ReasonCodes)
	}
	if hasReason(dr.ReasonCodes, domain.ReasonUnprovenFunctionEffect) {
		t.Errorf("MySQL must keep unknown_function_effect path, not unproven_function_effect; got %v", dr.ReasonCodes)
	}
}

//go:build postgresql

// Package queryaccess tests that internal effect candidates do not alter public output (T5).
// input: PostgreSQL SQL with effect candidates
// output: classification/admission/reason unchanged; candidates internal-only
// pos: application-level freeze for candidate extraction without promotion
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestEffectCandidates_DoNotChangePublicResult(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		sql        string
		wantReason domain.ReasonCode
	}{
		{name: "operator", sql: "SELECT id FROM users WHERE id = 1", wantReason: domain.ReasonUnprovenOperatorEffect},
		{name: "count", sql: "SELECT COUNT(*) FROM users", wantReason: domain.ReasonUnprovenFunctionEffect},
		{name: "cast", sql: "SELECT id::text FROM users", wantReason: domain.ReasonUnprovenCastEffect},
		{name: "limit", sql: "SELECT id FROM users LIMIT length('a')", wantReason: domain.ReasonUnprovenFunctionEffect},
		{name: "values", sql: "VALUES (length('a'))", wantReason: domain.ReasonUnprovenFunctionEffect},
		{name: "window", sql: "SELECT row_number() OVER (PARTITION BY length(name)) FROM users", wantReason: domain.ReasonUnprovenFunctionEffect},
		{name: "agg_filter", sql: "SELECT count(*) FILTER (WHERE length(name) > 0) FROM users", wantReason: domain.ReasonUnprovenFunctionEffect},
		{name: "udf", sql: "SELECT my_udf(id) FROM users", wantReason: domain.ReasonUnprovenFunctionEffect},
		{name: "current_setting", sql: "SELECT current_setting('search_path')", wantReason: domain.ReasonUnprovenFunctionEffect},
	}

	svc := &Service{}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, err := svc.Analyze(context.Background(), QueryAccessRequest{
				SQL: tc.sql, Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			dr := res.DomainResult
			assertIndeterminateClassAndAdmission(t, dr, tc.name)
			assertReasonsContain(t, dr.ReasonCodes, tc.wantReason)

			if len(res.EffectCandidates) == 0 {
				t.Fatalf("expected internal candidates for %s", tc.name)
			}
			// domain.Result must not gain candidate fields.
			rt := reflect.TypeOf(dr)
			if _, ok := rt.FieldByName("EffectCandidates"); ok {
				t.Fatal("domain.Result must not export EffectCandidates")
			}
			data, err := json.Marshal(dr)
			if err != nil {
				t.Fatalf("marshal domain result: %v", err)
			}
			raw := string(data)
			if strings.Contains(raw, "effect_candidates") || strings.Contains(raw, "EffectCandidates") {
				t.Errorf("domain JSON must not contain candidates: %s", raw)
			}
			// Public JSON must not embed candidate name spellings for common effects
			// when those names are not relation/column identifiers.
			for _, bad := range []string{"severity", "password", "postgres://"} {
				if strings.Contains(raw, bad) {
					t.Errorf("domain JSON leaked %q", bad)
				}
			}
		})
	}
}

func TestEffectCandidates_RequestCannotInject(t *testing.T) {
	t.Parallel()
	rt := reflect.TypeOf(QueryAccessRequest{})
	for _, name := range []string{"EffectCandidates", "Trusted", "Trust", "Candidates", "EffectIdentityResolver"} {
		if _, ok := rt.FieldByName(name); ok {
			t.Errorf("QueryAccessRequest must not allow injection field %q", name)
		}
	}
}

// TestEffectCandidates_T6IdentityContractDoesNotChangeAdmission freezes T5 public
// behavior after introducing the T6 facts-only identity resolver contract.
func TestEffectCandidates_T6IdentityContractDoesNotChangeAdmission(t *testing.T) {
	t.Parallel()
	svc := &Service{}
	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT id FROM users WHERE id = 1", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	assertIndeterminateClassAndAdmission(t, res.DomainResult, "comparison")
	assertReasonsContain(t, res.DomainResult.ReasonCodes, domain.ReasonUnprovenOperatorEffect)
	// Identity resolver is not invoked in T6: no identity_* reasons attached by Analyze.
	for _, code := range res.DomainResult.ReasonCodes {
		if strings.HasPrefix(string(code), "identity_") {
			t.Errorf("T6 must not attach identity_* reasons via Analyze yet: %q", code)
		}
	}
	data, err := json.Marshal(res.DomainResult)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)
	for _, bad := range []string{
		"ObjectOID", "object_oid", "CanonicalSignature", "NamespaceOID",
		"EffectIdentity", "identity_facts", "severity", "postgres://",
	} {
		if strings.Contains(raw, bad) {
			t.Errorf("public domain JSON leaked %q", bad)
		}
	}
}

func TestEffectCandidates_StableOrderThroughService(t *testing.T) {
	t.Parallel()
	sql := "SELECT COUNT(id)::text FROM users WHERE id = 1"
	svc := &Service{}
	var first []EffectCandidate
	for i := 0; i < 3; i++ {
		res, err := svc.Analyze(context.Background(), QueryAccessRequest{
			SQL: sql, Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
		})
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		if i == 0 {
			first = append([]EffectCandidate(nil), res.EffectCandidates...)
			continue
		}
		if len(res.EffectCandidates) != len(first) {
			t.Fatalf("count mismatch")
		}
		for j := range first {
			if res.EffectCandidates[j].Ordinal != first[j].Ordinal ||
				res.EffectCandidates[j].Kind != first[j].Kind {
				t.Fatalf("unstable candidate order: %v vs %v", res.EffectCandidates, first)
			}
		}
	}
}

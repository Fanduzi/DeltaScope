//go:build postgresql

// Package queryaccess verifies the Phase-1 pure-effect proof gateway.
// input: PostgreSQL effect candidates and controlled resolver facts
// output: fail-closed promotion decisions with no candidate leakage
// pos: PostgreSQL Phase-1 proof completeness and eligibility coverage
package queryaccess

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestValidatePhase1PureEffectCandidates(t *testing.T) {
	tests := []struct {
		name   string
		input  EffectCandidate
		ok     bool
		reason domain.ReasonCode
	}{
		{name: "count star", input: EffectCandidate{Kind: EffectCandidateFunction, NamePath: []string{"count"}, Arity: 0, OperandKinds: []string{"star"}}, ok: true},
		{name: "sum column", input: EffectCandidate{Kind: EffectCandidateFunction, Arity: 1, OperandKinds: []string{"column"}, OperandColumnRefs: []OperandColumnRef{{Column: "amount"}}}, ok: true},
		{name: "count filter", input: EffectCandidate{Kind: EffectCandidateFunction, Arity: 0, OperandKinds: []string{"star"}, HasFilter: true}, reason: domain.ReasonUnprovenFunctionEffect},
		{name: "count distinct", input: EffectCandidate{Kind: EffectCandidateFunction, Arity: 1, OperandKinds: []string{"column"}, OperandColumnRefs: []OperandColumnRef{{Column: "id"}}, HasDistinct: true}, reason: domain.ReasonUnprovenFunctionEffect},
		{name: "sum nested expression", input: EffectCandidate{Kind: EffectCandidateFunction, Arity: 1, OperandKinds: []string{"expr"}}, reason: domain.ReasonUnprovenFunctionEffect},
		{name: "sum incomplete refs", input: EffectCandidate{Kind: EffectCandidateFunction, Arity: 1, OperandKinds: []string{"column"}}, reason: domain.ReasonUnprovenFunctionEffect},
		{name: "windowed aggregate", input: EffectCandidate{Kind: EffectCandidateFunction, Arity: 1, OperandKinds: []string{"column"}, OperandColumnRefs: []OperandColumnRef{{Column: "amount"}}, HasWindow: true}, reason: domain.ReasonUnprovenFunctionEffect},
		{name: "row number", input: EffectCandidate{Kind: EffectCandidateFunction, NamePath: []string{"row_number"}, Arity: 0, HasWindow: true}, ok: true},
		{name: "row number frame", input: EffectCandidate{Kind: EffectCandidateFunction, NamePath: []string{"row_number"}, Arity: 0, HasWindow: true, HasFrame: true}, reason: domain.ReasonUnprovenFunctionEffect},
		{name: "cast", input: EffectCandidate{Kind: EffectCandidateCast, Arity: 1, OperandKinds: []string{"column"}}, reason: domain.ReasonUnprovenCastEffect},
		{name: "operator", input: EffectCandidate{Kind: EffectCandidateOperator, Arity: 2}, ok: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := ValidatePhase1PureEffectCandidates([]EffectCandidate{tc.input})
			if ok != tc.ok || reason != tc.reason {
				t.Fatalf("ValidatePhase1PureEffectCandidates() = (%t, %q), want (%t, %q)", ok, reason, tc.ok, tc.reason)
			}
		})
	}
}

func TestPureEffectProofGateway_CountStarEligible(t *testing.T) {
	policy := phase1Policy(t, TrustedEffectEntry{
		Kind: EffectCandidateFunction, ObjectOID: 2803, NamespaceOID: 11, ResultTypeOID: 20,
		Volatility: EffectVolatilityImmutable, CanonicalSignature: "pg_catalog.count()",
	})
	resolver := &mockControlledResolver{ctx: testResolutionContext(), batch: EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0, Status: domain.IdentityStatusResolved, Facts: &EffectIdentityFacts{
			Kind: EffectCandidateFunction, ObjectOID: 2803, NamespaceOID: 11, ResultTypeOID: 20,
			Volatility: EffectVolatilityImmutable, CanonicalSignature: "pg_catalog.count()",
			ResolvedObjectName: "count", DatabaseOID: 1, ServerVersionNum: 170000,
		},
	}}}}
	svc := phase1Service(t, resolver, policy)
	result, err := svc.Analyze(context.Background(), QueryAccessRequest{SQL: "SELECT count(*) FROM public.users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public"})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.DomainResult.ReadClassification != domain.ReadOnly || result.DomainResult.Admission != domain.Admissible {
		t.Fatalf("count(*) was not promoted: classification=%q admission=%q", result.DomainResult.ReadClassification, result.DomainResult.Admission)
	}
}

func TestPureEffectProofGateway_IneligibleFunctionNeverPromotes(t *testing.T) {
	policy := phase1Policy(t, TrustedEffectEntry{
		Kind: EffectCandidateFunction, ObjectOID: 2803, NamespaceOID: 11, ResultTypeOID: 20,
		Volatility: EffectVolatilityImmutable, CanonicalSignature: "pg_catalog.count()",
	})
	resolver := &mockControlledResolver{ctx: testResolutionContext(), batch: EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0, Status: domain.IdentityStatusResolved, Facts: &EffectIdentityFacts{
			Kind: EffectCandidateFunction, ObjectOID: 2803, NamespaceOID: 11, ResultTypeOID: 20,
			Volatility: EffectVolatilityImmutable, CanonicalSignature: "pg_catalog.count()",
			ResolvedObjectName: "count", DatabaseOID: 1, ServerVersionNum: 170000,
		},
	}}}}
	svc := phase1Service(t, resolver, policy)
	for _, sql := range []string{
		"SELECT count(*) FILTER (WHERE true) FROM public.users",
		"SELECT count(DISTINCT id) FROM public.users",
	} {
		result, err := svc.Analyze(context.Background(), QueryAccessRequest{SQL: sql, Dialect: "postgresql", Mode: "strict", DefaultSchema: "public"})
		if err != nil {
			t.Fatalf("Analyze %q: %v", sql, err)
		}
		if result.DomainResult.ReadClassification == domain.ReadOnly || result.DomainResult.Admission == domain.Admissible {
			t.Fatalf("ineligible function promoted for %q", sql)
		}
		if !hasReason(result.DomainResult.ReasonCodes, domain.ReasonUnprovenFunctionEffect) {
			t.Fatalf("unproven_function_effect removed for %q: %v", sql, result.DomainResult.ReasonCodes)
		}
	}
}

func TestPureEffectProofGateway_PartialBatchKeepsFunctionReasonAndNoLeak(t *testing.T) {
	extracted, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{SQL: "SELECT count(*) FROM public.users WHERE id = 1", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public"})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	operatorEntry := TrustedEffectEntry{Kind: EffectCandidateOperator, ObjectOID: 96, NamespaceOID: 11, OperandTypeOIDs: []uint32{23, 23}, ResultTypeOID: 16, ImplementationOID: 65, Volatility: EffectVolatilityImmutable, CanonicalSignature: "pg_catalog.=(23,23)"}
	resolver := &mockControlledResolver{ctx: testResolutionContext(), typeMap: map[int][]uint32{}}
	for _, candidate := range extracted.EffectCandidates {
		if candidate.Kind == EffectCandidateOperator {
			resolver.typeMap[candidate.Ordinal] = []uint32{23, 23}
			resolver.batch.Items = append(resolver.batch.Items, EffectIdentityItem{Ordinal: candidate.Ordinal, Status: domain.IdentityStatusResolved, Facts: &EffectIdentityFacts{Kind: EffectCandidateOperator, ObjectOID: 96, NamespaceOID: 11, OperandTypeOIDs: []uint32{23, 23}, ResultTypeOID: 16, ImplementationOID: 65, Volatility: EffectVolatilityImmutable, CanonicalSignature: operatorEntry.CanonicalSignature, ResolvedObjectName: "=", DatabaseOID: 1, ServerVersionNum: 170000}})
		} else {
			resolver.batch.Items = append(resolver.batch.Items, EffectIdentityItem{Ordinal: candidate.Ordinal, Status: domain.IdentityStatusUnknown})
		}
	}
	svc := phase1Service(t, resolver, phase1Policy(t, operatorEntry))
	result, err := svc.Analyze(context.Background(), QueryAccessRequest{SQL: "SELECT count(*) FROM public.users WHERE id = 1", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public"})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.DomainResult.ReadClassification == domain.ReadOnly || result.DomainResult.Admission == domain.Admissible {
		t.Fatal("partial proof incorrectly promoted")
	}
	if !hasReason(result.DomainResult.ReasonCodes, domain.ReasonUnprovenFunctionEffect) {
		t.Fatalf("unproven_function_effect missing: %v", result.DomainResult.ReasonCodes)
	}
	data, err := json.Marshal(result.DomainResult)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "effect_candidates") || strings.Contains(string(data), "OperandColumnRefs") {
		t.Fatalf("candidate facts leaked: %s", data)
	}
}

func phase1Policy(t *testing.T, entries ...TrustedEffectEntry) *TrustPolicy {
	t.Helper()
	manifest := TrustedEffectManifest{SchemaVersion: "test", PostgreSQLMajorMin: 17, PostgreSQLMajorMax: 17, Entries: entries}
	manifest.Hash = ComputeManifestHash(entries)
	policy, err := NewTrustPolicy(manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}
	return policy
}

func phase1Service(t *testing.T, resolver *mockControlledResolver, policy *TrustPolicy) *Service {
	t.Helper()
	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}
	return svc
}

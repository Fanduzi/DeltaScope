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

func TestScalarProof_DirectColumnCatalogOverloadPromotes(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		column    string
		typeOID   uint32
		objectOID uint32
		resultOID uint32
		signature string
		function  string
	}{
		{
			name: "text", sql: "SELECT lower(name) FROM public.users", column: "name", typeOID: 25,
			objectOID: 870, resultOID: 25, signature: "pg_catalog.lower(25)", function: "lower",
		},
		{
			name: "numeric", sql: "SELECT abs(amount) FROM public.orders", column: "amount", typeOID: 1700,
			objectOID: 1705, resultOID: 1700, signature: "pg_catalog.abs(1700)", function: "abs",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := TrustedEffectEntry{
				Kind:               EffectCandidateFunction,
				ObjectOID:          tc.objectOID,
				NamespaceOID:       11,
				OperandTypeOIDs:    []uint32{tc.typeOID},
				ResultTypeOID:      tc.resultOID,
				Volatility:         EffectVolatilityImmutable,
				CanonicalSignature: tc.signature,
			}
			resolver := &mockControlledResolver{
				ctx:     testResolutionContext(),
				typeMap: map[int][]uint32{0: {tc.typeOID}},
				batch: EffectIdentityBatch{Items: []EffectIdentityItem{{
					Ordinal: 0,
					Status:  domain.IdentityStatusResolved,
					Facts: &EffectIdentityFacts{
						Kind:               EffectCandidateFunction,
						ObjectOID:          tc.objectOID,
						NamespaceOID:       11,
						OperandTypeOIDs:    []uint32{tc.typeOID},
						ResultTypeOID:      tc.resultOID,
						Volatility:         EffectVolatilityImmutable,
						CanonicalSignature: tc.signature,
						ResolvedObjectName: tc.function,
						ResolvedSchemaName: "pg_catalog",
						DatabaseOID:        1,
						ServerVersionNum:   170000,
					},
				}}},
			}
			svc, err := NewTrustedService(resolver, phase1Policy(t, entry), scalarProofSchemaResolver{column: tc.column, typeOID: tc.typeOID})
			if err != nil {
				t.Fatalf("NewTrustedService: %v", err)
			}
			result, err := svc.Analyze(context.Background(), QueryAccessRequest{SQL: tc.sql, Dialect: "postgresql", Mode: "strict", DefaultSchema: "public"})
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if result.DomainResult.ReadClassification != domain.ReadOnly || result.DomainResult.Admission != domain.Admissible {
				t.Fatalf("classification=%q admission=%q, want read_only/admissible", result.DomainResult.ReadClassification, result.DomainResult.Admission)
			}
		})
	}
}

func TestScalarProof_VariadicPolymorphicPromotes(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		column    string
		typeOID   uint32
		objectOID uint32
		resultOID uint32
		signature string
		function  string
	}{
		{
			name: "coalesce variadic", sql: "SELECT COALESCE(name, email) FROM public.users", column: "name", typeOID: 25,
			objectOID: 840, resultOID: 2276, signature: "pg_catalog.coalesce(2276)", function: "coalesce",
		},
		{
			name: "nullif polymorphic", sql: "SELECT NULLIF(name, email) FROM public.users", column: "name", typeOID: 25,
			objectOID: 1706, resultOID: 2276, signature: "pg_catalog.nullif(2276,2276)", function: "nullif",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Manifest entry uses polymorphic type OIDs (2276) to match resolved facts.
			entry := TrustedEffectEntry{
				Kind:               EffectCandidateFunction,
				ObjectOID:          tc.objectOID,
				NamespaceOID:       11,
				OperandTypeOIDs:    []uint32{2276},
				ResultTypeOID:      tc.resultOID,
				Volatility:         EffectVolatilityImmutable,
				CanonicalSignature: tc.signature,
			}
			resolver := &mockControlledResolver{
				ctx:     testResolutionContext(),
				typeMap: map[int][]uint32{0: {tc.typeOID, tc.typeOID}},
				batch: EffectIdentityBatch{Items: []EffectIdentityItem{{
					Ordinal: 0,
					Status:  domain.IdentityStatusResolved,
					Facts: &EffectIdentityFacts{
						Kind:               EffectCandidateFunction,
						ObjectOID:          tc.objectOID,
						NamespaceOID:       11,
						OperandTypeOIDs:    []uint32{2276},
						ResultTypeOID:      tc.resultOID,
						Volatility:         EffectVolatilityImmutable,
						CanonicalSignature: tc.signature,
						ResolvedObjectName: tc.function,
						ResolvedSchemaName: "pg_catalog",
						DatabaseOID:        1,
						ServerVersionNum:   170000,
					},
				}}},
			}
			svc, err := NewTrustedService(resolver, phase1Policy(t, entry), scalarProofSchemaResolver{column: tc.column, typeOID: tc.typeOID})
			if err != nil {
				t.Fatalf("NewTrustedService: %v", err)
			}
			result, err := svc.Analyze(context.Background(), QueryAccessRequest{SQL: tc.sql, Dialect: "postgresql", Mode: "strict", DefaultSchema: "public"})
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			t.Logf("classification=%q admission=%q reasons=%v candidates=%d",
				result.DomainResult.ReadClassification, result.DomainResult.Admission,
				result.DomainResult.ReasonCodes, len(result.EffectCandidates))
			if result.DomainResult.ReadClassification != domain.ReadOnly || result.DomainResult.Admission != domain.Admissible {
				t.Fatalf("classification=%q admission=%q, want read_only/admissible", result.DomainResult.ReadClassification, result.DomainResult.Admission)
			}
		})
	}
}

type scalarProofSchemaResolver struct {
	column  string
	typeOID uint32
}

func (r scalarProofSchemaResolver) ResolveRelation(_ context.Context, _ string, schema, name string) (RelationSchema, error) {
	return RelationSchema{
		Schema: schema,
		Name:   name,
		Kind:   "table",
		Columns: []ColumnSchema{
			{Name: r.column, Ordinal: 1, TypeOID: r.typeOID},
			{Name: "email", Ordinal: 2, TypeOID: 25},
			{Name: "id", Ordinal: 3, TypeOID: 23},
		},
	}, nil
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

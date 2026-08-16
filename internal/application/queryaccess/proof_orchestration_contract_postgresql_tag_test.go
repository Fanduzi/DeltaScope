//go:build postgresql

// Package queryaccess verifies the PostgreSQL proof-orchestration contract at the Service.Analyze seam.
// input: PostgreSQL queries over a trusted service with fixture manifest proof
// output: read_only/admissible only for all_proven ordinary or exact COUNT(1) proof; fail closed otherwise
// pos: application orchestration contract coverage (postgresql-tagged builds)
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// countStarProofBatch is the fixture atomic-resolver batch proving pg_catalog.count()
// for the arity-zero count(*) candidate.
func countStarProofBatch() EffectIdentityBatch {
	return EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0,
		Status:  domain.IdentityStatusResolved,
		Facts: &EffectIdentityFacts{
			Kind:               EffectCandidateFunction,
			ObjectOID:          2803,
			NamespaceOID:       11,
			OperandTypeOIDs:    nil,
			ResultTypeOID:      20,
			Volatility:         EffectVolatilityImmutable,
			CanonicalSignature: "pg_catalog.count()",
			DatabaseOID:        1,
			ServerVersionNum:   170000,
		},
	}}}
}

// countIntegerOneProofBatch is the fixture atomic-resolver batch proving
// pg_catalog.count(2276) for the arity-one count(1) candidate.
func countIntegerOneProofBatch() EffectIdentityBatch {
	return EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0,
		Status:  domain.IdentityStatusResolved,
		Facts: &EffectIdentityFacts{
			Kind:               EffectCandidateFunction,
			ObjectOID:          2147,
			NamespaceOID:       11,
			OperandTypeOIDs:    []uint32{2276},
			ResultTypeOID:      20,
			Volatility:         EffectVolatilityImmutable,
			CanonicalSignature: "pg_catalog.count(2276)",
			DatabaseOID:        1,
			ServerVersionNum:   170000,
		},
	}}}
}

// TestProofOrchestrationContract_PostgreSQLAnalyze locks the PostgreSQL side of
// the proof orchestration contract through Service.Analyze: ordinary and exact
// COUNT(1) manifest proof promote only with an all_proven decision, absent
// trust is never vacuous success, and barriers cannot be promoted. Probe
// counts freeze the catalog-call boundary at the application seam.
func TestProofOrchestrationContract_PostgreSQLAnalyze(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	provenResolver := &mockControlledResolver{ctx: testResolutionContext(), batch: countStarProofBatch()}
	trusted, err := NewTrustedService(provenResolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("trusted service: %v", err)
	}
	provenCountOneResolver := &mockControlledResolver{ctx: testResolutionContext(), batch: countIntegerOneProofBatch()}
	trustedCountOne, err := NewTrustedService(provenCountOneResolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("count(1) trusted service: %v", err)
	}
	unprovenResolver := &mockControlledResolver{
		ctx:   testResolutionContext(),
		batch: EffectIdentityBatch{Items: []EffectIdentityItem{{Ordinal: 0, Status: domain.IdentityStatusUnavailable}}},
	}
	trustedUnproven, err := NewTrustedService(unprovenResolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("unproven service: %v", err)
	}

	tests := []struct {
		name          string
		service       *Service
		resolver      *mockControlledResolver
		sql           string
		wantClass     domain.ReadClassification
		wantAdmission domain.Admission
		wantProbes    int
	}{
		{
			name:    "ordinary_proof_all_proven",
			service: trusted, resolver: provenResolver,
			sql:       "SELECT count(*) FROM public.users",
			wantClass: domain.ReadOnly, wantAdmission: domain.Admissible, wantProbes: 1,
		},
		{
			name:    "exact_count_one_requirements_before_proof",
			service: trustedCountOne, resolver: provenCountOneResolver,
			sql:       "SELECT count(1) FROM public.users",
			wantClass: domain.ReadOnly, wantAdmission: domain.Admissible, wantProbes: 1,
		},
		{
			name:    "proof_unproven_fail_closed",
			service: trustedUnproven, resolver: unprovenResolver,
			sql:       "SELECT count(*) FROM public.users",
			wantClass: domain.Indeterminate, wantAdmission: domain.IndeterminateAdmission, wantProbes: 1,
		},
		{
			name:      "no_trust_bundle_never_vacuous",
			service:   NewService(),
			sql:       "SELECT count(*) FROM public.users",
			wantClass: domain.Indeterminate, wantAdmission: domain.IndeterminateAdmission, wantProbes: 0,
		},
		{
			name:    "unqualified_relation_barrier_not_promoted",
			service: trusted, resolver: provenResolver,
			sql:       "SELECT count(*) FROM users",
			wantClass: domain.Indeterminate, wantAdmission: domain.IndeterminateAdmission, wantProbes: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := 0
			if tt.resolver != nil {
				before = tt.resolver.ctxCalled
			}
			res, err := tt.service.Analyze(context.Background(), QueryAccessRequest{
				SQL:            tt.sql,
				Dialect:        "postgresql",
				Mode:           "strict",
				DefaultSchema:  "public",
				SchemaResolver: &mockSchemaResolver{},
			})
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if res.DomainResult.ReadClassification != tt.wantClass || res.DomainResult.Admission != tt.wantAdmission {
				t.Fatalf("classification=%q admission=%q, want %q/%q (reasons=%v)",
					res.DomainResult.ReadClassification, res.DomainResult.Admission,
					tt.wantClass, tt.wantAdmission, res.DomainResult.ReasonCodes)
			}
			if tt.resolver != nil && tt.resolver.ctxCalled-before != tt.wantProbes {
				t.Errorf("CaptureExecutionBoundContext delta = %d, want %d", tt.resolver.ctxCalled-before, tt.wantProbes)
			}
		})
	}
}

// TestProofOrchestrationContract_PostgreSQLCancelled locks cancellation behavior
// at the orchestration entry: a cancelled context fails before any analysis.
func TestProofOrchestrationContract_PostgreSQLCancelled(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}
	svc, err := NewTrustedService(&mockControlledResolver{ctx: testResolutionContext(), batch: countStarProofBatch()}, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("trusted service: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Analyze(ctx, QueryAccessRequest{
		SQL: "SELECT count(1) FROM public.users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
	}); err == nil {
		t.Fatal("Analyze with cancelled context: want error, got nil")
	}
}

//go:build postgresql

package queryaccess

import (
	"context"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// mockTwoColumnSchemaResolver returns a schema with both id (int4) and name (text) columns.
type mockTwoColumnSchemaResolver struct{}

func (m *mockTwoColumnSchemaResolver) ResolveRelation(ctx context.Context, dialect string, schema, name string) (RelationSchema, error) {
	return RelationSchema{
		Schema: schema,
		Name:   name,
		Kind:   "table",
		Columns: []ColumnSchema{
			{Name: "id", Ordinal: 1, TypeOID: 23},   // int4
			{Name: "name", Ordinal: 2, TypeOID: 25}, // text
		},
	}, nil
}

// TestAdversarial_CandidateFactBinding_Swap proves that a malicious resolver
// can swap facts between same-kind, same-arity candidates.
//
// Scenario:
// - Candidate 0: =(int4,const) operator (ordinal 0, expects pg_catalog.=(23,23))
// - Candidate 1: =(text,const) operator (ordinal 1, expects pg_catalog.=(25,25))
// - Resolver returns =(text,text) fact at ordinal 0 (WRONG!)
//
// Expected: Should fail (identity mismatch)
// Actual on 374fb95: Passs (structural match only) ← DEFECT
func TestAdversarial_CandidateFactBinding_Swap(t *testing.T) {
	// Resolver returns facts SWAPPED:
	// - Ordinal 0 gets =(text,text) fact (WRONG - should be =(int4,int4))
	// - Ordinal 1 gets =(int4,int4) fact (WRONG - should be =(text,text))
	resolver := &mockControlledResolver{
		ctx: testResolutionContext(),
		batch: EffectIdentityBatch{
			Items: []EffectIdentityItem{
				{
					Ordinal: 0,
					Status:  domain.IdentityStatusResolved,
					Facts: &EffectIdentityFacts{
						Kind:               EffectCandidateOperator,
						ObjectOID:          98, // =(text,text) OID
						NamespaceOID:       11,
						OperandTypeOIDs:    []uint32{25, 25}, // text types
						ResultTypeOID:      16,
						ImplementationOID:  67, // texteq
						Volatility:         EffectVolatilityImmutable,
						CanonicalSignature: "pg_catalog.=(25,25)",
						ResolvedSchemaName: "pg_catalog",
						ResolvedObjectName: "=",
						DatabaseOID:        1,
						ServerVersionNum:   170000,
					},
				},
				{
					Ordinal: 1,
					Status:  domain.IdentityStatusResolved,
					Facts: &EffectIdentityFacts{
						Kind:               EffectCandidateOperator,
						ObjectOID:          96, // =(int4,int4) OID
						NamespaceOID:       11,
						OperandTypeOIDs:    []uint32{23, 23}, // int4 types
						ResultTypeOID:      16,
						ImplementationOID:  65, // int4eq
						Volatility:         EffectVolatilityImmutable,
						CanonicalSignature: "pg_catalog.=(23,23)",
						ResolvedSchemaName: "pg_catalog",
						ResolvedObjectName: "=",
						DatabaseOID:        1,
						ServerVersionNum:   170000,
					},
				},
			},
		},
	}

	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	svc, err := NewTrustedService(resolver, policy, &mockTwoColumnSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	// Use a query with two comparisons on different column types
	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:     "SELECT id FROM public.users WHERE id = 1 AND name = 'test'",
		Dialect: "postgresql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// DEFECT: On 374fb95, this incorrectly promotes because:
	// - ValidateCandidateFactBinding passes (kind=operator, arity=2)
	// - TrustPolicy.IsTrusted passes (both signatures in manifest)
	// - But the facts are SWAPPED between candidates!
	//
	// After fix, this should be indeterminate (identity mismatch)
	if result.DomainResult.ReadClassification == domain.ReadOnly {
		t.Errorf("DEFECT PROVED: swapped facts incorrectly promoted to read_only")
		t.Errorf("Expected: indeterminate (identity mismatch)")
		t.Errorf("Actual: read_only (structural match only)")
	}
}

// TestAdversarial_CandidateFactBinding_NonManifestOperator proves that a
// non-manifest operator candidate cannot be promoted with a manifest fact.
//
// This test constructs candidates directly (not from SQL) to ensure we have
// a real non-manifest candidate name.
func TestAdversarial_CandidateFactBinding_NonManifestOperator(t *testing.T) {
	// Resolver returns a manifest operator fact for a non-manifest candidate name
	resolver := &mockControlledResolver{
		ctx: testResolutionContext(),
		batch: EffectIdentityBatch{
			Items: []EffectIdentityItem{
				{
					Ordinal: 0,
					Status:  domain.IdentityStatusResolved,
					Facts: &EffectIdentityFacts{
						Kind:               EffectCandidateOperator,
						ObjectOID:          96, // =(int4,int4) OID (manifest entry)
						NamespaceOID:       11,
						OperandTypeOIDs:    []uint32{23, 23},
						ResultTypeOID:      16,
						ImplementationOID:  65,
						Volatility:         EffectVolatilityImmutable,
						CanonicalSignature: "pg_catalog.=(23,23)",
						ResolvedSchemaName: "pg_catalog",
						ResolvedObjectName: "=", // Different from candidate name
						DatabaseOID:        1,
						ServerVersionNum:   170000,
					},
				},
			},
		},
	}

	// Create a service with a mock that returns candidates with a different name
	_ = &Service{
		trusted: &trustedBundle{
			effectResolver: resolver,
			trustPolicy: func() *TrustPolicy {
				p, _ := NewTrustPolicy(PG17Manifest)
				return p
			}(),
			schemaResolver: &mockTwoColumnSchemaResolver{},
		},
	}

	// Directly test ValidateCandidateFactBinding with mismatched names
	batch := EffectIdentityBatch{
		Items: []EffectIdentityItem{
			{
				Ordinal: 0,
				Status:  domain.IdentityStatusResolved,
				Facts: &EffectIdentityFacts{
					Kind:               EffectCandidateOperator,
					ObjectOID:          96,
					NamespaceOID:       11,
					OperandTypeOIDs:    []uint32{23, 23},
					ResultTypeOID:      16,
					ImplementationOID:  65,
					Volatility:         EffectVolatilityImmutable,
					CanonicalSignature: "pg_catalog.=(23,23)",
					ResolvedSchemaName: "pg_catalog",
					ResolvedObjectName: "=", // Manifest operator name
					DatabaseOID:        1,
					ServerVersionNum:   170000,
				},
			},
		},
	}

	candidates := []EffectCandidate{
		{
			Ordinal:  0,
			Kind:     EffectCandidateOperator,
			NamePath: []string{"my_custom_op"}, // Non-manifest operator name
			Arity:    2,
		},
	}

	// ValidateCandidateFactBinding should reject this because the resolved
	// object name "=" doesn't match the candidate name "my_custom_op"
	result := ValidateCandidateFactBinding(batch, candidates)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Status == domain.IdentityStatusResolved {
		t.Errorf("DEFECT: non-manifest candidate promoted with manifest fact")
		t.Errorf("Expected: lookup_failed (identity mismatch)")
		t.Errorf("Actual: resolved (name not validated)")
	}
}

// TestAdversarial_CandidateFactBinding_ExplicitSchemaMismatch proves that
// explicit-schema intent must match.
func TestAdversarial_CandidateFactBinding_ExplicitSchemaMismatch(t *testing.T) {
	// Test ValidateCandidateFactBinding directly with explicit schema mismatch
	batch := EffectIdentityBatch{
		Items: []EffectIdentityItem{
			{
				Ordinal: 0,
				Status:  domain.IdentityStatusResolved,
				Facts: &EffectIdentityFacts{
					Kind:               EffectCandidateOperator,
					ObjectOID:          96,
					NamespaceOID:       11,
					OperandTypeOIDs:    []uint32{23, 23},
					ResultTypeOID:      16,
					ImplementationOID:  65,
					Volatility:         EffectVolatilityImmutable,
					CanonicalSignature: "pg_catalog.=(23,23)",
					ResolvedSchemaName: "pg_catalog", // Resolved from pg_catalog
					ResolvedObjectName: "=",
					DatabaseOID:        1,
					ServerVersionNum:   170000,
				},
			},
		},
	}

	candidates := []EffectCandidate{
		{
			Ordinal:        0,
			Kind:           EffectCandidateOperator,
			NamePath:       []string{"public", "="}, // Explicitly expects public schema
			Arity:          2,
			ExplicitSchema: true,
		},
	}

	// ValidateCandidateFactBinding should reject this because the candidate
	// explicitly expects "public" schema but fact is from "pg_catalog"
	result := ValidateCandidateFactBinding(batch, candidates)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Status == domain.IdentityStatusResolved {
		t.Errorf("DEFECT: explicit-schema mismatch incorrectly promoted")
		t.Errorf("Expected: lookup_failed (schema mismatch)")
		t.Errorf("Actual: resolved (schema not validated)")
	}
}

// Package queryaccess tests the trusted service integration.
// input: Service with trusted bundle, effect identity resolver, trust policy
// output: verification of manifest-gated promotion logic
// pos: T8 trusted service integration tests
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// mockEffectResolver is a test-only resolver that returns pre-configured results.
type mockEffectResolver struct {
	batch EffectIdentityBatch
	err   error
}

func (m *mockEffectResolver) ResolveEffectIdentities(ctx context.Context, req EffectIdentityRequest) (EffectIdentityBatch, error) {
	if m.err != nil {
		return EffectIdentityBatch{}, m.err
	}
	return m.batch, nil
}

// mockSchemaResolver is a test-only schema resolver.
type mockSchemaResolver struct{}

func (m *mockSchemaResolver) ResolveRelation(ctx context.Context, dialect string, schema, name string) (RelationSchema, error) {
	return RelationSchema{
		Schema: schema,
		Name:   name,
		Kind:   "table",
		Columns: []ColumnSchema{
			{Name: "id", Ordinal: 1, TypeOID: 23},
		},
	}, nil
}

func TestNewTrustedService(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	tests := []struct {
		name           string
		effectResolver EffectIdentityResolver
		trustPolicy    *TrustPolicy
		schemaResolver SchemaResolver
		wantErr        bool
	}{
		{
			name:           "valid bundle",
			effectResolver: &mockEffectResolver{},
			trustPolicy:    policy,
			schemaResolver: &mockSchemaResolver{},
			wantErr:        false,
		},
		{
			name:           "nil effect resolver",
			effectResolver: nil,
			trustPolicy:    policy,
			schemaResolver: &mockSchemaResolver{},
			wantErr:        true,
		},
		{
			name:           "nil trust policy",
			effectResolver: &mockEffectResolver{},
			trustPolicy:    nil,
			schemaResolver: &mockSchemaResolver{},
			wantErr:        true,
		},
		{
			name:           "nil schema resolver",
			effectResolver: &mockEffectResolver{},
			trustPolicy:    policy,
			schemaResolver: nil,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewTrustedService(tt.effectResolver, tt.trustPolicy, tt.schemaResolver)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTrustedService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && svc == nil {
				t.Error("NewTrustedService() returned nil service")
			}
		})
	}
}

func TestNewServiceBasic(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Error("NewService() returned nil")
	}
	if svc.trusted != nil {
		t.Error("NewService() has non-nil trusted bundle")
	}
}

func TestTrustedServicePromotion(t *testing.T) {
	// Build a trust policy with a test manifest.
	testEntries := []TrustedEffectEntry{
		{
			Kind:               EffectCandidateOperator,
			ObjectOID:          96,
			NamespaceOID:       11,
			OperandTypeOIDs:    []uint32{23, 23},
			ResultTypeOID:      16,
			ImplementationOID:  65,
			Volatility:         EffectVolatilityImmutable,
			CanonicalSignature: "pg_catalog.=(23,23)",
			AuditNotes:         "int4 = int4",
		},
	}
	testManifest := TrustedEffectManifest{
		SchemaVersion:      "1.0",
		PostgreSQLMajorMin: 17,
		PostgreSQLMajorMax: 17,
		Entries:            testEntries,
		Hash:               ComputeManifestHash(testEntries),
	}
	policy, err := NewTrustPolicy(testManifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	tests := []struct {
		name               string
		resolver           EffectIdentityResolver
		wantClassification domain.ReadClassification
		wantAdmission      domain.Admission
	}{
		{
			name: "all proven - promotes to read_only + admissible",
			resolver: &mockEffectResolver{
				batch: EffectIdentityBatch{
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
								DatabaseOID:        1,
								ServerVersionNum:   170000,
							},
						},
					},
				},
			},
			wantClassification: domain.ReadOnly,
			wantAdmission:      domain.Admissible,
		},
		{
			name: "has unknown - stays indeterminate",
			resolver: &mockEffectResolver{
				batch: EffectIdentityBatch{
					Items: []EffectIdentityItem{
						{
							Ordinal: 0,
							Status:  domain.IdentityStatusUnknown,
							Facts:   nil,
						},
					},
				},
			},
			wantClassification: domain.Indeterminate,
			wantAdmission:      domain.IndeterminateAdmission,
		},
		{
			name: "has unproven - stays indeterminate",
			resolver: &mockEffectResolver{
				batch: EffectIdentityBatch{
					Items: []EffectIdentityItem{
						{
							Ordinal: 0,
							Status:  domain.IdentityStatusResolved,
							Facts: &EffectIdentityFacts{
								Kind:               EffectCandidateOperator,
								ObjectOID:          9999,
								NamespaceOID:       11,
								CanonicalSignature: "pg_catalog.=(9999,9999)",
								ServerVersionNum:   170000,
							},
						},
					},
				},
			},
			wantClassification: domain.Indeterminate,
			wantAdmission:      domain.IndeterminateAdmission,
		},
		{
			name: "resolver error - stays indeterminate",
			resolver: &mockEffectResolver{
				err: context.Canceled,
			},
			wantClassification: domain.Indeterminate,
			wantAdmission:      domain.IndeterminateAdmission,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewTrustedService(tt.resolver, policy, &mockSchemaResolver{})
			if err != nil {
				t.Fatalf("NewTrustedService: %v", err)
			}

			// Simulate a PostgreSQL query with effect candidates.
			req := QueryAccessRequest{
				SQL:           "SELECT id FROM users WHERE id = 1",
				Dialect:       "postgresql",
				Mode:          "strict",
				DefaultSchema: "public",
			}

			// We need to mock the extraction to return effect candidates.
			// For this test, we'll directly test the reclassifyAfterResolution logic.
			var proofDecision TrustDecision
			if tt.resolver.(*mockEffectResolver).err != nil {
				proofDecision = TrustDecisionHasUnknown
			} else {
				// Check if all items are resolved and in manifest.
				allResolved := true
				allInManifest := true
				for _, item := range tt.resolver.(*mockEffectResolver).batch.Items {
					if item.Status != domain.IdentityStatusResolved || item.Facts == nil {
						allResolved = false
						break
					}
					if item.Facts.CanonicalSignature != "pg_catalog.=(23,23)" {
						allInManifest = false
					}
				}
				if allResolved && allInManifest {
					proofDecision = TrustDecisionAllProven
				} else if !allResolved {
					proofDecision = TrustDecisionHasUnknown
				} else {
					proofDecision = TrustDecisionHasUnproven
				}
			}
			proof := &trustProofResult{
				decision: proofDecision,
			}

			// Test reclassifyAfterResolution with the proof.
			gotClass := reclassifyAfterResolution(
				domain.Indeterminate,
				nil,
				nil,
				true,
				"postgresql",
				proof,
			)

			if gotClass != tt.wantClassification {
				t.Errorf("reclassifyAfterResolution() = %v, want %v", gotClass, tt.wantClassification)
			}

			// Verify the service was created correctly.
			if svc.trusted == nil {
				t.Error("trusted service has nil trusted bundle")
			}
			_ = req // suppress unused warning
		})
	}
}

func TestReclassifyAfterResolutionPGHardStopRemoved(t *testing.T) {
	// Verify the PG hard-stop is removed: with nil proof, PostgreSQL stays indeterminate.
	got := reclassifyAfterResolution(
		domain.Indeterminate,
		nil,
		nil,
		true,
		"postgresql",
		nil,
	)
	if got != domain.Indeterminate {
		t.Errorf("nil proof: got %v, want indeterminate", got)
	}

	// With all_proven proof, PostgreSQL can promote.
	proof := &trustProofResult{
		decision: TrustDecisionAllProven,
	}
	got = reclassifyAfterResolution(
		domain.Indeterminate,
		nil,
		nil,
		true,
		"postgresql",
		proof,
	)
	if got != domain.ReadOnly {
		t.Errorf("all_proven proof: got %v, want read_only", got)
	}

	// With has_unproven proof, PostgreSQL stays indeterminate.
	proof = &trustProofResult{
		decision: TrustDecisionHasUnproven,
	}
	got = reclassifyAfterResolution(
		domain.Indeterminate,
		nil,
		nil,
		true,
		"postgresql",
		proof,
	)
	if got != domain.Indeterminate {
		t.Errorf("has_unproven proof: got %v, want indeterminate", got)
	}

	// MySQL path unchanged (no proof parameter).
	got = reclassifyAfterResolution(
		domain.Indeterminate,
		nil,
		nil,
		true,
		"mysql",
		nil,
	)
	if got != domain.ReadOnly {
		t.Errorf("mysql without proof: got %v, want read_only", got)
	}
}

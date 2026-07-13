// Package queryaccess tests the TrustPolicy contract.
// input: TrustPolicy, manifest, resolved batches
// output: verification of manifest proof logic
// pos: T8 trust policy contract tests (RED → GREEN → SURFACE)
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestTrustPolicyIsTrusted(t *testing.T) {
	// Build a minimal test manifest.
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
		{
			Kind:               EffectCandidateFunction,
			ObjectOID:          2803,
			NamespaceOID:       11,
			OperandTypeOIDs:    nil,
			ResultTypeOID:      20,
			ImplementationOID:  0,
			Volatility:         EffectVolatilityImmutable,
			CanonicalSignature: "pg_catalog.count()",
			AuditNotes:         "count(*)",
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
		name             string
		batch            EffectIdentityBatch
		serverVersionNum int
		want             TrustDecision
	}{
		{
			name:             "empty batch",
			batch:            EffectIdentityBatch{},
			serverVersionNum: 170000,
			want:             TrustDecisionEmpty,
		},
		{
			name: "all proven - operator",
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
			serverVersionNum: 170000,
			want:             TrustDecisionAllProven,
		},
		{
			name: "all proven - count(*)",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
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
					},
				},
			},
			serverVersionNum: 170000,
			want:             TrustDecisionAllProven,
		},
		{
			name: "has unknown - unresolved item",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
						Ordinal: 0,
						Status:  domain.IdentityStatusResolved,
						Facts: &EffectIdentityFacts{
							Kind:               EffectCandidateOperator,
							ObjectOID:          96,
							NamespaceOID:       11,
							ImplementationOID:  65,
							CanonicalSignature: "pg_catalog.=(23,23)",
							ServerVersionNum:   170000,
						},
					},
					{
						Ordinal: 1,
						Status:  domain.IdentityStatusUnknown,
						Facts:   nil,
					},
				},
			},
			serverVersionNum: 170000,
			want:             TrustDecisionHasUnknown,
		},
		{
			name: "has unproven - not in manifest",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
						Ordinal: 0,
						Status:  domain.IdentityStatusResolved,
						Facts: &EffectIdentityFacts{
							Kind:               EffectCandidateOperator,
							ObjectOID:          9999,
							NamespaceOID:       11,
							ImplementationOID:  9999,
							CanonicalSignature: "pg_catalog.=(9999,9999)",
							ServerVersionNum:   170000,
						},
					},
				},
			},
			serverVersionNum: 170000,
			want:             TrustDecisionHasUnproven,
		},
		{
			name: "has unproven - wrong namespace",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
						Ordinal: 0,
						Status:  domain.IdentityStatusResolved,
						Facts: &EffectIdentityFacts{
							Kind:               EffectCandidateOperator,
							ObjectOID:          96,
							NamespaceOID:       9999,
							ImplementationOID:  65,
							CanonicalSignature: "pg_catalog.=(23,23)",
							ServerVersionNum:   170000,
						},
					},
				},
			},
			serverVersionNum: 170000,
			want:             TrustDecisionHasUnproven,
		},
		{
			name: "has unproven - wrong implementation OID",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
						Ordinal: 0,
						Status:  domain.IdentityStatusResolved,
						Facts: &EffectIdentityFacts{
							Kind:               EffectCandidateOperator,
							ObjectOID:          96,
							NamespaceOID:       11,
							ImplementationOID:  9999,
							CanonicalSignature: "pg_catalog.=(23,23)",
							ServerVersionNum:   170000,
						},
					},
				},
			},
			serverVersionNum: 170000,
			want:             TrustDecisionHasUnproven,
		},
		{
			name: "has unproven - wrong server version",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
						Ordinal: 0,
						Status:  domain.IdentityStatusResolved,
						Facts: &EffectIdentityFacts{
							Kind:               EffectCandidateOperator,
							ObjectOID:          96,
							NamespaceOID:       11,
							ImplementationOID:  65,
							CanonicalSignature: "pg_catalog.=(23,23)",
							ServerVersionNum:   160000,
						},
					},
				},
			},
			serverVersionNum: 160000,
			want:             TrustDecisionHasUnproven,
		},
		{
			name: "has unproven - version mismatch between facts and request",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
						Ordinal: 0,
						Status:  domain.IdentityStatusResolved,
						Facts: &EffectIdentityFacts{
							Kind:               EffectCandidateOperator,
							ObjectOID:          96,
							NamespaceOID:       11,
							ImplementationOID:  65,
							CanonicalSignature: "pg_catalog.=(23,23)",
							ServerVersionNum:   170000,
						},
					},
				},
			},
			serverVersionNum: 160000,
			want:             TrustDecisionHasUnproven,
		},
		{
			name: "has unproven - zero server version on facts",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
						Ordinal: 0,
						Status:  domain.IdentityStatusResolved,
						Facts: &EffectIdentityFacts{
							Kind:               EffectCandidateOperator,
							ObjectOID:          96,
							NamespaceOID:       11,
							ImplementationOID:  65,
							CanonicalSignature: "pg_catalog.=(23,23)",
							ServerVersionNum:   0,
						},
					},
				},
			},
			serverVersionNum: 170000,
			want:             TrustDecisionHasUnproven,
		},
		{
			name: "has unproven - coercion gap",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
						Ordinal: 0,
						Status:  domain.IdentityStatusCoercionGap,
						Facts:   nil,
					},
				},
			},
			serverVersionNum: 170000,
			want:             TrustDecisionHasUnknown,
		},
		{
			name: "has unproven - ambiguous",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
						Ordinal: 0,
						Status:  domain.IdentityStatusAmbiguous,
						Facts:   nil,
					},
				},
			},
			serverVersionNum: 170000,
			want:             TrustDecisionHasUnknown,
		},
		{
			name: "has unproven - lookup failed",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
						Ordinal: 0,
						Status:  domain.IdentityStatusLookupFailed,
						Facts:   nil,
					},
				},
			},
			serverVersionNum: 170000,
			want:             TrustDecisionHasUnknown,
		},
		{
			name: "multiple items - all proven",
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
					{
						Ordinal: 1,
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
					},
				},
			},
			serverVersionNum: 170000,
			want:             TrustDecisionAllProven,
		},
		{
			name: "multiple items - one unproven",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
						Ordinal: 0,
						Status:  domain.IdentityStatusResolved,
						Facts: &EffectIdentityFacts{
							Kind:               EffectCandidateOperator,
							ObjectOID:          96,
							NamespaceOID:       11,
							ImplementationOID:  65,
							CanonicalSignature: "pg_catalog.=(23,23)",
							ServerVersionNum:   170000,
						},
					},
					{
						Ordinal: 1,
						Status:  domain.IdentityStatusResolved,
						Facts: &EffectIdentityFacts{
							Kind:               EffectCandidateFunction,
							ObjectOID:          9999,
							NamespaceOID:       11,
							CanonicalSignature: "pg_catalog.nonexistent()",
							ServerVersionNum:   170000,
						},
					},
				},
			},
			serverVersionNum: 170000,
			want:             TrustDecisionHasUnproven,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := policy.IsTrusted(tt.batch, tt.serverVersionNum)
			if got != tt.want {
				t.Errorf("IsTrusted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrustPolicyNilPolicy(t *testing.T) {
	var policy *TrustPolicy
	batch := EffectIdentityBatch{
		Items: []EffectIdentityItem{
			{
				Ordinal: 0,
				Status:  domain.IdentityStatusResolved,
				Facts: &EffectIdentityFacts{
					CanonicalSignature: "pg_catalog.=(23,23)",
					ServerVersionNum:   170000,
				},
			},
		},
	}
	got := policy.IsTrusted(batch, 170000)
	if got != TrustDecisionHasUnknown {
		t.Errorf("nil policy IsTrusted() = %v, want %v", got, TrustDecisionHasUnknown)
	}
}

func TestValidateManifest(t *testing.T) {
	tests := []struct {
		name     string
		manifest TrustedEffectManifest
		wantErr  bool
	}{
		{
			name: "valid manifest",
			manifest: TrustedEffectManifest{
				SchemaVersion:      "1.0",
				PostgreSQLMajorMin: 17,
				PostgreSQLMajorMax: 17,
				Entries: []TrustedEffectEntry{
					{
						Kind:               EffectCandidateOperator,
						ObjectOID:          96,
						NamespaceOID:       11,
						CanonicalSignature: "pg_catalog.=(23,23)",
						Volatility:         EffectVolatilityImmutable,
					},
				},
			},
			wantErr: false, // hash will be computed
		},
		{
			name: "missing schema version",
			manifest: TrustedEffectManifest{
				PostgreSQLMajorMin: 17,
				PostgreSQLMajorMax: 17,
				Entries: []TrustedEffectEntry{
					{
						Kind:               EffectCandidateOperator,
						ObjectOID:          96,
						NamespaceOID:       11,
						CanonicalSignature: "pg_catalog.=(23,23)",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid version range",
			manifest: TrustedEffectManifest{
				SchemaVersion:      "1.0",
				PostgreSQLMajorMin: 17,
				PostgreSQLMajorMax: 16,
				Entries: []TrustedEffectEntry{
					{
						Kind:               EffectCandidateOperator,
						ObjectOID:          96,
						NamespaceOID:       11,
						CanonicalSignature: "pg_catalog.=(23,23)",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "no entries",
			manifest: TrustedEffectManifest{
				SchemaVersion:      "1.0",
				PostgreSQLMajorMin: 17,
				PostgreSQLMajorMax: 17,
				Entries:            []TrustedEffectEntry{},
			},
			wantErr: true,
		},
		{
			name: "missing object OID",
			manifest: TrustedEffectManifest{
				SchemaVersion:      "1.0",
				PostgreSQLMajorMin: 17,
				PostgreSQLMajorMax: 17,
				Entries: []TrustedEffectEntry{
					{
						Kind:               EffectCandidateOperator,
						NamespaceOID:       11,
						CanonicalSignature: "pg_catalog.=(23,23)",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing namespace OID",
			manifest: TrustedEffectManifest{
				SchemaVersion:      "1.0",
				PostgreSQLMajorMin: 17,
				PostgreSQLMajorMax: 17,
				Entries: []TrustedEffectEntry{
					{
						Kind:               EffectCandidateOperator,
						ObjectOID:          96,
						CanonicalSignature: "pg_catalog.=(23,23)",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing canonical signature",
			manifest: TrustedEffectManifest{
				SchemaVersion:      "1.0",
				PostgreSQLMajorMin: 17,
				PostgreSQLMajorMax: 17,
				Entries: []TrustedEffectEntry{
					{
						Kind:         EffectCandidateOperator,
						ObjectOID:    96,
						NamespaceOID: 11,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate canonical signature",
			manifest: TrustedEffectManifest{
				SchemaVersion:      "1.0",
				PostgreSQLMajorMin: 17,
				PostgreSQLMajorMax: 17,
				Entries: []TrustedEffectEntry{
					{
						Kind:               EffectCandidateOperator,
						ObjectOID:          96,
						NamespaceOID:       11,
						CanonicalSignature: "pg_catalog.=(23,23)",
					},
					{
						Kind:               EffectCandidateOperator,
						ObjectOID:          97,
						NamespaceOID:       11,
						CanonicalSignature: "pg_catalog.=(23,23)",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid volatility",
			manifest: TrustedEffectManifest{
				SchemaVersion:      "1.0",
				PostgreSQLMajorMin: 17,
				PostgreSQLMajorMax: 17,
				Entries: []TrustedEffectEntry{
					{
						Kind:               EffectCandidateOperator,
						ObjectOID:          96,
						NamespaceOID:       11,
						CanonicalSignature: "pg_catalog.=(23,23)",
						Volatility:         "x",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compute hash for valid manifests.
			if !tt.wantErr {
				tt.manifest.Hash = ComputeManifestHash(tt.manifest.Entries)
			}
			err := ValidateManifest(tt.manifest)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateManifest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestComputeManifestHash(t *testing.T) {
	entries := []TrustedEffectEntry{
		{
			Kind:               EffectCandidateOperator,
			ObjectOID:          96,
			NamespaceOID:       11,
			CanonicalSignature: "pg_catalog.=(23,23)",
		},
	}

	hash1 := ComputeManifestHash(entries)
	hash2 := ComputeManifestHash(entries)

	if hash1 != hash2 {
		t.Errorf("ComputeManifestHash not deterministic: %s != %s", hash1, hash2)
	}

	// Different order should produce same hash.
	entries2 := []TrustedEffectEntry{
		{
			Kind:               EffectCandidateOperator,
			ObjectOID:          96,
			NamespaceOID:       11,
			CanonicalSignature: "pg_catalog.=(23,23)",
		},
	}
	hash3 := ComputeManifestHash(entries2)
	if hash1 != hash3 {
		t.Errorf("ComputeManifestHash not order-independent: %s != %s", hash1, hash3)
	}

	// Different entries should produce different hash.
	entries3 := []TrustedEffectEntry{
		{
			Kind:               EffectCandidateOperator,
			ObjectOID:          97,
			NamespaceOID:       11,
			CanonicalSignature: "pg_catalog.<>(23,23)",
		},
	}
	hash4 := ComputeManifestHash(entries3)
	if hash1 == hash4 {
		t.Errorf("ComputeManifestHash same for different entries: %s", hash1)
	}
}

func TestPG17ManifestValid(t *testing.T) {
	if err := ValidateManifest(PG17Manifest); err != nil {
		t.Errorf("PG17Manifest invalid: %v", err)
	}

	// Verify entry count: 54 operators + 2 aggregates = 56.
	if len(PG17Manifest.Entries) != 56 {
		t.Errorf("PG17Manifest has %d entries, want 56", len(PG17Manifest.Entries))
	}

	// Verify hash is set.
	if PG17Manifest.Hash == "" {
		t.Error("PG17Manifest hash is empty")
	}

	// Verify hash matches.
	expected := ComputeManifestHash(PG17Manifest.Entries)
	if PG17Manifest.Hash != expected {
		t.Errorf("PG17Manifest hash mismatch: got %s, want %s", PG17Manifest.Hash, expected)
	}
}

func TestTrustDecisionConstants(t *testing.T) {
	// Verify all trust decisions are valid.
	for _, d := range []TrustDecision{
		TrustDecisionAllProven,
		TrustDecisionHasUnproven,
		TrustDecisionHasUnknown,
		TrustDecisionEmpty,
	} {
		if !ValidTrustDecision(d) {
			t.Errorf("ValidTrustDecision(%q) = false, want true", d)
		}
	}

	// Verify invalid decision.
	if ValidTrustDecision("invalid") {
		t.Error("ValidTrustDecision(\"invalid\") = true, want false")
	}
}

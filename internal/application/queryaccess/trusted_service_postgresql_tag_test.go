//go:build postgresql

// Package queryaccess tests trusted service promotion via Service.Analyze.
// input: Service with trusted bundle calling real PostgreSQL extraction
// output: verification of manifest-gated promotion through full Analyze path
// pos: T8 trusted service promotion integration tests
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestTrustedService_CountStarAdmissible(t *testing.T) {
	countEntries := []TrustedEffectEntry{
		{
			Kind:               EffectCandidateFunction,
			ObjectOID:          2803,
			NamespaceOID:       11,
			OperandTypeOIDs:    nil,
			ResultTypeOID:      20,
			Volatility:         EffectVolatilityImmutable,
			CanonicalSignature: "pg_catalog.count()",
			AuditNotes:         "count(*)",
		},
	}
	countManifest := TrustedEffectManifest{
		SchemaVersion:      "1.0",
		PostgreSQLMajorMin: 17,
		PostgreSQLMajorMax: 17,
		Entries:            countEntries,
		Hash:               ComputeManifestHash(countEntries),
	}
	policy, err := NewTrustPolicy(countManifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	resolver := &mockControlledResolver{
		ctx: testResolutionContext(),
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
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT count(*) FROM users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if res.DomainResult.ReadClassification != domain.ReadOnly {
		t.Errorf("classification = %v, want read_only", res.DomainResult.ReadClassification)
	}
	if res.DomainResult.Admission != domain.Admissible {
		t.Errorf("admission = %v, want admissible", res.DomainResult.Admission)
	}
	for _, code := range res.DomainResult.ReasonCodes {
		if code == domain.ReasonUnprovenFunctionEffect {
			t.Error("unproven_function_effect should be removed after proof")
		}
	}
	if resolver.ctxCalled == 0 {
		t.Error("CaptureExecutionBoundContext was not called")
	}
}

func TestTrustedService_OperatorWithLiteralIndeterminate(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	resolver := &mockControlledResolver{
		ctx: testResolutionContext(),
		batch: EffectIdentityBatch{
			Items: []EffectIdentityItem{
				{
					Ordinal: 0,
					Status:  domain.IdentityStatusCoercionGap,
					Facts:   nil,
				},
			},
		},
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT id FROM users WHERE id = 1", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if res.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification = %v, want indeterminate", res.DomainResult.ReadClassification)
	}
	if res.DomainResult.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission = %v, want indeterminate", res.DomainResult.Admission)
	}
}

func TestTrustedService_CaptureContextFails(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	resolver := &mockControlledResolver{
		ctxErr: context.DeadlineExceeded,
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT count(*) FROM users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if res.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification = %v, want indeterminate", res.DomainResult.ReadClassification)
	}
	if res.DomainResult.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission = %v, want indeterminate", res.DomainResult.Admission)
	}
}

func TestTrustedService_IncompleteContextBlocked(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	resolver := &mockControlledResolver{
		ctx: EffectIdentityResolutionContext{
			Bound:          true,
			SessionBinding: "test",
		},
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT count(*) FROM users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if res.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification = %v, want indeterminate", res.DomainResult.ReadClassification)
	}
}

func TestTrustedService_ResolverErrorBlocked(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	resolver := &mockControlledResolver{
		ctx: testResolutionContext(),
		err: context.Canceled,
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT count(*) FROM users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if res.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification = %v, want indeterminate", res.DomainResult.ReadClassification)
	}
}

func TestTrustedService_ManifestMissBlocked(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	resolver := &mockControlledResolver{
		ctx: testResolutionContext(),
		batch: EffectIdentityBatch{
			Items: []EffectIdentityItem{
				{
					Ordinal: 0,
					Status:  domain.IdentityStatusResolved,
					Facts: &EffectIdentityFacts{
						Kind:               EffectCandidateFunction,
						ObjectOID:          9999,
						NamespaceOID:       11,
						CanonicalSignature: "pg_catalog.nonexistent()",
						DatabaseOID:        1,
						ServerVersionNum:   170000,
					},
				},
			},
		},
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT count(*) FROM users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if res.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification = %v, want indeterminate", res.DomainResult.ReadClassification)
	}
}

func TestTrustedService_NoPublicLeak(t *testing.T) {
	countEntries := []TrustedEffectEntry{
		{
			Kind:               EffectCandidateFunction,
			ObjectOID:          2803,
			NamespaceOID:       11,
			CanonicalSignature: "pg_catalog.count()",
			Volatility:         EffectVolatilityImmutable,
		},
	}
	countManifest := TrustedEffectManifest{
		SchemaVersion:      "1.0",
		PostgreSQLMajorMin: 17,
		PostgreSQLMajorMax: 17,
		Entries:            countEntries,
		Hash:               ComputeManifestHash(countEntries),
	}
	policy, err := NewTrustPolicy(countManifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	resolver := &mockControlledResolver{
		ctx: testResolutionContext(),
		batch: EffectIdentityBatch{
			Items: []EffectIdentityItem{
				{
					Ordinal: 0,
					Status:  domain.IdentityStatusResolved,
					Facts: &EffectIdentityFacts{
						Kind:               EffectCandidateFunction,
						ObjectOID:          2803,
						NamespaceOID:       11,
						CanonicalSignature: "pg_catalog.count()",
						DatabaseOID:        1,
						ServerVersionNum:   170000,
					},
				},
			},
		},
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT count(*) FROM users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	data, err := json.Marshal(res.DomainResult)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)
	for _, bad := range []string{
		"ObjectOID", "object_oid", "CanonicalSignature", "NamespaceOID",
		"EffectIdentity", "identity_facts", "severity", "postgres://",
		"password", "SessionBinding", "DatabaseOID", "ServerVersionNum",
		"candidate", "manifest",
	} {
		if strings.Contains(raw, bad) {
			t.Errorf("public domain JSON leaked %q in: %s", bad, raw)
		}
	}
}

func TestTrustedService_MySQLUnchanged(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	resolver := &mockControlledResolver{ctx: testResolutionContext()}
	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT id FROM users WHERE id = 1", Dialect: "mysql", Mode: "strict", DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if res.DomainResult.ReadClassification != domain.ReadOnly {
		t.Errorf("mysql classification = %v, want read_only", res.DomainResult.ReadClassification)
	}
	if res.DomainResult.Admission != domain.Admissible {
		t.Errorf("mysql admission = %v, want admissible", res.DomainResult.Admission)
	}
}

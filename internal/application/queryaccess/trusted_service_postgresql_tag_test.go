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
		SQL: "SELECT count(*) FROM public.users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
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

func TestTrustedService_UnqualifiedRelationRejected(t *testing.T) {
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

	if res.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification = %v, want indeterminate (unqualified relation)", res.DomainResult.ReadClassification)
	}
	if res.DomainResult.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission = %v, want indeterminate (unqualified relation)", res.DomainResult.Admission)
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
		SQL: "SELECT id FROM public.users WHERE id = 1", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
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
		SQL: "SELECT count(*) FROM public.users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
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
		SQL: "SELECT count(*) FROM public.users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
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
		SQL: "SELECT count(*) FROM public.users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
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
		SQL: "SELECT count(*) FROM public.users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
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
		SQL: "SELECT count(*) FROM public.users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
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

func TestTrustedService_NegativeFailClosedScenarios(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	cases := []struct {
		name  string
		sql   string
		batch EffectIdentityBatch
	}{
		{
			name: "literal_unknown_type",
			sql:  "SELECT id FROM public.users WHERE id = 1",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{Ordinal: 0, Status: domain.IdentityStatusCoercionGap},
				},
			},
		},
		{
			name: "param_unknown_type",
			sql:  "SELECT id FROM public.users WHERE id = $1",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{Ordinal: 0, Status: domain.IdentityStatusCoercionGap},
				},
			},
		},
		{
			name: "null_unknown_type",
			sql:  "SELECT id FROM public.users WHERE id = NULL",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{Ordinal: 0, Status: domain.IdentityStatusCoercionGap},
				},
			},
		},
		{
			name: "cast_indeterminate",
			sql:  "SELECT id::text FROM public.users WHERE id = 1",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{Ordinal: 0, Status: domain.IdentityStatusCoercionGap},
				},
			},
		},
		{
			name: "type_mismatch_int4_vs_text",
			sql:  "SELECT id FROM public.users WHERE id = name",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{Ordinal: 0, Status: domain.IdentityStatusCoercionGap},
				},
			},
		},
		{
			name: "cte_column",
			sql:  "WITH cte AS (SELECT id FROM public.users) SELECT id FROM cte WHERE id = 1",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{Ordinal: 0, Status: domain.IdentityStatusCoercionGap},
				},
			},
		},
		{
			name: "derived_table_column",
			sql:  "SELECT id FROM (SELECT id FROM public.users) sub WHERE id = 1",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{Ordinal: 0, Status: domain.IdentityStatusCoercionGap},
				},
			},
		},
		{
			name: "view_column",
			sql:  "SELECT id FROM public.users WHERE id = 1",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{Ordinal: 0, Status: domain.IdentityStatusCoercionGap},
				},
			},
		},
		{
			name: "ambiguous_column",
			sql:  "SELECT id FROM public.users, orders WHERE id = id",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{Ordinal: 0, Status: domain.IdentityStatusAmbiguous},
				},
			},
		},
		{
			name: "custom_operator_user_schema",
			sql:  "SELECT id FROM public.users WHERE id OPERATOR(app.~=) 1",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
						Ordinal: 0,
						Status:  domain.IdentityStatusResolved,
						Facts: &EffectIdentityFacts{
							Kind:               EffectCandidateOperator,
							ObjectOID:          99999,
							NamespaceOID:       2200,
							OperandTypeOIDs:    []uint32{23, 23},
							ResultTypeOID:      16,
							CanonicalSignature: "app.~=(23,23)",
							DatabaseOID:        1,
							ServerVersionNum:   170000,
						},
					},
				},
			},
		},
		{
			name: "non_manifest_pg_catalog_operator",
			sql:  "SELECT id FROM public.users WHERE id + 1 > 0",
			batch: EffectIdentityBatch{
				Items: []EffectIdentityItem{
					{
						Ordinal: 0,
						Status:  domain.IdentityStatusResolved,
						Facts: &EffectIdentityFacts{
							Kind:               EffectCandidateOperator,
							ObjectOID:          177,
							NamespaceOID:       11,
							OperandTypeOIDs:    []uint32{23, 23},
							ResultTypeOID:      23,
							CanonicalSignature: "pg_catalog.+(23,23)",
							DatabaseOID:        1,
							ServerVersionNum:   170000,
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &mockControlledResolver{
				ctx:   testResolutionContext(),
				batch: tc.batch,
			}
			svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
			if err != nil {
				t.Fatalf("NewTrustedService: %v", err)
			}

			res, err := svc.Analyze(context.Background(), QueryAccessRequest{
				SQL: tc.sql, Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
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
		})
	}
}

func TestTrustedService_DefaultServiceIndeterminateForJoinQuery(t *testing.T) {
	svc := NewService()

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:           "SELECT u.name, o.user_id FROM public.users u JOIN orders o ON u.id = o.user_id",
		Dialect:       "postgresql",
		Mode:          "strict",
		DefaultSchema: "public",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if res.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification = %v, want indeterminate (no trusted bundle)", res.DomainResult.ReadClassification)
	}
	if res.DomainResult.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission = %v, want indeterminate (no trusted bundle)", res.DomainResult.Admission)
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
		SQL: "SELECT id FROM public.users WHERE id = 1", Dialect: "mysql", Mode: "strict", DefaultSchema: "app",
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

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
	"fmt"
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

func TestTrustedService_UnqualifiedRelationFullPipeline(t *testing.T) {
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

	schemaResolver := &countingSchemaResolver{inner: &mockSchemaResolver{}}
	svc, err := NewTrustedService(resolver, policy, schemaResolver)
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
	assertReasonsContain(t, res.DomainResult.ReasonCodes, domain.ReasonUnqualifiedRelationBlocked)

	if len(res.DomainResult.Requirements) == 0 {
		t.Error("expected requirements to be populated via the unified pipeline")
	}
	for _, req := range res.DomainResult.Requirements {
		if req.Privilege == "read_table" || req.Privilege == "read_column" {
			t.Errorf("unexpected physical requirement from unbound relation: %+v", req)
		}
	}
	assertRequirement(t, res.DomainResult.Requirements, "unqualified_relation", "indeterminate")

	for _, rel := range res.DomainResult.Relations {
		if rel.Schema == "public" {
			t.Errorf("unbound relation was resolved to schema %q: %+v", rel.Schema, rel)
		}
	}
	for _, col := range res.DomainResult.ReferencedColumns {
		if col.Schema == "public" {
			t.Errorf("unbound column was resolved to schema %q: %+v", col.Schema, col)
		}
	}

	if resolver.ctxCalled > 0 {
		t.Error("CaptureExecutionBoundContext should not be called for unqualified relations")
	}
	if schemaResolver.calls > 0 {
		t.Errorf("schema resolver should not be called for unbound relations, got %d calls", schemaResolver.calls)
	}
}

func TestTrustedService_UnqualifiedRelationStructuralQuery(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}
	schemaResolver := &countingSchemaResolver{inner: &mockSchemaResolver{}}
	svc, err := NewTrustedService(&mockControlledResolver{ctx: testResolutionContext()}, policy, schemaResolver)
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT id FROM users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
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
	assertReasonsContain(t, res.DomainResult.ReasonCodes, domain.ReasonUnqualifiedRelationBlocked)
	assertRequirement(t, res.DomainResult.Requirements, "unqualified_relation", "indeterminate")
	assertNoRequirement(t, res.DomainResult.Requirements, "public.users", "read_table")
	assertNoRequirement(t, res.DomainResult.Requirements, "public.users.id", "read_column")
	assertNoRequirement(t, res.DomainResult.Requirements, "users", "read_table")
	assertNoRequirement(t, res.DomainResult.Requirements, "users.id", "read_column")
	assertNoReadColumnRequirements(t, res.DomainResult.Requirements)
	if schemaResolver.calls > 0 {
		t.Errorf("schema resolver should not be called for unbound relations, got %d calls", schemaResolver.calls)
	}
}

func TestTrustedService_UnqualifiedRelationResolverError(t *testing.T) {
	errResolver := &mockFailingSchemaResolver{err: fmt.Errorf("connection refused")}
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	svc, err := NewTrustedService(&mockControlledResolver{ctx: testResolutionContext()}, policy, errResolver)
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT id FROM users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if res.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification = %v, want indeterminate", res.DomainResult.ReadClassification)
	}
}

func TestTrustedService_UnqualifiedRelationNoPublicLeak(t *testing.T) {
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

func TestTrustedService_UnqualifiedWildcard(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}
	schemaResolver := &countingSchemaResolver{inner: &mockSchemaResolver{}}
	svc, err := NewTrustedService(&mockControlledResolver{ctx: testResolutionContext()}, policy, schemaResolver)
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT * FROM users", Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
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
	assertReasonsContain(t, res.DomainResult.ReasonCodes, domain.ReasonUnqualifiedRelationBlocked)
	assertRequirement(t, res.DomainResult.Requirements, "unqualified_relation", "indeterminate")
	assertNoRequirement(t, res.DomainResult.Requirements, "public.users", "read_table")
	assertNoRequirement(t, res.DomainResult.Requirements, "*", "read_column")
	assertNoRequirement(t, res.DomainResult.Requirements, "public.users.id", "read_column")
	assertNoReadColumnRequirements(t, res.DomainResult.Requirements)
	if schemaResolver.calls > 0 {
		t.Errorf("schema resolver should not be called for unbound relations, got %d calls", schemaResolver.calls)
	}
}

func TestTrustedService_UnqualifiedJoin(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}
	schemaResolver := &countingSchemaResolver{inner: &mockSchemaResolver{}}
	svc, err := NewTrustedService(&mockControlledResolver{ctx: testResolutionContext()}, policy, schemaResolver)
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT u.name, o.user_id FROM users u JOIN orders o ON u.id = o.user_id",
		Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
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
	assertReasonsContain(t, res.DomainResult.ReasonCodes, domain.ReasonUnqualifiedRelationBlocked)
	assertRequirement(t, res.DomainResult.Requirements, "unqualified_relation", "indeterminate")
	assertNoRequirement(t, res.DomainResult.Requirements, "public.users", "read_table")
	assertNoRequirement(t, res.DomainResult.Requirements, "public.orders", "read_table")
	assertNoRequirement(t, res.DomainResult.Requirements, "public.users.id", "read_column")
	assertNoRequirement(t, res.DomainResult.Requirements, "public.orders.user_id", "read_column")
	assertNoRequirement(t, res.DomainResult.Requirements, "public.users.name", "read_column")
	assertNoReadColumnRequirements(t, res.DomainResult.Requirements)
	if schemaResolver.calls > 0 {
		t.Errorf("schema resolver should not be called for unbound relations, got %d calls", schemaResolver.calls)
	}
	assertColumnUnbound(t, res.DomainResult.ReferencedColumns, "users", "name")
	assertColumnUnbound(t, res.DomainResult.ReferencedColumns, "orders", "user_id")
}

func TestTrustedService_UnqualifiedProjectionOnly(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}
	schemaResolver := &countingSchemaResolver{inner: &mockSchemaResolver{}}
	svc, err := NewTrustedService(&mockControlledResolver{ctx: testResolutionContext()}, policy, schemaResolver)
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT id FROM users", Dialect: "postgresql", Mode: "projection_only", DefaultSchema: "public",
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
	assertRequirement(t, res.DomainResult.Requirements, "unqualified_relation", "indeterminate")
	assertNoRequirement(t, res.DomainResult.Requirements, "public.users.id", "read_column")
	assertNoRequirement(t, res.DomainResult.Requirements, "users.id", "read_column")
	assertNoRequirement(t, res.DomainResult.Requirements, "*", "read_column")
	assertNoReadColumnRequirements(t, res.DomainResult.Requirements)
	if schemaResolver.calls > 0 {
		t.Errorf("schema resolver should not be called for unbound relations, got %d calls", schemaResolver.calls)
	}
	assertColumnUnbound(t, res.DomainResult.ReferencedColumns, "users", "id")
}

func TestTrustedService_MixedQualifiedUnqualified(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}
	schemaResolver := &countingSchemaResolver{inner: &mockSchemaResolver{}}
	svc, err := NewTrustedService(&mockControlledResolver{ctx: testResolutionContext()}, policy, schemaResolver)
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT p.id, u.name FROM public.users p JOIN users u ON p.id = u.id",
		Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
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
	assertReasonsContain(t, res.DomainResult.ReasonCodes, domain.ReasonUnqualifiedRelationBlocked)
	assertRequirement(t, res.DomainResult.Requirements, "unqualified_relation", "indeterminate")
	assertRequirement(t, res.DomainResult.Requirements, "public.users", "read_table")
	assertRequirement(t, res.DomainResult.Requirements, "public.users.id", "read_column")
	assertNoRequirement(t, res.DomainResult.Requirements, "public.users.name", "read_column")
	assertNoRequirement(t, res.DomainResult.Requirements, "users", "read_table")
	assertNoRequirement(t, res.DomainResult.Requirements, "users.name", "read_column")
	assertNoRequirement(t, res.DomainResult.Requirements, "users.id", "read_column")
	if schemaResolver.calls != 1 {
		t.Errorf("schema resolver should be called exactly once for the qualified relation, got %d calls", schemaResolver.calls)
	}
	assertColumnNotUnbound(t, res.DomainResult.ReferencedColumns, "users", "id")
}

func TestTrustedService_MySQLUnqualifiedUnchanged(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}
	svc, err := NewTrustedService(&mockControlledResolver{ctx: testResolutionContext()}, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT id FROM users", Dialect: "mysql", Mode: "strict", DefaultSchema: "app",
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
	assertRequirement(t, res.DomainResult.Requirements, "app.users", "read_table")
	assertRequirement(t, res.DomainResult.Requirements, "app.users.id", "read_column")
	assertNoRequirement(t, res.DomainResult.Requirements, "unqualified_relation", "indeterminate")
	for _, rel := range res.DomainResult.Relations {
		if rel.Unbound {
			t.Errorf("mysql relation should not be marked unbound: %+v", rel)
		}
	}
}

func TestTrustedService_TiDBUnqualifiedUnchanged(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}
	svc, err := NewTrustedService(&mockControlledResolver{ctx: testResolutionContext()}, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT id FROM users", Dialect: "tidb", Mode: "strict", DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if res.DomainResult.ReadClassification != domain.ReadOnly {
		t.Errorf("tidb classification = %v, want read_only", res.DomainResult.ReadClassification)
	}
	if res.DomainResult.Admission != domain.Admissible {
		t.Errorf("tidb admission = %v, want admissible", res.DomainResult.Admission)
	}
	assertRequirement(t, res.DomainResult.Requirements, "app.users", "read_table")
	assertRequirement(t, res.DomainResult.Requirements, "app.users.id", "read_column")
	assertNoRequirement(t, res.DomainResult.Requirements, "unqualified_relation", "indeterminate")
	for _, rel := range res.DomainResult.Relations {
		if rel.Unbound {
			t.Errorf("tidb relation should not be marked unbound: %+v", rel)
		}
	}
}

func assertColumnUnbound(t *testing.T, cols []domain.ColumnReference, table, column string) {
	t.Helper()
	for _, c := range cols {
		if strings.EqualFold(c.Table, table) && strings.EqualFold(c.Column, column) {
			if !c.Unbound {
				t.Errorf("expected column %s.%s to be unbound, got unbound=false", table, column)
			}
			return
		}
	}
	t.Errorf("column %s.%s not found in referenced columns", table, column)
}

func assertColumnNotUnbound(t *testing.T, cols []domain.ColumnReference, table, column string) {
	t.Helper()
	for _, c := range cols {
		if strings.EqualFold(c.Table, table) && strings.EqualFold(c.Column, column) {
			if c.Unbound {
				t.Errorf("expected column %s.%s to NOT be unbound, got unbound=true", table, column)
			}
			return
		}
	}
	t.Errorf("column %s.%s not found in referenced columns", table, column)
}

func assertNoReadColumnRequirements(t *testing.T, reqs []domain.Requirement) {
	t.Helper()
	for _, r := range reqs {
		if r.Privilege == "read_column" {
			t.Errorf("unexpected read_column requirement: %v", r)
		}
	}
}

// TestDefaultService_UnqualifiedRelation_FailClosed verifies that unqualified
// PostgreSQL base relations fail closed when using the default NewService()
// with a SchemaResolver (S1 proof — T11.1).
func TestDefaultService_UnqualifiedRelation_FailClosed(t *testing.T) {
	t.Parallel()
	svc := NewService()
	resolver := &mockSchemaResolver{}

	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:            "SELECT id FROM users",
		Dialect:        "postgresql",
		Mode:           "strict",
		DefaultSchema:  "public",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	dr := result.DomainResult
	if dr.ReadClassification != domain.Indeterminate {
		t.Errorf("classification: got %q, want %q", dr.ReadClassification, domain.Indeterminate)
	}
	if dr.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission: got %q, want %q", dr.Admission, domain.IndeterminateAdmission)
	}

	hasReason := false
	for _, rc := range dr.ReasonCodes {
		if rc == domain.ReasonUnqualifiedRelationBlocked {
			hasReason = true
			break
		}
	}
	if !hasReason {
		t.Errorf("expected ReasonUnqualifiedRelationBlocked in reasons: %v", dr.ReasonCodes)
	}

	for _, req := range dr.Requirements {
		if req.Privilege == "read_table" || req.Privilege == "read_column" {
			t.Errorf("unexpected physical requirement: %+v", req)
		}
	}

	hasUnqualifiedReq := false
	for _, req := range dr.Requirements {
		if req.Object == "unqualified_relation" && req.Privilege == "indeterminate" {
			hasUnqualifiedReq = true
			break
		}
	}
	if !hasUnqualifiedReq {
		t.Error("expected unqualified_relation indeterminate requirement")
	}
}

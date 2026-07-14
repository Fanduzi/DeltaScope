//go:build postgresql

// Package queryaccess tests adversarial scenarios for atomic-proof and no-leak.
// input: Service with trusted bundle, various mismatched contexts
// output: verification of fail-closed behavior under adversarial conditions
// pos: P2-2 adversarial tests for atomic resolver and no-leak assertions
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

// mockMismatchResolver returns a post-lookup context that differs from the request context.
type mockMismatchResolver struct {
	preContext  EffectIdentityResolutionContext
	postContext EffectIdentityResolutionContext
	batch       EffectIdentityBatch
	err         error
}

func (m *mockMismatchResolver) ResolveEffectIdentities(ctx context.Context, req EffectIdentityRequest) (EffectIdentityBatch, error) {
	if m.err != nil {
		return EffectIdentityBatch{}, m.err
	}
	return m.batch, nil
}

func (m *mockMismatchResolver) CaptureExecutionBoundContext(ctx context.Context) (EffectIdentityResolutionContext, error) {
	return m.preContext, nil
}

func (m *mockMismatchResolver) ResolveColumnTypesAndEffectIdentities(ctx context.Context, candidates []EffectCandidate, req EffectIdentityRequest) (map[int][]uint32, EffectIdentityBatch, EffectIdentityResolutionContext, error) {
	if m.err != nil {
		return nil, EffectIdentityBatch{}, EffectIdentityResolutionContext{}, m.err
	}
	return nil, m.batch, m.postContext, nil
}

// TestAdversarial_WrongSessionBinding verifies that mismatched session binding causes indeterminate.
func TestAdversarial_WrongSessionBinding(t *testing.T) {
	preCtx := testResolutionContext()
	postCtx := preCtx
	postCtx.SessionBinding = "wrong-session"

	resolver := &mockMismatchResolver{
		preContext:  preCtx,
		postContext: postCtx,
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

	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:     "SELECT count(*) FROM app.users",
		Dialect: "postgresql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("expected indeterminate, got %v", result.DomainResult.ReadClassification)
	}
	if result.DomainResult.Admission != domain.IndeterminateAdmission {
		t.Errorf("expected indeterminate admission, got %v", result.DomainResult.Admission)
	}
}

// TestAdversarial_WrongDatabaseOID verifies that mismatched database OID causes indeterminate.
func TestAdversarial_WrongDatabaseOID(t *testing.T) {
	preCtx := testResolutionContext()
	postCtx := preCtx
	postCtx.DatabaseOID = 999

	resolver := &mockMismatchResolver{
		preContext:  preCtx,
		postContext: postCtx,
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

	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:     "SELECT count(*) FROM app.users",
		Dialect: "postgresql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("expected indeterminate, got %v", result.DomainResult.ReadClassification)
	}
}

// TestAdversarial_WrongRoleOID verifies that mismatched role OID causes indeterminate.
func TestAdversarial_WrongRoleOID(t *testing.T) {
	preCtx := testResolutionContext()
	postCtx := preCtx
	postCtx.RoleOID = 999

	resolver := &mockMismatchResolver{
		preContext:  preCtx,
		postContext: postCtx,
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

	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:     "SELECT count(*) FROM app.users",
		Dialect: "postgresql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("expected indeterminate, got %v", result.DomainResult.ReadClassification)
	}
}

// TestAdversarial_WrongServerVersion verifies that mismatched server version causes indeterminate.
func TestAdversarial_WrongServerVersion(t *testing.T) {
	preCtx := testResolutionContext()
	postCtx := preCtx
	postCtx.ServerVersionNum = 160000

	resolver := &mockMismatchResolver{
		preContext:  preCtx,
		postContext: postCtx,
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

	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:     "SELECT count(*) FROM app.users",
		Dialect: "postgresql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("expected indeterminate, got %v", result.DomainResult.ReadClassification)
	}
}

// TestAdversarial_WrongPathEpoch verifies that mismatched path epoch causes indeterminate.
func TestAdversarial_WrongPathEpoch(t *testing.T) {
	preCtx := testResolutionContext()
	postCtx := preCtx
	postCtx.PathEpoch = 999

	resolver := &mockMismatchResolver{
		preContext:  preCtx,
		postContext: postCtx,
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

	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:     "SELECT count(*) FROM app.users",
		Dialect: "postgresql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("expected indeterminate, got %v", result.DomainResult.ReadClassification)
	}
}

// TestAdversarial_WrongSearchPath verifies that mismatched search path causes indeterminate for unqualified.
func TestAdversarial_WrongSearchPath(t *testing.T) {
	preCtx := testResolutionContext()
	postCtx := preCtx
	postCtx.NamespaceSearchOIDs = []uint32{999, 888}

	resolver := &mockMismatchResolver{
		preContext:  preCtx,
		postContext: postCtx,
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

	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:     "SELECT count(*) FROM app.users",
		Dialect: "postgresql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("expected indeterminate, got %v", result.DomainResult.ReadClassification)
	}
}

// TestAdversarial_FactDatabaseMismatch verifies that fact database mismatch causes indeterminate.
func TestAdversarial_FactDatabaseMismatch(t *testing.T) {
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
						DatabaseOID:        999, // Wrong database OID
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

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:     "SELECT count(*) FROM app.users",
		Dialect: "postgresql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("expected indeterminate, got %v", result.DomainResult.ReadClassification)
	}
}

// TestAdversarial_FactVersionMismatch verifies that fact version mismatch causes indeterminate.
func TestAdversarial_FactVersionMismatch(t *testing.T) {
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
						ServerVersionNum:   160000, // Wrong version
					},
				},
			},
		},
	}

	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:     "SELECT count(*) FROM app.users",
		Dialect: "postgresql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("expected indeterminate, got %v", result.DomainResult.ReadClassification)
	}
}

// TestAdversarial_IncompleteBatch verifies that incomplete batch causes indeterminate.
func TestAdversarial_IncompleteBatch(t *testing.T) {
	resolver := &mockControlledResolver{
		ctx: testResolutionContext(),
		batch: EffectIdentityBatch{
			Items: []EffectIdentityItem{
				// Missing ordinal 0
				{
					Ordinal: 1,
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

	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:     "SELECT count(*) FROM app.users",
		Dialect: "postgresql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("expected indeterminate, got %v", result.DomainResult.ReadClassification)
	}
}

// TestAdversarial_NoLeak_PublicJSON verifies that public JSON never leaks sensitive information.
func TestAdversarial_NoLeak_PublicJSON(t *testing.T) {
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

	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:     "SELECT count(*) FROM app.users WHERE id = 1",
		Dialect: "postgresql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Marshal to JSON to check for leaks
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	jsonStr := string(jsonBytes)

	// Check for forbidden leaks
	forbiddenLeaks := []string{
		"SELECT count(*)", // raw SQL
		"2803",            // OID
		"11",              // namespace OID
		"pg_catalog",      // catalog name
		"170000",          // server version
		"test-session",    // session binding
		"severity",        // severity field
		"dsn",             // DSN
		"password",        // credentials
		"current_setting", // SQL value function names
		"session_user",    // SQL value function names
	}

	for _, leak := range forbiddenLeaks {
		if strings.Contains(jsonStr, leak) {
			t.Errorf("public JSON contains forbidden leak: %q", leak)
		}
	}
}

// TestAdversarial_NoLeak_ReasonCodes verifies that reason codes don't leak sensitive information.
func TestAdversarial_NoLeak_ReasonCodes(t *testing.T) {
	resolver := &mockControlledResolver{
		ctx: testResolutionContext(),
		batch: EffectIdentityBatch{
			Items: []EffectIdentityItem{
				{
					Ordinal: 0,
					Status:  domain.IdentityStatusLookupFailed,
				},
			},
		},
	}

	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	svc, err := NewTrustedService(resolver, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:     "SELECT count(*) FROM app.users",
		Dialect: "postgresql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Check reason codes don't leak
	for _, code := range result.DomainResult.ReasonCodes {
		codeStr := string(code)
		if strings.Contains(codeStr, "pg_catalog") {
			t.Errorf("reason code leaks catalog name: %q", codeStr)
		}
		if strings.Contains(codeStr, "2803") {
			t.Errorf("reason code leaks OID: %q", codeStr)
		}
		if strings.Contains(codeStr, "SELECT") {
			t.Errorf("reason code leaks SQL: %q", codeStr)
		}
	}
}

// TestAdversarial_NoLeak_Unresolved verifies that unresolved references don't leak sensitive information.
func TestAdversarial_NoLeak_Unresolved(t *testing.T) {
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

	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	svc, err := NewTrustedService(resolver, policy, &mockFailingSchemaResolver{err: errTestSchemaFail})
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	result, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:     "SELECT count(*) FROM nonexistent_table",
		Dialect: "postgresql",
		Mode:    "strict",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Check unresolved references don't leak
	for _, u := range result.DomainResult.Unresolved {
		refStr := u.Reference
		if strings.Contains(refStr, "pg_catalog") {
			t.Errorf("unresolved reference leaks catalog name: %q", refStr)
		}
		if strings.Contains(refStr, "2803") {
			t.Errorf("unresolved reference leaks OID: %q", refStr)
		}
	}
}

// errTestSchemaFail is a test error for schema resolver failures.
var errTestSchemaFail = fmt.Errorf("test schema resolver failure")

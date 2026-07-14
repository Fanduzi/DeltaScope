// Package queryaccess tests the trusted service integration.
// input: Service with trusted bundle, effect identity resolver, trust policy
// output: verification of manifest-gated promotion logic
// pos: T8 trusted service integration tests (non-postgresql)
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// mockControlledResolver implements ControlledEffectIdentityResolver for tests.
type mockControlledResolver struct {
	batch     EffectIdentityBatch
	err       error
	ctx       EffectIdentityResolutionContext
	ctxErr    error
	ctxCalled int
}

func (m *mockControlledResolver) ResolveEffectIdentities(ctx context.Context, req EffectIdentityRequest) (EffectIdentityBatch, error) {
	if m.err != nil {
		return EffectIdentityBatch{}, m.err
	}
	return m.batch, nil
}

func (m *mockControlledResolver) CaptureExecutionBoundContext(ctx context.Context) (EffectIdentityResolutionContext, error) {
	m.ctxCalled++
	if m.ctxErr != nil {
		return EffectIdentityResolutionContext{}, m.ctxErr
	}
	return m.ctx, nil
}

func (m *mockControlledResolver) ResolveColumnTypesAndEffectIdentities(ctx context.Context, candidates []EffectCandidate, req EffectIdentityRequest) (map[int][]uint32, EffectIdentityBatch, EffectIdentityResolutionContext, error) {
	if m.err != nil {
		return nil, EffectIdentityBatch{}, EffectIdentityResolutionContext{}, m.err
	}
	return nil, m.batch, m.ctx, nil
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

// mockFailingSchemaResolver always returns an error.
type mockFailingSchemaResolver struct {
	err error
}

func (m *mockFailingSchemaResolver) ResolveRelation(ctx context.Context, dialect string, schema, name string) (RelationSchema, error) {
	return RelationSchema{}, m.err
}

type countingSchemaResolver struct {
	inner SchemaResolver
	calls int
}

func (c *countingSchemaResolver) ResolveRelation(ctx context.Context, dialect string, schema, name string) (RelationSchema, error) {
	c.calls++
	return c.inner.ResolveRelation(ctx, dialect, schema, name)
}

// testResolutionContext returns a session-complete resolution context for tests.
func testResolutionContext() EffectIdentityResolutionContext {
	return EffectIdentityResolutionContext{
		Bound:               true,
		SessionBinding:      "test-session",
		PathEpoch:           1,
		NamespaceSearchOIDs: []uint32{11, 2200},
		DatabaseOID:         1,
		RoleOID:             10,
		ServerVersionNum:    170000,
	}
}

func TestNewTrustedService(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	tests := []struct {
		name           string
		effectResolver ControlledEffectIdentityResolver
		trustPolicy    *TrustPolicy
		schemaResolver SchemaResolver
		wantErr        bool
	}{
		{
			name:           "valid bundle",
			effectResolver: &mockControlledResolver{ctx: testResolutionContext()},
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
			effectResolver: &mockControlledResolver{ctx: testResolutionContext()},
			trustPolicy:    nil,
			schemaResolver: &mockSchemaResolver{},
			wantErr:        true,
		},
		{
			name:           "nil schema resolver",
			effectResolver: &mockControlledResolver{ctx: testResolutionContext()},
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

func TestReclassifyAfterResolutionPGHardStopRemoved(t *testing.T) {
	got := reclassifyAfterResolution(domain.Indeterminate, nil, nil, true, "postgresql", nil)
	if got != domain.Indeterminate {
		t.Errorf("nil proof: got %v, want indeterminate", got)
	}

	proof := &trustProofResult{decision: TrustDecisionAllProven}
	got = reclassifyAfterResolution(domain.Indeterminate, nil, nil, true, "postgresql", proof)
	if got != domain.ReadOnly {
		t.Errorf("all_proven: got %v, want read_only", got)
	}

	proof = &trustProofResult{decision: TrustDecisionHasUnproven}
	got = reclassifyAfterResolution(domain.Indeterminate, nil, nil, true, "postgresql", proof)
	if got != domain.Indeterminate {
		t.Errorf("has_unproven: got %v, want indeterminate", got)
	}

	got = reclassifyAfterResolution(domain.Indeterminate, nil, nil, true, "mysql", nil)
	if got != domain.ReadOnly {
		t.Errorf("mysql: got %v, want read_only", got)
	}
}

func TestNewPG17Manifest_DeepCopy(t *testing.T) {
	m1 := NewPG17Manifest()
	m2 := NewPG17Manifest()

	// Find an operator entry with non-empty OperandTypeOIDs.
	var opIdx int
	for i, e := range m1.Entries {
		if len(e.OperandTypeOIDs) > 0 {
			opIdx = i
			break
		}
	}

	// Mutate m1's operator entry.
	m1.Entries[opIdx].ObjectOID = 99999
	m1.Entries[opIdx].OperandTypeOIDs[0] = 88888

	// m2 should be unaffected.
	if m2.Entries[opIdx].ObjectOID == 99999 {
		t.Error("mutation of m1.Entries affected m2")
	}
	if m2.Entries[opIdx].OperandTypeOIDs[0] == 88888 {
		t.Error("mutation of m1 OperandTypeOIDs affected m2")
	}

	// A third call should also be unaffected.
	m3 := NewPG17Manifest()
	if m3.Entries[opIdx].ObjectOID == 99999 {
		t.Error("mutation of m1 affected m3")
	}
	if m3.Entries[opIdx].OperandTypeOIDs[0] == 88888 {
		t.Error("mutation of m1 OperandTypeOIDs affected m3")
	}
}

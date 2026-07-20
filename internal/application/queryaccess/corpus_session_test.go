// Package queryaccess_test provides corpus-only proof-session fixtures.
// input: declared corpus session proof and direct physical column metadata
// output: the service capability required to verify admitted corpus cases
// pos: test-only session seam; production adapters remain independently tested
package queryaccess_test

import (
	"context"
	"strconv"
	"testing"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func newCorpusService(t *testing.T, tc corpusExpected) *appqa.Service {
	t.Helper()
	resolver := corpusSchemaResolver{}
	switch tc.SessionProof {
	case "":
		return &appqa.Service{}
	case "mysql_tidb_builtin":
		svc, err := appqa.NewMySQLTiDBSemanticService(resolver)
		if err != nil {
			t.Fatalf("NewMySQLTiDBSemanticService: %v", err)
		}
		return svc
	case "pg17_catalog_scalar":
		policy, err := appqa.NewTrustPolicy(appqa.NewPG17Manifest())
		if err != nil {
			t.Fatalf("NewTrustPolicy: %v", err)
		}
		svc, err := appqa.NewTrustedService(corpusPG17ScalarResolver{}, policy, resolver)
		if err != nil {
			t.Fatalf("NewTrustedService: %v", err)
		}
		return svc
	default:
		t.Fatal("unknown corpus session proof")
		return nil
	}
}

type corpusSchemaResolver struct{}

func (corpusSchemaResolver) ResolveRelation(_ context.Context, _ string, schema, name string) (appqa.RelationSchema, error) {
	return appqa.RelationSchema{
		Schema: schema,
		Name:   name,
		Kind:   "table",
		Columns: []appqa.ColumnSchema{
			{Name: "id", Ordinal: 1, TypeOID: 23},
			{Name: "name", Ordinal: 2, TypeOID: 25},
			{Name: "email", Ordinal: 3, TypeOID: 25},
			{Name: "amount", Ordinal: 4, TypeOID: 1700},
			{Name: "text_value", Ordinal: 5, TypeOID: 25},
			{Name: "numeric_value", Ordinal: 6, TypeOID: 1700},
		},
	}, nil
}

type corpusPG17ScalarResolver struct{}

func (corpusPG17ScalarResolver) CaptureExecutionBoundContext(context.Context) (appqa.EffectIdentityResolutionContext, error) {
	return corpusPG17ScalarResolutionContext(), nil
}

func (corpusPG17ScalarResolver) ResolveEffectIdentities(_ context.Context, req appqa.EffectIdentityRequest) (appqa.EffectIdentityBatch, error) {
	_, batch, _, _ := corpusPG17ScalarResolver{}.ResolveColumnTypesAndEffectIdentities(context.Background(), req.Candidates, req)
	return batch, nil
}

func (corpusPG17ScalarResolver) ResolveColumnTypesAndEffectIdentities(_ context.Context, candidates []appqa.EffectCandidate, _ appqa.EffectIdentityRequest) (map[int][]uint32, appqa.EffectIdentityBatch, appqa.EffectIdentityResolutionContext, error) {
	typeOIDs := make(map[int][]uint32, len(candidates))
	items := make([]appqa.EffectIdentityItem, 0, len(candidates))
	for _, candidate := range candidates {
		entry, ok := corpusPG17ScalarEntry(candidate)
		if !ok {
			items = append(items, appqa.EffectIdentityItem{Ordinal: candidate.Ordinal, Status: domain.IdentityStatusUnknown})
			continue
		}
		typeOIDs[candidate.Ordinal] = append([]uint32(nil), entry.OperandTypeOIDs...)
		items = append(items, appqa.EffectIdentityItem{
			Ordinal: candidate.Ordinal,
			Status:  domain.IdentityStatusResolved,
			Facts: &appqa.EffectIdentityFacts{
				Kind:               appqa.EffectCandidateFunction,
				ObjectOID:          entry.ObjectOID,
				NamespaceOID:       entry.NamespaceOID,
				OperandTypeOIDs:    append([]uint32(nil), entry.OperandTypeOIDs...),
				ResultTypeOID:      entry.ResultTypeOID,
				Volatility:         entry.Volatility,
				CanonicalSignature: entry.CanonicalSignature,
				ResolvedSchemaName: "pg_catalog",
				ResolvedObjectName: candidate.NamePath[len(candidate.NamePath)-1],
				DatabaseOID:        1,
				ServerVersionNum:   170000,
			},
		})
	}
	return typeOIDs, appqa.EffectIdentityBatch{Items: items}, corpusPG17ScalarResolutionContext(), nil
}

func corpusPG17ScalarResolutionContext() appqa.EffectIdentityResolutionContext {
	return appqa.EffectIdentityResolutionContext{
		Bound:               true,
		SessionBinding:      "corpus-pg17",
		PathEpoch:           1,
		NamespaceSearchOIDs: []uint32{11},
		DatabaseOID:         1,
		RoleOID:             1,
		ServerVersionNum:    170000,
	}
}

func corpusPG17ScalarEntry(candidate appqa.EffectCandidate) (appqa.TrustedEffectEntry, bool) {
	if candidate.Kind != appqa.EffectCandidateFunction || len(candidate.NamePath) != 1 || candidate.Arity != 1 || len(candidate.OperandColumnRefs) != 1 {
		return appqa.TrustedEffectEntry{}, false
	}
	name := candidate.NamePath[0]
	operandOID := uint32(25)
	if candidate.OperandColumnRefs[0].Column == "numeric_value" {
		operandOID = 1700
	}
	signature := "pg_catalog." + name + "(" + strconv.FormatUint(uint64(operandOID), 10) + ")"
	for _, entry := range appqa.NewPG17Manifest().Entries {
		if entry.Kind != appqa.EffectCandidateFunction || len(entry.OperandTypeOIDs) != 1 {
			continue
		}
		if entry.CanonicalSignature == signature {
			return entry, true
		}
	}
	return appqa.TrustedEffectEntry{}, false
}

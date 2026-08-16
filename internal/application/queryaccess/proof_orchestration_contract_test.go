// Package queryaccess verifies the application proof-orchestration contract at the Service.Analyze seam.
// input: MySQL/TiDB queries over fixture builtin semantic capability and default services
// output: read_only/admissible only when the no-effect or builtin proof applicability rule permits promotion
// pos: application orchestration contract coverage (untagged builds)
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"slices"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// TestProofOrchestrationContract_MySQLAnalyze locks the MySQL/TiDB side of the
// proof orchestration contract through Service.Analyze: builtin proof success
// promotes only its owned cases, statements without effect candidates keep
// ordinary reclassification, missing semantic capability stays fail closed,
// and barriers cannot be promoted.
func TestProofOrchestrationContract_MySQLAnalyze(t *testing.T) {
	fixture, err := newBuiltinSemanticService(&builtinTestResolver{}, mustBuiltinTestRegistry(t))
	if err != nil {
		t.Fatalf("fixture service: %v", err)
	}
	fixtureViews, err := newBuiltinSemanticService(&profileTestResolver{}, mustBuiltinTestRegistry(t))
	if err != nil {
		t.Fatalf("view fixture service: %v", err)
	}

	tests := []struct {
		name          string
		service       *Service
		resolver      SchemaResolver
		sql           string
		wantClass     domain.ReadClassification
		wantAdmission domain.Admission
	}{
		{
			name:      "builtin_proof_success",
			service:   fixture,
			sql:       "SELECT COUNT(*) FROM app.users",
			wantClass: domain.ReadOnly, wantAdmission: domain.Admissible,
		},
		{
			name:      "builtin_proof_failure_unknown_function",
			service:   fixture,
			sql:       "SELECT app_specific_rollup(id) FROM app.users",
			wantClass: domain.Indeterminate, wantAdmission: domain.IndeterminateAdmission,
		},
		{
			name:      "no_effect_candidates_ordinary_reclassification",
			service:   fixture,
			sql:       "SELECT id FROM app.users",
			wantClass: domain.ReadOnly, wantAdmission: domain.Admissible,
		},
		{
			name:    "candidates_without_semantic_bundle_fail_closed",
			service: NewService(), resolver: &builtinTestResolver{},
			sql:       "SELECT COUNT(*) FROM app.users",
			wantClass: domain.Indeterminate, wantAdmission: domain.IndeterminateAdmission,
		},
		{
			name:      "view_barrier_not_promoted",
			service:   fixtureViews,
			sql:       "SELECT COUNT(*) FROM app.users_view",
			wantClass: domain.Indeterminate, wantAdmission: domain.IndeterminateAdmission,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.service.Analyze(context.Background(), QueryAccessRequest{
				SQL:             tt.sql,
				Dialect:         "mysql",
				Mode:            "strict",
				DefaultSchema:   "app",
				AnalysisProfile: AnalysisProfileMySQL57,
				SchemaResolver:  tt.resolver,
			})
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if res.DomainResult.ReadClassification != tt.wantClass || res.DomainResult.Admission != tt.wantAdmission {
				t.Fatalf("classification=%q admission=%q, want %q/%q (reasons=%v)",
					res.DomainResult.ReadClassification, res.DomainResult.Admission,
					tt.wantClass, tt.wantAdmission, res.DomainResult.ReasonCodes)
			}
		})
	}
}

// orchestrationProofBatch is a fixture atomic-resolver batch proving one
// pg_catalog.count signature for the white-box orchestration contract.
func orchestrationProofBatch(objectOID uint32, operandTypeOIDs []uint32, signature string) EffectIdentityBatch {
	return EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0,
		Status:  domain.IdentityStatusResolved,
		Facts: &EffectIdentityFacts{
			Kind:               EffectCandidateFunction,
			ObjectOID:          objectOID,
			NamespaceOID:       11,
			OperandTypeOIDs:    operandTypeOIDs,
			ResultTypeOID:      20,
			Volatility:         EffectVolatilityImmutable,
			CanonicalSignature: signature,
			DatabaseOID:        1,
			ServerVersionNum:   170000,
		},
	}}}
}

// orchestrationCountStarCandidate is the count(*) effect candidate used by the
// white-box orchestration contract.
func orchestrationCountStarCandidate() EffectCandidate {
	return EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"count"},
		OriginalNamePath:     []string{"COUNT"},
		Canonical:            true,
		ParserClassification: "aggregate",
		Arity:                0,
		OperandKinds:         []string{"star"},
		IsAggregate:          true,
	}
}

// orchestrationCountOneCandidate is the exact COUNT(1) effect candidate.
func orchestrationCountOneCandidate() EffectCandidate {
	candidate := orchestrationCountStarCandidate()
	candidate.Arity = 1
	candidate.OperandKinds = []string{"integer_one"}
	return candidate
}

// TestProofOrchestrationContract_OrchestrationOwned exercises the private
// orchestration function directly: proof applicability per dialect, the exact
// COUNT(1) completeness gate before any catalog probe, barrier fail closure,
// and exact reason ownership (successful proof removes only its owned codes;
// failed or inapplicable proof removes nothing).
func TestProofOrchestrationContract_OrchestrationOwned(t *testing.T) {
	policy, err := NewTrustPolicy(PG17Manifest)
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}
	proven := &mockControlledResolver{ctx: testResolutionContext(), batch: orchestrationProofBatch(2803, nil, "pg_catalog.count()")}
	trusted, err := NewTrustedService(proven, policy, &mockSchemaResolver{})
	if err != nil {
		t.Fatalf("trusted service: %v", err)
	}
	provenCountOne := &mockControlledResolver{ctx: testResolutionContext(), batch: orchestrationProofBatch(2147, []uint32{2276}, "pg_catalog.count(2276)")}
	trustedCountOne, err := NewTrustedService(provenCountOne, policy, &mockSchemaResolver{})
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
	fixture, err := newBuiltinSemanticService(&builtinTestResolver{}, mustBuiltinTestRegistry(t))
	if err != nil {
		t.Fatalf("builtin fixture: %v", err)
	}

	pgUnproven := []domain.ReasonCode{domain.ReasonUnprovenFunctionEffect, domain.ReasonUnprovenOperatorEffect}
	mysqlUnproven := []domain.ReasonCode{"function_call", domain.ReasonFunctionEffect}
	residual := []domain.ReasonCode{"residual_reason"}

	pgResult := func(candidates []EffectCandidate, codes []domain.ReasonCode, requirements []domain.Requirement, exactCountOne bool, relations ...domain.RelationReference) QueryAccessResult {
		if len(relations) == 0 {
			relations = []domain.RelationReference{{Schema: "public", Name: "users", Kind: domain.RelationTable, PermissionRequired: true}}
		}
		return QueryAccessResult{
			DomainResult: domain.Result{
				Dialect:            "postgresql",
				Mode:               domain.ModeStrict,
				ReadClassification: domain.Indeterminate,
				Admission:          domain.IndeterminateAdmission,
				ReasonCodes:        codes,
				Relations:          relations,
				Requirements:       requirements,
			},
			EffectCandidates:              candidates,
			ExactCountIntegerOneStatement: exactCountOne,
		}
	}
	mysqlResult := func(candidates []EffectCandidate, codes []domain.ReasonCode, requirements []domain.Requirement, relations ...domain.RelationReference) QueryAccessResult {
		if len(relations) == 0 {
			relations = []domain.RelationReference{{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true}}
		}
		return QueryAccessResult{
			DomainResult: domain.Result{
				Dialect:            "mysql",
				Mode:               domain.ModeStrict,
				ReadClassification: domain.Indeterminate,
				Admission:          domain.IndeterminateAdmission,
				ReasonCodes:        codes,
				Relations:          relations,
				Requirements:       requirements,
			},
			EffectCandidates: candidates,
		}
	}

	oneRequirement := []domain.Requirement{{Object: "public.users", Privilege: "read_table"}}
	mysqlRequirement := []domain.Requirement{{Object: "app.users", Privilege: "read_table"}}
	distinctCount := func() EffectCandidate {
		candidate := builtinTestCandidate()
		candidate.HasDistinct = true
		return candidate
	}

	tests := []struct {
		name           string
		service        *Service
		req            QueryAccessRequest
		extracted      QueryAccessResult
		probe          *mockControlledResolver
		wantAllows     bool
		wantKept       []domain.ReasonCode
		wantProbeCalls int
	}{
		{
			name:    "pg_ordinary_all_proven_removes_only_owned",
			service: trusted, probe: proven,
			req:        QueryAccessRequest{Dialect: "postgresql", AnalysisProfile: AnalysisProfileEmpty},
			extracted:  pgResult([]EffectCandidate{orchestrationCountStarCandidate()}, append(append([]domain.ReasonCode(nil), pgUnproven...), residual...), nil, false),
			wantAllows: true, wantKept: residual, wantProbeCalls: 1,
		},
		{
			name:    "pg_exact_count_one_requirements_complete",
			service: trustedCountOne, probe: provenCountOne,
			req:        QueryAccessRequest{Dialect: "postgresql", AnalysisProfile: AnalysisProfileEmpty},
			extracted:  pgResult([]EffectCandidate{orchestrationCountOneCandidate()}, append(append([]domain.ReasonCode(nil), pgUnproven...), residual...), oneRequirement, true),
			wantAllows: true, wantKept: residual, wantProbeCalls: 1,
		},
		{
			name:    "pg_exact_count_one_requirements_missing_fail_closed_no_probe",
			service: trustedCountOne, probe: provenCountOne,
			req:        QueryAccessRequest{Dialect: "postgresql", AnalysisProfile: AnalysisProfileEmpty},
			extracted:  pgResult([]EffectCandidate{orchestrationCountOneCandidate()}, append(append([]domain.ReasonCode(nil), pgUnproven...), residual...), nil, true),
			wantAllows: false, wantKept: append(append([]domain.ReasonCode(nil), pgUnproven...), residual...), wantProbeCalls: 0,
		},
		{
			name:    "pg_proof_unproven_removes_none",
			service: trustedUnproven, probe: unprovenResolver,
			req:        QueryAccessRequest{Dialect: "postgresql", AnalysisProfile: AnalysisProfileEmpty},
			extracted:  pgResult([]EffectCandidate{orchestrationCountStarCandidate()}, append(append([]domain.ReasonCode(nil), pgUnproven...), residual...), nil, false),
			wantAllows: false, wantKept: append(append([]domain.ReasonCode(nil), pgUnproven...), residual...), wantProbeCalls: 1,
		},
		{
			name:    "pg_no_candidates_never_vacuous",
			service: trusted, probe: proven,
			req:        QueryAccessRequest{Dialect: "postgresql", AnalysisProfile: AnalysisProfileEmpty},
			extracted:  pgResult(nil, append(append([]domain.ReasonCode(nil), pgUnproven...), residual...), nil, false),
			wantAllows: false, wantKept: append(append([]domain.ReasonCode(nil), pgUnproven...), residual...), wantProbeCalls: 0,
		},
		{
			name:    "pg_unqualified_barrier_fail_closed_no_probe",
			service: trusted, probe: proven,
			req: QueryAccessRequest{Dialect: "postgresql", AnalysisProfile: AnalysisProfileEmpty},
			extracted: pgResult([]EffectCandidate{orchestrationCountStarCandidate()}, append(append([]domain.ReasonCode(nil), pgUnproven...), residual...), nil, false,
				[]domain.RelationReference{{Name: "users", Kind: domain.RelationTable, PermissionRequired: true}}...),
			wantAllows: false, wantKept: append(append([]domain.ReasonCode(nil), pgUnproven...), residual...), wantProbeCalls: 0,
		},
		{
			name:    "pg_view_barrier_fail_closed_no_probe",
			service: trusted, probe: proven,
			req: QueryAccessRequest{Dialect: "postgresql", AnalysisProfile: AnalysisProfileEmpty},
			extracted: pgResult([]EffectCandidate{orchestrationCountStarCandidate()}, append(append([]domain.ReasonCode(nil), pgUnproven...), residual...), nil, false,
				[]domain.RelationReference{{Schema: "public", Name: "users", Kind: domain.RelationView, PermissionRequired: true}}...),
			wantAllows: false, wantKept: append(append([]domain.ReasonCode(nil), pgUnproven...), residual...), wantProbeCalls: 0,
		},
		{
			name:       "mysql_candidates_all_proven_removes_only_owned",
			service:    fixture,
			req:        QueryAccessRequest{Dialect: "mysql", AnalysisProfile: AnalysisProfileMySQL57},
			extracted:  mysqlResult([]EffectCandidate{builtinTestCandidate()}, append(append([]domain.ReasonCode(nil), mysqlUnproven...), residual...), mysqlRequirement),
			wantAllows: true, wantKept: residual,
		},
		{
			name:       "mysql_candidates_unproven_removes_none",
			service:    fixture,
			req:        QueryAccessRequest{Dialect: "mysql", AnalysisProfile: AnalysisProfileMySQL57},
			extracted:  mysqlResult([]EffectCandidate{distinctCount()}, append(append([]domain.ReasonCode(nil), mysqlUnproven...), residual...), mysqlRequirement),
			wantAllows: false, wantKept: append(append([]domain.ReasonCode(nil), mysqlUnproven...), residual...),
		},
		{
			name:       "mysql_no_candidates_proof_not_required",
			service:    fixture,
			req:        QueryAccessRequest{Dialect: "mysql", AnalysisProfile: AnalysisProfileMySQL57},
			extracted:  mysqlResult(nil, append(append([]domain.ReasonCode(nil), mysqlUnproven...), residual...), mysqlRequirement),
			wantAllows: true, wantKept: append(append([]domain.ReasonCode(nil), mysqlUnproven...), residual...),
		},
		{
			name:    "mysql_view_barrier_fail_closed",
			service: fixture,
			req:     QueryAccessRequest{Dialect: "mysql", AnalysisProfile: AnalysisProfileMySQL57},
			extracted: mysqlResult([]EffectCandidate{builtinTestCandidate()}, append(append([]domain.ReasonCode(nil), mysqlUnproven...), residual...), mysqlRequirement,
				[]domain.RelationReference{{Schema: "app", Name: "users", Kind: domain.RelationView, PermissionRequired: true}}...),
			wantAllows: false, wantKept: append(append([]domain.ReasonCode(nil), mysqlUnproven...), residual...),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := 0
			if tt.probe != nil {
				before = tt.probe.ctxCalled
			}
			got := tt.service.orchestratePromotionProof(context.Background(), tt.req, &tt.extracted)
			if got.allowsPromotion != tt.wantAllows {
				t.Errorf("allowsPromotion = %v, want %v", got.allowsPromotion, tt.wantAllows)
			}
			if !slices.Equal(tt.extracted.DomainResult.ReasonCodes, tt.wantKept) {
				t.Errorf("kept reasons = %v, want %v (owned removal must leave exactly the residual set)", tt.extracted.DomainResult.ReasonCodes, tt.wantKept)
			}
			if tt.probe != nil && tt.probe.ctxCalled-before != tt.wantProbeCalls {
				t.Errorf("CaptureExecutionBoundContext delta = %d, want %d", tt.probe.ctxCalled-before, tt.wantProbeCalls)
			}
		})
	}
}

// Package queryaccess verifies the application proof-orchestration contract at the Service.Analyze seam.
// input: MySQL/TiDB queries over fixture builtin semantic capability and default services
// output: read_only/admissible only when the no-effect or builtin proof applicability rule permits promotion
// pos: application orchestration contract coverage (untagged builds)
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
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

//go:build postgresql

// Package audit verifies PostgreSQL DML impact estimates at the application seam.
// input: PostgreSQL offline and planner-backed audit requests
// output: shared statement-level impact contracts for PostgreSQL DML
// pos: PostgreSQL-tagged application audit regression coverage
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLPostgreSQLOfflineIDEqualityImpact(t *testing.T) {
	t.Parallel()

	for _, sql := range []string{
		"delete from users where id = 42",
		"delete from users where id = $1",
	} {
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{SQL: sql, Dialect: spec.DialectPostgreSQL})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}
			if len(result.Statements) != 1 || result.Statements[0].Impact == nil {
				t.Fatalf("expected one statement impact, got %#v", result.Statements)
			}

			impact := result.Statements[0].Impact
			if impact.EstimatedRows == nil || *impact.EstimatedRows != 1 {
				t.Fatalf("estimated rows = %#v, want 1", impact.EstimatedRows)
			}
			if impact.RiskLevel != spec.ImpactRiskLow || impact.Confidence != spec.ImpactConfidenceHigh {
				t.Fatalf("risk/confidence = %q/%q, want low/high", impact.RiskLevel, impact.Confidence)
			}
			if impact.Source != spec.ImpactSourceShape || len(impact.ReasonCodes) != 1 || impact.ReasonCodes[0] != "pk_equality" {
				t.Fatalf("source/reasons = %q/%#v, want shape/[pk_equality]", impact.Source, impact.ReasonCodes)
			}
		})
	}
}

func TestAuditSQLPostgreSQLPlannerImpactOverridesShapeEstimate(t *testing.T) {
	t.Parallel()
	provider := &fakeMetadataProvider{
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
		},
		planner: &spec.ImpactEstimate{
			EstimatedRows: ptrInt64(7),
			RiskLevel:     spec.ImpactRiskMedium,
			Confidence:    spec.ImpactConfidenceMedium,
			Source:        spec.ImpactSourcePlan,
			ReasonCodes:   []string{"explain_rows"},
		},
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:     "delete from users where id = $1",
		Dialect: spec.DialectPostgreSQL,
		Metadata: &MetadataRequest{
			Schema:   "public",
			Provider: provider,
		},
	})
	if err != nil {
		t.Fatalf("audit sql with planner: %v", err)
	}
	if provider.plannerCalls != 1 {
		t.Fatalf("planner calls = %d, want 1", provider.plannerCalls)
	}
	if len(result.Statements) != 1 || result.Statements[0].Impact == nil {
		t.Fatalf("expected one statement impact, got %#v", result.Statements)
	}
	impact := result.Statements[0].Impact
	if impact.Source != spec.ImpactSourcePlan || impact.EstimatedRows == nil || *impact.EstimatedRows != 7 {
		t.Fatalf("planner impact = %#v, want source=plan rows=7", impact)
	}
}

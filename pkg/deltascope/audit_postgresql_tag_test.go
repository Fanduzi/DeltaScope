//go:build postgresql

// Package deltascope verifies the public PostgreSQL offline impact contract.
// input: public audit requests for PostgreSQL DML
// output: stable statement-level impact fields exposed to SDK callers
// pos: PostgreSQL-tagged public API regression coverage
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"testing"
)

func TestAuditPostgreSQLOfflineIDEqualityImpact(t *testing.T) {
	t.Parallel()

	result, err := Audit(context.Background(), Request{
		SQL:     "delete from users where id = 42",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 || result.Statements[0].Impact == nil {
		t.Fatalf("expected one statement impact, got %#v", result.Statements)
	}

	impact := result.Statements[0].Impact
	if impact.EstimatedRows == nil || *impact.EstimatedRows != 1 {
		t.Fatalf("estimated rows = %#v, want 1", impact.EstimatedRows)
	}
	if impact.RiskLevel != ImpactRiskLow || impact.Confidence != ImpactConfidenceHigh || impact.Source != ImpactSourceShape {
		t.Fatalf("impact risk/confidence/source = %q/%q/%q, want low/high/shape", impact.RiskLevel, impact.Confidence, impact.Source)
	}
	if len(impact.ReasonCodes) != 1 || impact.ReasonCodes[0] != "pk_equality" {
		t.Fatalf("reason codes = %#v, want [pk_equality]", impact.ReasonCodes)
	}
}

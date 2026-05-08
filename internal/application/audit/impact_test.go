// Package audit verifies conservative shape-only DML impact estimation behavior.
// input: normalized statement specs with extracted DML shape hints
// output: test coverage for shape-only statement impact estimates
// pos: application audit impact-estimation test coverage
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestEstimateStatementImpact(t *testing.T) {
	t.Run("delete without where returns high risk", func(t *testing.T) {
		statement := spec.Statement{
			Kind: spec.KindDML,
			DML: &spec.DML{
				Operation:      spec.DMLOperationDelete,
				HasWhere:       false,
				PredicateShape: spec.PredicateShapeMissingWhere,
			},
		}

		impact := estimateStatementImpact(statement)
		if impact == nil {
			t.Fatalf("expected impact estimate")
		}
		if impact.EstimatedRatio == nil || *impact.EstimatedRatio != 1.0 {
			t.Fatalf("expected full-table ratio, got %#v", impact)
		}
		if impact.RiskLevel != spec.ImpactRiskHigh {
			t.Fatalf("expected high risk, got %q", impact.RiskLevel)
		}
		if impact.Confidence != spec.ImpactConfidenceHigh {
			t.Fatalf("expected high confidence, got %#v", impact)
		}
		if len(impact.ReasonCodes) != 1 || impact.ReasonCodes[0] != "missing_where" {
			t.Fatalf("expected missing_where reason code, got %#v", impact)
		}
		if len(impact.Notes) == 0 {
			t.Fatalf("expected note for missing_where impact, got %#v", impact)
		}
	})

	t.Run("update unique equality shape returns low risk", func(t *testing.T) {
		statement := spec.Statement{
			Kind: spec.KindDML,
			DML: &spec.DML{
				Operation:      spec.DMLOperationUpdate,
				HasWhere:       true,
				PredicateShape: spec.PredicateShapeUniqueEquality,
			},
		}

		impact := estimateStatementImpact(statement)
		if impact == nil {
			t.Fatalf("expected impact estimate")
		}
		if impact.EstimatedRows == nil || *impact.EstimatedRows != 1 {
			t.Fatalf("expected one estimated row, got %#v", impact)
		}
		if impact.RiskLevel != spec.ImpactRiskLow {
			t.Fatalf("expected low risk, got %q", impact.RiskLevel)
		}
		if impact.Confidence != spec.ImpactConfidenceHigh {
			t.Fatalf("expected high confidence, got %#v", impact)
		}
		if len(impact.ReasonCodes) != 1 || impact.ReasonCodes[0] != "pk_equality" {
			t.Fatalf("expected pk_equality reason code, got %#v", impact)
		}
		if len(impact.Notes) == 0 {
			t.Fatalf("expected note for pk equality impact, got %#v", impact)
		}
	})

	t.Run("join shape returns high risk with low confidence", func(t *testing.T) {
		statement := spec.Statement{
			Kind: spec.KindDML,
			DML: &spec.DML{
				Operation:      spec.DMLOperationDelete,
				HasWhere:       true,
				PredicateShape: spec.PredicateShapeJoin,
			},
		}

		impact := estimateStatementImpact(statement)
		if impact == nil {
			t.Fatalf("expected impact estimate")
		}
		if impact.RiskLevel != spec.ImpactRiskHigh {
			t.Fatalf("expected high risk, got %#v", impact)
		}
		if impact.Confidence != spec.ImpactConfidenceLow {
			t.Fatalf("expected low confidence, got %#v", impact)
		}
		if len(impact.Notes) == 0 {
			t.Fatalf("expected note for join impact, got %#v", impact)
		}
	})

	t.Run("unknown fallback ignores statement-level subquery facts", func(t *testing.T) {
		statement := spec.Statement{
			Kind: spec.KindDML,
			DML: &spec.DML{
				Operation:   spec.DMLOperationUpdate,
				HasWhere:    true,
				HasSubquery: true,
			},
		}

		impact := estimateStatementImpact(statement)
		if impact == nil {
			t.Fatalf("expected impact estimate")
		}
		if impact.RiskLevel != spec.ImpactRiskUnknown {
			t.Fatalf("expected unknown risk, got %#v", impact)
		}
		if impact.Confidence != spec.ImpactConfidenceLow {
			t.Fatalf("expected low-confidence fallback, got %#v", impact)
		}
		if len(impact.ReasonCodes) != 1 || impact.ReasonCodes[0] != "shape_unknown" {
			t.Fatalf("expected unknown fallback reason code, got %#v", impact)
		}
	})
}

func TestAttachImpactEstimatesMetadataRefinesUniqueEquality(t *testing.T) {
	statements := []spec.Statement{{
		Kind: spec.KindDML,
		DML: &spec.DML{
			Operation:      spec.DMLOperationUpdate,
			HasWhere:       true,
			PredicateShape: spec.PredicateShapeUniqueEquality,
			LookupColumns:  []string{"id"},
			MatchedKeyName: "PRIMARY",
			MatchedKeyKind: spec.IndexKindPrimary,
			IsSingleTable:  true,
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists: true,
				Table:  &spec.Table{Name: "users"},
				PrimaryKey: &spec.Index{
					Name:    "PRIMARY",
					Kind:    spec.IndexKindPrimary,
					Columns: []string{"id"},
				},
				Options: map[string]string{
					"table_rows": "100",
				},
			},
		},
	}}

	statements = attachImpactEstimates(context.Background(), statements)

	impact := statements[0].DML.Impact
	if impact == nil {
		t.Fatalf("expected impact estimate")
	}
	if impact.Source != spec.ImpactSourceMetadata {
		t.Fatalf("expected metadata source, got %#v", impact)
	}
	if impact.EstimatedRows == nil || *impact.EstimatedRows != 1 {
		t.Fatalf("expected one estimated row, got %#v", impact)
	}
	if impact.EstimatedRatio == nil || *impact.EstimatedRatio != 0.01 {
		t.Fatalf("expected estimated ratio 0.01, got %#v", impact)
	}
}

func TestAttachImpactEstimatesMetadataRefinesSourceForJoinShape(t *testing.T) {
	statements := []spec.Statement{{
		Kind: spec.KindDML,
		DML: &spec.DML{
			Operation:      spec.DMLOperationDelete,
			HasWhere:       true,
			PredicateShape: spec.PredicateShapeJoin,
		},
		Metadata: &spec.Metadata{
			Schema: "app",
			TargetTable: &spec.TableSnapshot{
				Exists: true,
				Table:  &spec.Table{Name: "users"},
			},
		},
	}}

	statements = attachImpactEstimates(context.Background(), statements)

	impact := statements[0].DML.Impact
	if impact == nil {
		t.Fatalf("expected impact estimate")
	}
	if impact.Source != spec.ImpactSourceMetadata {
		t.Fatalf("expected metadata source, got %#v", impact)
	}
	if impact.RiskLevel != spec.ImpactRiskHigh {
		t.Fatalf("expected high risk to stay unchanged, got %#v", impact)
	}
	if impact.Confidence != spec.ImpactConfidenceLow {
		t.Fatalf("expected low confidence to stay unchanged, got %#v", impact)
	}
	if len(impact.ReasonCodes) != 1 || impact.ReasonCodes[0] != string(spec.PredicateShapeJoin) {
		t.Fatalf("expected join reason code to stay unchanged, got %#v", impact)
	}
}

func TestAttachImpactEstimatesMissingTargetTableSnapshotKeepsShapeSource(t *testing.T) {
	statements := []spec.Statement{{
		Kind: spec.KindDML,
		DML: &spec.DML{
			Operation:      spec.DMLOperationDelete,
			HasWhere:       true,
			PredicateShape: spec.PredicateShapeJoin,
		},
		Metadata: &spec.Metadata{
			Schema: "app",
			TargetTable: &spec.TableSnapshot{
				Exists: false,
				Table:  &spec.Table{Name: "users"},
			},
		},
	}}

	statements = attachImpactEstimates(context.Background(), statements)

	impact := statements[0].DML.Impact
	if impact == nil {
		t.Fatalf("expected impact estimate")
	}
	if impact.Source != spec.ImpactSourceShape {
		t.Fatalf("expected missing target-table snapshot to keep shape source, got %#v", impact)
	}
}

func TestAttachImpactEstimatesMissingTargetTableKeepsUniqueEqualityShapeSource(t *testing.T) {
	statements := []spec.Statement{{
		Kind: spec.KindDML,
		DML: &spec.DML{
			Operation:      spec.DMLOperationUpdate,
			HasWhere:       true,
			PredicateShape: spec.PredicateShapeUniqueEquality,
			LookupColumns:  []string{"id"},
			MatchedKeyName: "PRIMARY",
			MatchedKeyKind: spec.IndexKindPrimary,
			IsSingleTable:  true,
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists: false,
				Table:  &spec.Table{Name: "users"},
				PrimaryKey: &spec.Index{
					Name:    "PRIMARY",
					Kind:    spec.IndexKindPrimary,
					Columns: []string{"id"},
				},
				Options: map[string]string{
					"table_rows": "100",
				},
			},
		},
	}}

	statements = attachImpactEstimates(context.Background(), statements)

	impact := statements[0].DML.Impact
	if impact == nil {
		t.Fatalf("expected impact estimate")
	}
	if impact.Source != spec.ImpactSourceShape {
		t.Fatalf("expected missing target-table snapshot to keep shape source, got %#v", impact)
	}
	if impact.EstimatedRatio != nil {
		t.Fatalf("expected no metadata ratio refinement for missing table snapshot, got %#v", impact)
	}
}

func TestAttachImpactEstimatesSchemaOnlyMetadataKeepsShapeSource(t *testing.T) {
	statements := []spec.Statement{{
		Kind: spec.KindDML,
		DML: &spec.DML{
			Operation:      spec.DMLOperationDelete,
			HasWhere:       true,
			PredicateShape: spec.PredicateShapeJoin,
		},
		Metadata: &spec.Metadata{
			Schema: "app",
		},
	}}

	statements = attachImpactEstimates(context.Background(), statements)

	impact := statements[0].DML.Impact
	if impact == nil {
		t.Fatalf("expected impact estimate")
	}
	if impact.Source != spec.ImpactSourceShape {
		t.Fatalf("expected schema-only metadata to keep shape source, got %#v", impact)
	}
}

func TestAttachImpactEstimatesKeepsShapeOnlyImpactOffline(t *testing.T) {
	statements := []spec.Statement{{
		Kind: spec.KindDML,
		DML: &spec.DML{
			Operation:      spec.DMLOperationDelete,
			HasWhere:       false,
			PredicateShape: spec.PredicateShapeMissingWhere,
		},
	}}

	statements = attachImpactEstimates(context.Background(), statements)

	impact := statements[0].DML.Impact
	if impact == nil {
		t.Fatalf("expected impact estimate")
	}
	if impact.Source != spec.ImpactSourceShape {
		t.Fatalf("expected shape-only source, got %#v", impact)
	}
	if impact.EstimatedRatio == nil || *impact.EstimatedRatio != 1.0 {
		t.Fatalf("expected full-table ratio, got %#v", impact)
	}
}

func TestAttachImpactEstimatesUsesPlanEstimateWhenAvailable(t *testing.T) {
	provider := &planEstimateProviderStub{
		estimate: &spec.ImpactEstimate{
			EstimatedRows: ptrInt64(7),
			RiskLevel:     spec.ImpactRiskMedium,
			Confidence:    spec.ImpactConfidenceMedium,
			Source:        spec.ImpactSourcePlan,
			ReasonCodes:   []string{"explain_rows"},
		},
	}
	statements := []spec.Statement{{
		Kind:   spec.KindDML,
		RawSQL: "update users set active = false where id = 42",
		DML: &spec.DML{
			Operation:      spec.DMLOperationUpdate,
			HasWhere:       true,
			PredicateShape: spec.PredicateShapeUniqueEquality,
		},
	}}

	statements = attachImpactEstimatesWithPlanner(context.Background(), provider, statements)

	impact := statements[0].DML.Impact
	if impact == nil {
		t.Fatalf("expected impact estimate")
	}
	if impact.Source != spec.ImpactSourcePlan {
		t.Fatalf("expected plan-backed source, got %#v", impact)
	}
	if impact.EstimatedRows == nil || *impact.EstimatedRows != 7 {
		t.Fatalf("expected planner estimated rows 7, got %#v", impact)
	}
	if provider.calls != 1 {
		t.Fatalf("expected one planner call, got %d", provider.calls)
	}
}

func TestAttachImpactEstimatesPlannerEstimateCanBeRefinedWithMetadata(t *testing.T) {
	provider := &planEstimateProviderStub{
		estimate: &spec.ImpactEstimate{
			EstimatedRows: ptrInt64(7),
			RiskLevel:     spec.ImpactRiskMedium,
			Confidence:    spec.ImpactConfidenceMedium,
			Source:        spec.ImpactSourcePlan,
			ReasonCodes:   []string{"explain_rows"},
			Notes:         []string{"plain EXPLAIN planner rows estimate"},
		},
	}
	statements := []spec.Statement{{
		Kind:   spec.KindDML,
		RawSQL: "update users set active = false where id = 42",
		DML: &spec.DML{
			Operation:      spec.DMLOperationUpdate,
			HasWhere:       true,
			PredicateShape: spec.PredicateShapeUniqueEquality,
			LookupColumns:  []string{"id"},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists: true,
				Table:  &spec.Table{Name: "users"},
				PrimaryKey: &spec.Index{
					Name:    "PRIMARY",
					Kind:    spec.IndexKindPrimary,
					Columns: []string{"id"},
				},
				Options: map[string]string{
					"table_rows": "100",
				},
			},
		},
	}}

	statements = attachImpactEstimatesWithPlanner(context.Background(), provider, statements)

	impact := statements[0].DML.Impact
	if impact == nil {
		t.Fatalf("expected impact estimate")
	}
	if impact.Source != spec.ImpactSourcePlan {
		t.Fatalf("expected planner source to remain authoritative, got %#v", impact)
	}
	if impact.EstimatedRows == nil || *impact.EstimatedRows != 7 {
		t.Fatalf("expected planner estimated rows 7, got %#v", impact)
	}
	if impact.EstimatedRatio == nil || *impact.EstimatedRatio != 0.07 {
		t.Fatalf("expected metadata-refined ratio 0.07, got %#v", impact)
	}
}

func TestAttachImpactEstimatesSkipsUnsupportedStatementKindsForPlanner(t *testing.T) {
	provider := &planEstimateProviderStub{}
	statements := []spec.Statement{{
		Kind:   spec.KindDDL,
		RawSQL: "create table users (id bigint primary key)",
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "users"},
		},
	}}

	statements = attachImpactEstimatesWithPlanner(context.Background(), provider, statements)

	if statements[0].DML != nil {
		t.Fatalf("expected no dml payload, got %#v", statements[0])
	}
	if provider.calls != 0 {
		t.Fatalf("expected planner to skip unsupported statement kinds, got %d calls", provider.calls)
	}
}

func TestAttachImpactEstimatesFallsBackWhenPlanEstimateFails(t *testing.T) {
	provider := &planEstimateProviderStub{err: errors.New("planner unavailable")}
	statements := []spec.Statement{{
		Kind:   spec.KindDML,
		RawSQL: "delete from users",
		DML: &spec.DML{
			Operation:      spec.DMLOperationDelete,
			HasWhere:       false,
			PredicateShape: spec.PredicateShapeMissingWhere,
		},
	}}

	statements = attachImpactEstimatesWithPlanner(context.Background(), provider, statements)

	impact := statements[0].DML.Impact
	if impact == nil {
		t.Fatalf("expected fallback impact estimate")
	}
	if impact.Source != spec.ImpactSourceShape {
		t.Fatalf("expected fallback to shape source, got %#v", impact)
	}
	if impact.EstimatedRatio == nil || *impact.EstimatedRatio != 1.0 {
		t.Fatalf("expected shape fallback full-table ratio, got %#v", impact)
	}
}

func TestAttachImpactEstimatesPlannerRatioIsClampedToOne(t *testing.T) {
	provider := &planEstimateProviderStub{
		estimate: &spec.ImpactEstimate{
			EstimatedRows: ptrInt64(500),
			RiskLevel:     spec.ImpactRiskHigh,
			Confidence:    spec.ImpactConfidenceMedium,
			Source:        spec.ImpactSourcePlan,
		},
	}
	statements := []spec.Statement{{
		Kind:   spec.KindDML,
		RawSQL: "update users set active = false where tenant_id = 42",
		DML: &spec.DML{
			Operation:      spec.DMLOperationUpdate,
			HasWhere:       true,
			PredicateShape: spec.PredicateShapeUniqueEquality,
			LookupColumns:  []string{"id"},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists: true,
				Table:  &spec.Table{Name: "users"},
				PrimaryKey: &spec.Index{
					Name:    "PRIMARY",
					Kind:    spec.IndexKindPrimary,
					Columns: []string{"id"},
				},
				Options: map[string]string{"table_rows": "100"},
			},
		},
	}}

	statements = attachImpactEstimatesWithPlanner(context.Background(), provider, statements)

	impact := statements[0].DML.Impact
	if impact == nil || impact.EstimatedRatio == nil {
		t.Fatalf("expected refined impact ratio, got %#v", impact)
	}
	if *impact.EstimatedRatio != 1.0 {
		t.Fatalf("expected ratio to clamp at 1.0, got %#v", impact)
	}
}

type planEstimateProviderStub struct {
	estimate *spec.ImpactEstimate
	err      error
	calls    int
}

func (s *planEstimateProviderStub) LoadPlanEstimate(_ context.Context, _ spec.Statement) (*spec.ImpactEstimate, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.estimate, nil
}

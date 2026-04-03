// Package audit orchestrates audit use cases at the application layer.
// input: extracted statement-local DML shape facts plus optional metadata snapshots for refinement
// output: conservative DML impact estimates attached during extraction and upgraded after metadata enrichment
// pos: application impact estimation step between extraction, metadata enrichment, and rule evaluation
// note: if this file changes, update this header and module README.md.
package audit

import (
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func attachImpactEstimates(statements []spec.Statement) []spec.Statement {
	if len(statements) == 0 {
		return statements
	}

	attached := make([]spec.Statement, len(statements))
	for i, statement := range statements {
		attached[i] = statement
		if statement.DML == nil {
			continue
		}

		dml := *statement.DML
		dml.Impact = estimateStatementImpact(statement)
		attached[i].DML = &dml
	}

	return attached
}

func estimateStatementImpact(statement spec.Statement) *spec.ImpactEstimate {
	estimate := estimateShapeOnlyImpact(statement)
	return refineImpactEstimateWithMetadata(statement, estimate)
}

func estimateShapeOnlyImpact(statement spec.Statement) *spec.ImpactEstimate {
	if statement.DML == nil {
		return nil
	}

	switch statement.DML.Operation {
	case spec.DMLOperationUpdate, spec.DMLOperationDelete:
	default:
		return nil
	}

	shape := statement.DML.PredicateShape
	if shape == "" {
		shape = fallbackPredicateShape(statement.DML)
	}

	estimate := &spec.ImpactEstimate{
		Source: spec.ImpactSourceShape,
	}

	switch shape {
	case spec.PredicateShapeMissingWhere:
		estimate.EstimatedRatio = ptrFloat64(1.0)
		estimate.RiskLevel = spec.ImpactRiskHigh
		estimate.Confidence = spec.ImpactConfidenceHigh
		estimate.ReasonCodes = []string{"missing_where"}
		estimate.Notes = []string{"statement can affect the full target set without a WHERE predicate"}
	case spec.PredicateShapeUniqueEquality:
		estimate.EstimatedRows = ptrInt64(1)
		estimate.RiskLevel = spec.ImpactRiskLow
		estimate.Confidence = spec.ImpactConfidenceHigh
		estimate.ReasonCodes = []string{"pk_equality"}
		estimate.Notes = []string{"single-table id equality is treated as a primary-key lookup"}
	case spec.PredicateShapeJoin, spec.PredicateShapeSubquery, spec.PredicateShapeNonSargable:
		estimate.RiskLevel = spec.ImpactRiskHigh
		estimate.Confidence = spec.ImpactConfidenceLow
		estimate.ReasonCodes = []string{string(shape)}
		estimate.Notes = []string{"shape suggests broad or hard-to-bound impact without metadata refinement"}
	default:
		estimate.RiskLevel = spec.ImpactRiskUnknown
		estimate.Confidence = spec.ImpactConfidenceLow
		estimate.ReasonCodes = []string{"shape_unknown"}
	}

	return estimate
}

func refineImpactEstimateWithMetadata(statement spec.Statement, estimate *spec.ImpactEstimate) *spec.ImpactEstimate {
	if estimate == nil {
		return estimate
	}
	if statement.Metadata == nil || statement.Metadata.TargetTable == nil || !statement.Metadata.TargetTable.Exists {
		return estimate
	}

	if statement.DML != nil && statement.DML.PredicateShape == spec.PredicateShapeUniqueEquality && metadataConfirmsPrimaryKeyID(statement) {
		refined := cloneImpactEstimate(estimate)
		refined.EstimatedRows = ptrInt64(1)
		refined.RiskLevel = spec.ImpactRiskLow
		refined.Confidence = spec.ImpactConfidenceHigh
		refined.Source = spec.ImpactSourceMetadata
		refined.ReasonCodes = []string{"pk_equality"}
		refined.Notes = []string{"metadata confirmed PRIMARY(id) for the target table"}

		if ratio, ok := estimateRatioFromTableRows(statement.Metadata.TargetTable); ok {
			refined.EstimatedRatio = ptrFloat64(ratio)
			refined.Notes = append(refined.Notes, "table_rows metadata refined the affected-row ratio")
		}

		return refined
	}

	refined := cloneImpactEstimate(estimate)
	refined.Source = spec.ImpactSourceMetadata
	return refined
}

func metadataConfirmsPrimaryKeyID(statement spec.Statement) bool {
	if statement.DML == nil || statement.Metadata == nil || statement.Metadata.TargetTable == nil || !statement.Metadata.TargetTable.Exists {
		return false
	}
	if len(statement.DML.LookupColumns) != 1 || !strings.EqualFold(statement.DML.LookupColumns[0], "id") {
		return false
	}

	primaryKey := statement.Metadata.TargetTable.PrimaryKey
	if primaryKey == nil || primaryKey.Kind != spec.IndexKindPrimary {
		return false
	}
	return len(primaryKey.Columns) == 1 && strings.EqualFold(primaryKey.Columns[0], "id")
}

func estimateRatioFromTableRows(snapshot *spec.TableSnapshot) (float64, bool) {
	if snapshot == nil || snapshot.Options == nil {
		return 0, false
	}

	raw := strings.TrimSpace(snapshot.Options["table_rows"])
	if raw == "" {
		return 0, false
	}

	rows, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || rows <= 0 {
		return 0, false
	}

	return 1 / float64(rows), true
}

func cloneImpactEstimate(estimate *spec.ImpactEstimate) *spec.ImpactEstimate {
	if estimate == nil {
		return nil
	}

	cloned := *estimate
	if estimate.ReasonCodes != nil {
		cloned.ReasonCodes = append([]string(nil), estimate.ReasonCodes...)
	}
	if estimate.Notes != nil {
		cloned.Notes = append([]string(nil), estimate.Notes...)
	}
	return &cloned
}

func fallbackPredicateShape(dml *spec.DML) spec.PredicateShape {
	switch {
	case dml == nil:
		return spec.PredicateShapeUnknown
	case dml.HasJoin:
		return spec.PredicateShapeJoin
	case !dml.HasWhere:
		return spec.PredicateShapeMissingWhere
	default:
		return spec.PredicateShapeUnknown
	}
}

func ptrInt64(value int64) *int64 {
	return &value
}

func ptrFloat64(value float64) *float64 {
	return &value
}

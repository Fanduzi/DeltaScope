// Package queryaccess implements requirement generation for query access analysis.
// input: resolved query access facts (relations, columns, outputs, unresolved) and mode
// output: access requirements, warnings, and reason codes
// pos: application requirement layer bridging resolved facts to permission requirements
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"fmt"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// buildRequirements generates access requirements based on mode.
// Both modes require every permission-bearing relation.
// Strict requires every resolved source column.
// Projection-only requires only output-contributing columns and emits inference_risk warning.
// Required unresolved references produce indeterminate requirements.
func buildRequirements(
	mode domain.Mode,
	relations []domain.RelationReference,
	columns []domain.ColumnReference,
	outputs []domain.OutputColumn,
	unresolved []domain.Unresolved,
) ([]domain.Requirement, []domain.WarningCode, []domain.ReasonCode, error) {
	if err := domain.ValidateMode(mode); err != nil {
		return nil, nil, nil, fmt.Errorf("build requirements: %w", err)
	}

	var reqs []domain.Requirement

	// 1. Both modes: require every permission-bearing relation
	for _, rel := range relations {
		if !rel.PermissionRequired {
			continue
		}
		key := domain.FormatRelationKey(rel.Schema, rel.Name)
		reqs = append(reqs, domain.Requirement{
			Object:    key,
			Privilege: "read_table",
		})
	}

	// 2. Column requirements depend on mode
	outputKeys := buildOutputSourceKeys(outputs)

	switch mode {
	case domain.ModeStrict:
		// Strict: require every resolved source column
		for _, col := range columns {
			key := domain.FormatColumnKey(col.Schema, col.Table, col.Column)
			reqs = append(reqs, domain.Requirement{
				Object:    key,
				Privilege: "read_column",
			})
		}
	case domain.ModeProjectionOnly:
		// Projection-only: require only output-contributing columns
		var warnings []domain.WarningCode
		hasNonOutput := false

		for _, col := range columns {
			key := domain.FormatColumnKey(col.Schema, col.Table, col.Column)
			if outputKeys[key] {
				reqs = append(reqs, domain.Requirement{
					Object:    key,
					Privilege: "read_column",
				})
			} else {
				hasNonOutput = true
			}
		}

		// Emit exactly one inference_risk warning when non-output columns exist
		if hasNonOutput {
			warnings = append(warnings, domain.WarningInferenceRisk)
		}

		// 3. Required unresolved → indeterminate
		reqs = appendUnresolvedRequirements(reqs, unresolved)

		return domain.SortRequirements(reqs), warnings, nil, nil
	}

	// 3. Required unresolved → indeterminate (strict path)
	reqs = appendUnresolvedRequirements(reqs, unresolved)

	return domain.SortRequirements(reqs), nil, nil, nil
}

// buildOutputSourceKeys builds a set of canonical column keys from output sources.
func buildOutputSourceKeys(outputs []domain.OutputColumn) map[string]bool {
	keys := make(map[string]bool)
	for _, out := range outputs {
		for _, src := range out.Sources {
			keys[src] = true
		}
	}
	return keys
}

// appendUnresolvedRequirements adds indeterminate requirements for unresolved references.
func appendUnresolvedRequirements(reqs []domain.Requirement, unresolved []domain.Unresolved) []domain.Requirement {
	for _, u := range unresolved {
		reqs = append(reqs, domain.Requirement{
			Object:    u.Reference,
			Privilege: "indeterminate",
		})
	}
	return reqs
}

// BuildRequirements exposes requirement generation for testing.
func BuildRequirements(
	mode domain.Mode,
	relations []domain.RelationReference,
	columns []domain.ColumnReference,
	outputs []domain.OutputColumn,
	unresolved []domain.Unresolved,
) ([]domain.Requirement, []domain.WarningCode, []domain.ReasonCode, error) {
	return buildRequirements(mode, relations, columns, outputs, unresolved)
}

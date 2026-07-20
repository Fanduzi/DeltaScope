//go:build postgresql

// Package postgresqlmeta resolves direct-column operand types for exact catalog lookup.
// input: session-pinned catalog, effect candidate, and captured search-path OIDs
// output: complete ordered argument type OIDs or a fail-closed miss
// pos: PostgreSQL scalar overload resolution helper
package postgresqlmeta

import (
	"context"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
)

func resolveDirectColumnOperandTypeOIDs(ctx context.Context, catalog effectIdentityCatalog, candidate appqa.EffectCandidate, searchPathOIDs []uint32) ([]uint32, bool) {
	if !candidateHasDirectColumnOperands(candidate) {
		return nil, false
	}

	typeOIDs := make([]uint32, 0, len(candidate.OperandColumnRefs))
	for _, ref := range candidate.OperandColumnRefs {
		oid, err := resolveOneColumnTypeOIDWithCatalog(ctx, catalog, ref, searchPathOIDs)
		if err != nil || oid == 0 {
			return nil, false
		}
		typeOIDs = append(typeOIDs, oid)
	}
	return typeOIDs, true
}

func candidateHasDirectColumnOperands(candidate appqa.EffectCandidate) bool {
	if candidate.Kind == appqa.EffectCandidateOperator {
		return candidate.Arity == 2 && len(candidate.OperandColumnRefs) == 2
	}
	if candidate.Kind != appqa.EffectCandidateFunction || candidate.Arity < 1 || len(candidate.OperandKinds) != candidate.Arity || len(candidate.OperandColumnRefs) != candidate.Arity {
		return false
	}
	for _, kind := range candidate.OperandKinds {
		if kind != "column" {
			return false
		}
	}
	return true
}

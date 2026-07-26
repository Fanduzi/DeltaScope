// Package queryaccess defines the fail-closed Phase-1 pure-effect eligibility gate.
// input: internal effect candidates collected from PostgreSQL query extraction
// output: bounded eligibility decision and reason code
// pos: application proof boundary before catalog identity promotion
package queryaccess

import (
	"strings"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// ValidatePhase1PureEffectCandidates checks whether every candidate is inside
// the Phase-1 proof boundary. It preserves candidates and returns only a
// bounded reason for the first ineligible candidate.
func ValidatePhase1PureEffectCandidates(candidates []EffectCandidate) (bool, domain.ReasonCode) {
	for _, candidate := range candidates {
		switch candidate.Kind {
		case EffectCandidateOperator:
			continue
		case EffectCandidateFunction:
			if !phase1FunctionEligible(candidate) {
				return false, domain.ReasonUnprovenFunctionEffect
			}
		case EffectCandidateCast:
			return false, domain.ReasonUnprovenCastEffect
		default:
			return false, domain.ReasonIdentityLookupFailed
		}
	}
	return true, ""
}

func phase1FunctionEligible(candidate EffectCandidate) bool {
	if candidate.Arity < 0 || candidate.HasFilter || candidate.HasDistinct ||
		candidate.HasAggOrder || candidate.HasWithinGroup {
		return false
	}
	if candidate.HasFrame {
		return false
	}

	switch candidate.Arity {
	case 0:
		if isCountStar(candidate) {
			return true
		}
		return candidate.HasWindow && isWindowRankingFunction(candidate) &&
			len(candidate.OperandKinds) == 0 && len(candidate.OperandColumnRefs) == 0
	case 1:
		if candidate.HasWindow {
			return false
		}
		if len(candidate.OperandKinds) != 1 {
			return false
		}
		kind := candidate.OperandKinds[0]
		if kind == "column" {
			return len(candidate.OperandColumnRefs) == 1
		}
		if kind == "const" {
			return true // literal-only eligible; requires physical relation from enclosing query
		}
		return false
	default:
		if candidate.HasWindow {
			return false
		}
		return scalarFunctionEligible(candidate)
	}
}

func scalarFunctionEligible(candidate EffectCandidate) bool {
	if candidate.IsAggregate || candidate.HasWindow || candidate.HasFilter || candidate.HasDistinct {
		return false
	}
	if len(candidate.OperandKinds) == 0 {
		return false
	}
	for _, kind := range candidate.OperandKinds {
		switch kind {
		case "column":
			// column operands are allowed
		case "const":
			// const operands are allowed
		default:
			return false
		}
	}
	return true
}

func isCountStar(candidate EffectCandidate) bool {
	return strings.EqualFold(candidateCanonicalName(candidate), "count") &&
		len(candidate.OperandKinds) == 1 && candidate.OperandKinds[0] == "star" &&
		len(candidate.OperandColumnRefs) == 0
}

func isWindowRankingFunction(candidate EffectCandidate) bool {
	switch strings.ToLower(candidateCanonicalName(candidate)) {
	case "row_number", "rank", "dense_rank":
		return true
	default:
		return false
	}
}

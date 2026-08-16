// Package queryaccess owns the single proof-orchestration point between requirements and final state.
// input: service capability, request, and extracted result with requirements already attached
// output: promotion permission fact plus the bounded set of owned reason codes removed by successful proof
// pos: application proof sequencing before the final normalize/reclassify/admission computation
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// promotionProof carries only what the common pipeline needs after proof
// orchestration: whether proof permits the common promotion checks to
// continue, and the bounded set of owned reason codes a successful proof
// removed. reasonCodes exists for the reason-ownership audit (successful
// proof removes only its owned codes; failed or inapplicable proof removes
// none); the pipeline consumes only allowsPromotion. It is not public, not
// serialized, and not an extensibility point.
type promotionProof struct {
	allowsPromotion bool
	reasonCodes     []domain.ReasonCode
}

// orchestratePromotionProof is the single application sequencing point for
// every Effect Proof path. Requirements must already be attached to the
// result. It routes ordinary PostgreSQL manifest proof, exact PostgreSQL
// COUNT(1) completeness-gated manifest proof, MySQL/TiDB builtin semantic
// proof, and the no-effect applicability rule through one fail-closed switch,
// and removes only the owned reason codes of a successful proof. The result
// is not an admission decision: resolvers, classification, reasons,
// unresolved facts, requirements, and barriers still govern promotion.
func (s *Service) orchestratePromotionProof(ctx context.Context, req QueryAccessRequest, extracted *QueryAccessResult) promotionProof {
	// Default: fail closed — no proof, no promotion permission.
	var result promotionProof
	if extracted == nil {
		return result
	}

	// Promotion Barriers: no proof runs and none may permit promotion.
	hasView := false
	for _, rel := range extracted.DomainResult.Relations {
		if rel.Kind == domain.RelationView {
			hasView = true
			break
		}
	}
	if hasView || (req.Dialect == "postgresql" && hasUnqualifiedRelation(extracted.DomainResult.Relations)) {
		return result
	}

	candidates := extracted.EffectCandidates
	switch req.Dialect {
	case "postgresql":
		if s == nil || s.trusted == nil || len(candidates) == 0 {
			// No applicable trusted proof: an indeterminate result stays indeterminate.
			return result
		}
		if hasExactCountIntegerOneCandidate(candidates) &&
			!countIntegerOneRequirementsComplete(extracted.DomainResult, extracted.DomainResult.Requirements, candidates, extracted.ExactCountIntegerOneStatement) {
			// Exact COUNT(1) completeness predicate failed: fail closed.
			return result
		}
		proof := s.resolveAndProveEffects(ctx, req, *extracted)
		if proof == nil || proof.decision != TrustDecisionAllProven {
			return result
		}
		kept, removed := removeUnprovenEffectReasons(extracted.DomainResult.ReasonCodes)
		extracted.DomainResult.ReasonCodes = kept
		result.allowsPromotion = true
		result.reasonCodes = removed
		return result
	case "mysql", "tidb":
		if len(candidates) == 0 {
			// No effect candidates: proof is not required and the ordinary
			// common promotion checks may proceed.
			result.allowsPromotion = true
			return result
		}
		if s == nil || s.builtinSemantic == nil {
			// Candidates with no semantic capability: fail closed.
			return result
		}
		proof := proveBuiltinSemantics(
			req.AnalysisProfile,
			req.Dialect,
			candidates,
			extracted.DomainResult,
			extracted.DomainResult.Requirements,
			s.builtinSemantic.registry,
		)
		if proof.decision != builtinSemanticAllProven {
			return result
		}
		kept, removed := removeBuiltinSemanticReason(extracted.DomainResult.ReasonCodes)
		extracted.DomainResult.ReasonCodes = kept
		result.allowsPromotion = true
		result.reasonCodes = removed
		return result
	default:
		return result
	}
}

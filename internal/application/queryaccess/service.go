// Package queryaccess provides the application service for query access analysis.
// input: SQL text, dialect, mode, default schema, and optional schema resolver
// output: domain-typed query access results with optional metadata resolution
// pos: application orchestration layer for query access analysis
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// ErrExtractionFailed indicates query access extraction failed without exposing SQL text.
var ErrExtractionFailed = errors.New("query access extraction failed")

// ErrTrustedBundleInvalid indicates the trusted bundle failed validation.
var ErrTrustedBundleInvalid = errors.New("invalid trusted bundle")

// unqualifiedRelationReference is a bounded, non-leaking machine identifier for the
// indeterminate requirement produced by unqualified PostgreSQL base relations.
const unqualifiedRelationReference = "unqualified_relation"

// trustProofResult holds the outcome of effect identity resolution and manifest proof.
// Internal only — never exposed on public JSON.
type trustProofResult struct {
	decision    TrustDecision
	reasonCodes []domain.ReasonCode
	batch       EffectIdentityBatch
}

// trustedBundle holds internal-only dependencies for PostgreSQL manifest proof.
// It is never exposed on public SDK/CLI/HTTP request or result schemas.
// When present, it enables effect identity resolution and manifest-gated promotion.
// effectResolver must be a ControlledEffectIdentityResolver so the application
// can capture and validate execution-bound context before resolution.
type trustedBundle struct {
	effectResolver ControlledEffectIdentityResolver
	trustPolicy    *TrustPolicy
	schemaResolver SchemaResolver
}

// validate checks the bundle for completeness.
func (b *trustedBundle) validate() error {
	if b == nil {
		return nil // nil bundle is valid (no trust)
	}
	if b.effectResolver == nil {
		return fmt.Errorf("%w: missing effect resolver", ErrTrustedBundleInvalid)
	}
	if b.trustPolicy == nil {
		return fmt.Errorf("%w: missing trust policy", ErrTrustedBundleInvalid)
	}
	if b.schemaResolver == nil {
		return fmt.Errorf("%w: missing schema resolver", ErrTrustedBundleInvalid)
	}
	return nil
}

// Service orchestrates query access analysis.
type Service struct {
	// trusted is internal-only; zero-value Service has no trust bundle.
	trusted *trustedBundle
}

// NewService creates a basic Service without manifest proof (fail-closed for PG effects).
func NewService() *Service {
	return &Service{}
}

// NewTrustedService creates a Service with PostgreSQL manifest proof capability.
// effectResolver must be a ControlledEffectIdentityResolver that can capture
// execution-bound context (pinned session with TOCTOU protection).
// All dependencies must be non-nil. The trust policy's manifest is validated
// on construction.
func NewTrustedService(effectResolver ControlledEffectIdentityResolver, trustPolicy *TrustPolicy, schemaResolver SchemaResolver) (*Service, error) {
	bundle := &trustedBundle{
		effectResolver: effectResolver,
		trustPolicy:    trustPolicy,
		schemaResolver: schemaResolver,
	}
	if err := bundle.validate(); err != nil {
		return nil, err
	}
	return &Service{trusted: bundle}, nil
}

// Analyze performs query access analysis with optional metadata resolution.
// When SchemaResolver is nil, wildcards and unqualified columns remain unresolved.
// For PostgreSQL with a trusted bundle, effect identities are resolved and
// manifest-proven effects may promote classification from indeterminate to read_only.
func (s *Service) Analyze(ctx context.Context, req QueryAccessRequest) (QueryAccessResult, error) {
	if err := ctx.Err(); err != nil {
		return QueryAccessResult{}, fmt.Errorf("analyze cancelled: %w", err)
	}

	extracted, err := extractByDialect(ctx, req)
	if err != nil {
		return QueryAccessResult{}, ErrExtractionFailed
	}

	// Use trusted bundle's schema resolver if available, otherwise request's.
	schemaResolver := req.SchemaResolver
	if s != nil && s.trusted != nil && s.trusted.schemaResolver != nil {
		schemaResolver = s.trusted.schemaResolver
	}

	// Barrier is unconditional for PostgreSQL when unqualified physical base
	// relations exist. This prevents unsafe DefaultSchema binding regardless
	// of whether a trusted bundle or schema resolver is present.
	hasUnqualified := req.Dialect == "postgresql" && hasUnqualifiedRelation(extracted.DomainResult.Relations)

	if hasUnqualified {
		unboundNames := make(map[string]struct{})
		for i := range extracted.DomainResult.Relations {
			rel := &extracted.DomainResult.Relations[i]
			if rel.Schema == "" && rel.Kind != domain.RelationCTE && rel.Kind != domain.RelationDerived {
				rel.Unbound = true
				unboundNames[strings.ToLower(rel.Name)] = struct{}{}
			}
		}
		extracted.DomainResult.Unresolved = append(extracted.DomainResult.Unresolved, domain.Unresolved{
			Reference: unqualifiedRelationReference,
			Reason:    domain.ReasonUnqualifiedRelationBlocked,
		})
	}

	if schemaResolver != nil {
		extracted.DomainResult = resolveMetadata(ctx, schemaResolver, req.Dialect, req.DefaultSchema, extracted.DomainResult)
	}

	if hasUnqualified {
		for i := range extracted.DomainResult.ReferencedColumns {
			col := &extracted.DomainResult.ReferencedColumns[i]
			if col.Schema == "" {
				col.Unbound = true
			}
		}
	}

	if hasFunctionCallReasonCode(extracted.DomainResult.ReasonCodes) {
		extracted.DomainResult.ReasonCodes = append(extracted.DomainResult.ReasonCodes, domain.ReasonFunctionEffect)
	}

	var proofResult *trustProofResult
	if hasUnqualified {
		// Barrier blocks promotion of read-only analysis, but must not
		// overwrite detected writes (not_read_only/rejected).
		if extracted.DomainResult.ReadClassification == domain.NotReadOnly {
			extracted.DomainResult.Admission = domain.Rejected
		} else {
			extracted.DomainResult.ReadClassification = domain.Indeterminate
			extracted.DomainResult.Admission = domain.IndeterminateAdmission
		}
		extracted.DomainResult.ReasonCodes = append(extracted.DomainResult.ReasonCodes, domain.ReasonUnqualifiedRelationBlocked)
	} else if req.Dialect == "postgresql" && s != nil && s.trusted != nil && len(extracted.EffectCandidates) > 0 {
		proofResult = s.resolveAndProveEffects(ctx, req, extracted)
		if proofResult != nil && proofResult.decision == TrustDecisionAllProven {
			extracted.DomainResult.ReasonCodes = removeUnprovenEffectReasons(extracted.DomainResult.ReasonCodes)
		}
	}

	extracted.DomainResult.ReasonCodes = domain.NormalizeReasonCodes(extracted.DomainResult.ReasonCodes)

	hasResolver := schemaResolver != nil
	extracted.DomainResult.Admission = recomputeAdmission(extracted.DomainResult.ReadClassification, extracted.DomainResult.Admission, extracted.DomainResult.Unresolved, hasResolver)
	extracted.DomainResult.ReadClassification = reclassifyAfterResolution(
		extracted.DomainResult.ReadClassification,
		extracted.DomainResult.ReasonCodes,
		extracted.DomainResult.Unresolved,
		hasResolver,
		req.Dialect,
		proofResult,
	)
	extracted.DomainResult.Admission = recomputeAdmission(extracted.DomainResult.ReadClassification, extracted.DomainResult.Admission, extracted.DomainResult.Unresolved, hasResolver)

	// Build requirements based on mode
	reqs, warnings, _, reqErr := buildRequirements(
		extracted.DomainResult.Mode,
		extracted.DomainResult.Relations,
		extracted.DomainResult.ReferencedColumns,
		extracted.DomainResult.Outputs,
		extracted.DomainResult.Unresolved,
	)
	if reqErr != nil {
		return QueryAccessResult{}, fmt.Errorf("build requirements: %w", reqErr)
	}
	extracted.DomainResult.Requirements = reqs
	extracted.DomainResult.Warnings = append(extracted.DomainResult.Warnings, warnings...)

	extracted.DomainResult.Relations = domain.SortRelations(extracted.DomainResult.Relations)
	extracted.DomainResult.ReferencedColumns = domain.SortColumns(extracted.DomainResult.ReferencedColumns)
	extracted.DomainResult.Requirements = domain.SortRequirements(extracted.DomainResult.Requirements)
	// Outputs preserve SELECT declaration order — do NOT sort.
	extracted.DomainResult.Unresolved = domain.SortUnresolved(extracted.DomainResult.Unresolved)
	extracted.DomainResult.ReasonCodes = domain.NormalizeReasonCodes(extracted.DomainResult.ReasonCodes)
	extracted.DomainResult.Warnings = domain.SortWarningCodes(extracted.DomainResult.Warnings)

	if err := domain.ValidateResult(&extracted.DomainResult); err != nil {
		return QueryAccessResult{}, fmt.Errorf("invalid result: %w", err)
	}

	return extracted, nil
}

// resolveAndProveEffects resolves effect identities and applies manifest proof.
// Returns nil if resolution fails or no candidates exist.
//
// The application captures execution-bound context from the controlled resolver
// and sets it explicitly on the request. This prevents generic resolvers from
// silently filling in unbound context that the application cannot verify.
func (s *Service) resolveAndProveEffects(ctx context.Context, req QueryAccessRequest, extracted QueryAccessResult) *trustProofResult {
	if s == nil || s.trusted == nil {
		return nil
	}

	// Capture explicit execution-bound context from the controlled resolver.
	// This proves the facts are bound to the expected execution session.
	resolutionCtx, err := s.trusted.effectResolver.CaptureExecutionBoundContext(ctx)
	if err != nil || !ResolutionContextSessionComplete(resolutionCtx) {
		return &trustProofResult{
			decision:    TrustDecisionHasUnknown,
			reasonCodes: []domain.ReasonCode{domain.ReasonIdentityLookupFailed},
		}
	}

	identityReq := EffectIdentityRequest{
		Dialect:    req.Dialect,
		Candidates: extracted.EffectCandidates,
		Resolution: resolutionCtx,
	}

	// Atomic proof resolver is required for promotion. Non-atomic paths
	// allow column type and effect identity to come from different catalog
	// snapshots, which is unsafe under concurrent DDL.
	atomicResolver, ok := s.trusted.effectResolver.(AtomicProofResolver)
	if !ok {
		return &trustProofResult{
			decision:    TrustDecisionHasUnknown,
			reasonCodes: []domain.ReasonCode{domain.ReasonIdentityLookupFailed},
		}
	}

	_, batch, finalCtx, atomicErr := atomicResolver.ResolveColumnTypesAndEffectIdentities(
		ctx, extracted.EffectCandidates, identityReq)
	if atomicErr != nil {
		return &trustProofResult{
			decision:    TrustDecisionHasUnknown,
			reasonCodes: []domain.ReasonCode{domain.ReasonIdentityLookupFailed},
		}
	}

	// INV-3, INV-7: Validate initial/final context compatibility.
	if err := ValidateResolutionContextForPromotion(resolutionCtx, finalCtx, extracted.EffectCandidates); err != nil {
		return &trustProofResult{
			decision:    TrustDecisionHasUnknown,
			reasonCodes: []domain.ReasonCode{domain.ReasonIdentityLookupFailed},
		}
	}

	// INV-6: Validate raw batch ordinals BEFORE completion/normalization.
	if err := ValidateBatchOrdinals(batch, extracted.EffectCandidates); err != nil {
		return &trustProofResult{
			decision:    TrustDecisionHasUnknown,
			reasonCodes: []domain.ReasonCode{domain.ReasonIdentityLookupFailed},
		}
	}

	// INV-4, INV-5: Validate every resolved fact against final context.
	for i := range batch.Items {
		if batch.Items[i].Status == domain.IdentityStatusResolved && batch.Items[i].Facts != nil {
			if !ValidateFactPinning(batch.Items[i].Facts, finalCtx) {
				batch.Items[i].Status = domain.IdentityStatusUnavailable
				batch.Items[i].Facts = nil
			}
		}
	}

	// Complete batch to ensure one item per candidate ordinal.
	batch = CompleteEffectIdentityBatch(identityReq, batch)

	// Apply trust policy.
	serverVersionNum := extractServerVersionFromBatch(batch)
	decision := s.trusted.trustPolicy.IsTrusted(batch, serverVersionNum)

	reasonCodes := FailClosedReasonCodes(batch)

	return &trustProofResult{
		decision:    decision,
		reasonCodes: reasonCodes,
		batch:       batch,
	}
}

// extractServerVersionFromBatch finds the server version from resolved facts.
func extractServerVersionFromBatch(batch EffectIdentityBatch) int {
	for _, item := range batch.Items {
		if item.Status == domain.IdentityStatusResolved && item.Facts != nil && item.Facts.ServerVersionNum > 0 {
			return item.Facts.ServerVersionNum
		}
	}
	return 0
}

// removeUnprovenEffectReasons removes unproven effect reason codes when
// manifest proof succeeds. These codes are no longer needed because the
// effects are now proven.
func removeUnprovenEffectReasons(codes []domain.ReasonCode) []domain.ReasonCode {
	result := make([]domain.ReasonCode, 0, len(codes))
	for _, code := range codes {
		switch code {
		case domain.ReasonUnprovenOperatorEffect, domain.ReasonUnprovenFunctionEffect, domain.ReasonUnprovenCastEffect:
			// Skip unproven reasons when proof succeeds.
			continue
		default:
			result = append(result, code)
		}
	}
	return result
}

func extractByDialect(ctx context.Context, req QueryAccessRequest) (QueryAccessResult, error) {
	switch req.Dialect {
	case "mysql", "tidb":
		return ExtractTiDBQueryAccess(ctx, req)
	case "postgresql":
		return AnalyzePostgreSQL(ctx, req)
	default:
		return QueryAccessResult{}, fmt.Errorf("unsupported dialect: %q", req.Dialect)
	}
}

func hasFunctionCallReasonCode(codes []domain.ReasonCode) bool {
	for _, code := range codes {
		if code == "function_call" {
			return true
		}
	}
	return false
}

func recomputeAdmission(classification domain.ReadClassification, current domain.Admission, unresolved []domain.Unresolved, hasResolver bool) domain.Admission {
	if current == domain.Rejected {
		return current
	}
	if !hasResolver && current == domain.IndeterminateAdmission {
		return current
	}
	if len(unresolved) > 0 {
		return domain.IndeterminateAdmission
	}
	if classification == domain.NotReadOnly {
		return domain.Rejected
	}
	if classification == domain.ReadOnly {
		return domain.Admissible
	}
	return domain.IndeterminateAdmission
}

func reclassifyAfterResolution(classification domain.ReadClassification, reasonCodes []domain.ReasonCode, unresolved []domain.Unresolved, hasResolver bool, dialect string, proof *trustProofResult) domain.ReadClassification {
	if classification != domain.Indeterminate {
		return classification
	}

	if !hasResolver {
		return classification
	}

	// PostgreSQL: promote only if manifest proof is all_proven.
	if dialect == "postgresql" {
		if proof == nil {
			return domain.Indeterminate
		}
		if proof.decision != TrustDecisionAllProven {
			return domain.Indeterminate
		}
		// All effects manifest-proven; check other preconditions.
		// Fall through to common checks below.
	}

	if len(reasonCodes) > 0 {
		return domain.Indeterminate
	}

	hasWildcardUnresolved := false
	for _, u := range unresolved {
		if u.Reason == domain.ReasonSchemaUnavailable || u.Reason == ReasonUnresolvedWildcard {
			hasWildcardUnresolved = true
			break
		}
	}
	if hasWildcardUnresolved {
		return domain.Indeterminate
	}

	return domain.ReadOnly
}

func hasUnqualifiedRelation(relations []domain.RelationReference) bool {
	for _, rel := range relations {
		if rel.Schema == "" && rel.Kind != domain.RelationCTE && rel.Kind != domain.RelationDerived {
			return true
		}
	}
	return false
}

func hasUnqualifiedEffectCandidates(candidates []EffectCandidate) bool {
	for _, c := range candidates {
		if !CandidateExplicitlyQualified(c) {
			return true
		}
	}
	return false
}

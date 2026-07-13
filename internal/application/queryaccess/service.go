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

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// ErrExtractionFailed indicates query access extraction failed without exposing SQL text.
var ErrExtractionFailed = errors.New("query access extraction failed")

// ErrTrustedBundleInvalid indicates the trusted bundle failed validation.
var ErrTrustedBundleInvalid = errors.New("invalid trusted bundle")

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
type trustedBundle struct {
	effectResolver EffectIdentityResolver
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
// All dependencies must be non-nil and session-pinned for PostgreSQL promotion.
// The trust policy's manifest is validated on construction.
func NewTrustedService(effectResolver EffectIdentityResolver, trustPolicy *TrustPolicy, schemaResolver SchemaResolver) (*Service, error) {
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

	if schemaResolver != nil {
		extracted.DomainResult = resolveMetadata(ctx, schemaResolver, req.Dialect, req.DefaultSchema, extracted.DomainResult)
	}

	if hasFunctionCallReasonCode(extracted.DomainResult.ReasonCodes) {
		extracted.DomainResult.ReasonCodes = append(extracted.DomainResult.ReasonCodes, domain.ReasonFunctionEffect)
	}

	// PostgreSQL effect identity resolution and manifest proof.
	// When trusted bundle is present and dialect is PostgreSQL, resolve effect
	// identities and apply manifest-gated promotion.
	var proofResult *trustProofResult
	if req.Dialect == "postgresql" && s != nil && s.trusted != nil && len(extracted.EffectCandidates) > 0 {
		proofResult = s.resolveAndProveEffects(ctx, req, extracted)
		// When proof succeeds, remove unproven effect reason codes.
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
func (s *Service) resolveAndProveEffects(ctx context.Context, req QueryAccessRequest, extracted QueryAccessResult) *trustProofResult {
	if s == nil || s.trusted == nil {
		return nil
	}

	// Build resolution context from the trusted bundle's pinned session.
	// For Phase-1, we require the caller to have provided a pre-captured context.
	// The adapter will capture live context internally for TOCTOU protection.
	identityReq := EffectIdentityRequest{
		Dialect:    req.Dialect,
		Candidates: extracted.EffectCandidates,
		// OperandTypeOIDs are populated from resolved column type OIDs.
		OperandTypeOIDs: buildOperandTypeOIDs(extracted.EffectCandidates, extracted.DomainResult.ReferencedColumns, s.trusted.schemaResolver, req),
	}

	// Validate request structure.
	if err := ValidateEffectIdentityRequest(identityReq); err != nil {
		return &trustProofResult{
			decision:    TrustDecisionHasUnknown,
			reasonCodes: []domain.ReasonCode{domain.ReasonIdentityLookupFailed},
		}
	}

	batch, err := s.trusted.effectResolver.ResolveEffectIdentities(ctx, identityReq)
	if err != nil {
		return &trustProofResult{
			decision:    TrustDecisionHasUnknown,
			reasonCodes: []domain.ReasonCode{domain.ReasonIdentityLookupFailed},
		}
	}

	// Complete batch to ensure one item per candidate ordinal.
	batch = CompleteEffectIdentityBatch(identityReq, batch)

	// Apply trust policy.
	// For Phase-1, we use the server version from the batch's resolved facts.
	// The adapter stamps facts with the live session's server version.
	serverVersionNum := extractServerVersionFromBatch(batch)
	decision := s.trusted.trustPolicy.IsTrusted(batch, serverVersionNum)

	// Collect reason codes for non-resolved items.
	reasonCodes := FailClosedReasonCodes(batch)

	return &trustProofResult{
		decision:    decision,
		reasonCodes: reasonCodes,
		batch:       batch,
	}
}

// buildOperandTypeOIDs populates type OID hints from resolved column metadata.
// For Phase-1, we don't populate operand type OIDs from column metadata.
// The adapter resolves types from the catalog directly.
func buildOperandTypeOIDs(_ []EffectCandidate, _ []domain.ColumnReference, _ SchemaResolver, _ QueryAccessRequest) map[int][]uint32 {
	return nil
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

// Package queryaccess defines the effect-identity resolver contract (facts only).
// input: internal EffectCandidate batch keyed by stable ordinal
// output: per-ordinal IdentityStatus + optional catalog facts (never Trusted/admission)
// pos: T6 internal application contract for T7 catalog adapters and T8 proof engine
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"errors"
	"fmt"
	"sort"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

var (
	// ErrDuplicateIdentityOrdinal indicates the request repeated a candidate ordinal.
	ErrDuplicateIdentityOrdinal = errors.New("duplicate effect identity candidate ordinal")
	// ErrIdentityRequestInvalid indicates the request failed structural validation.
	ErrIdentityRequestInvalid = errors.New("invalid effect identity request")
	// ErrIdentityBatchIncomplete indicates results omitted required ordinals.
	ErrIdentityBatchIncomplete = errors.New("effect identity batch incomplete")
)

// EffectVolatility is a bounded PostgreSQL provolatile / operator volatility fact.
// Values are catalog facts only — never a trust claim.
type EffectVolatility string

const (
	// EffectVolatilityImmutable is provolatile 'i'.
	EffectVolatilityImmutable EffectVolatility = "i"
	// EffectVolatilityStable is provolatile 's'.
	EffectVolatilityStable EffectVolatility = "s"
	// EffectVolatilityVolatile is provolatile 'v'.
	EffectVolatilityVolatile EffectVolatility = "v"
)

// ValidEffectVolatility reports whether v is a known volatility fact.
func ValidEffectVolatility(v EffectVolatility) bool {
	switch v {
	case EffectVolatilityImmutable, EffectVolatilityStable, EffectVolatilityVolatile:
		return true
	default:
		return false
	}
}

// EffectCastMethod is a bounded PostgreSQL castmethod fact (f/b/i).
// Values are catalog facts only — never a trust claim.
type EffectCastMethod string

const (
	// EffectCastMethodFunction is castmethod 'f'.
	EffectCastMethodFunction EffectCastMethod = "f"
	// EffectCastMethodBinary is castmethod 'b'.
	EffectCastMethodBinary EffectCastMethod = "b"
	// EffectCastMethodInOut is castmethod 'i'.
	EffectCastMethodInOut EffectCastMethod = "i"
)

// ValidEffectCastMethod reports whether m is a known cast method fact.
func ValidEffectCastMethod(m EffectCastMethod) bool {
	switch m {
	case EffectCastMethodFunction, EffectCastMethodBinary, EffectCastMethodInOut:
		return true
	default:
		return false
	}
}

// EffectIdentityResolver resolves catalog identity facts for effect candidates.
//
// Contract (normative for T6+ adapters):
//
//   - Returns catalog FACTS only. Must never return Trusted, admission, reason
//     free-text, connection strings, catalog SQL, or driver error text as status.
//   - Trust is decided only by later application/domain manifest policy (T8).
//   - Batch-primary: ResolveEffectIdentities accepts all candidates at once.
//     Per-candidate implementations may wrap single lookups but must still emit
//     one result item per input ordinal (partial failure uses status, not omission).
//   - Ordinals in the request must be unique (stable 0-based traversal order from T5).
//   - Result ordering is ascending by Ordinal after NormalizeEffectIdentityBatch.
//   - Context cancellation: check ctx; if cancelled before/during work, return an
//     error wrapping context.Canceled (batch-level). Do not encode cancel as a
//     per-item status. Callers fail the analysis request as today.
//   - Catalog/transport errors for individual candidates map to
//     IdentityStatusLookupFailed (or whole-batch error if the adapter cannot start).
//   - unknown / ambiguous / coercion_gap / lookup_failed / unavailable are all
//     fail-closed for pure-read promotion.
//
// This interface is intentionally NOT attached to public SDK/CLI/HTTP request
// schemas in T6. Wiring into Service.Analyze and public injection points is a
// later task once a catalog adapter exists.
type EffectIdentityResolver interface {
	ResolveEffectIdentities(ctx context.Context, req EffectIdentityRequest) (EffectIdentityBatch, error)
}

// EffectIdentityRequest is a batch of internal untrusted candidates for fact resolution.
// Inputs are associated by candidate Ordinal from T5 extraction.
//
// Callers cannot inject domain.Result fields, Trusted bits, or public reason text
// through this request: only internal candidate facts and optional type OID hints.
type EffectIdentityRequest struct {
	// Dialect is the analysis dialect (identity resolution is PostgreSQL-only in phase 1).
	Dialect string
	// Candidates are internal untrusted effect facts (kind, name path, arity, flags).
	// They are resolver INPUTS, not trust roots.
	Candidates []EffectCandidate
	// OperandTypeOIDs optionally supplies pre-resolved operand/argument type OIDs
	// keyed by candidate ordinal. Missing keys mean types are unknown (may yield
	// unknown or coercion_gap). Values must not include literal SQL text.
	OperandTypeOIDs map[int][]uint32
}

// EffectIdentityFacts are catalog facts for one uniquely resolved effect.
// There is intentionally no Trusted, Admission, or free-text reason field.
//
// CanonicalSignature is an internal manifest-matching key for T8; it must never
// be copied into domain.Result, SDK/CLI/HTTP JSON, or reason codes.
type EffectIdentityFacts struct {
	Kind EffectCandidateKind

	// ObjectOID is the primary catalog OID (pg_operator.oid / pg_proc.oid / cast identity).
	ObjectOID uint32
	// NamespaceOID is the schema namespace OID (e.g. pg_namespace.oid for pg_catalog).
	NamespaceOID uint32

	// OperandTypeOIDs are left/right or argument type OIDs in catalog order.
	OperandTypeOIDs []uint32
	// ResultTypeOID is the result type OID when known (0 if not applicable/unknown).
	ResultTypeOID uint32

	// ImplementationOID is operator oprcode (function implementing the operator), when applicable.
	ImplementationOID uint32

	// Volatility is a bounded function/operator volatility fact when known.
	Volatility EffectVolatility
	// CastMethod is set for cast identities (f/b/i).
	CastMethod EffectCastMethod
	// CastFunctionOID is the cast function OID when CastMethod is function; 0 otherwise.
	CastFunctionOID uint32

	// CanonicalSignature is an internal, deterministic identity key for manifest
	// membership checks. Not a public field; not a trust claim by itself.
	CanonicalSignature string
}

// EffectIdentityItem is the resolution outcome for one candidate ordinal.
// Status is always a bounded IdentityStatus; free-text must not be stored here.
// Facts is non-nil only when Status == resolved (and even then is not trusted).
type EffectIdentityItem struct {
	Ordinal int
	Status  domain.IdentityStatus
	Facts   *EffectIdentityFacts
}

// EffectIdentityBatch is the ordered set of per-candidate outcomes.
type EffectIdentityBatch struct {
	Items []EffectIdentityItem
}

// ValidateEffectIdentityRequest checks ordinal uniqueness and structural bounds.
// Empty candidate slices are valid (resolver returns an empty batch).
func ValidateEffectIdentityRequest(req EffectIdentityRequest) error {
	seen := make(map[int]struct{}, len(req.Candidates))
	for _, c := range req.Candidates {
		if _, ok := seen[c.Ordinal]; ok {
			return fmt.Errorf("%w: %d", ErrDuplicateIdentityOrdinal, c.Ordinal)
		}
		seen[c.Ordinal] = struct{}{}
		if !validEffectCandidateKind(c.Kind) {
			return fmt.Errorf("%w: unknown candidate kind", ErrIdentityRequestInvalid)
		}
	}
	return nil
}

func validEffectCandidateKind(k EffectCandidateKind) bool {
	switch k {
	case EffectCandidateOperator, EffectCandidateFunction, EffectCandidateCast:
		return true
	default:
		return false
	}
}

// BuildUnavailableBatch returns one unavailable item per candidate ordinal.
// Used when no EffectIdentityResolver is configured (fail-closed, facts-only).
// Result is sorted by ordinal. Does not inspect or leak candidate names.
func BuildUnavailableBatch(candidates []EffectCandidate) EffectIdentityBatch {
	items := make([]EffectIdentityItem, 0, len(candidates))
	for _, c := range candidates {
		items = append(items, EffectIdentityItem{
			Ordinal: c.Ordinal,
			Status:  domain.IdentityStatusUnavailable,
			Facts:   nil,
		})
	}
	return NormalizeEffectIdentityBatch(items)
}

// MapCatalogErrorToStatus maps a transport/catalog error to a bounded status.
// The error text is discarded: it must never become status, reason, or facts.
// context.Canceled / DeadlineExceeded are not mapped here — callers return them
// as batch-level errors.
func MapCatalogErrorToStatus(err error) domain.IdentityStatus {
	if err == nil {
		return domain.IdentityStatusUnknown
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// Callers should treat cancel as batch-level error; status fallback is lookup_failed.
		return domain.IdentityStatusLookupFailed
	}
	return domain.IdentityStatusLookupFailed
}

// NormalizeEffectIdentityBatch sorts by ascending ordinal, drops facts on
// non-resolved statuses, and rewrites free-text/invalid statuses to lookup_failed
// (fail-closed). Duplicate ordinals keep the first item and discard later ones
// after a stable sort (callers should validate requests first).
//
// Partial failure: missing ordinals are not invented here; use
// CompleteEffectIdentityBatch against the request to fill gaps.
func NormalizeEffectIdentityBatch(items []EffectIdentityItem) EffectIdentityBatch {
	if len(items) == 0 {
		return EffectIdentityBatch{}
	}
	out := make([]EffectIdentityItem, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Ordinal < out[j].Ordinal
	})

	seen := make(map[int]struct{}, len(out))
	deduped := make([]EffectIdentityItem, 0, len(out))
	for _, it := range out {
		if _, ok := seen[it.Ordinal]; ok {
			continue
		}
		seen[it.Ordinal] = struct{}{}
		deduped = append(deduped, sanitizeIdentityItem(it))
	}
	return EffectIdentityBatch{Items: deduped}
}

func sanitizeIdentityItem(it EffectIdentityItem) EffectIdentityItem {
	if !domain.ValidIdentityStatus(it.Status) {
		// Free-text / arbitrary error strings cannot become status.
		it.Status = domain.IdentityStatusLookupFailed
		it.Facts = nil
		return it
	}
	if it.Status != domain.IdentityStatusResolved {
		it.Facts = nil
		return it
	}
	// Resolved: facts may be present but must not carry invalid bounded enums.
	if it.Facts != nil {
		facts := *it.Facts
		if facts.Volatility != "" && !ValidEffectVolatility(facts.Volatility) {
			// Invalid volatility fact → fail closed rather than promote bad data.
			return EffectIdentityItem{
				Ordinal: it.Ordinal,
				Status:  domain.IdentityStatusLookupFailed,
				Facts:   nil,
			}
		}
		if facts.CastMethod != "" && !ValidEffectCastMethod(facts.CastMethod) {
			return EffectIdentityItem{
				Ordinal: it.Ordinal,
				Status:  domain.IdentityStatusLookupFailed,
				Facts:   nil,
			}
		}
		// Defensive copy of type OID slices.
		if len(facts.OperandTypeOIDs) > 0 {
			facts.OperandTypeOIDs = append([]uint32(nil), facts.OperandTypeOIDs...)
		}
		it.Facts = &facts
	}
	return it
}

// CompleteEffectIdentityBatch ensures one item per request candidate ordinal.
// Missing ordinals become unavailable (fail-closed). Extra ordinals not in the
// request are dropped. Output is normalized (sorted, sanitized).
func CompleteEffectIdentityBatch(req EffectIdentityRequest, batch EffectIdentityBatch) EffectIdentityBatch {
	byOrd := make(map[int]EffectIdentityItem, len(batch.Items))
	for _, it := range batch.Items {
		if _, exists := byOrd[it.Ordinal]; exists {
			continue
		}
		byOrd[it.Ordinal] = it
	}
	items := make([]EffectIdentityItem, 0, len(req.Candidates))
	for _, c := range req.Candidates {
		if it, ok := byOrd[c.Ordinal]; ok {
			items = append(items, it)
			continue
		}
		items = append(items, EffectIdentityItem{
			Ordinal: c.Ordinal,
			Status:  domain.IdentityStatusUnavailable,
			Facts:   nil,
		})
	}
	return NormalizeEffectIdentityBatch(items)
}

// BatchIsFullyResolved reports whether every item is resolved with facts.
// Does not imply trust or admission; T8 policy must still gate promotion.
func BatchIsFullyResolved(batch EffectIdentityBatch) bool {
	if len(batch.Items) == 0 {
		return true
	}
	for _, it := range batch.Items {
		if it.Status != domain.IdentityStatusResolved || it.Facts == nil {
			return false
		}
	}
	return true
}

// FailClosedReasonCodes collects bounded identity reason codes for non-resolved
// items. Free-text statuses are mapped via Normalize first by callers.
// Underlying errors must not be passed in; only IdentityStatus is accepted.
func FailClosedReasonCodes(batch EffectIdentityBatch) []domain.ReasonCode {
	var codes []domain.ReasonCode
	for _, it := range batch.Items {
		if !domain.IdentityStatusIsFailClosed(it.Status) {
			continue
		}
		code, ok := domain.ReasonForIdentityStatus(it.Status)
		if !ok {
			// Invalid status → lookup_failed reason, never free text.
			code = domain.ReasonIdentityLookupFailed
		}
		codes = append(codes, code)
	}
	return domain.NormalizeReasonCodes(codes)
}

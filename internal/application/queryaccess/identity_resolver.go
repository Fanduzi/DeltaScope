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
//   - Execution resolution context (EffectIdentityRequest.Resolution) is required
//     for any Phase-1 promotion-ready identity. Adapters MUST NOT guess
//     pg_catalog.<name> from spelling alone (T2 forbids name/schema allowlists).
//     Call GateIdentityBatchAgainstLiveContext (or Gate + live check) after
//     lookup so incomplete/mismatched contexts discard facts.
//   - Phase-1 bound context REQUIRES non-zero SessionBinding, PathEpoch,
//     DatabaseOID, RoleOID, and ServerVersionNum. OIDs are database-local;
//     cross-database or cross-major resolved facts must never feed T8 promotion.
//   - Explicit schema skips search_path ranking only — it still requires the
//     same session/database/role/server binding as unqualified resolution.
//   - TOCTOU: T7 must read live context, run identity lookup, and re-check live
//     context on the SAME controlled session before handing facts to T8.
//     Session/db/role/version/epoch mismatch fails closed for ALL candidates;
//     search_path-only mismatch fails closed for unqualified only.
//
// This interface is intentionally NOT attached to public SDK/CLI/HTTP request
// schemas in T6. Wiring into Service.Analyze and public injection points is a
// later task once a catalog adapter exists.
type EffectIdentityResolver interface {
	ResolveEffectIdentities(ctx context.Context, req EffectIdentityRequest) (EffectIdentityBatch, error)
}

// ControlledEffectIdentityResolver is the narrow contract for promotion-ready
// identity resolution. Only implementations that guarantee pinned-session,
// execution-bound resolution and TOCTOU gating satisfy this contract.
//
// Generic EffectIdentityResolver implementations MUST NOT satisfy this interface.
// The application uses CaptureExecutionBoundContext to set explicit Resolution
// on the request, proving session binding before the resolver runs.
type ControlledEffectIdentityResolver interface {
	EffectIdentityResolver
	// CaptureExecutionBoundContext returns the current session's resolution
	// context. Must return a session-complete context (all fields non-zero)
	// or an error. The application uses this to set explicit Resolution on
	// EffectIdentityRequest, proving the facts are bound to the expected
	// execution session.
	CaptureExecutionBoundContext(ctx context.Context) (EffectIdentityResolutionContext, error)
}

// ColumnTypeOIDResolver resolves column type OIDs for operand provenance
// on a pinned session. This is a narrow capability interface satisfied by
// the T7 EffectIdentityAdapter. The application uses a type assertion to
// access this when the controlled resolver also supports column type lookup.
//
// Only binary operators with two fully-qualified column operands from base
// tables are resolved. Missing or unresolvable columns are skipped (fail-closed
// for the affected candidate; other candidates may still succeed).
type ColumnTypeOIDResolver interface {
	ResolveColumnTypeOIDs(ctx context.Context, candidates []EffectCandidate) (map[int][]uint32, error)
}

// AtomicProofResolver combines column type OID resolution and effect identity
// resolution in a single atomic operation. This ensures both come from the
// same catalog snapshot (REPEATABLE READ), preventing TOCTOU issues with
// concurrent DDL.
//
// Only implementations that guarantee pinned-session, execution-bound
// resolution satisfy this contract. The application uses a type assertion
// to prefer this over separate ResolveColumnTypeOIDs + ResolveEffectIdentities
// calls when available.
//
// The returned EffectIdentityResolutionContext is captured INSIDE the atomic
// operation (same pinned session/transaction) and represents the final
// execution-bound state after all lookups. The application compares this
// with the initial captured context to detect TOCTOU drift.
//
// INV-12 (Defense-in-Depth): These checks protect against malformed, cross-wired,
// buggy, or contract-violating trusted-adapter output. They do NOT protect against
// a compromised in-process dependency: a malicious resolver that controls
// CaptureExecutionBoundContext, atomic resolution, facts, and type output can
// fabricate a mutually consistent proof. NewTrustedService accepts an in-process
// dependency that is necessarily trusted by construction.
type AtomicProofResolver interface {
	ResolveColumnTypesAndEffectIdentities(
		ctx context.Context,
		candidates []EffectCandidate,
		req EffectIdentityRequest,
	) (map[int][]uint32, EffectIdentityBatch, EffectIdentityResolutionContext, error)
}

// EffectIdentityResolutionContext is an internal, execution-bound name-resolution
// environment for effect identity lookup.
//
// It must never appear on domain.Result, SDK/CLI/HTTP JSON, reason codes, or
// public error text (no DSN, password, search_path string dump, or SQL).
//
// Phase-1 policy (normative, T2-aligned):
//
//  1. A promotion-ready bound context (ResolutionContextSessionComplete) requires
//     ALL of: Bound, non-empty SessionBinding, PathEpoch != 0, DatabaseOID != 0,
//     RoleOID != 0, ServerVersionNum != 0. Zero fields are incomplete — never
//     optional for Phase-1 promotion.
//  2. Unqualified operators/functions also require non-empty NamespaceSearchOIDs
//     (ResolutionContextUsableForUnqualified). Without that proof they MUST be
//     IdentityStatusUnavailable. Adapters must not invent pg_catalog.* names.
//  3. Explicitly schema-qualified candidates skip search_path ranking for the
//     namespace segment, but still require ResolutionContextSessionComplete.
//     They share the same database/server catalog as the session; OIDs are
//     database-local. Multi-match remains ambiguous.
//  4. Bound=true attests the caller controls the session used for both analysis
//     resolution and execution. PathEpoch must bump on search_path, role,
//     database, or server change.
//  5. TOCTOU: re-read live context on the same session after lookup; any session
//     field mismatch fails closed for every candidate (including explicit schema).
//     Search_path order mismatch alone fails closed for unqualified only.
type EffectIdentityResolutionContext struct {
	// Bound is true only when this context is proven to match the intended
	// execution environment. Never set Bound from untrusted public JSON.
	// Bound=true requires SessionComplete fields (see ValidateEffectIdentityRequest).
	Bound bool

	// SessionBinding is an opaque internal id for the controlled session or
	// frozen catalog snapshot. Required non-empty when Bound. Never a DSN or password.
	SessionBinding string

	// PathEpoch is a non-zero generation counter for the locked session snapshot
	// (search_path / role / database identity). Required when Bound. Live mismatch
	// invalidates the context.
	PathEpoch uint64

	// NamespaceSearchOIDs is the ordered schema OID list used for unqualified
	// resolution (PostgreSQL search_path after expansion). Required non-empty for
	// unqualified resolution; may be empty only when every candidate is explicit
	// schema (session fields still required).
	NamespaceSearchOIDs []uint32

	// DatabaseOID is the current database OID. Required non-zero when Bound.
	// Object OIDs are local to this database.
	DatabaseOID uint32

	// RoleOID is the session role OID used for name resolution. Required non-zero when Bound.
	RoleOID uint32

	// ServerVersionNum is PostgreSQL server_version_num. Required non-zero when Bound.
	// Version-scoped manifests (T8) must not accept facts from a different major.
	ServerVersionNum int
}

// EffectIdentityResolutionMode classifies how a candidate may use the context.
// Bounded machine identifiers only — not public Result fields.
type EffectIdentityResolutionMode string

const (
	// ResolutionModeExplicitSchema: NamePath/TargetTypePath is schema-qualified.
	ResolutionModeExplicitSchema EffectIdentityResolutionMode = "explicit_schema"
	// ResolutionModeUnqualifiedBound: unqualified with usable execution context.
	ResolutionModeUnqualifiedBound EffectIdentityResolutionMode = "unqualified_bound"
	// ResolutionModeUnqualifiedUnbound: unqualified without proven execution context.
	// Must fail closed as unavailable (no pg_catalog name guess).
	ResolutionModeUnqualifiedUnbound EffectIdentityResolutionMode = "unqualified_unbound"
)

// PgCatalogNamespaceName is the PostgreSQL system catalog schema name.
// Used only for internal explicit-schema checks; not a trust claim.
const PgCatalogNamespaceName = "pg_catalog"

// EffectIdentityRequest is a batch of internal untrusted candidates for fact resolution.
// Inputs are associated by candidate Ordinal from T5 extraction.
//
// Callers cannot inject domain.Result fields, Trusted bits, or public reason text
// through this request: only internal candidate facts, optional type OID hints,
// and an optional execution-bound resolution context (internal only).
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
	// Resolution is the execution-bound resolution environment (internal only).
	// Zero value means unbound: unqualified candidates must stay unavailable.
	Resolution EffectIdentityResolutionContext
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

	// Structured identity fields for candidate-to-fact binding validation.
	// These fields are populated by the catalog adapter and used by
	// ValidateCandidateFactBinding to prevent fact swaps between candidates.
	// They must never be copied into domain.Result, SDK/CLI/HTTP JSON, or reason codes.

	// ResolvedSchemaName is the resolved schema name (e.g., "pg_catalog").
	// Empty for unqualified resolution.
	ResolvedSchemaName string
	// ResolvedObjectName is the resolved object name (e.g., "=", "count").
	ResolvedObjectName string
	// CastSourceTypeName is the cast source type name (e.g., "int4").
	// Only populated for cast identities.
	CastSourceTypeName string
	// CastTargetTypeName is the cast target type name (e.g., "text").
	// Only populated for cast identities.
	CastTargetTypeName string

	// DatabaseOID pins ObjectOID locality (must match the resolution context).
	// Zero is incomplete; gates discard such resolved facts.
	DatabaseOID uint32
	// ServerVersionNum pins the server major/minor used for catalog lookup.
	// Zero is incomplete; gates discard such resolved facts.
	ServerVersionNum int
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
// Resolution may be zero (unbound); that is valid and forces all candidates to
// unavailable via GateIdentityBatchByResolutionContext (no promotion-ready facts).
// Bound=true requires a fully complete session context (binding, epoch, database,
// role, server version); partial Bound contexts are invalid, not "optional fields".
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
	if req.Resolution.Bound && !ResolutionContextSessionComplete(req.Resolution) {
		return fmt.Errorf("%w: bound resolution context requires SessionBinding, PathEpoch, DatabaseOID, RoleOID, and ServerVersionNum", ErrIdentityRequestInvalid)
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

// CandidateExplicitlyQualified reports whether the candidate names an explicit
// schema (NamePath/TargetTypePath multi-segment or ExplicitSchema flag).
// Explicit qualification does not imply trust — only that search_path ranking
// is not required to pick the namespace segment.
func CandidateExplicitlyQualified(c EffectCandidate) bool {
	if c.ExplicitSchema {
		return true
	}
	if len(c.NamePath) > 1 {
		return true
	}
	if c.Kind == EffectCandidateCast && len(c.TargetTypePath) > 1 {
		return true
	}
	return false
}

// CandidateExplicitSchemaName returns the leading schema segment when the
// candidate is explicitly qualified; otherwise "".
func CandidateExplicitSchemaName(c EffectCandidate) string {
	if !CandidateExplicitlyQualified(c) {
		return ""
	}
	if c.Kind == EffectCandidateCast {
		if len(c.TargetTypePath) > 1 {
			return c.TargetTypePath[0]
		}
		return ""
	}
	if len(c.NamePath) > 1 {
		return c.NamePath[0]
	}
	return ""
}

// CandidateExplicitPgCatalog reports explicit schema qualification under pg_catalog.
// This is a structural fact, not a trust claim and not a substitute for OID proof.
func CandidateExplicitPgCatalog(c EffectCandidate) bool {
	return CandidateExplicitSchemaName(c) == PgCatalogNamespaceName
}

// ResolutionContextSessionComplete reports whether the context is fully bound
// for Phase-1 promotion: Bound plus non-zero SessionBinding, PathEpoch,
// DatabaseOID, RoleOID, and ServerVersionNum. Missing any field is incomplete
// (not "optional"). Search_path may still be empty (explicit-schema-only batches).
func ResolutionContextSessionComplete(rc EffectIdentityResolutionContext) bool {
	if !rc.Bound {
		return false
	}
	if rc.SessionBinding == "" {
		return false
	}
	if rc.PathEpoch == 0 {
		return false
	}
	if rc.DatabaseOID == 0 {
		return false
	}
	if rc.RoleOID == 0 {
		return false
	}
	if rc.ServerVersionNum == 0 {
		return false
	}
	return true
}

// ResolutionContextUsableForUnqualified reports whether the context may resolve
// unqualified operators/functions: session-complete plus non-empty search path.
func ResolutionContextUsableForUnqualified(rc EffectIdentityResolutionContext) bool {
	if !ResolutionContextSessionComplete(rc) {
		return false
	}
	return len(rc.NamespaceSearchOIDs) > 0
}

// ResolutionContextSessionCompatible reports whether two contexts share the same
// session/database/role/server/epoch binding. Zeros never match (incomplete).
// Search_path is intentionally not compared here — explicit schema may skip it.
func ResolutionContextSessionCompatible(a, b EffectIdentityResolutionContext) bool {
	if !ResolutionContextSessionComplete(a) || !ResolutionContextSessionComplete(b) {
		return false
	}
	if a.SessionBinding != b.SessionBinding {
		return false
	}
	if a.PathEpoch != b.PathEpoch {
		return false
	}
	if a.DatabaseOID != b.DatabaseOID {
		return false
	}
	if a.RoleOID != b.RoleOID {
		return false
	}
	if a.ServerVersionNum != b.ServerVersionNum {
		return false
	}
	return true
}

// ResolutionContextSearchPathCompatible reports equal ordered NamespaceSearchOIDs.
// Empty paths are compatible with each other only when both are empty (explicit-only).
func ResolutionContextSearchPathCompatible(a, b EffectIdentityResolutionContext) bool {
	if len(a.NamespaceSearchOIDs) != len(b.NamespaceSearchOIDs) {
		return false
	}
	for i := range a.NamespaceSearchOIDs {
		if a.NamespaceSearchOIDs[i] != b.NamespaceSearchOIDs[i] {
			return false
		}
	}
	return true
}

// ResolutionContextsCompatible reports full compatibility for unqualified
// resolution: session binding + search_path order. Incomplete contexts never match.
func ResolutionContextsCompatible(a, b EffectIdentityResolutionContext) bool {
	if !ResolutionContextUsableForUnqualified(a) || !ResolutionContextUsableForUnqualified(b) {
		return false
	}
	if !ResolutionContextSessionCompatible(a, b) {
		return false
	}
	return ResolutionContextSearchPathCompatible(a, b)
}

// ValidateResolutionContextForPromotion validates initial and final execution
// contexts for the proof gateway. Encapsulates INV-3 and INV-7 checks.
//
// Returns nil if validation passes, or a bounded error if:
// - Initial context is not session-complete
// - Final context is not session-complete
// - Initial/final contexts are not session-compatible
// - Unqualified candidates exist and search-path is not compatible
func ValidateResolutionContextForPromotion(
	initialCtx, finalCtx EffectIdentityResolutionContext,
	candidates []EffectCandidate,
) error {
	// INV-3: Initial context must be session-complete.
	if !ResolutionContextSessionComplete(initialCtx) {
		return fmt.Errorf("%w: initial context incomplete", ErrIdentityRequestInvalid)
	}
	// Final context must be session-complete.
	if !ResolutionContextSessionComplete(finalCtx) {
		return fmt.Errorf("%w: final context incomplete", ErrIdentityRequestInvalid)
	}
	// INV-3: Initial/final contexts must be session-compatible.
	if !ResolutionContextSessionCompatible(initialCtx, finalCtx) {
		return fmt.Errorf("%w: context session mismatch", ErrIdentityRequestInvalid)
	}
	// INV-7: If unqualified candidates exist, search-path must be compatible.
	if hasUnqualifiedEffectCandidates(candidates) {
		if !ResolutionContextSearchPathCompatible(initialCtx, finalCtx) {
			return fmt.Errorf("%w: search-path drift with unqualified candidates", ErrIdentityRequestInvalid)
		}
	}
	return nil
}

// StampFactsFromResolution copies database/server locality pins from the
// resolution context onto facts. Adapters should call this for every resolved
// item before gating. Does not set Trusted or admission.
func StampFactsFromResolution(facts *EffectIdentityFacts, rc EffectIdentityResolutionContext) {
	if facts == nil {
		return
	}
	facts.DatabaseOID = rc.DatabaseOID
	facts.ServerVersionNum = rc.ServerVersionNum
}

// factsMatchResolution reports whether resolved facts are pinned to the request
// database/server. Zero pins or mismatches are fail-closed.
func factsMatchResolution(facts *EffectIdentityFacts, rc EffectIdentityResolutionContext) bool {
	if facts == nil {
		return false
	}
	if !ResolutionContextSessionComplete(rc) {
		return false
	}
	if facts.DatabaseOID == 0 || facts.ServerVersionNum == 0 {
		return false
	}
	if facts.DatabaseOID != rc.DatabaseOID {
		return false
	}
	if facts.ServerVersionNum != rc.ServerVersionNum {
		return false
	}
	return true
}

// ValidateFactPinning validates that resolved facts are pinned to the final
// execution context. Encapsulates INV-4 and INV-5 checks.
//
// Returns true if facts are valid (pinned to final context), false otherwise.
// Invalid facts should be converted to unavailable before IsTrusted.
func ValidateFactPinning(facts *EffectIdentityFacts, finalCtx EffectIdentityResolutionContext) bool {
	return factsMatchResolution(facts, finalCtx)
}

// ClassifyCandidateResolutionMode returns the bounded resolution mode for a
// candidate under the given execution context.
// Explicit schema still requires a session-complete context to keep resolved
// facts (mode is still "explicit_schema" for path ranking, but gates enforce
// session completeness separately).
func ClassifyCandidateResolutionMode(c EffectCandidate, rc EffectIdentityResolutionContext) EffectIdentityResolutionMode {
	if CandidateExplicitlyQualified(c) {
		return ResolutionModeExplicitSchema
	}
	if ResolutionContextUsableForUnqualified(rc) {
		return ResolutionModeUnqualifiedBound
	}
	return ResolutionModeUnqualifiedUnbound
}

// GateIdentityBatchByResolutionContext enforces Phase-1 resolution policy:
//
//   - Without a session-complete context, ALL candidates become unavailable
//     (including explicit schema): OIDs are database-local and cannot be proven.
//   - Unqualified candidates also require a non-empty search path; otherwise
//     unavailable (no pg_catalog name guess).
//   - Resolved facts must carry DatabaseOID/ServerVersionNum matching the
//     request context; otherwise facts are discarded as unavailable.
//
// Output is completed against the request and normalized. Context is never
// copied into items or public Result fields.
func GateIdentityBatchByResolutionContext(req EffectIdentityRequest, batch EffectIdentityBatch) EffectIdentityBatch {
	byOrd := make(map[int]EffectIdentityItem, len(batch.Items))
	for _, it := range batch.Items {
		if _, exists := byOrd[it.Ordinal]; exists {
			continue
		}
		byOrd[it.Ordinal] = it
	}
	sessionOK := ResolutionContextSessionComplete(req.Resolution)
	items := make([]EffectIdentityItem, 0, len(req.Candidates))
	for _, c := range req.Candidates {
		it, ok := byOrd[c.Ordinal]
		if !ok {
			items = append(items, EffectIdentityItem{
				Ordinal: c.Ordinal,
				Status:  domain.IdentityStatusUnavailable,
			})
			continue
		}
		it.Ordinal = c.Ordinal

		if !sessionOK {
			// Incomplete binding: cannot prove catalog locality for any candidate.
			it.Status = domain.IdentityStatusUnavailable
			it.Facts = nil
			items = append(items, it)
			continue
		}

		if !CandidateExplicitlyQualified(c) && !ResolutionContextUsableForUnqualified(req.Resolution) {
			it.Status = domain.IdentityStatusUnavailable
			it.Facts = nil
			items = append(items, it)
			continue
		}

		if it.Status == domain.IdentityStatusResolved {
			if !factsMatchResolution(it.Facts, req.Resolution) {
				it.Status = domain.IdentityStatusUnavailable
				it.Facts = nil
			}
		}
		items = append(items, it)
	}
	return NormalizeEffectIdentityBatch(items)
}

// LiveResolutionContext is an adapter callback that re-reads the session's
// current resolution snapshot on the same controlled connection used for lookup.
// T7 must supply this (or equivalent) before T8 promotion.
type LiveResolutionContext func() (EffectIdentityResolutionContext, error)

// GateIdentityBatchAgainstLiveContext applies policy gating and TOCTOU protection.
//
// When live is non-nil:
//   - live error or incomplete live snapshot → all candidates unavailable
//   - session/database/role/server/epoch mismatch → all candidates unavailable
//     (explicit schema does NOT skip these checks)
//   - search_path order mismatch only → unqualified unavailable; explicit schema
//     may keep facts if session-compatible and facts still match request pins
//
// When live is nil, only GateIdentityBatchByResolutionContext runs (T7 must not
// skip live re-check for promotion-ready paths).
func GateIdentityBatchAgainstLiveContext(req EffectIdentityRequest, batch EffectIdentityBatch, live LiveResolutionContext) EffectIdentityBatch {
	gated := GateIdentityBatchByResolutionContext(req, batch)
	if live == nil {
		return gated
	}
	if !ResolutionContextSessionComplete(req.Resolution) {
		return stripAllIdentityFacts(gated)
	}
	liveRC, err := live()
	if err != nil || !ResolutionContextSessionComplete(liveRC) {
		return stripAllIdentityFacts(gated)
	}
	if !ResolutionContextSessionCompatible(req.Resolution, liveRC) {
		// Role/database/server/epoch/session drift: strip everyone, including explicit.
		return stripAllIdentityFacts(gated)
	}
	if ResolutionContextSearchPathCompatible(req.Resolution, liveRC) {
		return gated
	}
	// Path-only drift: unqualified cannot be trusted; explicit schema may remain
	// if facts still pin to the request database/server.
	return stripUnqualifiedIdentityFacts(req, gated)
}

func stripAllIdentityFacts(batch EffectIdentityBatch) EffectIdentityBatch {
	items := make([]EffectIdentityItem, 0, len(batch.Items))
	for _, it := range batch.Items {
		items = append(items, EffectIdentityItem{
			Ordinal: it.Ordinal,
			Status:  domain.IdentityStatusUnavailable,
			Facts:   nil,
		})
	}
	return NormalizeEffectIdentityBatch(items)
}

func stripUnqualifiedIdentityFacts(req EffectIdentityRequest, batch EffectIdentityBatch) EffectIdentityBatch {
	candByOrd := make(map[int]EffectCandidate, len(req.Candidates))
	for _, c := range req.Candidates {
		candByOrd[c.Ordinal] = c
	}
	items := make([]EffectIdentityItem, 0, len(batch.Items))
	for _, it := range batch.Items {
		c, ok := candByOrd[it.Ordinal]
		if ok && !CandidateExplicitlyQualified(c) {
			it.Status = domain.IdentityStatusUnavailable
			it.Facts = nil
		}
		items = append(items, it)
	}
	return NormalizeEffectIdentityBatch(items)
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

// ValidateBatchOrdinals validates raw batch ordinals before completion/normalization.
// Encapsulates INV-6 checks.
//
// Returns nil if validation passes, or a bounded error if:
// - Batch has duplicate ordinals
// - Batch has ordinals not in the request
// - Request has ordinals not in the batch (missing)
func ValidateBatchOrdinals(batch EffectIdentityBatch, candidates []EffectCandidate) error {
	// Build set of request ordinals.
	reqOrds := make(map[int]struct{}, len(candidates))
	for _, c := range candidates {
		reqOrds[c.Ordinal] = struct{}{}
	}
	// Check for duplicates and out-of-range ordinals.
	seen := make(map[int]struct{}, len(batch.Items))
	for _, it := range batch.Items {
		if _, exists := seen[it.Ordinal]; exists {
			return fmt.Errorf("%w: duplicate ordinal %d", ErrDuplicateIdentityOrdinal, it.Ordinal)
		}
		seen[it.Ordinal] = struct{}{}
		if _, inRequest := reqOrds[it.Ordinal]; !inRequest {
			return fmt.Errorf("%w: out-of-range ordinal %d", ErrIdentityRequestInvalid, it.Ordinal)
		}
	}
	// Check for missing ordinals.
	for _, c := range candidates {
		if _, exists := seen[c.Ordinal]; !exists {
			return fmt.Errorf("%w: missing ordinal %d", ErrIdentityBatchIncomplete, c.Ordinal)
		}
	}
	return nil
}

// ValidateCandidateFactBinding validates that resolved facts correspond to the
// correct candidate shape. This prevents a resolver from returning manifest-valid
// facts for the wrong same-kind candidate.
//
// Validates:
//   - Fact kind matches candidate kind
//   - ObjectOID is nonzero
//   - For operators: OperandTypeOIDs has 1 or 2 entries (unary/binary)
//   - For functions: OperandTypeOIDs length matches candidate arity
//   - For casts: OperandTypeOIDs has exactly 1 entry
//
// On mismatch: converts item status to lookup_failed and removes facts.
func ValidateCandidateFactBinding(batch EffectIdentityBatch, candidates []EffectCandidate) EffectIdentityBatch {
	candByOrd := make(map[int]EffectCandidate, len(candidates))
	for _, c := range candidates {
		candByOrd[c.Ordinal] = c
	}
	items := make([]EffectIdentityItem, 0, len(batch.Items))
	for _, it := range batch.Items {
		if it.Status != domain.IdentityStatusResolved || it.Facts == nil {
			items = append(items, it)
			continue
		}
		c, ok := candByOrd[it.Ordinal]
		if !ok {
			it.Status = domain.IdentityStatusLookupFailed
			it.Facts = nil
			items = append(items, it)
			continue
		}
		if !factMatchesCandidate(it.Facts, c) {
			it.Status = domain.IdentityStatusLookupFailed
			it.Facts = nil
		}
		items = append(items, it)
	}
	return NormalizeEffectIdentityBatch(items)
}

func factMatchesCandidate(facts *EffectIdentityFacts, c EffectCandidate) bool {
	if facts == nil {
		return false
	}
	if string(facts.Kind) != string(c.Kind) {
		return false
	}
	if facts.ObjectOID == 0 {
		return false
	}

	// Validate structured identity fields (defense-in-depth against fact swaps).
	// These checks prevent a resolver from returning manifest-valid facts for the
	// wrong candidate with matching kind+arity.
	if facts.ResolvedObjectName != "" {
		candName := candidateCanonicalName(c)
		if candName != "" && facts.ResolvedObjectName != candName {
			return false
		}
	}

	// Validate explicit-schema intent matches.
	if c.ExplicitSchema && facts.ResolvedSchemaName != "" {
		candSchema := CandidateExplicitSchemaName(c)
		if candSchema != "" && facts.ResolvedSchemaName != candSchema {
			return false
		}
	}

	// Validate cast target type identity.
	if c.Kind == EffectCandidateCast && facts.CastTargetTypeName != "" {
		if len(c.TargetTypePath) > 0 {
			candTarget := c.TargetTypePath[len(c.TargetTypePath)-1]
			if candTarget != "" && facts.CastTargetTypeName != candTarget {
				return false
			}
		}
	}

	// Validate arity/operand count.
	switch c.Kind {
	case EffectCandidateOperator:
		n := len(facts.OperandTypeOIDs)
		if n < 1 || n > 2 {
			return false
		}
	case EffectCandidateFunction:
		if len(facts.OperandTypeOIDs) != c.Arity {
			return false
		}
	case EffectCandidateCast:
		if len(facts.OperandTypeOIDs) != 1 {
			return false
		}
	}
	return true
}

// candidateCanonicalName returns the canonical name for a candidate.
// For operators/functions: the last element of NamePath.
// For casts: the target type name.
func candidateCanonicalName(c EffectCandidate) string {
	switch c.Kind {
	case EffectCandidateCast:
		if len(c.TargetTypePath) > 0 {
			return c.TargetTypePath[len(c.TargetTypePath)-1]
		}
		return ""
	default:
		if len(c.NamePath) > 0 {
			return c.NamePath[len(c.NamePath)-1]
		}
		return ""
	}
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

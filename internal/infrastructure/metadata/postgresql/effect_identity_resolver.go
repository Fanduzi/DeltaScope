//go:build postgresql

// Package postgresqlmeta implements facts-only EffectIdentityResolver against pg_catalog.
// input: session-pinned PinnedSession + EffectIdentityRequest (candidates + type OIDs)
// output: EffectIdentityBatch statuses/facts only (no Trusted / admission / free-text)
// pos: T7 catalog adapter; not wired into Service.Analyze
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// effectIdentityCatalog is the session-pinned catalog surface used by the adapter.
// Production uses *PinnedSession; unit tests inject a fake with the same contract.
type effectIdentityCatalog interface {
	CaptureLiveContext(ctx context.Context) (appqa.EffectIdentityResolutionContext, error)
	namespaceOIDByName(ctx context.Context, name string) (uint32, error)
	typeOIDByName(ctx context.Context, schema, typname string, path []uint32) (uint32, error)
	lookupOperators(ctx context.Context, nsOID uint32, name string, left, right uint32) ([]operatorRow, error)
	lookupFunctions(ctx context.Context, nsOID uint32, name string, argOIDs []uint32) ([]functionRow, error)
	lookupCasts(ctx context.Context, sourceOID, targetOID uint32) ([]castRow, error)
}

// EffectIdentityAdapter resolves operator/function/cast catalog facts on one
// pinned PostgreSQL session. It does not apply manifest trust or admission.
//
// Contract:
//   - Production constructors accept only *PinnedSession (single *sql.Conn),
//     never a pool-style *sql.DB used directly for multi-connection lookups.
//   - ResolveEffectIdentities: capture live context → lookup → re-capture live →
//     GateIdentityBatchAgainstLiveContext.
//   - No cross-session fact cache.
type EffectIdentityAdapter struct {
	catalog effectIdentityCatalog
}

// NewEffectIdentityAdapter builds a facts-only resolver on a pinned session.
func NewEffectIdentityAdapter(session *PinnedSession) (*EffectIdentityAdapter, error) {
	if session == nil || session.Conn() == nil {
		return nil, ErrSessionNotPinned
	}
	return &EffectIdentityAdapter{catalog: session}, nil
}

// NewEffectIdentityAdapterFromDB is intentionally omitted: callers must PinSession
// (or NewPinnedSessionFromConn) so OID facts stay on one controlled backend.

var _ appqa.EffectIdentityResolver = (*EffectIdentityAdapter)(nil)
var _ appqa.ControlledEffectIdentityResolver = (*EffectIdentityAdapter)(nil)
var _ effectIdentityCatalog = (*PinnedSession)(nil)

// CaptureExecutionBoundContext implements appqa.ControlledEffectIdentityResolver.
// Returns the pinned session's current live resolution context. The application
// uses this to set explicit Resolution on the request, proving session binding.
func (a *EffectIdentityAdapter) CaptureExecutionBoundContext(ctx context.Context) (appqa.EffectIdentityResolutionContext, error) {
	if a == nil || a.catalog == nil {
		return appqa.EffectIdentityResolutionContext{}, ErrSessionNotPinned
	}
	return a.catalog.CaptureLiveContext(ctx)
}

// ResolveEffectIdentities implements appqa.EffectIdentityResolver.
func (a *EffectIdentityAdapter) ResolveEffectIdentities(ctx context.Context, req appqa.EffectIdentityRequest) (appqa.EffectIdentityBatch, error) {
	if a == nil || a.catalog == nil {
		return appqa.EffectIdentityBatch{}, ErrSessionNotPinned
	}
	if err := ctx.Err(); err != nil {
		return appqa.EffectIdentityBatch{}, err
	}
	if err := appqa.ValidateEffectIdentityRequest(req); err != nil {
		return appqa.EffectIdentityBatch{}, err
	}

	// 1) Live context before lookup.
	live1, err := a.catalog.CaptureLiveContext(ctx)
	if err != nil {
		return unavailableAll(req), nil
	}
	if !appqa.ResolutionContextSessionComplete(live1) {
		return unavailableAll(req), nil
	}

	// If caller supplied a session-complete resolution, it must match live session
	// identity (binding/db/role/version/epoch). Search path may differ only if
	// every candidate is explicit-schema; still require session compatibility.
	//
	// Unbound requests (no Resolution) are rejected: the application layer must
	// capture execution-bound context explicitly via CaptureExecutionBoundContext
	// before calling the adapter. This prevents direct callers from bypassing
	// the context-binding path.
	workReq := req
	if appqa.ResolutionContextSessionComplete(req.Resolution) {
		if !appqa.ResolutionContextSessionCompatible(req.Resolution, live1) {
			return unavailableAll(req), nil
		}
		// Prefer live path OIDs (authoritative for this session).
		workReq.Resolution = live1
	} else {
		// Incomplete or unbound Resolution: reject all candidates.
		return unavailableAll(req), nil
	}

	// 2) Catalog lookups under the pinned session.
	items := make([]appqa.EffectIdentityItem, 0, len(workReq.Candidates))
	for _, cand := range workReq.Candidates {
		items = append(items, a.resolveOne(ctx, workReq, cand))
	}
	batch := appqa.NormalizeEffectIdentityBatch(items)

	// 3) Live context after lookup + gate (TOCTOU).
	live2, liveErr := a.catalog.CaptureLiveContext(ctx)
	return appqa.GateIdentityBatchAgainstLiveContext(workReq, batch, func() (appqa.EffectIdentityResolutionContext, error) {
		return live2, liveErr
	}), nil
}

func unavailableAll(req appqa.EffectIdentityRequest) appqa.EffectIdentityBatch {
	return appqa.BuildUnavailableBatch(req.Candidates)
}

func (a *EffectIdentityAdapter) resolveOne(ctx context.Context, req appqa.EffectIdentityRequest, cand appqa.EffectCandidate) appqa.EffectIdentityItem {
	item := appqa.EffectIdentityItem{Ordinal: cand.Ordinal}

	// Operand kinds that require type inference beyond exact OID pins.
	if hasUnresolvedTypeKind(cand) {
		item.Status = domain.IdentityStatusCoercionGap
		return item
	}

	switch cand.Kind {
	case appqa.EffectCandidateOperator:
		return a.resolveOperator(ctx, req, cand)
	case appqa.EffectCandidateFunction:
		return a.resolveFunction(ctx, req, cand)
	case appqa.EffectCandidateCast:
		return a.resolveCast(ctx, req, cand)
	default:
		item.Status = domain.IdentityStatusUnknown
		return item
	}
}

func hasUnresolvedTypeKind(cand appqa.EffectCandidate) bool {
	// Arity-0 functions (e.g. count(*)) need no type resolution regardless
	// of OperandKinds (star is a structural hint, not a type-bearing operand).
	if cand.Kind == appqa.EffectCandidateFunction && cand.Arity == 0 {
		return false
	}
	for _, k := range cand.OperandKinds {
		switch k {
		case "param", "star", "subquery", "expr", "unknown":
			// Without exact type OIDs these are not exact-match resolvable.
			return true
		}
	}
	return false
}

func (a *EffectIdentityAdapter) resolveOperator(ctx context.Context, req appqa.EffectIdentityRequest, cand appqa.EffectCandidate) appqa.EffectIdentityItem {
	item := appqa.EffectIdentityItem{Ordinal: cand.Ordinal}
	name, schema, ok := candidateName(cand)
	if !ok || name == "" {
		item.Status = domain.IdentityStatusUnknown
		return item
	}
	typeOIDs := req.OperandTypeOIDs[cand.Ordinal]
	if len(typeOIDs) != cand.Arity || cand.Arity == 0 {
		// Exact operand OIDs required; missing ⇒ coercion_gap (not name guess).
		item.Status = domain.IdentityStatusCoercionGap
		return item
	}
	var left, right uint32
	switch cand.Arity {
	case 1:
		// Unary: PostgreSQL uses 0 on the unused side; we require one type OID.
		left, right = 0, typeOIDs[0]
	case 2:
		left, right = typeOIDs[0], typeOIDs[1]
	default:
		item.Status = domain.IdentityStatusUnknown
		return item
	}

	nsList, status := a.namespacesForCandidate(ctx, req, schema, cand)
	if status != "" {
		item.Status = status
		return item
	}

	var matches []operatorRow
	for _, ns := range nsList {
		rows, err := a.catalog.lookupOperators(ctx, ns, name, left, right)
		if err != nil {
			item.Status = appqa.MapCatalogErrorToStatus(err)
			return item
		}
		matches = append(matches, rows...)
	}
	switch len(matches) {
	case 0:
		item.Status = domain.IdentityStatusUnknown
		return item
	case 1:
		r := matches[0]
		facts := &appqa.EffectIdentityFacts{
			Kind:               appqa.EffectCandidateOperator,
			ObjectOID:          r.OID,
			NamespaceOID:       r.NamespaceOID,
			OperandTypeOIDs:    append([]uint32(nil), typeOIDs...),
			ResultTypeOID:      r.ResultTypeOID,
			ImplementationOID:  r.ImplementationOID,
			Volatility:         appqa.EffectVolatility(r.Volatility),
			CanonicalSignature: fmt.Sprintf("%s.%s(%s)", r.SchemaName, r.OperatorName, joinOIDs(typeOIDs)),
		}
		appqa.StampFactsFromResolution(facts, req.Resolution)
		item.Status = domain.IdentityStatusResolved
		item.Facts = facts
		return item
	default:
		item.Status = domain.IdentityStatusAmbiguous
		return item
	}
}

func (a *EffectIdentityAdapter) resolveFunction(ctx context.Context, req appqa.EffectIdentityRequest, cand appqa.EffectCandidate) appqa.EffectIdentityItem {
	item := appqa.EffectIdentityItem{Ordinal: cand.Ordinal}
	name, schema, ok := candidateName(cand)
	if !ok || name == "" {
		item.Status = domain.IdentityStatusUnknown
		return item
	}
	argOIDs := req.OperandTypeOIDs[cand.Ordinal]
	// Arity 0 (e.g. count(*)) allows empty arg list; otherwise require exact length.
	if cand.Arity > 0 && len(argOIDs) != cand.Arity {
		item.Status = domain.IdentityStatusCoercionGap
		return item
	}
	if cand.Arity == 0 {
		argOIDs = nil
	}

	nsList, status := a.namespacesForCandidate(ctx, req, schema, cand)
	if status != "" {
		item.Status = status
		return item
	}

	var matches []functionRow
	for _, ns := range nsList {
		rows, err := a.catalog.lookupFunctions(ctx, ns, name, argOIDs)
		if err != nil {
			item.Status = appqa.MapCatalogErrorToStatus(err)
			return item
		}
		matches = append(matches, rows...)
	}
	switch len(matches) {
	case 0:
		item.Status = domain.IdentityStatusUnknown
		return item
	case 1:
		r := matches[0]
		facts := &appqa.EffectIdentityFacts{
			Kind:               appqa.EffectCandidateFunction,
			ObjectOID:          r.OID,
			NamespaceOID:       r.NamespaceOID,
			OperandTypeOIDs:    append([]uint32(nil), r.ArgTypeOIDs...),
			ResultTypeOID:      r.ResultType,
			Volatility:         appqa.EffectVolatility(r.Volatility),
			CanonicalSignature: fmt.Sprintf("%s.%s(%s)", r.SchemaName, r.FuncName, joinOIDs(r.ArgTypeOIDs)),
		}
		appqa.StampFactsFromResolution(facts, req.Resolution)
		item.Status = domain.IdentityStatusResolved
		item.Facts = facts
		return item
	default:
		item.Status = domain.IdentityStatusAmbiguous
		return item
	}
}

func (a *EffectIdentityAdapter) resolveCast(ctx context.Context, req appqa.EffectIdentityRequest, cand appqa.EffectCandidate) appqa.EffectIdentityItem {
	item := appqa.EffectIdentityItem{Ordinal: cand.Ordinal}
	// Source type from OperandTypeOIDs[0]; target from TargetTypePath.
	srcOIDs := req.OperandTypeOIDs[cand.Ordinal]
	if len(srcOIDs) != 1 || srcOIDs[0] == 0 {
		item.Status = domain.IdentityStatusCoercionGap
		return item
	}
	targetOID, err := a.resolveTargetTypeOID(ctx, req, cand)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			item.Status = domain.IdentityStatusUnknown
			return item
		}
		item.Status = appqa.MapCatalogErrorToStatus(err)
		return item
	}
	if targetOID == 0 {
		item.Status = domain.IdentityStatusUnknown
		return item
	}

	rows, err := a.catalog.lookupCasts(ctx, srcOIDs[0], targetOID)
	if err != nil {
		item.Status = appqa.MapCatalogErrorToStatus(err)
		return item
	}
	switch len(rows) {
	case 0:
		// No exact cast row — not a coercion search.
		item.Status = domain.IdentityStatusUnknown
		return item
	case 1:
		r := rows[0]
		method := appqa.EffectCastMethod(r.CastMethod)
		facts := &appqa.EffectIdentityFacts{
			Kind:               appqa.EffectCandidateCast,
			ObjectOID:          r.OID,
			OperandTypeOIDs:    []uint32{r.SourceOID},
			ResultTypeOID:      r.TargetOID,
			CastMethod:         method,
			CastFunctionOID:    r.CastFuncOID,
			CanonicalSignature: fmt.Sprintf("cast(%s.%s->%s.%s)", r.SourceSchema, r.SourceName, r.TargetSchema, r.TargetName),
		}
		// Cast rows are not namespaced like operators; leave NamespaceOID 0 or set target ns.
		appqa.StampFactsFromResolution(facts, req.Resolution)
		item.Status = domain.IdentityStatusResolved
		item.Facts = facts
		return item
	default:
		item.Status = domain.IdentityStatusAmbiguous
		return item
	}
}

func (a *EffectIdentityAdapter) resolveTargetTypeOID(ctx context.Context, req appqa.EffectIdentityRequest, cand appqa.EffectCandidate) (uint32, error) {
	path := cand.TargetTypePath
	if len(path) == 0 {
		return 0, sql.ErrNoRows
	}
	var schema, typ string
	if len(path) == 1 {
		typ = path[0]
	} else {
		schema = path[0]
		typ = path[len(path)-1]
	}
	return a.catalog.typeOIDByName(ctx, schema, typ, req.Resolution.NamespaceSearchOIDs)
}

// namespacesForCandidate returns namespace OIDs to search.
// Explicit schema → single namespace OID (or unknown).
// Unqualified → full search path; empty path → unavailable.
func (a *EffectIdentityAdapter) namespacesForCandidate(ctx context.Context, req appqa.EffectIdentityRequest, schema string, cand appqa.EffectCandidate) ([]uint32, domain.IdentityStatus) {
	if appqa.CandidateExplicitlyQualified(cand) || schema != "" {
		if schema == "" {
			return nil, domain.IdentityStatusUnknown
		}
		oid, err := a.catalog.namespaceOIDByName(ctx, schema)
		if err == sql.ErrNoRows {
			return nil, domain.IdentityStatusUnknown
		}
		if err != nil {
			return nil, appqa.MapCatalogErrorToStatus(err)
		}
		return []uint32{oid}, ""
	}
	// Unqualified: require usable path; never invent pg_catalog.
	if !appqa.ResolutionContextUsableForUnqualified(req.Resolution) {
		return nil, domain.IdentityStatusUnavailable
	}
	return append([]uint32(nil), req.Resolution.NamespaceSearchOIDs...), ""
}

func candidateName(cand appqa.EffectCandidate) (name, schema string, ok bool) {
	path := cand.NamePath
	if len(path) == 0 {
		return "", "", false
	}
	if len(path) == 1 {
		return path[0], "", true
	}
	return path[len(path)-1], path[0], true
}

func joinOIDs(oids []uint32) string {
	if len(oids) == 0 {
		return ""
	}
	parts := make([]string, len(oids))
	for i, o := range oids {
		parts[i] = strconv.FormatUint(uint64(o), 10)
	}
	return strings.Join(parts, ",")
}

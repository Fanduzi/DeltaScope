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
	columnTypeOID(ctx context.Context, schema, table, column string) (uint32, error)
	resolveColumnTypeOIDBySearchPath(ctx context.Context, table, column string, searchPathOIDs []uint32) (uint32, error)
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
var _ appqa.AtomicProofResolver = (*EffectIdentityAdapter)(nil)
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

// ResolveColumnTypeOIDs resolves type OIDs for operand columns on the pinned session.
// Returns a map from candidate ordinal to type OID slice.
// Only resolves binary operators with two column operands from base tables.
// Returns empty map if any column cannot be resolved (fail-closed).
func (a *EffectIdentityAdapter) ResolveColumnTypeOIDs(ctx context.Context, candidates []appqa.EffectCandidate) (map[int][]uint32, error) {
	if a == nil || a.catalog == nil {
		return nil, ErrSessionNotPinned
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	live, err := a.catalog.CaptureLiveContext(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[int][]uint32)
	for _, cand := range candidates {
		if cand.Kind != appqa.EffectCandidateOperator || cand.Arity != 2 {
			continue
		}
		refs := cand.OperandColumnRefs
		if len(refs) != 2 {
			continue
		}
		leftOID, err := a.resolveOneColumnTypeOID(ctx, refs[0], live.NamespaceSearchOIDs)
		if err != nil || leftOID == 0 {
			continue
		}
		rightOID, err := a.resolveOneColumnTypeOID(ctx, refs[1], live.NamespaceSearchOIDs)
		if err != nil || rightOID == 0 {
			continue
		}
		result[cand.Ordinal] = []uint32{leftOID, rightOID}
	}
	return result, nil
}

func (a *EffectIdentityAdapter) resolveOneColumnTypeOID(ctx context.Context, ref appqa.OperandColumnRef, searchPathOIDs []uint32) (uint32, error) {
	if ref.Table == "" || ref.Column == "" {
		return 0, nil
	}
	if ref.Schema != "" {
		return a.catalog.columnTypeOID(ctx, ref.Schema, ref.Table, ref.Column)
	}
	if len(searchPathOIDs) == 0 {
		return 0, nil
	}
	return a.catalog.resolveColumnTypeOIDBySearchPath(ctx, ref.Table, ref.Column, searchPathOIDs)
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

// txCatalog implements effectIdentityCatalog using a *sql.Tx for
// snapshot-consistent reads under REPEATABLE READ isolation.
type txCatalog struct {
	tx *sql.Tx
}

func (t *txCatalog) CaptureLiveContext(ctx context.Context) (appqa.EffectIdentityResolutionContext, error) {
	var (
		databaseOID      uint32
		roleOID          uint32
		serverVersionNum int
		backendPID       int64
	)
	err := t.tx.QueryRowContext(ctx, `
		select
			(select d.oid from pg_catalog.pg_database d where d.datname = current_database()),
			(select r.oid from pg_catalog.pg_roles r where r.rolname = current_user),
			current_setting('server_version_num')::int,
			pg_catalog.pg_backend_pid()
	`).Scan(&databaseOID, &roleOID, &serverVersionNum, &backendPID)
	if err != nil {
		return appqa.EffectIdentityResolutionContext{}, errSessionCapture
	}
	if databaseOID == 0 || roleOID == 0 || serverVersionNum == 0 || backendPID == 0 {
		return appqa.EffectIdentityResolutionContext{}, errSessionCapture
	}

	pathOIDs, err := captureSearchPathOIDsTx(ctx, t.tx)
	if err != nil {
		return appqa.EffectIdentityResolutionContext{}, err
	}

	binding := "b" + strconv.FormatInt(backendPID, 10) + "-d" + strconv.FormatUint(uint64(databaseOID), 10)

	return appqa.EffectIdentityResolutionContext{
		Bound:               true,
		SessionBinding:      binding,
		PathEpoch:           1,
		NamespaceSearchOIDs: pathOIDs,
		DatabaseOID:         databaseOID,
		RoleOID:             roleOID,
		ServerVersionNum:    serverVersionNum,
	}, nil
}

func captureSearchPathOIDsTx(ctx context.Context, tx *sql.Tx) ([]uint32, error) {
	rows, err := tx.QueryContext(ctx, `
		select n.oid
		from unnest(pg_catalog.current_schemas(true)) with ordinality as s(nspname, ord)
		join pg_catalog.pg_namespace n on n.nspname = s.nspname
		order by s.ord
	`)
	if err != nil {
		return nil, errSessionCapture
	}
	defer rows.Close()

	var out []uint32
	for rows.Next() {
		var oid uint32
		if err := rows.Scan(&oid); err != nil {
			return nil, errSessionCapture
		}
		if oid != 0 {
			out = append(out, oid)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, errSessionCapture
	}
	return out, nil
}

func (t *txCatalog) namespaceOIDByName(ctx context.Context, name string) (uint32, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, sql.ErrNoRows
	}
	var oid uint32
	err := t.tx.QueryRowContext(ctx, `
		select n.oid from pg_catalog.pg_namespace n where n.nspname = $1
	`, name).Scan(&oid)
	return oid, err
}

func (t *txCatalog) typeOIDByName(ctx context.Context, schema, typname string, path []uint32) (uint32, error) {
	typname = strings.TrimSpace(typname)
	if typname == "" {
		return 0, sql.ErrNoRows
	}
	if schema != "" {
		var oid uint32
		err := t.tx.QueryRowContext(ctx, `
			select t.oid
			from pg_catalog.pg_type t
			join pg_catalog.pg_namespace n on n.oid = t.typnamespace
			where n.nspname = $1 and t.typname = $2
		`, schema, typname).Scan(&oid)
		return oid, err
	}
	for _, ns := range path {
		var oid uint32
		err := t.tx.QueryRowContext(ctx, `
			select t.oid from pg_catalog.pg_type t
			where t.typnamespace = $1 and t.typname = $2
		`, ns, typname).Scan(&oid)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return 0, err
		}
		return oid, nil
	}
	return 0, sql.ErrNoRows
}

func (t *txCatalog) lookupOperators(ctx context.Context, nsOID uint32, name string, left, right uint32) ([]operatorRow, error) {
	rows, err := t.tx.QueryContext(ctx, `
		select o.oid, o.oprnamespace, o.oprcode::pg_catalog.oid, o.oprresult, coalesce(p.provolatile, ''),
		       n.nspname, o.oprname
		from pg_catalog.pg_operator o
		join pg_catalog.pg_namespace n on n.oid = o.oprnamespace
		left join pg_catalog.pg_proc p on p.oid = o.oprcode
		where o.oprnamespace = $1
		  and o.oprname = $2
		  and o.oprleft = $3
		  and o.oprright = $4
	`, nsOID, name, left, right)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []operatorRow
	for rows.Next() {
		var r operatorRow
		if err := rows.Scan(&r.OID, &r.NamespaceOID, &r.ImplementationOID, &r.ResultTypeOID, &r.Volatility, &r.SchemaName, &r.OperatorName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (t *txCatalog) lookupFunctions(ctx context.Context, nsOID uint32, name string, argOIDs []uint32) ([]functionRow, error) {
	argVec := oidVectorLiteral(argOIDs)
	rows, err := t.tx.QueryContext(ctx, `
		select p.oid, p.pronamespace, p.prorettype, p.provolatile, n.nspname, p.proname, p.proargtypes::text
		from pg_catalog.pg_proc p
		join pg_catalog.pg_namespace n on n.oid = p.pronamespace
		where p.pronamespace = $1
		  and p.proname = $2
		  and p.proargtypes = $3::pg_catalog.oidvector
	`, nsOID, name, argVec)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []functionRow
	for rows.Next() {
		var r functionRow
		var argText string
		if err := rows.Scan(&r.OID, &r.NamespaceOID, &r.ResultType, &r.Volatility, &r.SchemaName, &r.FuncName, &argText); err != nil {
			return nil, err
		}
		r.ArgTypeOIDs = parseOIDVectorText(argText)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (t *txCatalog) lookupCasts(ctx context.Context, sourceOID, targetOID uint32) ([]castRow, error) {
	rows, err := t.tx.QueryContext(ctx, `
		select c.oid, c.castsource, c.casttarget, coalesce(c.castfunc, 0), c.castmethod::text,
		       ns.nspname, ts.typname, nt.nspname, tt.typname
		from pg_catalog.pg_cast c
		join pg_catalog.pg_type ts on ts.oid = c.castsource
		join pg_catalog.pg_namespace ns on ns.oid = ts.typnamespace
		join pg_catalog.pg_type tt on tt.oid = c.casttarget
		join pg_catalog.pg_namespace nt on nt.oid = tt.typnamespace
		where c.castsource = $1 and c.casttarget = $2
	`, sourceOID, targetOID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []castRow
	for rows.Next() {
		var r castRow
		if err := rows.Scan(&r.OID, &r.SourceOID, &r.TargetOID, &r.CastFuncOID, &r.CastMethod,
			&r.SourceSchema, &r.SourceName, &r.TargetSchema, &r.TargetName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (t *txCatalog) columnTypeOID(ctx context.Context, schema, table, column string) (uint32, error) {
	var oid uint32
	err := t.tx.QueryRowContext(ctx, `
		SELECT a.atttypid
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2 AND a.attname = $3
		  AND a.attnum > 0 AND NOT a.attisdropped
	`, schema, table, column).Scan(&oid)
	if err != nil {
		return 0, err
	}
	return oid, nil
}

func (t *txCatalog) resolveColumnTypeOIDBySearchPath(ctx context.Context, table, column string, searchPathOIDs []uint32) (uint32, error) {
	for _, nsOID := range searchPathOIDs {
		var oid uint32
		err := t.tx.QueryRowContext(ctx, `
			SELECT a.atttypid
			FROM pg_catalog.pg_attribute a
			JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
			WHERE c.relnamespace = $1 AND c.relname = $2 AND a.attname = $3
			  AND a.attnum > 0 AND NOT a.attisdropped
		`, nsOID, table, column).Scan(&oid)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return 0, err
		}
		if oid != 0 {
			return oid, nil
		}
	}
	return 0, sql.ErrNoRows
}

// ResolveColumnTypesAndEffectIdentities performs column type OID resolution
// and effect identity resolution in a single atomic REPEATABLE READ transaction.
// This ensures both type facts and identity facts come from the same catalog
// snapshot, preventing TOCTOU issues with concurrent DDL.
//
// Returns the final execution-bound context captured INSIDE the atomic
// transaction for TOCTOU validation by the application.
func (a *EffectIdentityAdapter) ResolveColumnTypesAndEffectIdentities(
	ctx context.Context,
	candidates []appqa.EffectCandidate,
	req appqa.EffectIdentityRequest,
) (map[int][]uint32, appqa.EffectIdentityBatch, appqa.EffectIdentityResolutionContext, error) {
	if a == nil || a.catalog == nil {
		return nil, appqa.EffectIdentityBatch{}, appqa.EffectIdentityResolutionContext{}, ErrSessionNotPinned
	}
	if err := ctx.Err(); err != nil {
		return nil, appqa.EffectIdentityBatch{}, appqa.EffectIdentityResolutionContext{}, err
	}

	// Access the pinned session to start a REPEATABLE READ transaction.
	pinned, ok := a.catalog.(*PinnedSession)
	if !ok {
		return nil, appqa.EffectIdentityBatch{}, appqa.EffectIdentityResolutionContext{}, errors.New("atomic proof requires PinnedSession")
	}

	tx, err := pinned.beginRepeatableReadTx(ctx)
	if err != nil {
		return nil, appqa.EffectIdentityBatch{}, appqa.EffectIdentityResolutionContext{}, err
	}
	defer tx.Rollback() //nolint:errcheck // rollback is best-effort

	txCat := &txCatalog{tx: tx}

	// 1) Live context before lookup (under tx snapshot).
	live1, err := txCat.CaptureLiveContext(ctx)
	if err != nil {
		return nil, appqa.BuildUnavailableBatch(candidates), appqa.EffectIdentityResolutionContext{}, nil
	}
	if !appqa.ResolutionContextSessionComplete(live1) {
		return nil, appqa.BuildUnavailableBatch(candidates), appqa.EffectIdentityResolutionContext{}, nil
	}

	// Validate caller-supplied resolution matches live session.
	workReq := req
	if appqa.ResolutionContextSessionComplete(req.Resolution) {
		if !appqa.ResolutionContextSessionCompatible(req.Resolution, live1) {
			return nil, appqa.BuildUnavailableBatch(candidates), appqa.EffectIdentityResolutionContext{}, nil
		}
		workReq.Resolution = live1
	} else {
		return nil, appqa.BuildUnavailableBatch(candidates), appqa.EffectIdentityResolutionContext{}, nil
	}

	// 2) Resolve column type OIDs for binary column operators under the same tx.
	typeOIDs := make(map[int][]uint32)
	for _, cand := range candidates {
		if cand.Kind != appqa.EffectCandidateOperator || cand.Arity != 2 {
			continue
		}
		refs := cand.OperandColumnRefs
		if len(refs) != 2 {
			continue
		}
		leftOID, err := resolveOneColumnTypeOIDWithCatalog(ctx, txCat, refs[0], live1.NamespaceSearchOIDs)
		if err != nil || leftOID == 0 {
			continue
		}
		rightOID, err := resolveOneColumnTypeOIDWithCatalog(ctx, txCat, refs[1], live1.NamespaceSearchOIDs)
		if err != nil || rightOID == 0 {
			continue
		}
		typeOIDs[cand.Ordinal] = []uint32{leftOID, rightOID}
	}

	// Merge caller-provided OperandTypeOIDs with resolved column type OIDs.
	// Resolved OIDs take precedence for candidates that have column refs.
	mergedTypeOIDs := make(map[int][]uint32, len(req.OperandTypeOIDs)+len(typeOIDs))
	for k, v := range req.OperandTypeOIDs {
		mergedTypeOIDs[k] = v
	}
	for k, v := range typeOIDs {
		mergedTypeOIDs[k] = v
	}
	workReq.OperandTypeOIDs = mergedTypeOIDs

	// 3) Resolve effect identities under the same tx.
	items := make([]appqa.EffectIdentityItem, 0, len(candidates))
	for _, cand := range candidates {
		items = append(items, resolveOneWithCatalog(ctx, txCat, workReq, cand))
	}
	batch := appqa.NormalizeEffectIdentityBatch(items)

	// 4) Live context after lookup + gate (TOCTOU) under same tx.
	// This is the final context captured INSIDE the atomic operation.
	live2, liveErr := txCat.CaptureLiveContext(ctx)
	batch = appqa.GateIdentityBatchAgainstLiveContext(workReq, batch, func() (appqa.EffectIdentityResolutionContext, error) {
		return live2, liveErr
	})

	// 5) Commit the REPEATABLE READ transaction.
	if err := tx.Commit(); err != nil {
		return nil, appqa.BuildUnavailableBatch(candidates), appqa.EffectIdentityResolutionContext{}, nil
	}

	// Return live2 as the final execution-bound context for TOCTOU validation.
	// If live2 capture failed, return empty context (will fail validation).
	finalCtx := live2
	if liveErr != nil {
		finalCtx = appqa.EffectIdentityResolutionContext{}
	}

	return typeOIDs, batch, finalCtx, nil
}

// resolveOneWithCatalog resolves a single candidate using the provided catalog.
// This is the tx-aware variant of EffectIdentityAdapter.resolveOne.
func resolveOneWithCatalog(ctx context.Context, cat effectIdentityCatalog, req appqa.EffectIdentityRequest, cand appqa.EffectCandidate) appqa.EffectIdentityItem {
	item := appqa.EffectIdentityItem{Ordinal: cand.Ordinal}

	if hasUnresolvedTypeKind(cand) {
		item.Status = domain.IdentityStatusCoercionGap
		return item
	}

	switch cand.Kind {
	case appqa.EffectCandidateOperator:
		return resolveOperatorWithCatalog(ctx, cat, req, cand)
	case appqa.EffectCandidateFunction:
		return resolveFunctionWithCatalog(ctx, cat, req, cand)
	case appqa.EffectCandidateCast:
		return resolveCastWithCatalog(ctx, cat, req, cand)
	default:
		item.Status = domain.IdentityStatusUnknown
		return item
	}
}

func resolveOneColumnTypeOIDWithCatalog(ctx context.Context, cat effectIdentityCatalog, ref appqa.OperandColumnRef, searchPathOIDs []uint32) (uint32, error) {
	if ref.Table == "" || ref.Column == "" {
		return 0, nil
	}
	if ref.Schema != "" {
		return cat.columnTypeOID(ctx, ref.Schema, ref.Table, ref.Column)
	}
	if len(searchPathOIDs) == 0 {
		return 0, nil
	}
	return cat.resolveColumnTypeOIDBySearchPath(ctx, ref.Table, ref.Column, searchPathOIDs)
}

func resolveOperatorWithCatalog(ctx context.Context, cat effectIdentityCatalog, req appqa.EffectIdentityRequest, cand appqa.EffectCandidate) appqa.EffectIdentityItem {
	item := appqa.EffectIdentityItem{Ordinal: cand.Ordinal}
	name, schema, ok := candidateName(cand)
	if !ok || name == "" {
		item.Status = domain.IdentityStatusUnknown
		return item
	}
	tOIDs := req.OperandTypeOIDs[cand.Ordinal]
	if len(tOIDs) != cand.Arity || cand.Arity == 0 {
		item.Status = domain.IdentityStatusCoercionGap
		return item
	}
	var left, right uint32
	switch cand.Arity {
	case 1:
		left, right = 0, tOIDs[0]
	case 2:
		left, right = tOIDs[0], tOIDs[1]
	default:
		item.Status = domain.IdentityStatusUnknown
		return item
	}

	nsList, status := namespacesForCandidateWithCatalog(ctx, cat, req, schema, cand)
	if status != "" {
		item.Status = status
		return item
	}

	var matches []operatorRow
	for _, ns := range nsList {
		rows, err := cat.lookupOperators(ctx, ns, name, left, right)
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
			OperandTypeOIDs:    append([]uint32(nil), tOIDs...),
			ResultTypeOID:      r.ResultTypeOID,
			ImplementationOID:  r.ImplementationOID,
			Volatility:         appqa.EffectVolatility(r.Volatility),
			CanonicalSignature: fmt.Sprintf("%s.%s(%s)", r.SchemaName, r.OperatorName, joinOIDs(tOIDs)),
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

func resolveFunctionWithCatalog(ctx context.Context, cat effectIdentityCatalog, req appqa.EffectIdentityRequest, cand appqa.EffectCandidate) appqa.EffectIdentityItem {
	item := appqa.EffectIdentityItem{Ordinal: cand.Ordinal}
	name, schema, ok := candidateName(cand)
	if !ok || name == "" {
		item.Status = domain.IdentityStatusUnknown
		return item
	}
	argOIDs := req.OperandTypeOIDs[cand.Ordinal]
	if cand.Arity > 0 && len(argOIDs) != cand.Arity {
		item.Status = domain.IdentityStatusCoercionGap
		return item
	}
	if cand.Arity == 0 {
		argOIDs = nil
	}

	nsList, status := namespacesForCandidateWithCatalog(ctx, cat, req, schema, cand)
	if status != "" {
		item.Status = status
		return item
	}

	var matches []functionRow
	for _, ns := range nsList {
		rows, err := cat.lookupFunctions(ctx, ns, name, argOIDs)
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

func resolveCastWithCatalog(ctx context.Context, cat effectIdentityCatalog, req appqa.EffectIdentityRequest, cand appqa.EffectCandidate) appqa.EffectIdentityItem {
	item := appqa.EffectIdentityItem{Ordinal: cand.Ordinal}
	srcOIDs := req.OperandTypeOIDs[cand.Ordinal]
	if len(srcOIDs) != 1 || srcOIDs[0] == 0 {
		item.Status = domain.IdentityStatusCoercionGap
		return item
	}
	targetOID, err := resolveTargetTypeOIDWithCatalog(ctx, cat, req, cand)
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

	rows, err := cat.lookupCasts(ctx, srcOIDs[0], targetOID)
	if err != nil {
		item.Status = appqa.MapCatalogErrorToStatus(err)
		return item
	}
	switch len(rows) {
	case 0:
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
		appqa.StampFactsFromResolution(facts, req.Resolution)
		item.Status = domain.IdentityStatusResolved
		item.Facts = facts
		return item
	default:
		item.Status = domain.IdentityStatusAmbiguous
		return item
	}
}

func resolveTargetTypeOIDWithCatalog(ctx context.Context, cat effectIdentityCatalog, req appqa.EffectIdentityRequest, cand appqa.EffectCandidate) (uint32, error) {
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
	return cat.typeOIDByName(ctx, schema, typ, req.Resolution.NamespaceSearchOIDs)
}

func namespacesForCandidateWithCatalog(ctx context.Context, cat effectIdentityCatalog, req appqa.EffectIdentityRequest, schema string, cand appqa.EffectCandidate) ([]uint32, domain.IdentityStatus) {
	if appqa.CandidateExplicitlyQualified(cand) || schema != "" {
		if schema == "" {
			return nil, domain.IdentityStatusUnknown
		}
		oid, err := cat.namespaceOIDByName(ctx, schema)
		if err == sql.ErrNoRows {
			return nil, domain.IdentityStatusUnknown
		}
		if err != nil {
			return nil, appqa.MapCatalogErrorToStatus(err)
		}
		return []uint32{oid}, ""
	}
	if !appqa.ResolutionContextUsableForUnqualified(req.Resolution) {
		return nil, domain.IdentityStatusUnavailable
	}
	return append([]uint32(nil), req.Resolution.NamespaceSearchOIDs...), ""
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

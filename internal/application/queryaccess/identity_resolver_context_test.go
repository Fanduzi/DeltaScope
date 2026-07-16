// Package queryaccess tests execution-bound resolution context policy (T6 P1).
// input: unqualified/explicit candidates, bound/unbound contexts, fake resolvers
// output: fail-closed statuses for shadowing, overload, db/role/version mismatch
// pos: contract safety for session binding / TOCTOU before T7 catalog adapter
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// completeBoundContext returns a Phase-1 promotion-ready resolution context.
func completeBoundContext() EffectIdentityResolutionContext {
	return EffectIdentityResolutionContext{
		Bound:               true,
		SessionBinding:      "sess-1",
		PathEpoch:           3,
		NamespaceSearchOIDs: []uint32{2200, 11},
		DatabaseOID:         16384,
		RoleOID:             10,
		ServerVersionNum:    160004,
	}
}

func resolvedFacts(objectOID, ns uint32, rc EffectIdentityResolutionContext) *EffectIdentityFacts {
	f := &EffectIdentityFacts{
		Kind:               EffectCandidateFunction,
		ObjectOID:          objectOID,
		NamespaceOID:       ns,
		CanonicalSignature: "test",
	}
	StampFactsFromResolution(f, rc)
	return f
}

func TestCandidateExplicitQualification(t *testing.T) {
	t.Parallel()
	unqual := EffectCandidate{Kind: EffectCandidateFunction, NamePath: []string{"count"}, Ordinal: 0}
	if CandidateExplicitlyQualified(unqual) || CandidateExplicitPgCatalog(unqual) {
		t.Fatalf("unqualified count must not be treated as explicit pg_catalog")
	}
	op := EffectCandidate{Kind: EffectCandidateOperator, NamePath: []string{"="}, Ordinal: 1}
	if CandidateExplicitlyQualified(op) {
		t.Fatal("bare = is unqualified")
	}
	explicit := EffectCandidate{
		Kind: EffectCandidateFunction, NamePath: []string{"pg_catalog", "count"},
		ExplicitSchema: true, Ordinal: 2,
	}
	if !CandidateExplicitlyQualified(explicit) || !CandidateExplicitPgCatalog(explicit) {
		t.Fatal("pg_catalog.count must be explicit pg_catalog")
	}
	if CandidateExplicitSchemaName(explicit) != PgCatalogNamespaceName {
		t.Fatalf("schema=%q", CandidateExplicitSchemaName(explicit))
	}
	publicUDF := EffectCandidate{
		Kind: EffectCandidateFunction, NamePath: []string{"public", "count"},
		ExplicitSchema: true, Ordinal: 3,
	}
	if !CandidateExplicitlyQualified(publicUDF) || CandidateExplicitPgCatalog(publicUDF) {
		t.Fatal("public.count is explicit but not pg_catalog")
	}
	cast := EffectCandidate{
		Kind: EffectCandidateCast, TargetTypePath: []string{"pg_catalog", "text"},
		ExplicitSchema: true, Ordinal: 4,
	}
	if !CandidateExplicitPgCatalog(cast) {
		t.Fatal("pg_catalog.text cast must be explicit pg_catalog")
	}
}

func TestResolutionContext_SessionCompleteRequiresAllFields(t *testing.T) {
	t.Parallel()
	base := completeBoundContext()
	if !ResolutionContextSessionComplete(base) {
		t.Fatal("complete context must pass")
	}
	if !ResolutionContextUsableForUnqualified(base) {
		t.Fatal("complete context with path must be usable for unqualified")
	}

	// Each required field zeroed independently → incomplete.
	cases := []struct {
		name string
		mut  func(*EffectIdentityResolutionContext)
	}{
		{"unbound", func(rc *EffectIdentityResolutionContext) { rc.Bound = false }},
		{"empty binding", func(rc *EffectIdentityResolutionContext) { rc.SessionBinding = "" }},
		{"zero epoch", func(rc *EffectIdentityResolutionContext) { rc.PathEpoch = 0 }},
		{"zero database", func(rc *EffectIdentityResolutionContext) { rc.DatabaseOID = 0 }},
		{"zero role", func(rc *EffectIdentityResolutionContext) { rc.RoleOID = 0 }},
		{"zero version", func(rc *EffectIdentityResolutionContext) { rc.ServerVersionNum = 0 }},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rc := completeBoundContext()
			tc.mut(&rc)
			if ResolutionContextSessionComplete(rc) {
				t.Fatalf("%s still complete", tc.name)
			}
			if ResolutionContextUsableForUnqualified(rc) {
				t.Fatalf("%s still usable for unqualified", tc.name)
			}
		})
	}

	// Empty search path: session complete OK, unqualified not usable.
	rc := completeBoundContext()
	rc.NamespaceSearchOIDs = nil
	if !ResolutionContextSessionComplete(rc) {
		t.Fatal("empty path may still be session-complete")
	}
	if ResolutionContextUsableForUnqualified(rc) {
		t.Fatal("empty path not usable for unqualified")
	}
}

func TestResolutionContext_CompatibilityStrict(t *testing.T) {
	t.Parallel()
	a := completeBoundContext()
	b := completeBoundContext()
	if !ResolutionContextSessionCompatible(a, b) || !ResolutionContextsCompatible(a, b) {
		t.Fatal("identical complete contexts must be compatible")
	}

	// Zero on either side never matches (no "optional" comparison).
	b = a
	b.DatabaseOID = 0
	if ResolutionContextSessionCompatible(a, b) {
		t.Fatal("zero DatabaseOID must not be compatible")
	}
	b = a
	b.RoleOID = 0
	if ResolutionContextSessionCompatible(a, b) {
		t.Fatal("zero RoleOID must not be compatible")
	}
	b = a
	b.ServerVersionNum = 0
	if ResolutionContextSessionCompatible(a, b) {
		t.Fatal("zero ServerVersionNum must not be compatible")
	}
	b = a
	b.PathEpoch = 0
	if ResolutionContextSessionCompatible(a, b) {
		t.Fatal("zero PathEpoch must not be compatible")
	}

	// Field mismatches.
	for _, mut := range []func(*EffectIdentityResolutionContext){
		func(rc *EffectIdentityResolutionContext) { rc.SessionBinding = "other" },
		func(rc *EffectIdentityResolutionContext) { rc.PathEpoch = 99 },
		func(rc *EffectIdentityResolutionContext) { rc.DatabaseOID = 1 },
		func(rc *EffectIdentityResolutionContext) { rc.RoleOID = 1 },
		func(rc *EffectIdentityResolutionContext) { rc.ServerVersionNum = 150000 },
	} {
		b = a
		mut(&b)
		if ResolutionContextSessionCompatible(a, b) {
			t.Fatalf("mismatch still compatible: %+v", b)
		}
		if ResolutionContextsCompatible(a, b) {
			t.Fatalf("full compat must fail on session mismatch: %+v", b)
		}
	}

	// Search path order only: session compatible, full not.
	b = a
	b.NamespaceSearchOIDs = []uint32{11, 2200}
	if !ResolutionContextSessionCompatible(a, b) {
		t.Fatal("path reorder must not break session compatibility")
	}
	if ResolutionContextsCompatible(a, b) {
		t.Fatal("path reorder must break full compatibility")
	}
	if ResolutionContextSearchPathCompatible(a, b) {
		t.Fatal("reordered path must not be path-compatible")
	}
}

func TestValidateEffectIdentityRequest_BoundRequiresCompleteSession(t *testing.T) {
	t.Parallel()
	cand := []EffectCandidate{{Kind: EffectCandidateOperator, Ordinal: 0, NamePath: []string{"="}}}
	// Bound with only binding+path is invalid.
	req := EffectIdentityRequest{
		Candidates: cand,
		Resolution: EffectIdentityResolutionContext{
			Bound: true, SessionBinding: "sess", PathEpoch: 1,
			NamespaceSearchOIDs: []uint32{11},
		},
	}
	if err := ValidateEffectIdentityRequest(req); !errors.Is(err, ErrIdentityRequestInvalid) {
		t.Fatalf("partial bound want invalid, got %v", err)
	}
	// Zero version still invalid.
	req.Resolution = completeBoundContext()
	req.Resolution.ServerVersionNum = 0
	if err := ValidateEffectIdentityRequest(req); !errors.Is(err, ErrIdentityRequestInvalid) {
		t.Fatalf("zero version want invalid, got %v", err)
	}
	req.Resolution = completeBoundContext()
	if err := ValidateEffectIdentityRequest(req); err != nil {
		t.Fatalf("complete bound: %v", err)
	}
	// Unbound zero context is valid (gates make everything unavailable).
	req.Resolution = EffectIdentityResolutionContext{}
	if err := ValidateEffectIdentityRequest(req); err != nil {
		t.Fatalf("unbound: %v", err)
	}
}

func TestGate_UnqualifiedWithoutContextUnavailable_NotPgCatalogGuess(t *testing.T) {
	t.Parallel()
	req := EffectIdentityRequest{
		Dialect: "postgresql",
		Candidates: []EffectCandidate{
			{Kind: EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}, IsAggregate: true},
			{Kind: EffectCandidateOperator, Ordinal: 1, NamePath: []string{"="}, Arity: 2},
		},
	}
	guessed := EffectIdentityBatch{Items: []EffectIdentityItem{
		{
			Ordinal: 0, Status: domain.IdentityStatusResolved,
			Facts: &EffectIdentityFacts{ObjectOID: 2803, NamespaceOID: 11, DatabaseOID: 1, ServerVersionNum: 160004},
		},
		{
			Ordinal: 1, Status: domain.IdentityStatusResolved,
			Facts: &EffectIdentityFacts{ObjectOID: 96, NamespaceOID: 11, DatabaseOID: 1, ServerVersionNum: 160004},
		},
	}}
	gated := GateIdentityBatchByResolutionContext(req, guessed)
	for _, it := range gated.Items {
		if it.Status != domain.IdentityStatusUnavailable || it.Facts != nil {
			t.Errorf("unbound must discard all facts: %+v", it)
		}
	}
}

func TestGate_ExplicitSchemaRequiresSessionComplete(t *testing.T) {
	t.Parallel()
	// Explicit schema without session binding cannot keep database-local OIDs.
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{{
			Kind: EffectCandidateFunction, Ordinal: 0,
			NamePath: []string{"pg_catalog", "count"}, ExplicitSchema: true,
		}},
	}
	batch := EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0, Status: domain.IdentityStatusResolved,
		Facts: &EffectIdentityFacts{ObjectOID: 2803, NamespaceOID: 11, DatabaseOID: 1, ServerVersionNum: 160004},
	}}}
	gated := GateIdentityBatchByResolutionContext(req, batch)
	if gated.Items[0].Status != domain.IdentityStatusUnavailable || gated.Items[0].Facts != nil {
		t.Fatalf("explicit schema without session-complete context must drop facts: %+v", gated.Items[0])
	}

	// With complete session (path optional for explicit): facts kept when pinned.
	rc := completeBoundContext()
	rc.NamespaceSearchOIDs = nil // explicit does not need search_path
	req.Resolution = rc
	facts := resolvedFacts(2803, 11, rc)
	batch = EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0, Status: domain.IdentityStatusResolved, Facts: facts,
	}}}
	gated = GateIdentityBatchByResolutionContext(req, batch)
	if gated.Items[0].Status != domain.IdentityStatusResolved || gated.Items[0].Facts == nil {
		t.Fatalf("explicit with session-complete may keep facts: %+v", gated.Items[0])
	}
}

func TestGate_ZeroFieldAndUnpinnedFactsUnavailable(t *testing.T) {
	t.Parallel()
	rc := completeBoundContext()
	req := EffectIdentityRequest{
		Resolution: rc,
		Candidates: []EffectCandidate{
			{Kind: EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}},
			{Kind: EffectCandidateFunction, Ordinal: 1, NamePath: []string{"pg_catalog", "count"}, ExplicitSchema: true},
		},
	}
	// Resolved but missing DatabaseOID/ServerVersionNum pins on facts.
	batch := EffectIdentityBatch{Items: []EffectIdentityItem{
		{Ordinal: 0, Status: domain.IdentityStatusResolved, Facts: &EffectIdentityFacts{ObjectOID: 1, NamespaceOID: 11}},
		{Ordinal: 1, Status: domain.IdentityStatusResolved, Facts: &EffectIdentityFacts{ObjectOID: 2, NamespaceOID: 11}},
	}}
	gated := GateIdentityBatchByResolutionContext(req, batch)
	for _, it := range gated.Items {
		if it.Status != domain.IdentityStatusUnavailable || it.Facts != nil {
			t.Errorf("unpinned facts must be unavailable: %+v", it)
		}
	}

	// Facts pinned to wrong database.
	bad := resolvedFacts(1, 11, rc)
	bad.DatabaseOID = rc.DatabaseOID + 1
	batch = EffectIdentityBatch{Items: []EffectIdentityItem{
		{Ordinal: 0, Status: domain.IdentityStatusResolved, Facts: bad},
		{Ordinal: 1, Status: domain.IdentityStatusResolved, Facts: resolvedFacts(2, 11, rc)},
	}}
	gated = GateIdentityBatchByResolutionContext(req, batch)
	if gated.Items[0].Status != domain.IdentityStatusUnavailable {
		t.Errorf("wrong DatabaseOID on facts: %+v", gated.Items[0])
	}
	if gated.Items[1].Status != domain.IdentityStatusResolved {
		t.Errorf("matching facts should remain: %+v", gated.Items[1])
	}

	// Facts pinned to wrong server version.
	badVer := resolvedFacts(2, 11, rc)
	badVer.ServerVersionNum = 150000
	batch = EffectIdentityBatch{Items: []EffectIdentityItem{
		{Ordinal: 0, Status: domain.IdentityStatusResolved, Facts: resolvedFacts(1, 11, rc)},
		{Ordinal: 1, Status: domain.IdentityStatusResolved, Facts: badVer},
	}}
	gated = GateIdentityBatchByResolutionContext(req, batch)
	if gated.Items[1].Status != domain.IdentityStatusUnavailable {
		t.Errorf("wrong ServerVersionNum on facts: %+v", gated.Items[1])
	}
}

func TestGate_SearchPathShadowing_UnqualifiedUsesBoundContextNotNameAllowlist(t *testing.T) {
	t.Parallel()
	rc := completeBoundContext() // public then pg_catalog
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{{
			Kind: EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}, IsAggregate: true,
		}},
		Resolution: rc,
	}
	customOID := uint32(900001)
	facts := resolvedFacts(customOID, 2200, rc)
	facts.Kind = EffectCandidateFunction
	facts.CanonicalSignature = "public.count(*)"
	batch := EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0, Status: domain.IdentityStatusResolved, Facts: facts,
	}}}
	gated := GateIdentityBatchByResolutionContext(req, batch)
	if gated.Items[0].Status != domain.IdentityStatusResolved {
		t.Fatalf("bound unqualified may resolve under path: %+v", gated.Items[0])
	}
	if gated.Items[0].Facts.ObjectOID != customOID || gated.Items[0].Facts.NamespaceOID != 2200 {
		t.Fatalf("must keep actual shadowed identity facts: %+v", gated.Items[0].Facts)
	}
}

func TestGate_OverloadAmbiguousFailClosed(t *testing.T) {
	t.Parallel()
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{{
			Kind: EffectCandidateOperator, Ordinal: 0, NamePath: []string{"+"}, Arity: 2,
		}},
		Resolution: completeBoundContext(),
	}
	for _, st := range []domain.IdentityStatus{
		domain.IdentityStatusAmbiguous,
		domain.IdentityStatusCoercionGap,
		domain.IdentityStatusUnknown,
	} {
		batch := EffectIdentityBatch{Items: []EffectIdentityItem{{Ordinal: 0, Status: st}}}
		gated := GateIdentityBatchByResolutionContext(req, batch)
		if gated.Items[0].Status != st || gated.Items[0].Facts != nil {
			t.Errorf("status %s: got %+v", st, gated.Items[0])
		}
		if !domain.IdentityStatusIsFailClosed(gated.Items[0].Status) {
			t.Errorf("%s must be fail-closed", st)
		}
	}
}

func TestGate_SameNameCustomOperator_NotTrustedByContract(t *testing.T) {
	t.Parallel()
	rc := completeBoundContext()
	rc.NamespaceSearchOIDs = nil
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{{
			Kind: EffectCandidateOperator, Ordinal: 0,
			NamePath: []string{"public", "="}, ExplicitSchema: true, Arity: 2,
		}},
		Resolution: rc,
	}
	facts := resolvedFacts(900002, 2200, rc)
	facts.Kind = EffectCandidateOperator
	batch := EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0, Status: domain.IdentityStatusResolved, Facts: facts,
	}}}
	gated := GateIdentityBatchByResolutionContext(req, batch)
	if gated.Items[0].Status != domain.IdentityStatusResolved {
		t.Fatalf("explicit public operator may resolve as facts: %+v", gated.Items[0])
	}
	if _, ok := reflect.TypeOf(*gated.Items[0].Facts).FieldByName("Trusted"); ok {
		t.Fatal("facts must not have Trusted")
	}
}

func TestGate_Live_RoleDatabaseVersionMismatchStripsAllIncludingExplicit(t *testing.T) {
	t.Parallel()
	rc := completeBoundContext()
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{
			{Kind: EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}},
			{Kind: EffectCandidateFunction, Ordinal: 1, NamePath: []string{"pg_catalog", "length"}, ExplicitSchema: true},
		},
		Resolution: rc,
	}
	batch := EffectIdentityBatch{Items: []EffectIdentityItem{
		{Ordinal: 0, Status: domain.IdentityStatusResolved, Facts: resolvedFacts(2803, 11, rc)},
		{Ordinal: 1, Status: domain.IdentityStatusResolved, Facts: resolvedFacts(1374, 11, rc)},
	}}

	mismatches := []struct {
		name string
		live EffectIdentityResolutionContext
	}{
		{"role", func() EffectIdentityResolutionContext {
			l := rc
			l.RoleOID = rc.RoleOID + 1
			return l
		}()},
		{"database", func() EffectIdentityResolutionContext {
			l := rc
			l.DatabaseOID = rc.DatabaseOID + 1
			return l
		}()},
		{"server_version", func() EffectIdentityResolutionContext {
			l := rc
			l.ServerVersionNum = 150000
			return l
		}()},
		{"epoch", func() EffectIdentityResolutionContext {
			l := rc
			l.PathEpoch = rc.PathEpoch + 1
			return l
		}()},
		{"session", func() EffectIdentityResolutionContext {
			l := rc
			l.SessionBinding = "other-sess"
			return l
		}()},
	}
	for _, tc := range mismatches {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			live := tc.live
			gated := GateIdentityBatchAgainstLiveContext(req, batch, func() (EffectIdentityResolutionContext, error) {
				return live, nil
			})
			for _, it := range gated.Items {
				if it.Status != domain.IdentityStatusUnavailable || it.Facts != nil {
					t.Errorf("%s mismatch must strip all candidates including explicit: %+v", tc.name, it)
				}
			}
		})
	}
}

func TestGate_Live_SearchPathOnlyMismatch_StripsUnqualifiedKeepsExplicit(t *testing.T) {
	t.Parallel()
	rc := completeBoundContext()
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{
			{Kind: EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}},
			{Kind: EffectCandidateFunction, Ordinal: 1, NamePath: []string{"pg_catalog", "length"}, ExplicitSchema: true},
		},
		Resolution: rc,
	}
	batch := EffectIdentityBatch{Items: []EffectIdentityItem{
		{Ordinal: 0, Status: domain.IdentityStatusResolved, Facts: resolvedFacts(2803, 11, rc)},
		{Ordinal: 1, Status: domain.IdentityStatusResolved, Facts: resolvedFacts(1374, 11, rc)},
	}}
	live := rc
	live.NamespaceSearchOIDs = []uint32{11, 2200} // reordered only
	gated := GateIdentityBatchAgainstLiveContext(req, batch, func() (EffectIdentityResolutionContext, error) {
		return live, nil
	})
	if gated.Items[0].Status != domain.IdentityStatusUnavailable || gated.Items[0].Facts != nil {
		t.Errorf("unqualified after path drift: %+v", gated.Items[0])
	}
	if gated.Items[1].Status != domain.IdentityStatusResolved || gated.Items[1].Facts == nil {
		t.Errorf("explicit may keep facts on path-only drift when session matches: %+v", gated.Items[1])
	}
}

func TestGate_Live_ErrorAndIncompleteLiveStripAll(t *testing.T) {
	t.Parallel()
	rc := completeBoundContext()
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{
			{Kind: EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}},
			{Kind: EffectCandidateFunction, Ordinal: 1, NamePath: []string{"pg_catalog", "count"}, ExplicitSchema: true},
		},
		Resolution: rc,
	}
	batch := EffectIdentityBatch{Items: []EffectIdentityItem{
		{Ordinal: 0, Status: domain.IdentityStatusResolved, Facts: resolvedFacts(1, 11, rc)},
		{Ordinal: 1, Status: domain.IdentityStatusResolved, Facts: resolvedFacts(2, 11, rc)},
	}}
	gated := GateIdentityBatchAgainstLiveContext(req, batch, func() (EffectIdentityResolutionContext, error) {
		return EffectIdentityResolutionContext{}, errors.New("connection reset")
	})
	for _, it := range gated.Items {
		if it.Status != domain.IdentityStatusUnavailable {
			t.Errorf("live error: %+v", it)
		}
	}
	// Incomplete live (missing role) strips all.
	gated = GateIdentityBatchAgainstLiveContext(req, batch, func() (EffectIdentityResolutionContext, error) {
		l := rc
		l.RoleOID = 0
		return l, nil
	})
	for _, it := range gated.Items {
		if it.Status != domain.IdentityStatusUnavailable || it.Facts != nil {
			t.Errorf("incomplete live: %+v", it)
		}
	}
}

func TestGate_CompatibleLiveKeepsResolved(t *testing.T) {
	t.Parallel()
	rc := completeBoundContext()
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{{
			Kind: EffectCandidateOperator, Ordinal: 0, NamePath: []string{"="}, Arity: 2,
		}},
		Resolution: rc,
	}
	batch := EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0, Status: domain.IdentityStatusResolved,
		Facts: resolvedFacts(96, 11, rc),
	}}}
	gated := GateIdentityBatchAgainstLiveContext(req, batch, func() (EffectIdentityResolutionContext, error) {
		return rc, nil
	})
	if gated.Items[0].Status != domain.IdentityStatusResolved || gated.Items[0].Facts == nil {
		t.Fatalf("compatible live must keep resolved: %+v", gated.Items[0])
	}
}

func TestResolutionContext_NotInPublicSurfaces(t *testing.T) {
	t.Parallel()
	if _, ok := reflect.TypeOf(domain.Result{}).FieldByName("Resolution"); ok {
		t.Fatal("domain.Result must not export Resolution")
	}
	if _, ok := reflect.TypeOf(domain.Result{}).FieldByName("SessionBinding"); ok {
		t.Fatal("domain.Result must not export SessionBinding")
	}
	if _, ok := reflect.TypeOf(QueryAccessRequest{}).FieldByName("Resolution"); ok {
		t.Fatal("QueryAccessRequest must not export Resolution in T6")
	}
	rc := completeBoundContext()
	data, err := json.Marshal(rc)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(data)
	for _, bad := range []string{
		"postgres://", "password", "search_path=", "severity",
		"SELECT ", "pg_operator",
	} {
		if strings.Contains(raw, bad) {
			t.Errorf("resolution context leaked %q: %s", bad, raw)
		}
	}
}

func TestFakeResolver_UnqualifiedRequiresCompleteContext(t *testing.T) {
	t.Parallel()
	fake := &contextAwareFakeResolver{}
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{
			{Kind: EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}},
			{Kind: EffectCandidateFunction, Ordinal: 1, NamePath: []string{"pg_catalog", "count"}, ExplicitSchema: true},
		},
	}
	batch, err := fake.ResolveEffectIdentities(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	batch = GateIdentityBatchByResolutionContext(req, batch)
	if batch.Items[0].Status != domain.IdentityStatusUnavailable {
		t.Errorf("unqualified unbound: %+v", batch.Items[0])
	}
	if batch.Items[1].Status != domain.IdentityStatusUnavailable {
		t.Errorf("explicit without session-complete must be unavailable: %+v", batch.Items[1])
	}

	req.Resolution = completeBoundContext()
	batch, err = fake.ResolveEffectIdentities(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	batch = GateIdentityBatchAgainstLiveContext(req, batch, func() (EffectIdentityResolutionContext, error) {
		return req.Resolution, nil
	})
	if batch.Items[0].Status != domain.IdentityStatusResolved {
		t.Errorf("unqualified complete+live should resolve: %+v", batch.Items[0])
	}
	if batch.Items[1].Status != domain.IdentityStatusResolved {
		t.Errorf("explicit complete+live should resolve: %+v", batch.Items[1])
	}
}

// contextAwareFakeResolver models a correct T7-shaped adapter: no pg_catalog
// name guess without complete session context; stamps db/version on facts.
type contextAwareFakeResolver struct{}

func (f *contextAwareFakeResolver) ResolveEffectIdentities(ctx context.Context, req EffectIdentityRequest) (EffectIdentityBatch, error) {
	if err := ctx.Err(); err != nil {
		return EffectIdentityBatch{}, err
	}
	if err := ValidateEffectIdentityRequest(req); err != nil {
		return EffectIdentityBatch{}, err
	}
	items := make([]EffectIdentityItem, 0, len(req.Candidates))
	for _, c := range req.Candidates {
		if !ResolutionContextSessionComplete(req.Resolution) {
			items = append(items, EffectIdentityItem{Ordinal: c.Ordinal, Status: domain.IdentityStatusUnavailable})
			continue
		}
		if !CandidateExplicitlyQualified(c) && !ResolutionContextUsableForUnqualified(req.Resolution) {
			items = append(items, EffectIdentityItem{Ordinal: c.Ordinal, Status: domain.IdentityStatusUnavailable})
			continue
		}
		name := ""
		if len(c.NamePath) > 0 {
			name = c.NamePath[len(c.NamePath)-1]
		}
		if name != "count" {
			items = append(items, EffectIdentityItem{Ordinal: c.Ordinal, Status: domain.IdentityStatusUnknown})
			continue
		}
		ns := uint32(11)
		if CandidateExplicitSchemaName(c) == "public" {
			ns = 2200
		}
		if !CandidateExplicitlyQualified(c) && len(req.Resolution.NamespaceSearchOIDs) > 0 &&
			req.Resolution.NamespaceSearchOIDs[0] == 2200 {
			ns = 2200
		}
		facts := &EffectIdentityFacts{Kind: c.Kind, ObjectOID: 1, NamespaceOID: ns}
		StampFactsFromResolution(facts, req.Resolution)
		items = append(items, EffectIdentityItem{
			Ordinal: c.Ordinal,
			Status:  domain.IdentityStatusResolved,
			Facts:   facts,
		})
	}
	return NormalizeEffectIdentityBatch(items), nil
}

var _ EffectIdentityResolver = (*contextAwareFakeResolver)(nil)

// Package queryaccess tests execution-bound resolution context policy (T6 P1).
// input: unqualified/explicit candidates, bound/unbound contexts, fake resolvers
// output: fail-closed statuses for shadowing, overload, mismatch; no public leak
// pos: contract safety for search_path / TOCTOU before T7 catalog adapter
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

func TestResolutionContextUsableAndCompatible(t *testing.T) {
	t.Parallel()
	unbound := EffectIdentityResolutionContext{}
	if ResolutionContextUsableForUnqualified(unbound) {
		t.Fatal("zero context must not be usable")
	}
	// Bound without binding id is unusable (and Validate rejects it on request).
	if ResolutionContextUsableForUnqualified(EffectIdentityResolutionContext{
		Bound: true, NamespaceSearchOIDs: []uint32{11},
	}) {
		t.Fatal("Bound without SessionBinding is unusable")
	}
	// Bound without namespace path is unusable for unqualified resolution.
	if ResolutionContextUsableForUnqualified(EffectIdentityResolutionContext{
		Bound: true, SessionBinding: "sess-1",
	}) {
		t.Fatal("bound without NamespaceSearchOIDs is unusable")
	}
	a := EffectIdentityResolutionContext{
		Bound: true, SessionBinding: "sess-1", PathEpoch: 3,
		NamespaceSearchOIDs: []uint32{2200, 11}, DatabaseOID: 5, RoleOID: 10,
	}
	if !ResolutionContextUsableForUnqualified(a) {
		t.Fatal("expected usable")
	}
	b := a
	if !ResolutionContextsCompatible(a, b) {
		t.Fatal("identical contexts must be compatible")
	}
	// search_path order change (shadowing risk) ⇒ incompatible.
	b.NamespaceSearchOIDs = []uint32{11, 2200}
	if ResolutionContextsCompatible(a, b) {
		t.Fatal("reordered search_path must not be compatible")
	}
	// Epoch bump (TOCTOU) ⇒ incompatible.
	b = a
	b.PathEpoch = 4
	if ResolutionContextsCompatible(a, b) {
		t.Fatal("epoch mismatch must not be compatible")
	}
	// Session change ⇒ incompatible.
	b = a
	b.SessionBinding = "sess-2"
	if ResolutionContextsCompatible(a, b) {
		t.Fatal("session mismatch must not be compatible")
	}
	// Zero database on one side is not asserted.
	b = a
	b.DatabaseOID = 0
	if !ResolutionContextsCompatible(a, b) {
		t.Fatal("zero DatabaseOID is optional")
	}
	b = a
	b.DatabaseOID = 99
	if ResolutionContextsCompatible(a, b) {
		t.Fatal("database mismatch must not be compatible")
	}
}

func TestValidateEffectIdentityRequest_BoundRequiresSessionBinding(t *testing.T) {
	t.Parallel()
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{{Kind: EffectCandidateOperator, Ordinal: 0, NamePath: []string{"="}}},
		Resolution: EffectIdentityResolutionContext{
			Bound: true, NamespaceSearchOIDs: []uint32{11},
		},
	}
	if err := ValidateEffectIdentityRequest(req); !errors.Is(err, ErrIdentityRequestInvalid) {
		t.Fatalf("want invalid bound context, got %v", err)
	}
	req.Resolution.SessionBinding = "sess"
	if err := ValidateEffectIdentityRequest(req); err != nil {
		t.Fatalf("valid bound: %v", err)
	}
}

func TestGate_UnqualifiedWithoutContextUnavailable_NotPgCatalogGuess(t *testing.T) {
	t.Parallel()
	// Classic production SQL shapes that are unqualified:
	//   SELECT count(*) FROM t
	//   SELECT * FROM t WHERE id = 1
	req := EffectIdentityRequest{
		Dialect: "postgresql",
		Candidates: []EffectCandidate{
			{Kind: EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}, IsAggregate: true},
			{Kind: EffectCandidateOperator, Ordinal: 1, NamePath: []string{"="}, Arity: 2},
		},
		// Resolution zero = unbound
	}
	// Malicious/naive adapter that guesses pg_catalog from name spelling.
	guessed := EffectIdentityBatch{Items: []EffectIdentityItem{
		{
			Ordinal: 0, Status: domain.IdentityStatusResolved,
			Facts: &EffectIdentityFacts{
				Kind: EffectCandidateFunction, ObjectOID: 2803, NamespaceOID: 11,
				CanonicalSignature: "pg_catalog.count(*)",
			},
		},
		{
			Ordinal: 1, Status: domain.IdentityStatusResolved,
			Facts: &EffectIdentityFacts{
				Kind: EffectCandidateOperator, ObjectOID: 96, NamespaceOID: 11,
				CanonicalSignature: "pg_catalog.=(int4,int4)",
			},
		},
	}}
	gated := GateIdentityBatchByResolutionContext(req, guessed)
	if len(gated.Items) != 2 {
		t.Fatalf("len=%d", len(gated.Items))
	}
	for _, it := range gated.Items {
		if it.Status != domain.IdentityStatusUnavailable || it.Facts != nil {
			t.Errorf("unqualified+unbound must be unavailable without facts, got %+v", it)
		}
		if ClassifyCandidateResolutionMode(req.Candidates[it.Ordinal], req.Resolution) != ResolutionModeUnqualifiedUnbound {
			t.Errorf("mode for ordinal %d", it.Ordinal)
		}
	}
	// Fail-closed reasons are identity_resolver_unavailable (not free text).
	codes := FailClosedReasonCodes(gated)
	if len(codes) != 1 || codes[0] != domain.ReasonIdentityResolverUnavailable {
		t.Errorf("codes=%v", codes)
	}
}

func TestGate_ExplicitPgCatalogMayKeepResolvedWithoutSearchPath(t *testing.T) {
	t.Parallel()
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{{
			Kind: EffectCandidateFunction, Ordinal: 0,
			NamePath: []string{"pg_catalog", "count"}, ExplicitSchema: true, IsAggregate: true,
		}},
		// Unbound context is OK for explicit schema (no search_path ranking needed).
	}
	batch := EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0, Status: domain.IdentityStatusResolved,
		Facts: &EffectIdentityFacts{ObjectOID: 2803, NamespaceOID: 11},
	}}}
	gated := GateIdentityBatchByResolutionContext(req, batch)
	if gated.Items[0].Status != domain.IdentityStatusResolved || gated.Items[0].Facts == nil {
		t.Fatalf("explicit pg_catalog must not be stripped solely for unbound context: %+v", gated.Items[0])
	}
	if ClassifyCandidateResolutionMode(req.Candidates[0], req.Resolution) != ResolutionModeExplicitSchema {
		t.Fatal("expected explicit_schema mode")
	}
}

func TestGate_SearchPathShadowing_UnqualifiedUsesBoundContextNotNameAllowlist(t *testing.T) {
	t.Parallel()
	// search_path = public, pg_catalog with a custom public.count shadowing builtin.
	// Bound context allows resolution to the *actual* identity (public OID).
	// Trust is still T8 — here we only assert facts-only status handling.
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{{
			Kind: EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}, IsAggregate: true,
		}},
		Resolution: EffectIdentityResolutionContext{
			Bound: true, SessionBinding: "sess-shadow", PathEpoch: 1,
			NamespaceSearchOIDs: []uint32{2200 /*public*/, 11 /*pg_catalog*/},
		},
	}
	// Correct adapter: resolves to public custom function under path order.
	customOID := uint32(900001)
	batch := EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0, Status: domain.IdentityStatusResolved,
		Facts: &EffectIdentityFacts{
			Kind: EffectCandidateFunction, ObjectOID: customOID, NamespaceOID: 2200,
			CanonicalSignature: "public.count(*)",
		},
	}}}
	gated := GateIdentityBatchByResolutionContext(req, batch)
	if gated.Items[0].Status != domain.IdentityStatusResolved {
		t.Fatalf("bound unqualified may resolve under path: %+v", gated.Items[0])
	}
	if gated.Items[0].Facts.ObjectOID != customOID || gated.Items[0].Facts.NamespaceOID != 2200 {
		t.Fatalf("must keep actual shadowed identity facts, not force pg_catalog: %+v", gated.Items[0].Facts)
	}
	// Incorrect name-allowlist adapter that always returns pg_catalog.count is still
	// "allowed" as facts by the gate when bound (adapter bug); T8 must not trust
	// without matching real execution. Document: gate does not re-query catalog.
	// Shadowing safety is: unbound forbids guess; bound requires adapter honesty
	// + TOCTOU live check. See TestGate_TOCTOU_LiveMismatch.
}

func TestGate_OverloadAmbiguousFailClosed(t *testing.T) {
	t.Parallel()
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{{
			Kind: EffectCandidateOperator, Ordinal: 0, NamePath: []string{"+"}, Arity: 2,
		}},
		Resolution: EffectIdentityResolutionContext{
			Bound: true, SessionBinding: "sess", PathEpoch: 1,
			NamespaceSearchOIDs: []uint32{11},
		},
		// Missing operand types ⇒ adapter reports ambiguous/coercion_gap.
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
	// OPERATOR(public.=) or unqualified = resolving to a custom operator OID.
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{{
			Kind: EffectCandidateOperator, Ordinal: 0,
			NamePath: []string{"public", "="}, ExplicitSchema: true, Arity: 2,
		}},
	}
	batch := EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0, Status: domain.IdentityStatusResolved,
		Facts: &EffectIdentityFacts{
			ObjectOID: 900002, NamespaceOID: 2200,
			CanonicalSignature: "public.=(int4,int4)",
		},
	}}}
	gated := GateIdentityBatchByResolutionContext(req, batch)
	// Contract returns facts only — no Trusted field; promotion is T8.
	if gated.Items[0].Status != domain.IdentityStatusResolved {
		t.Fatalf("explicit public operator may resolve as facts: %+v", gated.Items[0])
	}
	if _, ok := reflect.TypeOf(*gated.Items[0].Facts).FieldByName("Trusted"); ok {
		t.Fatal("facts must not have Trusted")
	}
	// Unqualified same spelling without context stays unavailable (no allowlist).
	req2 := EffectIdentityRequest{
		Candidates: []EffectCandidate{{
			Kind: EffectCandidateOperator, Ordinal: 0, NamePath: []string{"="}, Arity: 2,
		}},
	}
	guess := EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0, Status: domain.IdentityStatusResolved,
		Facts: &EffectIdentityFacts{ObjectOID: 96, NamespaceOID: 11},
	}}}
	g2 := GateIdentityBatchByResolutionContext(req2, guess)
	if g2.Items[0].Status != domain.IdentityStatusUnavailable {
		t.Fatalf("unqualified custom/builtin = without context: %+v", g2.Items[0])
	}
}

func TestGate_TOCTOU_LiveMismatchStripsUnqualified(t *testing.T) {
	t.Parallel()
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{
			{Kind: EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}},
			{Kind: EffectCandidateFunction, Ordinal: 1, NamePath: []string{"pg_catalog", "length"}, ExplicitSchema: true},
		},
		Resolution: EffectIdentityResolutionContext{
			Bound: true, SessionBinding: "sess", PathEpoch: 1,
			NamespaceSearchOIDs: []uint32{11},
		},
	}
	batch := EffectIdentityBatch{Items: []EffectIdentityItem{
		{Ordinal: 0, Status: domain.IdentityStatusResolved, Facts: &EffectIdentityFacts{ObjectOID: 2803, NamespaceOID: 11}},
		{Ordinal: 1, Status: domain.IdentityStatusResolved, Facts: &EffectIdentityFacts{ObjectOID: 1374, NamespaceOID: 11}},
	}}
	// Live path reordered / epoch advanced mid-flight.
	live := func() (EffectIdentityResolutionContext, error) {
		return EffectIdentityResolutionContext{
			Bound: true, SessionBinding: "sess", PathEpoch: 2, // epoch drift
			NamespaceSearchOIDs: []uint32{11},
		}, nil
	}
	gated := GateIdentityBatchAgainstLiveContext(req, batch, live)
	if gated.Items[0].Status != domain.IdentityStatusUnavailable || gated.Items[0].Facts != nil {
		t.Errorf("unqualified after TOCTOU must be unavailable: %+v", gated.Items[0])
	}
	// Explicit schema survives TOCTOU path check (does not depend on search_path rank).
	if gated.Items[1].Status != domain.IdentityStatusResolved || gated.Items[1].Facts == nil {
		t.Errorf("explicit pg_catalog should keep facts on path epoch drift: %+v", gated.Items[1])
	}

	// Live read error also fail-closes unqualified.
	liveErr := func() (EffectIdentityResolutionContext, error) {
		return EffectIdentityResolutionContext{}, errors.New("connection reset mid batch")
	}
	gated2 := GateIdentityBatchAgainstLiveContext(req, batch, liveErr)
	if gated2.Items[0].Status != domain.IdentityStatusUnavailable {
		t.Errorf("live error: %+v", gated2.Items[0])
	}
}

func TestGate_CompatibleLiveKeepsUnqualifiedResolved(t *testing.T) {
	t.Parallel()
	rc := EffectIdentityResolutionContext{
		Bound: true, SessionBinding: "sess", PathEpoch: 7,
		NamespaceSearchOIDs: []uint32{11},
	}
	req := EffectIdentityRequest{
		Candidates: []EffectCandidate{{
			Kind: EffectCandidateOperator, Ordinal: 0, NamePath: []string{"="}, Arity: 2,
		}},
		Resolution: rc,
	}
	batch := EffectIdentityBatch{Items: []EffectIdentityItem{{
		Ordinal: 0, Status: domain.IdentityStatusResolved,
		Facts: &EffectIdentityFacts{ObjectOID: 96, NamespaceOID: 11},
	}}}
	live := func() (EffectIdentityResolutionContext, error) { return rc, nil }
	gated := GateIdentityBatchAgainstLiveContext(req, batch, live)
	if gated.Items[0].Status != domain.IdentityStatusResolved || gated.Items[0].Facts == nil {
		t.Fatalf("compatible live must keep resolved: %+v", gated.Items[0])
	}
}

func TestResolutionContext_NotInPublicSurfaces(t *testing.T) {
	t.Parallel()
	// Request type may hold Resolution internally; domain Result / public SDK request must not.
	if _, ok := reflect.TypeOf(domain.Result{}).FieldByName("Resolution"); ok {
		t.Fatal("domain.Result must not export Resolution")
	}
	if _, ok := reflect.TypeOf(domain.Result{}).FieldByName("SessionBinding"); ok {
		t.Fatal("domain.Result must not export SessionBinding")
	}
	if _, ok := reflect.TypeOf(QueryAccessRequest{}).FieldByName("Resolution"); ok {
		t.Fatal("QueryAccessRequest must not export Resolution in T6")
	}
	// Context JSON must not be a public contract field; when marshaled as internal
	// helper, still no credential-shaped fields.
	rc := EffectIdentityResolutionContext{
		Bound: true, SessionBinding: "opaque-id", PathEpoch: 1,
		NamespaceSearchOIDs: []uint32{11},
	}
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

func TestFakeResolver_UnqualifiedRequiresContext(t *testing.T) {
	t.Parallel()
	// Contract-level fake that refuses to resolve unqualified without context
	// (correct T7 shape) rather than guessing pg_catalog.
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
	if batch.Items[1].Status != domain.IdentityStatusResolved {
		t.Errorf("explicit pg_catalog: %+v", batch.Items[1])
	}

	req.Resolution = EffectIdentityResolutionContext{
		Bound: true, SessionBinding: "s", PathEpoch: 1,
		NamespaceSearchOIDs: []uint32{11},
	}
	batch, err = fake.ResolveEffectIdentities(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	batch = GateIdentityBatchAgainstLiveContext(req, batch, func() (EffectIdentityResolutionContext, error) {
		return req.Resolution, nil
	})
	if batch.Items[0].Status != domain.IdentityStatusResolved {
		t.Errorf("unqualified bound should resolve under controlled path: %+v", batch.Items[0])
	}
}

// contextAwareFakeResolver models a correct T7-shaped adapter: no pg_catalog
// name guess without resolution context; multi-match → ambiguous.
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
		mode := ClassifyCandidateResolutionMode(c, req.Resolution)
		switch mode {
		case ResolutionModeUnqualifiedUnbound:
			items = append(items, EffectIdentityItem{Ordinal: c.Ordinal, Status: domain.IdentityStatusUnavailable})
		case ResolutionModeUnqualifiedBound, ResolutionModeExplicitSchema:
			// Simulate unique hit for count / pg_catalog.count only.
			name := ""
			if len(c.NamePath) > 0 {
				name = c.NamePath[len(c.NamePath)-1]
			}
			if name == "count" {
				ns := uint32(11)
				if CandidateExplicitSchemaName(c) == "public" {
					ns = 2200
				}
				if mode == ResolutionModeUnqualifiedBound && len(req.Resolution.NamespaceSearchOIDs) > 0 &&
					req.Resolution.NamespaceSearchOIDs[0] == 2200 {
					// Path-first public shadow.
					ns = 2200
				}
				items = append(items, EffectIdentityItem{
					Ordinal: c.Ordinal,
					Status:  domain.IdentityStatusResolved,
					Facts:   &EffectIdentityFacts{Kind: c.Kind, ObjectOID: 1, NamespaceOID: ns},
				})
				continue
			}
			items = append(items, EffectIdentityItem{Ordinal: c.Ordinal, Status: domain.IdentityStatusUnknown})
		default:
			items = append(items, EffectIdentityItem{Ordinal: c.Ordinal, Status: domain.IdentityStatusUnavailable})
		}
	}
	return NormalizeEffectIdentityBatch(items), nil
}

var _ EffectIdentityResolver = (*contextAwareFakeResolver)(nil)

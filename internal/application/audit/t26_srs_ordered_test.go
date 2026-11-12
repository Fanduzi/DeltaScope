package audit_test

// T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01 — the ordered
// publication channel's real-product-chain rows (OP-1/OP-2/OP-4/
// OP-5/OP-6/OP-8 witnesses over the r3 stub driver → mysqlmeta.NewProvider
// supply chain). Every vector is measured at the independent seam
// counters: R counts ResolveObject forwards, Qexist/Qdeps/Qo count
// issued catalog queries (attempts included), L counts privilege
// loads; an ordered hit issues Qo+Qdeps while skipping R/Qexist, and
// a warm objectCache hit issues nothing at all.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	audit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// opNoCapProvider is the OP-2g/OP-2h thin adapter — it exposes the
// full ObjectResolver and the privilege loader but deliberately NOT
// MySQLSRSDependencyReader, so the ordered capability check on the
// provider itself must fail and every statement takes the existing
// full path.
type opNoCapProvider struct{ inner *r3ProductProvider }

func (p *opNoCapProvider) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	return p.inner.LoadInstanceFacts(ctx, dialect, schema)
}
func (p *opNoCapProvider) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	return p.inner.LoadTableSnapshot(ctx, dialect, schema, table)
}
func (p *opNoCapProvider) ResolveObject(ctx context.Context, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return p.inner.ResolveObject(ctx, dialect, req)
}
func (p *opNoCapProvider) LoadMySQLServerDropPrivilege(ctx context.Context, dialect spec.Dialect) (*spec.MySQLServerDropPrivilegeObservation, error) {
	return p.inner.LoadMySQLServerDropPrivilege(ctx, dialect)
}

func opSRS(t *testing.T, stmt spec.Statement) *spec.SpatialReferenceSystemState {
	t.Helper()
	if stmt.Metadata == nil || stmt.Metadata.SpatialReference == nil {
		t.Fatalf("no bound SRS state: %+v", stmt.Metadata)
	}
	return stmt.Metadata.SpatialReference
}

func opEntry(t *testing.T, state *audit.TestBatchState, srid uint32) audit.SRSOrderedEntryView {
	t.Helper()
	view, ok := state.SRSOrderedEntry(srid)
	if !ok {
		t.Fatalf("no ordered entry for SRID %d", srid)
	}
	return view
}

var opGrantConsumable = []string{"GRANT SELECT, SUPER ON *.* TO `app`@`%`"}
var opGrantNegative = []string{"GRANT SELECT ON *.* TO 'app'@'%'"}

func opAbsentFixture(grants []string) *r3Fixture {
	return &r3Fixture{version: "8.0.36", account: "app@%", grants: grants}
}

// --- OP-2a/OP-2c: the real ordered hit ---------------------------------

// OP-2a: stmt1's catalog-certified A-plain mints the entry; stmt2
// consumes it — marker bound, arm stays absent_refused, Qexist never
// issued on the hit while Qo+Qdeps run fresh against the real
// provider.
func TestT26SRSOrderedHitPlainRealChain(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")

	s1, s2 := enriched[0], enriched[1]
	if arm := paAdj(t, s1).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("stmt1 arm=%v want absent_refused", arm)
	}
	if opSRS(t, s1).OrderedAbsent {
		t.Fatal("stmt1 is catalog-served — OrderedAbsent must stay false")
	}
	if arm := paAdj(t, s2).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("stmt2 arm=%v want absent_refused — ordered absence consumed", arm)
	}
	bound2 := opSRS(t, s2)
	if !bound2.OrderedAbsent {
		t.Fatal("stmt2 bound state must carry OrderedAbsent")
	}
	if bound2.Read || bound2.Exists || bound2.Absence || bound2.SRID != 100001 {
		t.Fatalf("ordered bind must not fabricate catalog facts: %+v", bound2)
	}
	if bound2.Dependencies == nil || len(bound2.Dependencies.Columns) != 0 || bound2.Dependencies.Complete {
		t.Fatalf("ordered hit must carry the fresh deps fact: %+v", bound2.Dependencies)
	}
	// Measured vector: one R forward + one Qexist (stmt1 only), two
	// Qdeps + two Qo (stmt1 full pass + stmt2 direct read).
	r3AssertReads(t, provider, fx, 1, 1, 2, 2, 1, -1)

	entry := opEntry(t, state, 100001)
	if !entry.Absent || entry.SourceArm != spec.SRSDropArmAbsentRefused ||
		entry.OwnPrivilege != spec.SRSDropPrivilegeConsumable ||
		entry.PublishedIdx != 0 || entry.Barrier {
		t.Fatalf("entry=%+v want {absent, absent_refused, consumable, idx0, no barrier}", entry)
	}
	if len(state.SRSOrderedSRIDs()) != 1 {
		t.Fatalf("ordered channel keys=%v want exactly {100001}", state.SRSOrderedSRIDs())
	}
}

// OP-2c: the same hit under IF EXISTS — absent_noop on both, the
// ordered-served statement still discharges its own consumable
// premise.
func TestT26SRSOrderedHitIfExistsRealChain(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentNoop {
		t.Fatalf("stmt1 arm=%v want absent_noop", arm)
	}
	adj2 := paAdj(t, enriched[1])
	if adj2.Arm != spec.SRSDropArmAbsentNoop {
		t.Fatalf("stmt2 arm=%v want absent_noop — the ordered A arm runs the same IE×privilege table", adj2.Arm)
	}
	if adj2.Privilege != spec.SRSDropPrivilegeConsumable || len(adj2.Owed) != 0 {
		t.Fatalf("stmt2 adj=%+v want consumable premise discharged on the ordered A arm", adj2)
	}
	if !opSRS(t, enriched[1]).OrderedAbsent {
		t.Fatal("stmt2 must be ordered-served")
	}
	entry := opEntry(t, state, 100001)
	if entry.SourceArm != spec.SRSDropArmAbsentNoop {
		t.Fatalf("entry sourceArm=%v want absent_noop — first producer wins", entry.SourceArm)
	}
	r3AssertReads(t, provider, fx, 1, 1, 2, 2, 1, -1)
}

// --- OP-1: non-producer arms and foreign-SRID control (real chain) -----

// OP-1: an S2 unproven statement and an S1 refused-dependency
// statement never mint an ordered entry; a foreign-SRID drop mints
// its own entry and leaves siblings untouched.
func TestT26SRSOrderedNonProducersRealChain(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.srs = map[uint32]string{100001: "SRID 100001"}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmUnproven {
		t.Fatalf("arm=%v want unproven — existence proven present", arm)
	}
	if _, ok := state.SRSOrderedEntry(100001); ok {
		t.Fatal("an S2 statement must never mint an ordered entry")
	}
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{100001: true}, "S2 minted stale")

	// S1: positive dependency witness → refused, no mint.
	fx2 := opAbsentFixture(opGrantConsumable)
	fx2.deps = []r3DepRow{{schema: "s", table: "t", column: "g", srid: 100001}}
	provider2, state2 := paRealState(t, fx2)
	enriched2 := audit.EnrichSRSPrivForTest(t, state2, provider2,
		"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, enriched2[0]).Arm; arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("arm=%v want refused_dependency", arm)
	}
	if _, ok := state2.SRSOrderedEntry(100001); ok {
		t.Fatal("an S1 statement must never mint an ordered entry")
	}

	// Foreign SRID: DROP 200002 mints its own entry keyed by its own
	// authored SRID; 100001's channel stays untouched.
	fx3 := opAbsentFixture(opGrantConsumable)
	provider3, state3 := paRealState(t, fx3)
	enriched3 := audit.EnrichSRSPrivForTest(t, state3, provider3,
		"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 200002;")
	if len(state3.SRSOrderedSRIDs()) != 2 {
		t.Fatalf("ordered keys=%v want {100001,200002}", state3.SRSOrderedSRIDs())
	}
	e2 := opEntry(t, state3, 200002)
	if e2.PublishedIdx != 1 || !e2.Absent {
		t.Fatalf("foreign entry=%+v want {absent, publishedIdx=1}", e2)
	}
	if opSRS(t, enriched3[1]).OrderedAbsent {
		t.Fatal("the foreign-SRID statement is a producer, not a consumer")
	}
}

// --- OP-2d/OP-2e: shared vs new batchState -----------------------------

// OP-2d: a second enrich on the SAME batchState consumes the ordered
// entry — the now-present catalog answer is deliberately invisible to
// the ordered channel (Qo+Qdeps still run fresh per hit).
// OP-2e: a NEW batchState sees no entry and takes the full path to
// the changed answer (S2 + stale mint). Identical fixture, negative
// grant, unproven premise — only batchState lifetime differs.
func TestT26SRSOrderedSharedVsNewStateRealChain(t *testing.T) {
	fx := opAbsentFixture(opGrantNegative)

	provider, state := paRealState(t, fx)
	e1 := audit.EnrichSRSPrivForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, e1[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("enrich1 arm=%v want absent_refused", arm)
	}
	r3AssertReads(t, provider, fx, 1, 1, 1, 1, 1, -1)

	fx.srs = map[uint32]string{100001: "SRID 100001"} // the catalog answer changes
	e2 := audit.EnrichSRSPrivForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	adj := paAdj(t, e2[0])
	if adj.Arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("OP-2d arm=%v want absent_refused — the ordered entry outranks the changed catalog answer", adj.Arm)
	}
	if !opSRS(t, e2[0]).OrderedAbsent {
		t.Fatal("OP-2d must be ordered-served")
	}
	// The ordered hit: Qo+Qdeps only — no R forward, no Qexist.
	if fx.count["exist"] != 1 || fx.count["deps"] != 2 || fx.count["ver"] != 2 {
		t.Fatalf("OP-2d totals exist/deps/ver=%d/%d/%d want 1/2/2",
			fx.count["exist"], fx.count["deps"], fx.count["ver"])
	}
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{}, "OP-2d — ordered A mints no stale")
	if entry := opEntry(t, state, 100001); entry.Barrier || entry.PublishedIdx != 0 {
		t.Fatalf("entry=%+v want first-producer provenance intact", entry)
	}

	// OP-2e: identical fixture shape on a NEW batchState — no entry,
	// cold cache → real full read sees the present answer → S2.
	fxE := opAbsentFixture(opGrantNegative)
	fxE.srs = map[uint32]string{100001: "SRID 100001"}
	providerE, stateE := paRealState(t, fxE)
	e3 := audit.EnrichSRSPrivForTest(t, stateE, providerE, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	adj3 := paAdj(t, e3[0])
	if adj3.Arm != spec.SRSDropArmUnproven {
		t.Fatalf("OP-2e arm=%v want unproven — the new batchState never saw the entry", adj3.Arm)
	}
	if opSRS(t, e3[0]).OrderedAbsent {
		t.Fatal("OP-2e must not carry the ordered marker")
	}
	if _, ok := stateE.SRSOrderedEntry(100001); ok {
		t.Fatal("a new batchState has no ordered channel state to consume")
	}
	paSameStale(t, audit.BatchStateStaleSet(stateE), map[uint32]bool{100001: true}, "OP-2e S2 minted")
	r3AssertReads(t, providerE, fxE, 1, 1, 1, 1, 1, -1)
}

// --- OP-2g/OP-2h: active entry + no capability --------------------------

// OP-2g: warm-cache no-capability — the thin adapter supports the full
// resolver and the privilege loader but not the deps-reader method.
// stmt1 mints through the full path; stmt2's ordered lookup cannot
// commit (no capability → the entry is NOT consumed), so the full
// path's per-enrich cache serves the identical key: (1,1,1,1) then
// (0,0,0,0) with cache-hit=1 — the second statement performs ZERO
// provider reads of any kind.
func TestT26SRSOrderedNoCapabilityWarmCache(t *testing.T) {
	fx := opAbsentFixture(opGrantNegative)
	inner := r3Harness(t, fx)
	provider := &opNoCapProvider{inner: inner}
	state := audit.NewTestBatchState(provider)

	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("stmt1 arm=%v want absent_refused", arm)
	}
	bound2 := opSRS(t, enriched[1])
	if bound2.OrderedAbsent {
		t.Fatal("OP-2g: no capability → OrderedAbsent must stay false")
	}
	if !bound2.Read || bound2.Exists || !bound2.Absence {
		t.Fatalf("OP-2g stmt2 bound=%+v want the cached catalog answer (read+absent)", bound2)
	}
	if arm := paAdj(t, enriched[1]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("stmt2 arm=%v want absent_refused via the cached catalog answer", arm)
	}
	// One entry, first provenance unchanged, no stale/barrier.
	if ids := state.SRSOrderedSRIDs(); len(ids) != 1 {
		t.Fatalf("ordered keys=%v want exactly {100001}", ids)
	}
	entry := opEntry(t, state, 100001)
	if entry.Barrier || entry.PublishedIdx != 0 || entry.SourceArm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("entry=%+v want unchanged first provenance", entry)
	}
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{}, "no stale mint")
	// Vectors: stmt1 (1,1,1,1); stmt2 (0,0,0,0) cache-hit — the
	// objectCache serves the identical key with zero forwards.
	r3AssertReads(t, inner, fx, 1, 1, 1, 1, 1, -1)
}

// OP-2h: cold-cache no-capability — same batchState, new objectCache
// per enrich. The second enrich performs the real full lookup, sees
// the changed answer, lands S2, and the SAME entry flips
// barrier=false→true with the stale mint — first provenance kept.
func TestT26SRSOrderedNoCapabilityColdCache(t *testing.T) {
	fx := opAbsentFixture(opGrantNegative)
	inner := r3Harness(t, fx)
	provider := &opNoCapProvider{inner: inner}
	state := audit.NewTestBatchState(provider)

	e1 := audit.EnrichSRSPrivForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, e1[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("enrich1 arm=%v want absent_refused", arm)
	}
	before := opEntry(t, state, 100001)

	fx.srs = map[uint32]string{100001: "SRID 100001"}
	e2 := audit.EnrichSRSPrivForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	adj2 := paAdj(t, e2[0])
	if adj2.Arm != spec.SRSDropArmUnproven {
		t.Fatalf("OP-2h arm=%v want unproven — the cold full read saw the present answer", adj2.Arm)
	}
	if opSRS(t, e2[0]).OrderedAbsent {
		t.Fatal("OP-2h: no capability → OrderedAbsent must stay false")
	}
	wantOwed := []string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}
	if len(adj2.Owed) != len(wantOwed) {
		t.Fatalf("OP-2h owed=%v want exactly %v", adj2.Owed, wantOwed)
	}
	for i := range wantOwed {
		if adj2.Owed[i] != wantOwed[i] {
			t.Fatalf("OP-2h owed[%d]=%q want %q", i, adj2.Owed[i], wantOwed[i])
		}
	}
	after := opEntry(t, state, 100001)
	if !after.Barrier {
		t.Fatal("the same entry's barrier must flip false→true")
	}
	if after.SourceArm != before.SourceArm || after.OwnPrivilege != before.OwnPrivilege ||
		after.PublishedIdx != before.PublishedIdx || !after.Absent {
		t.Fatalf("first provenance must survive: %+v vs %+v", before, after)
	}
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{100001: true}, "barrier ⇒ stale mint")
	// Each enrich performed its own real full read: (1,1,1,1) ×2.
	r3AssertReads(t, inner, fx, 2, 2, 2, 2, 2, -1)
}

// --- OP-4: per-hit freshness and errors ---------------------------------

// OP-4a: three ordered hits across one batch perform a FRESH deps
// read each — answers empty → empty → positive land A, A, S1.
func TestT26SRSOrderedFreshDepsRealChain(t *testing.T) {
	positive := []r3DepRow{{schema: "s", table: "t", column: "g", srid: 100001}}
	fx := opAbsentFixture(opGrantConsumable)
	fx.depsScript = [][]r3DepRow{nil, nil, nil, positive}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	wantArms := []spec.SRSDropArm{
		spec.SRSDropArmAbsentRefused, spec.SRSDropArmAbsentRefused,
		spec.SRSDropArmAbsentRefused, spec.SRSDropArmRefusedDependency,
	}
	for i, stmt := range enriched {
		if arm := paAdj(t, stmt).Arm; arm != wantArms[i] {
			t.Fatalf("stmt%d arm=%v want %v", i, arm, wantArms[i])
		}
		if i > 0 && !opSRS(t, stmt).OrderedAbsent {
			t.Fatalf("stmt%d must be ordered-served", i)
		}
	}
	// Four deps attempts total (stmt1 full pass + 3 direct reads) —
	// the script proves each hit read fresh.
	if fx.depsCalls != 4 || fx.count["deps"] != 4 {
		t.Fatalf("deps attempts=%d want 4 — every hit reads fresh", fx.count["deps"])
	}
	if fx.count["exist"] != 1 || fx.count["ver"] != 4 {
		t.Fatalf("exist/Qo=%d/%d want 1/4", fx.count["exist"], fx.count["ver"])
	}
}

// OP-4b: a positive dependency witness under srsDepsStale is demoted
// — the ordered A arm still governs (staleness demotes witness use
// only, never the proven absence entry).
func TestT26SRSOrderedDepsStaleDemotesRealChain(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.deps = []r3DepRow{{schema: "s", table: "t", column: "g", srid: 100001}}
	provider, state := paRealState(t, fx)
	state.SetSRSDepsStale(true)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("stmt1 arm=%v want absent_refused — demoted witness, absence governs", arm)
	}
	adj2 := paAdj(t, enriched[1])
	if adj2.Arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("stmt2 arm=%v want absent_refused — stale deps demote S1, ordered A still holds", adj2.Arm)
	}
	if !opSRS(t, enriched[1]).OrderedAbsent {
		t.Fatal("stmt2 must be ordered-served")
	}
	if !state.SRSDepsStale() {
		t.Fatal("depsStale must persist")
	}
}

// OP-4c: an ordered-hit deps-reader error propagates — attempts are
// counted, errors.Is reaches the injected sentinel, no adjudication
// or apply runs on the failing statement, and the prior bound
// snapshot + entry stay unchanged.
func TestT26SRSOrderedDepsErrorRealChain(t *testing.T) {
	sentinel := errors.New("op ordered deps sentinel")
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	e1 := audit.EnrichSRSPrivForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, e1[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("enrich1 arm=%v want absent_refused", arm)
	}
	snap1 := paSnapStmt(e1[0])
	entryBefore := opEntry(t, state, 100001)

	fx.failOn = map[string]error{"deps": sentinel}
	_, err := audit.EnrichSRSPrivErrForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if !errors.Is(err, sentinel) {
		t.Fatalf("err=%v want errors.Is(sentinel)", err)
	}
	if fx.count["deps"] != 2 {
		t.Fatalf("deps attempts=%d want 2 — the ordered read attempt counted", fx.count["deps"])
	}
	if fx.count["exist"] != 1 {
		t.Fatalf("exist=%d want 1 — the failing hit never re-read existence", fx.count["exist"])
	}
	paSnapEqual(t, "stmt1 snapshot after failed enrich", snap1, paSnapStmt(e1[0]))
	entryAfter := opEntry(t, state, 100001)
	if entryAfter != entryBefore {
		t.Fatalf("entry changed across the failed hit: %+v vs %+v", entryBefore, entryAfter)
	}
	if entryAfter.Barrier {
		t.Fatal("a deps error must not flip the barrier")
	}
}

// --- OP-5: invalidation -------------------------------------------------

// OP-5: a same-SRID boundary write shadows the entry (barrier + stale,
// provenance kept); an inert op and a REVOKE leave the entry
// consumable; a touching DROP TABLE mints depsStale which demotes the
// witness only.
func TestT26SRSOrderedInvalidationRealChain(t *testing.T) {
	t.Run("OP-5a-boundary-barrier", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		provider, state := paRealState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM 100001;"+
				"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 DEFINITION 'x';"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		if arm := paAdj(t, enriched[2]).Arm; arm != spec.SRSDropArmBlocked {
			t.Fatalf("stmt3 arm=%v want blocked — the boundary write shadowed the entry", arm)
		}
		entry := opEntry(t, state, 100001)
		if !entry.Barrier || !entry.Absent || entry.SourceArm != spec.SRSDropArmAbsentRefused || entry.PublishedIdx != 0 {
			t.Fatalf("entry=%+v want barrier flipped, provenance kept", entry)
		}
		paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{100001: true}, "barrier ⇒ stale")
	})

	t.Run("OP-5b-inert-op", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		provider, state := paRealState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM 100001;"+
				"DROP VIEW v;"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		adj3 := paAdj(t, enriched[2])
		if adj3.Arm != spec.SRSDropArmAbsentRefused || !opSRS(t, enriched[2]).OrderedAbsent {
			t.Fatalf("stmt3 arm=%v marker=%v — an inert op never shadows the entry", adj3.Arm, opSRS(t, enriched[2]).OrderedAbsent)
		}
		if entry := opEntry(t, state, 100001); entry.Barrier {
			t.Fatalf("entry=%+v want no barrier from an inert op", entry)
		}
		paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{}, "inert op mints nothing")
	})

	t.Run("OP-5c-touching-deps-stale", func(t *testing.T) {
		positive := []r3DepRow{{schema: "s", table: "t", column: "g", srid: 100001}}
		fx := opAbsentFixture(opGrantConsumable)
		fx.depsScript = [][]r3DepRow{nil, positive}
		provider, state := paRealState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM 100001;"+
				"DROP TABLE s.t;"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		if !state.SRSDepsStale() {
			t.Fatal("DROP TABLE must mint depsStale")
		}
		adj3 := paAdj(t, enriched[2])
		if adj3.Arm != spec.SRSDropArmAbsentRefused || !opSRS(t, enriched[2]).OrderedAbsent {
			t.Fatalf("stmt3 arm=%v — stale demotes the positive witness, ordered A still holds", adj3.Arm)
		}
		if entry := opEntry(t, state, 100001); entry.Barrier {
			t.Fatal("depsStale demotes the witness only — the entry keeps no barrier")
		}
		paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{}, "no existence stale")
	})

	t.Run("OP-5d-revoke-scope", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		provider, state := paRealState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
				"REVOKE SELECT ON *.* FROM 'app'@'%';"+
				"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
		if !state.ScopeEnded() {
			t.Fatal("REVOKE must end the privilege scope")
		}
		adj3 := paAdj(t, enriched[2])
		if adj3.Arm != spec.SRSDropArmAbsentConditional {
			t.Fatalf("stmt3 arm=%v want absent_conditional — REVOKE ends the premise, not the fact", adj3.Arm)
		}
		if !opSRS(t, enriched[2]).OrderedAbsent {
			t.Fatal("stmt3 must still be ordered-served — the fact-level entry survives")
		}
		if entry := opEntry(t, state, 100001); entry.Barrier || !entry.Absent {
			t.Fatalf("entry=%+v want REVOKE leaves the entry intact", entry)
		}
	})

	t.Run("OP-5e-contamination", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		provider, state := paRealState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM 100001;"+
				"GARBAGE;"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		if !state.Contaminated() {
			t.Fatal("GARBAGE must contaminate the batch")
		}
		// The GARBAGE statement is a parse failure — it takes no
		// enriched slot; the trailing DROP is enriched[1].
		adj3 := paAdj(t, enriched[1])
		if adj3.Arm != spec.SRSDropArmBlocked {
			t.Fatalf("trailing arm=%v want blocked — contamination shadows consumption", adj3.Arm)
		}
		if opSRS(t, enriched[1]).OrderedAbsent {
			t.Fatal("a contaminated bind never commits the ordered marker")
		}
		if entry := opEntry(t, state, 100001); !entry.Absent {
			t.Fatalf("entry=%+v kept — contamination shadows, never deletes", entry)
		}
	})
}

// --- OP-6: admission ----------------------------------------------------

// OP-6: an out-of-domain resolved version takes the version boundary
// before any ordered lookup (no marker, no entry mint); a providerless
// state has no capability and no provider — the existing path governs.
func TestT26SRSOrderedAdmissionRealChain(t *testing.T) {
	t.Run("OP-6-version-boundary", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		provider, state := paRealState(t, fx)
		state.SeedResolvedVersion(&spec.VersionIdentity{
			Product: spec.VersionProductMySQL, Version: "8.0.3", Major: 8, Minor: 0, Patch: 3,
		})
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM 100001;"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		for i, stmt := range enriched {
			if stmt.Unsupported == nil || stmt.Unsupported.Metadata["gate"] != "version" {
				t.Fatalf("stmt%d must take the version boundary: %+v", i, stmt.Unsupported)
			}
			if stmt.Metadata != nil && stmt.Metadata.SpatialReference != nil && stmt.Metadata.SpatialReference.OrderedAbsent {
				t.Fatalf("stmt%d boundary statements never carry the marker", i)
			}
		}
		if len(state.SRSOrderedSRIDs()) != 0 {
			t.Fatal("a boundary statement never mints ordered entries")
		}
	})

	t.Run("OP-6-providerless", func(t *testing.T) {
		state := audit.NewTestBatchState(nil)
		enriched := audit.EnrichSRSPrivForTest(t, state, nil,
			"DROP SPATIAL REFERENCE SYSTEM 100001;"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		// stmt1 is unproven providerless; its own apply mints the
		// same-SRID stale flag, so stmt2 blocks at the bind — neither
		// statement produces or consumes an ordered entry.
		wantArms := []spec.SRSDropArm{spec.SRSDropArmUnproven, spec.SRSDropArmBlocked}
		for i, stmt := range enriched {
			if adj := paAdj(t, stmt); adj.Arm != wantArms[i] {
				t.Fatalf("stmt%d arm=%v want %v — providerless never produces or consumes", i, adj.Arm, wantArms[i])
			}
			if opSRS(t, stmt).OrderedAbsent {
				t.Fatalf("stmt%d must not carry the marker", i)
			}
		}
		if len(state.SRSOrderedSRIDs()) != 0 {
			t.Fatal("providerless mints nothing")
		}
	})
}

// --- OP-8: bound-state snapshot invariance ------------------------------

// OP-8a: the producer statement's bound projection is invariant across
// post-bind / own-apply / a later ordered hit's apply — the ordered
// channel never mutates earlier statements' metadata.
func TestT26SRSOrderedSnapshotInvariance(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	var producerAtApply paStmtSnap
	state.SetAfterApplyObserve(func(i int, stmt *spec.Statement) {
		if i == 0 {
			producerAtApply = paSnapStmt(*stmt)
		}
	})
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	// The hook fires post-apply per statement: the producer's snapshot
	// captured after its own apply must equal the same statement
	// re-captured at batch end — no later statement mutates it.
	if producerAtApply.srsN || producerAtApply.adjN {
		t.Fatal("producer must carry bound state + adjudication")
	}
	paSnapEqual(t, "producer after-own-apply vs final", producerAtApply, paSnapStmt(enriched[0]))
	// The consumer's bound state carries the marker while the
	// producer's stays catalog-served.
	if opSRS(t, enriched[0]).OrderedAbsent || !opSRS(t, enriched[1]).OrderedAbsent {
		t.Fatal("marker positions wrong")
	}
}

// --- OP-7: public projection ---------------------------------------------

// OP-7: an ordered-served statement projects the same public findings
// shape as a catalog-served A arm — identical finding identity,
// zero gaps on the same-arm rows, and no ordered provenance in the
// serialized output.
func TestT26SRSOrderedPublicProjection(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	result := audit.EvaluateEnrichedForTest(t, enriched)
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	// Same-arm public shape: each statement carries exactly the
	// absent_noop notice — the ordered channel never surfaces.
	for i, stmt := range result.Statements {
		paAbsentNoop(t, stmt, 100001)
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("stmt%d gaps=%+v want none on an absent_noop arm", i, stmt.EvidenceGaps)
		}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"OrderedAbsent", "ordered_absent", "source_arm", "published_idx", "own_privilege", "barrier"} {
		if strings.Contains(string(raw), needle) {
			t.Fatalf("public output leaks ordered provenance %q", needle)
		}
	}
}

package audit_test

// T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01 — the ordered
// publication channel's real-product-chain rows (OP-1..OP-8 witnesses
// over the r3 stub driver → mysqlmeta.NewProvider supply chain). Every
// vector is measured at the independent seam counters: the contract's
// R counts BOTH forwarding kinds at the actual dispatch attempt —
// ResolveObject full forwards plus ResolveMySQLSRSDependencies direct
// forwards (failed attempts included); Qexist/Qdeps/Qo count issued
// catalog queries (attempts included); L counts privilege loads. An
// ordered hit issues one direct R + Qo+Qdeps while Qexist is skipped
// (labeled skipped-by-ordered, never executed), and a warm objectCache
// hit issues nothing at all (labeled cache-hit).

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	audit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
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

// opTableProvider is the accepted `s.t` table fixture (R3-C2): a
// table-only adapter wrapping the real product provider — it answers
// LoadTableSnapshot for s.t with a scripted EXISTING table carrying
// the geometry column g, counts those table reads on its own seam,
// and forwards every other seam untouched. Table reads never enter
// the SRS R/Qexist/Qdeps/Qo/L vectors.
type opTableProvider struct {
	inner      *r3ProductProvider
	tableCalls []string // "schema.table" — the table-read seam
}

func (p *opTableProvider) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	return p.inner.LoadInstanceFacts(ctx, dialect, schema)
}
func (p *opTableProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	p.tableCalls = append(p.tableCalls, schema+"."+table)
	if schema == "s" && table == "t" {
		return &spec.TableSnapshot{
			Schema: "s", Exists: true,
			Table:   &spec.Table{Schema: "s", Name: "t"},
			Columns: []spec.Column{{Name: "g", Type: "geometry"}},
		}, nil
	}
	return &spec.TableSnapshot{Schema: schema}, nil
}
func (p *opTableProvider) ResolveObject(ctx context.Context, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return p.inner.ResolveObject(ctx, dialect, req)
}
func (p *opTableProvider) LoadMySQLServerDropPrivilege(ctx context.Context, dialect spec.Dialect) (*spec.MySQLServerDropPrivilegeObservation, error) {
	return p.inner.LoadMySQLServerDropPrivilege(ctx, dialect)
}
func (p *opTableProvider) ResolveMySQLSRSDependencies(ctx context.Context, srid uint32) (*spec.SRSDependencyFact, error) {
	return p.inner.ResolveMySQLSRSDependencies(ctx, srid)
}

// opTableState mirrors paRealState with the accepted s.t table
// adapter layered on top — the SRS provider chain stays real.
func opTableState(t *testing.T, fx *r3Fixture) (*r3ProductProvider, *opTableProvider, *audit.TestBatchState) {
	t.Helper()
	inner := r3Harness(t, fx)
	provider := &opTableProvider{inner: inner}
	return inner, provider, audit.NewTestBatchState(provider)
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
	return &r3Fixture{version: "8.0.36", discoverVer: "8.0.36", account: "app@%", grants: grants}
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
	// Measured total (2,1,2,2): stmt1 Δ(1,1,1,1) full dispatch; stmt2
	// Δ(1,0,1,1) — one direct-reader R forward, Qexist skipped-by-ordered,
	// fresh Qdeps+Qo.
	r3AssertReads(t, provider, fx, 2, 1, 2, 2, 1, -1)

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
	// Same total as OP-2a: (2,1,2,2).
	r3AssertReads(t, provider, fx, 2, 1, 2, 2, 1, -1)
}

// OP-2f (frozen): the accepted s.t table fixture bound via the real
// LoadTableSnapshot seam, the deps answer stays EMPTY — stmt1
// A-plain publishes; stmt2's real DROP TABLE mints srsDepsStale;
// stmt3's ordered hit still consumes the existence entry (the empty
// stale witness and the proven absence both reach A — staleness never
// invalidates the existence entry).
func TestT26SRSOrderedDepsStaleEmptyWitnessRealChain(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	inner, tables, state := opTableState(t, fx)
	var vecs [][5]int
	state.SetAfterApplyObserve(func(int, *spec.Statement) {
		vecs = append(vecs, r3SnapReads(inner, fx))
	})
	enriched := audit.EnrichSRSPrivForTest(t, state, tables,
		"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP TABLE s.t;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("stmt1 arm=%v want absent_refused", arm)
	}
	if len(tables.tableCalls) != 1 || tables.tableCalls[0] != "s.t" {
		t.Fatalf("table reads=%v want exactly [s.t]", tables.tableCalls)
	}
	if !state.SRSDepsStale() {
		t.Fatal("DROP TABLE must mint depsStale")
	}
	adj3 := paAdj(t, enriched[2])
	if adj3.Arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("stmt3 arm=%v want absent_refused — depsStale never invalidates the existence entry", adj3.Arm)
	}
	if !opSRS(t, enriched[2]).OrderedAbsent {
		t.Fatal("stmt3 must be ordered-served")
	}
	if entry := opEntry(t, state, 100001); entry.Barrier {
		t.Fatal("depsStale demotes the witness only — the entry keeps no barrier")
	}
	// stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0) on the SRS seams (the table
	// read is its own seam); stmt3 direct Δ(1,0,1,1).
	if len(vecs) != 3 {
		t.Fatalf("snapshots=%d want 3", len(vecs))
	}
	r3AssertDelta(t, "stmt1", [5]int{}, vecs[0], 1, 1, 1, 1, 1)
	r3AssertDelta(t, "stmt2", vecs[0], vecs[1], 0, 0, 0, 0, 0)
	r3AssertDelta(t, "stmt3", vecs[1], vecs[2], 1, 0, 1, 1, 0)
	r3AssertReads(t, inner, fx, 2, 1, 2, 2, 1, -1)
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

// --- OP-1 producer variants ---------------------------------------------

// OP-1c: an IF EXISTS producer under a negative grant — the premise is
// loaded but SUPER-less, so stmt1 adjudicates absent_conditional owing
// exactly [P]; the catalog-certified A still publishes with the
// producing statement's own (unproven) premise recorded.
func TestT26SRSOrderedProducerNegativeGrant(t *testing.T) {
	fx := opAbsentFixture(opGrantNegative)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	adj := paAdj(t, enriched[0])
	if adj.Arm != spec.SRSDropArmAbsentConditional {
		t.Fatalf("arm=%v want absent_conditional", adj.Arm)
	}
	if len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactDropPrivilege {
		t.Fatalf("owed=%v want exactly [srs.drop.privilege]", adj.Owed)
	}
	entry := opEntry(t, state, 100001)
	if !entry.Absent || entry.Barrier ||
		entry.SourceArm != spec.SRSDropArmAbsentConditional ||
		entry.PublishedIdx != 0 {
		t.Fatalf("entry=%+v want A-IE? provenance", entry)
	}
	r3AssertReads(t, provider, fx, 1, 1, 1, 1, 1, -1)
}

// OP-1d: authored SRID 0 takes S0 after the full read — the catalog
// answer never matters; no entry is ever minted for a refused arm.
func TestT26SRSOrderedProducerSRID0(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 0;")
	if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmRefusedSRID0 {
		t.Fatalf("arm=%v want refused_srid0", arm)
	}
	if len(state.SRSOrderedSRIDs()) != 0 {
		t.Fatal("S0 never mints an ordered entry")
	}
	r3AssertReads(t, provider, fx, 1, 1, 1, 1, 1, -1)
}

// OP-1f: a real spatialTouching DROP TABLE ahead of the first SRS
// statement — the accepted s.t table fixture is bound through the
// real LoadTableSnapshot seam; depsStale is minted before the SRS
// statement ever runs; the tail is still a producer (no entry exists)
// and lands A-plain.
func TestT26SRSOrderedDepsStaleBeforeFirstSRS(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	inner, tables, state := opTableState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, tables,
		"DROP TABLE s.t;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if len(tables.tableCalls) != 1 || tables.tableCalls[0] != "s.t" {
		t.Fatalf("table reads=%v want exactly [s.t] — the accepted fixture bound", tables.tableCalls)
	}
	if !state.SRSDepsStale() {
		t.Fatal("DROP TABLE must mint depsStale")
	}
	adj := paAdj(t, enriched[1])
	if adj.Arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("arm=%v want absent_refused — first SRS statement is a producer", adj.Arm)
	}
	if opSRS(t, enriched[1]).OrderedAbsent {
		t.Fatal("a producer never carries the ordered marker")
	}
	entry := opEntry(t, state, 100001)
	if !entry.Absent || entry.Barrier || entry.SourceArm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("entry=%+v want plain producer provenance", entry)
	}
	// Table reads never enter the SRS vector — the SRS seams still
	// show the single producer full read.
	r3AssertReads(t, inner, fx, 1, 1, 1, 1, 1, -1)
}

// OP-1g: an unparseable Qo banner can never carry the completeness
// proof — S2 with the authored stale mint; no entry, full reads issued.
func TestT26SRSOrderedUnparseableQo(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.version = "unparseable"
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmUnproven {
		t.Fatalf("arm=%v want unproven — unparseable Qo", arm)
	}
	if len(state.SRSOrderedSRIDs()) != 0 {
		t.Fatal("S2 never mints an ordered entry")
	}
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{100001: true}, "S2 minted stale")
	r3AssertReads(t, provider, fx, 1, 1, 1, 1, 1, -1)
}

// OP-1h: a same-SRID CREATE boundary ahead of the first DROP — the
// boundary mints the authored stale; the tail exits blocked-B owing
// exactly [B]; no entry is ever minted.
func TestT26SRSOrderedBoundaryBeforeFirstDrop(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 DEFINITION 'x';"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if enriched[0].Unsupported == nil {
		t.Fatal("stmt1 must carry the implementation boundary")
	}
	adj := paAdj(t, enriched[1])
	if adj.Arm != spec.SRSDropArmBlocked {
		t.Fatalf("arm=%v want blocked — the boundary shadowed the batch", adj.Arm)
	}
	if len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactBatchEffects {
		t.Fatalf("owed=%v want exactly [B]", adj.Owed)
	}
	if len(state.SRSOrderedSRIDs()) != 0 {
		t.Fatal("blocked never consumes or mints")
	}
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{100001: true}, "boundary minted authored stale")
}

// --- OP-3: privilege lifecycle on the channel ---------------------------

// OP-3b: IE producer (consumable) → REVOKE ends the scope → IE tail is
// ordered-served and re-derives its own suppressed premise →
// absent_conditional owing exactly [P]; the entry keeps the earliest
// (consumable) provenance.
func TestT26SRSOrderedRevokeBetweenIEHits(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
			"REVOKE SELECT ON *.* FROM 'app'@'%';"+
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentNoop {
		t.Fatalf("stmt1 arm=%v want absent_noop", arm)
	}
	if !state.ScopeEnded() {
		t.Fatal("REVOKE must end the privilege scope")
	}
	adj3 := paAdj(t, enriched[2])
	if adj3.Arm != spec.SRSDropArmAbsentConditional {
		t.Fatalf("stmt3 arm=%v want absent_conditional on the ordered hit", adj3.Arm)
	}
	if adj3.Privilege != spec.SRSDropPrivilegeSuppressed {
		t.Fatalf("stmt3 premise=%v want suppressed — the positive witness was REVOKE-ended, so the re-derived premise is suppressed, not unproven", adj3.Privilege)
	}
	if len(adj3.Owed) != 1 || adj3.Owed[0] != spec.SRSFactDropPrivilege {
		t.Fatalf("stmt3 owed=%v want exactly [srs.drop.privilege]", adj3.Owed)
	}
	if !opSRS(t, enriched[2]).OrderedAbsent {
		t.Fatal("stmt3 must be ordered-served")
	}
	// The entry keeps the earliest provenance — the tail's re-derived
	// premise never rewrites the publication record.
	entry := opEntry(t, state, 100001)
	if !entry.Absent || entry.Barrier || entry.SourceArm != spec.SRSDropArmAbsentNoop ||
		entry.OwnPrivilege != spec.SRSDropPrivilegeConsumable || entry.PublishedIdx != 0 {
		t.Fatalf("entry=%+v want earliest provenance kept", entry)
	}
	r3AssertReads(t, provider, fx, 2, 1, 2, 2, 1, -1)
}

// OP-3c (frozen): negative grant on BOTH sides of a REVOKE —
//
//	DROP IF EXISTS 100001; REVOKE …; DROP IF EXISTS 100001
//
// Because NO positive witness was ever established, REVOKE ends the
// scope but the tail's re-derived premise stays UNPROVEN — never
// suppressed. stmt1 and stmt3 both land absent_conditional owing [P];
// the ordered fact remains consumable (stmt3 ordered-served); the
// producer's provenance is not rewritten.
func TestT26SRSOrderedNegativeGrantBothSides(t *testing.T) {
	fx := opAbsentFixture(opGrantNegative)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
			"REVOKE SELECT ON *.* FROM 'app'@'%';"+
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	if len(enriched) != 3 {
		t.Fatalf("enriched=%d want 3", len(enriched))
	}
	if !state.ScopeEnded() {
		t.Fatal("REVOKE must end the privilege scope")
	}
	for _, i := range []int{0, 2} {
		adj := paAdj(t, enriched[i])
		if adj.Arm != spec.SRSDropArmAbsentConditional {
			t.Fatalf("stmt%d arm=%v want absent_conditional", i+1, adj.Arm)
		}
		if len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactDropPrivilege {
			t.Fatalf("stmt%d owed=%v want exactly [srs.drop.privilege]", i+1, adj.Owed)
		}
	}
	// The frozen distinction: no positive witness ever existed, so the
	// tail premise stays UNPROVEN — suppressed requires a prior
	// discharged witness (OP-3b proves that arm).
	adj3 := paAdj(t, enriched[2])
	if adj3.Privilege != spec.SRSDropPrivilegeUnproven {
		t.Fatalf("stmt3 premise=%v want unproven — never a positive witness, so REVOKE cannot suppress it", adj3.Privilege)
	}
	if opSRS(t, enriched[0]).OrderedAbsent {
		t.Fatal("stmt1 is the producer — no marker")
	}
	if !opSRS(t, enriched[2]).OrderedAbsent {
		t.Fatal("stmt3 must remain ordered-served — REVOKE is premise-level, the ordered fact stays consumable")
	}
	// Prefix metadata and provenance are not rewritten.
	entry := opEntry(t, state, 100001)
	if entry.SourceArm != spec.SRSDropArmAbsentConditional ||
		entry.OwnPrivilege != spec.SRSDropPrivilegeUnproven ||
		entry.PublishedIdx != 0 || entry.Barrier || !entry.Absent {
		t.Fatalf("entry=%+v want the producer's unchanged conditional provenance", entry)
	}
	r3AssertReads(t, provider, fx, 2, 1, 2, 2, 1, -1)
}

// OP-3d: negative-grant IE producer + plain tail — the plain tail
// never consults the premise: ordered-served A-plain.
func TestT26SRSOrderedNegativeGrantPlainTail(t *testing.T) {
	fx := opAbsentFixture(opGrantNegative)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	adj := paAdj(t, enriched[1])
	if adj.Arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("arm=%v want absent_refused — plain tail ordered hit", adj.Arm)
	}
	if len(adj.Owed) != 0 {
		t.Fatalf("owed=%v want none — plain drops never consult the premise", adj.Owed)
	}
	if !opSRS(t, enriched[1]).OrderedAbsent {
		t.Fatal("tail must be ordered-served")
	}
	r3AssertReads(t, provider, fx, 2, 1, 2, 2, 1, -1)
}

// --- OP-2d/OP-2e: shared vs new batchState -----------------------------

// opReads snapshots the R/Qexist/Qdeps/Qo/L seam counters — R = SRS-type
// full forwards + direct deps-reader forwards, all measured at the
// actual dispatch attempt.
func opReads(p *r3ProductProvider, fx *r3Fixture) [5]int {
	r := p.depsForward
	for _, req := range p.forwards {
		if req.Type == "mysql_spatial_reference_system" {
			r++
		}
	}
	return [5]int{r, fx.count["exist"], fx.count["deps"], fx.count["ver"], p.privCalls}
}

func opDelta(before, after [5]int) [5]int {
	var d [5]int
	for i := range d {
		d[i] = after[i] - before[i]
	}
	return d
}

func opWantDelta(t *testing.T, who string, got, want [5]int) {
	t.Helper()
	if got != want {
		t.Fatalf("%s Δ(R,Qexist,Qdeps,Qo,L)=%v want %v", who, got, want)
	}
}

// OP-2d/OP-2e: the genuinely comparable two-call lifecycle. BOTH sides
// run call1 on the clean-zero-absence fixture, the provider answer
// mutates to present, then BOTH run call2 — the ONLY difference is
// call2's batchState (reused vs new). Per-call deltas are measured
// independently; each call's own premise is unproven (negative grant,
// loader-produced).
func TestT26SRSOrderedSharedVsNewStateRealChain(t *testing.T) {
	const drop = "DROP SPATIAL REFERENCE SYSTEM 100001;"

	// --- OP-2d: shared batchState --------------------------------------
	fx := opAbsentFixture(opGrantNegative)
	provider, state := paRealState(t, fx)

	v0 := opReads(provider, fx)
	e1 := audit.EnrichSRSPrivForTest(t, state, provider, drop)
	adj1 := paAdj(t, e1[0])
	if adj1.Arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("call1 arm=%v want absent_refused", adj1.Arm)
	}
	if adj1.Privilege != spec.SRSDropPrivilegeUnproven {
		t.Fatalf("call1 premise=%v want unproven (loader-produced, negative grant)", adj1.Privilege)
	}
	opWantDelta(t, "call1", opDelta(v0, opReads(provider, fx)), [5]int{1, 1, 1, 1, 1})

	fx.srs = map[uint32]string{100001: "SRID 100001"} // the catalog answer changes

	v1 := opReads(provider, fx)
	e2 := audit.EnrichSRSPrivForTest(t, state, provider, drop)
	adj2 := paAdj(t, e2[0])
	if adj2.Arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("OP-2d call2 arm=%v want absent_refused — the ordered entry outranks the changed catalog answer", adj2.Arm)
	}
	if adj2.Privilege != spec.SRSDropPrivilegeUnproven {
		t.Fatalf("call2 premise=%v want unproven", adj2.Privilege)
	}
	if !opSRS(t, e2[0]).OrderedAbsent {
		t.Fatal("OP-2d call2 must be ordered-served")
	}
	// call2 ordered hit: one direct R, Qexist skipped-by-ordered, fresh
	// Qo+Qdeps, own loader ΔL=1 — the changed presence is never re-read.
	opWantDelta(t, "OP-2d call2", opDelta(v1, opReads(provider, fx)), [5]int{1, 0, 1, 1, 1})
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{}, "OP-2d — ordered A mints no stale")
	if entry := opEntry(t, state, 100001); entry.Barrier || !entry.Absent ||
		entry.SourceArm != spec.SRSDropArmAbsentRefused ||
		entry.OwnPrivilege != spec.SRSDropPrivilegeUnproven || entry.PublishedIdx != 0 {
		t.Fatalf("entry=%+v want first-producer provenance intact", entry)
	}

	// --- OP-2e: identical lifecycle, NEW batchState for call2 -----------
	fxE := opAbsentFixture(opGrantNegative)
	providerE, stateE1 := paRealState(t, fxE)

	vE0 := opReads(providerE, fxE)
	eE1 := audit.EnrichSRSPrivForTest(t, stateE1, providerE, drop)
	adjE1 := paAdj(t, eE1[0])
	if adjE1.Arm != spec.SRSDropArmAbsentRefused || adjE1.Privilege != spec.SRSDropPrivilegeUnproven {
		t.Fatalf("OP-2e call1 adj=%+v want absent_refused + unproven premise", adjE1)
	}
	opWantDelta(t, "OP-2e call1", opDelta(vE0, opReads(providerE, fxE)), [5]int{1, 1, 1, 1, 1})
	entryBefore := opEntry(t, stateE1, 100001)

	fxE.srs = map[uint32]string{100001: "SRID 100001"} // identical mutation

	stateE2 := audit.NewTestBatchState(providerE)
	vE1 := opReads(providerE, fxE)
	eE2 := audit.EnrichSRSPrivForTest(t, stateE2, providerE, drop)
	adjE2 := paAdj(t, eE2[0])
	if adjE2.Arm != spec.SRSDropArmUnproven {
		t.Fatalf("OP-2e call2 arm=%v want unproven — the fresh batchState performs the real full read", adjE2.Arm)
	}
	if adjE2.Privilege != spec.SRSDropPrivilegeUnproven {
		t.Fatalf("OP-2e call2 premise=%v want unproven", adjE2.Privilege)
	}
	if opSRS(t, eE2[0]).OrderedAbsent {
		t.Fatal("OP-2e call2 must not carry the ordered marker")
	}
	wantOwed := []string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}
	if len(adjE2.Owed) != len(wantOwed) {
		t.Fatalf("OP-2e owed=%v want exactly %v", adjE2.Owed, wantOwed)
	}
	for i := range wantOwed {
		if adjE2.Owed[i] != wantOwed[i] {
			t.Fatalf("OP-2e owed[%d]=%q want %q", i, adjE2.Owed[i], wantOwed[i])
		}
	}
	opWantDelta(t, "OP-2e call2", opDelta(vE1, opReads(providerE, fxE)), [5]int{1, 1, 1, 1, 1})
	paSameStale(t, audit.BatchStateStaleSet(stateE2), map[uint32]bool{100001: true}, "OP-2e S2 minted authored stale")
	if _, ok := stateE2.SRSOrderedEntry(100001); ok {
		t.Fatal("a new batchState has no ordered channel state to consume")
	}
	// The prior batchState's entry is untouched by the fresh state's read.
	if entry := opEntry(t, stateE1, 100001); entry != entryBefore {
		t.Fatalf("prior batchState entry mutated: %+v vs %+v", entryBefore, entry)
	}
}

// --- OP-2g/OP-2h: active entry + no capability --------------------------

// OP-2g: warm-cache no-capability — the thin adapter supports the full
// resolver and the privilege loader but not the deps-reader method.
// stmt1 mints through the full path; stmt2's ordered lookup cannot
// commit (no capability → the entry is NOT consumed), so the full
// path's per-enrich cache serves the identical full key: stmt1's own
// Δ(1,1,1,1) is measured, then stmt2's own Δ(0,0,0,0) proves the real
// cache hit — never inferred from the batch total. Zero L on stmt2.
func TestT26SRSOrderedNoCapabilityWarmCache(t *testing.T) {
	fx := opAbsentFixture(opGrantNegative)
	inner := r3Harness(t, fx)
	provider := &opNoCapProvider{inner: inner}
	// The adapter must NOT satisfy the ordered capability — the whole
	// row depends on that negative.
	if _, ok := interface{}(provider).(audit.MySQLSRSDependencyReader); ok {
		t.Fatal("opNoCapProvider must not satisfy MySQLSRSDependencyReader")
	}
	state := audit.NewTestBatchState(provider)

	// Per-statement counter snapshots at post-apply — stmt2's cache hit
	// is measured inside its own window, not inferred.
	var snapAt [][5]int
	state.SetAfterApplyObserve(func(idx int, _ *spec.Statement) {
		snapAt = append(snapAt, r3SnapReads(inner, fx))
	})

	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("stmt1 arm=%v want absent_refused", arm)
	}
	if len(snapAt) != 2 {
		t.Fatalf("post-apply snapshots=%d want 2", len(snapAt))
	}
	bound2 := opSRS(t, enriched[1])
	if bound2.OrderedAbsent {
		t.Fatal("OP-2g: no capability → OrderedAbsent must stay false")
	}
	// The cached catalog answer is asserted from its own fields: the
	// full-key cache served the identical lookup — read+absent.
	if !bound2.Read || bound2.Exists || !bound2.Absence {
		t.Fatalf("OP-2g stmt2 bound=%+v want the cached catalog answer (read+absent)", bound2)
	}
	if arm := paAdj(t, enriched[1]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("stmt2 arm=%v want absent_refused via the cached catalog answer", arm)
	}
	// stmt1 minted: Δ(1,1,1,1)+L=1 from zero; stmt2: Δ(0,0,0,0)+L=0 —
	// a REAL full-key cache hit, zero provider reads of any kind.
	r3AssertDelta(t, "stmt1", [5]int{}, snapAt[0], 1, 1, 1, 1, 1)
	r3AssertDelta(t, "stmt2", snapAt[0], snapAt[1], 0, 0, 0, 0, 0)
	// One entry, complete provenance unchanged by stmt2: capture the
	// keyset AND the full entry — stmt2 touched neither.
	if ids := state.SRSOrderedSRIDs(); len(ids) != 1 || ids[0] != 100001 {
		t.Fatalf("ordered keys=%v want exactly {100001}", ids)
	}
	entry := opEntry(t, state, 100001)
	if entry.Barrier || entry.PublishedIdx != 0 || entry.SourceArm != spec.SRSDropArmAbsentRefused ||
		!entry.Absent || entry.OwnPrivilege != spec.SRSDropPrivilegeUnproven {
		t.Fatalf("entry=%+v want the complete unchanged first provenance", entry)
	}
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{}, "no stale mint")
	// Batch total preserved as the summary check.
	r3AssertReads(t, inner, fx, 1, 1, 1, 1, 1, -1)
}

// OP-2h: cold-cache no-capability — same batchState, new objectCache
// per enrich. The second enrich performs its own real full lookup,
// sees the changed answer, lands S2, and the SAME entry flips
// barrier=false→true with the stale mint — first provenance kept.
// Each call's own Δ(1,1,1,1)+L=1 is measured; the entry is compared
// as expectedAfter=before with Barrier the only changed field.
func TestT26SRSOrderedNoCapabilityColdCache(t *testing.T) {
	fx := opAbsentFixture(opGrantNegative)
	inner := r3Harness(t, fx)
	provider := &opNoCapProvider{inner: inner}
	if _, ok := interface{}(provider).(audit.MySQLSRSDependencyReader); ok {
		t.Fatal("opNoCapProvider must not satisfy MySQLSRSDependencyReader")
	}
	state := audit.NewTestBatchState(provider)

	var snapAt [][5]int
	state.SetAfterApplyObserve(func(idx int, _ *spec.Statement) {
		snapAt = append(snapAt, r3SnapReads(inner, fx))
	})

	e1 := audit.EnrichSRSPrivForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, e1[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("enrich1 arm=%v want absent_refused", arm)
	}
	before := opEntry(t, state, 100001)
	if before.Barrier {
		t.Fatal("before.Barrier must be false — the flip is the measured delta")
	}
	if !before.Absent || before.PublishedIdx != 0 ||
		before.SourceArm != spec.SRSDropArmAbsentRefused ||
		before.OwnPrivilege != spec.SRSDropPrivilegeUnproven {
		t.Fatalf("before=%+v want the complete first provenance", before)
	}
	if len(snapAt) != 1 {
		t.Fatalf("snapshots after enrich1=%d want 1", len(snapAt))
	}
	r3AssertDelta(t, "enrich1", [5]int{}, snapAt[0], 1, 1, 1, 1, 1)

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
	if len(snapAt) != 2 {
		t.Fatalf("snapshots after enrich2=%d want 2", len(snapAt))
	}
	r3AssertDelta(t, "enrich2", snapAt[0], snapAt[1], 1, 1, 1, 1, 1)

	// expectedAfter = before with Barrier the only changed field —
	// the complete entry comparison, not a field pick-list.
	expectedAfter := before
	expectedAfter.Barrier = true
	after := opEntry(t, state, 100001)
	if after != expectedAfter {
		t.Fatalf("entry after=%+v want %+v (barrier the ONLY change)", after, expectedAfter)
	}
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{100001: true}, "barrier ⇒ stale mint")
	// Batch total preserved: two independent real full reads.
	r3AssertReads(t, inner, fx, 2, 2, 2, 2, 2, -1)
}

// --- OP-4: per-hit freshness and errors ---------------------------------

// OP-4a: the per-hit-freshness discriminant — three statements, deps
// answers empty → empty → positive land A-plain, ordered-hit A-plain,
// ordered-hit S1. The positive witness at stmt3 outranks the ordered
// absence; the entry itself survives.
func TestT26SRSOrderedFreshDepsRealChain(t *testing.T) {
	positive := []r3DepRow{{schema: "s", table: "t", column: "g", srid: 100001}}
	fx := opAbsentFixture(opGrantConsumable)
	fx.depsScript = [][]r3DepRow{nil, nil, positive}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	wantArms := []spec.SRSDropArm{
		spec.SRSDropArmAbsentRefused, spec.SRSDropArmAbsentRefused,
		spec.SRSDropArmRefusedDependency,
	}
	for i, stmt := range enriched {
		if arm := paAdj(t, stmt).Arm; arm != wantArms[i] {
			t.Fatalf("stmt%d arm=%v want %v", i, arm, wantArms[i])
		}
		if i > 0 && !opSRS(t, stmt).OrderedAbsent {
			t.Fatalf("stmt%d must be ordered-served", i)
		}
	}
	// Three deps attempts total (stmt1 full pass + 2 direct reads) —
	// the script proves each hit read fresh.
	if fx.depsCalls != 3 || fx.count["deps"] != 3 {
		t.Fatalf("deps attempts=%d want 3 — every hit reads fresh", fx.count["deps"])
	}
	// Frozen total (3,1,3,3): stmt1 Δ(1,1,1,1) + stmt2/3 Δ(1,0,1,1) each.
	r3AssertReads(t, provider, fx, 3, 1, 3, 3, 1, -1)
	// S1 is a non-producer but never invalidates the published entry.
	entry := opEntry(t, state, 100001)
	if entry.Barrier || entry.PublishedIdx != 0 || entry.SourceArm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("entry=%+v want first provenance intact after the S1 hit", entry)
	}
}

// OP-4b: a positive dependency witness under srsDepsStale is demoted
// — stmt2's real DROP TABLE (accepted s.t fixture) mints the flag;
// stmt3's ordered hit binds the ACTUAL positive witness [s.t.g@100001]
// through the real merge, yet the A arm still governs (staleness
// demotes witness use only, never the proven absence entry).
func TestT26SRSOrderedDepsStaleDemotesRealChain(t *testing.T) {
	positive := []r3DepRow{{schema: "s", table: "t", column: "g", srid: 100001}}
	fx := opAbsentFixture(opGrantConsumable)
	fx.depsScript = [][]r3DepRow{nil, positive}
	inner, tables, state := opTableState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, tables,
		"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP TABLE s.t;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("stmt1 arm=%v want absent_refused", arm)
	}
	if len(tables.tableCalls) != 1 || tables.tableCalls[0] != "s.t" {
		t.Fatalf("table reads=%v want exactly [s.t]", tables.tableCalls)
	}
	if !state.SRSDepsStale() {
		t.Fatal("DROP TABLE must mint depsStale")
	}
	// First the bound-state fact: stmt3 carries the ACTUAL positive
	// dependency [s.t.g@100001] — the real merge attached the fresh
	// witness before staleness demoted it.
	bound3 := opSRS(t, enriched[2])
	if !bound3.OrderedAbsent {
		t.Fatal("stmt3 must be ordered-served")
	}
	if bound3.Dependencies == nil || len(bound3.Dependencies.Columns) != 1 {
		t.Fatalf("stmt3 Dependencies=%+v want the positive witness bound", bound3.Dependencies)
	}
	col := bound3.Dependencies.Columns[0]
	if col.Schema != "s" || col.Table != "t" || col.Column != "g" || col.SRID != 100001 {
		t.Fatalf("stmt3 witness=%+v want exactly s.t.g@100001", col)
	}
	// Then the demotion: A governs despite the positive witness.
	adj3 := paAdj(t, enriched[2])
	if adj3.Arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("stmt3 arm=%v want absent_refused — positive witness demoted by staleness", adj3.Arm)
	}
	// Frozen total (2,1,2,2): stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0) SRS-seam;
	// stmt3 Δ(1,0,1,1) direct read.
	r3AssertReads(t, inner, fx, 2, 1, 2, 2, 1, -1)
	if entry := opEntry(t, state, 100001); entry.Barrier {
		t.Fatal("depsStale demotes the witness only — the entry keeps no barrier")
	}
}

// OP-4c: the frozen same-enrich two-statement error witness — stmt1's
// catalog A publishes the entry, stmt2's ordered direct-reader forward
// fails mid-enrich. The public error is the bounded metadataObjectReadError
// shape; the sentinel survives errors.Is but its raw text never leaks;
// stmt2 never reaches adjudication or own-apply; stmt1's bound metadata
// and the complete ordered entry stay unchanged.
func TestT26SRSOrderedDepsErrorSameEnrich(t *testing.T) {
	sentinel := errors.New("op ordered deps sentinel")
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)

	// Independent observation closure: stmt1's post-bind and post-apply
	// entries are recorded by index, and at stmt1's own-apply an
	// independent deep snapshot of its bound projection plus the
	// complete ordered entry is captured — the post-error comparison
	// never relies on live pointers or on the arm field alone.
	var bindIdx, applyIdx []int
	var bound1 *spec.Statement
	var snap1 paStmtSnap
	var entryAtApply audit.SRSOrderedEntryView
	var entryAtApplyOK bool
	state.SetAfterBindObserve(func(i int, stmt *spec.Statement) {
		bindIdx = append(bindIdx, i)
	})
	// stmt1's deps read must SUCCEED while stmt2's fails — the fixture
	// fires failOn on every "deps"-kind query, so arm it at stmt1's
	// own-apply (which runs before stmt2's deps read).
	state.SetAfterApplyObserve(func(i int, stmt *spec.Statement) {
		applyIdx = append(applyIdx, i)
		if i == 0 {
			bound1 = stmt
			snap1 = paSnapStmt(*stmt)
			entryAtApply, entryAtApplyOK = state.SRSOrderedEntry(100001)
			fx.failOn = map[string]error{"deps": sentinel}
		}
	})
	_, err := audit.EnrichSRSPrivErrForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err == nil {
		t.Fatal("the direct-reader failure must propagate")
	}
	if err.Error() != "metadata object read failed" {
		t.Fatalf("err.Error()=%q want the bounded metadataObjectReadError shape", err.Error())
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("errors.Is(sentinel) must hold through the bounded wrap: %v", err)
	}
	if strings.Contains(err.Error(), sentinel.Error()) {
		t.Fatalf("raw sentinel text must never leak into the public error: %q", err.Error())
	}
	// stmt2 never reached EITHER observation point: bind-observe saw
	// only index 0 (the failing direct read happens inside stmt2's
	// bind), apply-observe saw only index 0 (no adjudication, no
	// own-apply for stmt2).
	if len(bindIdx) != 1 || bindIdx[0] != 0 {
		t.Fatalf("bind-observe indexes=%v want [0] — stmt2 never completed bind", bindIdx)
	}
	if len(applyIdx) != 1 || applyIdx[0] != 0 {
		t.Fatalf("apply-observe indexes=%v want [0] — stmt2 never reached adjudication/own-apply", applyIdx)
	}
	if bound1 == nil || !entryAtApplyOK {
		t.Fatal("stmt1 bound state / ordered entry missing at own-apply")
	}
	if arm := paAdj(t, *bound1).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("stmt1 arm=%v want absent_refused", arm)
	}
	// Final counters include the FAILED direct forward: stmt1 Δ(1,1,1,1)
	// + stmt2 Δ(1,0,1,1) attempted before the error → total (2,1,2,2);
	// stmt2's Qexist stayed 0 (skipped-by-ordered).
	r3AssertReads(t, provider, fx, 2, 1, 2, 2, 1, -1)
	// stmt1's bound metadata unchanged — the live statement re-snapshot
	// must equal the independent capture taken at own-apply.
	paSnapEqual(t, "stmt1 bound state after the failed hit", snap1, paSnapStmt(*bound1))
	// Complete ordered entry unchanged — the apply-time view, the
	// post-error view, and the expected provenance all agree.
	want := audit.SRSOrderedEntryView{
		SRID: 100001, Absent: true, SourceArm: spec.SRSDropArmAbsentRefused,
		OwnPrivilege: spec.SRSDropPrivilegeConsumable, PublishedIdx: 0, Barrier: false,
	}
	if entryAtApply != want {
		t.Fatalf("entry at own-apply=%+v want %+v", entryAtApply, want)
	}
	if entry := opEntry(t, state, 100001); entry != want {
		t.Fatalf("entry after error=%+v want %+v (apply-time %+v)", entry, want, entryAtApply)
	}
}

// OP-4 merge-table consumer rows — the REAL ordered-bind merge path
// over the direct reader's non-error outcome shapes, which neither
// the provider unit rows (return values only) nor the synthetic
// marker/adjudication units can stand in for:
//
//   - proven-unsupported Qo: the reader's support gate declines →
//     (nil, nil); the ordered absence still consumes — marker bound,
//     A arm, zero Qexist/Qdeps on the hit.
//   - unparseable Qo: support unproven → the reader continues to a
//     real deps fact; the same merge path adjudicates the ordered A.
//
// versionScript scripts the nth Qo attempt: stmt1's full-path bind
// reads 8.0.36; stmt2's direct-reader Qo is the discriminating input.
// The entry's provenance and the read vectors are asserted exactly.
func TestT26SRSOrderedConsumerReaderVariants(t *testing.T) {
	t.Run("unsupported-qo-nil-fact", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		fx.versionScript = []string{"8.0.36", "5.7.44"}
		provider, state := paRealState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM 100001;"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
			t.Fatalf("stmt1 arm=%v want absent_refused (8.0.36 full-path A)", arm)
		}
		bound2 := opSRS(t, enriched[1])
		if !bound2.OrderedAbsent {
			t.Fatal("stmt2 must consume the ordered absence — nil fact still commits the ordered branch")
		}
		if bound2.Dependencies != nil {
			t.Fatalf("nil fact must bind nil Dependencies, got %+v", bound2.Dependencies)
		}
		if arm := paAdj(t, enriched[1]).Arm; arm != spec.SRSDropArmAbsentRefused {
			t.Fatalf("stmt2 arm=%v want absent_refused — ordered A consumes with a nil witness", arm)
		}
		// stmt1 Δ(1,1,1,1) + stmt2 direct attempt Δ(1,0,0,1) — the
		// support gate declined before the deps query; Qexist stayed
		// skipped-by-ordered.
		r3AssertReads(t, provider, fx, 2, 1, 1, 2, 1, -1)
		entry := opEntry(t, state, 100001)
		if !entry.Absent || entry.Barrier || entry.SourceArm != spec.SRSDropArmAbsentRefused {
			t.Fatalf("entry=%+v want unchanged provenance", entry)
		}
	})

	t.Run("unparseable-qo-deps-fact", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		fx.versionScript = []string{"8.0.36", "unparseable-banner"}
		provider, state := paRealState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM 100001;"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
			t.Fatalf("stmt1 arm=%v want absent_refused", arm)
		}
		bound2 := opSRS(t, enriched[1])
		if !bound2.OrderedAbsent {
			t.Fatal("stmt2 must be ordered-served")
		}
		if bound2.Dependencies == nil {
			t.Fatal("unparseable Qo continues the deps read — a real (empty) fact must bind")
		}
		if arm := paAdj(t, enriched[1]).Arm; arm != spec.SRSDropArmAbsentRefused {
			t.Fatalf("stmt2 arm=%v want absent_refused — empty witness, ordered A", arm)
		}
		// stmt1 Δ(1,1,1,1) + stmt2 direct Δ(1,0,1,1) — support unproven
		// continues the read.
		r3AssertReads(t, provider, fx, 2, 1, 2, 2, 1, -1)
	})
}

// OP-4c additional witness: the same propagation across TWO enrich
// passes — kept as supplementary coverage; the frozen OP-4c shape is
// the same-enrich witness above.
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
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("err=%v want errors.Is(sentinel) through the bounded wrap", err)
	}
	if err.Error() != "metadata object read failed" {
		t.Fatalf("err.Error()=%q want the bounded shape", err.Error())
	}
	// Attempts counted: enrich1's deps read + the failed direct forward.
	if fx.count["deps"] != 2 || provider.depsForward != 1 {
		t.Fatalf("deps attempts=%d depsForward=%d want 2/1", fx.count["deps"], provider.depsForward)
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

// opStmtVecs installs the apply hook that captures the seam counters at
// each statement's own-apply — the per-statement read attribution the
// frozen OP-5 rows require. The caller reads *vecs after the enrich.
func opStmtVecs(provider *r3ProductProvider, fx *r3Fixture, state *audit.TestBatchState, vecs *[][5]int) {
	state.SetAfterApplyObserve(func(i int, stmt *spec.Statement) {
		*vecs = append(*vecs, opReads(provider, fx))
	})
}

// OP-5: the six frozen invalidation rows /5a../5f.
func TestT26SRSOrderedInvalidationRealChain(t *testing.T) {
	// OP-5a: IF EXISTS producer → same-SRID CREATE boundary → plain tail.
	// The boundary binds via the full path (ordered gate excludes
	// Unsupported!=nil) and hits stmt1's memoized objectCache answer;
	// its apply mints stale+barrier; the plain tail is blocked-B.
	t.Run("OP-5a-boundary-barrier", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		provider, state := paRealState(t, fx)
		var vecs [][5]int
		opStmtVecs(provider, fx, state, &vecs)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
				"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 DEFINITION 'x';"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentNoop {
			t.Fatalf("stmt1 arm=%v want absent_noop", arm)
		}
		if arm := paAdj(t, enriched[2]).Arm; arm != spec.SRSDropArmBlocked {
			t.Fatalf("stmt3 arm=%v want blocked — the boundary write shadowed the entry", arm)
		}
		if owed := paAdj(t, enriched[2]).Owed; len(owed) != 1 || owed[0] != spec.SRSFactBatchEffects {
			t.Fatalf("stmt3 owed=%v want exactly [B]", owed)
		}
		// stmt2 served its bound state via the objectCache full-key hit:
		// zero reads, yet Read+Absence populated.
		bound2 := opSRS(t, enriched[1])
		if !bound2.Read || bound2.Exists || !bound2.Absence {
			t.Fatalf("stmt2 bound=%+v want the memoized catalog answer", bound2)
		}
		entry := opEntry(t, state, 100001)
		if !entry.Barrier || !entry.Absent || entry.SourceArm != spec.SRSDropArmAbsentNoop || entry.PublishedIdx != 0 {
			t.Fatalf("entry=%+v want barrier flipped, absent_noop provenance kept", entry)
		}
		paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{100001: true}, "barrier ⇒ stale")
		if len(vecs) != 3 {
			t.Fatalf("vecs=%d want 3 applies", len(vecs))
		}
		// Per-statement attribution: stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0)
		// cache-hit; stmt3 Δ(0,0,0,0) blocked before reads.
		opWantDelta(t, "stmt1", vecs[0], [5]int{1, 1, 1, 1, 1})
		opWantDelta(t, "stmt2", opDelta(vecs[0], vecs[1]), [5]int{0, 0, 0, 0, 0})
		opWantDelta(t, "stmt3", opDelta(vecs[1], vecs[2]), [5]int{0, 0, 0, 0, 0})
		r3AssertReads(t, provider, fx, 1, 1, 1, 1, 1, -1)
	})

	// OP-5b: two publishes then consume — sibling provenance preserved.
	t.Run("OP-5b-siblings", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		provider, state := paRealState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
				"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100002;"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		if arm := paAdj(t, enriched[2]).Arm; arm != spec.SRSDropArmAbsentRefused {
			t.Fatalf("stmt3 arm=%v want absent_refused — ordered hit on 100001", arm)
		}
		if !opSRS(t, enriched[2]).OrderedAbsent {
			t.Fatal("stmt3 must be ordered-served")
		}
		e1 := opEntry(t, state, 100001)
		e2 := opEntry(t, state, 100002)
		if e1.PublishedIdx != 0 || e2.PublishedIdx != 1 ||
			e1.SourceArm != spec.SRSDropArmAbsentNoop || e2.SourceArm != spec.SRSDropArmAbsentNoop ||
			e1.Barrier || e2.Barrier {
			t.Fatalf("sibling entries=%+v/%+v want independent provenance", e1, e2)
		}
		if len(state.SRSOrderedSRIDs()) != 2 {
			t.Fatalf("ordered keys=%v want exactly {100001,100002}", state.SRSOrderedSRIDs())
		}
		// Frozen total (3,2,3,3): two full dispatches + one direct read.
		r3AssertReads(t, provider, fx, 3, 2, 3, 3, 1, -1)
	})

	// OP-5c (exact frozen input): `GARBAGE@#$;` — the real parser's
	// failure record swallows the tail into its RawSQL, so NO tail
	// statement exists. This is the bounded counterexample: the batch
	// is still contaminated by the failure record, the producer's
	// entry survives, and zero reads happen after stmt1. Registered
	// as a frozen-fixture erratum — see the execution receipt.
	t.Run("OP-5c-exact-counterexample", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		provider, state := paRealState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
				"GARBAGE@#$;"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		if !state.Contaminated() {
			t.Fatal("the failure record must contaminate the batch")
		}
		// The tail DROP never exists — the failed chunk swallowed it
		// (parse witness in the internal layer). Only stmt1 is authored.
		if len(enriched) != 1 {
			t.Fatalf("enriched=%d want exactly 1 — no tail under GARBAGE@#$", len(enriched))
		}
		if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentNoop {
			t.Fatalf("stmt1 arm=%v want absent_noop — the producer still publishes", arm)
		}
		if entry := opEntry(t, state, 100001); !entry.Absent || entry.Barrier {
			t.Fatalf("entry=%+v kept — contamination shadows, never deletes", entry)
		}
		// Frozen total (1,1,1,1): stmt1 only — nothing after it reads.
		r3AssertReads(t, provider, fx, 1, 1, 1, 1, 1, -1)
	})

	// OP-5c companion: `GARBAGE;` — the parser recovers past the
	// failure record, so the tail IS a real statement; whole-batch
	// contamination shadows the channel: the tail exits before the
	// ordered lookup with ZERO reads; the entry is kept.
	t.Run("OP-5c-contamination-companion", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		provider, state := paRealState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
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
		if entry := opEntry(t, state, 100001); !entry.Absent || entry.Barrier {
			t.Fatalf("entry=%+v kept — contamination shadows, never deletes", entry)
		}
		// Frozen total (1,1,1,1): stmt1 only — the tail performs zero
		// reads (blocked before the ordered lookup).
		r3AssertReads(t, provider, fx, 1, 1, 1, 1, 1, -1)
	})

	// OP-5d: a real DROP TABLE (accepted s.t fixture) mints srsDepsStale
	// only — the plain tail consumes the existence entry
	// (ordered-hit A-plain).
	t.Run("OP-5d-touching-deps-stale", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		inner, tables, state := opTableState(t, fx)
		var vecs [][5]int
		opStmtVecs(inner, fx, state, &vecs)
		enriched := audit.EnrichSRSPrivForTest(t, state, tables,
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
				"DROP TABLE s.t;"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		if len(tables.tableCalls) != 1 || tables.tableCalls[0] != "s.t" {
			t.Fatalf("table reads=%v want exactly [s.t]", tables.tableCalls)
		}
		if !state.SRSDepsStale() {
			t.Fatal("DROP TABLE must mint depsStale")
		}
		adj3 := paAdj(t, enriched[2])
		if adj3.Arm != spec.SRSDropArmAbsentRefused || !opSRS(t, enriched[2]).OrderedAbsent {
			t.Fatalf("stmt3 arm=%v — depsStale never invalidates the existence entry", adj3.Arm)
		}
		if entry := opEntry(t, state, 100001); entry.Barrier {
			t.Fatal("depsStale demotes the witness only — the entry keeps no barrier")
		}
		paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{}, "no existence stale")
		if len(vecs) == 3 {
			opWantDelta(t, "stmt3 direct", opDelta(vecs[1], vecs[2]), [5]int{1, 0, 1, 1, 0})
		}
		// Frozen total (2,1,2,2).
		r3AssertReads(t, inner, fx, 2, 1, 2, 2, 1, -1)
	})

	// OP-5e: a complete engine-only ALTER TABLE is inert — neither
	// flag mints, the plain tail consumes the entry cleanly. The
	// accepted s.t fixture is bound through the real table seam.
	t.Run("OP-5e-inert-alter", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		inner, tables, state := opTableState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, tables,
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
				"ALTER TABLE s.t ENGINE=InnoDB;"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		if state.SRSDepsStale() {
			t.Fatal("an engine-only ALTER must never mint depsStale")
		}
		adj3 := paAdj(t, enriched[2])
		if adj3.Arm != spec.SRSDropArmAbsentRefused || !opSRS(t, enriched[2]).OrderedAbsent {
			t.Fatalf("stmt3 arm=%v — an inert op never shadows the entry", adj3.Arm)
		}
		if entry := opEntry(t, state, 100001); entry.Barrier {
			t.Fatalf("entry=%+v want no barrier from an inert op", entry)
		}
		paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{}, "inert op mints nothing")
		r3AssertReads(t, inner, fx, 2, 1, 2, 2, 1, -1)
	})

	// OP-5f: REVOKE is premise-level only — the plain tail re-derives
	// its own (suppressed) premise while the fact-level entry survives;
	// plain drops never consult the premise.
	t.Run("OP-5f-revoke-scope", func(t *testing.T) {
		fx := opAbsentFixture(opGrantConsumable)
		provider, state := paRealState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
				"REVOKE SELECT ON *.* FROM 'app'@'%';"+
				"DROP SPATIAL REFERENCE SYSTEM 100001;")
		if !state.ScopeEnded() {
			t.Fatal("REVOKE must end the privilege scope")
		}
		adj3 := paAdj(t, enriched[2])
		if adj3.Arm != spec.SRSDropArmAbsentRefused {
			t.Fatalf("stmt3 arm=%v want absent_refused — plain tail ordered-hit A-plain", adj3.Arm)
		}
		if adj3.Privilege != spec.SRSDropPrivilegeSuppressed {
			t.Fatalf("stmt3 premise=%v want suppressed — own premise re-derived", adj3.Privilege)
		}
		if !opSRS(t, enriched[2]).OrderedAbsent {
			t.Fatal("stmt3 must still be ordered-served — the fact-level entry survives")
		}
		if entry := opEntry(t, state, 100001); entry.Barrier || !entry.Absent {
			t.Fatalf("entry=%+v want REVOKE leaves the entry intact", entry)
		}
		r3AssertReads(t, provider, fx, 2, 1, 2, 2, 1, -1)
	})
}

// --- OP-6: admittance & target boundary ---------------------------------

// OP-6a: the in-domain publish baseline — one plain DROP, Δ(1,1,1,1),
// entry minted. (OP-1a is the same row on the single-statement shape.)
func TestT26SRSOrderedBaselineProduceRealChain(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("arm=%v want absent_refused", arm)
	}
	entry := opEntry(t, state, 100001)
	if !entry.Absent || entry.SourceArm != spec.SRSDropArmAbsentRefused ||
		entry.OwnPrivilege != spec.SRSDropPrivilegeConsumable || entry.PublishedIdx != 0 || entry.Barrier {
		t.Fatalf("entry=%+v want the OP-6a minted provenance", entry)
	}
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{}, "no stale mint")
	r3AssertReads(t, provider, fx, 1, 1, 1, 1, 1, -1)
}

// OP-6b (frozen): the below-floor request row as a REAL public
// request — NO target-version pin; the observed banner resolves to
// 8.0.3 through the real discovery seam (Vd=1). The version boundary
// binds via the full path (ordered gate excludes it): reads issue
// Δ(1,1,1,1), L=0 (the loader gate is never entered), the public
// result carries the frozen unaudited/version-gate boundary shape.
// The internal SeedResolvedVersion variant is the companion below.
func TestT26SRSOrderedBelowFloorRealChain(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.version = "8.0.3"
	fx.discoverVer = "8.0.3"
	provider := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL,
		MetadataProvider:   provider,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	// The public version-boundary result — the frozen boundary shape.
	r3Boundary(t, result, err, "spatial_reference_system")
	if result.Unsupported[0].Feature != "drop_spatial_reference_system" {
		t.Fatalf("boundary feature=%q want drop_spatial_reference_system", result.Unsupported[0].Feature)
	}
	// Full bind issued its reads; L=0 — no audited drop ever asked
	// the privilege loader; Vd=1 — the REAL discovery ran.
	r3AssertReads(t, provider, fx, 1, 1, 1, 1, 0, 1)
}

// OP-6b companion: the identical below-floor boundary reached through
// the internal seeded identity — kept as the internal observation of
// own-apply's authored stale mint, which the public layer cannot see.
func TestT26SRSOrderedBelowFloorInternalCompanion(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.version = "8.0.3"
	fx.discoverVer = "8.0.3"
	provider, state := paRealState(t, fx)
	state.SeedResolvedVersion(&spec.VersionIdentity{
		Product: spec.VersionProductMySQL, Version: "8.0.3", Major: 8, Minor: 0, Patch: 3,
	})
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;")
	stmt := enriched[0]
	if stmt.Unsupported == nil || stmt.Unsupported.Metadata["gate"] != "version" {
		t.Fatalf("must take the version boundary: %+v", stmt.Unsupported)
	}
	if stmt.Metadata != nil && stmt.Metadata.SpatialReference != nil && stmt.Metadata.SpatialReference.OrderedAbsent {
		t.Fatal("a boundary statement never carries the marker")
	}
	if stmt.Metadata != nil && stmt.Metadata.SRSDrop != nil {
		t.Fatalf("a boundary statement is never adjudicated: %+v", stmt.Metadata.SRSDrop)
	}
	// The boundary's own-apply minted the authored stale key.
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{100001: true}, "boundary minted authored stale")
	if len(state.SRSOrderedSRIDs()) != 0 {
		t.Fatal("a boundary statement never mints ordered entries")
	}
	r3AssertReads(t, provider, fx, 1, 1, 1, 1, 0, -1)
}

// OP-6c realchain: the REAL request-layer TiDB path — AuditSQL with
// dialect=tidb, no pin, and the recognized banner 8.0.11-TiDB-v8.5.0
// served by the real discovery seam. The vendor boundary marks the
// statement before any SRS/privilege read: Δ(0,0,0,0), L=0, Vd=1.
func TestT26SRSOrderedTiDBVendorBoundaryPublic(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.version = "8.0.11-TiDB-v8.5.0"
	fx.discoverVer = "8.0.11-TiDB-v8.5.0"
	provider := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectTiDB,
		MetadataProvider:   provider,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if !errors.Is(err, audit.ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement (vendor boundary)", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements=%d want 1", len(result.Statements))
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("vendor boundary carries no findings/gaps: %+v/%+v", stmt.Findings, stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("coverage=%v want incomplete on the vendor boundary", stmt.Coverage.Status)
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Reason != spec.UnsupportedVendorBoundaryReason {
		t.Fatalf("unsupported=%+v want exactly one vendor-boundary entry", result.Unsupported)
	}
	// Zero SRS/privilege reads of any kind — only discovery ran (Vd=1).
	r3AssertReads(t, provider, fx, 0, 0, 0, 0, 0, 1)
}

// OP-6d: single providerless public/API row — pin supplies the
// version; provider seams are N/A (no provider attached). The
// providerless S2 projection is unchanged by the ordered channel.
func TestT26SRSOrderedProviderlessPublic(t *testing.T) {
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL,
		TargetVersion:      "8.0.36",
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements=%d want 1", len(result.Statements))
	}
	stmt := result.Statements[0]
	r3ExactGap(t, stmt, []string{spec.SRSFactExistence, spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege})
	if len(stmt.Findings) != 0 {
		t.Fatalf("findings=%+v want none — providerless never discharges", stmt.Findings)
	}
	if stmt.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage=%v want unverified", stmt.Coverage.Status)
	}
}

// OP-6e: malformed target-version pin — typed preflight exit before
// any discovery: errors.Is(ErrInvalidTargetVersion), the installed
// provider's counters show ZERO attempts of every kind, Vd=0.
func TestT26SRSOrderedMalformedPinPublic(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider := r3Harness(t, fx)
	_, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL,
		TargetVersion:      "8.0.x.x",
		MetadataProvider:   provider,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if !errors.Is(err, audit.ErrInvalidTargetVersion) {
		t.Fatalf("err=%v want errors.Is(ErrInvalidTargetVersion)", err)
	}
	r3AssertReads(t, provider, fx, 0, 0, 0, 0, 0, 0)
}

// OP-6 providerless internal twin — kept for the arm-level rows; the
// public row above owns the request-layer assertions.
func TestT26SRSOrderedProviderlessInternal(t *testing.T) {
	state := audit.NewTestBatchState(nil)
	enriched := audit.EnrichSRSPrivForTest(t, state, nil,
		"DROP SPATIAL REFERENCE SYSTEM 100001;"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
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
}

// --- OP-8: snapshots & isolation -----------------------------------------

// OP-8a: stmt1's bound projection (SpatialReference + SRSDrop) is
// deep-copied at the four frozen observation points — post-bind,
// post-own-apply, post-REVOKE-apply, and post-tail/final — and all
// four are identical. The channel itself is asserted at the same
// points: no entry at post-bind, exactly the minted entry after
// own-apply, and the SAME earliest provenance later — publication is
// never judged a bound-snapshot mutation. Per-hook vector deltas are
// kept separate from the enrich-level L.
func TestT26SRSOrderedSnapshotInvariance(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)

	var first *spec.Statement
	var snapPostBind, snapOwnApply, snapRevokeApply paStmtSnap
	var entryAtOwnApply, entryAtRevoke audit.SRSOrderedEntryView
	entryAtBind := false
	var vPrev, vPostBind, vOwnApply, vRevokeApply [5]int

	vPrev = opReads(provider, fx)
	state.SetAfterBindObserve(func(i int, stmt *spec.Statement) {
		if i == 0 {
			snapPostBind = paSnapStmt(*stmt)
			_, entryAtBind = state.SRSOrderedEntry(100001)
			vPostBind = opReads(provider, fx)
			first = stmt
		}
	})
	state.SetAfterApplyObserve(func(i int, stmt *spec.Statement) {
		switch i {
		case 0:
			snapOwnApply = paSnapStmt(*stmt)
			entryAtOwnApply, _ = state.SRSOrderedEntry(100001)
			vOwnApply = opReads(provider, fx)
		case 1:
			// post-REVOKE-apply: stmt1's bound state re-captured through
			// the same object the bind hook kept.
			snapRevokeApply = paSnapStmt(*first)
			entryAtRevoke, _ = state.SRSOrderedEntry(100001)
			vRevokeApply = opReads(provider, fx)
		}
	})

	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
			"REVOKE SELECT ON *.* FROM 'app'@'%';"+
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")

	// (1) Bound-state invariance: all four deep copies identical.
	if snapPostBind.srsN || snapPostBind.adjN {
		t.Fatal("stmt1 must carry bound state + adjudication at post-bind")
	}
	paSnapEqual(t, "post-bind vs post-own-apply", snapPostBind, snapOwnApply)
	paSnapEqual(t, "post-bind vs post-REVOKE-apply", snapPostBind, snapRevokeApply)
	paSnapEqual(t, "post-bind vs final", snapPostBind, paSnapStmt(enriched[0]))

	// (2) Channel transitions at the same four points.
	if entryAtBind {
		t.Fatal("post-bind: the entry must not exist before own-apply")
	}
	wantEntry := audit.SRSOrderedEntryView{
		SRID: 100001, Absent: true, SourceArm: spec.SRSDropArmAbsentNoop,
		OwnPrivilege: spec.SRSDropPrivilegeConsumable, PublishedIdx: 0, Barrier: false,
	}
	if entryAtOwnApply != wantEntry {
		t.Fatalf("post-own-apply entry=%+v want %+v", entryAtOwnApply, wantEntry)
	}
	if entryAtRevoke != wantEntry {
		t.Fatalf("post-REVOKE-apply entry=%+v want SAME earliest provenance", entryAtRevoke)
	}
	if final, _ := state.SRSOrderedEntry(100001); final != wantEntry {
		t.Fatalf("final entry=%+v want SAME earliest provenance", final)
	}

	// (3) Per-hook read attribution: vPrev is sampled BEFORE the enrich
	// call, so the vPrev→post-bind delta measures everything stmt1 did
	// up to the bind hook — resolver dispatch plus the enrich-entry
	// privilege-loader work (L=1 is attributed to that enrich-entry
	// window, not to a "bind window" the sampling cannot isolate).
	// Own-apply issues zero reads; REVOKE's window is read-free.
	opWantDelta(t, "stmt1 pre-bind→post-bind", opDelta(vPrev, vPostBind), [5]int{1, 1, 1, 1, 1})
	opWantDelta(t, "stmt1 own-apply window", opDelta(vPostBind, vOwnApply), [5]int{0, 0, 0, 0, 0})
	opWantDelta(t, "REVOKE window", opDelta(vOwnApply, vRevokeApply), [5]int{0, 0, 0, 0, 0})
	r3AssertReads(t, provider, fx, 2, 1, 2, 2, 1, -1)
}

// OP-8b (frozen): the OP-5a three-statement batch —
//
//	DROP IF EXISTS 100001 → CREATE OR REPLACE boundary → plain DROP
//
// After the producer's own-apply the complete entry is captured; the
// boundary's apply flips exactly one field — barrier false→true —
// while the stale set gains precisely the authored key; the tail is
// blocked-B owing exactly [B]; srsDepsStale and contaminated hold
// their prior values throughout.
func TestT26SRSOrderedBoundaryFlipIsolation(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	var before, after audit.ChannelControls
	state.SetAfterApplyObserve(func(i int, stmt *spec.Statement) {
		switch i {
		case 0:
			before = state.ChannelControlsCapture()
		case 1:
			after = state.ChannelControlsCapture()
		}
	})
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
			"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 DEFINITION 'x';"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentNoop {
		t.Fatalf("stmt1 arm=%v want absent_noop (IE producer)", arm)
	}
	if enriched[1].Unsupported == nil {
		t.Fatal("stmt2 must carry the CREATE implementation boundary")
	}
	// The plain tail is blocked-B — the boundary shadowed the batch.
	adj3 := paAdj(t, enriched[2])
	if adj3.Arm != spec.SRSDropArmBlocked {
		t.Fatalf("stmt3 arm=%v want blocked", adj3.Arm)
	}
	if len(adj3.Owed) != 1 || adj3.Owed[0] != spec.SRSFactBatchEffects {
		t.Fatalf("stmt3 owed=%v want exactly [B]", adj3.Owed)
	}
	if opSRS(t, enriched[2]).OrderedAbsent {
		t.Fatal("a blocked tail never carries the ordered marker")
	}
	// Entry compare: complete before vs complete after — Barrier is
	// the ONLY changed field (full-struct compare, not a field pick).
	entryBefore := before.Ordered[100001]
	entryAfter := after.Ordered[100001]
	wantBefore := audit.SRSOrderedEntryView{
		SRID: 100001, Absent: true, SourceArm: spec.SRSDropArmAbsentNoop,
		OwnPrivilege: spec.SRSDropPrivilegeConsumable, PublishedIdx: 0, Barrier: false,
	}
	wantAfter := wantBefore
	wantAfter.Barrier = true
	if entryBefore != wantBefore || entryAfter != wantAfter {
		t.Fatalf("entry delta=%+v→%+v want ONLY barrier false→true", entryBefore, entryAfter)
	}
	// The stale set gains EXACTLY the authored key — compare a
	// complete expected map, never a whole-map reset.
	wantStale := map[uint32]bool{}
	for k := range before.Stale {
		wantStale[k] = true
	}
	wantStale[100001] = true
	if !reflect.DeepEqual(after.Stale, wantStale) {
		t.Fatalf("stale set=%v want %v (authored key only)", after.Stale, wantStale)
	}
	// Deps-stale and contamination expectations hold: neither flag
	// moved across the boundary.
	if before.DepsStale || after.DepsStale {
		t.Fatalf("depsStale must hold its baseline value: %v→%v", before.DepsStale, after.DepsStale)
	}
	if before.Contaminated || after.Contaminated {
		t.Fatalf("contaminated must hold its baseline value: %v→%v", before.Contaminated, after.Contaminated)
	}
	// Whole-channel compare: strip the two sanctioned delta dimensions
	// (entry view + stale set) — everything else verbatim.
	before.Stale = nil
	after.Stale = nil
	before.Ordered = nil
	after.Ordered = nil
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("channel delta beyond entry+stale: %+v vs %+v", before, after)
	}
}

// OP-8c (frozen): the single producer's own-apply mints EXACTLY its
// own entry. Frozen pre-state: no 100001 entry; unrelated 100002
// entry preset with complete provenance; non-empty controls — SERVER
// s1 bound entry, the s.t table fixture bound through the real table
// seam, schema s populated; the baseline stale/contamination flags
// held. Deep snapshots are captured post-BIND and post-own-apply; the
// only sanctioned delta is the insertion of the complete 100001
// entry; every other channel compares verbatim.
func TestT26SRSOrderedSiblingIsolation(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.servers = map[string]bool{"s1": true}
	inner, tables, state := opTableState(t, fx)

	// Establish the frozen pre-state in a prior enrich on the same
	// state: SERVER s1 bound entry, the s.t table entry (real table
	// seam), schema s populated, and the unrelated 100002 entry with
	// complete provenance.
	pre := audit.EnrichSRSPrivForTest(t, state, tables,
		"DROP SERVER s1;"+
			"DROP TABLE s.t;"+
			"DROP SCHEMA s;"+
			"DROP SPATIAL REFERENCE SYSTEM 100002;")
	if len(pre) != 4 {
		t.Fatalf("control batch enriched=%d want 4", len(pre))
	}
	if len(tables.tableCalls) != 1 || tables.tableCalls[0] != "s.t" {
		t.Fatalf("table reads=%v want exactly [s.t] — the populated fixture", tables.tableCalls)
	}
	before := state.ChannelControlsCapture()
	if len(before.Servers) == 0 || len(before.Tables) == 0 {
		t.Fatalf("controls must be non-empty: servers=%d tables=%d",
			len(before.Servers), len(before.Tables))
	}
	if _, ok := before.Schemas["s"]; !ok {
		t.Fatalf("schema s must be populated by the s.t binding: schemas=%v", before.Schemas)
	}
	entry100002, ok := before.Ordered[100002]
	if !ok {
		t.Fatal("the unrelated 100002 entry must be preset")
	}
	// The 100002 entry carries COMPLETE provenance — every field
	// asserted, not just presence.
	want100002 := audit.SRSOrderedEntryView{
		SRID: 100002, Absent: true, SourceArm: spec.SRSDropArmAbsentRefused,
		OwnPrivilege: spec.SRSDropPrivilegeConsumable, PublishedIdx: 3, Barrier: false,
	}
	if entry100002 != want100002 {
		t.Fatalf("preset 100002 entry=%+v want complete provenance %+v", entry100002, want100002)
	}
	if _, ok := before.Ordered[100001]; ok {
		t.Fatal("100001 must have no entry yet")
	}
	// Baseline flag dims recorded for the verbatim compare.
	baseDeps, baseContam := before.DepsStale, before.Contaminated

	// Post-bind and post-own-apply deep captures.
	var atBind, atApply audit.ChannelControls
	state.SetAfterBindObserve(func(i int, stmt *spec.Statement) {
		atBind = state.ChannelControlsCapture()
	})
	state.SetAfterApplyObserve(func(i int, stmt *spec.Statement) {
		atApply = state.ChannelControlsCapture()
	})
	v0 := r3SnapReads(inner, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, tables,
		"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, enriched[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("producer arm=%v want absent_refused", arm)
	}
	// The producer statement discharges its premise at the
	// adjudication level — the entry field set is provenance-only.
	if adj := paAdj(t, enriched[0]); adj.Privilege != spec.SRSDropPrivilegeConsumable {
		t.Fatalf("premise=%v want consumable", adj.Privilege)
	}
	r3AssertDelta(t, "producer enrich", v0, r3SnapReads(inner, fx), 1, 1, 1, 1, 1)

	// Post-bind: the 100001 entry does not exist yet — publication is
	// an own-apply act, never a bind act.
	if _, ok := atBind.Ordered[100001]; ok {
		t.Fatal("post-bind must have no 100001 entry — minting is own-apply's act")
	}
	wantNew := audit.SRSOrderedEntryView{
		SRID: 100001, Absent: true, SourceArm: spec.SRSDropArmAbsentRefused,
		OwnPrivilege: spec.SRSDropPrivilegeConsumable, PublishedIdx: 0, Barrier: false,
	}
	// Post-own-apply: exactly the complete new entry is added.
	after := atApply
	if len(after.Ordered) != len(before.Ordered)+1 {
		t.Fatalf("ordered keys %v → %v want exactly +{100001}", before.Ordered, after.Ordered)
	}
	if got := after.Ordered[100001]; got != wantNew {
		t.Fatalf("minted entry=%+v want %+v", got, wantNew)
	}
	if got := after.Ordered[100002]; got != before.Ordered[100002] {
		t.Fatalf("sibling 100002 verbatim compare failed: %+v vs %+v", before.Ordered[100002], got)
	}
	// Baseline flag dims unchanged by the mint.
	if after.DepsStale != baseDeps || after.Contaminated != baseContam {
		t.Fatalf("flag dims moved: deps %v→%v contaminated %v→%v",
			baseDeps, after.DepsStale, baseContam, after.Contaminated)
	}
	// Every OTHER channel compares verbatim — strip only the ordered
	// map itself (the one sanctioned delta).
	before.Ordered = nil
	after.Ordered = nil
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("channel mutation beyond the 100001 mint: %+v vs %+v", before, after)
	}
}

// --- OP-7: public projection ----------------------------------------------

// opScanLeak fails when the serialized FINAL public result carries any
// ordered-channel provenance token.
func opScanLeak(t *testing.T, raw []byte) {
	t.Helper()
	for _, needle := range []string{"OrderedAbsent", "ordered_absent", "source_arm", "sourceArm",
		"published_idx", "publishedIdx", "own_privilege", "ownPrivilege", "barrier"} {
		if strings.Contains(string(raw), needle) {
			t.Fatalf("public output leaks ordered provenance %q", needle)
		}
	}
}

// OP-7a: the two-plain-drop request row — stmt1 catalog-served
// A-plain, stmt2 ordered-hit A-plain. The COMPLETE public inventory
// is asserted per statement and identical across the two: exactly one
// ddl.mysql.srs.drop.absent blocker (fixed rule/level/message/metadata
// {srid, outcome}), zero gaps, complete coverage — plus aggregate
// coverage=complete, verdict=reject. Ordered provenance never reaches
// the serialized result.
func TestT26SRSOrderedPublicAbsentPlain(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL:     "DROP SPATIAL REFERENCE SYSTEM 100001;DROP SPATIAL REFERENCE SYSTEM 100001;",
		Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider:   provider,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	for i, stmt := range result.Statements {
		r3OnlyFinding(t, stmt, "ddl.mysql.srs.drop.absent",
			"spatial reference system 100001 does not exist",
			map[string]any{"srid": uint32(100001), "outcome": "refused_absent"})
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("stmt%d gaps=%+v want zero", i, stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status != report.CoverageComplete {
			t.Fatalf("stmt%d coverage=%v want complete", i, stmt.Coverage.Status)
		}
	}
	// Catalog-served and ordered-served same-arm outputs are identical
	// on every non-positional dimension (rule/level/message/metadata);
	// positional index/location are asserted separately.
	for i := range result.Statements {
		f := result.Statements[i].Findings[0]
		if f.StatementIndex != i {
			t.Fatalf("stmt%d finding index=%d want own position (omitempty preserved)", i, f.StatementIndex)
		}
		cat := result.Statements[0].Findings[0]
		if i > 0 && (f.RuleID != cat.RuleID || f.Level != cat.Level || f.Message != cat.Message ||
			!reflect.DeepEqual(f.Metadata, cat.Metadata)) {
			t.Fatalf("ordered-served finding=%+v must equal the catalog-served same-arm finding", f)
		}
	}
	if result.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictReject {
		t.Fatalf("aggregate=%v/%v want complete+reject", result.Coverage.Status, result.Verdict)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	opScanLeak(t, raw)
	r3AssertReads(t, provider, fx, 2, 1, 2, 2, 1, 1)
}

// OP-7c: reserved-SRID co-fire control — IF EXISTS absent_noop arms
// additionally carry the frozen reserved advisory (the arm is not a
// refusal). Both the catalog and the ordered-served statement project
// the identical two-finding inventory.
func TestT26SRSOrderedPublicReservedCoFire(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL:     "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;",
		Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider:   provider,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for i, stmt := range result.Statements {
		if len(stmt.Findings) != 2 {
			t.Fatalf("stmt%d findings=%+v want exactly absent_noop + reserved advisory", i, stmt.Findings)
		}
		var noop, reserved *rule.Finding
		for j := range stmt.Findings {
			switch stmt.Findings[j].RuleID {
			case "ddl.mysql.srs.drop.absent_noop":
				noop = &stmt.Findings[j]
			case "ddl.mysql.srs.drop.reserved":
				reserved = &stmt.Findings[j]
			}
		}
		if noop == nil || reserved == nil {
			t.Fatalf("stmt%d findings=%+v want absent_noop + reserved co-fire", i, stmt.Findings)
		}
		paAbsentNoopFinding(t, *noop, 4326)
		if reserved.Level != rule.LevelNotice ||
			reserved.Message != "spatial reference system 4326 is in a reserved SRID range" ||
			reserved.Metadata["srid"] != uint32(4326) || len(reserved.Metadata) != 1 {
			t.Fatalf("stmt%d reserved=%+v want the frozen advisory", i, *reserved)
		}
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("stmt%d gaps=%+v want zero", i, stmt.EvidenceGaps)
		}
	}
	// Same-arm identical output across the two statements (positional
	// fields excluded — each finding keeps its own statement index).
	for i := range result.Statements[0].Findings {
		a, b := result.Statements[0].Findings[i], result.Statements[1].Findings[i]
		if a.RuleID != b.RuleID || a.Level != b.Level || a.Message != b.Message ||
			!reflect.DeepEqual(a.Metadata, b.Metadata) {
			t.Fatal("ordered-served reserved output must equal the catalog-served one")
		}
	}
}

// OP-7c′: the plain-drop reserved control — A-plain is a refusal arm
// and NEVER carries the reserved advisory; each statement has exactly
// the absent blocker.
func TestT26SRSOrderedPublicReservedPlain(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL:     "DROP SPATIAL REFERENCE SYSTEM 4326;DROP SPATIAL REFERENCE SYSTEM 4326;",
		Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider:   provider,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for i, stmt := range result.Statements {
		r3OnlyFinding(t, stmt, "ddl.mysql.srs.drop.absent",
			"spatial reference system 4326 does not exist",
			map[string]any{"srid": uint32(4326), "outcome": "refused_absent"})
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("stmt%d gaps=%+v want zero", i, stmt.EvidenceGaps)
		}
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("verdict=%v want reject", result.Verdict)
	}
}

// OP-7d: the absent rule disabled by policy — public asserts exactly
// zero findings AND zero gaps per statement, complete coverage,
// verdict pass; the internal companion on the same fixture proves
// publication is policy-independent (one entry, earliest provenance).
func TestT26SRSOrderedPolicyDisabled(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL:     "DROP SPATIAL REFERENCE SYSTEM 100001;DROP SPATIAL REFERENCE SYSTEM 100001;",
		Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider:   provider,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
		ConfigPath:         paPolicyFile(t, "rules:\n  \"ddl.mysql.srs.drop.absent\":\n    enabled: false\n"),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for i, stmt := range result.Statements {
		if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("stmt%d findings/gaps=%+v/%+v want zero/zero (rule disabled)", i, stmt.Findings, stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status != report.CoverageComplete {
			t.Fatalf("stmt%d coverage=%v want complete", i, stmt.Coverage.Status)
		}
	}
	if result.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
		t.Fatalf("aggregate=%v/%v want complete+pass", result.Coverage.Status, result.Verdict)
	}

	// Internal companion — identical fixture, fresh state: publication
	// is policy-blind. The complete entry is captured after stmt1 and
	// compared field-for-field after stmt2 — earliest producer
	// provenance never rewritten by the ordered hit.
	provider2, state2 := paRealState(t, opAbsentFixture(opGrantConsumable))
	var afterStmt1, afterStmt2 audit.SRSOrderedEntryView
	state2.SetAfterApplyObserve(func(idx int, _ *spec.Statement) {
		if view, ok := state2.SRSOrderedEntry(100001); ok {
			if idx == 0 {
				afterStmt1 = view
			} else if idx == 1 {
				afterStmt2 = view
			}
		}
	})
	audit.EnrichSRSPrivForTest(t, state2, provider2,
		"DROP SPATIAL REFERENCE SYSTEM 100001;DROP SPATIAL REFERENCE SYSTEM 100001;")
	ids := state2.SRSOrderedSRIDs()
	if len(ids) != 1 {
		t.Fatalf("entries=%v want exactly one (policy never gates publication)", ids)
	}
	if afterStmt1 != afterStmt2 {
		t.Fatalf("stmt2 rewrote the earliest provenance: %+v vs %+v", afterStmt1, afterStmt2)
	}
	if afterStmt2.PublishedIdx != 0 || afterStmt2.SourceArm != spec.SRSDropArmAbsentRefused ||
		afterStmt2.Barrier || !afterStmt2.Absent ||
		afterStmt2.OwnPrivilege != spec.SRSDropPrivilegeConsumable {
		t.Fatalf("entry=%+v want complete first-producer provenance unchanged", afterStmt2)
	}
}

// OP-7e: the absent rule at warning level — each statement carries
// the absent finding rewritten to warning, zero gaps, complete,
// verdict review; internal companion identical to OP-7d's.
func TestT26SRSOrderedPolicyWarning(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL:     "DROP SPATIAL REFERENCE SYSTEM 100001;DROP SPATIAL REFERENCE SYSTEM 100001;",
		Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider:   provider,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
		ConfigPath:         paPolicyFile(t, "rules:\n  \"ddl.mysql.srs.drop.absent\":\n    enabled: true\n    level: warning\n"),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for i, stmt := range result.Statements {
		if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.mysql.srs.drop.absent" ||
			stmt.Findings[0].Level != rule.LevelWarning {
			t.Fatalf("stmt%d findings=%+v want the absent finding at warning", i, stmt.Findings)
		}
		if len(stmt.EvidenceGaps) != 0 || stmt.Coverage.Status != report.CoverageComplete {
			t.Fatalf("stmt%d gaps/coverage=%+v/%v want zero/complete", i, stmt.EvidenceGaps, stmt.Coverage.Status)
		}
	}
	if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("aggregate=%v/%v want review+complete", result.Verdict, result.Coverage.Status)
	}

	provider2, state2 := paRealState(t, opAbsentFixture(opGrantConsumable))
	var afterStmt1, afterStmt2 audit.SRSOrderedEntryView
	state2.SetAfterApplyObserve(func(idx int, _ *spec.Statement) {
		if view, ok := state2.SRSOrderedEntry(100001); ok {
			if idx == 0 {
				afterStmt1 = view
			} else if idx == 1 {
				afterStmt2 = view
			}
		}
	})
	audit.EnrichSRSPrivForTest(t, state2, provider2,
		"DROP SPATIAL REFERENCE SYSTEM 100001;DROP SPATIAL REFERENCE SYSTEM 100001;")
	if ids := state2.SRSOrderedSRIDs(); len(ids) != 1 {
		t.Fatalf("entries=%v want exactly one (policy never gates publication)", ids)
	}
	if afterStmt1 != afterStmt2 {
		t.Fatalf("stmt2 rewrote the earliest provenance: %+v vs %+v", afterStmt1, afterStmt2)
	}
	if afterStmt2.PublishedIdx != 0 || afterStmt2.SourceArm != spec.SRSDropArmAbsentRefused ||
		afterStmt2.Barrier || !afterStmt2.Absent {
		t.Fatalf("entry=%+v want complete first-producer provenance unchanged", afterStmt2)
	}
}

// OP-7 supplementary: an ordered-served statement projects the same
// findings shape as a catalog-served A-IE arm — kept as the internal
// evaluation twin of the public rows above.
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
	opScanLeak(t, raw)
}

// OP-7b: the OP-3b batch at the real public layer — the REVOKE leg is
// a request-layer unsupported item; stmt1 publishes the absent_noop
// notice; the ordered-served IE tail re-derives its suppressed premise
// and publishes exactly the [srs.drop.privilege] debt under
// unproven_privilege. No internal ordered provenance ever appears in
// the serialized projection.
func TestT26SRSOrderedPublicRevokeLifecycle(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;" +
			"REVOKE SELECT ON *.* FROM 'app'@'%';" +
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;",
		Dialect:            spec.DialectMySQL,
		TargetVersion:      "8.0.36",
		MetadataProvider:   provider,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if !errors.Is(err, audit.ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement — the REVOKE leg is unsupported", err)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("unsupported=%+v want exactly the REVOKE item", result.Unsupported)
	}
	if len(result.Statements) != 3 {
		t.Fatalf("statements=%d want 3", len(result.Statements))
	}
	paAbsentNoop(t, result.Statements[0], 100001)
	if len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("stmt1 gaps=%+v want none on absent_noop", result.Statements[0].EvidenceGaps)
	}
	tail := result.Statements[2]
	if len(tail.Findings) != 0 {
		t.Fatalf("stmt3 findings=%+v want none — the conditional arm publishes a debt, not a notice", tail.Findings)
	}
	paGap(t, tail, "unproven_privilege", []string{spec.SRSFactDropPrivilege})
	if tail.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("stmt3 coverage=%v want unverified — the conditional debt holds the floor", tail.Coverage.Status)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	opScanLeak(t, raw)
}

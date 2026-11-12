package audit

// T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01 internal rows — the
// enrich-apply and unit-layer halves of the frozen case matrix
// (docs/verification/t26-srs-drop-proven-absent-contract-01
// case-matrix PA-4e/PA-4e2/PA-4f/PA-4f2, PA-5a–PA-5f, PA-6d, PA-7a).
// Every adjudication in these rows is produced by the real bind →
// AdjudicateSRSDrop → own-apply chain (or, where marked unit, by the
// shared adjudication function directly); no arm is ever recomputed
// in the test.

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// --- PA-4e/4f/4f2 + the plain/IF-EXISTS split: unit discriminant rows --------

// PA-4e/PA-4f/PA-4f2: ValidAbsence's authored-SRID binding and its
// Read/!Exists/Absence supply conjunction — a foreign-SRID snapshot,
// an unread snapshot, and a contradictory Exists=true pair each stay
// S2 while a valid same-SRID absence selects the A arm.
func TestT26SRSAbsentAdjSupplyDiscriminants(t *testing.T) {
	absent := &spec.SpatialReferenceSystemState{SRID: 100001, Read: true, Exists: false, Absence: true}
	cases := []struct {
		name     string
		state    *spec.SpatialReferenceSystemState
		wantArm  spec.SRSDropArm
		wantOwed []string
	}{
		// PA-4e: foreign SRID — ValidAbsence fails on the authored bind.
		{"foreign-srid", &spec.SpatialReferenceSystemState{SRID: 100002, Read: true, Exists: false, Absence: true},
			spec.SRSDropArmUnproven, []string{spec.SRSFactExistence, spec.SRSFactColumnDependencies}},
		// PA-4f: unread — the Absence bit alone never supplies.
		{"unread", &spec.SpatialReferenceSystemState{SRID: 100001, Read: false, Exists: false, Absence: true},
			spec.SRSDropArmUnproven, []string{spec.SRSFactExistence, spec.SRSFactColumnDependencies}},
		// PA-4f2: contradictory Exists=true — the positive witness governs.
		{"exists-contradicts", &spec.SpatialReferenceSystemState{SRID: 100001, Read: true, Exists: true, Absence: true},
			spec.SRSDropArmUnproven, []string{spec.SRSFactColumnDependencies}},
		// Read && !Exists without Absence stays an ordinary unknown zero-row.
		{"zero-row-unknown", &spec.SpatialReferenceSystemState{SRID: 100001, Read: true, Exists: false, Absence: false},
			spec.SRSDropArmUnproven, []string{spec.SRSFactExistence, spec.SRSFactColumnDependencies}},
		// A valid positive dependency witness outranks a simultaneous
		// proven absence — S1 before A in the first-match order.
		{"dependency-outranks-absence", &spec.SpatialReferenceSystemState{
			SRID: 100001, Read: true, Exists: false, Absence: true,
			Dependencies: &spec.SRSDependencyFact{Columns: []spec.SRSDependentColumn{{Schema: "s", Table: "t", Column: "g", SRID: 100001}}},
		}, spec.SRSDropArmRefusedDependency, nil},
		// The valid row itself — plain drop → absent_refused.
		{"valid-plain", absent, spec.SRSDropArmAbsentRefused, nil},
	}
	for _, tc := range cases {
		adj := spec.AdjudicateSRSDrop(tc.state, 100001, false, spec.SRSDropPrivilegeConsumable, tc.name != "valid-plain")
		if adj.Arm != tc.wantArm {
			t.Fatalf("%s: arm=%v want %v", tc.name, adj.Arm, tc.wantArm)
		}
		if len(adj.Owed) != len(tc.wantOwed) {
			t.Fatalf("%s: owed=%v want %v", tc.name, adj.Owed, tc.wantOwed)
		}
		for i := range tc.wantOwed {
			if adj.Owed[i] != tc.wantOwed[i] {
				t.Fatalf("%s: owed[%d]=%q want %v", tc.name, i, adj.Owed[i], tc.wantOwed)
			}
		}
	}
}

// The plain/IF-EXISTS split on one valid absence: plain refuses
// without consuming; IF EXISTS no-ops while consumable; every
// non-consumable premise yields absent_conditional owing exactly
// [srs.drop.privilege] — never consumed.
func TestT26SRSAbsentAdjIfExistsSplit(t *testing.T) {
	absent := func() *spec.SpatialReferenceSystemState {
		return &spec.SpatialReferenceSystemState{SRID: 100001, Read: true, Exists: false, Absence: true}
	}
	adj := spec.AdjudicateSRSDrop(absent(), 100001, false, spec.SRSDropPrivilegeConsumable, false)
	if adj.Arm != spec.SRSDropArmAbsentRefused || len(adj.Owed) != 0 {
		t.Fatalf("plain: arm=%v owed=%v want absent_refused/no-owed", adj.Arm, adj.Owed)
	}
	adj = spec.AdjudicateSRSDrop(absent(), 100001, false, spec.SRSDropPrivilegeConsumable, true)
	if adj.Arm != spec.SRSDropArmAbsentNoop || len(adj.Owed) != 0 {
		t.Fatalf("ie-consumable: arm=%v owed=%v want absent_noop/no-owed", adj.Arm, adj.Owed)
	}
	for _, premise := range []spec.SRSDropPrivilege{spec.SRSDropPrivilegeUnproven, spec.SRSDropPrivilegeSuppressed} {
		adj = spec.AdjudicateSRSDrop(absent(), 100001, false, premise, true)
		if adj.Arm != spec.SRSDropArmAbsentConditional ||
			len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactDropPrivilege {
			t.Fatalf("premise %v: arm=%v owed=%v want absent_conditional/[srs.drop.privilege]", premise, adj.Arm, adj.Owed)
		}
		if adj.Privilege != premise {
			t.Fatalf("premise %v projected as %v — the projection never rewrites the premise", premise, adj.Privilege)
		}
	}
}

// --- PA-4e2: foreign-SRID state at the real bind seam -------------------------

// srsForeignAbsentProvider is the PA-4e2 custom resolver — it answers
// every SRS lookup with a deliberately foreign-SRID absence state,
// while the privilege loader rides the positive observation.
type srsForeignAbsentProvider struct {
	forwards  []spec.ObjectLookupRequest
	privCalls int
}

func (p *srsForeignAbsentProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "8.0.36"}, nil
}

func (p *srsForeignAbsentProvider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}

func (p *srsForeignAbsentProvider) LoadMySQLServerDropPrivilege(context.Context, spec.Dialect) (*spec.MySQLServerDropPrivilegeObservation, error) {
	p.privCalls++
	return privObservationSUPER(), nil
}

func (p *srsForeignAbsentProvider) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.forwards = append(p.forwards, req)
	return &spec.ObjectSnapshot{
		Schema: req.Schema, Type: req.Type, Name: req.Name,
		SpatialReference: &spec.SpatialReferenceSystemState{
			SRID: 100002, Read: true, Exists: false, Absence: true,
		},
	}, nil
}

// PA-4e2: a foreign-SRID absence at the real bind never selects the A
// arm — ValidAbsence's authored bind fails, the statement lands S2
// with [E,D] owed, and own-apply mints the AUTHORED 100001 only.
// R2-B2 completes the frozen observation scope: premise=consumable is
// asserted directly, L=1 is measured, and the same enrich is carried
// through the real rule projection — the strict S2 privilege notice
// plus the dependency-owned [E,D] gap.
func TestT26SRSAbsentEnrichForeignSRID(t *testing.T) {
	provider := &srsForeignAbsentProvider{}
	state := newSRSState(provider)
	enriched := enrichSRSPriv(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	if len(enriched) != 1 {
		t.Fatalf("enriched=%d want 1", len(enriched))
	}
	adj := srsAdj(t, enriched[0])
	if adj.Arm != spec.SRSDropArmUnproven {
		t.Fatalf("arm=%v want S2 — foreign-SRID absence is not this object's absence", adj.Arm)
	}
	if adj.Privilege != spec.SRSDropPrivilegeConsumable {
		t.Fatalf("premise=%v want consumable — the positive observation still carries, never consumed", adj.Privilege)
	}
	if len(adj.Owed) != 2 || adj.Owed[0] != spec.SRSFactExistence || adj.Owed[1] != spec.SRSFactColumnDependencies {
		t.Fatalf("owed=%v want [srs.existence srs.column_dependencies]", adj.Owed)
	}
	if provider.privCalls != 1 {
		t.Fatalf("L=%d want exactly 1", provider.privCalls)
	}
	sameStaleSet(t, staleSet(state), map[uint32]bool{100001: true}, "own-apply")
	if len(provider.forwards) != 1 {
		t.Fatalf("resolver forwards=%d want exactly 1", len(provider.forwards))
	}
	// The same enrich through the real rule projection: the strict S2
	// privilege notice and the dependency-owned [E,D] gap.
	result := evaluateEnriched(t, enriched)
	if len(result.Statements) != 1 {
		t.Fatalf("result statements=%d want 1", len(result.Statements))
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.mysql.srs.drop.privilege" {
		t.Fatalf("findings=%+v want exactly the S2 privilege notice", stmt.Findings)
	}
	assertSRSPrivilegeNotice(t, stmt, 100001)
	intExactGap(t, stmt, []string{spec.SRSFactExistence, spec.SRSFactColumnDependencies})
}

// --- PA-5: stale channels and own-apply on the A arms -------------------------

// PA-5a/5b/5c: a minted stale flag for a FOREIGN SRID plus a real
// dependency staleness demote the witness while ValidAbsence still
// governs — plain refuses, IF EXISTS no-ops under the consumable
// premise, and the negative-witness premise yields the conditional
// arm; no A arm mints or clears stale state.
func TestT26SRSAbsentStaleChannels(t *testing.T) {
	newState := func(t *testing.T, fx *srsSQLFixture, super bool) (*srsSQLFixture, *srsSQLProvider, *batchState) {
		fixture, provider := newSRSSQLHarness(t, fx, nil)
		if super {
			provider.privObservation = privObservationSUPER()
		}
		state := newSRSState(provider)
		state.srsExistenceStale[77777] = struct{}{}
		state.srsDepsStale = true
		return fixture, provider, state
	}
	fxAbsentDeps := func() *srsSQLFixture {
		return &srsSQLFixture{
			version: "8.0.36",
			deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 100001}},
		}
	}

	t.Run("PA-5a-plain-refused", func(t *testing.T) {
		_, provider, state := newState(t, fxAbsentDeps(), true)
		enriched := enrichSRSPriv(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
		if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmAbsentRefused {
			t.Fatalf("arm=%v want absent_refused — depsStale demotes the witness, absence governs", arm)
		}
		sameStaleSet(t, staleSet(state), map[uint32]bool{77777: true}, "post-apply")
		if !state.srsDepsStale {
			t.Fatal("depsStale must persist — no A arm clears state")
		}
	})

	t.Run("PA-5b-ifexists-noop", func(t *testing.T) {
		_, provider, state := newState(t, fxAbsentDeps(), true)
		enriched := enrichSRSPriv(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
		if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmAbsentNoop {
			t.Fatalf("arm=%v want absent_noop", arm)
		}
		sameStaleSet(t, staleSet(state), map[uint32]bool{77777: true}, "post-apply")
	})

	t.Run("PA-5c-ifexists-unproven", func(t *testing.T) {
		fixture, provider, state := newState(t, fxAbsentDeps(), false)
		provider.privObservation = &spec.MySQLServerDropPrivilegeObservation{
			Account:    "app@%",
			GrantLines: []string{"GRANT SELECT ON *.* TO `app`@`%`"},
		}
		enriched := enrichSRSPriv(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
		adj := srsAdj(t, enriched[0])
		if adj.Arm != spec.SRSDropArmAbsentConditional {
			t.Fatalf("arm=%v want absent_conditional", adj.Arm)
		}
		if len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactDropPrivilege {
			t.Fatalf("owed=%v want exactly [srs.drop.privilege]", adj.Owed)
		}
		sameStaleSet(t, staleSet(state), map[uint32]bool{77777: true}, "post-apply")
		assertReads(t, provider, fixture, 1, 1, 1, 1, 1)
	})

	// PA-5d: the same-SRID stale flag blocks at the bind — zero reads
	// issue and no mint/clear happens.
	t.Run("PA-5d-same-srid-blocked", func(t *testing.T) {
		fixture, provider, state := newState(t, fxAbsentDeps(), true)
		state.srsExistenceStale[100001] = struct{}{}
		enriched := enrichSRSPriv(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
		if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmBlocked {
			t.Fatalf("arm=%v want blocked", arm)
		}
		sameStaleSet(t, staleSet(state), map[uint32]bool{77777: true, 100001: true}, "post-apply")
		assertReads(t, provider, fixture, 0, 0, 0, 0, 1)
	})

	// PA-5e: a positive existence answer under foreign-SRID stale +
	// depsStale lands S2 — own-apply mints the authored SRID alongside
	// the seeded foreign key.
	t.Run("PA-5e-exists-unproven-mints", func(t *testing.T) {
		fx := fxAbsentDeps()
		fx.srs = map[uint32]string{100001: "SRID 100001"}
		_, provider, state := newState(t, fx, true)
		enriched := enrichSRSPriv(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
		adj := srsAdj(t, enriched[0])
		if adj.Arm != spec.SRSDropArmUnproven {
			t.Fatalf("arm=%v want S2 — depsStale demotes the witness, existence is proven present", adj.Arm)
		}
		if len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactColumnDependencies {
			t.Fatalf("owed=%v want [srs.column_dependencies]", adj.Owed)
		}
		sameStaleSet(t, staleSet(state), map[uint32]bool{77777: true, 100001: true}, "post-apply")
	})

	// PA-5f: whole-batch contamination (GARBAGE parse failure) blocks
	// the trailing qualified DROP — never reaches the resolver.
	t.Run("PA-5f-contamination-blocked", func(t *testing.T) {
		fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36"}, nil)
		provider.privObservation = privObservationSUPER()
		state := newSRSState(provider)
		enriched := enrichSRSPriv(t, state, provider,
			"GARBAGE; DROP SPATIAL REFERENCE SYSTEM 100001;")
		if !state.contaminated {
			t.Fatal("GARBAGE prefix must contaminate the batch")
		}
		adj := srsAdj(t, enriched[0])
		if adj.Arm != spec.SRSDropArmBlocked {
			t.Fatalf("arm=%v want blocked", adj.Arm)
		}
		if len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactBatchEffects {
			t.Fatalf("owed=%v want [srs.batch_effects]", adj.Owed)
		}
		sameStaleSet(t, staleSet(state), map[uint32]bool{}, "post-apply")
		assertReads(t, provider, fixture, 0, 0, 0, 0, 1)
	})
}

// --- PA-6d: the object cache is per-enrich — a second call re-reads -----------

// PA-6d: two enrichStatements calls sharing one batchState never
// share the object cache — call2 re-reads and lands S2 while call1's
// proven absence minted nothing.
func TestT26SRSAbsentRecachesPerEnrich(t *testing.T) {
	fx := &srsSQLFixture{version: "8.0.36"}
	fixture, provider := newSRSSQLHarness(t, fx, nil)
	provider.privObservation = privObservationSUPER()
	state := newSRSState(provider)

	before := readsVector(provider, fixture)
	enriched1 := enrichSRSPriv(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := srsArm(t, enriched1[0]); arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("call1 arm=%v want absent_refused", arm)
	}
	sameStaleSet(t, staleSet(state), map[uint32]bool{}, "after call1")
	mid := readsVector(provider, fixture)
	for i, want := range [5]int{1, 1, 1, 1, 1} {
		if got := mid[i] - before[i]; got != want {
			t.Fatalf("call1 delta[%d]=%d want %d (R,Qexist,Qdeps,Qo,L)", i, got, want)
		}
	}

	fx.srs = map[uint32]string{100001: "SRID 100001"}
	enriched2 := enrichSRSPriv(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	adj := srsAdj(t, enriched2[0])
	if adj.Arm != spec.SRSDropArmUnproven {
		t.Fatalf("call2 arm=%v want S2 — the per-enrich cache re-read the now-present row", adj.Arm)
	}
	if len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactColumnDependencies {
		t.Fatalf("call2 owed=%v want [srs.column_dependencies]", adj.Owed)
	}
	sameStaleSet(t, staleSet(state), map[uint32]bool{100001: true}, "after call2")
	after := readsVector(provider, fixture)
	for i, want := range [5]int{1, 1, 1, 1, 1} {
		if got := after[i] - mid[i]; got != want {
			t.Fatalf("call2 delta[%d]=%d want %d (R,Qexist,Qdeps,Qo,L)", i, got, want)
		}
	}
	if provider.privCalls != 2 {
		t.Fatalf("L=%d want 2 — the loader re-evaluates at each enrich entry", provider.privCalls)
	}
}

// --- PA-7a: REVOKE scoping — three-point snapshots ----------------------------

// PA-7a: DROP-IF-EXISTS consumable, REVOKE ends the scope, the next
// DROP-IF-EXISTS on a different SRID lands absent_conditional under
// the suppressed premise — stmt1's adjudication snapshot never
// changes, and no arm mints stale state.
func TestT26SRSAbsentRevokeScoping(t *testing.T) {
	fx := &srsSQLFixture{version: "8.0.36"}
	_, provider := newSRSSQLHarness(t, fx, nil)
	provider.privObservation = privObservationSUPER()
	state := newSRSState(provider)

	var snap1, snap3 srsAdjSnapshot
	var scopeEnded [3]bool
	var staleAt [3]map[uint32]bool
	var depsStaleAt [3]bool
	state.afterApplyObserve = func(i int, stmt *spec.Statement) {
		scopeEnded[i] = state.serverPrivilegeScopeEnded
		staleAt[i] = staleSet(state)
		depsStaleAt[i] = state.srsDepsStale
		if i == 0 {
			snap1 = snapSRSDropAdj(*stmt)
		}
		if i == 2 {
			snap3 = snapSRSDropAdj(*stmt)
		}
	}
	enriched := enrichSRSPriv(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
			"REVOKE SELECT ON *.* FROM 'app'@'%';"+
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100002;")
	if len(enriched) != 3 {
		t.Fatalf("enriched=%d want 3", len(enriched))
	}
	if snap1.arm != spec.SRSDropArmAbsentNoop || snap1.privilege != spec.SRSDropPrivilegeConsumable || len(snap1.owed) != 0 {
		t.Fatalf("stmt1 snapshot=%+v want {absent_noop consumable no-owed}", snap1)
	}
	if snap3.arm != spec.SRSDropArmAbsentConditional || snap3.privilege != spec.SRSDropPrivilegeSuppressed ||
		len(snap3.owed) != 1 || snap3.owed[0] != spec.SRSFactDropPrivilege {
		t.Fatalf("stmt3 snapshot=%+v want {absent_conditional suppressed [srs.drop.privilege]}", snap3)
	}
	snap1.sameAs(t, enriched[0], "stmt1-post-batch")
	snap3.sameAs(t, enriched[2], "stmt3-post-batch")
	for i := 0; i < 3; i++ {
		if scopeEnded[i] != (i > 0) {
			t.Fatalf("scopeEnded[%d]=%v want %v", i, scopeEnded[i], i > 0)
		}
		sameStaleSet(t, staleAt[i], map[uint32]bool{}, "stale")
		if depsStaleAt[i] {
			t.Fatalf("depsStale[%d] must stay false — no A arm mints or clears", i)
		}
	}
	assertReads(t, provider, fx, 2, 2, 2, 2, 1)
}

package audit

// Internal companions for T26-SRS-DROP-PRIVILEGE-EXECUTION-01 — the
// internal-layer halves of the accepted matrix rows (P5 non-consuming
// arms with exact stale-set before/after, P6a stale-source demotion,
// P6b two enrich calls on one batchState, P7 positional suppression
// with the in-position REVOKE observation, P9b/P9c boundary mints,
// P8 sibling isolation in both orders, P5b snapshot stability). They
// observe the real batchState
// through the afterBindObserve/afterApplyObserve seams and never claim
// their snapshots came from the same public API run — that layer is
// proven separately by the audit_test request-matrix file.

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// privObservationSUPER is the internal-layer positive witness — the
// injected request-level observation the real witness derivation
// consumes (account bound, direct global SELECT,SUPER).
func privObservationSUPER() *spec.MySQLServerDropPrivilegeObservation {
	return &spec.MySQLServerDropPrivilegeObservation{
		Account:    "app@%",
		GrantLines: []string{"GRANT SELECT, SUPER ON *.* TO `app`@`%`"},
	}
}

// enrichSRSPriv runs one real enrichStatements call under
// observed_account_new_session — the internal-layer carrier for the
// privilege rows; failures/parse come from the authored SQL only.
func enrichSRSPriv(t *testing.T, state *batchState, provider MetadataProvider, sqlText string) []spec.Statement {
	t.Helper()
	statements, failures := parseT26(t, sqlText)
	enriched, err := state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider,
			PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession},
		statements, failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err != nil {
		t.Fatalf("enrich %q: %v", sqlText, err)
	}
	return enriched
}

// srsAdj returns the statement's attached SRS adjudication — the
// shared projection the SRS rules consume.
func srsAdj(t *testing.T, statement spec.Statement) *spec.SRSDropAdjudication {
	t.Helper()
	if statement.Metadata == nil || statement.Metadata.SRSDrop == nil {
		t.Fatalf("no SRS adjudication attached: %+v", statement.Metadata)
	}
	return statement.Metadata.SRSDrop
}

// srsAdjSnapshot is the independent deep-value capture of one
// adjudication — the immutability witness: it survives later
// statements in the same ordered loop.
type srsAdjSnapshot struct {
	arm       spec.SRSDropArm
	privilege spec.SRSDropPrivilege
	owed      []string
	owedNil   bool
}

func snapSRSDropAdj(statement spec.Statement) srsAdjSnapshot {
	if statement.Metadata == nil || statement.Metadata.SRSDrop == nil {
		return srsAdjSnapshot{arm: -1}
	}
	adj := statement.Metadata.SRSDrop
	return srsAdjSnapshot{
		arm:       adj.Arm,
		privilege: adj.Privilege,
		owed:      append([]string(nil), adj.Owed...),
		owedNil:   adj.Owed == nil,
	}
}

// intExactGap is the internal-layer twin of r3ExactGap — one
// dependency-owned unknown_srs_state gap with the exact ordered fact
// list, evaluated on the rule-evaluation result.
func intExactGap(t *testing.T, stmt report.StatementResult, wantFacts []string) {
	t.Helper()
	if len(stmt.EvidenceGaps) != 1 {
		t.Fatalf("gaps=%+v want exactly one", stmt.EvidenceGaps)
	}
	gap := stmt.EvidenceGaps[0]
	if gap.RuleID != "ddl.mysql.srs.drop.dependency" || gap.ReasonCode != "unknown_srs_state" {
		t.Fatalf("gap=%+v want owner ddl.mysql.srs.drop.dependency / unknown_srs_state", gap)
	}
	if len(gap.RequiredFacts) != len(wantFacts) {
		t.Fatalf("gap facts=%v want ordered %v", gap.RequiredFacts, wantFacts)
	}
	for i := range wantFacts {
		if gap.RequiredFacts[i] != wantFacts[i] {
			t.Fatalf("gap facts[%d]=%q want ordered %v", i, gap.RequiredFacts[i], wantFacts)
		}
	}
}

// staleSet renders the existence-stale map as a comparable key set —
// R2-B requires real set equality, never length equality.
func staleSet(s *batchState) map[uint32]bool {
	out := make(map[uint32]bool, len(s.srsExistenceStale))
	for k := range s.srsExistenceStale {
		out[k] = true
	}
	return out
}

func sameStaleSet(t *testing.T, got, want map[uint32]bool, who string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s stale set=%v want exactly %v", who, got, want)
	}
	for k := range want {
		if !got[k] {
			t.Fatalf("%s stale set=%v missing %d of %v", who, got, k, want)
		}
	}
	for k := range got {
		if !want[k] {
			t.Fatalf("%s stale set=%v unexpected %d (want %v)", who, got, k, want)
		}
	}
}

func (snap srsAdjSnapshot) sameAs(t *testing.T, statement spec.Statement, who string) {
	t.Helper()
	adj := srsAdj(t, statement)
	if adj.Arm != snap.arm || adj.Privilege != snap.privilege {
		t.Fatalf("%s adjudication changed: %+v vs snapshot %+v", who, adj, snap)
	}
	if (adj.Owed == nil) != snap.owedNil || len(adj.Owed) != len(snap.owed) {
		t.Fatalf("%s owed changed: %v vs snapshot %v", who, adj.Owed, snap.owed)
	}
	for i := range snap.owed {
		if adj.Owed[i] != snap.owed[i] {
			t.Fatalf("%s owed[%d] changed: %v vs %v", who, i, adj.Owed[i], snap.owed[i])
		}
	}
}

// --- P6a: authored stale source — depsStale demotes a positive read --------

// P6a internal: `DROP TABLE s.t` (accepted tblTFixture shape) mints
// srsDepsStale at its own apply; the same-batch DROP then reads a
// positive dependency which the stale flag demotes — S2 consumable
// [D], one privilege notice + one gap, and the DROP's own apply mints
// its SRID's existence stale.
func TestT26SRSPrivP6aInternalStaleDemotion(t *testing.T) {
	fx := &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{100001: "SRID 100001"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 100001}},
	}
	_, provider := newSRSSQLHarness(t, fx, tblTFixture())
	provider.privObservation = privObservationSUPER()
	state := newSRSState(provider)

	var depsStaleAfterStmt1 bool
	state.afterApplyObserve = func(i int, _ *spec.Statement) {
		if i == 0 {
			depsStaleAfterStmt1 = state.srsDepsStale
		}
	}
	enriched := enrichSRSPriv(t, state, provider,
		"DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 100001;")
	if len(enriched) != 2 {
		t.Fatalf("enriched=%d want 2", len(enriched))
	}
	if !depsStaleAfterStmt1 {
		t.Fatal("DROP TABLE s.t must mint srsDepsStale at its own apply")
	}
	if state.contaminated {
		t.Fatal("a cleanly-applied touching write contaminates nothing")
	}
	adj := srsAdj(t, enriched[1])
	if adj.Arm != spec.SRSDropArmUnproven {
		t.Fatalf("arm=%v want S2 — positive deps demoted by stale flag, never S1", adj.Arm)
	}
	if adj.Privilege != spec.SRSDropPrivilegeConsumable {
		t.Fatalf("premise=%v want consumable", adj.Privilege)
	}
	if len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactColumnDependencies {
		t.Fatalf("owed=%v want exactly [D] — stale facts stay owed even though positive deps were read", adj.Owed)
	}
	result := evaluateEnriched(t, enriched)
	stmt := result.Statements[1]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.mysql.srs.drop.privilege" {
		t.Fatalf("findings=%+v want exactly the privilege notice", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) != 1 {
		t.Fatalf("gaps=%+v want one gap [D]", stmt.EvidenceGaps)
	}
	if _, ok := state.srsExistenceStale[100001]; !ok {
		t.Fatal("the DROP's own apply must mint srsExistenceStale[100001]")
	}
	assertReads(t, provider, fx, 1, 1, 1, 1, 1)
}

// --- R2-B1: P5 non-consuming arms — attached adjudication + stale sets --------

// B1: every non-consuming arm carries the consumable premise
// projection without using it; the target statement's own apply adds
// no stale of either kind. Stale maps are compared as real key sets
// around the target's own apply.
func TestT26SRSPrivP5ArmsInternalNonConsumption(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sql      string
		srs      map[uint32]string
		deps     []srsDepRow
		stmtIdx  int
		wantArm  spec.SRSDropArm
		wantOwed []string
	}{
		{"P5a contamination B", "GARBAGE; DROP SPATIAL REFERENCE SYSTEM 100001;",
			map[uint32]string{100001: "x"}, nil, 0,
			spec.SRSDropArmBlocked, []string{spec.SRSFactBatchEffects}},
		{"P6a-bis parse B", "CREATE TABLE s.x (g GEOMETRY); DROP SPATIAL REFERENCE SYSTEM 100001;",
			map[uint32]string{100001: "x"}, nil, 0,
			spec.SRSDropArmBlocked, []string{spec.SRSFactBatchEffects}},
		{"P5c S0 plain", "DROP SPATIAL REFERENCE SYSTEM 0;",
			map[uint32]string{0: "0"}, nil, 0,
			spec.SRSDropArmRefusedSRID0, nil},
		{"P5d S0 IF EXISTS", "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 0;",
			map[uint32]string{0: "0"}, nil, 0,
			spec.SRSDropArmRefusedSRID0, nil},
		{"P5e S1 positive deps", "DROP SPATIAL REFERENCE SYSTEM 100001;",
			map[uint32]string{100001: "x"}, []srsDepRow{{schema: "s", table: "t", column: "g", srid: 100001}}, 0,
			spec.SRSDropArmRefusedDependency, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := &srsSQLFixture{version: "8.0.36", srs: tc.srs, deps: tc.deps}
			_, provider := newSRSSQLHarness(t, fx, nil)
			provider.privObservation = privObservationSUPER()
			state := newSRSState(provider)
			var staleBefore, staleAfter map[uint32]bool
			var depsBefore, depsAfter bool
			state.afterApplyObserve = func(i int, _ *spec.Statement) {
				if i == tc.stmtIdx {
					staleAfter = staleSet(state)
					depsAfter = state.srsDepsStale
				}
			}
			state.afterBindObserve = func(i int, _ *spec.Statement) {
				if i == tc.stmtIdx {
					staleBefore = staleSet(state)
					depsBefore = state.srsDepsStale
				}
			}
			enriched := enrichSRSPriv(t, state, provider, tc.sql)
			adj := srsAdj(t, enriched[tc.stmtIdx])
			if adj.Arm != tc.wantArm {
				t.Fatalf("%s arm=%v want %v", tc.name, adj.Arm, tc.wantArm)
			}
			if adj.Privilege != spec.SRSDropPrivilegeConsumable {
				t.Fatalf("%s premise=%v want consumable — carried, never consumed", tc.name, adj.Privilege)
			}
			if len(adj.Owed) != len(tc.wantOwed) {
				t.Fatalf("%s owed=%v want %v", tc.name, adj.Owed, tc.wantOwed)
			}
			for i := range tc.wantOwed {
				if adj.Owed[i] != tc.wantOwed[i] {
					t.Fatalf("%s owed=%v want %v", tc.name, adj.Owed, tc.wantOwed)
				}
			}
			// The TARGET statement's own apply minted nothing —
			// contamination-side state changes are not attributed to it.
			sameStaleSet(t, staleAfter, staleBefore, tc.name+" own-apply stale")
			if depsBefore != depsAfter {
				t.Fatalf("%s depsStale changed %v -> %v across own apply", tc.name, depsBefore, depsAfter)
			}
			if state.serverPrivilegeScopeEnded {
				t.Fatalf("%s scope ended — nothing terminated it", tc.name)
			}
		})
	}
}

// --- P6b: two enrich calls sharing one batchState ----------------------------

// P6b internal: call1 consumes (S2 [D], mints existence stale); call2
// reruns the entry gate against the same batchState — the same-SRID
// stale keeps it at B, the loader reruns (delta L=1) while every SRS
// read stays a zero delta; call1's captured adjudication survives.
func TestT26SRSPrivP6bInternalDoubleEnrich(t *testing.T) {
	fx := &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{100001: "SRID 100001"},
	}
	_, provider := newSRSSQLHarness(t, fx, nil)
	provider.privObservation = privObservationSUPER()
	state := newSRSState(provider)

	// call1
	var snap1 srsAdjSnapshot
	var captured bool
	state.afterApplyObserve = func(i int, stmt *spec.Statement) {
		if i == 0 && !captured {
			snap1 = snapSRSDropAdj(*stmt)
			captured = true
		}
	}
	call1 := enrichSRSPriv(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	state.afterApplyObserve = nil // call1's snapshot is sealed — call2 must never overwrite it
	adj1 := srsAdj(t, call1[0])
	if adj1.Arm != spec.SRSDropArmUnproven || adj1.Privilege != spec.SRSDropPrivilegeConsumable ||
		len(adj1.Owed) != 1 || adj1.Owed[0] != spec.SRSFactColumnDependencies {
		t.Fatalf("call1 adjudication=%+v want S2 consumable [D]", adj1)
	}
	assertReads(t, provider, fx, 1, 1, 1, 1, 1)
	staleAfterCall1 := staleSet(state)
	sameStaleSet(t, staleAfterCall1, map[uint32]bool{100001: true}, "call1 post-apply stale")
	depsStaleAfterCall1 := state.srsDepsStale
	before := readsVector(provider, fx)

	// call2 — a fresh enrichStatements entry on the same batchState:
	// the gate reruns (L delta 1) but the stale SRID stops every
	// SRS-family read before it happens.
	call2 := enrichSRSPriv(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	adj2 := srsAdj(t, call2[0])
	if adj2.Arm != spec.SRSDropArmBlocked {
		t.Fatalf("call2 arm=%v want B — the same-SRID stale blocks the read", adj2.Arm)
	}
	if adj2.Privilege != spec.SRSDropPrivilegeConsumable {
		t.Fatalf("call2 premise=%v want consumable — B keeps the premise unconsumed", adj2.Privilege)
	}
	// R2-B2: B owes its blocking fact — Owed is exactly [B].
	if len(adj2.Owed) != 1 || adj2.Owed[0] != spec.SRSFactBatchEffects {
		t.Fatalf("call2 owed=%v want exactly [srs.batch_effects]", adj2.Owed)
	}
	after := readsVector(provider, fx)
	delta := [5]int{after[0] - before[0], after[1] - before[1], after[2] - before[2], after[3] - before[3], after[4] - before[4]}
	if delta != [5]int{0, 0, 0, 0, 1} {
		t.Fatalf("call2 read delta=%v want 0/0/0/0/1 — only the loader reruns", delta)
	}
	// R2-B2: call2's own apply mints nothing — the post-call-2 stale
	// set equals the call-1 post-state exactly; deps stale untouched.
	sameStaleSet(t, staleSet(state), staleAfterCall1, "call2 post-apply stale")
	if state.srsDepsStale != depsStaleAfterCall1 {
		t.Fatalf("depsStale changed %v -> %v across call2", depsStaleAfterCall1, state.srsDepsStale)
	}
	snap1.sameAs(t, call1[0], "call1 adjudication")
	if state.serverPrivilegeScopeEnded {
		t.Fatal("scope must stay open — nothing terminated it")
	}
	// Public projections for each call are separately named evaluations
	// over the same enriched statements — they are not claimed to come
	// from one public request.
	r1 := evaluateEnriched(t, call1)
	s1 := r1.Statements[0]
	if len(s1.Findings) != 1 || s1.Findings[0].RuleID != "ddl.mysql.srs.drop.privilege" {
		t.Fatalf("call1 findings=%+v want the privilege notice", s1.Findings)
	}
	intExactGap(t, s1, []string{spec.SRSFactColumnDependencies})
	if s1.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("call1 coverage=%v want unverified", s1.Coverage.Status)
	}
	r2 := evaluateEnriched(t, call2)
	s2 := r2.Statements[0]
	if len(s2.Findings) != 0 {
		t.Fatalf("call2 findings=%+v want 0 on B", s2.Findings)
	}
	intExactGap(t, s2, []string{spec.SRSFactBatchEffects})
	if s2.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("call2 coverage=%v want unverified", s2.Coverage.Status)
	}
}

// --- P7: positional suppression + the premise truth table --------------------

// P7 internal (R2-B3): REVOKE ends the observed scope at its loop
// position — the first DROP consumed, the third's premise is
// suppressed and stays owed [D,P]; stmt1's post-apply snapshot
// survives stmt3. The observation after stmt2 (REVOKE) proves the
// scope end is minted exactly there and that REVOKE touches no SRS
// staleness — the stale set at that position equals stmt1's
// post-apply set, and depsStale is unchanged by REVOKE.
func TestT26SRSPrivP7InternalSuppression(t *testing.T) {
	fx := &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{100001: "SRID 100001", 100002: "SRID 100002"},
	}
	_, provider := newSRSSQLHarness(t, fx, nil)
	provider.privObservation = privObservationSUPER()
	state := newSRSState(provider)

	var snap1 srsAdjSnapshot
	var staleAfterStmt1 map[uint32]bool
	var depsStaleAfterStmt1, depsStaleAfterRevoke bool
	var scopeEndedAfterRevoke bool
	var staleAfterRevoke map[uint32]bool
	state.afterApplyObserve = func(i int, stmt *spec.Statement) {
		switch i {
		case 0:
			snap1 = snapSRSDropAdj(*stmt)
			staleAfterStmt1 = staleSet(state)
			depsStaleAfterStmt1 = state.srsDepsStale
		case 1: // in-position observation after REVOKE, before stmt3 binds
			scopeEndedAfterRevoke = state.serverPrivilegeScopeEnded
			staleAfterRevoke = staleSet(state)
			depsStaleAfterRevoke = state.srsDepsStale
		}
	}
	enriched := enrichSRSPriv(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001; REVOKE SELECT ON *.* FROM 'r1'@'%'; DROP SPATIAL REFERENCE SYSTEM 100002;")
	if len(enriched) != 3 {
		t.Fatalf("enriched=%d want 3", len(enriched))
	}
	adj1 := srsAdj(t, enriched[0])
	if adj1.Arm != spec.SRSDropArmUnproven || adj1.Privilege != spec.SRSDropPrivilegeConsumable {
		t.Fatalf("stmt1=%+v want S2 consumable — scope still open at its position", adj1)
	}
	// stmt1's own apply minted exactly its own SRID.
	sameStaleSet(t, staleAfterStmt1, map[uint32]bool{100001: true}, "stmt1 post-apply stale")
	// R2-B3: at the REVOKE position the scope is already ended, while
	// the stale set is exactly stmt1's post-state — REVOKE mints
	// nothing SRS-side.
	if !scopeEndedAfterRevoke {
		t.Fatal("serverPrivilegeScopeEnded must already be true immediately after REVOKE")
	}
	sameStaleSet(t, staleAfterRevoke, staleAfterStmt1, "stale after REVOKE")
	if depsStaleAfterRevoke != depsStaleAfterStmt1 {
		t.Fatalf("depsStale changed across REVOKE %v -> %v", depsStaleAfterStmt1, depsStaleAfterRevoke)
	}
	adj3 := srsAdj(t, enriched[2])
	if adj3.Privilege != spec.SRSDropPrivilegeSuppressed {
		t.Fatalf("stmt3 premise=%v want suppressed", adj3.Privilege)
	}
	if adj3.Arm != spec.SRSDropArmUnproven {
		t.Fatalf("stmt3 arm=%v want S2 — suppression only keeps P owed", adj3.Arm)
	}
	if len(adj3.Owed) != 2 || adj3.Owed[0] != spec.SRSFactColumnDependencies || adj3.Owed[1] != spec.SRSFactDropPrivilege {
		t.Fatalf("stmt3 owed=%v want ordered [D,P]", adj3.Owed)
	}
	snap1.sameAs(t, enriched[0], "stmt1 adjudication")
	// Final set is exactly the two authored SRIDs — each DROP's own
	// apply minted its own SRID, REVOKE minted none.
	sameStaleSet(t, staleSet(state), map[uint32]bool{100001: true, 100002: true}, "final stale")
	assertReads(t, provider, fx, 2, 2, 2, 2, 1)
}

// Minimal premise unit: the four derivations of
// (*batchState).srsDropPrivilegePremise() — including the frozen edge
// that a scope end WITHOUT a positive witness stays unproven (never
// suppressed).
func TestT26SRSPrivPremiseTruthTable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		witness bool
		ended   bool
		want    spec.SRSDropPrivilege
	}{
		{"no witness, open", false, false, spec.SRSDropPrivilegeUnproven},
		{"no witness, ended", false, true, spec.SRSDropPrivilegeUnproven},
		{"witness, open", true, false, spec.SRSDropPrivilegeConsumable},
		{"witness, ended", true, true, spec.SRSDropPrivilegeSuppressed},
	} {
		state := &batchState{
			serverDropPrivilegeWitness: tc.witness,
			serverPrivilegeScopeEnded:  tc.ended,
		}
		if got := state.srsDropPrivilegePremise(); got != tc.want {
			t.Fatalf("%s: premise=%v want %v", tc.name, got, tc.want)
		}
	}
}

// --- P9b internal: boundary apply mints existence stale ----------------------

// P9b internal companion: below-floor resolved version gates the SRS
// statement out — the boundary apply still mints its SRID's existence
// stale (never deps stale, never contamination).
func TestT26SRSPrivP9bInternalBoundaryMint(t *testing.T) {
	fx := &srsSQLFixture{
		// R2-A4: the loader's select version() banner is the same
		// 8.0.3 the request resolved — the old 8.0.36-loader/8.0.3-resolved
		// pairing survives only as a documented historical variant.
		version: "8.0.3-log",
		srs:     map[uint32]string{100001: "SRID 100001"},
	}
	_, provider := newSRSSQLHarness(t, fx, nil)
	provider.privObservation = privObservationSUPER()
	state := newSRSState(provider)
	state.resolvedVersion = &spec.VersionIdentity{
		Product: spec.VersionProductMySQL, Version: "8.0.3",
		Major: 8, Minor: 0, Patch: 3, ValidatedRange: true,
	}
	enriched := enrichSRSPriv(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if len(enriched) != 1 || enriched[0].Unsupported == nil {
		t.Fatalf("enriched=%+v want the version-gated boundary mark", enriched)
	}
	if _, ok := state.srsExistenceStale[100001]; !ok {
		t.Fatal("boundary apply must mint srsExistenceStale[100001]")
	}
	if state.srsDepsStale {
		t.Fatal("boundary apply must never mint deps stale")
	}
	if state.contaminated {
		t.Fatal("a boundary statement contaminates nothing")
	}
}

// P9c internal companion (R2-B5 — COUNTEREXAMPLE): the frozen
// expectation asked for srsExistenceStale[100001] minted by the TiDB
// vendor-boundary own-apply. The real product chain contradicts it:
// the TiDB vendor boundary never sets statement.Unsupported (it lives
// only in the computed coverage aspects projected to
// result.Unsupported), so the apply dispatch reaches
// applySRSStatement with no attached adjudication — minting nothing.
// This test pins the actual behavior as the bounded counterexample
// for ruling, rather than asserting a mint that does not happen.
func TestT26SRSPrivP9cInternalVendorBoundaryMint(t *testing.T) {
	fx := &srsSQLFixture{
		version: "8.0.11-TiDB-v8.5.0",
		srs:     map[uint32]string{100001: "SRID 100001"},
	}
	_, provider := newSRSSQLHarness(t, fx, nil)
	provider.privObservation = privObservationSUPER()
	state := newBatchState(spec.DialectTiDB, "golden", provider)
	state.resolvedVersion = &spec.VersionIdentity{
		Product: spec.VersionProductTiDB, Version: "8.0.11-TiDB-v8.5.0",
		ValidatedRange: true,
	}
	state.instanceFacts = &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0"}
	enriched := enrichSRSPriv(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if len(enriched) != 1 {
		t.Fatalf("enriched=%d want 1", len(enriched))
	}
	// The vendor boundary is an aspect-only mark: statement.Unsupported
	// stays nil (it is set only for version-gated unaudited rows,
	// metadata.go:274-287), and no SRS adjudication is attached under
	// TiDB (the bind block is mysql-gated, metadata.go:702).
	if enriched[0].Unsupported != nil {
		t.Fatalf("unsupported=%+v want nil — vendor boundary lives in coverage aspects, not the marker", enriched[0].Unsupported)
	}
	if enriched[0].Metadata != nil && enriched[0].Metadata.SRSDrop != nil {
		t.Fatalf("SRS adjudication=%+v want none — a boundary row is never adjudicated", enriched[0].Metadata.SRSDrop)
	}
	// Counterexample to the R2-B5 mint expectation: own-apply mints
	// nothing — the Unsupported!=nil branch (batch_state.go:481-495)
	// is never reached, applySRSStatement requires an attached S2
	// adjudication, and spatialTouching exits on the family marker.
	sameStaleSet(t, staleSet(state), map[uint32]bool{}, "vendor-boundary stale — no mint")
	if state.srsDepsStale {
		t.Fatal("vendor-boundary apply must never mint deps stale")
	}
	if state.contaminated {
		t.Fatal("a boundary statement contaminates nothing")
	}
}

// --- P8 internal: sibling independence ----------------------------------------

// P8 internal companion: SERVER and SRS each project their own premise
// from the same request-level observation — SERVER's consumable arm
// publishes its ordered post-state; the SRS row's own premise is
// independently consumable, its arm S2.
func TestT26SRSPrivP8InternalSiblingIndependence(t *testing.T) {
	fx := &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{100001: "SRID 100001"},
		servers: map[string]bool{"s1": true},
	}
	_, provider := newSRSSQLHarness(t, fx, nil)
	provider.privObservation = privObservationSUPER()
	state := newSRSState(provider)
	enriched := enrichSRSPriv(t, state, provider,
		"DROP SERVER s1; DROP SPATIAL REFERENCE SYSTEM 100001;")
	if len(enriched) != 2 {
		t.Fatalf("enriched=%d want 2", len(enriched))
	}
	srvAdj := enriched[0].Metadata.ServerDrop
	if srvAdj == nil {
		t.Fatalf("no SERVER adjudication attached: %+v", enriched[0].Metadata)
	}
	if srvAdj.Privilege != spec.ServerDropPrivilegeConsumable {
		t.Fatalf("SERVER premise=%v want consumable", srvAdj.Privilege)
	}
	adj := srsAdj(t, enriched[1])
	if adj.Privilege != spec.SRSDropPrivilegeConsumable || adj.Arm != spec.SRSDropArmUnproven ||
		len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactColumnDependencies {
		t.Fatalf("SRS adjudication=%+v want independent S2 consumable [D]", adj)
	}
	entry := state.serverEntryFor("s1")
	if entry == nil || !entry.absent {
		t.Fatalf("SERVER entry=%+v want OrderedAbsent published by its own consumable arm", entry)
	}
	// R2-B4: per-family post-state — SRS's own apply minted its own
	// SRID's existence stale; deps stale untouched; scope open.
	sameStaleSet(t, staleSet(state), map[uint32]bool{100001: true}, "SRS-first post-state")
	if state.srsDepsStale {
		t.Fatal("neither family's apply mints deps stale")
	}
	if state.serverPrivilegeScopeEnded {
		t.Fatal("scope stays open")
	}
	assertReads(t, provider, fx, 1, 1, 1, 1, 1)
}

// P8b internal: the reversed order — the SRS statement binds/consumes
// first (mints its own stale), SERVER's own consumable arm still
// publishes OrderedAbsent at its position. Same per-family post-state
// shape, independent projections.
func TestT26SRSPrivP8bInternalSiblingReversed(t *testing.T) {
	fx := &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{100001: "SRID 100001"},
		servers: map[string]bool{"s1": true},
	}
	_, provider := newSRSSQLHarness(t, fx, nil)
	provider.privObservation = privObservationSUPER()
	state := newSRSState(provider)
	enriched := enrichSRSPriv(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001; DROP SERVER s1;")
	if len(enriched) != 2 {
		t.Fatalf("enriched=%d want 2", len(enriched))
	}
	adj := srsAdj(t, enriched[0])
	if adj.Privilege != spec.SRSDropPrivilegeConsumable || adj.Arm != spec.SRSDropArmUnproven ||
		len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactColumnDependencies {
		t.Fatalf("SRS adjudication=%+v want independent S2 consumable [D]", adj)
	}
	srvAdj := enriched[1].Metadata.ServerDrop
	if srvAdj == nil {
		t.Fatalf("no SERVER adjudication attached: %+v", enriched[1].Metadata)
	}
	if srvAdj.Privilege != spec.ServerDropPrivilegeConsumable {
		t.Fatalf("SERVER premise=%v want consumable — SRS never consumes the shared witness", srvAdj.Privilege)
	}
	entry := state.serverEntryFor("s1")
	if entry == nil || !entry.absent {
		t.Fatalf("SERVER entry=%+v want OrderedAbsent published by its own consumable arm", entry)
	}
	sameStaleSet(t, staleSet(state), map[uint32]bool{100001: true}, "SRS-first post-state")
	if state.srsDepsStale {
		t.Fatal("neither family's apply mints deps stale")
	}
	assertReads(t, provider, fx, 1, 1, 1, 1, 1)
}

// --- P5b internal: first statement's snapshot survives the second ------------

// P5b internal companion: the consumed first DROP mints its own
// existence stale; the second lands B — its premise is still the
// consumable projection (B carries but never consumes); stmt1's
// post-apply snapshot is unchanged after stmt2; scope stays open.
func TestT26SRSPrivP5bInternalSnapshotStable(t *testing.T) {
	fx := &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{100001: "SRID 100001"},
	}
	_, provider := newSRSSQLHarness(t, fx, nil)
	provider.privObservation = privObservationSUPER()
	state := newSRSState(provider)

	var snap1 srsAdjSnapshot
	var staleAfterStmt1 map[uint32]bool
	state.afterApplyObserve = func(i int, stmt *spec.Statement) {
		if i == 0 {
			snap1 = snapSRSDropAdj(*stmt)
			staleAfterStmt1 = staleSet(state)
		}
	}
	enriched := enrichSRSPriv(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;")
	if len(enriched) != 2 {
		t.Fatalf("enriched=%d want 2", len(enriched))
	}
	sameStaleSet(t, staleAfterStmt1, map[uint32]bool{100001: true}, "stmt1 post-apply stale")
	adj2 := srsAdj(t, enriched[1])
	if adj2.Arm != spec.SRSDropArmBlocked || adj2.Privilege != spec.SRSDropPrivilegeConsumable {
		t.Fatalf("stmt2=%+v want B + consumable premise (carried, never consumed)", adj2)
	}
	// B owes exactly its blocking fact; the blocked statement minted
	// nothing — final set equals stmt1's post-state exactly.
	if len(adj2.Owed) != 1 || adj2.Owed[0] != spec.SRSFactBatchEffects {
		t.Fatalf("stmt2 owed=%v want exactly [srs.batch_effects]", adj2.Owed)
	}
	sameStaleSet(t, staleSet(state), staleAfterStmt1, "final stale after blocked stmt2")
	snap1.sameAs(t, enriched[0], "stmt1 adjudication")
	if state.serverPrivilegeScopeEnded {
		t.Fatal("scope stays open across the blocked statement")
	}
	assertReads(t, provider, fx, 1, 1, 1, 1, 1)
}

// --- P2 internal: own-apply lifecycle on the consuming path -------------------

// P2 internal companion: the consumable S2 arm's own apply mints the
// authored SRID's existence staleness — the lifecycle write the
// request-layer row cannot observe directly.
func TestT26SRSPrivP2InternalOwnApplyStale(t *testing.T) {
	fx := &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{100001: "SRID 100001"},
	}
	_, provider := newSRSSQLHarness(t, fx, nil)
	provider.privObservation = privObservationSUPER()
	state := newSRSState(provider)
	enriched := enrichSRSPriv(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	adj := srsAdj(t, enriched[0])
	if adj.Privilege != spec.SRSDropPrivilegeConsumable || adj.Arm != spec.SRSDropArmUnproven {
		t.Fatalf("adjudication=%+v want S2 consumable", adj)
	}
	if _, ok := state.srsExistenceStale[100001]; !ok {
		t.Fatal("consumable S2 must mint its authored SRID's existence stale")
	}
	result := evaluateEnriched(t, enriched)
	if result.Verdict != report.VerdictReview {
		t.Fatalf("verdict=%v want review", result.Verdict)
	}
}

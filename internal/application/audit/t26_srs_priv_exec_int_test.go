package audit

// Internal companions for T26-SRS-DROP-PRIVILEGE-EXECUTION-01 — the
// internal-layer halves of the accepted matrix rows (P6a stale-source
// demotion, P6b two enrich calls on one batchState, P7 positional
// suppression + minimal premise unit, P9b boundary mint, P8 sibling
// isolation, P5b snapshot stability). They observe the real batchState
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
	if _, ok := state.srsExistenceStale[100001]; !ok {
		t.Fatal("call1 apply must mint existence stale")
	}
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
	after := readsVector(provider, fx)
	delta := [5]int{after[0] - before[0], after[1] - before[1], after[2] - before[2], after[3] - before[3], after[4] - before[4]}
	if delta != [5]int{0, 0, 0, 0, 1} {
		t.Fatalf("call2 read delta=%v want 0/0/0/0/1 — only the loader reruns", delta)
	}
	snap1.sameAs(t, call1[0], "call1 adjudication")
	if state.serverPrivilegeScopeEnded {
		t.Fatal("scope must stay open — nothing terminated it")
	}
	// Public projections for each call are separately named evaluations
	// over the same enriched statements — they are not claimed to come
	// from one public request.
	r1 := evaluateEnriched(t, call1)
	if len(r1.Statements[0].Findings) != 1 || r1.Statements[0].Findings[0].RuleID != "ddl.mysql.srs.drop.privilege" {
		t.Fatalf("call1 findings=%+v want the privilege notice", r1.Statements[0].Findings)
	}
	r2 := evaluateEnriched(t, call2)
	if len(r2.Statements[0].Findings) != 0 {
		t.Fatalf("call2 findings=%+v want 0 on B", r2.Statements[0].Findings)
	}
}

// --- P7: positional suppression + the premise truth table --------------------

// P7 internal: REVOKE ends the observed scope at its loop position —
// the first DROP consumed, the third's premise is suppressed and stays
// owed [D,P]; stmt1's post-apply snapshot survives stmt3. The REVOKE
// mints no SRS staleness.
func TestT26SRSPrivP7InternalSuppression(t *testing.T) {
	fx := &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{100001: "SRID 100001", 100002: "SRID 100002"},
	}
	_, provider := newSRSSQLHarness(t, fx, nil)
	provider.privObservation = privObservationSUPER()
	state := newSRSState(provider)

	var snap1 srsAdjSnapshot
	state.afterApplyObserve = func(i int, stmt *spec.Statement) {
		if i == 0 {
			snap1 = snapSRSDropAdj(*stmt)
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
	if !state.serverPrivilegeScopeEnded {
		t.Fatal("REVOKE must mint the ordered scope end")
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
	if len(state.srsExistenceStale) != 2 {
		t.Fatalf("existenceStale=%v want exactly the two authored SRIDs — REVOKE mints no SRS staleness", state.srsExistenceStale)
	}
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
		version: "8.0.36",
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
	var staleAfterStmt1 int
	state.afterApplyObserve = func(i int, stmt *spec.Statement) {
		if i == 0 {
			snap1 = snapSRSDropAdj(*stmt)
			staleAfterStmt1 = len(state.srsExistenceStale)
		}
	}
	enriched := enrichSRSPriv(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;")
	if len(enriched) != 2 {
		t.Fatalf("enriched=%d want 2", len(enriched))
	}
	if staleAfterStmt1 != 1 {
		t.Fatalf("existenceStale after stmt1=%d want 1 — own apply minted exactly one", staleAfterStmt1)
	}
	adj2 := srsAdj(t, enriched[1])
	if adj2.Arm != spec.SRSDropArmBlocked || adj2.Privilege != spec.SRSDropPrivilegeConsumable {
		t.Fatalf("stmt2=%+v want B + consumable premise (carried, never consumed)", adj2)
	}
	if len(state.srsExistenceStale) != 1 {
		t.Fatalf("existenceStale=%v want still 1 — the blocked statement mints nothing", state.srsExistenceStale)
	}
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

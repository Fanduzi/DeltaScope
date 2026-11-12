package audit

// T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01 — internal-layer
// ordered-channel witnesses: the OrderedAbsentConsumed predicate, the
// mint/keep/barrier table over applySRSStatement (synthetic facts —
// interface-level controls, labeled internal per the work order), the
// S1-over-ordered-A adjudication order, and the JSON non-leak of the
// internal marker.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// opDropStmt builds a synthetic audited drop with the supplied bound
// state and adjudication — internal-only fixture for the apply table.
func opDropStmt(srid uint32, bound *spec.SpatialReferenceSystemState, adj *spec.SRSDropAdjudication) spec.Statement {
	return spec.Statement{
		DDL:      &spec.DDL{Operation: spec.DDLOperationDropSpatialReferenceSystem, SpatialReference: &spec.SpatialReferenceDeclaration{Kind: spec.SRSKindDrop, SRID: srid}},
		Metadata: &spec.Metadata{SpatialReference: bound, SRSDrop: adj},
	}
}

func opAbsenceBound(srid uint32) *spec.SpatialReferenceSystemState {
	return &spec.SpatialReferenceSystemState{SRID: srid, Read: true, Exists: false, Absence: true}
}

// OrderedAbsentConsumed: marker + authored-SRID equality, nothing
// else — no barrier re-check at adjudication, no fabrication.
func TestT26SRSOrderedConsumedPredicate(t *testing.T) {
	cases := []struct {
		name  string
		state *spec.SpatialReferenceSystemState
		want  bool
	}{
		{"nil-state", nil, false},
		{"no-marker", &spec.SpatialReferenceSystemState{SRID: 100001}, false},
		{"hit", &spec.SpatialReferenceSystemState{SRID: 100001, OrderedAbsent: true}, true},
		{"foreign-srid", &spec.SpatialReferenceSystemState{SRID: 200002, OrderedAbsent: true}, false},
	}
	for _, c := range cases {
		if got := spec.OrderedAbsentConsumed(c.state, 100001); got != c.want {
			t.Fatalf("%s: OrderedAbsentConsumed=%v want %v", c.name, got, c.want)
		}
	}
	// Adjudication order: a valid positive dependency witness outranks
	// the ordered A slot (S1 first-match wins even on an ordered hit).
	witness := &spec.SpatialReferenceSystemState{
		SRID:          100001,
		OrderedAbsent: true,
		Dependencies: &spec.SRSDependencyFact{
			Columns: []spec.SRSDependentColumn{{Schema: "s", Table: "t", Column: "g", SRID: 100001}},
		},
	}
	adj := spec.AdjudicateSRSDrop(witness, 100001, false, spec.SRSDropPrivilegeConsumable, false)
	if adj.Arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("arm=%v want refused_dependency — S1 outranks ordered A", adj.Arm)
	}
	// The same witness demoted by depsStale falls through to the
	// ordered A slot.
	adj = spec.AdjudicateSRSDrop(witness, 100001, true, spec.SRSDropPrivilegeConsumable, false)
	if adj.Arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("arm=%v want absent_refused — demoted witness, ordered A governs", adj.Arm)
	}
	// A-IE? on an ordered hit owes exactly the privilege fact.
	adj = spec.AdjudicateSRSDrop(&spec.SpatialReferenceSystemState{SRID: 100001, OrderedAbsent: true},
		100001, false, spec.SRSDropPrivilegeUnproven, true)
	if adj.Arm != spec.SRSDropArmAbsentConditional ||
		len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactDropPrivilege {
		t.Fatalf("adj=%+v want absent_conditional owing exactly [srs.drop.privilege]", adj)
	}
	// An invalid per-column witness (a column declaring a foreign
	// SRID) never enters S1 — the ordered A slot still consumes.
	invalid := &spec.SpatialReferenceSystemState{
		SRID:          100001,
		OrderedAbsent: true,
		Dependencies: &spec.SRSDependencyFact{
			Columns: []spec.SRSDependentColumn{{Schema: "s", Table: "t", Column: "g", SRID: 200002}},
		},
	}
	adj = spec.AdjudicateSRSDrop(invalid, 100001, false, spec.SRSDropPrivilegeConsumable, false)
	if adj.Arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("arm=%v want absent_refused — invalid witness never blocks the ordered A", adj.Arm)
	}
}

// orderedSRSDropEligible: the admission gate's conjuncts — MySQL
// dialect, drop kind, no unsupported marker, audited resolvedVersion.
func TestT26SRSOrderedEligibilityInt(t *testing.T) {
	drop := func() spec.Statement {
		return opDropStmt(100001, nil, nil)
	}
	mysql := newSRSState(nil)
	if !mysql.orderedSRSDropEligible(drop()) {
		t.Fatal("audited MySQL drop under the golden version must be eligible")
	}
	tidb := newBatchState(spec.DialectTiDB, "golden", nil)
	tidb.resolvedVersion = srsGoldenVersion
	if tidb.orderedSRSDropEligible(drop()) {
		t.Fatal("a TiDB statement is never ordered-eligible")
	}
	boundary := drop()
	boundary.Unsupported = &spec.UnsupportedDetail{Feature: "x", Reason: spec.UnsupportedUnauditedReason}
	if mysql.orderedSRSDropEligible(boundary) {
		t.Fatal("a boundary statement is never ordered-eligible")
	}
	out := newSRSState(nil)
	out.resolvedVersion = &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.3", Major: 8, Minor: 0, Patch: 3}
	if out.orderedSRSDropEligible(drop()) {
		t.Fatal("an out-of-domain resolvedVersion is never ordered-eligible")
	}
	unknown := newBatchState(spec.DialectMySQL, "golden", nil)
	if unknown.orderedSRSDropEligible(drop()) {
		t.Fatal("an unknown resolvedVersion is never ordered-eligible")
	}
}

// Mint/keep/barrier over the real own-apply — synthetic bound facts,
// internal layer only.
func TestT26SRSOrderedApplyTableInt(t *testing.T) {
	state := newSRSState(nil)

	// Mint: catalog-certified A-plain publishes the entry.
	state.applyStmtIndex = 0
	if err := state.applySRSStatement(opDropStmt(100001, opAbsenceBound(100001),
		&spec.SRSDropAdjudication{Arm: spec.SRSDropArmAbsentRefused, Privilege: spec.SRSDropPrivilegeUnproven})); err != nil {
		t.Fatal(err)
	}
	entry := state.srsOrdered[100001]
	if entry == nil || !entry.absent || entry.barrier ||
		entry.sourceArm != spec.SRSDropArmAbsentRefused ||
		entry.ownPrivilege != spec.SRSDropPrivilegeUnproven || entry.publishedIdx != 0 {
		t.Fatalf("minted entry=%+v", entry)
	}

	// Keep: a later catalog-certified A-IE? for the same SRID never
	// re-mints — first provenance (arm/premise/index) stands.
	state.applyStmtIndex = 3
	if err := state.applySRSStatement(opDropStmt(100001, opAbsenceBound(100001),
		&spec.SRSDropAdjudication{Arm: spec.SRSDropArmAbsentConditional, Privilege: spec.SRSDropPrivilegeConsumable})); err != nil {
		t.Fatal(err)
	}
	if got := state.srsOrdered[100001]; got.sourceArm != spec.SRSDropArmAbsentRefused ||
		got.ownPrivilege != spec.SRSDropPrivilegeUnproven || got.publishedIdx != 0 {
		t.Fatalf("keep violated — entry rewritten: %+v", got)
	}

	// Ordered-served A (marker, no ValidAbsence) consumes — no mint,
	// no rewrite, for a SRID WITH and WITHOUT an entry.
	if err := state.applySRSStatement(opDropStmt(100001,
		&spec.SpatialReferenceSystemState{SRID: 100001, OrderedAbsent: true},
		&spec.SRSDropAdjudication{Arm: spec.SRSDropArmAbsentNoop})); err != nil {
		t.Fatal(err)
	}
	if got := state.srsOrdered[100001]; got.sourceArm != spec.SRSDropArmAbsentRefused || got.publishedIdx != 0 {
		t.Fatalf("ordered-served A must not rewrite provenance: %+v", got)
	}
	if err := state.applySRSStatement(opDropStmt(300003,
		&spec.SpatialReferenceSystemState{SRID: 300003, OrderedAbsent: true},
		&spec.SRSDropAdjudication{Arm: spec.SRSDropArmAbsentNoop})); err != nil {
		t.Fatal(err)
	}
	if _, ok := state.srsOrdered[300003]; ok {
		t.Fatal("an ordered-served A must never mint a new entry")
	}

	// Contradictory state: a bound SRID that does not match the
	// authored SRID fails ValidAbsence — no mint for the declared SRID
	// (internal control; the real chain always binds authored SRIDs).
	if err := state.applySRSStatement(opDropStmt(400004, opAbsenceBound(400005),
		&spec.SRSDropAdjudication{Arm: spec.SRSDropArmAbsentRefused})); err != nil {
		t.Fatal(err)
	}
	if _, ok := state.srsOrdered[400004]; ok {
		t.Fatal("a contradictory bound SRID must not mint for the authored SRID")
	}

	// Non-producer arms write nothing: B/S0/S1 adjudications on an
	// absent-certified bound state mint no entry.
	for _, arm := range []spec.SRSDropArm{spec.SRSDropArmBlocked, spec.SRSDropArmRefusedSRID0, spec.SRSDropArmRefusedDependency} {
		if err := state.applySRSStatement(opDropStmt(500005, opAbsenceBound(500005),
			&spec.SRSDropAdjudication{Arm: arm})); err != nil {
			t.Fatal(err)
		}
		if _, ok := state.srsOrdered[500005]; ok {
			t.Fatalf("arm %v must not mint an ordered entry", arm)
		}
	}

	// Barrier + stale: an S2 own-apply mints the SRID's stale flag and
	// flips the existing entry's barrier — provenance kept (reachable
	// invariant barrier ⇒ srsExistenceStale[srid]).
	if err := state.applySRSStatement(opDropStmt(100001, &spec.SpatialReferenceSystemState{SRID: 100001, Read: true, Exists: true},
		&spec.SRSDropAdjudication{Arm: spec.SRSDropArmUnproven})); err != nil {
		t.Fatal(err)
	}
	entry = state.srsOrdered[100001]
	if entry == nil || !entry.barrier || !entry.absent ||
		entry.sourceArm != spec.SRSDropArmAbsentRefused || entry.publishedIdx != 0 {
		t.Fatalf("barrier'd entry=%+v want provenance kept", entry)
	}
	if _, ok := state.srsExistenceStale[100001]; !ok {
		t.Fatal("barrier ⇒ srsExistenceStale[srid] must hold")
	}
	// S2 on a SRID with no entry mints stale without touching the
	// channel.
	if err := state.applySRSStatement(opDropStmt(600006, &spec.SpatialReferenceSystemState{SRID: 600006, Read: true, Exists: true},
		&spec.SRSDropAdjudication{Arm: spec.SRSDropArmUnproven})); err != nil {
		t.Fatal(err)
	}
	if _, ok := state.srsOrdered[600006]; ok {
		t.Fatal("S2 must not mint an ordered entry")
	}
	if _, ok := state.srsExistenceStale[600006]; !ok {
		t.Fatal("S2 must mint existence stale")
	}
}

// The internal marker and the ordered channel never reach the public
// serialization: OrderedAbsent is json:"-" and no entry field is a
// spec type.
func TestT26SRSOrderedJSONShape(t *testing.T) {
	bound := &spec.SpatialReferenceSystemState{
		SRID: 100001, Read: true, Absence: true, OrderedAbsent: true,
		Dependencies: &spec.SRSDependencyFact{
			Columns: []spec.SRSDependentColumn{{Schema: "s", Table: "t", Column: "g", SRID: 100001}},
		},
	}
	raw, err := json.Marshal(bound)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"OrderedAbsent", "ordered", "absent_noop", "sourceArm", "publishedIdx", "ownPrivilege", "barrier"} {
		if strings.Contains(string(raw), needle) {
			t.Fatalf("public JSON leaks ordered provenance %q: %s", needle, raw)
		}
	}
}

package audit

// T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01 — internal-layer
// ordered-channel witnesses: the OrderedAbsentConsumed predicate, the
// mint/keep/barrier table over applySRSStatement (synthetic facts —
// interface-level controls, labeled internal per the work order), the
// S1-over-ordered-A adjudication order, and the JSON non-leak of the
// internal marker.

import (
	"context"
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

// --- OP-1i/OP-1j: resolver→bind→adjudication→apply on mismatched bound --
//
// opScriptedSRSProvider is the OP-1i/1j custom resolver: every SRS
// lookup returns the one scripted bound state while the privilege
// loader serves the negative-grant observation (unproven premise).
// This is a deliberately mismatched resolver seam — it makes no claim
// that mysqlmeta would ever produce the mismatch; it proves the real
// chain's handling of a bound state that fails ValidAbsence.
type opScriptedSRSProvider struct {
	bound     *spec.SpatialReferenceSystemState
	forwards  []spec.ObjectLookupRequest
	privCalls int
}

func (p *opScriptedSRSProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "8.0.36"}, nil
}

func (p *opScriptedSRSProvider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}

func (p *opScriptedSRSProvider) LoadMySQLServerDropPrivilege(context.Context, spec.Dialect) (*spec.MySQLServerDropPrivilegeObservation, error) {
	p.privCalls++
	return &spec.MySQLServerDropPrivilegeObservation{
		Account:    "app@%",
		GrantLines: []string{"GRANT SELECT ON *.* TO `app`@`%`"},
	}, nil
}

func (p *opScriptedSRSProvider) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.forwards = append(p.forwards, req)
	return &spec.ObjectSnapshot{
		Schema: req.Schema, Type: req.Type, Name: req.Name,
		SpatialReference: p.bound,
	}, nil
}

// OP-1i: authored 100001, resolver binds a FOREIGN-SRID absence —
// ValidAbsence's authored bind fails, S2 owes [E,D,P] under the
// unproven premise; own-apply mints only authored-100001 stale; the
// foreign SRID never earns an entry or a stale key.
func TestT26SRSOrderedForeignBoundNoPublication(t *testing.T) {
	provider := &opScriptedSRSProvider{bound: &spec.SpatialReferenceSystemState{
		SRID: 100002, Read: true, Exists: false, Absence: true,
	}}
	state := newSRSState(provider)
	enriched := enrichSRSPriv(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if len(enriched) != 1 {
		t.Fatalf("enriched=%d want 1", len(enriched))
	}
	adj := srsAdj(t, enriched[0])
	if adj.Arm != spec.SRSDropArmUnproven {
		t.Fatalf("arm=%v want S2 — a foreign-SRID absence is not this object's absence", adj.Arm)
	}
	if adj.Privilege != spec.SRSDropPrivilegeUnproven {
		t.Fatalf("premise=%v want unproven — the negative grant never discharges", adj.Privilege)
	}
	wantOwed := []string{spec.SRSFactExistence, spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}
	if len(adj.Owed) != len(wantOwed) {
		t.Fatalf("owed=%v want exactly %v", adj.Owed, wantOwed)
	}
	for i := range wantOwed {
		if adj.Owed[i] != wantOwed[i] {
			t.Fatalf("owed[%d]=%q want %q", i, adj.Owed[i], wantOwed[i])
		}
	}
	if len(provider.forwards) != 1 || provider.privCalls != 1 {
		t.Fatalf("resolver R=%d L=%d want 1/1", len(provider.forwards), provider.privCalls)
	}
	if len(state.srsOrdered) != 0 {
		t.Fatalf("S2 never publishes — ordered entries=%v", state.srsOrdered)
	}
	sameStaleSet(t, staleSet(state), map[uint32]bool{100001: true}, "own-apply mints authored stale only")
}

// OP-1j: authored 100001, resolver binds a same-SRID state that is
// simultaneously Exists AND Absence — the contradictory bound state
// fails ValidAbsence; S2 owes [D,P] under the unproven premise;
// own-apply mints authored-100001 stale only.
func TestT26SRSOrderedContradictoryBoundNoPublication(t *testing.T) {
	provider := &opScriptedSRSProvider{bound: &spec.SpatialReferenceSystemState{
		SRID: 100001, Read: true, Exists: true, Absence: true,
	}}
	state := newSRSState(provider)
	enriched := enrichSRSPriv(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if len(enriched) != 1 {
		t.Fatalf("enriched=%d want 1", len(enriched))
	}
	adj := srsAdj(t, enriched[0])
	if adj.Arm != spec.SRSDropArmUnproven {
		t.Fatalf("arm=%v want S2 — exists+absence never certifies an absence", adj.Arm)
	}
	if adj.Privilege != spec.SRSDropPrivilegeUnproven {
		t.Fatalf("premise=%v want unproven", adj.Privilege)
	}
	wantOwed := []string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}
	if len(adj.Owed) != len(wantOwed) {
		t.Fatalf("owed=%v want exactly %v", adj.Owed, wantOwed)
	}
	for i := range wantOwed {
		if adj.Owed[i] != wantOwed[i] {
			t.Fatalf("owed[%d]=%q want %q", i, adj.Owed[i], wantOwed[i])
		}
	}
	if len(provider.forwards) != 1 || provider.privCalls != 1 {
		t.Fatalf("resolver R=%d L=%d want 1/1", len(provider.forwards), provider.privCalls)
	}
	if len(state.srsOrdered) != 0 {
		t.Fatalf("S2 never publishes — ordered entries=%v", state.srsOrdered)
	}
	sameStaleSet(t, staleSet(state), map[uint32]bool{100001: true}, "own-apply mints authored stale only")
}

// --- ChannelControlsCapture fidelity controls (R3-E1) -------------------
//
// The capture is only useful if it is a TRUE deep-value snapshot.
// These controls pin the discriminations a shallow copy would hide:
// nil vs non-nil-empty, nested-element mutation, and bidirectional
// isolation — a live mutation must not reach the capture, and a
// capture mutation must not reach live state.
func TestT26SRSChannelControlsFidelity(t *testing.T) {
	newState := func() *batchState {
		state := &batchState{
			entries:            map[batchTableKey]*batchTableEntry{},
			tempEntries:        map[batchTableKey]*batchTableEntry{},
			serverEntries:      map[string]*batchServerEntry{},
			schemaEntries:      map[string]*batchSchemaEntry{},
			invalidatedSchemas: map[string]struct{}{},
			srsExistenceStale:  map[uint32]struct{}{},
		}
		return state
	}
	key := batchTableKey{dialect: spec.DialectMySQL, schema: "s", table: "t"}

	t.Run("nil-vs-empty", func(t *testing.T) {
		state := newState()
		// A nil-shape entry vs a non-nil-shape with nil Columns vs a
		// non-nil-shape with empty-non-nil Columns — three distinct
		// shapes the capture must preserve.
		state.entries[key] = &batchTableEntry{shape: nil}
		key2 := batchTableKey{dialect: spec.DialectMySQL, schema: "s", table: "u"}
		state.entries[key2] = &batchTableEntry{shape: &spec.TableSnapshot{Schema: "s", Exists: true}}
		key3 := batchTableKey{dialect: spec.DialectMySQL, schema: "s", table: "v"}
		state.entries[key3] = &batchTableEntry{shape: &spec.TableSnapshot{
			Schema: "s", Exists: true, Columns: []spec.Column{},
		}}
		snap := state.ChannelControlsCapture()
		if snap.Tables[key].shape != nil {
			t.Fatal("nil shape must stay nil")
		}
		if snap.Tables[key2].shape == nil || snap.Tables[key2].shape.Columns != nil {
			t.Fatal("non-nil shape with nil Columns must keep nil Columns")
		}
		if snap.Tables[key3].shape == nil || snap.Tables[key3].shape.Columns == nil ||
			len(snap.Tables[key3].shape.Columns) != 0 {
			t.Fatal("empty-non-nil Columns must stay non-nil empty")
		}
	})

	t.Run("nested-element-isolation", func(t *testing.T) {
		state := newState()
		state.entries[key] = &batchTableEntry{shape: &spec.TableSnapshot{
			Schema: "s", Exists: true,
			Columns: []spec.Column{{Name: "g", Type: "geometry"}},
			Options: map[string]string{"engine": "InnoDB"},
		}}
		before := state.ChannelControlsCapture()

		// Live mutation must NOT reach the capture — mutate a nested
		// element (the first column's type) and a map value.
		state.entries[key].shape.Columns[0].Type = "int"
		state.entries[key].shape.Options["engine"] = "MyISAM"
		if before.Tables[key].shape.Columns[0].Type != "geometry" {
			t.Fatal("live nested mutation leaked into the capture")
		}
		if before.Tables[key].shape.Options["engine"] != "InnoDB" {
			t.Fatal("live map mutation leaked into the capture")
		}

		// Capture mutation must NOT reach live state — same nested
		// element, opposite direction.
		before.Tables[key].shape.Columns[0].Type = "blob"
		if state.entries[key].shape.Columns[0].Type != "int" {
			t.Fatal("capture mutation leaked into live state")
		}
	})

	t.Run("invalidated-schemas-captured", func(t *testing.T) {
		state := newState()
		state.invalidatedSchemas["s"] = struct{}{}
		snap := state.ChannelControlsCapture()
		if _, ok := snap.Invalidated["s"]; !ok {
			t.Fatal("invalidatedSchemas must be captured — it is schema channel state")
		}
	})
}

// --- OP-5c frozen-fixture parse witness ---------------------------------
//
// The frozen OP-5c input text is exactly `GARBAGE@#$;` between the
// producer and the tail. The real parseSQL is the decisive source:
//
//	GARBAGE@#$: the failure record's RawSQL swallows the tail —
//	  stmts=1 (producer only), failures=1 with RawSQL
//	  "GARBAGE@#$;DROP SPATIAL REFERENCE SYSTEM 100001;" at
//	  line 1 column 48. The tail DROP never exists as an
//	  authored statement — a bounded counterexample; the frozen
//	  fixture's contamination-blocking tail is not exercisable.
//	GARBAGE;  : the parser recovers past the failure record —
//	  stmts=2 (tail is real), failures=1 at line 1 column 48.
//	  This is the contamination companion the realchain row uses.
//
// Captured verbatim from the real parser — never engineered to match
// an expectation.
func TestT26SRSOrderedGarbageParseWitness(t *testing.T) {
	ctx := context.Background()

	exact, err := parseSQL(ctx,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;GARBAGE@#$;DROP SPATIAL REFERENCE SYSTEM 100001;",
		spec.DialectMySQL)
	if err == nil {
		t.Fatal("GARBAGE@#$ must return a parse error")
	}
	if len(exact.Statements) != 1 {
		t.Fatalf("GARBAGE@#$ authored statements=%d want exactly 1 — the tail never exists", len(exact.Statements))
	}
	if len(exact.failures) != 1 {
		t.Fatalf("GARBAGE@#$ failures=%d want exactly 1", len(exact.failures))
	}
	f := exact.failures[0]
	if f.RawSQL != "GARBAGE@#$;DROP SPATIAL REFERENCE SYSTEM 100001;" {
		t.Fatalf("failure RawSQL=%q — the tail is swallowed into the failed chunk", f.RawSQL)
	}
	if f.Line != 1 || f.Column != 48 {
		t.Fatalf("failure coords=%d:%d want 1:48", f.Line, f.Column)
	}
	if f.Err == nil {
		t.Fatal("failure record must carry its parse error")
	}

	companion, err := parseSQL(ctx,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;GARBAGE;DROP SPATIAL REFERENCE SYSTEM 100001;",
		spec.DialectMySQL)
	if err == nil {
		t.Fatal("GARBAGE; must return a parse error")
	}
	if len(companion.Statements) != 2 {
		t.Fatalf("GARBAGE; authored statements=%d want exactly 2 — the tail survives recovery", len(companion.Statements))
	}
	if len(companion.failures) != 1 {
		t.Fatalf("GARBAGE; failures=%d want exactly 1", len(companion.failures))
	}
	f = companion.failures[0]
	if f.RawSQL != "GARBAGE;" {
		t.Fatalf("companion failure RawSQL=%q want exactly GARBAGE;", f.RawSQL)
	}
	if f.Line != 1 || f.Column != 48 {
		t.Fatalf("companion failure coords=%d:%d want 1:48", f.Line, f.Column)
	}
	if companion.Statements[1].RawSQL != "DROP SPATIAL REFERENCE SYSTEM 100001;" {
		t.Fatalf("tail statement=%q — the real tail exists only on the recovery path", companion.Statements[1].RawSQL)
	}
}

// --- OP-6c controlled TiDB parse companion -------------------------------
//
// The frozen TiDB row separates two facts the vendor boundary must
// not conflate: the SHARED parser still authors the statement under
// dialect=tidb (the vendor boundary is an AUDIT-layer decision, not
// a parse failure), and the discovered banner is what trips the
// boundary. This companion captures the controlled parse witness —
// the same SQL parsed under both dialects.
func TestT26SRSOrderedTiDBParseCompanion(t *testing.T) {
	ctx := context.Background()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		parsed, err := parseSQL(ctx, "DROP SPATIAL REFERENCE SYSTEM 100001;", dialect)
		if err != nil {
			t.Fatalf("%s parse: %v — the shared parser must author the statement", dialect, err)
		}
		if len(parsed.Statements) != 1 || len(parsed.failures) != 0 {
			t.Fatalf("%s: stmts=%d failures=%d want 1/0", dialect,
				len(parsed.Statements), len(parsed.failures))
		}
		if parsed.Statements[0].RawSQL != "DROP SPATIAL REFERENCE SYSTEM 100001;" {
			t.Fatalf("%s: RawSQL=%q — the authored statement is the same input", dialect, parsed.Statements[0].RawSQL)
		}
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
	// The marker is MISSING — not serialized as null. Distinguish the
	// two explicitly: decode to a map and check the keys; a present
	// key with a null value would still count as a leak.
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ordered_absent", "orderedAbsent", "OrderedAbsent"} {
		if v, present := decoded[key]; present {
			t.Fatalf("marker serialized as key %q=%v — must be absent, never null", key, v)
		}
	}
	// Whole-document scan for internal ordered provenance — the
	// dependency witness and its SRID echo are json:"-" too.
	for _, needle := range []string{"OrderedAbsent", "ordered", "absent_noop", "sourceArm", "publishedIdx", "ownPrivilege", "barrier"} {
		if strings.Contains(string(raw), needle) {
			t.Fatalf("public JSON leaks ordered provenance %q: %s", needle, raw)
		}
	}
}

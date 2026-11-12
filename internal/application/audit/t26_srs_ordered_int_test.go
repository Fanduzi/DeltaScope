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
	out.resolvedVersion = &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.3", Major: 8, Minor: 0, Patch: 3, Source: spec.VersionSourceObserved}
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

// --- ChannelControlsCapture fidelity controls (R4-B) -------------------
//
// The capture is only useful if it is a TRUE deep-value snapshot built
// WITHOUT product normalization helpers. These controls discriminate
// with real different inputs — a bare batchState whose channel maps
// are genuinely nil vs one whose maps are non-nil (empty or populated)
// — plus per-container nilness inside TableSnapshot, non-empty nested
// values, and bidirectional isolation across multiple member kinds.
func TestT26SRSChannelControlsFidelity(t *testing.T) {
	key := batchTableKey{dialect: spec.DialectMySQL, schema: "s", table: "t"}
	key2 := batchTableKey{dialect: spec.DialectMySQL, schema: "s", table: "u"}
	key3 := batchTableKey{dialect: spec.DialectMySQL, schema: "s", table: "v"}

	t.Run("channel-map-nilness", func(t *testing.T) {
		// Bare state: every channel map genuinely nil.
		bare := &batchState{}
		snap := bare.ChannelControlsCapture()
		if snap.Servers != nil || snap.Tables != nil || snap.TempTables != nil ||
			snap.Schemas != nil || snap.Invalidated != nil || snap.Stale != nil || snap.Ordered != nil {
			t.Fatalf("nil channel maps must stay nil: %+v", snap)
		}
		// Non-nil-but-empty maps must NOT collapse into nil.
		empty := &batchState{
			entries:            map[batchTableKey]*batchTableEntry{},
			tempEntries:        map[batchTableKey]*batchTableEntry{},
			serverEntries:      map[string]*batchServerEntry{},
			schemaEntries:      map[string]*batchSchemaEntry{},
			invalidatedSchemas: map[string]struct{}{},
			srsExistenceStale:  map[uint32]struct{}{},
			srsOrdered:         map[uint32]*srsOrderedEntry{},
		}
		snap2 := empty.ChannelControlsCapture()
		if snap2.Servers == nil || snap2.Tables == nil || snap2.TempTables == nil ||
			snap2.Schemas == nil || snap2.Invalidated == nil || snap2.Stale == nil || snap2.Ordered == nil {
			t.Fatalf("non-nil-empty channel maps must not collapse to nil: %+v", snap2)
		}
		if len(snap2.Tables) != 0 || len(snap2.Ordered) != 0 || len(snap2.Stale) != 0 {
			t.Fatal("empty source maps must produce empty captures")
		}
	})

	t.Run("member-container-nilness", func(t *testing.T) {
		state := &batchState{entries: map[batchTableKey]*batchTableEntry{}}
		// Three distinct observed shapes the capture must keep apart.
		state.entries[key] = &batchTableEntry{shape: nil}
		state.entries[key2] = &batchTableEntry{shape: &spec.TableSnapshot{Schema: "s", Exists: true}}
		state.entries[key3] = &batchTableEntry{shape: &spec.TableSnapshot{
			Schema: "s", Exists: true,
			Columns: []spec.Column{},
			Options: map[string]string{},
		}}
		snap := state.ChannelControlsCapture()
		if snap.Tables[key].shape != nil {
			t.Fatal("nil shape must stay nil")
		}
		if snap.Tables[key2].shape == nil || snap.Tables[key2].shape.Columns != nil ||
			snap.Tables[key2].shape.Options != nil {
			t.Fatal("non-nil shape with nil members must keep nil members")
		}
		s3 := snap.Tables[key3].shape
		if s3 == nil || s3.Columns == nil || len(s3.Columns) != 0 ||
			s3.Options == nil || len(s3.Options) != 0 {
			t.Fatal("empty-non-nil members must stay non-nil empty")
		}
	})

	t.Run("nested-values-and-isolation", func(t *testing.T) {
		state := &batchState{entries: map[batchTableKey]*batchTableEntry{}}
		card := int64(42)
		updatable := true
		state.entries[key] = &batchTableEntry{shape: &spec.TableSnapshot{
			Schema: "s", Exists: true,
			Columns: []spec.Column{
				{Name: "g", Type: "geometry", IdentityOptions: map[string]any{"seed": "1"}},
			},
			PrimaryKey: &spec.Index{Name: "PRIMARY", Cardinality: &card, Columns: []string{"g"}},
			Indexes: []spec.Index{
				{Name: "gi", Columns: []string{"g"}, KeyParts: []spec.IndexKeyPart{{Column: "g"}}},
			},
			Constraints:         []spec.Constraint{{Type: "check", Columns: []string{"g"}}},
			Options:             map[string]string{"engine": "InnoDB"},
			View:                &spec.ViewSnapshot{Updatable: &updatable},
			TTL:                 &spec.TTLConfig{TimeColumn: "ts"},
			PartitionPlacements: map[string]string{"p0": "x"},
		}}
		before := state.ChannelControlsCapture()

		// Non-empty nested values complete preserved.
		sh := before.Tables[key].shape
		if sh.Columns[0].IdentityOptions["seed"] != "1" ||
			sh.PrimaryKey.Cardinality == nil || *sh.PrimaryKey.Cardinality != 42 ||
			sh.Indexes[0].KeyParts[0].Column != "g" ||
			sh.Constraints[0].Type != "check" ||
			*sh.View.Updatable != true ||
			sh.TTL.TimeColumn != "ts" ||
			sh.PartitionPlacements["p0"] != "x" {
			t.Fatalf("non-empty nested values not preserved: %+v", sh)
		}

		// Live mutation across MULTIPLE member kinds must not reach the
		// capture: slice element, map value, pointer target, nested map.
		live := state.entries[key].shape
		live.Columns[0].Type = "int"
		live.Columns[0].IdentityOptions["seed"] = "2"
		live.Options["engine"] = "MyISAM"
		*live.PrimaryKey.Cardinality = 7
		live.Indexes[0].KeyParts[0].Column = "h"
		*live.View.Updatable = false
		live.PartitionPlacements["p0"] = "y"
		c := before.Tables[key].shape
		if c.Columns[0].Type != "geometry" || c.Columns[0].IdentityOptions["seed"] != "1" ||
			c.Options["engine"] != "InnoDB" || *c.PrimaryKey.Cardinality != 42 ||
			c.Indexes[0].KeyParts[0].Column != "g" || *c.View.Updatable != true ||
			c.PartitionPlacements["p0"] != "x" {
			t.Fatal("live mutation leaked into the capture")
		}

		// Capture mutation must not reach live state — same members,
		// opposite direction.
		c.Columns[0].Type = "blob"
		c.Options["engine"] = "CSV"
		*c.PrimaryKey.Cardinality = 9
		c.PartitionPlacements["p0"] = "z"
		if live.Columns[0].Type != "int" || live.Options["engine"] != "MyISAM" ||
			*live.PrimaryKey.Cardinality != 7 || live.PartitionPlacements["p0"] != "y" {
			t.Fatal("capture mutation leaked into live state")
		}
	})

	t.Run("required-channels-populated", func(t *testing.T) {
		state := &batchState{
			serverEntries: map[string]*batchServerEntry{
				"s1": {name: "s1", barrier: true},
			},
			schemaEntries: map[string]*batchSchemaEntry{
				"s": {state: schemaPresent, attrs: map[string]string{"k": "v"}},
			},
			invalidatedSchemas: map[string]struct{}{"s": {}},
			srsExistenceStale:  map[uint32]struct{}{100001: {}},
			srsOrdered:         map[uint32]*srsOrderedEntry{},
			srsDepsStale:       true,
			contaminated:       true,
		}
		snap := state.ChannelControlsCapture()
		if !snap.Servers["s1"].barrier || snap.Schemas["s"].attrs["k"] != "v" ||
			!snap.Stale[100001] || !snap.DepsStale || !snap.Contaminated {
			t.Fatal("required channels must carry their real values")
		}
		if _, ok := snap.Invalidated["s"]; !ok {
			t.Fatal("invalidatedSchemas must be captured")
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

	// Parse-shape contrast on the REAL shared parser: both dialects
	// author the identical row.
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

	// The ordered-channel dimension through the REAL Extract +
	// enrich + own-apply path on a TiDB batchState (a nil provider
	// discriminates: any attempted catalog read fails loudly). The
	// extractor itself marks the row Unsupported{boundary:vendor},
	// so even though orderedStateDialect(tidb) is true the ordered
	// mechanism is unreachable: no entry mints on either statement,
	// while the boundary own-apply still mints the authored stale
	// key — a TiDB batch can never publish or consume an ordered
	// entry, and it never sees a MySQL minted one either (dialect is
	// per-state).
	parsed, err := parseSQL(ctx, "DROP SPATIAL REFERENCE SYSTEM 100001;"+
		"DROP SPATIAL REFERENCE SYSTEM 100001;", spec.DialectTiDB)
	if err != nil {
		t.Fatalf("tidb parse: %v", err)
	}
	statements, err := Extract(ctx, parsed)
	if err != nil {
		t.Fatalf("tidb extract: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("extracted=%d want 2", len(statements))
	}
	for i, stmt := range statements {
		if stmt.Dialect != spec.DialectTiDB {
			t.Fatalf("stmt%d dialect=%v want tidb", i, stmt.Dialect)
		}
		if stmt.Unsupported == nil || stmt.Unsupported.Reason != spec.UnsupportedVendorBoundaryReason ||
			stmt.Unsupported.Feature != string(spec.DDLOperationDropSpatialReferenceSystem) {
			t.Fatalf("stmt%d unsupported=%+v want the extractor's vendor boundary", i, stmt.Unsupported)
		}
	}
	state := newBatchState(spec.DialectTiDB, "golden", nil)
	state.resolvedVersion = spec.ParseObservedVersion("8.0.11-TiDB-v8.5.0")
	state.instanceFacts = &spec.InstanceFacts{Version: "8.0.11-TiDB-v8.5.0"}
	enriched, err := state.enrichStatements(ctx,
		&MetadataRequest{Schema: "golden",
			PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession},
		statements, parsed.failures, state.instanceFacts,
		auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err != nil {
		t.Fatalf("tidb enrich: %v", err)
	}
	if len(enriched) != 2 {
		t.Fatalf("enriched=%d want 2", len(enriched))
	}
	for i, stmt := range enriched {
		if stmt.Metadata != nil && stmt.Metadata.SRSDrop != nil {
			t.Fatalf("stmt%d adjudication=%+v want none — boundary rows are never adjudicated", i, stmt.Metadata.SRSDrop)
		}
		if stmt.Metadata != nil && stmt.Metadata.SpatialReference != nil &&
			stmt.Metadata.SpatialReference.OrderedAbsent {
			t.Fatalf("stmt%d carries the ordered marker — a boundary row never commits it", i)
		}
	}
	// Ordered channel empty after BOTH statements; the boundary
	// own-apply mints exactly the authored stale key — never a
	// publication.
	if ids := state.SRSOrderedSRIDs(); len(ids) != 0 {
		t.Fatalf("tidb ordered keys=%v want empty — vendor boundary never publishes", ids)
	}
	sameStaleSet(t, staleSet(state), map[uint32]bool{100001: true}, "boundary minted authored stale only")
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

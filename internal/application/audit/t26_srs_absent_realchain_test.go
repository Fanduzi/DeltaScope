package audit_test

// T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01 R2-B — the internal positive
// rows (PA-5 / PA-6d / PA-7) delivered over the REAL product supply
// chain: r3ProductProvider delegates ResolveObject to
// mysqlmeta.NewProvider(db) while the batchState observation hooks and
// independent seam counters come through export_test.go. The mirror
// loader rows stay in t26_srs_absent_int_test.go as the bind/apply
// semantics layer; these rows own the product-supply-chain dimension.

import (
	"testing"

	audit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// paRealState builds a batchState over the real product provider —
// r3ProductProvider delegates ResolveObject to mysqlmeta.NewProvider,
// counts R forwards and L calls, and the r3Fixture supplies the
// scripted catalog answers (independent counters per seam).
func paRealState(t *testing.T, fx *r3Fixture) (*r3ProductProvider, *audit.TestBatchState) {
	t.Helper()
	provider := r3Harness(t, fx)
	return provider, audit.NewTestBatchState(provider)
}

// paSameStale mirrors sameStaleSet — exact set equality, never length.
func paSameStale(t *testing.T, got, want map[uint32]bool, who string) {
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

func paAdj(t *testing.T, stmt spec.Statement) *spec.SRSDropAdjudication {
	t.Helper()
	if stmt.Metadata == nil || stmt.Metadata.SRSDrop == nil {
		t.Fatalf("no SRS adjudication attached: %+v", stmt.Metadata)
	}
	return stmt.Metadata.SRSDrop
}

func paReadsVector(provider *r3ProductProvider, fx *r3Fixture) [5]int {
	var v [5]int
	for _, req := range provider.forwards {
		if req.Type == "mysql_spatial_reference_system" {
			v[0]++
		}
	}
	v[1] = fx.count["exist"]
	v[2] = fx.count["deps"]
	v[3] = fx.count["ver"]
	v[4] = provider.privCalls
	return v
}

// paBothStale asserts BOTH stale channels on an A arm (R2-B3): the
// seeded existence-stale set is unchanged and the dependency-stale
// flag persists — no A arm mints or clears either channel.
func paBothStale(t *testing.T, state *audit.TestBatchState, want map[uint32]bool) {
	t.Helper()
	paSameStale(t, audit.BatchStateStaleSet(state), want, "existence-stale")
	if !state.SRSDepsStale() {
		t.Fatal("depsStale must persist — no A arm clears state")
	}
}

// --- R2-B3 / PA-5: stale channels on the three A arms — real chain --

// PA-5a/5b/5c over the real provider: seeded foreign-SRID existence
// staleness plus dependency staleness demote the positive witness;
// ValidAbsence still governs the arm, and NO A arm mutates either
// stale channel — both are compared on every arm (R2-B3).
func TestT26SRSAbsentStaleChannelsRealChain(t *testing.T) {
	newState := func(t *testing.T, fx *r3Fixture) (*r3ProductProvider, *audit.TestBatchState) {
		provider, state := paRealState(t, fx)
		state.SeedSRSExistenceStale(77777)
		state.SetSRSDepsStale(true)
		return provider, state
	}
	fxAbsentDeps := func() *r3Fixture {
		return &r3Fixture{
			version: "8.0.36",
			deps:    []r3DepRow{{schema: "s", table: "t", column: "g", srid: 100001}},
		}
	}
	fxConsumable := func() *r3Fixture {
		fx := fxAbsentDeps()
		fx.account = "app@%"
		fx.grants = []string{"GRANT SELECT, SUPER ON *.* TO `app`@`%`"}
		return fx
	}

	t.Run("PA-5a-plain-refused", func(t *testing.T) {
		fx := fxConsumable()
		provider, state := newState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
		adj := paAdj(t, enriched[0])
		if adj.Arm != spec.SRSDropArmAbsentRefused {
			t.Fatalf("arm=%v want absent_refused — depsStale demotes the witness, absence governs", adj.Arm)
		}
		if adj.Privilege != spec.SRSDropPrivilegeConsumable || len(adj.Owed) != 0 {
			t.Fatalf("adj=%+v want consumable/no-owed", adj)
		}
		paBothStale(t, state, map[uint32]bool{77777: true})
		r3AssertReads(t, provider, fx, 1, 1, 1, 1, 1, -1)
	})

	t.Run("PA-5b-ifexists-noop", func(t *testing.T) {
		fx := fxConsumable()
		provider, state := newState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
		adj := paAdj(t, enriched[0])
		if adj.Arm != spec.SRSDropArmAbsentNoop {
			t.Fatalf("arm=%v want absent_noop", adj.Arm)
		}
		if adj.Privilege != spec.SRSDropPrivilegeConsumable || len(adj.Owed) != 0 {
			t.Fatalf("adj=%+v want consumable/no-owed", adj)
		}
		paBothStale(t, state, map[uint32]bool{77777: true})
		// R3-C1: this row's own read vector — one resolver call, one
		// existence read, one deps read, one Qo read, one privilege
		// load; Vd is N/A at the internal enrich layer (no discovery).
		r3AssertReads(t, provider, fx, 1, 1, 1, 1, 1, -1)
	})

	t.Run("PA-5c-ifexists-unproven", func(t *testing.T) {
		fx := fxAbsentDeps()
		fx.account = "app@%"
		fx.grants = []string{"GRANT SELECT ON *.* TO `app`@`%`"}
		provider, state := newState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
		adj := paAdj(t, enriched[0])
		if adj.Arm != spec.SRSDropArmAbsentConditional {
			t.Fatalf("arm=%v want absent_conditional", adj.Arm)
		}
		if adj.Privilege != spec.SRSDropPrivilegeUnproven ||
			len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactDropPrivilege {
			t.Fatalf("adj=%+v want unproven/exactly [srs.drop.privilege]", adj)
		}
		paBothStale(t, state, map[uint32]bool{77777: true})
		r3AssertReads(t, provider, fx, 1, 1, 1, 1, 1, -1)
	})

	// PA-5d: the same-SRID stale flag blocks at the bind — zero reads
	// issue and no mint/clear happens.
	t.Run("PA-5d-same-srid-blocked", func(t *testing.T) {
		fx := fxConsumable()
		provider, state := newState(t, fx)
		state.SeedSRSExistenceStale(100001)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
		adj := paAdj(t, enriched[0])
		if adj.Arm != spec.SRSDropArmBlocked {
			t.Fatalf("arm=%v want blocked", adj.Arm)
		}
		paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{77777: true, 100001: true}, "post-apply")
		if !state.SRSDepsStale() {
			t.Fatal("depsStale must persist")
		}
		r3AssertReads(t, provider, fx, 0, 0, 0, 0, 1, -1)
	})

	// PA-5e: a positive existence answer under foreign-SRID stale +
	// depsStale lands S2 — own-apply mints the authored SRID alongside
	// the seeded foreign key.
	t.Run("PA-5e-exists-unproven-mints", func(t *testing.T) {
		fx := fxConsumable()
		fx.srs = map[uint32]string{100001: "SRID 100001"}
		provider, state := newState(t, fx)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
		adj := paAdj(t, enriched[0])
		if adj.Arm != spec.SRSDropArmUnproven {
			t.Fatalf("arm=%v want S2 — depsStale demotes the witness, existence is proven present", adj.Arm)
		}
		if len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactColumnDependencies {
			t.Fatalf("owed=%v want [srs.column_dependencies]", adj.Owed)
		}
		paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{77777: true, 100001: true}, "post-apply")
		if !state.SRSDepsStale() {
			t.Fatal("depsStale must persist")
		}
		// R3-C1: the minted row's own vector — a positive existence
		// answer still costs the full read pass and one privilege load.
		r3AssertReads(t, provider, fx, 1, 1, 1, 1, 1, -1)
	})

	// PA-5f: whole-batch contamination (GARBAGE parse failure) blocks
	// the trailing qualified DROP — never reaches the resolver.
	t.Run("PA-5f-contamination-blocked", func(t *testing.T) {
		fx := fxConsumable()
		provider, state := paRealState(t, fx)
		state.SetSRSDepsStale(true)
		enriched := audit.EnrichSRSPrivForTest(t, state, provider,
			"GARBAGE; DROP SPATIAL REFERENCE SYSTEM 100001;")
		if !state.Contaminated() {
			t.Fatal("GARBAGE prefix must contaminate the batch")
		}
		adj := paAdj(t, enriched[0])
		if adj.Arm != spec.SRSDropArmBlocked {
			t.Fatalf("arm=%v want blocked", adj.Arm)
		}
		if len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactBatchEffects {
			t.Fatalf("owed=%v want [srs.batch_effects]", adj.Owed)
		}
		paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{}, "post-apply")
		if !state.SRSDepsStale() {
			t.Fatal("depsStale must persist")
		}
		r3AssertReads(t, provider, fx, 0, 0, 0, 0, 1, -1)
	})
}

// --- R2-B / PA-6d: per-enrich cache over the real chain --------------

// PA-6d over the real provider: two enrichStatements calls sharing one
// batchState never share the object cache — call2 re-reads and lands
// S2 while call1's proven absence minted nothing.
func TestT26SRSAbsentRecachesPerEnrichRealChain(t *testing.T) {
	fx := &r3Fixture{
		version: "8.0.36",
		account: "app@%",
		grants:  []string{"GRANT SELECT, SUPER ON *.* TO `app`@`%`"},
	}
	provider, state := paRealState(t, fx)

	before := paReadsVector(provider, fx)
	enriched1 := audit.EnrichSRSPrivForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if arm := paAdj(t, enriched1[0]).Arm; arm != spec.SRSDropArmAbsentRefused {
		t.Fatalf("call1 arm=%v want absent_refused", arm)
	}
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{}, "after call1")
	mid := paReadsVector(provider, fx)
	for i, want := range [5]int{1, 1, 1, 1, 1} {
		if got := mid[i] - before[i]; got != want {
			t.Fatalf("call1 delta[%d]=%d want %d (R,Qexist,Qdeps,Qo,L)", i, got, want)
		}
	}

	fx.srs = map[uint32]string{100001: "SRID 100001"}
	enriched2 := audit.EnrichSRSPrivForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	adj := paAdj(t, enriched2[0])
	if adj.Arm != spec.SRSDropArmUnproven {
		t.Fatalf("call2 arm=%v want S2 — the per-enrich cache re-read the now-present row", adj.Arm)
	}
	if len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactColumnDependencies {
		t.Fatalf("call2 owed=%v want [srs.column_dependencies]", adj.Owed)
	}
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{100001: true}, "after call2")
	after := paReadsVector(provider, fx)
	for i, want := range [5]int{1, 1, 1, 1, 1} {
		if got := after[i] - mid[i]; got != want {
			t.Fatalf("call2 delta[%d]=%d want %d (R,Qexist,Qdeps,Qo,L)", i, got, want)
		}
	}
	if provider.privCalls != 2 {
		t.Fatalf("L=%d want 2 — the loader re-evaluates at each enrich entry", provider.privCalls)
	}
}

// --- R2-B4 / PA-7: three-timepoint snapshot of the same stmt1 --------

// paStmtSnap is a deep, independent copy of one statement's full SRS
// projection at one internal hook point — complete SpatialReference
// plus the SRSDrop adjudication (arm, premise, owed). No mutable
// descendant is shared with the live statement.
type paStmtSnap struct {
	srs  *spec.SpatialReferenceSystemState
	adj  *spec.SRSDropAdjudication
	srsN bool
	adjN bool
}

// R3-B: fidelity clones — a nil slice stays nil and a non-nil empty
// slice stays non-nil empty. append([]T(nil), x...) erases that
// distinction, so each collection is rebuilt nil-preserving.
func paCloneSRIDs(s []uint32) []uint32 {
	if s == nil {
		return nil
	}
	return append([]uint32{}, s...)
}

func paCloneDepCols(s []spec.SRSDependentColumn) []spec.SRSDependentColumn {
	if s == nil {
		return nil
	}
	return append([]spec.SRSDependentColumn{}, s...)
}

func paCloneStrings(s []string) []string {
	if s == nil {
		return nil
	}
	return append([]string{}, s...)
}

func paCloneConflict(f *spec.SRSConflictFact) *spec.SRSConflictFact {
	if f == nil {
		return nil
	}
	c := *f
	c.MatchedSRIDs = paCloneSRIDs(f.MatchedSRIDs)
	return &c
}

func paSnapStmt(stmt spec.Statement) paStmtSnap {
	var out paStmtSnap
	if stmt.Metadata == nil {
		return out
	}
	out.srsN = stmt.Metadata.SpatialReference == nil
	out.adjN = stmt.Metadata.SRSDrop == nil
	if s := stmt.Metadata.SpatialReference; s != nil {
		c := *s
		c.NameConflict = paCloneConflict(s.NameConflict)
		c.OrgConflict = paCloneConflict(s.OrgConflict)
		if s.Dependencies != nil {
			d := *s.Dependencies
			d.Columns = paCloneDepCols(s.Dependencies.Columns)
			c.Dependencies = &d
		}
		out.srs = &c
	}
	if a := stmt.Metadata.SRSDrop; a != nil {
		c := *a
		c.Owed = paCloneStrings(a.Owed)
		out.adj = &c
	}
	return out
}

// paSRSConflictSame is the pure comparator — false on ANY difference:
// pointer nilness, Complete, MatchedSRIDs nilness, length, or elements.
// paSRSConflictEqual wraps it with fatal detail.
func paSRSConflictSame(a, b *spec.SRSConflictFact) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if a.Complete != b.Complete || len(a.MatchedSRIDs) != len(b.MatchedSRIDs) ||
		(a.MatchedSRIDs == nil) != (b.MatchedSRIDs == nil) {
		return false
	}
	for i := range a.MatchedSRIDs {
		if a.MatchedSRIDs[i] != b.MatchedSRIDs[i] {
			return false
		}
	}
	return true
}

func paSRSConflictEqual(t *testing.T, who string, a, b *spec.SRSConflictFact) {
	t.Helper()
	if !paSRSConflictSame(a, b) {
		t.Fatalf("%s conflict %+v vs %+v", who, a, b)
	}
}

// paSnapEqual compares two independent stmt1 captures field-by-field —
// complete SpatialReference, SRSDrop arm/premise/owed, and the
// nil/empty distinction on each collection.
func paSnapEqual(t *testing.T, who string, a, b paStmtSnap) {
	t.Helper()
	if a.srsN != b.srsN || a.adjN != b.adjN {
		t.Fatalf("%s nilness differs: %+v vs %+v", who, a, b)
	}
	if !a.srsN {
		x, y := a.srs, b.srs
		if x.SRID != y.SRID || x.Status != y.Status || x.Exists != y.Exists ||
			x.Absence != y.Absence || x.Read != y.Read || x.UnreadReason != y.UnreadReason ||
			x.CatalogName != y.CatalogName {
			t.Fatalf("%s SpatialReference differs: %+v vs %+v", who, x, y)
		}
		paSRSConflictEqual(t, who+" name", x.NameConflict, y.NameConflict)
		paSRSConflictEqual(t, who+" org", x.OrgConflict, y.OrgConflict)
		if (x.Dependencies == nil) != (y.Dependencies == nil) {
			t.Fatalf("%s deps nilness differs: %v vs %v", who, x.Dependencies, y.Dependencies)
		}
		if x.Dependencies != nil {
			if len(x.Dependencies.Columns) != len(y.Dependencies.Columns) ||
				(x.Dependencies.Columns == nil) != (y.Dependencies.Columns == nil) {
				t.Fatalf("%s deps differ: %+v vs %+v", who, x.Dependencies, y.Dependencies)
			}
			for i := range x.Dependencies.Columns {
				if x.Dependencies.Columns[i] != y.Dependencies.Columns[i] {
					t.Fatalf("%s deps[%d] %+v vs %+v", who, i, x.Dependencies.Columns[i], y.Dependencies.Columns[i])
				}
			}
		}
	}
	if !a.adjN {
		x, y := a.adj, b.adj
		if x.Arm != y.Arm || x.Privilege != y.Privilege ||
			len(x.Owed) != len(y.Owed) || (x.Owed == nil) != (y.Owed == nil) {
			t.Fatalf("%s SRSDrop differs: %+v vs %+v", who, x, y)
		}
		for i := range x.Owed {
			if x.Owed[i] != y.Owed[i] {
				t.Fatalf("%s owed[%d]=%q vs %q", who, i, x.Owed[i], y.Owed[i])
			}
		}
	}
}

// R3-B: the snapshot helpers' fidelity — real nil vs real non-nil
// empty are distinguishable captures, element changes are caught, and
// the isolation is bidirectional (mutating the live value never
// retro-changes a captured snapshot; mutating a snapshot never bleeds
// back into the live value). Covers the three collections PA-7
// actually copies: MatchedSRIDs, Dependencies.Columns, Owed.
func TestT26SRSAbsentSnapshotFidelity(t *testing.T) {
	nilStmt := spec.Statement{Metadata: &spec.Metadata{
		SpatialReference: &spec.SpatialReferenceSystemState{
			SRID: 100001,
			NameConflict: &spec.SRSConflictFact{
				Complete:     true,
				MatchedSRIDs: nil, // real nil
			},
			Dependencies: &spec.SRSDependencyFact{Columns: nil},
		},
		SRSDrop: &spec.SRSDropAdjudication{Owed: nil},
	}}
	emptyStmt := spec.Statement{Metadata: &spec.Metadata{
		SpatialReference: &spec.SpatialReferenceSystemState{
			SRID: 100001,
			NameConflict: &spec.SRSConflictFact{
				Complete:     true,
				MatchedSRIDs: []uint32{}, // real non-nil empty
			},
			Dependencies: &spec.SRSDependencyFact{Columns: []spec.SRSDependentColumn{}},
		},
		SRSDrop: &spec.SRSDropAdjudication{Owed: []string{}},
	}}
	nilSnap := paSnapStmt(nilStmt)
	emptySnap := paSnapStmt(emptyStmt)

	// 1. nil vs non-nil empty remain distinguishable captures.
	if nilSnap.srs.NameConflict.MatchedSRIDs != nil {
		t.Fatal("nil MatchedSRIDs captured as non-nil")
	}
	if emptySnap.srs.NameConflict.MatchedSRIDs == nil || len(emptySnap.srs.NameConflict.MatchedSRIDs) != 0 {
		t.Fatal("non-nil empty MatchedSRIDs captured as nil")
	}
	if nilSnap.srs.Dependencies.Columns != nil || emptySnap.srs.Dependencies.Columns == nil {
		t.Fatal("Dependencies.Columns nilness not preserved")
	}
	if nilSnap.adj.Owed != nil || emptySnap.adj.Owed == nil {
		t.Fatal("Owed nilness not preserved")
	}

	// 2. A same-field mismatch between captures is caught by the
	// comparator — nil vs empty must compare different, not pass.
	if paSRSConflictSame(nilSnap.srs.NameConflict, emptySnap.srs.NameConflict) {
		t.Fatal("comparator must distinguish nil from non-nil empty MatchedSRIDs")
	}

	// 3. Exact-value preservation: the capture must carry the original
	// values, not merely a non-nil slice of the same shape (R4-B).
	live := spec.Statement{Metadata: &spec.Metadata{
		SpatialReference: &spec.SpatialReferenceSystemState{
			SRID:         100001,
			NameConflict: &spec.SRSConflictFact{Complete: true, MatchedSRIDs: []uint32{1}},
			Dependencies: &spec.SRSDependencyFact{Columns: []spec.SRSDependentColumn{{Schema: "s", Table: "t", Column: "g", SRID: 9}}},
		},
		SRSDrop: &spec.SRSDropAdjudication{Owed: []string{"a"}},
	}}
	snap := paSnapStmt(live)
	wantCol := spec.SRSDependentColumn{Schema: "s", Table: "t", Column: "g", SRID: 9}
	if len(snap.srs.NameConflict.MatchedSRIDs) != 1 || snap.srs.NameConflict.MatchedSRIDs[0] != 1 {
		t.Fatalf("MatchedSRIDs=%v want exactly [1]", snap.srs.NameConflict.MatchedSRIDs)
	}
	if len(snap.srs.Dependencies.Columns) != 1 || snap.srs.Dependencies.Columns[0] != wantCol {
		t.Fatalf("Dependencies.Columns=%+v want exactly [%+v]", snap.srs.Dependencies.Columns, wantCol)
	}
	if len(snap.adj.Owed) != 1 || snap.adj.Owed[0] != "a" {
		t.Fatalf("Owed=%v want exactly [a]", snap.adj.Owed)
	}
	// mutate the live value — the snapshot must keep the ORIGINAL values
	live.Metadata.SpatialReference.NameConflict.MatchedSRIDs[0] = 999
	live.Metadata.SpatialReference.Dependencies.Columns[0].SRID = 999
	live.Metadata.SRSDrop.Owed[0] = "zzz"
	if snap.srs.NameConflict.MatchedSRIDs[0] != 1 ||
		snap.srs.Dependencies.Columns[0] != wantCol ||
		snap.adj.Owed[0] != "a" {
		t.Fatal("live mutation leaked into the captured snapshot")
	}
	// mutate the snapshot — the live value must keep the WRITTEN values
	snap.srs.NameConflict.MatchedSRIDs[0] = 777
	snap.srs.Dependencies.Columns[0].SRID = 777
	snap.adj.Owed[0] = "yyy"
	if live.Metadata.SpatialReference.NameConflict.MatchedSRIDs[0] != 999 ||
		live.Metadata.SpatialReference.Dependencies.Columns[0].SRID != 999 ||
		live.Metadata.SRSDrop.Owed[0] != "zzz" {
		t.Fatal("snapshot mutation leaked back into the live value")
	}

	// 4. Comparator controls (R4-B): a positive control proves two
	// identical non-empty conflicts compare equal; a negative control
	// proves same Complete/same nilness/same length but a differing
	// ELEMENT is caught — a shape-preserving clone that zeroed values
	// cannot slip through.
	posA := &spec.SRSConflictFact{Complete: true, MatchedSRIDs: []uint32{1, 2}}
	posB := &spec.SRSConflictFact{Complete: true, MatchedSRIDs: []uint32{1, 2}}
	if !paSRSConflictSame(posA, posB) {
		t.Fatal("comparator must return true for identical non-empty MatchedSRIDs")
	}
	neg := &spec.SRSConflictFact{Complete: true, MatchedSRIDs: []uint32{1, 42}}
	if paSRSConflictSame(posA, neg) {
		t.Fatal("comparator must catch a differing element at same shape/nilness/length")
	}
}

// PA-7a over the real provider: stmt1's COMPLETE projection (full
// SpatialReference + SRSDrop + privilege premise) is independently
// deep-copied at three real hook points — ① after stmt1's own apply,
// ② after the REVOKE apply, ③ after stmt3's apply / final — and all
// three captures must be identical. The stmt3 arm is
// absent_conditional under the suppressed premise; no arm mints stale
// state (R2-B4).
func TestT26SRSAbsentRevokeScopingRealChain(t *testing.T) {
	fx := &r3Fixture{
		version: "8.0.36",
		account: "app@%",
		grants:  []string{"GRANT SELECT, SUPER ON *.* TO `app`@`%`"},
	}
	provider, state := paRealState(t, fx)

	var stmt1 *spec.Statement
	var snaps [3]paStmtSnap
	var snap3 paStmtSnap
	var scopeEnded [3]bool
	var staleAt [3]map[uint32]bool
	var depsStaleAt [3]bool
	// R3-C2: per-statement read vectors captured at each apply hook —
	// baseline before enrich, then after each statement's own apply.
	base := paReadsVector(provider, fx)
	var readsAt [3][5]int
	state.SetAfterApplyObserve(func(i int, stmt *spec.Statement) {
		scopeEnded[i] = state.ScopeEnded()
		staleAt[i] = audit.BatchStateStaleSet(state)
		depsStaleAt[i] = state.SRSDepsStale()
		readsAt[i] = paReadsVector(provider, fx)
		if i == 0 {
			stmt1 = stmt
		}
		// Each hook point deep-copies the SAME stmt1 — ① its own apply,
		// ② after the REVOKE apply, ③ after stmt3's apply.
		snaps[i] = paSnapStmt(*stmt1)
		if i == 2 {
			snap3 = paSnapStmt(*stmt)
		}
	})
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
			"REVOKE SELECT ON *.* FROM 'app'@'%';"+
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100002;")
	if len(enriched) != 3 {
		t.Fatalf("enriched=%d want 3", len(enriched))
	}
	// The three stmt1 captures are identical across the scope end.
	paSnapEqual(t, "stmt1 after-own-apply vs after-REVOKE", snaps[0], snaps[1])
	paSnapEqual(t, "stmt1 after-own-apply vs after-stmt3/final", snaps[0], snaps[2])
	paSnapEqual(t, "stmt1 after-own-apply vs post-batch", snaps[0], paSnapStmt(enriched[0]))
	// stmt1's own arm/premise: absent_noop under the consumable premise.
	adj1 := paAdj(t, enriched[0])
	if adj1.Arm != spec.SRSDropArmAbsentNoop || adj1.Privilege != spec.SRSDropPrivilegeConsumable || len(adj1.Owed) != 0 {
		t.Fatalf("stmt1 adj=%+v want {absent_noop consumable no-owed}", adj1)
	}
	// stmt3: absent_conditional under the suppressed premise.
	adj3 := paAdj(t, enriched[2])
	if snap3.adj == nil || snap3.adj.Arm != spec.SRSDropArmAbsentConditional ||
		snap3.adj.Privilege != spec.SRSDropPrivilegeSuppressed ||
		len(snap3.adj.Owed) != 1 || snap3.adj.Owed[0] != spec.SRSFactDropPrivilege {
		t.Fatalf("stmt3 snap=%+v want {absent_conditional suppressed [srs.drop.privilege]}", snap3.adj)
	}
	if adj3.Arm != spec.SRSDropArmAbsentConditional {
		t.Fatalf("stmt3 arm=%v want absent_conditional", adj3.Arm)
	}
	for i := 0; i < 3; i++ {
		if scopeEnded[i] != (i > 0) {
			t.Fatalf("scopeEnded[%d]=%v want %v", i, scopeEnded[i], i > 0)
		}
		paSameStale(t, staleAt[i], map[uint32]bool{}, "stale")
		if depsStaleAt[i] {
			t.Fatalf("depsStale[%d] must stay false — no A arm mints or clears", i)
		}
	}
	// R3-C2: per-statement increments measured at each apply point —
	// not just the final total. stmt1's increment is the full single
	// pass (R/Qexist/Qdeps/Qo +1 each) PLUS the one-time privilege
	// observation L=1 — the loader runs once with the first premise
	// derivation and is an enrich-level observation, never a per-
	// statement read. The REVOKE apply adds ZERO reads. stmt3's
	// increment is its own pass and no further load. Vd is N/A at
	// the internal enrich layer — no discovery runs there.
	incr := func(a, b [5]int) [5]int {
		var d [5]int
		for i := range d {
			d[i] = a[i] - b[i]
		}
		return d
	}
	if d := incr(readsAt[0], base); d != [5]int{1, 1, 1, 1, 1} {
		t.Fatalf("stmt1 increment=%v want [1 1 1 1 1] — its own pass + the one-time premise load", d)
	}
	if d := incr(readsAt[1], readsAt[0]); d != [5]int{0, 0, 0, 0, 0} {
		t.Fatalf("REVOKE increment=%v want [0 0 0 0 0] — a scope event never reads the catalog", d)
	}
	if d := incr(readsAt[2], readsAt[1]); d != [5]int{1, 1, 1, 1, 0} {
		t.Fatalf("stmt3 increment=%v want [1 1 1 1 0] — its own pass under the closed scope, no second load", d)
	}
	if readsAt[2][4]-base[4] != 1 {
		t.Fatalf("privilege loads=%d want exactly 1 across the whole enrich", readsAt[2][4]-base[4])
	}
	r3AssertReads(t, provider, fx, 2, 2, 2, 2, 1, -1)
}

// --- R3-C3: internal companions for PA-6c / PA-6e -------------------

// PA-6c internal companion — same fixture as the public row
// (zero-row SRS cell, Qo 8.0.36, consumable SUPER observation):
// the CREATE OR REPLACE prefix carries the implementation boundary
// with NO SRSDrop adjudication; its own-apply mints the authored
// SRID stale set {100001} without contaminating the batch; the tail
// DROP is audited B short-circuited by the same-SRID stale flag.
// Prefix reads and the tail's ZERO increment are measured at the
// respective apply hooks — never inferred from totals (R3-C3).
func TestT26SRSAbsentBoundaryPrefixInternal(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	provider, state := paRealState(t, fx)

	base := paReadsVector(provider, fx)
	var readsAt [2][5]int
	var staleAt [2]map[uint32]bool
	var contaminatedAt [2]bool
	state.SetAfterApplyObserve(func(i int, stmt *spec.Statement) {
		readsAt[i] = paReadsVector(provider, fx)
		staleAt[i] = audit.BatchStateStaleSet(state)
		contaminatedAt[i] = state.Contaminated()
	})
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 DEFINITION 'x';"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if len(enriched) != 2 {
		t.Fatalf("enriched=%d want 2", len(enriched))
	}

	// Prefix: implementation boundary, no SRSDrop adjudication.
	prefix := enriched[0]
	if prefix.Unsupported == nil ||
		prefix.Unsupported.Metadata["boundary"] != "implementation" ||
		prefix.Unsupported.Metadata["object"] != "spatial_reference_system" ||
		prefix.Unsupported.Reason != spec.UnsupportedUnauditedReason {
		t.Fatalf("prefix unsupported=%+v want implementation/spatial_reference_system boundary", prefix.Unsupported)
	}
	if prefix.Metadata != nil && prefix.Metadata.SRSDrop != nil {
		t.Fatalf("prefix carries an SRSDrop adjudication — boundary rows never adjudicate: %+v", prefix.Metadata.SRSDrop)
	}
	// Prefix own-apply: authored stale set exactly {100001}; no
	// contamination (the boundary lifecycle write is the ONLY effect).
	paSameStale(t, staleAt[0], map[uint32]bool{100001: true}, "prefix apply")
	if contaminatedAt[0] {
		t.Fatal("prefix apply must not contaminate the batch — tracked facts provably stand")
	}
	// Prefix reads: the pre-state existence pass it issued before its
	// boundary apply — measured, not inferred.
	if d := [5]int{readsAt[0][0] - base[0], readsAt[0][1] - base[1],
		readsAt[0][2] - base[2], readsAt[0][3] - base[3], readsAt[0][4] - base[4]}; d != [5]int{1, 1, 1, 1, 1} {
		t.Fatalf("prefix increment=%v want [1 1 1 1 1] — its single pass + the one-time load", d)
	}

	// Tail: audited B — the same-SRID stale minted by the prefix
	// short-circuits its bind; zero further reads.
	tail := enriched[1]
	if tail.Metadata == nil || tail.Metadata.SRSDrop == nil {
		t.Fatalf("tail missing its audited adjudication: %+v", tail.Metadata)
	}
	if tail.Metadata.SRSDrop.Arm != spec.SRSDropArmBlocked ||
		len(tail.Metadata.SRSDrop.Owed) != 1 || tail.Metadata.SRSDrop.Owed[0] != spec.SRSFactBatchEffects {
		t.Fatalf("tail adj=%+v want blocked/[srs.batch_effects]", tail.Metadata.SRSDrop)
	}
	if d := [5]int{readsAt[1][0] - readsAt[0][0], readsAt[1][1] - readsAt[0][1],
		readsAt[1][2] - readsAt[0][2], readsAt[1][3] - readsAt[0][3], readsAt[1][4] - readsAt[0][4]}; d != [5]int{0, 0, 0, 0, 0} {
		t.Fatalf("tail increment=%v want [0 0 0 0 0] — same-SRID stale short-circuits the bind", d)
	}
	paSameStale(t, staleAt[1], map[uint32]bool{100001: true}, "tail apply")
	if contaminatedAt[1] {
		t.Fatal("tail apply must not contaminate the batch")
	}
}

// PA-6e internal companion — same fixture difference as the public row
// (Qo 8.0.3 ⇒ resolved version OUTSIDE the audited domain): BOTH
// statements are version boundaries, NEITHER carries an SRSDrop
// adjudication; stmt1's boundary apply mints the authored stale key;
// stmt1's single pass and stmt2's ZERO increment are measured at their
// own hooks. L=0 is an AFTER-APPLY measured value at each hook — the
// reason the loader never fired is the enrich-entry qualification gate
// plus this fixture, which these counts do not themselves time-order
// (R4-E1: the hook observes after apply, not before the mint).
// Vd is N/A at this layer: no discovery runs inside enrich.
func TestT26SRSAbsentBoundaryVersionPairInternal(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	fx.version = "8.0.3"
	provider, state := paRealState(t, fx)
	state.SeedResolvedVersion(&spec.VersionIdentity{
		Product: spec.VersionProductMySQL, Version: "8.0.3",
		Major: 8, Minor: 0, Patch: 3, ValidatedRange: true,
	})

	base := paReadsVector(provider, fx)
	var readsAt [2][5]int
	var staleAt [2]map[uint32]bool
	state.SetAfterApplyObserve(func(i int, stmt *spec.Statement) {
		readsAt[i] = paReadsVector(provider, fx)
		staleAt[i] = audit.BatchStateStaleSet(state)
	})
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;")
	if len(enriched) != 2 {
		t.Fatalf("enriched=%d want 2", len(enriched))
	}
	for i, stmt := range enriched {
		if stmt.Unsupported == nil ||
			stmt.Unsupported.Metadata["boundary"] != "unaudited" ||
			stmt.Unsupported.Metadata["gate"] != "version" ||
			stmt.Unsupported.Metadata["object"] != "spatial_reference_system" {
			t.Fatalf("stmt%d unsupported=%+v want version/spatial_reference_system boundary", i, stmt.Unsupported)
		}
		if stmt.Metadata != nil && stmt.Metadata.SRSDrop != nil {
			t.Fatalf("stmt%d carries an SRSDrop adjudication — boundary rows never adjudicate", i)
		}
	}
	// stmt1's boundary apply mints the authored key — its own apply
	// is the mint, not an adjudication.
	paSameStale(t, staleAt[0], map[uint32]bool{100001: true}, "stmt1 apply")
	// stmt1's single pass measured at its hook; stmt2's zero increment
	// measured at its own hook (same-SRID stale short-circuit).
	if d := [5]int{readsAt[0][0] - base[0], readsAt[0][1] - base[1],
		readsAt[0][2] - base[2], readsAt[0][3] - base[3], readsAt[0][4] - base[4]}; d != [5]int{1, 1, 1, 1, 0} {
		t.Fatalf("stmt1 increment=%v want [1 1 1 1 0] — the boundary still reads, L never runs", d)
	}
	if d := [5]int{readsAt[1][0] - readsAt[0][0], readsAt[1][1] - readsAt[0][1],
		readsAt[1][2] - readsAt[0][2], readsAt[1][3] - readsAt[0][3], readsAt[1][4] - readsAt[0][4]}; d != [5]int{0, 0, 0, 0, 0} {
		t.Fatalf("stmt2 increment=%v want [0 0 0 0 0] — minted stale short-circuits its bind", d)
	}
	// L=0 measured at both after-apply hooks. The gate story — the
	// loader is never triggered because no audited statement reaches
	// the premise-derivation point on a version boundary — comes from
	// the enrich-entry qualification gate and this fixture; these
	// after-apply counts do not themselves time-order the mint
	// (R4-E1: after-apply measurement, not a pre-mint observation).
	if readsAt[0][4] != 0 || readsAt[1][4] != 0 {
		t.Fatalf("L=%d/%d want 0/0 — no audited statement ever derived a premise", readsAt[0][4], readsAt[1][4])
	}
}

// T26-SERVER-ALTER-EXECUTION-01 R2: distinguishing witnesses for the
// accepted AM matrix — the uniform A1 no-op on all three shadow
// sources, real two-call contamination, metadata lifecycle snapshots
// at the own-apply seam, frozen lifecycle and boundary chains, and
// the frozen public projection. Every snapshot is an independent
// deep copy taken before the transition, never a re-read of shared
// pointers afterwards.
package audit

import (
	"context"
	"reflect"
	"strings"
	"testing"

	apppolicy "github.com/Fanduzi/DeltaScope/internal/application/policy"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// serverChannelSnapshot is an independent deep copy of the COMPLETE
// ordered SERVER channel — every entry's own field values (reusing
// the accepted serverEntrySnapshot field copy), the write history,
// and the channel-level flags — captured before the transition under
// test. Missing entries stay distinct from existing empty entries:
// the map's nil-ness and each entry's absence are preserved exactly.
// A mutation anywhere — a rewritten field, a minted or cleared
// entry, an appended write, a flipped flag — is detected by
// reflect.DeepEqual against the stored copy, never by comparing
// keys, lengths, or pointer addresses.
type serverChannelSnapshot struct {
	entriesNil   bool
	entries      map[string]serverEntrySnapshot
	writes       []serverWrite
	taint        bool
	contaminated bool
}

func snapshotServerChannel(s *batchState) serverChannelSnapshot {
	snap := serverChannelSnapshot{
		entriesNil:   s.serverEntries == nil,
		writes:       append([]serverWrite(nil), s.serverWrites...),
		taint:        s.serverChannelTainted,
		contaminated: s.contaminated,
	}
	if s.serverEntries != nil {
		snap.entries = snapshotServerEntries(s)
	}
	return snap
}

// enrichT26AlterEnrichSlice runs the real enrich loop on one batchState
// for a statement slice — the call boundary is the apply seam.
func enrichSlice(t *testing.T, state *batchState, provider *t26Provider, mode spec.PrivilegeAuditMode, stmts []spec.Statement, failures []parseFailure) []spec.Statement {
	t.Helper()
	enriched, err := state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider, PrivilegeAuditMode: mode},
		stmts, failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err != nil {
		t.Fatalf("enrich slice: %v", err)
	}
	return enriched
}

// evaluateEnriched runs the real rule registry over already-enriched
// statements — the same public projection evaluateT26Statements
// produces, usable on an arbitrary enriched slice (e.g. one enrich
// call's output from a two-call contamination fixture).
func evaluateEnriched(t *testing.T, stmts []spec.Statement) report.Result {
	t.Helper()
	cfg, err := apppolicy.Load("")
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	registry, err := buildRegistry(cfg)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	result, err := EvaluateStatements(context.Background(), registry, stmts)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	return result
}

func newAlterState(provider *t26Provider) *batchState {
	state := newBatchState(spec.DialectMySQL, "golden", provider)
	state.resolvedVersion = &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.36", Major: 8, Minor: 0, Patch: 36, ValidatedRange: true}
	state.instanceFacts = &spec.InstanceFacts{Version: "8.0.36"}
	return state
}

func parseT26(t *testing.T, sql string) ([]spec.Statement, []parseFailure) {
	t.Helper()
	parsed, _ := parseSQL(context.Background(), sql, spec.DialectMySQL)
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract %q: %v", sql, err)
	}
	return statements, parsed.failures
}

// AM-7.6 (R2): ALTER sé → ALTER se → DROP sé — the ordered present is
// the final consumer, never the barriered name. sé catalog present +
// observed consumable: stmt1 republishes flag=false; stmt2 is the
// uniform A1 — exact {server.batch_effects}, no publish, no marker,
// and a COMPLETE channel no-op (no se entry, no serverWrite, no
// barrier, no rewrite of sé's standing present); stmt3's DROP consumes
// the standing presence through the proven path. One catalog read
// total: L=1/R=1/S=1 (S asserted at the pkg seam — the R layer is the
// provider-facing resolver call count recorded here).
func TestT26AlterAM76OrderedPresentFinalConsumer(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"sé": true}, version: "8.0.36", privilege: superObservation()}
	statements, failures := parseT26(t, "ALTER SERVER `sé` OPTIONS (USER 'u'); ALTER SERVER se OPTIONS (USER 'v'); DROP SERVER `sé`")
	state := newAlterState(provider)
	// One enrich call carries the whole batch — the loader fires once
	// per call, so slicing here would double-count L against the
	// contract's per-request semantics. stmt2's own apply is inspected
	// through the complete channel snapshot below.
	enriched := enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements, failures)
	if adj := enriched[0].Metadata.ServerAlter; adj == nil || adj.Arm != spec.ServerAlterArmProvenAlter ||
		!adj.PublishPresent || adj.ConditionalSource {
		t.Fatalf("stmt1 sé alter = %+v, want proven alter republishing flag=false", adj)
	}
	adj2 := enriched[1].Metadata.ServerAlter
	if adj2 == nil || adj2.Arm != spec.ServerAlterArmBarrier ||
		len(adj2.Owed) != 1 || adj2.Owed[0] != spec.ServerFactBatchEffects {
		t.Fatalf("stmt2 se alter = %+v, want A1 owing exactly {server.batch_effects}", adj2)
	}
	if adj2.PublishPresent || adj2.ConditionalSource || adj2.ConditionalAudit {
		t.Fatalf("A1 carries no publish and no conditional provenance: %+v", adj2)
	}
	// stmt2's own apply was a complete no-op: no se entry, no write,
	// no barrier, and the sé entry stands verbatim through stmt3's
	// drop consume.
	if entry := state.serverEntries[serverFold("se")]; entry != nil {
		t.Fatalf("A1 minted an se entry: %+v", entry)
	}
	drop := enriched[2].Metadata.ServerDrop
	if drop == nil || drop.Arm != spec.ServerDropArmProven {
		t.Fatalf("drop sé = %+v, want proven on the standing present", drop)
	}
	seEntry := state.serverEntries[serverFold("sé")]
	if seEntry == nil || !seEntry.absent {
		t.Fatalf("sé entry = %+v, want the drop's own absent publish on the standing present", seEntry)
	}
	if len(state.serverWrites) != 0 || state.serverChannelTainted {
		t.Fatalf("A1 recorded a write or taint: writes=%v taint=%v", state.serverWrites, state.serverChannelTainted)
	}
	if len(provider.calls) != 1 || provider.privilegeCalls != 1 {
		t.Fatalf("reads = resolver:%+v loader:%d, want exactly 1/1 — channel answered every later statement", provider.calls, provider.privilegeCalls)
	}
}

// AM-7.3a (R2): ALTER sé → plain DROP se, sé present / se zero,
// observed consumable. The relation barrier binds on the DROP: owed
// exactly {server.batch_effects}, marker from the DROP's own still-
// consumable premise (no ordered source was consumed), coverage
// unverified, no OrderedAbsent published; DROP's own apply mints the
// se barrier entry while the sé presence stands. L=1/R=1/S=1.
func TestT26AlterAM73aDropOwnBarrier(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"sé": true}, version: "8.0.36", privilege: superObservation()}
	result, enriched := evaluateT26Statements(t,
		"ALTER SERVER `sé` OPTIONS (USER 'u'); DROP SERVER se",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	dropAdj := enriched[1].Metadata.ServerDrop
	if dropAdj == nil || dropAdj.Arm != spec.ServerDropArmBarrier ||
		len(dropAdj.Owed) != 1 || dropAdj.Owed[0] != spec.ServerFactBatchEffects {
		t.Fatalf("drop se = %+v, want barrier owing exactly {server.batch_effects}", dropAdj)
	}
	if dropAdj.ConditionalSource {
		t.Fatalf("drop marker must come from its own consumable premise, never an ordered source: %+v", dropAdj)
	}
	dropStmt := result.Statements[1]
	if _, ok := dropStmt.Findings[0].Metadata["conditional_audit"]; !ok {
		t.Fatalf("drop marker missing on the public finding: %+v", dropStmt.Findings[0].Metadata)
	}
	if dropStmt.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("drop coverage = %v, want unverified", dropStmt.Coverage.Status)
	}
	if len(provider.calls) != 1 || provider.privilegeCalls != 1 {
		t.Fatalf("reads = resolver:%+v loader:%d, want 1/1", provider.calls, provider.privilegeCalls)
	}
}

// AM-7.3b (R2): both names zero on the catalog. ALTER sé keeps
// existence unknown — no publish, no entry, no barrier, no write —
// then a plain DROP se takes its own fresh catalog read, owes exactly
// {server.existence} (its own consumable premise discharged the
// privilege premise), marks via its own premise, and lands coverage
// unverified with the DROP's own barrier entry. L=1/R=2/S=2.
func TestT26AlterAM73bOwnReadExistence(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{}, version: "8.0.36", privilege: superObservation()}
	result, enriched := evaluateT26Statements(t,
		"ALTER SERVER `sé` OPTIONS (USER 'u'); DROP SERVER se",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	alterAdj := enriched[0].Metadata.ServerAlter
	if alterAdj == nil || alterAdj.Arm != spec.ServerAlterArmUnproven || alterAdj.PublishPresent {
		t.Fatalf("zero sé alter = %+v, want unproven without publish", alterAdj)
	}
	if len(alterAdj.Owed) != 1 || alterAdj.Owed[0] != spec.ServerFactExistence {
		t.Fatalf("zero sé owed = %v, want exactly {server.existence}", alterAdj.Owed)
	}
	dropAdj := enriched[1].Metadata.ServerDrop
	if dropAdj == nil || len(dropAdj.Owed) != 1 || dropAdj.Owed[0] != spec.ServerFactExistence {
		t.Fatalf("drop se = %+v, want owed exactly {server.existence} — own fresh read, not the relation barrier", dropAdj)
	}
	if _, ok := result.Statements[1].Findings[0].Metadata["conditional_audit"]; !ok {
		t.Fatalf("drop marker must come from its own consumable premise: %+v", dropAdj)
	}
	if result.Statements[1].Coverage.Status != report.CoverageUnverified {
		t.Fatalf("drop coverage = %v, want unverified", result.Statements[1].Coverage.Status)
	}
	if len(provider.calls) != 2 || provider.privilegeCalls != 1 {
		t.Fatalf("reads = resolver:%+v loader:%d, want 2/1 — sé and se are distinct catalog identities", provider.calls, provider.privilegeCalls)
	}
}

// AM-7.3 apply boundary: verify at the batchState seam that the se
// barrier entry the DROP minted is the DROP's own — and the ALTER's
// apply minted nothing on either variant.
func TestT26AlterAM73ApplyOwnership(t *testing.T) {
	for _, tc := range []struct {
		name    string
		servers map[string]bool
	}{
		{"sé present", map[string]bool{"sé": true}},
		{"sé zero", map[string]bool{}},
	} {
		provider := &t26Provider{servers: tc.servers, version: "8.0.36", privilege: superObservation()}
		statements, failures := parseT26(t, "ALTER SERVER `sé` OPTIONS (USER 'u'); DROP SERVER se")
		state := newAlterState(provider)
		enriched := enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements[:1], failures)
		if adj := enriched[0].Metadata.ServerAlter; adj == nil {
			t.Fatalf("%s: alter adjudication missing", tc.name)
		}
		afterAlter := snapshotServerChannel(state)
		if entry := state.serverEntries[serverFold("se")]; entry != nil {
			t.Fatalf("%s: ALTER minted an se entry: %+v", tc.name, entry)
		}
		enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements[1:], nil)
		// The se barrier entry is the DROP's own apply effect.
		entry := state.serverEntries[serverFold("se")]
		if entry == nil || !entry.barrier || entry.present || entry.absent || entry.publishedArm != 0 {
			t.Fatalf("%s: se entry = %+v, want the DROP's own barrier", tc.name, entry)
		}
		if tc.servers["sé"] {
			if se := state.serverEntries[serverFold("sé")]; se == nil || !se.present {
				t.Fatalf("%s: sé presence must stand after the drop's own barrier", tc.name)
			}
		} else if _, had := afterAlter.entries[serverFold("se")]; had {
			t.Fatalf("%s: unexpected se entry existed before the drop", tc.name)
		}
	}
}

// AM-8.1 (R2): real whole-batch contamination across two enrich calls
// on one batchState. call1 (mode observed): ALTER s1 + the real
// GARBAGE failure record — the A3 republish lands flag=false and the
// record marks the batch contaminated inside the loop. call2 (mode
// off): the tail ALTER binds unvouched → A1 exact {batch_effects}, no
// marker, no publish; its apply leaves the COMPLETE channel byte-
// identical. Counts: call1 L=1/R=1/S=1; call2 ΔL=0 (mode off — the
// loader gate is per-request, never contamination-suppressed),
// ΔR=0/ΔS=0 (contaminated bind never reaches the resolver); total 1/1/1.
func TestT26AlterAM81TwoCallContamination(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	parsed, _ := parseSQL(context.Background(),
		"ALTER SERVER s1 OPTIONS (USER 'u'); GARBAGE(; ALTER SERVER s1 OPTIONS (HOST 'h')", spec.DialectMySQL)
	if len(parsed.failures) != 1 {
		t.Fatalf("parse: failures=%d, want exactly 1", len(parsed.failures))
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("statements = %d, want 2 (failure is a record, not a slot)", len(statements))
	}
	state := newAlterState(provider)
	if state.contaminated {
		t.Fatalf("precondition broken: contaminated before call1")
	}
	// call1: mode observed; the slice carries the failure record, so
	// the loop's own failure handling marks the batch — never test-set.
	call1 := enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements[:1], parsed.failures)
	if !state.contaminated {
		t.Fatalf("contaminated = false after the real failure record")
	}
	adj1 := call1[0].Metadata.ServerAlter
	if adj1 == nil || adj1.Arm != spec.ServerAlterArmProvenAlter || !adj1.PublishPresent || adj1.ConditionalSource {
		t.Fatalf("call1 alter = %+v, want A3 publishing flag=false", adj1)
	}
	bound1 := call1[0].Metadata.Server
	if bound1 == nil || !bound1.Read || !bound1.Exists {
		t.Fatalf("call1 bound = %+v, want catalog read present", bound1)
	}
	// call1's public projection: A3 → conditional_alter notice with
	// the own-consumable conditional marker.
	public1 := evaluateEnriched(t, call1)
	f1 := public1.Statements[0].Findings[0]
	if f1.Metadata["outcome"] != "conditional_alter" {
		t.Fatalf("call1 public = %+v, want conditional_alter", f1.Metadata)
	}
	if _, ok := f1.Metadata["conditional_audit"]; !ok {
		t.Fatalf("call1 marker = %+v, want conditional_audit from the own consumable premise", f1.Metadata)
	}
	// R3-C AM-8.1(a): independent copies of call1's two metadata
	// objects, compared across call2's own bind+apply.
	adj1Snap := snapshotAlterAdj(adj1)
	bound1Snap := *bound1
	snap := snapshotServerChannel(state)
	resolver0, loader0 := len(provider.calls), provider.privilegeCalls
	// call2: mode off — the per-request loader gate never fired for
	// this slice; ΔR=0 comes from the real contamination flag.
	call2 := enrichSlice(t, state, provider, "", statements[1:], nil)
	adj2 := call2[0].Metadata.ServerAlter
	if adj2 == nil || adj2.Arm != spec.ServerAlterArmBarrier ||
		len(adj2.Owed) != 1 || adj2.Owed[0] != spec.ServerFactBatchEffects {
		t.Fatalf("call2 alter = %+v, want A1 exact {server.batch_effects}", adj2)
	}
	if adj2.PublishPresent || adj2.ConditionalAudit || adj2.ConditionalSource {
		t.Fatalf("contaminated alter carries no publish/provenance: %+v", adj2)
	}
	if bound2 := call2[0].Metadata.Server; bound2 == nil || bound2.Read || bound2.OrderedPresent {
		t.Fatalf("call2 bound = %+v, want unvouched — no answer consumed", bound2)
	}
	if after := snapshotServerChannel(state); !reflect.DeepEqual(after, snap) {
		t.Fatalf("A1 apply mutated the channel\nbefore=%+v\nafter=%+v", snap, after)
	}
	// call2 never rewrote call1's bound/adjudicated metadata.
	if !sameAlterAdj(adj1Snap, call1[0].Metadata.ServerAlter) {
		t.Fatalf("call1 adjudication rewritten across call2")
	}
	if got := *call1[0].Metadata.Server; got != bound1Snap {
		t.Fatalf("call1 bound metadata rewritten across call2\nbefore=%+v\nafter=%+v", bound1Snap, got)
	}
	// call2's public projection: A1 → blocked_batch_effects, owed
	// exactly {server.batch_effects}, no conditional marker.
	public2 := evaluateEnriched(t, call2)
	f2 := public2.Statements[0].Findings[0]
	if f2.Metadata["outcome"] != "blocked_batch_effects" {
		t.Fatalf("call2 public = %+v, want blocked_batch_effects", f2.Metadata)
	}
	if _, ok := f2.Metadata["conditional_audit"]; ok {
		t.Fatalf("call2 marker = %+v, want absent — A1 carries no conditional audit", f2.Metadata)
	}
	if facts := public2.Statements[0].EvidenceGaps[0].RequiredFacts; len(facts) != 1 || facts[0] != spec.ServerFactBatchEffects {
		t.Fatalf("call2 owed = %v, want exactly {server.batch_effects}", facts)
	}
	if d := len(provider.calls) - resolver0; d != 0 {
		t.Fatalf("call2 resolver delta = %d, want 0 — contaminated bind never enters the resolver", d)
	}
	if d := provider.privilegeCalls - loader0; d != 0 {
		t.Fatalf("call2 loader delta = %d, want 0 — mode off, never contamination-suppressed", d)
	}
	if len(provider.calls) != 1 || provider.privilegeCalls != 1 {
		t.Fatalf("totals = R:%d L:%d, want 1/1", len(provider.calls), provider.privilegeCalls)
	}
}

// AM-8.2 (R3-C): authored order — failure record #1 ("GARBAGE(")
// precedes ALTER statement #2. The failure is a record, never a
// statement slot: statements has exactly 1 entry, the authored-second
// ALTER. Under mode observed the loader fires once at enrich entry
// (L=1) before the loop's own failure handling marks the batch, so
// the ALTER lands A1: exact {server.batch_effects}, no marker,
// unvouched bind, R=0/S=0.
func TestT26AlterAM82LoaderBeforeFailure(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	parsed, _ := parseSQL(context.Background(),
		"GARBAGE(; ALTER SERVER s1 OPTIONS (USER 'u')", spec.DialectMySQL)
	if len(parsed.failures) != 1 {
		t.Fatalf("parse: failures=%d, want exactly 1 (record #1)", len(parsed.failures))
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(statements) != 1 || statements[0].DDL == nil || statements[0].DDL.Server == nil ||
		statements[0].DDL.Server.Kind != spec.ServerKindAlter {
		t.Fatalf("statements = %+v, want exactly stmt#2 — the authored-second ALTER", statements)
	}
	state := newAlterState(provider)
	enriched := enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements, parsed.failures)
	adj := enriched[0].Metadata.ServerAlter
	if adj == nil || adj.Arm != spec.ServerAlterArmBarrier ||
		len(adj.Owed) != 1 || adj.Owed[0] != spec.ServerFactBatchEffects {
		t.Fatalf("alter after leading failure = %+v, want A1 exact {server.batch_effects}", adj)
	}
	if adj.PublishPresent || adj.ConditionalAudit || adj.ConditionalSource {
		t.Fatalf("A1 carries no publish/provenance: %+v", adj)
	}
	if provider.privilegeCalls != 1 {
		t.Fatalf("loader = %d, want 1 — fired at enrich entry before the failure marked the batch", provider.privilegeCalls)
	}
	if len(provider.calls) != 0 {
		t.Fatalf("resolver calls = %+v, want 0 — contaminated bind never reaches the seam", provider.calls)
	}
}

// alterAdjSnapshot is an independent deep copy of one
// ServerAlterAdjudication — the Owed slice is copied, never shared, so
// a later in-place rewrite anywhere is detected.
type alterAdjSnapshot struct {
	arm              spec.ServerAlterArm
	privilege        spec.ServerAlterPrivilege
	owed             []string
	publishPresent   bool
	conditionalSrc   bool
	conditionalAudit bool
}

func snapshotAlterAdj(adj *spec.ServerAlterAdjudication) *alterAdjSnapshot {
	if adj == nil {
		return nil
	}
	return &alterAdjSnapshot{
		arm: adj.Arm, privilege: adj.Privilege,
		owed:             append([]string(nil), adj.Owed...),
		publishPresent:   adj.PublishPresent,
		conditionalSrc:   adj.ConditionalSource,
		conditionalAudit: adj.ConditionalAudit,
	}
}

func sameAlterAdj(a *alterAdjSnapshot, b *spec.ServerAlterAdjudication) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return equalAlterAdj(a, snapshotAlterAdj(b))
}

func equalAlterAdj(a, b *alterAdjSnapshot) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.arm != b.arm || a.privilege != b.privilege || a.publishPresent != b.publishPresent ||
		a.conditionalSrc != b.conditionalSrc || a.conditionalAudit != b.conditionalAudit {
		return false
	}
	if len(a.owed) != len(b.owed) {
		return false
	}
	for i := range a.owed {
		if a.owed[i] != b.owed[i] {
			return false
		}
	}
	return true
}

// AM-6.6 + AM-10.2 (R3-B): metadata lifecycle at the three REAL
// internal timepoints — the afterBindObserve hook fires between the
// statement's bind/adjudication attachment and its apply (point a),
// afterApplyObserve fires immediately after that statement's own
// apply (point b), and the later statement's afterApplyObserve marks
// point c. Copies are taken inside the loop at the real seams on one
// enrich call — never a post-hoc re-read of shared pointers, and the
// single-call fixture keeps the loader at the request's own L=1.
// The dirty-options catalog-present ALTER owes exactly
// {server.alter_options}, republishes flag=false presence while its
// own consumable premise marks the audit; the later CREATE refuses
// on the standing unconditional present; the ALTER's two metadata
// objects are never rewritten across either transition.
func TestT26AlterAM66MetadataLifecycle(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	statements, failures := parseT26(t,
		"ALTER SERVER s1 OPTIONS (USER 'u', USER 'v'); CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')")
	state := newAlterState(provider)
	var tpA, tpB *alterAdjSnapshot
	var srvA, srvB *spec.ServerState
	laterApplied := false
	state.afterBindObserve = func(i int, stmt *spec.Statement) {
		if i == 0 && stmt.Metadata != nil {
			if stmt.Metadata.Server != nil {
				v := *stmt.Metadata.Server
				srvA = &v
			}
			tpA = snapshotAlterAdj(stmt.Metadata.ServerAlter)
		}
	}
	state.afterApplyObserve = func(i int, stmt *spec.Statement) {
		if i == 0 && stmt.Metadata != nil {
			if stmt.Metadata.Server != nil {
				v := *stmt.Metadata.Server
				srvB = &v
			}
			tpB = snapshotAlterAdj(stmt.Metadata.ServerAlter)
		}
		if i == 1 {
			laterApplied = true
		}
	}
	enriched := enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements, failures)
	first := enriched[0]
	if !laterApplied || tpA == nil || tpB == nil || srvA == nil || srvB == nil {
		t.Fatalf("hooks did not fire at the real seams: tpA=%v tpB=%v later=%v", tpA != nil, tpB != nil, laterApplied)
	}
	// Point a == point b == point c: the adjudication and the bound
	// server state survive own-apply and the later CREATE's apply.
	if !sameAlterAdj(tpA, first.Metadata.ServerAlter) || !sameAlterAdj(tpB, first.Metadata.ServerAlter) {
		t.Fatalf("Metadata.ServerAlter changed across the lifecycle\n@bind=%+v\n@apply=%+v\n@later=%+v", tpA, tpB, first.Metadata.ServerAlter)
	}
	if *first.Metadata.Server != *srvA || *first.Metadata.Server != *srvB {
		t.Fatalf("Metadata.Server changed across the lifecycle\n@bind=%+v\n@apply=%+v\n@later=%+v", srvA, srvB, first.Metadata.Server)
	}
	// The adjudication is immutable at bind time — the apply seam
	// never rewrote it between point a and point b either.
	if !equalAlterAdj(tpA, tpB) || tpA == nil {
		t.Fatalf("bind→own-apply rewrote the adjudication: %+v vs %+v", tpA, tpB)
	}
	if tpA.arm != spec.ServerAlterArmUnproven || len(tpA.owed) != 1 || tpA.owed[0] != spec.ServerFactAlterOptions {
		t.Fatalf("alter adj = %+v, want A4 owed exactly {server.alter_options}", tpA)
	}
	if !tpA.publishPresent || tpA.conditionalSrc || !tpA.conditionalAudit {
		t.Fatalf("alter adj = %+v, want publish + flag=false + own-premise marker", tpA)
	}
	entry := state.serverEntries[serverFold("s1")]
	if entry == nil || !entry.present || entry.presentConditional {
		t.Fatalf("entry = %+v, want republished flag=false presence", entry)
	}
	// The standing present refuses the CREATE — refused_present, no
	// inherited marker.
	createAdj := enriched[1].Metadata.ServerCreate
	if createAdj == nil || createAdj.Arm != spec.ServerCreateArmTerminalRefusal || createAdj.ConditionalAudit {
		t.Fatalf("create = %+v, want refused_present without marker", createAdj)
	}
	// Single-request counts: one enrich call, one loader invocation —
	// the frozen L=1 shape. R=1: the refused create asked the channel.
	if len(provider.calls) != 1 || provider.privilegeCalls != 1 {
		t.Fatalf("reads = R:%d L:%d, want 1/1 single-request", len(provider.calls), provider.privilegeCalls)
	}
	// Aggregate public shape on the same authored fixture: refused
	// create blocker → verdict reject, unverified coverage.
	result, pub := evaluateT26Statements(t,
		"ALTER SERVER s1 OPTIONS (USER 'u', USER 'v'); CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageUnverified ||
		result.Statements[0].Coverage.Status != report.CoverageUnverified {
		t.Fatalf("public = verdict %v coverage %v/%v, want reject + unverified",
			result.Verdict, result.Coverage.Status, result.Statements[0].Coverage.Status)
	}
	_ = pub
}

// AM-10.2 (R3-B): ALTER → DROP — the drop consumes the republished
// presence through the proven path, and the ALTER's two metadata
// objects (Metadata.Server + Metadata.ServerAlter) are not rewritten
// by the later apply — asserted on independent copies taken at the
// three REAL lifecycle seams (post-bind/pre-apply, post-own-apply,
// post-later-apply) via the observe hooks on one enrich call.
func TestT26AlterAM102DropConsumesMetadataStable(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	statements, failures := parseT26(t, t26AlterSQL+"; DROP SERVER s1")
	state := newAlterState(provider)
	var tpA, tpB *alterAdjSnapshot
	var srvA, srvB *spec.ServerState
	laterApplied := false
	state.afterBindObserve = func(i int, stmt *spec.Statement) {
		if i == 0 && stmt.Metadata != nil {
			if stmt.Metadata.Server != nil {
				v := *stmt.Metadata.Server
				srvA = &v
			}
			tpA = snapshotAlterAdj(stmt.Metadata.ServerAlter)
		}
	}
	state.afterApplyObserve = func(i int, stmt *spec.Statement) {
		if i == 0 && stmt.Metadata != nil {
			if stmt.Metadata.Server != nil {
				v := *stmt.Metadata.Server
				srvB = &v
			}
			tpB = snapshotAlterAdj(stmt.Metadata.ServerAlter)
		}
		if i == 1 {
			laterApplied = true
		}
	}
	enriched := enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements, failures)
	first := enriched[0]
	if !laterApplied || tpA == nil || tpB == nil || srvA == nil || srvB == nil {
		t.Fatalf("hooks did not fire at the real seams")
	}
	if !sameAlterAdj(tpA, first.Metadata.ServerAlter) || !sameAlterAdj(tpB, first.Metadata.ServerAlter) {
		t.Fatalf("Metadata.ServerAlter rewritten across the lifecycle")
	}
	if *first.Metadata.Server != *srvA || *first.Metadata.Server != *srvB {
		t.Fatalf("Metadata.Server rewritten across the lifecycle\n@bind=%+v\n@apply=%+v\n@later=%+v", srvA, srvB, first.Metadata.Server)
	}
	if !equalAlterAdj(tpA, tpB) {
		t.Fatalf("bind→own-apply rewrote the adjudication")
	}
	dropAdj := enriched[1].Metadata.ServerDrop
	if dropAdj == nil || dropAdj.Arm != spec.ServerDropArmProven {
		t.Fatalf("drop = %+v, want proven on the alter's republished presence", dropAdj)
	}
	if entry := state.serverEntries[serverFold("s1")]; entry == nil || !entry.absent {
		t.Fatalf("entry = %+v, want the drop's own absent publish", entry)
	}
	// Single-request counts: one call → L=1, R=1 (the ALTER's own
	// read; the DROP consumed the ordered answer without a new read).
	if len(provider.calls) != 1 || provider.privilegeCalls != 1 {
		t.Fatalf("reads = R:%d L:%d, want 1/1 single-request", len(provider.calls), provider.privilegeCalls)
	}
}

// AM-2.4i/2.5i (R2): the scope terminator suppresses the ALTER's OWN
// premise (Suppressed — never mislabeled Unproven), while the
// conditional ordered source keeps its verdict:
//
//	i)  conditional OrderedAbsent → A2 refused_absent, marker inherited.
//	ii) conditional OrderedPresent → A4 owed {alter_privilege},
//	    republish flag=true, marker inherited.
func TestT26AlterAM24iAM25iSuppressedChains(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	// i) DROP publishes conditional absent → terminator → ALTER refuses.
	result, enriched := evaluateT26Statements(t,
		"DROP SERVER s1; REVOKE SELECT ON *.* FROM 'r1'@'%'; "+t26AlterSQL,
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	adj := enriched[2].Metadata.ServerAlter
	if adj == nil || adj.Privilege != spec.ServerAlterPrivilegeSuppressed {
		t.Fatalf("own premise = %+v, want suppressed — never unproven", adj)
	}
	if adj.Arm != spec.ServerAlterArmTerminalRefusal || len(adj.Owed) != 0 || !adj.ConditionalSource || !adj.ConditionalAudit {
		t.Fatalf("refusal = %+v, want A2 with inherited-only marker", adj)
	}
	if result.Statements[2].Findings[0].Metadata["outcome"] != "refused_absent" ||
		result.Statements[2].Findings[0].Level != rule.LevelBlocker {
		t.Fatalf("public = %+v, want refused_absent blocker", result.Statements[2].Findings[0].Metadata)
	}
	if result.Statements[2].Coverage.Status != report.CoverageComplete {
		t.Fatalf("refusal coverage = %v, want complete", result.Statements[2].Coverage.Status)
	}
	// ii) DROP→CREATE publishes conditional present → terminator →
	// ALTER owes alter_privilege, republishes flag=true, marker
	// inherited from the consumed source.
	provider = &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, enriched = evaluateT26Statements(t,
		"DROP SERVER s1; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u'); REVOKE SELECT ON *.* FROM 'r1'@'%'; "+t26AlterSQL,
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	adj = enriched[3].Metadata.ServerAlter
	if adj == nil || adj.Privilege != spec.ServerAlterPrivilegeSuppressed {
		t.Fatalf("own premise = %+v, want suppressed", adj)
	}
	if adj.Arm != spec.ServerAlterArmUnproven || len(adj.Owed) != 1 || adj.Owed[0] != spec.ServerFactAlterPrivilege {
		t.Fatalf("alter = %+v, want A4 owed exactly {server.alter_privilege}", adj)
	}
	if !adj.PublishPresent || !adj.ConditionalSource || !adj.ConditionalAudit {
		t.Fatalf("republish = %+v, want flag=true + inherited marker", adj)
	}
	if result.Statements[3].Findings[0].Metadata["outcome"] != "unverified" ||
		result.Statements[3].Coverage.Status != report.CoverageUnverified {
		t.Fatalf("public = %+v coverage %v, want unverified", result.Statements[3].Findings[0].Metadata, result.Statements[3].Coverage.Status)
	}
	if len(provider.calls) != 1 || provider.privilegeCalls != 1 {
		t.Fatalf("reads = R:%d L:%d, want 1/1", len(provider.calls), provider.privilegeCalls)
	}
	// R3-B.4: the A4 republish's actual stored flag is observed at the
	// ALTER's own post-apply seam on the same authored fixture — the
	// stored entry reads present + presentConditional=true.
	provider3 := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	statements, _ := parseT26(t,
		"DROP SERVER s1; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u'); REVOKE SELECT ON *.* FROM 'r1'@'%'; "+t26AlterSQL)
	state := newAlterState(provider3)
	var storedAfterAlter *batchServerEntry
	state.afterApplyObserve = func(i int, _ *spec.Statement) {
		if i == 3 {
			if e := state.serverEntries[serverFold("s1")]; e != nil {
				v := *e
				storedAfterAlter = &v
			}
		}
	}
	enrichSlice(t, state, provider3, spec.PrivilegeAuditModeObservedAccountNewSession, statements, nil)
	if storedAfterAlter == nil || !storedAfterAlter.present || !storedAfterAlter.presentConditional {
		t.Fatalf("stored entry after A4 republish = %+v, want present + presentConditional=true", storedAfterAlter)
	}
}

// AM-2.3 (R2): frozen per-ALTER public projection for the suppressed-
// premise shape — 2 notice findings (main unverified + fixed
// connection_members advisory), owed exactly {server.alter_privilege},
// no conditional marker, coverage unverified, L=1/R=1/S=1. The
// REVOKE's own public output is asserted only as the terminator's
// scope effect, not re-adjudicated.
func TestT26AlterAM23FrozenPublicProjection(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, enriched := evaluateT26Statements(t,
		"REVOKE SELECT ON *.* FROM 'r1'@'%'; "+t26AlterSQL,
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	alter := result.Statements[1]
	if len(alter.Findings) != 2 {
		t.Fatalf("findings = %+v, want main + advisory", alter.Findings)
	}
	main := alter.Findings[0]
	if main.Metadata["outcome"] != "unverified" || main.Level != rule.LevelNotice {
		t.Fatalf("main = %+v, want notice/unverified", main)
	}
	if _, ok := main.Metadata["conditional_audit"]; ok {
		t.Fatalf("suppressed premise carries no marker: %+v", main.Metadata)
	}
	if alter.Findings[1].Metadata["notice"] != "connection_members" {
		t.Fatalf("advisory = %+v", alter.Findings[1])
	}
	if facts := alter.EvidenceGaps[0].RequiredFacts; len(facts) != 1 || facts[0] != spec.ServerFactAlterPrivilege {
		t.Fatalf("owed = %v, want exactly {server.alter_privilege}", facts)
	}
	if alter.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage = %v, want unverified", alter.Coverage.Status)
	}
	if adj := enriched[1].Metadata.ServerAlter; adj == nil || adj.Privilege != spec.ServerAlterPrivilegeSuppressed {
		t.Fatalf("premise = %+v, want suppressed by the leading terminator", adj)
	}
	if len(provider.calls) != 1 || provider.privilegeCalls != 1 {
		t.Fatalf("reads = R:%d L:%d, want 1/1", len(provider.calls), provider.privilegeCalls)
	}
}

// AM-1.2/2.2 (R2): catalog-present identity under mode off — the
// privilege premise stays unproven, so A4 owes exactly
// {server.alter_privilege}, republishes flag=false presence, carries
// no marker; L=0/R=1/S=1 (the catalog read still runs — mode only
// gates the privilege loader).
func TestT26AlterAM12ModeOff(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, err := AuditSQL(context.Background(), Request{
		SQL: t26AlterSQL, Dialect: spec.DialectMySQL, Schema: "golden", MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if facts := stmt.EvidenceGaps[0].RequiredFacts; len(facts) != 1 || facts[0] != spec.ServerFactAlterPrivilege {
		t.Fatalf("owed = %v, want exactly {server.alter_privilege}", facts)
	}
	if _, ok := stmt.Findings[0].Metadata["conditional_audit"]; ok {
		t.Fatalf("mode off carries no marker")
	}
	if stmt.Findings[0].Metadata["outcome"] != "unverified" || stmt.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("public = %+v coverage %v, want unverified", stmt.Findings[0].Metadata, stmt.Coverage.Status)
	}
	if provider.privilegeCalls != 0 {
		t.Fatalf("mode off loader = %d, want 0", provider.privilegeCalls)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver = %+v, want 1 — the catalog read still runs under mode off", provider.calls)
	}
}

// AM-1.4/1.5 (R2): zero-row catalog stays unknown — the ALTER's own
// apply mints no entry, no barrier, no write, and the owed set is the
// exact declared formula (existence under consumable; existence +
// alter_privilege under mode off / unproven).
func TestT26AlterAM14AM15ExactUnknown(t *testing.T) {
	// 1.4 consumable: owed {existence} exactly.
	provider := &t26Provider{servers: map[string]bool{}, version: "8.0.36", privilege: superObservation()}
	statements, _ := parseT26(t, t26AlterSQL)
	state := newAlterState(provider)
	enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements, nil)
	if entry := state.serverEntries[serverFold("s1")]; entry != nil {
		t.Fatalf("unknown alter minted an entry: %+v", entry)
	}
	if len(state.serverWrites) != 0 || state.serverChannelTainted {
		t.Fatalf("unknown alter recorded write/taint")
	}
	// 1.5 mode off: owed {existence, alter_privilege} exactly.
	provider = &t26Provider{servers: map[string]bool{}, version: "8.0.36"}
	result, err := AuditSQL(context.Background(), Request{
		SQL: t26AlterSQL, Dialect: spec.DialectMySQL, Schema: "golden", MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	facts := result.Statements[0].EvidenceGaps[0].RequiredFacts
	if len(facts) != 2 || facts[0] != spec.ServerFactExistence || facts[1] != spec.ServerFactAlterPrivilege {
		t.Fatalf("mode-off owed = %v, want exactly {existence, alter_privilege}", facts)
	}
}

// AM-4.1 (R2): DROP → ALTER refusal → CREATE. The proven absent
// refuses the ALTER (A2), and the trailing CREATE proves the absent
// is still consumable — OrderedAbsent consumed verbatim into the
// create's own proven publish. L=1/R=1/S=1.
func TestT26AlterAM41AbsentSurvivesRefusal(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t,
		"DROP SERVER s1; "+t26AlterSQL+"; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if adj := enriched[1].Metadata.ServerAlter; adj == nil || adj.Arm != spec.ServerAlterArmTerminalRefusal {
		t.Fatalf("alter on ordered absent = %+v, want refusal", adj)
	}
	createAdj := enriched[2].Metadata.ServerCreate
	if createAdj == nil || createAdj.Arm != spec.ServerCreateArmProven {
		t.Fatalf("create after refused alter = %+v, want proven — the absent survived the refusal", createAdj)
	}
	if len(provider.calls) != 1 || provider.privilegeCalls != 1 {
		t.Fatalf("reads = R:%d L:%d, want 1/1", len(provider.calls), provider.privilegeCalls)
	}
}

// AM-4.2 (R2): mode-off DROP leaves its accepted barrier; the ALTER
// binds that barrier — A1 owed exactly {batch_effects}, the original
// barrier entry preserved verbatim. L=0/R=1/S=1.
func TestT26AlterAM42ModeOffBarrier(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36"}
	statements, _ := parseT26(t, "DROP SERVER s1; "+t26AlterSQL)
	state := newAlterState(provider)
	enriched := enrichSlice(t, state, provider, "", statements, nil)
	adj := enriched[1].Metadata.ServerAlter
	if adj == nil || adj.Arm != spec.ServerAlterArmBarrier ||
		len(adj.Owed) != 1 || adj.Owed[0] != spec.ServerFactBatchEffects {
		t.Fatalf("alter on drop barrier = %+v, want A1 exact {server.batch_effects}", adj)
	}
	if adj.PublishPresent || adj.ConditionalAudit || adj.ConditionalSource {
		t.Fatalf("mode-off barrier carries no publish/provenance: %+v", adj)
	}
	entry := state.serverEntries[serverFold("s1")]
	if entry == nil || !entry.barrier {
		t.Fatalf("s1 entry = %+v, want the mode-off drop's barrier preserved", entry)
	}
	if provider.privilegeCalls != 0 || len(provider.calls) != 1 {
		t.Fatalf("reads = L:%d R:%d, want 0/1", provider.privilegeCalls, len(provider.calls))
	}
}

// AM-4.3 (R2): mode-off literal DROP taints the whole channel; the
// audited ALTER binds A1 — literal forward issued no provider read
// (S=0 at the SQL seam; the t26Provider stub still records the
// forward). L=0/R=1/S=0 for the literal; the ALTER itself binds the
// barrier with no read.
func TestT26AlterAM43LiteralDropTaint(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36"}
	enriched := enrichT26Premise(t, "DROP SERVER 'lit'; "+t26AlterSQL, provider, "")
	if enriched[0].Unsupported == nil {
		t.Fatalf("literal drop must stay boundary")
	}
	adj := enriched[1].Metadata.ServerAlter
	if adj == nil || adj.Arm != spec.ServerAlterArmBarrier || len(adj.Owed) != 1 || adj.Owed[0] != spec.ServerFactBatchEffects {
		t.Fatalf("tainted-channel alter = %+v, want A1 exact {batch_effects}", adj)
	}
	if adj.ConditionalAudit || adj.PublishPresent {
		t.Fatalf("mode-off barrier carries no marker/publish: %+v", adj)
	}
	if provider.privilegeCalls != 0 {
		t.Fatalf("mode off loader = %d, want 0", provider.privilegeCalls)
	}
}

// AM-5.1 (R2): observed consumable DROP→CREATE→ALTER→DROP — no
// terminator inserted. The ALTER is A3 consuming the conditional
// OrderedPresent and republishing flag=true; the trailing DROP
// consumes it. All four statements carry the conditional marker;
// L=1/R=1/S=1.
func TestT26AlterAM51FullChain(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, enriched := evaluateT26Statements(t,
		"DROP SERVER s1; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u'); "+t26AlterSQL+"; DROP SERVER s1",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	adj := enriched[2].Metadata.ServerAlter
	if adj == nil || adj.Arm != spec.ServerAlterArmProvenAlter || !adj.PublishPresent ||
		!adj.ConditionalSource || !adj.ConditionalAudit {
		t.Fatalf("alter = %+v, want A3 republishing flag=true with marker", adj)
	}
	for i := 0; i < 4; i++ {
		if _, ok := result.Statements[i].Findings[0].Metadata["conditional_audit"]; !ok {
			t.Fatalf("statement %d marker missing: %+v", i, result.Statements[i].Findings[0].Metadata)
		}
	}
	if last := enriched[3].Metadata.ServerDrop; last == nil || last.Arm != spec.ServerDropArmProven {
		t.Fatalf("tail drop = %+v, want proven on the republished present", last)
	}
	if len(provider.calls) != 1 || provider.privilegeCalls != 1 {
		t.Fatalf("reads = R:%d L:%d, want 1/1", len(provider.calls), provider.privilegeCalls)
	}
	// R3-B.4: the actual flag=true publication is observed at the
	// ALTER's own post-apply seam — afterApplyObserve on index 2
	// reads the stored entry while the ALTER's republish just
	// landed; the final DROP's ConditionalSource=true proves the
	// conditional flag was really consumed, not merely marked.
	provider2 := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	statements, _ := parseT26(t,
		"DROP SERVER s1; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u'); "+t26AlterSQL+"; DROP SERVER s1")
	state := newAlterState(provider2)
	var storedAfterAlter *batchServerEntry
	state.afterApplyObserve = func(i int, _ *spec.Statement) {
		if i == 2 {
			if e := state.serverEntries[serverFold("s1")]; e != nil {
				v := *e
				storedAfterAlter = &v
			}
		}
	}
	enriched2 := enrichSlice(t, state, provider2, spec.PrivilegeAuditModeObservedAccountNewSession, statements, nil)
	if storedAfterAlter == nil || !storedAfterAlter.present || !storedAfterAlter.presentConditional {
		t.Fatalf("stored entry after ALTER apply = %+v, want present + presentConditional=true", storedAfterAlter)
	}
	lastDrop := enriched2[3].Metadata.ServerDrop
	if lastDrop == nil || lastDrop.Arm != spec.ServerDropArmProven || !lastDrop.ConditionalSource {
		t.Fatalf("final drop = %+v, want proven consuming ConditionalSource=true", lastDrop)
	}
}

// AM-6.1 (R2): catalog-present ALTER → ALTER — the first republishes
// flag=false, the second consumes the ordered answer (no second
// read): both A3. L=1/R=1/S=1.
func TestT26AlterAM61OrderedConsume(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t, t26AlterSQL+"; ALTER SERVER s1 OPTIONS (HOST 'h')", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	for i := 0; i < 2; i++ {
		adj := enriched[i].Metadata.ServerAlter
		if adj == nil || adj.Arm != spec.ServerAlterArmProvenAlter || !adj.PublishPresent {
			t.Fatalf("stmt%d = %+v, want A3 publish", i, adj)
		}
	}
	if enriched[0].Metadata.ServerAlter.ConditionalSource || enriched[1].Metadata.ServerAlter.ConditionalSource {
		t.Fatalf("flags must stay false — catalog source republishes unconditional")
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver = %+v, want 1 — ordered answer consumed the second", provider.calls)
	}
}

// AM-6.2 (R2): two zero-row ALTERs under mode off — each owes the
// exact {existence, alter_privilege} pair, each apply mints nothing,
// and the verbatim cache reuses the one read.
func TestT26AlterAM62ZeroPair(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{}, version: "8.0.36"}
	statements, _ := parseT26(t, t26AlterSQL+"; ALTER SERVER s1 OPTIONS (HOST 'h')")
	state := newAlterState(provider)
	enriched := enrichSlice(t, state, provider, "", statements, nil)
	for i := 0; i < 2; i++ {
		adj := enriched[i].Metadata.ServerAlter
		if adj == nil || len(adj.Owed) != 2 || adj.Owed[0] != spec.ServerFactExistence || adj.Owed[1] != spec.ServerFactAlterPrivilege {
			t.Fatalf("stmt%d owed = %v, want exact {existence, alter_privilege}", i, adj.Owed)
		}
		if adj.PublishPresent {
			t.Fatalf("stmt%d must not publish on unknown", i)
		}
	}
	if len(state.serverEntries) != 0 {
		t.Fatalf("unknown alters minted entries: %+v", state.serverEntries)
	}
	if len(provider.calls) != 1 || provider.calls[0].name != "s1" {
		t.Fatalf("resolver = %+v, want one verbatim-cached read", provider.calls)
	}
}

// AM-6.4/6.5 (R2): zero-row ALTER (mode off) does not shadow siblings —
// the trailing CREATE/DROP gets its own fresh read and its own exact
// owed set; the verbatim cache shares the identity read.
// AM-6.3 (R3-C): the CLEAN catalog-present ALTER runs first — its
// A3 republishes unconditional presence (catalog source, flag=false);
// the later CREATE consumes that ordered publication and refuses
// refused_present with NO inherited marker (the source was never
// conditional). Distinct from AM-6.6's dirty-options row and from
// any CREATE→ALTER order.
func TestT26AlterAM63CleanAlterThenCreateRefused(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, enriched := evaluateT26Statements(t,
		t26AlterSQL+"; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	alter := enriched[0].Metadata.ServerAlter
	if alter == nil || alter.Arm != spec.ServerAlterArmProvenAlter || !alter.PublishPresent ||
		alter.ConditionalSource || !alter.ConditionalAudit {
		t.Fatalf("clean alter = %+v, want A3 publish flag=false own-marker", alter)
	}
	entry := enriched[0].Metadata.Server
	if entry == nil || !entry.Read || !entry.Exists {
		t.Fatalf("alter bound = %+v, want catalog read present", entry)
	}
	create := enriched[1].Metadata.ServerCreate
	if create == nil || create.Arm != spec.ServerCreateArmTerminalRefusal {
		t.Fatalf("create = %+v, want refused_present on the ordered publication", create)
	}
	if create.ConditionalSource || create.ConditionalAudit {
		t.Fatalf("create marker = %+v, want none — catalog source is never conditional", create)
	}
	createPublic := result.Statements[1]
	if createPublic.Findings[0].Metadata["outcome"] != "refused_present" ||
		createPublic.Findings[0].Level != rule.LevelBlocker {
		t.Fatalf("create public = %+v, want refused_present blocker", createPublic.Findings[0])
	}
	if _, ok := createPublic.Findings[0].Metadata["conditional_audit"]; ok {
		t.Fatalf("create public marker = %+v, want absent", createPublic.Findings[0].Metadata)
	}
	// One catalog read total: the ALTER's own read fed both
	// adjudications through the ordered publication.
	if len(provider.calls) != 1 || provider.privilegeCalls != 1 {
		t.Fatalf("reads = R:%d L:%d, want 1/1", len(provider.calls), provider.privilegeCalls)
	}
}

// AM-6.4/6.5 (R3-C): the frozen sibling fixtures — ALTER s1 runs
// first on a zero catalog, then a clean-options CREATE (HOST +
// DATABASE + USER) or a bare DROP on the same zero identity. Neither
// sibling shadows or inherits the ALTER's owed set: each owes its own
// EXACT pair — {server.existence, server.create_privilege} or
// {server.existence, server.drop_privilege} — never a superset, never
// the ALTER's alter_privilege.
func TestT26AlterAM64AM65NoShadow(t *testing.T) {
	for _, tc := range []struct {
		sql     string
		sibling string
		fact    string
	}{
		{"CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')", "create", spec.ServerFactCreatePrivilege},
		{"DROP SERVER s1", "drop", spec.ServerFactDropPrivilege},
	} {
		provider := &t26Provider{servers: map[string]bool{}, version: "8.0.36"}
		enriched := enrichT26Premise(t, t26AlterSQL+"; "+tc.sql, provider, "")
		var owed []string
		if tc.sibling == "create" {
			a := enriched[1].Metadata.ServerCreate
			if a == nil {
				t.Fatalf("%s: sibling adjudication missing", tc.sibling)
			}
			owed = a.Owed
		} else {
			a := enriched[1].Metadata.ServerDrop
			if a == nil {
				t.Fatalf("%s: sibling adjudication missing", tc.sibling)
			}
			owed = a.Owed
		}
		if len(owed) != 2 || owed[0] != spec.ServerFactExistence || owed[1] != tc.fact {
			t.Fatalf("%s: sibling owed = %v, want exactly {server.existence, %s}", tc.sibling, owed, tc.fact)
		}
		if len(provider.calls) != 1 {
			t.Fatalf("%s: resolver = %+v, want one shared verbatim read", tc.sibling, provider.calls)
		}
	}
}

// AM-7.1/7.2 (R3-C): the frozen ALTER→DROP ASCII alias chain. The
// present branch: ALTER s1 reads the catalog once and republishes
// flag=false, and DROP S1 consumes that ordered answer through the
// ASCII-fold provably-same relation with NO second resolver call —
// R=1. The zero and mode-off branches never produce an ordered
// answer, so s9 and its fold cousin S9 each take their own verbatim
// read — R=2. The ALTER→ALTER variants stay as supplementary rows.
func TestT26AlterAM71AM72CaseAlias(t *testing.T) {
	// AM-7.1 present branch: ALTER s1 → DROP S1 consumes the ordered
	// answer — one resolver call total.
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t, t26AlterSQL+"; DROP SERVER S1", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	alter := enriched[0].Metadata.ServerAlter
	if alter == nil || alter.Arm != spec.ServerAlterArmProvenAlter || !alter.PublishPresent {
		t.Fatalf("s1 alter = %+v, want A3 publishing flag=false", alter)
	}
	drop := enriched[1].Metadata.ServerDrop
	if drop == nil || drop.Arm != spec.ServerDropArmProven {
		t.Fatalf("S1 drop = %+v, want proven on the ordered alias answer", drop)
	}
	if bound := enriched[1].Metadata.Server; bound == nil || !bound.OrderedPresent {
		t.Fatalf("drop bound = %+v, want the ordered answer consumed, not a fresh read", bound)
	}
	if len(provider.calls) != 1 || provider.calls[0].name != "s1" {
		t.Fatalf("resolver = %+v, want exactly the ALTER's verbatim read", provider.calls)
	}
	// AM-7.2 zero branch: s9 zero → unknown, never an ordered answer;
	// DROP S9 takes its own verbatim read — R=2.
	provider = &t26Provider{servers: map[string]bool{}, version: "8.0.36"}
	enriched = enrichT26Premise(t, "ALTER SERVER s9 OPTIONS (USER 'u'); DROP SERVER S9", provider, "")
	if enriched[0].Metadata.ServerAlter == nil || enriched[1].Metadata.ServerDrop == nil {
		t.Fatalf("adjudications missing on the zero chain")
	}
	if bound := enriched[1].Metadata.Server; bound != nil && bound.OrderedPresent {
		t.Fatalf("zero chain bound = %+v, no ordered answer may exist", bound)
	}
	if len(provider.calls) != 2 || provider.calls[0].name != "s9" || provider.calls[1].name != "S9" {
		t.Fatalf("resolver = %+v, want two verbatim reads: s9 then S9", provider.calls)
	}
	// AM-7.2 mode-off branch: same verbatim reads under mode off —
	// the loader never fires but the resolver still reads each
	// authored spelling.
	provider = &t26Provider{servers: map[string]bool{}, version: "8.0.36"}
	enrichT26Premise(t, "ALTER SERVER s9 OPTIONS (USER 'u'); DROP SERVER S9", provider, "")
	if len(provider.calls) != 2 || provider.privilegeCalls != 0 {
		t.Fatalf("mode-off reads = R:%d L:%d, want 2/0", len(provider.calls), provider.privilegeCalls)
	}
	// Supplementary: the ALTER→ALTER alias consume stays registered —
	// it is not the AM-7.2 DROP-chain evidence.
	provider = &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched = enrichT26Premise(t, t26AlterSQL+"; ALTER SERVER S1 OPTIONS (HOST 'h')", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	adj := enriched[1].Metadata.ServerAlter
	if adj == nil || adj.Arm != spec.ServerAlterArmProvenAlter || !adj.PublishPresent {
		t.Fatalf("S1 alter = %+v, want A3 consuming the ordered alias", adj)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("alter→alter resolver = %+v, want 1", provider.calls)
	}
}

// AM-7.4 (R2): literal name and 65-rune name stay boundary separately —
// the literal forwards to the provider but declines without SQL
// (R=1/S=0 at the SQL seam); the 65-rune forwards likewise (R=1/S=1
// only when the provider executes — the stub counts the forward), and
// both record boundary writes that barrier the trailing statement.
func TestT26AlterAM74LiteralVsOverlong(t *testing.T) {
	// Literal: boundary + write record + provider forward declined.
	provider := &t26Provider{servers: map[string]bool{}, version: "8.0.36"}
	enriched := enrichT26Premise(t, "ALTER SERVER 'lit9' OPTIONS (USER 'u'); DROP SERVER s9", provider, "")
	if enriched[0].Unsupported == nil {
		t.Fatalf("literal alter must stay boundary")
	}
	if len(provider.calls) != 1 || provider.calls[0].name != "lit9" {
		t.Fatalf("resolver = %+v, want the literal forward (declined without identifier proof)", provider.calls)
	}
	if adj := enriched[1].Metadata.ServerDrop; adj == nil || adj.Arm != spec.ServerDropArmBarrier {
		t.Fatalf("drop after literal alter = %+v, want channel barrier", adj)
	}
	// 65-rune: boundary + own write → relation blocker on next read.
	long := strings.Repeat("a", 65)
	provider = &t26Provider{servers: map[string]bool{}, version: "8.0.36"}
	enriched = enrichT26Premise(t, "ALTER SERVER "+long+" OPTIONS (USER 'u'); DROP SERVER s1", provider, "")
	if enriched[0].Unsupported == nil {
		t.Fatalf("65-rune alter must stay boundary")
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver = %+v, want the boundary write's own forward", provider.calls)
	}
	if adj := enriched[1].Metadata.ServerDrop; adj == nil || adj.Arm != spec.ServerDropArmBarrier {
		t.Fatalf("drop after 65-rune alter = %+v, want relation barrier", adj)
	}
}

// AM-7.5 (R3-A.4): proven-present ALTER → literal boundary ALTER →
// DROP S1. The literal's OWN apply is observed directly on the
// slice-seam: it taints the channel AND clears the standing
// OrderedPresent — the s1 entry's present flag reads false
// immediately after stmt2's apply, before any later statement runs.
// The trailing DROP's barrier is corroboration only: a tainted bind
// exits before any consumption, so the DROP alone could never prove
// the presence was cleared. L=1/R=2/S=1 (literal forward counts R,
// declines S).
func TestT26AlterAM75LiteralClearsStalePresence(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	statements, failures := parseT26(t, t26AlterSQL+"; ALTER SERVER 'lit' OPTIONS (USER 'u'); DROP SERVER S1")
	state := newAlterState(provider)
	enriched := enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements[:1], failures)
	if adj := enriched[0].Metadata.ServerAlter; adj == nil || adj.Arm != spec.ServerAlterArmProvenAlter {
		t.Fatalf("first alter = %+v, want A3", adj)
	}
	s1key := serverFold("s1")
	if entry := state.serverEntries[s1key]; entry == nil || !entry.present {
		t.Fatalf("s1 entry before literal = %+v, want published OrderedPresent", entry)
	}
	literal := enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements[1:2], nil)
	if literal[0].Unsupported == nil {
		t.Fatalf("literal alter must stay boundary")
	}
	if !state.serverChannelTainted {
		t.Fatalf("literal apply must taint the channel")
	}
	entry := state.serverEntries[s1key]
	if entry == nil || entry.present || entry.presentConditional || entry.absent || entry.publishedArm != 0 {
		t.Fatalf("s1 entry after literal apply = %+v, want OrderedPresent directly cleared", entry)
	}
	enriched2 := enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements[2:], nil)
	dropAdj := enriched2[0].Metadata.ServerDrop
	if dropAdj == nil || dropAdj.Arm != spec.ServerDropArmBarrier {
		t.Fatalf("S1 drop = %+v, want barrier — literal write invalidated the stale presence", dropAdj)
	}
	// Segmented-fixture counts: three enrich calls mean the loader
	// fired for the two privilege-needing slices (stmt1 ALTER, stmt3
	// DROP — the literal slice needs none), L=2 honestly — distinct
	// from the frozen single-request chain's L=1 (the whole-batch
	// fixtures assert that shape; R3-B.5). R=2: the ALTER's catalog
	// read + the literal's declined forward.
	if len(provider.calls) != 2 || provider.privilegeCalls != 2 {
		t.Fatalf("reads = R:%d L:%d, want 2/2 segmented (literal forward counted, declined)", len(provider.calls), provider.privilegeCalls)
	}
}

// AM-3.1–3.5 (R2): authored OPTIONS → OptionsClean → arm chain.
// Duplicate keys and out-of-range numerics make the options premise
// dirty — the arm falls to A4 owing {server.alter_options} on the
// exact dirty path. In-range PORT values, the unknown FOO key, and a
// quoted PORT stay clean — the consumable arm is A3. Empty OPTIONS
// and malformed lists remain the parser's own boundary (covered in
// the tidb package's strict tests), never reaching adjudication.
func TestT26AlterAM3OptionsChain(t *testing.T) {
	for _, tc := range []struct {
		sql   string
		clean bool
	}{
		{"ALTER SERVER s1 OPTIONS (PORT 9999)", true},
		// The documented bound is 0–9999: one rung over it is a real
		// parsed numeric but out-of-range — dirty premise (AM-3.2).
		{"ALTER SERVER s1 OPTIONS (PORT 10000)", false},
		{"ALTER SERVER s1 OPTIONS (USER 'u', USER 'v')", false},
		{"ALTER SERVER s1 OPTIONS (PORT 100000)", false},
	} {
		provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
		parsed, _ := parseSQL(context.Background(), tc.sql, spec.DialectMySQL)
		statements, err := Extract(context.Background(), parsed)
		if err != nil {
			t.Fatalf("%s: extract: %v", tc.sql, err)
		}
		decl := statements[0].DDL.Server
		if decl == nil {
			t.Fatalf("%s: declaration missing", tc.sql)
		}
		if decl.OptionsClean() != tc.clean {
			t.Fatalf("%s: OptionsClean = %v, want %v", tc.sql, decl.OptionsClean(), tc.clean)
		}
		state := newAlterState(provider)
		enriched := enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements, nil)
		adj := enriched[0].Metadata.ServerAlter
		if tc.clean {
			if adj == nil || adj.Arm != spec.ServerAlterArmProvenAlter || len(adj.Owed) != 0 {
				t.Fatalf("%s: clean consumable alter = %+v, want A3 no-owed", tc.sql, adj)
			}
		} else {
			if adj == nil || adj.Arm != spec.ServerAlterArmUnproven ||
				len(adj.Owed) != 1 || adj.Owed[0] != spec.ServerFactAlterOptions {
				t.Fatalf("%s: dirty alter = %+v, want A4 owed exactly {server.alter_options}", tc.sql, adj)
			}
		}
	}
	// Unclaimed option forms are the parser's own boundary — a real
	// failure record, no audited declaration, downstream fields N/A.
	for _, sql := range []string{
		"ALTER SERVER s1 OPTIONS (FOO 'x')",
		"ALTER SERVER s1 OPTIONS (PORT '3306')",
		"ALTER SERVER s1 OPTIONS ()",
	} {
		parsed, _ := parseSQL(context.Background(), sql, spec.DialectMySQL)
		statements, err := Extract(context.Background(), parsed)
		if err != nil {
			t.Fatalf("%s: extract: %v", sql, err)
		}
		if len(parsed.failures) != 1 || len(statements) != 0 {
			t.Fatalf("%s: want one failure record and no statement — failures=%+v statements=%d", sql, parsed.failures, len(statements))
		}
	}
}

// AM-10.4 (R2): mode-off literal ALTER → DROP s9 — the boundary form
// keeps the fixed skeleton and no audited finding; its apply taints
// the channel so the trailing drop binds the barrier with zero reads.
func TestT26AlterAM104LiteralModeOff(t *testing.T) {
	// Frozen fixture (R3-C): provider s9 zero-row, mode off, clean
	// channel — the literal write alone produces the write record and
	// channel taint the trailing DROP binds.
	provider := &t26Provider{servers: map[string]bool{}, version: "8.0.36"}
	result, _ := evaluateT26Statements(t, "ALTER SERVER 'lit9' OPTIONS (USER 'u'); DROP SERVER s9", provider, "")
	lit := result.Statements[0]
	if lit.RawSQL != "ALTER SERVER <redacted>" {
		t.Fatalf("boundary alter must carry the fixed skeleton: %q", lit.RawSQL)
	}
	if len(lit.Findings) != 0 {
		t.Fatalf("boundary alter must emit no audited findings: %+v", lit.Findings)
	}
	drop := result.Statements[1]
	if drop.Findings[0].Metadata["outcome"] != "blocked_batch_effects" {
		t.Fatalf("drop after literal alter = %+v, want channel barrier", drop.Findings[0].Metadata)
	}
	if provider.privilegeCalls != 0 {
		t.Fatalf("mode off loader = %d, want 0", provider.privilegeCalls)
	}
	// literal forward (R) recorded once; trailing drop binds taint — no
	// further resolver call.
	if len(provider.calls) != 1 || provider.calls[0].name != "lit9" {
		t.Fatalf("resolver = %+v, want exactly the literal's forward", provider.calls)
	}
}

// T26-SERVER-ALTER-EXECUTION-01 application-layer verification:
// version gate ordering before the family loader, the independent
// ALTER premise projection, the shared adjudication arms end to end,
// OrderedPresent republish/consumption, the presence-preservation
// no-ops, contamination semantics, and the bounded public output
// shape — the accepted contract's AM rows as real tests.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t26AlterSQL = "ALTER SERVER s1 OPTIONS (USER 'u')"

// G2 fires before the family loader: an unknown or out-of-domain
// resolved version marks the qualified ALTER unaudited with
// gate:"version" — the loader never runs and no adjudication is
// attached. A proven 8.0.x banner leaves the statement audited and
// an ALTER-only batch triggers the family observation once.
func TestT26AlterVersionGate(t *testing.T) {
	for _, tc := range []struct {
		name         string
		version      string
		wantGate     bool
		wantPrivCall int
	}{
		{"unknown version gates", "", true, 0},
		{"out-of-domain 8.4 gates", "8.4.10", true, 0},
		{"5.7 gates", "5.7.44", true, 0},
		{"proven 8.0.x audits", "8.0.36", false, 1},
	} {
		provider := &t26Provider{servers: map[string]bool{}, version: tc.version, privilege: superObservation()}
		enriched := enrichT26Premise(t, t26AlterSQL, provider, spec.PrivilegeAuditModeObservedAccountNewSession)
		stmt := enriched[0]
		if tc.wantGate {
			if stmt.Unsupported == nil || stmt.Unsupported.Reason != spec.UnsupportedUnauditedReason {
				t.Fatalf("%s: must be unaudited: %+v", tc.name, stmt.Unsupported)
			}
			if stmt.Unsupported.Metadata["gate"] != "version" {
				t.Fatalf("%s: gate marker missing: %+v", tc.name, stmt.Unsupported.Metadata)
			}
			if stmt.Metadata != nil && stmt.Metadata.ServerAlter != nil {
				t.Fatalf("%s: gated statement must carry no adjudication", tc.name)
			}
		} else {
			if stmt.Unsupported != nil {
				t.Fatalf("%s: proven 8.0.x must audit: %+v", tc.name, stmt.Unsupported)
			}
			if stmt.Metadata == nil || stmt.Metadata.ServerAlter == nil {
				t.Fatalf("%s: audited alter must carry the adjudication", tc.name)
			}
		}
		if provider.privilegeCalls != tc.wantPrivCall {
			t.Fatalf("%s: loader calls = %d, want %d", tc.name, provider.privilegeCalls, tc.wantPrivCall)
		}
	}
}

// Keep-boundary-bind (contract §E): a version-gated ALTER still walks
// the kind-agnostic bind — the provider-executing read fires once
// under a clean channel while the loader stays gated off.
func TestT26AlterVersionGateStillBinds(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "5.7.44", privilege: superObservation()}
	enriched := enrichT26Premise(t, t26AlterSQL, provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if enriched[0].Unsupported == nil || enriched[0].Unsupported.Metadata["gate"] != "version" {
		t.Fatalf("out-of-domain alter must be version-gated: %+v", enriched[0].Unsupported)
	}
	if len(provider.calls) != 1 || provider.calls[0].typ != "mysql_server" {
		t.Fatalf("resolver calls = %+v, want one keep-boundary-bind read", provider.calls)
	}
	if provider.privilegeCalls != 0 {
		t.Fatalf("loader must stay gated: %d calls", provider.privilegeCalls)
	}
}

// The premise is independent: a consumable witness discharges only
// server.alter_privilege on the ALTER, and a scope terminator
// suppresses the ALTER's own projection — never a borrowed sibling
// premise (AM-2.1/2.3).
func TestT26AlterPrivilegeProjection(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t, t26AlterSQL, provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	adj := enriched[0].Metadata.ServerAlter
	if adj == nil || adj.Privilege != spec.ServerAlterPrivilegeConsumable {
		t.Fatalf("privilege = %+v, want consumable", adj)
	}
	for _, f := range adj.Owed {
		if f == spec.ServerFactDropPrivilege || f == spec.ServerFactCreatePrivilege {
			t.Fatalf("a sibling premise must never be owed on an alter: %v", adj.Owed)
		}
	}

	provider = &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched = enrichT26Premise(t, "REVOKE SELECT ON *.* FROM 'r1'@'%'; "+t26AlterSQL, provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	adj = enriched[1].Metadata.ServerAlter
	if adj == nil || adj.Privilege != spec.ServerAlterPrivilegeSuppressed {
		t.Fatalf("premise = %+v, want suppressed after the terminator", adj)
	}
	if len(adj.Owed) != 1 || adj.Owed[0] != spec.ServerFactAlterPrivilege {
		t.Fatalf("suppressed alter owes exactly server.alter_privilege: %v", adj.Owed)
	}
}

// A3 ProvenAlter: catalog present + consumable + clean owes nothing,
// emits main + advisory under the notice policy, republishes
// OrderedPresent flag=false, and a later same-identity DROP consumes
// the re-published present without a second catalog read (AM-1.1,
// AM-6.x republish).
func TestT26AlterProvenRepublish(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, err := AuditSQL(context.Background(), Request{
		SQL: t26AlterSQL + "; DROP SERVER s1", Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	findings := result.Statements[0].Findings
	if len(findings) != 2 {
		t.Fatalf("A3 + advisory = 2 findings, got %+v", findings)
	}
	if findings[0].Metadata["outcome"] != "conditional_alter" || findings[0].Level != rule.LevelNotice {
		t.Fatalf("arm finding = %+v, want notice/conditional_alter", findings[0])
	}
	if findings[0].Metadata["conditional_audit"] != string(spec.PrivilegeAuditModeObservedAccountNewSession) {
		t.Fatalf("A3 marker missing: %+v", findings[0].Metadata)
	}
	if findings[1].Metadata["notice"] != "connection_members" {
		t.Fatalf("advisory = %+v, want connection_members", findings[1])
	}
	if len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("A3 owes nothing: %+v", result.Statements[0].EvidenceGaps)
	}
	// The re-published presence answers the drop — one catalog read total.
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v, want exactly one", provider.calls)
	}
	dropFinding := result.Statements[1].Findings[0]
	if dropFinding.Metadata["outcome"] != "conditional_drop" {
		t.Fatalf("drop after republished alter = %+v, want conditional_drop on OrderedPresent", dropFinding)
	}
}

// The published flag is ConditionalSource — never ConditionalAudit
// (AM-6.6): a catalog-sourced A4 under an own consumable premise marks
// its own audit conditional while republishing presence flag=false;
// a later CREATE over the standing present refuses with no marker.
func TestT26AlterConditionalAxesProjection(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "ALTER SERVER s1 OPTIONS (USER 'u', USER 'u2'); CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')",
		Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	alter := result.Statements[0]
	if alter.Findings[0].Metadata["outcome"] != "unverified" {
		t.Fatalf("dirty-catalog alter outcome = %v, want unverified", alter.Findings[0].Metadata["outcome"])
	}
	if _, ok := alter.Findings[0].Metadata["conditional_audit"]; !ok {
		t.Fatalf("own consumable premise must mark the audit: %+v", alter.Findings[0].Metadata)
	}
	// Republished presence is unconditional — the create refuses with
	// no inherited marker.
	create := result.Statements[1]
	if create.Findings[0].Metadata["outcome"] != "refused_present" {
		t.Fatalf("create after republished alter = %+v, want refused_present", create.Findings[0].Metadata)
	}
	if _, ok := create.Findings[0].Metadata["conditional_audit"]; ok {
		t.Fatalf("published flag must be ConditionalSource=false, never ConditionalAudit: %+v", create.Findings[0].Metadata)
	}
	if result.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReject {
		t.Fatalf("AM-6.6 aggregate = coverage %v verdict %v, want unverified/reject", result.Coverage.Status, result.Verdict)
	}
}

// A4 on ordered present with a suppressed premise: owed exactly
// {server.alter_privilege}, the inherited conditional flag republishes
// verbatim, and the marker comes from the consumed source (AM-2.5i).
func TestT26AlterInheritedRepublish(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, _ := evaluateT26Statements(t,
		"DROP SERVER s1; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u'); REVOKE SELECT ON *.* FROM 'r1'@'%'; "+t26AlterSQL,
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	alter := result.Statements[3]
	if facts := alter.EvidenceGaps[0].RequiredFacts; len(facts) != 1 || facts[0] != spec.ServerFactAlterPrivilege {
		t.Fatalf("owed = %v, want exactly {server.alter_privilege}", facts)
	}
	if alter.Findings[0].Metadata["outcome"] != "unverified" {
		t.Fatalf("outcome = %v, want unverified", alter.Findings[0].Metadata["outcome"])
	}
	if _, ok := alter.Findings[0].Metadata["conditional_audit"]; !ok {
		t.Fatalf("inherited conditional marker missing: %+v", alter.Findings[0].Metadata)
	}
	if len(alter.Findings) != 2 || alter.Findings[1].Metadata["notice"] != "connection_members" {
		t.Fatalf("A4 emits main + advisory: %+v", alter.Findings)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v, want one — ordered channel answered the alter", provider.calls)
	}
}

// A2 refused_absent: DROP publishes the conditional absent and the
// trailing ALTER refuses — blocker, owed ∅, inherited marker, complete
// coverage, no advisory (AM-2.4i public shape).
func TestT26AlterRefusedAbsent(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, _ := evaluateT26Statements(t,
		"DROP SERVER s1; REVOKE SELECT ON *.* FROM 'r1'@'%'; "+t26AlterSQL,
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	alter := result.Statements[2]
	if len(alter.Findings) != 1 || alter.Findings[0].Level != rule.LevelBlocker {
		t.Fatalf("A2 emits one blocker, got %+v", alter.Findings)
	}
	if alter.Findings[0].Metadata["outcome"] != "refused_absent" {
		t.Fatalf("outcome = %v, want refused_absent", alter.Findings[0].Metadata["outcome"])
	}
	if _, ok := alter.Findings[0].Metadata["conditional_audit"]; !ok {
		t.Fatalf("refusal on conditional absent must inherit the marker: %+v", alter.Findings[0].Metadata)
	}
	if len(alter.EvidenceGaps) != 0 || alter.Coverage.Status != report.CoverageComplete {
		t.Fatalf("A2 = gaps %+v coverage %v, want none/complete", alter.EvidenceGaps, alter.Coverage.Status)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("verdict = %v, want reject on the refusal blocker", result.Verdict)
	}
}

// A4 on zero-row catalog: unknown stays unknown — no publish, no
// barrier, no entry; the consumable own premise still marks the audit
// (AM-1.4); mode off owes existence+privilege (AM-1.5).
func TestT26AlterZeroRowNoState(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{}, version: "8.0.36", privilege: superObservation()}
	result, err := AuditSQL(context.Background(), Request{
		SQL: t26AlterSQL + "; " + t26AlterSQL, Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	first := result.Statements[0]
	if facts := first.EvidenceGaps[0].RequiredFacts; len(facts) != 1 || facts[0] != spec.ServerFactExistence {
		t.Fatalf("zero-row owed = %v, want exactly {server.existence}", facts)
	}
	if _, ok := first.Findings[0].Metadata["conditional_audit"]; !ok {
		t.Fatalf("own consumable premise must mark the audit: %+v", first.Findings[0].Metadata)
	}
	// No publish on unknown — the second ALTER takes its own
	// unproven read through the same objectCache identity (deduped),
	// never a barrier the first minted.
	second := result.Statements[1]
	if second.Findings[0].Metadata["outcome"] != "unverified" {
		t.Fatalf("second zero-row alter outcome = %v, want unverified (never batch_effects)", second.Findings[0].Metadata["outcome"])
	}
	if facts := second.EvidenceGaps[0].RequiredFacts; len(facts) != 1 || facts[0] != spec.ServerFactExistence {
		t.Fatalf("second owed = %v, want {server.existence}", facts)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v, want one deduped identity read", provider.calls)
	}

	// Mode off: every premise stays owed, the loader never runs.
	provider = &t26Provider{servers: map[string]bool{}, version: "8.0.36"}
	result, err = AuditSQL(context.Background(), Request{
		SQL: t26AlterSQL, Dialect: spec.DialectMySQL, Schema: "golden", MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	facts := map[string]bool{}
	for _, f := range result.Statements[0].EvidenceGaps[0].RequiredFacts {
		facts[f] = true
	}
	if !facts[spec.ServerFactExistence] || !facts[spec.ServerFactAlterPrivilege] {
		t.Fatalf("mode-off owed = %v, want {existence, alter_privilege}", facts)
	}
	if _, ok := result.Statements[0].Findings[0].Metadata["conditional_audit"]; ok {
		t.Fatalf("mode off carries no marker")
	}
	if provider.privilegeCalls != 0 {
		t.Fatalf("mode off must never run the loader: %d", provider.privilegeCalls)
	}
}

// A1 relation-scan barrier is a uniform ordered-channel no-op
// (AM-7.3/AM-7.6): ALTER sé on catalog presence publishes; ALTER se
// hits the unproven-relation barrier, owes exactly {batch_effects},
// mints nothing — the se entry is created only by the later DROP,
// which records its own barrier while sé's presence stands.
func TestT26AlterRelationBarrierNoOp(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"sé": true}, version: "8.0.36", privilege: superObservation()}
	parsed, parseErr := parseSQL(context.Background(),
		"ALTER SERVER `sé` OPTIONS (USER 'u'); ALTER SERVER se OPTIONS (USER 'u'); DROP SERVER se",
		spec.DialectMySQL)
	if parseErr != nil {
		t.Fatalf("parse: %v", parseErr)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	state := newBatchState(spec.DialectMySQL, "golden", provider)
	state.resolvedVersion = &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.36", Major: 8, Minor: 0, Patch: 36, ValidatedRange: true}
	state.instanceFacts = &spec.InstanceFacts{Version: "8.0.36"}
	// The call boundary is the apply seam: stmt1 alone, snapshot the
	// COMPLETE ordered channel, then stmt2 — its own apply must leave
	// entries, writes and taint byte-identical (R2 §2.3 uniform A1).
	enriched := enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements[:1], nil)
	if adj := enriched[0].Metadata.ServerAlter; adj == nil || !adj.PublishPresent {
		t.Fatalf("sé alter = %+v, want publish on proven presence", adj)
	}
	before := snapshotServerChannel(state)
	enriched = enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements[1:2], nil)
	adj2 := enriched[0].Metadata.ServerAlter
	if adj2 == nil || adj2.Arm != spec.ServerAlterArmBarrier || len(adj2.Owed) != 1 || adj2.Owed[0] != spec.ServerFactBatchEffects {
		t.Fatalf("se alter = %+v, want barrier owing exactly {server.batch_effects}", adj2)
	}
	if adj2.PublishPresent || adj2.ConditionalAudit {
		t.Fatalf("A1 must not publish or mark: %+v", adj2)
	}
	if after := snapshotServerChannel(state); after != before {
		t.Fatalf("A1 apply mutated the ordered channel\nbefore=%s\nafter=%s", before, after)
	}
	// stmt3's own DROP apply mints the se barrier entry — the
	// snapshot boundary stays around stmt2 so the later statement's
	// legitimate effect is never blamed on the ALTER.
	enriched = enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements[2:], nil)
	// stmt3 DROP se: the relation barrier still binds; DROP's own
	// consumable premise marks it and it mints the se barrier entry
	// (accepted DROP behavior, unchanged).
	dropAdj := enriched[0].Metadata.ServerDrop
	if dropAdj == nil || dropAdj.Arm != spec.ServerDropArmBarrier || len(dropAdj.Owed) != 1 || dropAdj.Owed[0] != spec.ServerFactBatchEffects {
		t.Fatalf("drop se = %+v, want barrier owing exactly {server.batch_effects}", dropAdj)
	}
	entry := state.serverEntries[serverFold("se")]
	if entry == nil || !entry.barrier || entry.present || entry.absent {
		t.Fatalf("se entry after drop = %+v, want the drop's own barrier", entry)
	}
	// sé's republished presence stands untouched.
	seEntry := state.serverEntries[serverFold("sé")]
	if seEntry == nil || !seEntry.present || seEntry.barrier || seEntry.absent {
		t.Fatalf("sé entry = %+v, want republished presence standing", seEntry)
	}
}

// A1 channel taint: a literal-name boundary ALTER taints the whole
// channel; a later audited ALTER binds the barrier — the uniform
// no-op, never a stale answer (AM-7.5/AM-10.4 shape).
func TestT26AlterChannelTaintBarrier(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	statements, _ := parseT26(t, "ALTER SERVER 'lit9' OPTIONS (USER 'u'); "+t26AlterSQL+"; DROP SERVER s9")
	state := newAlterState(provider)
	enriched := enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements[:1], nil)
	if enriched[0].Unsupported == nil || enriched[0].Unsupported.Reason != spec.UnsupportedUnauditedReason {
		t.Fatalf("literal alter must keep the unaudited boundary: %+v", enriched[0].Unsupported)
	}
	// The literal's own apply recorded its write + channel taint; the
	// audited ALTER's own apply must leave the complete channel
	// byte-identical (R2 §2.3 uniform A1).
	before := snapshotServerChannel(state)
	enriched = enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements[1:2], nil)
	adj := enriched[0].Metadata.ServerAlter
	if adj == nil || adj.Arm != spec.ServerAlterArmBarrier || len(adj.Owed) != 1 || adj.Owed[0] != spec.ServerFactBatchEffects {
		t.Fatalf("tainted-channel alter = %+v, want barrier owing batch_effects", adj)
	}
	if after := snapshotServerChannel(state); after != before {
		t.Fatalf("A1 apply mutated the tainted channel\nbefore=%s\nafter=%s", before, after)
	}
	// The trailing DROP hits the same barrier — consistent with the
	// accepted literal-write taint.
	enriched = enrichSlice(t, state, provider, spec.PrivilegeAuditModeObservedAccountNewSession, statements[2:], nil)
	dropAdj := enriched[0].Metadata.ServerDrop
	if dropAdj == nil || dropAdj.Arm != spec.ServerDropArmBarrier {
		t.Fatalf("drop after literal alter = %+v, want barrier", dropAdj)
	}
}

// Real whole-batch contamination is A1's third source (AM-8.1): a
// parse failure record sets state.contaminated through the real enrich
// loop; the tail ALTER binds unvouched, owes {batch_effects}, mints
// nothing — the complete entry map is byte-identical across the call
// boundary.
func TestT26AlterRealContamination(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	parsed, parseErr := parseSQL(context.Background(),
		t26AlterSQL+"; GARBAGE TOKENS HERE; ALTER SERVER s1 OPTIONS (HOST 'h')",
		spec.DialectMySQL)
	if len(parsed.failures) != 1 {
		t.Fatalf("parse: failures=%d err=%v, want exactly 1", len(parsed.failures), parseErr)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	state := newBatchState(spec.DialectMySQL, "golden", provider)
	state.resolvedVersion = &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.36", Major: 8, Minor: 0, Patch: 36, ValidatedRange: true}
	state.instanceFacts = &spec.InstanceFacts{Version: "8.0.36"}
	// call1: prefix publishes OrderedPresent; the failure record is
	// consumed by this call's own loop, so the trailing ALTER sees
	// the real contaminated flag.
	enriched, err := state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession},
		statements, parsed.failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if !state.contaminated {
		t.Fatalf("contaminated = false after the real parse failure record")
	}
	if adj := enriched[0].Metadata.ServerAlter; adj == nil || !adj.PublishPresent {
		t.Fatalf("prefix alter = %+v, want republish on proven presence", adj)
	}
	tail := enriched[1].Metadata.ServerAlter
	if tail == nil || tail.Arm != spec.ServerAlterArmBarrier || len(tail.Owed) != 1 || tail.Owed[0] != spec.ServerFactBatchEffects {
		t.Fatalf("tail alter = %+v, want barrier owing exactly {server.batch_effects}", tail)
	}
	if tail.PublishPresent || tail.ConditionalAudit || tail.ConditionalSource {
		t.Fatalf("contaminated alter must not publish or carry provenance: %+v", tail)
	}
	entry := state.serverEntries[serverFold("s1")]
	if entry == nil || !entry.present || entry.barrier || entry.absent {
		t.Fatalf("entry = %+v, want the prefix's OrderedPresent standing — A1 never clears", entry)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v, want one — the tail read never issued", provider.calls)
	}
}

// Server-channel isolation (AM-10.3): an audited ALTER touches only
// server state — the COMPLETE table/schema registry state is compared
// across the ALTER's own apply via deep-rendered snapshots, never a
// key-presence projection, so an in-place field rewrite on a
// pre-existing entry is caught. A prior CREATE TABLE mints a real
// populated entry first so the compare actually exercises rewrite
// detection; the trailing CREATE TABLE keeps its own model effect.
// Read counts report the server channel only — table-channel reads
// are recorded separately, never mixed in.
func TestT26AlterTableChannelIsolation(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	parsed, _ := parseSQL(context.Background(),
		"CREATE TABLE pre (a int); "+t26AlterSQL+"; CREATE TABLE t (a int)", spec.DialectMySQL)
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	state := newBatchState(spec.DialectMySQL, "golden", provider)
	state.resolvedVersion = &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.36", Major: 8, Minor: 0, Patch: 36, ValidatedRange: true}
	state.instanceFacts = &spec.InstanceFacts{Version: "8.0.36"}
	req := &MetadataRequest{Schema: "golden", Provider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession}
	// stmt0: CREATE TABLE pre mints a real table entry — a populated
	// registry the ALTER must leave untouched.
	if _, err = state.enrichStatements(context.Background(), req,
		statements[:1], nil, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit}); err != nil {
		t.Fatalf("enrich stmt0: %v", err)
	}
	if len(state.entries) == 0 {
		t.Fatalf("pre-create minted no table entry — fixture broken")
	}
	before := snapshotTableSchemaState(state)
	serverReadsBefore := len(provider.calls)
	// stmt1: the audited ALTER's own bind+apply.
	if _, err = state.enrichStatements(context.Background(), req,
		statements[1:2], nil, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit}); err != nil {
		t.Fatalf("enrich stmt1: %v", err)
	}
	after := snapshotTableSchemaState(state)
	if before != after {
		t.Fatalf("table/schema state changed across the ALTER's own apply\nbefore=%s\nafter=%s", before, after)
	}
	if entry := state.serverEntries[serverFold("s1")]; entry == nil || !entry.present {
		t.Fatalf("server channel = %+v, want republished present on s1", entry)
	}
	// stmt2: CREATE TABLE t's own model effect — mints t@golden while
	// the server channel still holds exactly the s1 record.
	if _, err = state.enrichStatements(context.Background(), req,
		statements[2:], nil, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit}); err != nil {
		t.Fatalf("enrich stmt2: %v", err)
	}
	if state.entries[batchTableKey{dialect: spec.DialectMySQL, schema: "golden", table: "t"}] == nil {
		t.Fatalf("CREATE TABLE t must mint its own entry")
	}
	serverReads := 0
	for _, c := range provider.calls {
		if c.typ == "mysql_server" {
			serverReads++
		}
	}
	if serverReads != 1 {
		t.Fatalf("server-channel reads = %d, want exactly one (the ALTER's catalog read)", serverReads)
	}
	if other := len(provider.calls) - serverReads - serverReadsBefore; other < 0 {
		t.Fatalf("read accounting broken: total=%d server=%d", len(provider.calls), serverReads)
	}
}

// snapshotTableSchemaState renders the complete table, temp-table,
// and schema registry values into an immutable string — an
// independent copy of every field, not a key-presence projection, so
// any in-place mutation anywhere in the entry graphs is detected.
func snapshotTableSchemaState(s *batchState) string {
	return fmt.Sprintf("entries=%#v|temp=%#v|schemas=%#v", s.entries, s.tempEntries, s.schemaEntries)
}

// AM-1.6: enrich-layer fixture with the in-domain resolved version
// supplied directly (resolvedVersion=8.0.36), the observed mode
// requested, and no provider/loader wired at all. The premise stays
// unproven so the alter adjudicates A4 owing exactly
// {server.existence, server.alter_privilege} — no marker, no publish.
// With no provider or loader wired there is nothing to count: L/R/S=0
// by construction, never a count asserted on an unwired object.
func TestT26AlterResolvedVersionFixture(t *testing.T) {
	parsed, _ := parseSQL(context.Background(), t26AlterSQL, spec.DialectMySQL)
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectMySQL,
		&MetadataRequest{
			Schema:             "golden",
			PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
			TargetVersion:      &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.36", Major: 8, Minor: 0, Patch: 36, ValidatedRange: true},
		},
		statements, nil)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	adj := enriched[0].Metadata.ServerAlter
	if adj == nil || adj.Arm != spec.ServerAlterArmUnproven {
		t.Fatalf("providerless observed alter = %+v, want unproven", adj)
	}
	if adj.Privilege != spec.ServerAlterPrivilegeUnproven {
		t.Fatalf("premise = %+v, want unproven — the loader never ran", adj.Privilege)
	}
	if len(adj.Owed) != 2 || adj.Owed[0] != spec.ServerFactExistence || adj.Owed[1] != spec.ServerFactAlterPrivilege {
		t.Fatalf("owed = %v, want exactly {server.existence, server.alter_privilege}", adj.Owed)
	}
	if adj.PublishPresent || adj.ConditionalSource || adj.ConditionalAudit {
		t.Fatalf("A4 must not publish or carry provenance: %+v", adj)
	}
}

// Public redaction holds on every arm (AM-10.1, §6.3): distinguishable
// name, host, and password sentinels never reach ANY serialized public
// field — the COMPLETE public result object is marshalled and the
// whole byte stream is scanned, never a hand-built concatenation of a
// few known fields. RawSQL and NormalizedSQL both equal the fixed
// skeleton exactly.
func TestT26AlterPublicRedaction(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"pw-sentinel-455": true}, version: "8.0.36", privilege: superObservation()}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "ALTER SERVER `pw-sentinel-455` OPTIONS (USER 'u', HOST 'host-sentinel-77', PASSWORD 's3cr3t')",
		Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if stmt.RawSQL != "ALTER SERVER <redacted>" || stmt.NormalizedSQL != "ALTER SERVER <redacted>" {
		t.Fatalf("sql fields must equal the fixed redacted skeleton exactly: raw=%q normalized=%q", stmt.RawSQL, stmt.NormalizedSQL)
	}
	serialized, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	blob := string(serialized)
	for _, protected := range []string{"pw-sentinel-455", "host-sentinel-77", "s3cr3t", "audited@%", "GRANT", "'u'"} {
		if strings.Contains(blob, protected) {
			t.Fatalf("protected detail %q leaked into the serialized public object", protected)
		}
	}
}

// AM-6.3 shape: the sibling's audited projection stays intact in a
// mixed batch — the create refusal and marker are unchanged by the
// new slice's presence.
func TestT26AlterSiblingUnchanged(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u'); " + t26AlterSQL,
		Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	create := result.Statements[0]
	if create.Findings[0].Metadata["outcome"] != "refused_present" || create.Findings[0].Level != rule.LevelBlocker {
		t.Fatalf("catalog-present create = %+v, want the accepted refusal", create.Findings[0])
	}
	alter := result.Statements[1]
	if alter.Findings[0].Metadata["outcome"] != "conditional_alter" {
		t.Fatalf("alter after refused create = %+v, want conditional_alter on proven presence", alter.Findings[0].Metadata)
	}
}

// A loader failure propagates the bounded observation error — the
// audited ALTER triggers the same one observation the family already
// shared; the sentinel stays reachable through errors.Is (AM-9.x
// bounded seam).
func TestT26AlterLoaderErrorBoundary(t *testing.T) {
	loaderErr := &t26LoaderError{msg: "t26-alter-loader-sentinel-9"}
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilegeErr: loaderErr}
	_, err := AuditSQL(context.Background(), Request{
		SQL: t26AlterSQL, Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err == nil {
		t.Fatalf("loader failure must propagate")
	}
	if !errors.Is(err, loaderErr) {
		t.Fatalf("loader sentinel must stay reachable: %v", err)
	}
	if strings.Contains(err.Error(), "s1") || strings.Contains(err.Error(), "ALTER SERVER") {
		t.Fatalf("public error text echoes authored payload: %v", err)
	}
}

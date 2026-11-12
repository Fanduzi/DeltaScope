// T26-SERVER-CREATE-EXECUTION-01 application-layer verification:
// version gate ordering before the family loader, the independent
// CREATE premise projection, the shared adjudication arms end to end,
// OrderedPresent production/consumption, contamination semantics, and
// the bounded public output shape.
package audit

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	apppolicy "github.com/Fanduzi/DeltaScope/internal/application/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t26CreateSQL = "CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')"

// The G2 version gate fires before the family loader: an unknown or
// out-of-domain resolved version marks the qualified CREATE unaudited
// with gate:"version" — the loader never runs and no adjudication is
// attached. A proven 8.0.x banner leaves the statement audited.
func TestT26CreateVersionGate(t *testing.T) {
	for _, tc := range []struct {
		name        string
		version     string
		wantGate    bool
		wantPrivCal int
	}{
		{"unknown version gates", "", true, 0},
		{"out-of-domain 8.4 gates", "8.4.10", true, 0},
		{"5.7 gates", "5.7.44", true, 0},
		{"proven 8.0.x audits", "8.0.36", false, 1},
	} {
		provider := &t26Provider{servers: map[string]bool{}, version: tc.version, privilege: superObservation()}
		enriched := enrichT26Premise(t, t26CreateSQL, provider, spec.PrivilegeAuditModeObservedAccountNewSession)
		stmt := enriched[0]
		if tc.wantGate {
			if stmt.Unsupported == nil || stmt.Unsupported.Reason != spec.UnsupportedUnauditedReason {
				t.Fatalf("%s: must be unaudited: %+v", tc.name, stmt.Unsupported)
			}
			if stmt.Unsupported.Metadata["gate"] != "version" {
				t.Fatalf("%s: gate marker missing: %+v", tc.name, stmt.Unsupported.Metadata)
			}
			if stmt.Metadata != nil && stmt.Metadata.ServerCreate != nil {
				t.Fatalf("%s: gated statement must carry no adjudication", tc.name)
			}
		} else {
			if stmt.Unsupported != nil {
				t.Fatalf("%s: proven 8.0.x must audit: %+v", tc.name, stmt.Unsupported)
			}
			if stmt.Metadata == nil || stmt.Metadata.ServerCreate == nil {
				t.Fatalf("%s: audited create must carry the adjudication", tc.name)
			}
		}
		if provider.privilegeCalls != tc.wantPrivCal {
			t.Fatalf("%s: loader calls = %d, want %d", tc.name, provider.privilegeCalls, tc.wantPrivCal)
		}
	}
}

// The premise is independent: a consumable witness discharges only
// server.create_privilege on the CREATE, and a scope terminator
// suppresses the CREATE's own projection — never a borrowed DROP
// premise.
func TestT26CreatePrivilegeProjection(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t, t26CreateSQL, provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	adj := enriched[0].Metadata.ServerCreate
	if adj == nil || adj.Privilege != spec.ServerCreatePrivilegeConsumable {
		t.Fatalf("privilege = %+v, want consumable", adj)
	}
	for _, f := range adj.Owed {
		if f == spec.ServerFactDropPrivilege {
			t.Fatalf("a drop premise must never be owed on a create: %v", adj.Owed)
		}
	}

	provider = &t26Provider{servers: map[string]bool{}, version: "8.0.36", privilege: superObservation()}
	enriched = enrichT26Premise(t, "REVOKE SELECT ON *.* FROM 'u'@'%'; "+t26CreateSQL, provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	adj = enriched[1].Metadata.ServerCreate
	if adj == nil || adj.Privilege != spec.ServerCreatePrivilegeSuppressed {
		t.Fatalf("premise = %+v, want suppressed after the terminator", adj)
	}
	set := map[string]bool{}
	for _, f := range adj.Owed {
		set[f] = true
	}
	if !set[spec.ServerFactCreatePrivilege] {
		t.Fatalf("suppressed create must owe server.create_privilege: %v", adj.Owed)
	}
}

// Catalog-proven presence is the terminal refusal — blocker finding,
// no owed facts, verdict reject — and the privilege/options premises
// never reach the question.
func TestT26CreateRefusalPublicShape(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, err := AuditSQL(context.Background(), Request{
		SQL: t26CreateSQL, Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("proven refusal must reject: %+v", result.Verdict)
	}
	finding := result.Statements[0].Findings[0]
	if finding.Level != rule.LevelBlocker || finding.Metadata["outcome"] != "refused_present" {
		t.Fatalf("refusal finding = %+v, want blocker/refused_present", finding)
	}
	if len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("refusal owes nothing: %+v", result.Statements[0].EvidenceGaps)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v, want exactly one catalog read", provider.calls)
	}
	blob := finding.Message + finding.Explanation.Why + finding.Explanation.Risk + finding.Explanation.Suggestion
	for key, value := range finding.Metadata {
		blob += key
		if s, ok := value.(string); ok {
			blob += s
		}
	}
	for _, protected := range []string{"audited@%", "GRANT SELECT", "SHOW GRANTS", "s1", "OPTIONS", "HOST 'h'"} {
		if strings.Contains(blob, protected) {
			t.Fatalf("protected detail %q leaked into the finding: %q", protected, blob)
		}
	}
}

// A clean zero-row catalog read stays unknown — never an inferred
// absence: consumable + clean owes exactly server.existence.
func TestT26CreateZeroRowStaysUnknown(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{}, version: "8.0.36", privilege: superObservation()}
	result, err := AuditSQL(context.Background(), Request{
		SQL: t26CreateSQL, Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	facts := result.Statements[0].EvidenceGaps[0].RequiredFacts
	if len(facts) != 1 || facts[0] != spec.ServerFactExistence {
		t.Fatalf("owed = %v, want exactly {server.existence}", facts)
	}
	if result.Statements[0].Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage = %v, want unverified", result.Statements[0].Coverage.Status)
	}
}

// OrderedAbsent + consumable + clean is the proven create: it owes
// nothing, publishes OrderedPresent, and a later same-identity DROP
// consumes the present as proven presence — the resolver never runs
// twice for the identity.
func TestT26CreateOrderedPresentLifecycle(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t, "DROP SERVER s1; "+t26CreateSQL+"; DROP SERVER s1", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	drop := enriched[0].Metadata.ServerDrop
	if drop == nil || drop.Arm != spec.ServerDropArmProven || !drop.PublishAbsent {
		t.Fatalf("first drop = %+v, want proven publish", drop)
	}
	create := enriched[1].Metadata.ServerCreate
	if create == nil || create.Arm != spec.ServerCreateArmProven || !create.PublishPresent || len(create.Owed) != 0 {
		t.Fatalf("create = %+v, want proven publish owing nothing", create)
	}
	state := enriched[1].Metadata.Server
	if state == nil || !state.OrderedAbsent {
		t.Fatalf("create must have consumed OrderedAbsent: %+v", state)
	}
	redrop := enriched[2].Metadata.Server
	if redrop == nil || !redrop.OrderedPresent {
		t.Fatalf("re-drop must consume OrderedPresent: %+v", redrop)
	}
	redropAdj := enriched[2].Metadata.ServerDrop
	if redropAdj == nil || redropAdj.Arm != spec.ServerDropArmProven || !redropAdj.PublishAbsent {
		t.Fatalf("re-drop on proven present = %+v, want proven publish", redropAdj)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v, want exactly one catalog read for the identity", provider.calls)
	}
	if provider.privilegeCalls != 1 {
		t.Fatalf("loader calls = %d, want exactly one family read", provider.privilegeCalls)
	}
}

// OrderedAbsent alone never discharges privilege or options: an
// unclean OPTIONS list owes server.create_options, and the statement's
// own premise stays owed when unproven.
func TestT26CreateUncleanOptionsOwed(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t,
		"DROP SERVER s1; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', PORT 10000)",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	adj := enriched[1].Metadata.ServerCreate
	if adj == nil || adj.Arm != spec.ServerCreateArmUnproven || adj.PublishPresent {
		t.Fatalf("unclean create = %+v, want unproven without publish", adj)
	}
	set := map[string]bool{}
	for _, f := range adj.Owed {
		set[f] = true
	}
	if !set[spec.ServerFactCreateOptions] || set[spec.ServerFactExistence] || set[spec.ServerFactCreatePrivilege] {
		t.Fatalf("consumable + ordered absent + unclean owes exactly options: %v", adj.Owed)
	}
	// The unproven create is an unknown effect — a later same-identity
	// statement hits the barrier, never a stale answer.
	provider = &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched = enrichT26Premise(t,
		"DROP SERVER s1; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', PORT 10000); DROP SERVER s1",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	redrop := enriched[2].Metadata.ServerDrop
	if redrop == nil || redrop.Arm != spec.ServerDropArmBarrier {
		t.Fatalf("re-drop after unproven create = %+v, want barrier", redrop)
	}
}

// OrderedPresent refuse-before-read: a re-create over the proven
// present is the terminal refusal — no second resolver call, no owed
// facts, and the published present survives the refusal.
func TestT26CreateOrderedPresentRefusal(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t,
		"DROP SERVER s1; "+t26CreateSQL+"; "+t26CreateSQL+"; DROP SERVER s1",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	second := enriched[2].Metadata.ServerCreate
	if second == nil || second.Arm != spec.ServerCreateArmTerminalRefusal || len(second.Owed) != 0 {
		t.Fatalf("re-create = %+v, want refusal owing nothing", second)
	}
	if !second.ConditionalSource {
		t.Fatalf("refusal on conditional present must keep the marker: %+v", second)
	}
	// The refusal never mutates the channel — the trailing drop still
	// consumes the proven present.
	last := enriched[3].Metadata.ServerDrop
	if last == nil || last.Arm != spec.ServerDropArmProven {
		t.Fatalf("drop after refused re-create = %+v, want proven on standing present", last)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v, want exactly one catalog read", provider.calls)
	}
}

// Missing documented connection members emit exactly the audit-model
// completeness notice — never a blocker, never an owed fact.
func TestT26CreateConnectionMembersNotice(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "DROP SERVER s1; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h')",
		Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	findings := result.Statements[1].Findings
	if len(findings) != 2 {
		t.Fatalf("proven create + completeness notice = 2 findings, got %+v", findings)
	}
	if findings[0].Metadata["outcome"] != "conditional_create" || findings[0].Level != rule.LevelNotice {
		t.Fatalf("arm finding = %+v, want notice/conditional_create", findings[0])
	}
	if findings[1].Metadata["notice"] != "connection_members" || findings[1].Level != rule.LevelNotice {
		t.Fatalf("completeness notice = %+v", findings[1])
	}
	for _, protected := range []string{"'h'", "HOST 'h'"} {
		if strings.Contains(findings[1].Message, protected) {
			t.Fatalf("options content leaked into the notice: %q", findings[1].Message)
		}
	}
	// A complete member set emits no completeness notice.
	provider = &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, err = AuditSQL(context.Background(), Request{
		SQL: "DROP SERVER s1; " + t26CreateSQL, Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements[1].Findings) != 1 {
		t.Fatalf("complete members = 1 finding, got %+v", result.Statements[1].Findings)
	}
}

// SERVER-channel taint — a literal-name drop records an unprovable
// write on the channel: the later create adjudicates the same A1
// barrier and owes exactly batch_effects. This exercises the channel
// taint, NOT batchState.contaminated — whole-batch contamination is
// covered by TestT26CreateRealContamination below (R2-D).
func TestT26CreateChannelTaintBarrier(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t, "DROP SERVER 'x'; "+t26CreateSQL, provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	adj := enriched[1].Metadata.ServerCreate
	if adj == nil || adj.Arm != spec.ServerCreateArmBarrier || len(adj.Owed) != 1 || adj.Owed[0] != spec.ServerFactBatchEffects {
		t.Fatalf("tainted-channel create = %+v, want barrier owing batch_effects", adj)
	}
}

// serverEntrySnapshot is an independent deep copy of one server
// ordered-channel entry — every field, never a shared pointer, so an
// in-place rewrite is detected rather than masked by map aliasing.
type serverEntrySnapshot struct {
	name               string
	barrier            bool
	absent             bool
	publishedArm       spec.ServerDropArm
	present            bool
	presentConditional bool
}

func snapshotServerEntries(s *batchState) map[string]serverEntrySnapshot {
	out := make(map[string]serverEntrySnapshot, len(s.serverEntries))
	for key, e := range s.serverEntries {
		out[key] = serverEntrySnapshot{
			name: e.name, barrier: e.barrier, absent: e.absent,
			publishedArm: e.publishedArm, present: e.present,
			presentConditional: e.presentConditional,
		}
	}
	return out
}

// Real whole-batch contamination: a parse failure earlier in the batch
// sets batchState.contaminated — never a forged UnreadReason and never
// a test-set flag. The batch is processed through the real
// enrichStatements entry point twice on one state: the prefix publishes
// the ordered entries, then the tail slice carries the real parse
// failure record so the second call's own loop sets the flag — the
// call boundary is the snapshot seam around the contaminated CREATE's
// bind+apply. The tail call runs with the privilege mode off so the
// request-level loader count for that slice is independently zero;
// the object-resolver delta is zero too. The complete before/after
// map must match: no new entry, no clear, no rewrite, every field
// byte-identical (R2-D.1, R3-B).
func TestT26CreateRealContamination(t *testing.T) {
	enrich := func(t *testing.T, sql string, tail int) (*batchState, []spec.Statement, *t26Provider) {
		t.Helper()
		provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
		parsed, parseErr := parseSQL(context.Background(), sql, spec.DialectMySQL)
		if len(parsed.failures) != 1 {
			t.Fatalf("parse: failures=%d err=%v, want exactly 1", len(parsed.failures), parseErr)
		}
		statements, err := Extract(context.Background(), parsed)
		if err != nil {
			t.Fatalf("extract: %v", err)
		}
		if tail <= 0 || tail >= len(statements) {
			t.Fatalf("tail index %d out of range for %d statements", tail, len(statements))
		}
		state := newBatchState(spec.DialectMySQL, "golden", provider)
		state.resolvedVersion = &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.36", Major: 8, Minor: 0, Patch: 36, ValidatedRange: true}
		state.instanceFacts = &spec.InstanceFacts{Version: "8.0.36"}
		// Prefix call: no failures among these statements — the ordered
		// entries publish normally.
		enriched, err := state.enrichStatements(context.Background(),
			&MetadataRequest{Schema: "golden", Provider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession},
			statements[:tail], nil, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
		if err != nil {
			t.Fatalf("prefix enrich: %v", err)
		}
		if state.contaminated {
			t.Fatalf("flag set before the failure was processed — contamination must come from the real parse record, not a preset")
		}
		before := snapshotServerEntries(state)
		resolverBefore, loaderBefore := len(provider.calls), provider.privilegeCalls
		// Tail call: the real parse failure record precedes the tail
		// statement, so this call's own loop sets contaminated — the
		// flag asserted below is the product's, not the test's. The
		// mode-off request keeps this call's loader gate from firing,
		// so both read layers report their own independent zero.
		enriched, err = state.enrichStatements(context.Background(),
			&MetadataRequest{Schema: "golden", Provider: provider},
			statements[tail:], parsed.failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
		if err != nil {
			t.Fatalf("tail enrich: %v", err)
		}
		if !state.contaminated {
			t.Fatalf("state.contaminated = false after the real parse failure — want the actual flag set")
		}
		after := snapshotServerEntries(state)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("serverEntries rewritten across the contaminated CREATE\nbefore=%+v\nafter=%+v", before, after)
		}
		if d := len(provider.calls) - resolverBefore; d != 0 {
			t.Fatalf("resolver delta across the contaminated CREATE = %d, want 0 — contaminated bind never enters the resolver", d)
		}
		if d := provider.privilegeCalls - loaderBefore; d != 0 {
			t.Fatalf("loader delta across the contaminated CREATE call = %d, want 0 — request-level loader recorded independently of resolver-0", d)
		}
		return state, enriched, provider
	}
	assertBarrier := func(enriched []spec.Statement, i int) {
		t.Helper()
		adj := enriched[i].Metadata.ServerCreate
		if adj == nil || adj.Arm != spec.ServerCreateArmBarrier {
			t.Fatalf("statement %d arm = %+v, want A1 barrier under whole-batch contamination", i, adj)
		}
		if len(adj.Owed) != 1 || adj.Owed[0] != spec.ServerFactBatchEffects {
			t.Fatalf("statement %d owed = %v, want exactly {server.batch_effects}", i, adj.Owed)
		}
		if adj.PublishPresent {
			t.Fatalf("statement %d must not publish under contamination", i)
		}
		// No conditional marker: the consumable premise and every
		// ordered source were shadowed, never consumed.
		if adj.ConditionalAudit || adj.ConditionalSource {
			t.Fatalf("statement %d conditional provenance = %+v, want none — shadowed premises never feed the marker", i, adj)
		}
		bound := enriched[i].Metadata.Server
		if bound == nil || bound.OrderedAbsent || bound.OrderedPresent || bound.Read {
			t.Fatalf("statement %d bound = %+v, want the unbound shape — no ordered answer consumed, no resolver entered", i, bound)
		}
	}

	// OrderedPresent written before the failure: DROP+CREATE publish
	// the conditional present, GARBAGE contaminates the batch, and the
	// trailing create reads the barrier — the entry survives the
	// contaminated bind+apply byte-identical (proved by the complete
	// snapshot comparison inside enrich).
	state, enriched, provider := enrich(t,
		"DROP SERVER s1; "+t26CreateSQL+"; GARBAGE TOKENS HERE; "+t26CreateSQL, 2)
	assertBarrier(enriched, 0)
	entry := state.serverEntries[serverFold("s1")]
	if entry == nil || !entry.present || !entry.presentConditional || entry.absent || entry.barrier {
		t.Fatalf("entry = %+v, want the conditional OrderedPresent standing — never cleared or rewritten", entry)
	}
	if len(provider.calls) != 1 || provider.privilegeCalls != 1 {
		t.Fatalf("reads = resolver:%+v loader:%d, want 1 catalog read + 1 loader call from the prefix", provider.calls, provider.privilegeCalls)
	}

	// OrderedAbsent written before the failure survives the same way.
	state, enriched, provider = enrich(t,
		"DROP SERVER s1; GARBAGE TOKENS HERE; "+t26CreateSQL, 1)
	assertBarrier(enriched, 0)
	entry = state.serverEntries[serverFold("s1")]
	if entry == nil || !entry.absent || entry.present || entry.barrier || entry.publishedArm != spec.ServerDropArmProven {
		t.Fatalf("entry = %+v, want the OrderedAbsent the proven drop published — never cleared or rewritten", entry)
	}
	if len(provider.calls) != 1 || provider.privilegeCalls != 1 {
		t.Fatalf("reads = resolver:%+v loader:%d, want 1 catalog read + 1 loader call from the prefix", provider.calls, provider.privilegeCalls)
	}
}

// An unsupported literal-name create mutates nothing proven — but the
// channel taint blocks every later consumption and publication.
func TestT26CreateLiteralTaint(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t,
		"DROP SERVER s1; CREATE SERVER 's 2' FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h'); DROP SERVER s1",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if enriched[1].Unsupported == nil {
		t.Fatalf("literal-name create stays boundary")
	}
	last := enriched[2].Metadata.ServerDrop
	if last == nil || last.Arm != spec.ServerDropArmBarrier {
		t.Fatalf("re-drop after literal create = %+v, want barrier — tainted channel blocks consumption", last)
	}
}

// The objectCache shares one catalog read across resolvers for the
// same key — a create refused on proven presence and a same-identity
// drop together issue exactly one provider-facing resolver call.
func TestT26CreateObjectCacheShare(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t, t26CreateSQL+"; DROP SERVER s1", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if adj := enriched[0].Metadata.ServerCreate; adj == nil || adj.Arm != spec.ServerCreateArmTerminalRefusal {
		t.Fatalf("create on proven presence = %+v, want refusal", adj)
	}
	if adj := enriched[1].Metadata.ServerDrop; adj == nil || adj.Arm != spec.ServerDropArmProven {
		t.Fatalf("drop on cached presence = %+v, want proven", adj)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v, want exactly one — objectCache shares the read", provider.calls)
	}
}

// Mode not selected keeps every premise owed — existence, the create
// privilege, and (when unclean) options all stay bounded gaps.
func TestT26CreateNoModeOwesAll(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{}, version: "8.0.36"}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (PORT 10000)", Dialect: spec.DialectMySQL,
		Schema: "golden", MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	facts := map[string]bool{}
	for _, f := range result.Statements[0].EvidenceGaps[0].RequiredFacts {
		facts[f] = true
	}
	for _, want := range []string{spec.ServerFactExistence, spec.ServerFactCreatePrivilege, spec.ServerFactCreateOptions} {
		if !facts[want] {
			t.Fatalf("owed = %v, missing %s", facts, want)
		}
	}
	if provider.privilegeCalls != 0 {
		t.Fatalf("no-mode must never run the loader: %d", provider.privilegeCalls)
	}
}

// evaluateT26Statements runs the real enrich → registry → evaluate
// seam: parse failures feed the batch's own contamination path, and
// the finding/coverage/verdict projection comes from the real rule
// registry — service-level unsupported gating is orthogonal to the
// finding contract under test.
func evaluateT26Statements(t *testing.T, sql string, provider MetadataProvider, mode spec.PrivilegeAuditMode) (report.Result, []spec.Statement) {
	t.Helper()
	parsed, parseErr := parseSQL(context.Background(), sql, spec.DialectMySQL)
	if parseErr != nil && len(parsed.failures) == 0 {
		t.Fatalf("parse %q: %v", sql, parseErr)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract %q: %v", sql, err)
	}
	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectMySQL,
		&MetadataRequest{Schema: "golden", Provider: provider, PrivilegeAuditMode: mode}, statements, parsed.failures)
	if err != nil {
		t.Fatalf("enrich %q: %v", sql, err)
	}
	cfg, err := apppolicy.Load("")
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	registry, err := buildRegistry(cfg)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	result, err := EvaluateStatements(context.Background(), registry, enriched)
	if err != nil {
		t.Fatalf("evaluate %q: %v", sql, err)
	}
	return result, enriched
}

// Inherited-only marker (R2-D.2): after a privilege-scope terminator
// suppresses the statement's own premise, the conditional marker that
// survives comes only from the consumed ordered source — it never
// substitutes for the statement's own privilege and never claims it
// was proven.
func TestT26CreateInheritedMarkerChains(t *testing.T) {
	provider := func() *t26Provider {
		return &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	}
	marked := func(stmt report.StatementResult) bool {
		for _, f := range stmt.Findings {
			if _, ok := f.Metadata["conditional_audit"]; ok {
				return true
			}
		}
		return false
	}

	// present → terminator → CREATE: A2 on the consumed conditional
	// OrderedPresent, owed empty, complete; the refusal leaves the
	// ordered present standing — the trailing drop still consumes it.
	result, enriched := evaluateT26Statements(t,
		"DROP SERVER s1; "+t26CreateSQL+"; REVOKE SELECT ON *.* FROM 'u'@'%'; "+
			t26CreateSQL+"; DROP SERVER s1",
		provider(), spec.PrivilegeAuditModeObservedAccountNewSession)
	adj := enriched[3].Metadata.ServerCreate
	if adj == nil || adj.Privilege != spec.ServerCreatePrivilegeSuppressed {
		t.Fatalf("post-terminator create premise = %+v, want suppressed", adj)
	}
	if adj.Arm != spec.ServerCreateArmTerminalRefusal || len(adj.Owed) != 0 || !adj.ConditionalSource || !adj.ConditionalAudit {
		t.Fatalf("present->create = %+v, want A2 with inherited-only marker", adj)
	}
	finding := result.Statements[3].Findings[0]
	if finding.Metadata["outcome"] != "refused_present" || !marked(result.Statements[3]) {
		t.Fatalf("public shape = %+v, want refused_present + marker", finding)
	}
	if strings.Contains(finding.Message, "privilege") {
		t.Fatalf("inherited-only refusal text must not claim the own premise: %q", finding.Message)
	}
	if len(result.Statements[3].EvidenceGaps) != 0 || result.Statements[3].Coverage.Status != report.CoverageComplete {
		t.Fatalf("refusal coverage = %+v gaps %+v, want complete/none", result.Statements[3].Coverage, result.Statements[3].EvidenceGaps)
	}
	// The refusal published nothing — the consumed OrderedPresent
	// stands for the trailing drop.
	last := enriched[4].Metadata.ServerDrop
	if last == nil || last.Arm != spec.ServerDropArmUnproven {
		t.Fatalf("drop after refused re-create = %+v, want unproven on the standing present (suppressed own premise)", last)
	}

	// present → terminator → DROP: the suppressed premise keeps the
	// arm unproven owing only server.drop_privilege with the inherited
	// marker; no OrderedAbsent publishes — the unknown effect
	// invalidates the present into a barrier for the next statement.
	result, enriched = evaluateT26Statements(t,
		"DROP SERVER s1; "+t26CreateSQL+"; REVOKE SELECT ON *.* FROM 'u'@'%'; DROP SERVER s1; DROP SERVER s1",
		provider(), spec.PrivilegeAuditModeObservedAccountNewSession)
	dropAdj := enriched[3].Metadata.ServerDrop
	if dropAdj == nil || dropAdj.Arm != spec.ServerDropArmUnproven || dropAdj.Privilege != spec.ServerDropPrivilegeSuppressed {
		t.Fatalf("present->drop = %+v, want unproven suppressed", dropAdj)
	}
	owed := map[string]bool{}
	for _, f := range dropAdj.Owed {
		owed[f] = true
	}
	if len(owed) != 1 || !owed[spec.ServerFactDropPrivilege] {
		t.Fatalf("present->drop owed = %v, want exactly {server.drop_privilege}", dropAdj.Owed)
	}
	if !dropAdj.ConditionalSource || !marked(result.Statements[3]) {
		t.Fatalf("present->drop must keep the inherited marker: adj=%+v", dropAdj)
	}
	if barrier := enriched[4].Metadata.ServerDrop; barrier == nil || barrier.Arm != spec.ServerDropArmBarrier {
		t.Fatalf("the unknown drop must invalidate the present into a barrier: %+v", barrier)
	}

	// absent → terminator → CREATE: the inherited marker never
	// substitutes for the own premise — clean options still owe
	// exactly server.create_privilege and nothing publishes.
	result, enriched = evaluateT26Statements(t,
		"DROP SERVER s1; REVOKE SELECT ON *.* FROM 'u'@'%'; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')",
		provider(), spec.PrivilegeAuditModeObservedAccountNewSession)
	adj = enriched[2].Metadata.ServerCreate
	if adj == nil || adj.Privilege != spec.ServerCreatePrivilegeSuppressed || adj.Arm != spec.ServerCreateArmUnproven {
		t.Fatalf("absent->create = %+v, want unproven suppressed", adj)
	}
	if len(adj.Owed) != 1 || adj.Owed[0] != spec.ServerFactCreatePrivilege || adj.PublishPresent {
		t.Fatalf("absent->create owed = %v publish=%v, want exactly {server.create_privilege}, no publish", adj.Owed, adj.PublishPresent)
	}
	if !adj.ConditionalSource || !adj.ConditionalAudit || !marked(result.Statements[2]) {
		t.Fatalf("absent->create must keep the inherited marker: %+v", adj)
	}
	finding = result.Statements[2].Findings[0]
	if !strings.Contains(finding.Message, "execution premise") || strings.Contains(finding.Message, "existence") {
		t.Fatalf("owed-accurate message = %q, want only the execution premise claimed unverified", finding.Message)
	}
}

// Ordered lifecycle to the public projection (R2-D.3): findings,
// gaps, markers, two-level coverage, and the verdict across the
// publish → consume → invalidate chains.
func TestT26CreateOrderedLifecyclePublic(t *testing.T) {
	provider := func() *t26Provider {
		return &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	}

	// DROP→CREATE→DROP: conditional publish, conditional consume.
	result, _ := evaluateT26Statements(t,
		"DROP SERVER s1; "+t26CreateSQL+"; DROP SERVER s1",
		provider(), spec.PrivilegeAuditModeObservedAccountNewSession)
	for i, want := range []string{"conditional_drop", "conditional_create", "conditional_drop"} {
		stmt := result.Statements[i]
		if stmt.Findings[0].Metadata["outcome"] != want {
			t.Fatalf("chain A statement %d outcome = %v, want %s", i, stmt.Findings[0].Metadata["outcome"], want)
		}
		if _, ok := stmt.Findings[0].Metadata["conditional_audit"]; !ok {
			t.Fatalf("chain A statement %d marker missing: %+v", i, stmt.Findings[0].Metadata)
		}
		if len(stmt.EvidenceGaps) != 0 || stmt.Coverage.Status != report.CoverageComplete {
			t.Fatalf("chain A statement %d coverage = %v gaps = %+v, want complete/none", i, stmt.Coverage.Status, stmt.EvidenceGaps)
		}
	}
	if result.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
		t.Fatalf("chain A aggregate = %v verdict %v, want complete/pass", result.Coverage.Status, result.Verdict)
	}

	// DROP→CREATE→CREATE→DROP: the re-create refuses on the standing
	// OrderedPresent — blocker verdict, nothing owed, refusal leaves
	// the present for the trailing drop.
	result, _ = evaluateT26Statements(t,
		"DROP SERVER s1; "+t26CreateSQL+"; "+t26CreateSQL+"; DROP SERVER s1",
		provider(), spec.PrivilegeAuditModeObservedAccountNewSession)
	for i, want := range []string{"conditional_drop", "conditional_create", "refused_present", "conditional_drop"} {
		stmt := result.Statements[i]
		if stmt.Findings[0].Metadata["outcome"] != want {
			t.Fatalf("chain B statement %d outcome = %v, want %s", i, stmt.Findings[0].Metadata["outcome"], want)
		}
		if _, ok := stmt.Findings[0].Metadata["conditional_audit"]; !ok {
			t.Fatalf("chain B statement %d marker missing: %+v — every arm consumed a conditional premise or a conditional ordered state", i, stmt.Findings[0].Metadata)
		}
		if len(stmt.EvidenceGaps) != 0 || stmt.Coverage.Status != report.CoverageComplete {
			t.Fatalf("chain B statement %d coverage = %v gaps = %+v, want complete/none", i, stmt.Coverage.Status, stmt.EvidenceGaps)
		}
	}
	if result.Statements[2].Findings[0].Level != rule.LevelBlocker || result.Verdict != report.VerdictReject {
		t.Fatalf("chain B verdict = %v, want reject on the refusal blocker", result.Verdict)
	}
	if result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("chain B aggregate coverage = %v, want complete", result.Coverage.Status)
	}

	// unknown CREATE→CREATE: the first create's own consumable premise
	// marks it while existence stays owed; the second reads the
	// barrier the first's unknown effect left — no marker on A1. The
	// catalog holds no s9, so the zero-row read stays unknown.
	result, _ = evaluateT26Statements(t,
		"CREATE SERVER s9 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u'); "+
			"CREATE SERVER s9 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')",
		provider(), spec.PrivilegeAuditModeObservedAccountNewSession)
	first := result.Statements[0]
	if _, ok := first.Findings[0].Metadata["conditional_audit"]; !ok {
		t.Fatalf("chain C first create must carry the own-premise marker: %+v", first.Findings[0].Metadata)
	}
	if facts := first.EvidenceGaps[0].RequiredFacts; len(facts) != 1 || facts[0] != spec.ServerFactExistence {
		t.Fatalf("chain C first owed = %v, want {server.existence} (zero-row catalog stays unknown)", facts)
	}
	second := result.Statements[1]
	if second.Findings[0].Metadata["outcome"] != "blocked_batch_effects" {
		t.Fatalf("chain C second outcome = %v, want blocked_batch_effects", second.Findings[0].Metadata["outcome"])
	}
	if _, ok := second.Findings[0].Metadata["conditional_audit"]; ok {
		t.Fatalf("chain C barrier must not carry conditional_audit: %+v", second.Findings[0].Metadata)
	}
	if facts := second.EvidenceGaps[0].RequiredFacts; len(facts) != 1 || facts[0] != spec.ServerFactBatchEffects {
		t.Fatalf("chain C barrier owed = %v, want {server.batch_effects} only", facts)
	}
	if first.Coverage.Status != report.CoverageUnverified || second.Coverage.Status != report.CoverageUnverified || result.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("chain C coverage = %v/%v/%v, want unverified throughout", first.Coverage.Status, second.Coverage.Status, result.Coverage.Status)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("chain C verdict = %v, want review", result.Verdict)
	}

	// absent → dirty CREATE → DROP: the create owes only options
	// (existence proven, premise consumable); its unknown effect
	// invalidates the absent into the barrier the re-drop reads.
	result, enriched := evaluateT26Statements(t,
		"DROP SERVER s1; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', PORT 10000); DROP SERVER s1",
		provider(), spec.PrivilegeAuditModeObservedAccountNewSession)
	// First drop: the full public projection — conditional outcome, no
	// owed facts, the conditional marker, complete coverage.
	firstDrop := result.Statements[0]
	if firstDrop.Findings[0].Metadata["outcome"] != "conditional_drop" {
		t.Fatalf("dirty chain drop outcome = %v, want conditional_drop", firstDrop.Findings[0].Metadata["outcome"])
	}
	if len(firstDrop.EvidenceGaps) != 0 {
		t.Fatalf("dirty chain drop gaps = %+v, want none — existence and premise both discharged", firstDrop.EvidenceGaps)
	}
	if _, ok := firstDrop.Findings[0].Metadata["conditional_audit"]; !ok {
		t.Fatalf("dirty chain drop marker missing: %+v", firstDrop.Findings[0].Metadata)
	}
	if firstDrop.Coverage.Status != report.CoverageComplete {
		t.Fatalf("dirty chain drop coverage = %v, want complete", firstDrop.Coverage.Status)
	}
	// Dirty create: A4 unproven on the ordered absent — owes exactly
	// {server.create_options}, nothing else; the consumable premise
	// marks it; the options-only owed-set wording is the fixed bounded
	// phrase; the connection-members advisory is a separate notice
	// finding that adds no gap.
	createAdj := enriched[1].Metadata.ServerCreate
	if createAdj.Arm != spec.ServerCreateArmUnproven || len(createAdj.Owed) != 1 || createAdj.Owed[0] != spec.ServerFactCreateOptions {
		t.Fatalf("dirty create = %+v, want unproven owing exactly options", createAdj)
	}
	dirtyCreate := result.Statements[1]
	if dirtyCreate.Findings[0].Metadata["outcome"] != "unverified" {
		t.Fatalf("dirty create outcome = %v, want unverified", dirtyCreate.Findings[0].Metadata["outcome"])
	}
	if len(dirtyCreate.EvidenceGaps) != 1 {
		t.Fatalf("dirty create gaps = %+v, want the single owed-fact gap", dirtyCreate.EvidenceGaps)
	}
	if facts := dirtyCreate.EvidenceGaps[0].RequiredFacts; len(facts) != 1 || facts[0] != spec.ServerFactCreateOptions {
		t.Fatalf("dirty create owed = %v, want exactly {server.create_options} — no existence, no create_privilege", facts)
	}
	if _, ok := dirtyCreate.Findings[0].Metadata["conditional_audit"]; !ok {
		t.Fatalf("dirty create marker missing: %+v — the consumable premise fed the conclusion", dirtyCreate.Findings[0].Metadata)
	}
	if dirtyCreate.Findings[0].Message != "CREATE SERVER targets a server definition whose options remains unverified under the observed_account_new_session conditional audit" {
		t.Fatalf("dirty create message = %q, want the options-only owed-set wording", dirtyCreate.Findings[0].Message)
	}
	if dirtyCreate.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("dirty create coverage = %v, want unverified", dirtyCreate.Coverage.Status)
	}
	if len(dirtyCreate.Findings) != 2 || dirtyCreate.Findings[1].Metadata["notice"] != "connection_members" || dirtyCreate.Findings[1].Level != rule.LevelNotice {
		t.Fatalf("dirty create findings = %+v, want primary + a separate connection_members advisory notice", dirtyCreate.Findings)
	}
	// Tail drop: the invalidated absent leaves the barrier arm — its
	// consumable premise still marks it under the accepted DROP
	// contract; only the CREATE projection was corrected (R2-A).
	redrop := result.Statements[2]
	if redrop.Findings[0].Metadata["outcome"] != "blocked_batch_effects" {
		t.Fatalf("re-drop after unknown create = %v, want barrier — the absent was invalidated", redrop.Findings[0].Metadata["outcome"])
	}
	if len(redrop.EvidenceGaps) != 1 {
		t.Fatalf("re-drop gaps = %+v, want the single barrier gap", redrop.EvidenceGaps)
	}
	if facts := redrop.EvidenceGaps[0].RequiredFacts; len(facts) != 1 || facts[0] != spec.ServerFactBatchEffects {
		t.Fatalf("re-drop owed = %v, want exactly {server.batch_effects}", facts)
	}
	if _, ok := redrop.Findings[0].Metadata["conditional_audit"]; !ok {
		t.Fatalf("re-drop marker missing: %+v — the accepted DROP contract marks a barrier whose premise was consumable", redrop.Findings[0].Metadata)
	}
	if redrop.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("re-drop coverage = %v, want unverified", redrop.Coverage.Status)
	}
	if result.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
		t.Fatalf("dirty chain aggregate = %v verdict %v, want unverified/review", result.Coverage.Status, result.Verdict)
	}

	// OrderedPresent side of the literal taint: the provable create
	// publishes present; the literal-name create taints the channel —
	// the trailing drop reads the barrier, never a stale present.
	_, enriched = evaluateT26Statements(t,
		"DROP SERVER s1; "+t26CreateSQL+"; CREATE SERVER 's 9' FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h'); DROP SERVER s1",
		provider(), spec.PrivilegeAuditModeObservedAccountNewSession)
	if adj := enriched[3].Metadata.ServerDrop; adj == nil || adj.Arm != spec.ServerDropArmBarrier {
		t.Fatalf("drop after literal taint on ordered present = %+v, want barrier", adj)
	}

	// OrderedPresent side of the relation scan: s1x is
	// provably-distinct from s1 (S7), so its unproven write neither
	// shadows s1x's own read nor invalidates s1's published present —
	// the trailing same-identity drop consumes the ordered present
	// and lands the proven arm (T26-SERVER-ORDERED-ABSENCE-01 S6).
	_, enriched = evaluateT26Statements(t,
		"DROP SERVER s1; "+t26CreateSQL+"; DROP SERVER s1x; DROP SERVER s1",
		provider(), spec.PrivilegeAuditModeObservedAccountNewSession)
	if adj := enriched[2].Metadata.ServerDrop; adj == nil || adj.Arm != spec.ServerDropArmUnproven || len(adj.Owed) != 1 || adj.Owed[0] != spec.ServerFactExistence {
		t.Fatalf("s1x drop = %+v, want unproven owing existence — distinct name reads its own catalog", adj)
	}
	if adj := enriched[3].Metadata.ServerDrop; adj == nil || adj.Arm != spec.ServerDropArmProven || !adj.PublishAbsent {
		t.Fatalf("drop after distinct write on ordered present = %+v, want proven — the present was preserved", adj)
	}
}

// Default entry without a provider (R2-E): a valid 8.0.x target pin
// admits the create to the audited slice — the unbound catalog fact
// and the unobserved premise are the only two gaps, and the loader
// never runs under no-mode. Without a version the same statement
// keeps the gate:"version" boundary, and the typed version errors
// (malformed pin, pin/observed mismatch) exit before the family
// loader with the error type preserved.
func TestT26CreateDefaultEntryAndVersionGate(t *testing.T) {
	// Provider nil + valid 8.0.x pin + no mode + clean options.
	result, err := AuditSQL(context.Background(), Request{
		SQL: t26CreateSQL, Dialect: spec.DialectMySQL, Schema: "golden", TargetVersion: "8.0.36",
	})
	if err != nil {
		t.Fatalf("pinned create audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.EvidenceGaps) != 1 {
		t.Fatalf("gaps = %+v, want the single owed-fact gap", stmt.EvidenceGaps)
	}
	facts := map[string]bool{}
	for _, f := range stmt.EvidenceGaps[0].RequiredFacts {
		facts[f] = true
	}
	if len(facts) != 2 || !facts[spec.ServerFactExistence] || !facts[spec.ServerFactCreatePrivilege] {
		t.Fatalf("owed = %v, want exactly {server.existence, server.create_privilege}", stmt.EvidenceGaps[0].RequiredFacts)
	}
	if stmt.Coverage.Status != report.CoverageUnverified || result.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage = %v/%v, want unverified at both levels", stmt.Coverage.Status, result.Coverage.Status)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("verdict = %v, want review under owed premises", result.Verdict)
	}

	// No provider and no version: the G2 gate keeps the boundary —
	// the statement is unaudited with gate:"version" (the service
	// surfaces it as the unsupported-statement error) and carries no
	// adjudication.
	_, err = AuditSQL(context.Background(), Request{
		SQL: t26CreateSQL, Dialect: spec.DialectMySQL, Schema: "golden",
	})
	if err == nil || !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("unpinned create = %v, want ErrUnsupportedStatement", err)
	}
	enriched := enrichT26Premise(t, t26CreateSQL, &t26Provider{servers: map[string]bool{}, version: ""}, "")
	stmt0 := enriched[0]
	if stmt0.Unsupported == nil || stmt0.Unsupported.Metadata["gate"] != "version" {
		t.Fatalf("unsupported = %+v, want gate:version boundary", stmt0.Unsupported)
	}
	if stmt0.Metadata != nil && stmt0.Metadata.ServerCreate != nil {
		t.Fatalf("gated create must carry no adjudication: %+v", stmt0.Metadata.ServerCreate)
	}

	// Malformed pin exits as the typed version error before the
	// family loader — provider untouched.
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	_, err = AuditSQL(context.Background(), Request{
		SQL: t26CreateSQL, Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
		TargetVersion: "eight-dot-zero",
	})
	if err == nil || !errors.Is(err, ErrInvalidTargetVersion) {
		t.Fatalf("malformed pin = %v, want ErrInvalidTargetVersion", err)
	}
	if provider.privilegeCalls != 0 {
		t.Fatalf("typed version error triggered %d loader calls", provider.privilegeCalls)
	}
	// A pin that conflicts with the observed product exits typed —
	// again before the family loader.
	provider = &t26Provider{servers: map[string]bool{"s1": true}, version: "8.4.10", privilege: superObservation()}
	_, err = AuditSQL(context.Background(), Request{
		SQL: t26CreateSQL, Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
		TargetVersion: "8.0.36",
	})
	if err == nil || !errors.Is(err, ErrTargetVersionMismatch) {
		t.Fatalf("pin/observed mismatch = %v, want ErrTargetVersionMismatch", err)
	}
	if provider.privilegeCalls != 0 {
		t.Fatalf("version mismatch triggered %d loader calls", provider.privilegeCalls)
	}
}

// CREATE-only read boundaries (R2-E): a failed family observation and
// a failed catalog read both propagate as bounded public errors —
// errors.Is/errors.As reach the typed cause, no authored name or
// grant text rides the public message, and nothing is laundered into
// an unproven grant or an evidence gap.
func TestT26CreateReadErrorBoundaries(t *testing.T) {
	marker := errors.New("t26-create-priv-marker-3k9")
	provider := &t26Provider{servers: map[string]bool{}, version: "8.0.36", privilegeErr: marker}
	_, err := AuditSQL(context.Background(), Request{
		SQL: t26CreateSQL, Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err == nil || !errors.Is(err, marker) {
		t.Fatalf("create loader failure = %v, want errors.Is(marker)", err)
	}
	for _, protected := range []string{marker.Error(), "s1", "audited", "SHOW GRANTS"} {
		if strings.Contains(err.Error(), protected) {
			t.Fatalf("public error text echoes protected content %q: %v", protected, err)
		}
	}
	// A failed catalog read propagates the same bounded boundary —
	// isolated to CREATE's own resolver (R3-C): a single qualified
	// CREATE request, the family loader succeeds once, the error is
	// injected at the object resolver the create itself reaches.
	readMarker := errors.New("t26-create-read-marker-7w2")
	provider = &t26Provider{servers: map[string]bool{}, version: "8.0.36", privilege: superObservation(), err: readMarker}
	_, err = AuditSQL(context.Background(), Request{
		SQL: t26CreateSQL, Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err == nil || !errors.Is(err, readMarker) {
		t.Fatalf("create catalog read failure = %v, want errors.Is(readMarker)", err)
	}
	if provider.privilegeCalls != 1 {
		t.Fatalf("loader calls = %d, want 1 — the family observation succeeded before the resolver was reached", provider.privilegeCalls)
	}
	if len(provider.calls) != 1 || provider.calls[0].typ != "mysql_server" || provider.calls[0].name != "s1" {
		t.Fatalf("resolver calls = %+v, want exactly one mysql_server lookup for the authored identity — proving the create's own read failed", provider.calls)
	}
	for _, protected := range []string{readMarker.Error(), "s1"} {
		if strings.Contains(err.Error(), protected) {
			t.Fatalf("public error text echoes protected content %q: %v", protected, err)
		}
	}
}

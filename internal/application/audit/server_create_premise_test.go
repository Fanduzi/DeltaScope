// T26-SERVER-CREATE-EXECUTION-01 application-layer verification:
// version gate ordering before the family loader, the independent
// CREATE premise projection, the shared adjudication arms end to end,
// OrderedPresent production/consumption, contamination semantics, and
// the bounded public output shape.
package audit

import (
	"context"
	"strings"
	"testing"

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

// Whole-batch contamination reaches the arm table directly: the
// create adjudicates barrier and owes exactly batch_effects, while
// channel entries are never rewritten on the contaminated path.
func TestT26CreateContaminationBarrier(t *testing.T) {
	// A literal-name drop taints the whole channel before the create
	// binds — A1 wins over every other input.
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t, "DROP SERVER 'x'; "+t26CreateSQL, provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	adj := enriched[1].Metadata.ServerCreate
	if adj == nil || adj.Arm != spec.ServerCreateArmBarrier || len(adj.Owed) != 1 || adj.Owed[0] != spec.ServerFactBatchEffects {
		t.Fatalf("contaminated create = %+v, want barrier owing batch_effects", adj)
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

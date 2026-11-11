// T26-SERVER-DROP-01: the SERVER ordered channel — audited identifier
// drops leave an unproven barrier, unaudited writes leave records or
// channel taint, and every barrier decision completes before the
// provider seam. Contract: docs/verification/t26-semantics-01.
package audit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// An ASCII case alias consumes the same barrier — R("S1","s1") is
// provably-same under the registered fold, so the second drop binds
// batch_effects and the resolver is never asked again.
func TestT26DropServerCaseAliasConsumesBarrier(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}}
	enriched := enrichT26(t, "DROP SERVER s1; DROP SERVER `S1`", spec.DialectMySQL, provider)
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v, want the alias barrier to suppress the second lookup", provider.calls)
	}
	second := enriched[1].Metadata.Server
	if second == nil || second.Name != "S1" || second.UnreadReason != spec.MetadataUnreadBatchEffects {
		t.Fatalf("alias drop must bind batch_effects with the verbatim name: %+v", second)
	}
}

// An unreviewed same-identity write — an unaudited CREATE SERVER — is a
// write record, never a consumable fact: a later drop on the same
// identity binds the barrier before any catalog read.
func TestT26DropServerUnauditedWriteBlocksLaterLookup(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}}
	enriched := enrichT26(t, "CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS(HOST 'h'); DROP SERVER s1", spec.DialectMySQL, provider)
	if len(enriched) != 2 || enriched[0].Unsupported == nil || enriched[1].Unsupported != nil {
		t.Fatalf("create must stay boundary, drop must be audited: %+v / %+v", enriched[0].Unsupported, enriched[1].Unsupported)
	}
	if len(provider.calls) != 1 || provider.calls[0].name != "s1" {
		t.Fatalf("resolver calls = %+v, want the create's own lookup only", provider.calls)
	}
	second := enriched[1].Metadata.Server
	if second == nil || second.UnreadReason != spec.MetadataUnreadBatchEffects || second.Read {
		t.Fatalf("drop after unreviewed write must bind batch_effects: %+v", second)
	}
}

// A string-literal SERVER write has no provable identity — it taints the
// whole channel and every later statement exits before the relation
// scan or the resolver.
func TestT26DropServerLiteralWriteTaintsChannel(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}}
	enriched := enrichT26(t, "DROP SERVER 'lit'; DROP SERVER s1", spec.DialectMySQL, provider)
	if enriched[0].Unsupported == nil {
		t.Fatalf("literal drop must stay the unaudited boundary")
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v, want only the literal's own unqualified lookup", provider.calls)
	}
	second := enriched[1].Metadata.Server
	if second == nil || second.UnreadReason != spec.MetadataUnreadBatchEffects || second.Read {
		t.Fatalf("drop after literal write must bind batch_effects — channel taint: %+v", second)
	}
}

// Non-ASCII names are outside the registered fold subset — a drop on
// `s1` barriers a later `ü`-named statement through the unproven
// relation, even though byte folding could never alias them.
func TestT26DropServerNonASCIIRelationUnproven(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}}
	enriched := enrichT26(t, "DROP SERVER s1; DROP SERVER `ü`", spec.DialectMySQL, provider)
	second := enriched[1].Metadata.Server
	if second == nil || second.UnreadReason != spec.MetadataUnreadBatchEffects {
		t.Fatalf("non-ascii relation must stay unproven — batch_effects: %+v", second)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v, want the barrier path to skip the resolver", provider.calls)
	}
}

// A proven-present binding is never rewritten by later applies — the
// first statement's adjudicated fact stays intact and the provider
// snapshot member is untouched.
func TestT26DropServerPriorBindingImmutable(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}}
	enriched := enrichT26(t, "DROP SERVER s1; DROP SERVER s1", spec.DialectMySQL, provider)
	first := enriched[0].Metadata.Server
	if first == nil || !first.Read || !first.Exists || first.Status != spec.MetadataStatusConfirmed {
		t.Fatalf("first binding must stay the proven-present fact: %+v", first)
	}
}

// The SERVER channel never reaches the table/schema model — a provable
// table pre-state (batch-created and provider-vouched alike) stays
// consumable on the table statement following the SERVER statement.
func TestT26DropServerTableSiblingsUnaffected(t *testing.T) {
	provider := &t26Provider{
		servers: map[string]bool{"s1": true},
		tables:  map[string]*spec.TableSnapshot{"golden.u": presentTable("golden", "u", "k")},
	}
	enriched := enrichT26(t,
		"CREATE TABLE t (id int); DROP SERVER s1; DROP TABLE t; DROP TABLE u",
		spec.DialectMySQL, provider)
	if len(provider.calls) != 1 || provider.calls[0].typ != "mysql_server" {
		t.Fatalf("resolver calls = %+v, want only the server lookup", provider.calls)
	}
	for i, stmt := range enriched {
		if stmt.Metadata != nil && stmt.Metadata.Server != nil && i != 1 {
			t.Fatalf("table statements must not bind server facts: %+v", stmt.Metadata)
		}
	}
	// The batch-created table binds its ordered present shape after the
	// SERVER statement — contamination would degrade this binding.
	dropT := enriched[2].Metadata.TargetTables[0]
	if dropT.Snapshot == nil || !dropT.Snapshot.Exists || dropT.Snapshot.FindColumn("id") == nil {
		t.Fatalf("post-SERVER drop must still consume the batch-derived present table: %+v", dropT)
	}
	// The provider-vouched table keeps its catalog snapshot — a tainted
	// or poisoned batch would shadow it into unvouched.
	dropU := enriched[3].Metadata.TargetTables[0]
	if dropU.Snapshot == nil || !dropU.Snapshot.Exists || dropU.Snapshot.FindColumn("k") == nil {
		t.Fatalf("post-SERVER drop must still consume the provider-present table: %+v", dropU)
	}
}

// A zero-row catalog answer binds read-unknown — never absent, never
// an error — and still leaves the drop's own barrier.
func TestT26DropServerZeroRowBindsUnknown(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{}}
	enriched := enrichT26(t, "DROP SERVER ghost", spec.DialectMySQL, provider)
	state := enriched[0].Metadata.Server
	if state == nil || !state.Read || state.Exists || state.Name != "ghost" {
		t.Fatalf("zero-row state = %+v, want read+unknown with the verbatim name", state)
	}
}

// Without a provider the identity binds unvouched — never absent.
func TestT26DropServerNoProviderUnvouched(t *testing.T) {
	enriched := enrichT26(t, "DROP SERVER s1", spec.DialectMySQL, nil)
	state := enriched[0].Metadata.Server
	if state == nil || state.Read || state.Exists || state.UnreadReason != "" {
		t.Fatalf("provider-less state = %+v, want bound + unvouched", state)
	}
}

// Under TiDB the whole family stays the vendor boundary — the audited
// slice never widens past the MySQL dialect.
func TestT26DropServerTiDBVendorBoundary(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}}
	enriched := enrichT26(t, "DROP SERVER s1", spec.DialectTiDB, provider)
	if enriched[0].Unsupported == nil || enriched[0].Unsupported.Reason != spec.UnsupportedVendorBoundaryReason {
		t.Fatalf("tidb drop must stay vendor boundary: %+v", enriched[0].Unsupported)
	}
	for _, call := range provider.calls {
		if call.typ == "mysql_server" {
			t.Fatalf("tidb must never issue a server lookup")
		}
	}
}

// The audited statement reaches the public result with the notice and
// the bounded gap vocabulary — existence unproven plus the always-owed
// drop privilege — and coverage lands unverified, never complete.
func TestT26DropServerPublicShape(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: spec.DialectMySQL, Schema: "golden", MetadataProvider: provider,
	})
	if errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("audited drop must not surface the unsupported boundary: %v", err)
	}
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("audited drop must carry no unsupported entries: %+v", result.Unsupported)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements = %d", len(result.Statements))
	}
	stmt := result.Statements[0]
	var found bool
	for _, f := range stmt.Findings {
		if f.RuleID == "ddl.mysql.server.drop.notice" {
			found = true
			if strings.Contains(f.Message, "s1") {
				t.Fatalf("authored name leaked into the notice: %q", f.Message)
			}
		}
	}
	if !found {
		t.Fatalf("drop notice missing: %+v", stmt.Findings)
	}
	var facts []string
	for _, g := range stmt.EvidenceGaps {
		facts = append(facts, g.RequiredFacts...)
	}
	if len(stmt.EvidenceGaps) != 1 || !containsString(facts, "server.drop_privilege") {
		t.Fatalf("proven-present drop must owe only the privilege gap: %+v", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage = %v, want unverified", stmt.Coverage.Status)
	}
}

// An unproven drop surfaces both gaps — existence plus privilege — and
// no absent publication anywhere in the result shape.
func TestT26DropServerPublicShapeUnproven(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{}}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SERVER IF EXISTS ghost", Dialect: spec.DialectMySQL, Schema: "golden", MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	var facts []string
	for _, g := range stmt.EvidenceGaps {
		facts = append(facts, g.RequiredFacts...)
	}
	if !containsString(facts, "server.existence") || !containsString(facts, "server.drop_privilege") {
		t.Fatalf("unproven if-exists drop must owe existence+privilege: %+v", stmt.EvidenceGaps)
	}
	for _, f := range stmt.Findings {
		if strings.Contains(strings.ToLower(f.Message), "absent") || strings.Contains(f.Message, "ghost") {
			t.Fatalf("absent verdict or authored name leaked: %q", f.Message)
		}
	}
}

// A two-statement batch exercises the barrier's public shape end to
// end: the case alias binds batch_effects, the rule emits the notice
// with if_exists, the gap set substitutes batch_effects for existence,
// and coverage stays unverified — the binding, registration, gap
// aggregation, and coverage merge hold together.
func TestT26DropServerBarrierPublicShape(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "DROP SERVER s1; DROP SERVER IF EXISTS `S1`",
		Dialect: spec.DialectMySQL, Schema: "golden", MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 2 || len(result.Unsupported) != 0 {
		t.Fatalf("both audited drops expected, unsupported=%+v statements=%d", result.Unsupported, len(result.Statements))
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("top-level verdict = %v, want review", result.Verdict)
	}
	second := result.Statements[1]
	var found bool
	for _, f := range second.Findings {
		if f.RuleID == "ddl.mysql.server.drop.notice" {
			found = true
			if f.Metadata["if_exists"] != true {
				t.Fatalf("if_exists flag missing on the barrier notice: %+v", f.Metadata)
			}
			if strings.Contains(f.Message, "S1") || strings.Contains(strings.ToLower(f.Message), "dropped") ||
				strings.Contains(strings.ToLower(f.Message), "removed") {
				t.Fatalf("notice claims an executed result or leaks the authored name: %q", f.Message)
			}
		}
	}
	if !found {
		t.Fatalf("notice missing on the barrier statement: %+v", second.Findings)
	}
	var facts []string
	for _, g := range second.EvidenceGaps {
		facts = append(facts, g.RequiredFacts...)
	}
	if !containsString(facts, "server.batch_effects") || !containsString(facts, "server.drop_privilege") {
		t.Fatalf("barrier must owe batch_effects+privilege: %+v", second.EvidenceGaps)
	}
	if containsString(facts, "server.existence") {
		t.Fatalf("batch_effects replaces existence — dedup contract: %+v", second.EvidenceGaps)
	}
	if second.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage = %v, want unverified", second.Coverage.Status)
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

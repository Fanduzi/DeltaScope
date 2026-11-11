// T26-SERVER-DROP-EXECUTION-01: the privilege premise's supply
// binding, ordered scope, shared-adjudication consumption, and the
// bounded error boundary — all verified through the real
// parse→extract→enrich seam and the public AuditSQL surface. The stub
// provider records every loader invocation so tests assert the narrow
// gate actually held.
package audit

import (
	"context"
	"errors"
	"strings"
	"testing"

	apppolicy "github.com/Fanduzi/DeltaScope/internal/application/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t26PlainProvider is a provider that does NOT implement the optional
// privilege loader — the mode gate must leave the premise owed.
type t26PlainProvider struct{ t26ProviderBase }

type t26ProviderBase struct{ servers map[string]bool }

func (p *t26PlainProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "8.0.36"}, nil
}

func (p *t26PlainProvider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}

func (p *t26PlainProvider) ResolveObject(_ context.Context, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	snapshot := &spec.ObjectSnapshot{Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}
	if req.Type != "mysql_server" || dialect != spec.DialectMySQL {
		return snapshot, nil
	}
	state := &spec.ServerState{Name: req.Name, Read: true}
	if p.servers[req.Name] {
		state.Exists, state.Status = true, spec.MetadataStatusConfirmed
		snapshot.Status, snapshot.Exists = spec.MetadataStatusConfirmed, true
	}
	snapshot.Server = state
	return snapshot, nil
}

// t26LoaderError is the typed cause for the errors.As assertion — the
// bounded public boundary must let it through the chain.
type t26LoaderError struct{ msg string }

func (e *t26LoaderError) Error() string { return e.msg }

// superObservation is the stubbed positive witness — bound account,
// exact-token SUPER on a direct global grant line.
func superObservation() *spec.MySQLServerDropPrivilegeObservation {
	return &spec.MySQLServerDropPrivilegeObservation{
		Account: "audited@%",
		GrantLines: []string{
			"GRANT SELECT, INSERT, SUPER ON *.* TO 'audited'@'%'",
		},
	}
}

// enrichT26Premise runs the shared seam under the conditional mode —
// the same path AuditSQL takes, observing bound metadata members the
// public result does not carry.
func enrichT26Premise(t *testing.T, sql string, provider MetadataProvider, mode spec.PrivilegeAuditMode) []spec.Statement {
	t.Helper()
	parsed, parseErr := parseSQL(context.Background(), sql, spec.DialectMySQL)
	if parseErr != nil || len(parsed.Statements) == 0 {
		t.Fatalf("parse %q: err=%v statements=%d", sql, parseErr, len(parsed.Statements))
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
	return enriched
}

// The loader runs at most once per request and only under the exact
// gate: conditional mode selected AND an audited identifier-form DROP
// SERVER present AND the MySQL dialect. Boundary forms, non-target
// families, other dialects, and unselected mode never trigger the
// read.
func TestT26PrivilegeSupplyGate(t *testing.T) {
	cases := []struct {
		name     string
		sql      string
		dialect  spec.Dialect
		mode     spec.PrivilegeAuditMode
		wantCall int
	}{
		{"mode selected + audited drop", "DROP SERVER s1", spec.DialectMySQL, spec.PrivilegeAuditModeObservedAccountNewSession, 1},
		{"mode selected + three audited drops still one read", "DROP SERVER s1; DROP SERVER s2; DROP SERVER s3", spec.DialectMySQL, spec.PrivilegeAuditModeObservedAccountNewSession, 1},
		{"mode not selected keeps premise owed", "DROP SERVER s1", spec.DialectMySQL, "", 0},
		{"non-target family never reads", "CREATE TABLE t (id int); DROP TABLE t", spec.DialectMySQL, spec.PrivilegeAuditModeObservedAccountNewSession, 0},
		{"literal-name drop stays boundary — no read", "DROP SERVER 's 1'", spec.DialectMySQL, spec.PrivilegeAuditModeObservedAccountNewSession, 0},
		{"qualified create server reads once", "CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h')", spec.DialectMySQL, spec.PrivilegeAuditModeObservedAccountNewSession, 1},
		{"literal-name create stays boundary — no read", "CREATE SERVER 's 1' FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h')", spec.DialectMySQL, spec.PrivilegeAuditModeObservedAccountNewSession, 0},
		{"non-mysql wrapper create stays boundary — no read", "CREATE SERVER s1 FOREIGN DATA WRAPPER pgsql OPTIONS (HOST 'h')", spec.DialectMySQL, spec.PrivilegeAuditModeObservedAccountNewSession, 0},
		{"tidb vendor boundary never reads", "DROP SERVER s1", spec.DialectTiDB, spec.PrivilegeAuditModeObservedAccountNewSession, 0},
	}
	for _, tc := range cases {
		version := "8.0.36"
		if tc.dialect != spec.DialectMySQL {
			// An observed MySQL banner under another dialect fails the
			// product gate before the family gate — keep the banner
			// unobserved so the family gate itself is what the row
			// registers.
			version = ""
		}
		provider := &t26Provider{servers: map[string]bool{"s1": true}, version: version, privilege: superObservation()}
		parsed, parseErr := parseSQL(context.Background(), tc.sql, tc.dialect)
		if parseErr != nil {
			t.Fatalf("%s: parse: %v", tc.name, parseErr)
		}
		statements, err := Extract(context.Background(), parsed)
		if err != nil {
			t.Fatalf("%s: extract: %v", tc.name, err)
		}
		_, err = enrichStatementsWithMetadata(context.Background(), tc.dialect,
			&MetadataRequest{Schema: "golden", Provider: provider, PrivilegeAuditMode: tc.mode}, statements, parsed.failures)
		if err != nil {
			t.Fatalf("%s: enrich: %v", tc.name, err)
		}
		if provider.privilegeCalls != tc.wantCall {
			t.Fatalf("%s: loader calls = %d, want %d", tc.name, provider.privilegeCalls, tc.wantCall)
		}
	}
}

// A provider that never implemented the loader leaves the premise
// owed — the mode selection never fabricates a witness.
func TestT26PrivilegeNoLoaderStaysOwed(t *testing.T) {
	provider := &t26PlainProvider{t26ProviderBase{servers: map[string]bool{"s1": true}}}
	enriched := enrichT26Premise(t, "DROP SERVER s1", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	adj := enriched[0].Metadata.ServerDrop
	if adj == nil || adj.Privilege != spec.ServerDropPrivilegeUnproven {
		t.Fatalf("privilege premise = %+v, want unproven without a loader", adj)
	}
}

// The witness binds all premises at once — every missing or
// out-of-domain input keeps the premise unproven, never a denial.
func TestT26PrivilegeWitnessBindings(t *testing.T) {
	cases := []struct {
		name    string
		version string
		obs     *spec.MySQLServerDropPrivilegeObservation
		want    spec.ServerDropPrivilege
	}{
		{"super token discharges", "8.0.36", superObservation(), spec.ServerDropPrivilegeConsumable},
		{"all privileges on 8.0 discharges", "8.0.36", &spec.MySQLServerDropPrivilegeObservation{Account: "a@%", GrantLines: []string{"GRANT ALL PRIVILEGES ON *.* TO 'a'@'%'"}}, spec.ServerDropPrivilegeConsumable},
		{"all privileges on 8.4 stays unproven", "8.4.2", &spec.MySQLServerDropPrivilegeObservation{Account: "a@%", GrantLines: []string{"GRANT ALL PRIVILEGES ON *.* TO 'a'@'%'"}}, spec.ServerDropPrivilegeUnproven},
		{"unbound account stays unproven", "8.0.36", &spec.MySQLServerDropPrivilegeObservation{Account: "", GrantLines: []string{"GRANT SUPER ON *.* TO 'a'@'%'"}}, spec.ServerDropPrivilegeUnproven},
		{"connection_admin is not super", "8.0.36", &spec.MySQLServerDropPrivilegeObservation{Account: "a@%", GrantLines: []string{"GRANT CONNECTION_ADMIN ON *.* TO 'a'@'%'"}}, spec.ServerDropPrivilegeUnproven},
		{"schema-scoped super is not global", "8.0.36", &spec.MySQLServerDropPrivilegeObservation{Account: "a@%", GrantLines: []string{"GRANT SUPER ON app.* TO 'a'@'%'"}}, spec.ServerDropPrivilegeUnproven},
		{"empty grant read stays unproven", "8.0.36", &spec.MySQLServerDropPrivilegeObservation{Account: "a@%"}, spec.ServerDropPrivilegeUnproven},
		{"unresolved version stays unproven", "", superObservation(), spec.ServerDropPrivilegeUnproven},
		{"tidb banner stays unproven", "8.0.36-TiDB-v8.5.0", nil, spec.ServerDropPrivilegeUnproven},
		{"nil observation stays unproven", "8.0.36", nil, spec.ServerDropPrivilegeUnproven},
	}
	for _, tc := range cases {
		provider := &t26Provider{servers: map[string]bool{"s1": true}, version: tc.version, privilege: tc.obs}
		// A TiDB banner under the MySQL dialect fails the
		// dialect/product gate before privilege binding — that guard
		// is itself the boundary being registered.
		parsed, _ := parseSQL(context.Background(), "DROP SERVER s1", spec.DialectMySQL)
		statements, _ := Extract(context.Background(), parsed)
		enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectMySQL,
			&MetadataRequest{Schema: "golden", Provider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession}, statements, parsed.failures)
		if strings.Contains(tc.version, "TiDB") {
			if err == nil || !errors.Is(err, ErrDialectProductMismatch) {
				t.Fatalf("%s: tidb banner must fail the product gate: %v", tc.name, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: enrich: %v", tc.name, err)
		}
		adj := enriched[0].Metadata.ServerDrop
		if adj == nil || adj.Privilege != tc.want {
			t.Fatalf("%s: premise = %+v, want %v", tc.name, adj, tc.want)
		}
	}
}

// Consumable premise + proven-present existence discharges both facts —
// the shared adjudication publishes the drop and the statement carries
// the conditional-audit token, never the account or grant text.
func TestT26PrivilegeProvenDropPublishes(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t, "DROP SERVER s1", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	adj := enriched[0].Metadata.ServerDrop
	if adj == nil || adj.Arm != spec.ServerDropArmProven || !adj.PublishAbsent {
		t.Fatalf("adjudication = %+v, want proven publish arm", adj)
	}
	if len(adj.Owed) != 0 {
		t.Fatalf("proven arm owes nothing: %v", adj.Owed)
	}
	if provider.privilegeCalls != 1 || len(provider.calls) != 1 {
		t.Fatalf("reads = privilege:%d resolver:%d, want 1 each", provider.privilegeCalls, len(provider.calls))
	}
}

// OrderedAbsent consumption: the proven drop's publish answers the
// later same-identity statement before any resolver call — a plain
// re-drop is the terminal refusal and owes nothing, an IF EXISTS
// re-drop is the guarded absent whose privilege premise stands on its
// own.
func TestT26PrivilegeOrderedAbsentConsumption(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t, "DROP SERVER s1; DROP SERVER s1; DROP SERVER IF EXISTS s1", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if len(enriched) != 3 {
		t.Fatalf("statements = %d", len(enriched))
	}
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v — OrderedAbsent must answer before the provider seam", provider.calls)
	}
	second := enriched[1].Metadata.Server
	if second == nil || !second.OrderedAbsent {
		t.Fatalf("second drop must bind OrderedAbsent: %+v", second)
	}
	secondAdj := enriched[1].Metadata.ServerDrop
	if secondAdj == nil || secondAdj.Arm != spec.ServerDropArmTerminalRefusal || len(secondAdj.Owed) != 0 {
		t.Fatalf("second adjudication = %+v, want refusal owing nothing", secondAdj)
	}
	thirdAdj := enriched[2].Metadata.ServerDrop
	if thirdAdj == nil || thirdAdj.Arm != spec.ServerDropArmGuardedAbsent || len(thirdAdj.Owed) != 0 {
		t.Fatalf("third adjudication = %+v, want guarded absent owing nothing under consumable premise", thirdAdj)
	}
	// The ASCII fold relation consumes the same absent — the canonical
	// contract's provable-same subset.
	provider = &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched = enrichT26Premise(t, "DROP SERVER s1; DROP SERVER S1", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if len(provider.calls) != 1 {
		t.Fatalf("fold-equal identity must consume absent without a second lookup: %+v", provider.calls)
	}
	if adj := enriched[1].Metadata.ServerDrop; adj == nil || adj.Arm != spec.ServerDropArmTerminalRefusal {
		t.Fatalf("fold-equal re-drop = %+v, want refusal", adj)
	}
}

// The convergent arm: IF EXISTS with an unproven pre-state and the
// consumable premise publishes the same absent — both branches end
// absent, so the statement owes nothing and the post-state is proven.
func TestT26PrivilegeConvergentPublishes(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t, "DROP SERVER IF EXISTS s1; DROP SERVER s1", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	first := enriched[0].Metadata.ServerDrop
	if first == nil || first.Arm != spec.ServerDropArmConvergent || !first.PublishAbsent || len(first.Owed) != 0 {
		t.Fatalf("convergent adjudication = %+v", first)
	}
	second := enriched[1].Metadata.Server
	if second == nil || !second.OrderedAbsent {
		t.Fatalf("convergent publish must answer the later drop: %+v", second)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("the convergent publish suppresses the second lookup: %+v", provider.calls)
	}
}

// Barriers outrank the premise: an undischarged unknown effect keeps
// existence shadowed and owes batch_effects even when the privilege is
// consumable — the witness never clears a barrier.
func TestT26PrivilegeNeverClearsBarrier(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true, "s2": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t, "DROP SERVER s1; DROP SERVER 'lit x'; DROP SERVER s2", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	third := enriched[2].Metadata.ServerDrop
	if third == nil || third.Arm != spec.ServerDropArmBarrier {
		t.Fatalf("tainted channel must barrier the third drop: %+v", third)
	}
	set := map[string]bool{}
	for _, f := range third.Owed {
		set[f] = true
	}
	if !set[spec.ServerFactBatchEffects] || set[spec.ServerFactDropPrivilege] || set[spec.ServerFactExistence] {
		t.Fatalf("barrier + consumable owes batch_effects only: %v", third.Owed)
	}
	// The published absent on s1 is shadowed too — a literal write may
	// have recreated anything.
	second := enriched[1]
	if second.Unsupported == nil {
		t.Fatalf("literal create stays boundary")
	}
	// The literal drop reaches the seam without the identifier proof
	// (the accepted contract) — but the tainted channel keeps the
	// third drop off the resolver entirely.
	if len(provider.calls) != 2 || provider.calls[0].name != "s1" || provider.calls[1].name != "lit x" {
		t.Fatalf("resolver calls = %+v, want s1 then the unproven literal", provider.calls)
	}
	if provider.calls[1].qualifiers[spec.ObjectLookupQualifierNameForm] == spec.ObjectLookupNameFormIdentifier {
		t.Fatalf("literal name must never carry the identifier proof: %+v", provider.calls[1].qualifiers)
	}
}

// The ordered scope terminator: an authored GRANT/REVOKE or the
// boundary grant/role features end the observation's applicable range
// — a suppressed premise stays owed exactly like an unproven one.
func TestT26PrivilegeScopeTerminator(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"revoke", "REVOKE SELECT ON *.* FROM 'u'@'%'; DROP SERVER s1"},
		{"grant", "GRANT SELECT ON *.* TO 'u'@'%'; DROP SERVER s1"},
		{"set role boundary", "SET ROLE ALL; DROP SERVER s1"},
		{"set default role boundary", "SET DEFAULT ROLE r1 TO 'u'@'%'; DROP SERVER s1"},
		{"grant role boundary", "GRANT 'r'@'%' TO 'u'@'%'; DROP SERVER s1"},
	}
	for _, tc := range cases {
		provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
		enriched := enrichT26Premise(t, tc.sql, provider, spec.PrivilegeAuditModeObservedAccountNewSession)
		last := enriched[len(enriched)-1].Metadata.ServerDrop
		if last == nil || last.Privilege != spec.ServerDropPrivilegeSuppressed {
			t.Fatalf("%s: premise = %+v, want suppressed after the terminator", tc.name, last)
		}
		set := map[string]bool{}
		for _, f := range last.Owed {
			set[f] = true
		}
		if !set[spec.ServerFactDropPrivilege] {
			t.Fatalf("%s: suppressed premise must owe drop_privilege: %v", tc.name, last.Owed)
		}
	}
	// SET DEFAULT ROLE is a real boundary classification — assert the
	// feature identity alongside the projection effect, and that an
	// EARLIER drop keeps its consumable premise (scope is positional,
	// never retroactive).
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t, "DROP SERVER s1; SET DEFAULT ROLE r1 TO 'u'@'%'; DROP SERVER s2", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if enriched[1].Unsupported == nil || enriched[1].Unsupported.Feature != "set_default_role" {
		t.Fatalf("SET DEFAULT ROLE classification = %+v", enriched[1].Unsupported)
	}
	if adj := enriched[0].Metadata.ServerDrop; adj == nil || adj.Privilege != spec.ServerDropPrivilegeConsumable {
		t.Fatalf("earlier drop keeps the consumable premise: %+v", adj)
	}
	if adj := enriched[2].Metadata.ServerDrop; adj == nil || adj.Privilege != spec.ServerDropPrivilegeSuppressed {
		t.Fatalf("later drop premise = %+v, want suppressed", adj)
	}
	// A terminator AFTER the drop never suppresses it retroactively —
	// ordered scope is positional.
	provider = &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched = enrichT26Premise(t, "DROP SERVER s1; REVOKE SELECT ON *.* FROM 'u'@'%'", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if adj := enriched[0].Metadata.ServerDrop; adj == nil || adj.Privilege != spec.ServerDropPrivilegeConsumable {
		t.Fatalf("earlier drop keeps the consumable premise: %+v", adj)
	}
}

// End-to-end public shape: the consumable proven drop yields no gaps
// and the conditional tokens, while the same batch without the mode
// keeps the legacy owed set. The refusal's re-drop is a blocker under
// the same audit.
func TestT26PrivilegeAuditSQLPublicShape(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != report.VerdictPass {
		t.Fatalf("consumable proven drop must pass: %+v", result.Verdict)
	}
	if len(result.Statements) != 1 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("proven arm owes no gaps: %+v", result.Statements)
	}
	if len(result.Statements[0].Findings) != 1 {
		t.Fatalf("proven arm emits exactly one finding: %+v", result.Statements[0])
	}
	finding := result.Statements[0].Findings[0]
	if finding.Level != rule.LevelNotice || finding.Metadata["outcome"] != "conditional_drop" {
		t.Fatalf("finding = %+v, want notice/conditional_drop", finding)
	}
	if finding.Metadata["conditional_audit"] != string(spec.PrivilegeAuditModeObservedAccountNewSession) {
		t.Fatalf("conditional scope token missing: %+v", finding.Metadata)
	}
	blob := finding.Message + finding.Explanation.Why + finding.Explanation.Risk + finding.Explanation.Suggestion
	for key, value := range finding.Metadata {
		blob += key
		if s, ok := value.(string); ok {
			blob += s
		}
	}
	for _, protected := range []string{"audited@%", "GRANT SELECT", "SHOW GRANTS", "s1"} {
		if strings.Contains(blob, protected) {
			t.Fatalf("protected detail %q leaked into the finding: %q", protected, blob)
		}
	}

	// The same batch without the mode keeps the legacy owed set — the
	// premise is never silently discharged.
	provider = &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, err = AuditSQL(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: spec.DialectMySQL, Schema: "golden", MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("unwired premise keeps the review floor: %+v", result.Verdict)
	}
	// The no-mode proven-present path owes EXACTLY the privilege
	// premise — existence is already proven by the resolver read.
	facts := result.Statements[0].EvidenceGaps[0].RequiredFacts
	if len(facts) != 1 || facts[0] != spec.ServerFactDropPrivilege {
		t.Fatalf("default path owed = %v, want exactly {server.drop_privilege}", facts)
	}
	if result.Statements[0].Coverage.Status != report.CoverageUnverified {
		t.Fatalf("statement coverage = %v, want unverified under the owed premise", result.Statements[0].Coverage.Status)
	}
	if result.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("aggregate coverage = %v, want unverified", result.Coverage.Status)
	}
}

// The re-drop of a proven-absent entry is the shared refusal shape —
// blocker, reject verdict, no gaps.
func TestT26PrivilegeRefusalPublicShape(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SERVER s1; DROP SERVER s1", Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("proven refusal must reject: %+v", result.Verdict)
	}
	finding := result.Statements[1].Findings[0]
	if finding.Level != rule.LevelBlocker || finding.Metadata["outcome"] != "refused_absent" {
		t.Fatalf("refusal finding = %+v, want blocker/refused_absent", finding)
	}
	if len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("refusal owes nothing: %+v", result.Statements[1].EvidenceGaps)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("OrderedAbsent must answer before the provider seam: %+v", provider.calls)
	}
}

// R1-D: a refusal or guard resting on an absent a CONDITIONAL arm
// published must keep the conditional-audit marker even when the
// consuming statement's own premise is suppressed by a scope
// terminator — the conclusion stays conditional. Real batch
// reachability: proven drop publishes, REVOKE suppresses later
// consumability, the re-drop still binds OrderedAbsent first. The
// finding surface is evaluated through the real registry seam —
// service-level unsupported gating on the terminator statement is
// orthogonal to the DROP finding contract.
func TestT26ConditionalAbsentPublicShape(t *testing.T) {
	evaluate := func(t *testing.T, sql string, provider *t26Provider) report.Result {
		t.Helper()
		parsed, parseErr := parseSQL(context.Background(), sql, spec.DialectMySQL)
		if parseErr != nil {
			t.Fatalf("parse %q: %v", sql, parseErr)
		}
		statements, err := Extract(context.Background(), parsed)
		if err != nil {
			t.Fatalf("extract %q: %v", sql, err)
		}
		enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectMySQL,
			&MetadataRequest{Schema: "golden", Provider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession}, statements, parsed.failures)
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
		return result
	}

	// Plain re-drop after a terminator: refusal blocker, no gaps,
	// conditional marker from the publishing arm — and the bounded
	// refusal text never claims a specific native response.
	result := evaluate(t, "DROP SERVER s1; REVOKE SELECT ON *.* FROM 'u'@'%'; DROP SERVER s1",
		&t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()})
	refusal := result.Statements[2].Findings[0]
	if refusal.Level != rule.LevelBlocker || refusal.Metadata["outcome"] != "refused_absent" {
		t.Fatalf("refusal finding = %+v", refusal)
	}
	if refusal.Metadata["conditional_audit"] != string(spec.PrivilegeAuditModeObservedAccountNewSession) {
		t.Fatalf("refusal on a conditional absent must keep the marker: %+v", refusal.Metadata)
	}
	if len(result.Statements[2].EvidenceGaps) != 0 {
		t.Fatalf("refusal owes nothing: %+v", result.Statements[2].EvidenceGaps)
	}
	blob := refusal.Message + refusal.Explanation.Why + refusal.Explanation.Risk
	for _, overclaimed := range []string{"already gone", "not-found error at execution", "server-not-found"} {
		if strings.Contains(blob, overclaimed) {
			t.Fatalf("refusal text overclaims a native response %q: %q", overclaimed, blob)
		}
	}

	// IF EXISTS re-drop after a terminator: guarded absent, owes
	// drop_privilege only, conditional marker present, and the public
	// text is the model-projection shape — never "proven guard no-op"
	// or "entry already gone".
	result = evaluate(t, "DROP SERVER s1; REVOKE SELECT ON *.* FROM 'u'@'%'; DROP SERVER IF EXISTS s1",
		&t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()})
	guarded := result.Statements[2]
	if len(guarded.EvidenceGaps) != 1 {
		t.Fatalf("guarded + suppressed owes exactly the privilege gap: %+v", guarded.EvidenceGaps)
	}
	facts := guarded.EvidenceGaps[0].RequiredFacts
	if len(facts) != 1 || facts[0] != spec.ServerFactDropPrivilege {
		t.Fatalf("guarded + suppressed owed = %v, want drop_privilege only", facts)
	}
	finding := guarded.Findings[0]
	if finding.Metadata["outcome"] != "guarded_absent" {
		t.Fatalf("guarded finding outcome = %+v", finding.Metadata)
	}
	if finding.Metadata["conditional_audit"] != string(spec.PrivilegeAuditModeObservedAccountNewSession) {
		t.Fatalf("guarded on a conditional absent must keep the marker: %+v", finding.Metadata)
	}
	blob = finding.Message + finding.Explanation.Why + finding.Explanation.Risk + finding.Explanation.Suggestion
	for _, overclaimed := range []string{"proven guard no-op", "proven no-op", "already gone"} {
		if strings.Contains(blob, overclaimed) {
			t.Fatalf("guarded text overclaims %q: %q", overclaimed, blob)
		}
	}
	for _, required := range []string{"premise remains unverified", "model projection"} {
		if !strings.Contains(blob, required) {
			t.Fatalf("guarded text must state %q: %q", required, blob)
		}
	}
	for _, protected := range []string{"audited@%", "GRANT SELECT", "s1", "'u'@'%'"} {
		if strings.Contains(blob, protected) {
			t.Fatalf("protected detail %q leaked: %q", protected, blob)
		}
	}
}

// A failed privilege observation propagates as a bounded public error —
// fixed text outside, the original cause reachable through errors.Is,
// and never laundered into an unproven grant or an evidence gap.
func TestT26PrivilegeReadErrorBoundary(t *testing.T) {
	marker := errors.New("t26-privilege-marker-9z1")
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilegeErr: marker}
	_, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err == nil {
		t.Fatalf("privilege read failure must still fail the audit")
	}
	if !errors.Is(err, marker) {
		t.Fatalf("public error chain lost the loader cause: %v", err)
	}
	for _, protected := range []string{marker.Error(), "s1", "audited", "SHOW GRANTS"} {
		if strings.Contains(err.Error(), protected) {
			t.Fatalf("public error text echoes protected content %q: %v", protected, err)
		}
	}
	// errors.As reaches the typed cause through the same bounded
	// boundary — a distinct guarantee from errors.Is.
	typed := &t26LoaderError{msg: "typed-priv-marker-7x2"}
	provider = &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilegeErr: typed}
	_, err = AuditSQL(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: spec.DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	var reached *t26LoaderError
	if !errors.As(err, &reached) || reached != typed {
		t.Fatalf("errors.As must reach the typed loader cause: %v", err)
	}
	if strings.Contains(err.Error(), typed.msg) {
		t.Fatalf("public error text echoes the typed cause: %v", err)
	}
}

// The mode token is a closed vocabulary — an unregistered value is an
// input error, never a silently-defaulted mode.
func TestT26PrivilegeModeValidation(t *testing.T) {
	_, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: spec.DialectMySQL, PrivilegeAuditMode: "bogus_mode",
	})
	if err == nil || !errors.Is(err, ErrInvalidPrivilegeMode) {
		t.Fatalf("unregistered mode must fail closed: %v", err)
	}
	// R1-B: the effective mode merges top-level + legacy before
	// validation — an invalid legacy fallback must error before any
	// provider read rather than silently degrade to mode-unselected.
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	_, err = AuditSQL(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: spec.DialectMySQL,
		Metadata: &MetadataRequest{Provider: provider, PrivilegeAuditMode: "bogus_mode"},
	})
	if err == nil || !errors.Is(err, ErrInvalidPrivilegeMode) {
		t.Fatalf("invalid legacy fallback must fail closed: %v", err)
	}
	if provider.privilegeCalls != 0 {
		t.Fatalf("invalid effective mode triggered %d loader calls", provider.privilegeCalls)
	}
	// Top empty + legacy valid: the merged effective value enters the
	// normal gate — the loader runs once.
	provider = &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		Metadata: &MetadataRequest{Provider: provider, PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession},
	})
	if err != nil {
		t.Fatalf("valid legacy fallback must reach the normal path: %v", err)
	}
	if provider.privilegeCalls != 1 {
		t.Fatalf("valid legacy mode must run the loader once: %d", provider.privilegeCalls)
	}
	if result.Verdict != report.VerdictPass {
		t.Fatalf("legacy-selected mode result = %+v", result.Verdict)
	}
	// Top-level non-empty wins over a differing legacy value — the
	// unadopted legacy bogus value never reaches validation.
	provider = &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	result, err = AuditSQL(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
		Metadata:           &MetadataRequest{Provider: provider, PrivilegeAuditMode: "bogus_mode"},
	})
	if err != nil {
		t.Fatalf("top-level precedence must ignore the unadopted legacy value: %v", err)
	}
	if provider.privilegeCalls != 1 || result.Verdict != report.VerdictPass {
		t.Fatalf("top-level precedence: calls=%d verdict=%v", provider.privilegeCalls, result.Verdict)
	}
}

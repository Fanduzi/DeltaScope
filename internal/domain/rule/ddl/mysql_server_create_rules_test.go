// T26-SERVER-CREATE-EXECUTION-01: the audited MySQL CREATE SERVER
// rule — the finding set projects the shared adjudication, refusal is
// a blocker, the completeness notice is advisory-only, and no authored
// name, wrapper, OPTIONS content, or account reaches any emitted
// shape.
package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func mysqlCreateServerStatement(t *testing.T, state *spec.ServerState, adjudication *spec.ServerCreateAdjudication, options []spec.ServerOption) spec.Statement {
	t.Helper()
	stmt := spec.Statement{
		Dialect: spec.DialectMySQL,
		Kind:    spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateServer,
			Server: &spec.ServerDeclaration{
				Kind:      spec.ServerKindCreate,
				Name:      "s1",
				NameIdent: true,
				Wrapper:   "mysql",
				Options:   options,
			},
		},
		Metadata: &spec.Metadata{Server: state, ServerCreate: adjudication},
	}
	return stmt
}

func mysqlCreateServerRuleUnderTest(t *testing.T) mysqlCreateServerRule {
	t.Helper()
	r, err := newMySQLCreateServerRule(policy.RulePolicy{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	return r.(mysqlCreateServerRule)
}

func fullCreateOptions() []spec.ServerOption {
	return []spec.ServerOption{
		{Key: "user", ValueKind: "char"},
		{Key: "host", ValueKind: "char"},
		{Key: "database", ValueKind: "char"},
	}
}

// The proven arm projects the conditional create outcome, owes no
// gaps, and carries the conditional-audit scope token.
func TestMySQLCreateServerProven(t *testing.T) {
	r := mysqlCreateServerRuleUnderTest(t)
	adj := spec.AdjudicateServerCreate(
		&spec.ServerState{Name: "s1", OrderedAbsent: true, OrderedAbsentConditional: true},
		spec.ServerCreatePrivilegeConsumable, true, false)
	stmt := mysqlCreateServerStatement(t, nil, &adj, fullCreateOptions())
	if !r.AppliesTo(stmt) {
		t.Fatalf("audited create must apply")
	}
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil || len(findings) != 1 {
		t.Fatalf("findings = %v err=%v, want one notice", findings, err)
	}
	if findings[0].Level != rule.LevelNotice || findings[0].Metadata["outcome"] != "conditional_create" {
		t.Fatalf("proven finding = %+v, want notice/conditional_create", findings[0])
	}
	if findings[0].Metadata["conditional_audit"] != string(spec.PrivilegeAuditModeObservedAccountNewSession) {
		t.Fatalf("conditional scope token missing: %+v", findings[0].Metadata)
	}
	if got := gapFacts(r.EvidenceGaps(stmt)); len(got) != 0 {
		t.Fatalf("proven arm owes no gaps: %v", got)
	}
}

// The terminal refusal is a blocker that owes nothing — occupancy
// alone carries the verdict.
func TestMySQLCreateServerRefusal(t *testing.T) {
	r := mysqlCreateServerRuleUnderTest(t)
	adj := spec.AdjudicateServerCreate(
		&spec.ServerState{Name: "s1", OrderedPresent: true, OrderedPresentConditional: true},
		spec.ServerCreatePrivilegeSuppressed, false, false)
	stmt := mysqlCreateServerStatement(t, &spec.ServerState{Name: "s1", OrderedPresent: true}, &adj, fullCreateOptions())
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil || len(findings) != 1 {
		t.Fatalf("refusal findings = %v err=%v", findings, err)
	}
	if findings[0].Level != rule.LevelBlocker || findings[0].Metadata["outcome"] != "refused_present" {
		t.Fatalf("refusal finding = %+v, want blocker/refused_present", findings[0])
	}
	if !strings.Contains(findings[0].Message, "refused") {
		t.Fatalf("refusal message must state the refusal: %q", findings[0].Message)
	}
	if got := gapFacts(r.EvidenceGaps(stmt)); len(got) != 0 {
		t.Fatalf("refusal owes nothing: %v", got)
	}
}

// The barrier arm owes exactly server.batch_effects — never
// existence — regardless of the statement's own premise.
func TestMySQLCreateServerBarrier(t *testing.T) {
	r := mysqlCreateServerRuleUnderTest(t)
	adj := spec.AdjudicateServerCreate(
		&spec.ServerState{Name: "s1", UnreadReason: spec.MetadataUnreadBatchEffects},
		spec.ServerCreatePrivilegeConsumable, true, false)
	stmt := mysqlCreateServerStatement(t, &spec.ServerState{UnreadReason: spec.MetadataUnreadBatchEffects}, &adj, fullCreateOptions())
	facts := gapFacts(r.EvidenceGaps(stmt))
	if len(facts) != 1 || !hasFact(facts, "server.batch_effects") {
		t.Fatalf("barrier owes exactly batch_effects: %v", facts)
	}
	findings, _ := r.Evaluate(context.Background(), stmt)
	if findings[0].Metadata["outcome"] != "blocked_batch_effects" {
		t.Fatalf("barrier outcome = %+v", findings[0].Metadata)
	}
}

// The unproven arm projects the owed set from the adjudication — never
// a fixed list.
func TestMySQLCreateServerUnprovenOwedSet(t *testing.T) {
	r := mysqlCreateServerRuleUnderTest(t)
	adj := spec.AdjudicateServerCreate(
		&spec.ServerState{Name: "s1", OrderedAbsent: true},
		spec.ServerCreatePrivilegeUnproven, false, false)
	stmt := mysqlCreateServerStatement(t, &spec.ServerState{OrderedAbsent: true}, &adj, nil)
	facts := gapFacts(r.EvidenceGaps(stmt))
	if !hasFact(facts, "server.create_privilege") || !hasFact(facts, "server.create_options") {
		t.Fatalf("unproven on ordered absent must owe privilege+options: %v", facts)
	}
	if hasFact(facts, "server.existence") || hasFact(facts, "server.drop_privilege") {
		t.Fatalf("owed set must never carry existence or a drop premise: %v", facts)
	}
}

// Missing documented members emit a second advisory notice — never a
// blocker and never an owed fact.
func TestMySQLCreateServerCompletenessNotice(t *testing.T) {
	r := mysqlCreateServerRuleUnderTest(t)
	adj := spec.AdjudicateServerCreate(
		&spec.ServerState{Name: "s1", OrderedAbsent: true},
		spec.ServerCreatePrivilegeConsumable, true, false)
	stmt := mysqlCreateServerStatement(t, nil, &adj,
		[]spec.ServerOption{{Key: "host", ValueKind: "char"}})
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil || len(findings) != 2 {
		t.Fatalf("findings = %v err=%v, want arm + completeness notice", findings, err)
	}
	notice := findings[1]
	if notice.Level != rule.LevelNotice || notice.Metadata["notice"] != "connection_members" {
		t.Fatalf("completeness notice = %+v", notice)
	}
	// The public text names no option keys — the notice vocabulary is
	// fixed and generic (R2-B).
	if notice.Message != "CREATE SERVER omits one or more documented connection members; connection completeness is advisory only." {
		t.Fatalf("notice message = %q", notice.Message)
	}
	blob := notice.Message
	if notice.Explanation != nil {
		blob += notice.Explanation.Why + notice.Explanation.Risk + notice.Explanation.Suggestion
	}
	for k, v := range notice.Metadata {
		if s, ok := v.(string); ok {
			blob += k + s
		}
	}
	for _, protected := range []string{"USER", "HOST", "DATABASE", "PASSWORD", "SOCKET", "OWNER", "PORT", "s1"} {
		if strings.Contains(blob, protected) {
			t.Fatalf("notice leaked option-key or authored detail %q: %q", protected, blob)
		}
	}
	// Complete members emit no second finding.
	complete := mysqlCreateServerStatement(t, nil, &adj, fullCreateOptions())
	findings, _ = r.Evaluate(context.Background(), complete)
	if len(findings) != 1 {
		t.Fatalf("complete members = 1 finding, got %+v", findings)
	}
}

// R2-A: conditional_audit is the adjudication's provenance fact — the
// rule projects it verbatim and never re-derives conditionality from
// whether a privilege witness merely exists.
func TestMySQLCreateServerConditionalMarkerProvenance(t *testing.T) {
	r := mysqlCreateServerRuleUnderTest(t)
	hasMarker := func(stmt spec.Statement) bool {
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil || len(findings) == 0 {
			t.Fatalf("evaluate = %v err=%v", findings, err)
		}
		_, ok := findings[0].Metadata["conditional_audit"]
		return ok
	}
	message := func(stmt spec.Statement) string {
		findings, _ := r.Evaluate(context.Background(), stmt)
		return findings[0].Message
	}

	// Consumable premise on the barrier arm stays invisible — the
	// shadowed premise never fed the conclusion.
	barrierState := &spec.ServerState{Name: "s1", UnreadReason: spec.MetadataUnreadBatchEffects}
	barrierAdj := spec.AdjudicateServerCreate(barrierState, spec.ServerCreatePrivilegeConsumable, true, false)
	barrier := mysqlCreateServerStatement(t, barrierState, &barrierAdj, fullCreateOptions())
	if hasMarker(barrier) {
		t.Fatalf("barrier + consumable must not carry conditional_audit")
	}
	if msg := message(barrier); strings.Contains(msg, "conditional") {
		t.Fatalf("barrier message must not invoke the conditional scope: %q", msg)
	}

	// Catalog refusal with a consumable premise still carries no
	// marker — own privilege is never read on the refusal arm.
	catalogState := &spec.ServerState{Name: "s1", Read: true, Exists: true, Status: spec.MetadataStatusConfirmed}
	catalogAdj := spec.AdjudicateServerCreate(catalogState, spec.ServerCreatePrivilegeConsumable, true, false)
	if hasMarker(mysqlCreateServerStatement(t, catalogState, &catalogAdj, fullCreateOptions())) {
		t.Fatalf("catalog refusal + consumable must not carry conditional_audit")
	}

	// Inherited-only: the conditional OrderedPresent alone marks the
	// consumer — the statement's own suppressed premise stays unproven
	// and the text must not claim it was discharged.
	condPresent := &spec.ServerState{Name: "s1", OrderedPresent: true, OrderedPresentConditional: true}
	inherited := spec.AdjudicateServerCreate(condPresent, spec.ServerCreatePrivilegeSuppressed, true, false)
	inheritedStmt := mysqlCreateServerStatement(t, &spec.ServerState{Name: "s1", OrderedPresent: true}, &inherited, fullCreateOptions())
	if !hasMarker(inheritedStmt) {
		t.Fatalf("consumed conditional OrderedPresent must mark the finding")
	}
	msg := message(inheritedStmt)
	if !strings.Contains(msg, "ordered state") || strings.Contains(msg, "privilege premise") {
		t.Fatalf("inherited-only refusal text = %q, want ordered-source attribution only", msg)
	}

	// Dirty options on a proven-absent slot with a consumable premise
	// owes options only — existence was proven, the text must not
	// claim it unproven (R2-A wording fix).
	dirty := spec.AdjudicateServerCreate(&spec.ServerState{Name: "s1", OrderedAbsent: true}, spec.ServerCreatePrivilegeConsumable, false, false)
	dirtyMsg := message(mysqlCreateServerStatement(t, &spec.ServerState{OrderedAbsent: true}, &dirty, nil))
	if strings.Contains(dirtyMsg, "existence") {
		t.Fatalf("proven-absent dirty-options message must not claim existence unverified: %q", dirtyMsg)
	}
	if !strings.Contains(dirtyMsg, "options") || !strings.Contains(dirtyMsg, "observed_account_new_session") {
		t.Fatalf("dirty-options message = %q, want options-only unverified under the conditional scope", dirtyMsg)
	}

	// A suppressed premise on a conditional OrderedAbsent keeps the
	// inherited marker and owes only the privilege — the plain
	// non-conditional text carries no own-premise claim.
	condAbsent := &spec.ServerState{Name: "s1", OrderedAbsent: true, OrderedAbsentConditional: true}
	absAdj := spec.AdjudicateServerCreate(condAbsent, spec.ServerCreatePrivilegeSuppressed, true, false)
	absStmt := mysqlCreateServerStatement(t, &spec.ServerState{OrderedAbsent: true}, &absAdj, nil)
	if !hasMarker(absStmt) {
		t.Fatalf("consumed conditional OrderedAbsent must mark the finding")
	}
	if msg := message(absStmt); strings.Contains(msg, "conditional") {
		t.Fatalf("inherited-only unproven message must not claim the conditional scope for the own premise: %q", msg)
	}
}

// The audited-slice gate: boundary statements, drops, TiDB, and
// non-server DDL never reach this rule.
func TestMySQLCreateServerAppliesToNarrow(t *testing.T) {
	r := mysqlCreateServerRuleUnderTest(t)
	base := mysqlCreateServerStatement(t, nil, nil, nil)

	literal := base
	literal.DDL = &spec.DDL{Operation: spec.DDLOperationCreateServer, Server: &spec.ServerDeclaration{Kind: spec.ServerKindCreate, Name: "s1", NameIdent: false}}
	literal.Unsupported = &spec.UnsupportedDetail{Reason: spec.UnsupportedUnauditedReason}
	if r.AppliesTo(literal) {
		t.Fatalf("literal-named create stays boundary — must not apply")
	}
	gated := base
	gated.Unsupported = &spec.UnsupportedDetail{Reason: spec.UnsupportedUnauditedReason, Metadata: map[string]any{"gate": "version"}}
	if r.AppliesTo(gated) {
		t.Fatalf("version-gated create stays boundary — must not apply")
	}
	drop := base
	drop.DDL = &spec.DDL{Operation: spec.DDLOperationDropServer, Server: &spec.ServerDeclaration{Kind: spec.ServerKindDrop, Name: "s1", NameIdent: true}}
	if r.AppliesTo(drop) {
		t.Fatalf("drop server is a different rule — must not apply")
	}
	tidb := base
	tidb.Dialect = spec.DialectTiDB
	if r.AppliesTo(tidb) {
		t.Fatalf("tidb is a vendor boundary — must not apply")
	}
}

// No authored name, wrapper, OPTIONS content, or account reaches any
// emitted shape across every arm.
func TestMySQLCreateServerNoLeak(t *testing.T) {
	r := mysqlCreateServerRuleUnderTest(t)
	for _, tc := range []struct {
		name  string
		state *spec.ServerState
		priv  spec.ServerCreatePrivilege
		clean bool
	}{
		{"refusal", &spec.ServerState{OrderedPresent: true}, spec.ServerCreatePrivilegeUnproven, false},
		{"proven", &spec.ServerState{OrderedAbsent: true}, spec.ServerCreatePrivilegeConsumable, true},
		{"barrier", &spec.ServerState{UnreadReason: spec.MetadataUnreadBatchEffects}, spec.ServerCreatePrivilegeConsumable, true},
		{"unproven", &spec.ServerState{Read: true, Exists: false}, spec.ServerCreatePrivilegeUnproven, false},
	} {
		adj := spec.AdjudicateServerCreate(tc.state, tc.priv, tc.clean, false)
		stmt := mysqlCreateServerStatement(t, tc.state, &adj,
			[]spec.ServerOption{{Key: "host", ValueKind: "char"}, {Key: "password", ValueKind: "char"}})
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("%s: evaluate: %v", tc.name, err)
		}
		for _, f := range findings {
			blob := f.Message
			if f.Explanation != nil {
				blob += f.Explanation.Why + f.Explanation.Risk + f.Explanation.Suggestion
			}
			for k, v := range f.Metadata {
				if s, ok := v.(string); ok {
					blob += k + s
				}
			}
			for _, protected := range []string{"s1", "audited@%", "GRANT", "OPTIONS", "password", "PASSWORD", "'h'"} {
				if strings.Contains(blob, protected) {
					t.Fatalf("%s: protected detail %q leaked: %q", tc.name, protected, blob)
				}
			}
		}
		for _, g := range r.EvidenceGaps(stmt) {
			for _, f := range g.RequiredFacts {
				if strings.Contains(f, "s1") {
					t.Fatalf("%s: authored name leaked into gap facts: %v", tc.name, g.RequiredFacts)
				}
			}
		}
	}
}

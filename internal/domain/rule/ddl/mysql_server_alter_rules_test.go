// T26-SERVER-ALTER-EXECUTION-01: the audited MySQL ALTER SERVER rule —
// the finding set projects the shared adjudication, refusal is a
// blocker, the partial-update advisory is a separate scoped notice on
// A3/A4 only, and no authored name, OPTIONS content, or account
// reaches any emitted shape.
package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func mysqlAlterServerStatement(t *testing.T, state *spec.ServerState, adjudication *spec.ServerAlterAdjudication, options []spec.ServerOption) spec.Statement {
	t.Helper()
	return spec.Statement{
		Dialect: spec.DialectMySQL,
		Kind:    spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterServer,
			Server: &spec.ServerDeclaration{
				Kind:      spec.ServerKindAlter,
				Name:      "s1",
				NameIdent: true,
				Options:   options,
			},
		},
		Metadata: &spec.Metadata{Server: state, ServerAlter: adjudication},
	}
}

func mysqlAlterServerRuleUnderTest(t *testing.T) mysqlAlterServerRule {
	t.Helper()
	r, err := newMySQLAlterServerRule(policy.RulePolicy{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	return r.(mysqlAlterServerRule)
}

// The proven arm projects conditional_alter under the marker, owes no
// gaps, and emits main + advisory — the decided P2 public shape.
func TestMySQLAlterServerProven(t *testing.T) {
	r := mysqlAlterServerRuleUnderTest(t)
	adj := spec.AdjudicateServerAlter(
		&spec.ServerState{Name: "s1", OrderedPresent: true},
		spec.ServerAlterPrivilegeConsumable, true, false)
	stmt := mysqlAlterServerStatement(t, nil, &adj, []spec.ServerOption{{Key: "user", ValueKind: "char"}})
	if !r.AppliesTo(stmt) {
		t.Fatalf("audited alter must apply")
	}
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil || len(findings) != 2 {
		t.Fatalf("findings = %v err=%v, want main + advisory", findings, err)
	}
	if findings[0].Level != rule.LevelNotice || findings[0].Metadata["outcome"] != "conditional_alter" {
		t.Fatalf("proven finding = %+v, want notice/conditional_alter", findings[0])
	}
	if findings[0].Metadata["conditional_audit"] != string(spec.PrivilegeAuditModeObservedAccountNewSession) {
		t.Fatalf("conditional scope token missing: %+v", findings[0].Metadata)
	}
	if findings[1].Metadata["notice"] != "connection_members" {
		t.Fatalf("advisory = %+v, want connection_members", findings[1])
	}
	if findings[1].Message != "ALTER SERVER specifies a partial options update; stored and resulting connection member sets are unverified." {
		t.Fatalf("advisory text = %q, want the fixed contract wording", findings[1].Message)
	}
	// The advisory carries no outcome, marker, or gap.
	if _, ok := findings[1].Metadata["outcome"]; ok {
		t.Fatalf("advisory must carry no outcome: %+v", findings[1].Metadata)
	}
	if _, ok := findings[1].Metadata["conditional_audit"]; ok {
		t.Fatalf("advisory must carry no marker: %+v", findings[1].Metadata)
	}
	if got := gapFacts(r.EvidenceGaps(stmt)); len(got) != 0 {
		t.Fatalf("proven arm owes no gaps: %v", got)
	}
}

// The terminal refusal is a blocker that owes nothing and emits no
// advisory — the ordered absent alone carries the verdict.
func TestMySQLAlterServerRefusal(t *testing.T) {
	r := mysqlAlterServerRuleUnderTest(t)
	adj := spec.AdjudicateServerAlter(
		&spec.ServerState{Name: "s1", OrderedAbsent: true, OrderedAbsentConditional: true},
		spec.ServerAlterPrivilegeSuppressed, false, false)
	stmt := mysqlAlterServerStatement(t, &spec.ServerState{Name: "s1", OrderedAbsent: true}, &adj, nil)
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil || len(findings) != 1 {
		t.Fatalf("refusal findings = %v err=%v, want the blocker only", findings, err)
	}
	if findings[0].Level != rule.LevelBlocker || findings[0].Metadata["outcome"] != "refused_absent" {
		t.Fatalf("refusal finding = %+v, want blocker/refused_absent", findings[0])
	}
	if _, ok := findings[0].Metadata["conditional_audit"]; !ok {
		t.Fatalf("refusal on conditional absent must inherit the marker: %+v", findings[0].Metadata)
	}
	if got := gapFacts(r.EvidenceGaps(stmt)); len(got) != 0 {
		t.Fatalf("refusal owes nothing: %v", got)
	}
}

// The barrier arm owes exactly server.batch_effects and emits no
// advisory — regardless of the statement's own premise, and never a
// marker.
func TestMySQLAlterServerBarrier(t *testing.T) {
	r := mysqlAlterServerRuleUnderTest(t)
	adj := spec.AdjudicateServerAlter(
		&spec.ServerState{Name: "s1", UnreadReason: spec.MetadataUnreadBatchEffects},
		spec.ServerAlterPrivilegeConsumable, true, false)
	stmt := mysqlAlterServerStatement(t, &spec.ServerState{UnreadReason: spec.MetadataUnreadBatchEffects}, &adj, nil)
	facts := gapFacts(r.EvidenceGaps(stmt))
	if len(facts) != 1 || !hasFact(facts, "server.batch_effects") {
		t.Fatalf("barrier owes exactly batch_effects: %v", facts)
	}
	findings, _ := r.Evaluate(context.Background(), stmt)
	if len(findings) != 1 || findings[0].Metadata["outcome"] != "blocked_batch_effects" {
		t.Fatalf("barrier findings = %+v, want one blocked_batch_effects", findings)
	}
	if _, ok := findings[0].Metadata["conditional_audit"]; ok {
		t.Fatalf("barrier never carries the marker: %+v", findings[0].Metadata)
	}
}

// The unproven arm projects the adjudication's owed set — never a
// fixed list — and still emits the advisory.
func TestMySQLAlterServerUnprovenOwedSet(t *testing.T) {
	r := mysqlAlterServerRuleUnderTest(t)
	adj := spec.AdjudicateServerAlter(
		&spec.ServerState{Name: "s1", OrderedPresent: true},
		spec.ServerAlterPrivilegeUnproven, false, false)
	stmt := mysqlAlterServerStatement(t, &spec.ServerState{OrderedPresent: true}, &adj, nil)
	facts := gapFacts(r.EvidenceGaps(stmt))
	if !hasFact(facts, "server.alter_privilege") || !hasFact(facts, "server.alter_options") {
		t.Fatalf("unproven on ordered present must owe privilege+options: %v", facts)
	}
	if hasFact(facts, "server.existence") || hasFact(facts, "server.drop_privilege") || hasFact(facts, "server.create_privilege") {
		t.Fatalf("owed set must never carry existence or a sibling premise: %v", facts)
	}
	findings, _ := r.Evaluate(context.Background(), stmt)
	if len(findings) != 2 || findings[0].Metadata["outcome"] != "unverified" || findings[1].Metadata["notice"] != "connection_members" {
		t.Fatalf("unproven findings = %+v, want unverified + advisory", findings)
	}
}

// Marker provenance (contract §D axes): the rule projects
// ConditionalAudit verbatim — a consumable own premise marks only
// arms that read it; an ordered source marks its consumers; a
// shadowed source never marks.
func TestMySQLAlterServerConditionalMarkerProvenance(t *testing.T) {
	r := mysqlAlterServerRuleUnderTest(t)
	hasMarker := func(stmt spec.Statement) bool {
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil || len(findings) == 0 {
			t.Fatalf("evaluate = %v err=%v", findings, err)
		}
		_, ok := findings[0].Metadata["conditional_audit"]
		return ok
	}

	// Barrier + consumable: the shadowed premise never fed anything.
	barrierState := &spec.ServerState{Name: "s1", UnreadReason: spec.MetadataUnreadBatchEffects}
	barrierAdj := spec.AdjudicateServerAlter(barrierState, spec.ServerAlterPrivilegeConsumable, true, false)
	if hasMarker(mysqlAlterServerStatement(t, barrierState, &barrierAdj, nil)) {
		t.Fatalf("barrier + consumable must not carry conditional_audit")
	}
	// A2 on unconditional absent + consumable: own premise never read.
	absAdj := spec.AdjudicateServerAlter(&spec.ServerState{Name: "s1", OrderedAbsent: true}, spec.ServerAlterPrivilegeConsumable, true, false)
	if hasMarker(mysqlAlterServerStatement(t, &spec.ServerState{Name: "s1", OrderedAbsent: true}, &absAdj, nil)) {
		t.Fatalf("refusal on unconditional absent must not mark — own premise is never read on A2")
	}
	// A4 catalog-present + consumable: own premise marks even though
	// the republished presence flag stays unconditional.
	catAdj := spec.AdjudicateServerAlter(&spec.ServerState{Name: "s1", Read: true, Exists: true, Status: spec.MetadataStatusConfirmed}, spec.ServerAlterPrivilegeConsumable, false, false)
	if !hasMarker(mysqlAlterServerStatement(t, nil, &catAdj, nil)) {
		t.Fatalf("consumable own premise must mark the unproven arm")
	}
	// A4 conditional ordered present + suppressed: inherited marker.
	condPresent := &spec.ServerState{Name: "s1", OrderedPresent: true, OrderedPresentConditional: true}
	inheritedAdj := spec.AdjudicateServerAlter(condPresent, spec.ServerAlterPrivilegeSuppressed, true, false)
	if !hasMarker(mysqlAlterServerStatement(t, condPresent, &inheritedAdj, nil)) {
		t.Fatalf("consumed conditional OrderedPresent must mark the finding")
	}
}

// The audited-slice gate: boundary statements, siblings, TiDB, and
// non-server DDL never reach this rule.
func TestMySQLAlterServerAppliesToNarrow(t *testing.T) {
	r := mysqlAlterServerRuleUnderTest(t)
	base := mysqlAlterServerStatement(t, nil, nil, nil)

	literal := base
	literal.DDL = &spec.DDL{Operation: spec.DDLOperationAlterServer, Server: &spec.ServerDeclaration{Kind: spec.ServerKindAlter, Name: "s1", NameIdent: false}}
	literal.Unsupported = &spec.UnsupportedDetail{Reason: spec.UnsupportedUnauditedReason}
	if r.AppliesTo(literal) {
		t.Fatalf("literal-named alter stays boundary — must not apply")
	}
	gated := base
	gated.Unsupported = &spec.UnsupportedDetail{Reason: spec.UnsupportedUnauditedReason, Metadata: map[string]any{"gate": "version"}}
	if r.AppliesTo(gated) {
		t.Fatalf("version-gated alter stays boundary — must not apply")
	}
	drop := base
	drop.DDL = &spec.DDL{Operation: spec.DDLOperationDropServer, Server: &spec.ServerDeclaration{Kind: spec.ServerKindDrop, Name: "s1", NameIdent: true}}
	if r.AppliesTo(drop) {
		t.Fatalf("drop server is a different rule — must not apply")
	}
	create := base
	create.DDL = &spec.DDL{Operation: spec.DDLOperationCreateServer, Server: &spec.ServerDeclaration{Kind: spec.ServerKindCreate, Name: "s1", NameIdent: true, Wrapper: "mysql"}}
	if r.AppliesTo(create) {
		t.Fatalf("create server is a different rule — must not apply")
	}
	tidb := base
	tidb.Dialect = spec.DialectTiDB
	if r.AppliesTo(tidb) {
		t.Fatalf("tidb is a vendor boundary — must not apply")
	}
}

// No authored name, OPTIONS content, account, or grant text reaches
// any emitted shape across every arm.
func TestMySQLAlterServerNoLeak(t *testing.T) {
	r := mysqlAlterServerRuleUnderTest(t)
	for _, tc := range []struct {
		name  string
		state *spec.ServerState
		priv  spec.ServerAlterPrivilege
		clean bool
	}{
		{"refusal", &spec.ServerState{OrderedAbsent: true}, spec.ServerAlterPrivilegeUnproven, false},
		{"proven", &spec.ServerState{OrderedPresent: true}, spec.ServerAlterPrivilegeConsumable, true},
		{"barrier", &spec.ServerState{UnreadReason: spec.MetadataUnreadBatchEffects}, spec.ServerAlterPrivilegeConsumable, true},
		{"unproven", &spec.ServerState{Read: true, Exists: false}, spec.ServerAlterPrivilegeUnproven, false},
	} {
		adj := spec.AdjudicateServerAlter(tc.state, tc.priv, tc.clean, false)
		stmt := mysqlAlterServerStatement(t, tc.state, &adj,
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
			for _, protected := range []string{"s1", "audited@%", "GRANT", "password", "PASSWORD", "'h'"} {
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

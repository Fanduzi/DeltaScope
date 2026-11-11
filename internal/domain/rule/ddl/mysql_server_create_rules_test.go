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
	if strings.Contains(notice.Message, "s1") {
		t.Fatalf("authored name leaked into the notice: %q", notice.Message)
	}
	// Complete members emit no second finding.
	complete := mysqlCreateServerStatement(t, nil, &adj, fullCreateOptions())
	findings, _ = r.Evaluate(context.Background(), complete)
	if len(findings) != 1 {
		t.Fatalf("complete members = 1 finding, got %+v", findings)
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

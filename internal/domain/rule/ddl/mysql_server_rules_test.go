// T26-SERVER-DROP-01: the audited MySQL DROP SERVER rule — one notice
// per audited statement, bounded gap vocabulary, no authored names in
// any emitted shape, IF EXISTS never publishing absent.
package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func mysqlDropServerStatement(t *testing.T, state *spec.ServerState, ifExists bool) spec.Statement {
	t.Helper()
	stmt := spec.Statement{
		Dialect: spec.DialectMySQL,
		Kind:    spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationDropServer,
			Server: &spec.ServerDeclaration{
				Kind:      spec.ServerKindDrop,
				Name:      "s1",
				NameIdent: true,
				IfExists:  ifExists,
			},
		},
	}
	if state != nil {
		stmt.Metadata = &spec.Metadata{Server: state}
	}
	return stmt
}

func mysqlDropServerRuleUnderTest(t *testing.T) mysqlDropServerRule {
	t.Helper()
	r, err := newMySQLDropServerRule(policy.RulePolicy{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	return r.(mysqlDropServerRule)
}

func gapFacts(gaps []rule.EvidenceGap) []string {
	var facts []string
	for _, g := range gaps {
		facts = append(facts, g.RequiredFacts...)
	}
	return facts
}

func hasFact(facts []string, want string) bool {
	for _, f := range facts {
		if f == want {
			return true
		}
	}
	return false
}

// Proven-present existence keeps the notice and owes only the drop
// privilege — the catalog answered, the execution premise did not.
func TestMySQLDropServerProvenPresent(t *testing.T) {
	r := mysqlDropServerRuleUnderTest(t)
	stmt := mysqlDropServerStatement(t, &spec.ServerState{Name: "s1", Read: true, Exists: true, Status: spec.MetadataStatusConfirmed}, false)
	if !r.AppliesTo(stmt) {
		t.Fatalf("audited drop must apply")
	}
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil || len(findings) != 1 {
		t.Fatalf("findings = %v err=%v, want one notice", findings, err)
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("level = %v, want notice", findings[0].Level)
	}
	facts := gapFacts(r.EvidenceGaps(stmt))
	if hasFact(facts, "server.existence") || hasFact(facts, "server.batch_effects") {
		t.Fatalf("proven present must not owe existence gaps: %v", facts)
	}
	if !hasFact(facts, "server.drop_privilege") {
		t.Fatalf("drop privilege must stay owed: %v", facts)
	}
}

// A zero-row read is bound-unknown — existence and privilege both owed,
// and nothing publishes absent.
func TestMySQLDropServerZeroRowUnknown(t *testing.T) {
	r := mysqlDropServerRuleUnderTest(t)
	stmt := mysqlDropServerStatement(t, &spec.ServerState{Name: "s1", Read: true, Exists: false}, false)
	facts := gapFacts(r.EvidenceGaps(stmt))
	if !hasFact(facts, "server.existence") || !hasFact(facts, "server.drop_privilege") {
		t.Fatalf("zero-row must owe existence+privilege: %v", facts)
	}
	findings, _ := r.Evaluate(context.Background(), stmt)
	for _, f := range findings {
		if strings.Contains(f.Message, "s1") || strings.Contains(strings.ToLower(f.Message), "absent") {
			t.Fatalf("finding must carry neither the authored name nor an absent verdict: %q", f.Message)
		}
	}
}

// The ordered-channel barrier replaces the existence gap with the
// batch-effects gap — the catalog answer was shadowed, not absent.
func TestMySQLDropServerBatchEffectsBarrier(t *testing.T) {
	r := mysqlDropServerRuleUnderTest(t)
	stmt := mysqlDropServerStatement(t, &spec.ServerState{Name: "s1", UnreadReason: spec.MetadataUnreadBatchEffects}, false)
	facts := gapFacts(r.EvidenceGaps(stmt))
	if !hasFact(facts, "server.batch_effects") || !hasFact(facts, "server.drop_privilege") {
		t.Fatalf("barrier path must owe batch_effects+privilege: %v", facts)
	}
	if hasFact(facts, "server.existence") {
		t.Fatalf("batch_effects replaces the existence gap: %v", facts)
	}
}

// An unbound state (no provider) still owes existence — unbound is
// never absent.
func TestMySQLDropServerUnboundStillOwesExistence(t *testing.T) {
	r := mysqlDropServerRuleUnderTest(t)
	facts := gapFacts(r.EvidenceGaps(mysqlDropServerStatement(t, nil, false)))
	if !hasFact(facts, "server.existence") || !hasFact(facts, "server.drop_privilege") {
		t.Fatalf("unbound must owe existence+privilege: %v", facts)
	}
}

// IF EXISTS on an unproven identity emits the same notice+gap shape —
// the guard skips the error, it never licenses an absent finding.
func TestMySQLDropServerIfExistsNeverPublishesAbsent(t *testing.T) {
	r := mysqlDropServerRuleUnderTest(t)
	stmt := mysqlDropServerStatement(t, &spec.ServerState{Name: "s1", Read: true, Exists: false}, true)
	findings, _ := r.Evaluate(context.Background(), stmt)
	if len(findings) != 1 || findings[0].Metadata["if_exists"] != true {
		t.Fatalf("if_exists flag must surface on the notice: %+v", findings)
	}
	facts := gapFacts(r.EvidenceGaps(stmt))
	if !hasFact(facts, "server.existence") || !hasFact(facts, "server.drop_privilege") {
		t.Fatalf("if-exists unproven must still owe existence+privilege: %v", facts)
	}
}

// The audited-slice gate: boundary statements (literal names,
// CREATE/ALTER, TiDB, non-server DDL) never reach this rule.
func TestMySQLDropServerAppliesToNarrow(t *testing.T) {
	r := mysqlDropServerRuleUnderTest(t)
	base := mysqlDropServerStatement(t, nil, false)

	literal := base
	literal.DDL = &spec.DDL{Operation: spec.DDLOperationDropServer, Server: &spec.ServerDeclaration{Kind: spec.ServerKindDrop, Name: "s1", NameIdent: false}}
	literal.Unsupported = &spec.UnsupportedDetail{Reason: spec.UnsupportedUnauditedReason}
	if r.AppliesTo(literal) {
		t.Fatalf("literal-named drop stays boundary — must not apply")
	}

	create := base
	create.DDL = &spec.DDL{Operation: spec.DDLOperationCreateServer, Server: &spec.ServerDeclaration{Kind: spec.ServerKindCreate, Name: "s1", NameIdent: true}}
	if r.AppliesTo(create) {
		t.Fatalf("create server stays boundary — must not apply")
	}

	tidb := base
	tidb.Dialect = spec.DialectTiDB
	if r.AppliesTo(tidb) {
		t.Fatalf("tidb is a vendor boundary — must not apply")
	}
}

// No authored name reaches any emitted shape — finding metadata, gap
// facts, and messages stay free of the server identity.
func TestMySQLDropServerNoAuthoredNameLeak(t *testing.T) {
	r := mysqlDropServerRuleUnderTest(t)
	stmt := mysqlDropServerStatement(t, &spec.ServerState{Name: "s1", Read: true, Exists: true, Status: spec.MetadataStatusConfirmed}, false)
	findings, _ := r.Evaluate(context.Background(), stmt)
	for _, f := range findings {
		blob := f.Message
		for k, v := range f.Metadata {
			if s, ok := v.(string); ok {
				blob += k + s
			}
		}
		if strings.Contains(blob, "s1") {
			t.Fatalf("authored name leaked into finding: %+v", f)
		}
	}
	for _, g := range r.EvidenceGaps(stmt) {
		for _, f := range g.RequiredFacts {
			if strings.Contains(f, "s1") {
				t.Fatalf("authored name leaked into gap facts: %v", g.RequiredFacts)
			}
		}
	}
}

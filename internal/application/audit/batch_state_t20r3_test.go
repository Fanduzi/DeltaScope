package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T20-R3/#99: two contract corrections verified at the public Audit path.
//
//  1. Topology gap: #99 requires "拓扑未知给gap而非猜测可部署" — a proven
//     applied bind or policy ALTER that reassigns live physical placement
//     must carry a structured evidence gap while the product holds no
//     topology facts (labels/stores/scheduling). The warn rule keeps its
//     binding-risk finding; the gap travels with RequiredFacts
//     placement_policy.topology under the stable unknown_table_state code.
//     CREATE/DROP policy and DEFAULT unbinds never claim deployability, so
//     they carry no topology premise.
//  2. Missing-policy refusal: #99 requires a proven-absent policy
//     reference to surface the corresponding error, not a silent pass.
//     ddl.tidb.placement.exists.require is the catalog-premise check —
//     blocker on a proven refusal (8239 missing reference, 8241 bound
//     drop, create-on-existing), catalog/bound_targets gaps on unvouched.
//     Both rules attribute only the premise actually missing (cell I),
//     and an applied CREATE OR REPLACE on a proven-existing policy is
//     topology-relevant the same way ALTER POLICY is (cell J).
func TestT20R3TopologyGapAndMissingRefusal(t *testing.T) {
	existsTable := &spec.TableSnapshot{Options: map[string]string{"cache_status": "disable"}, Exists: true, Table: &spec.Table{Schema: "golden", Name: "t"}}
	boundProvider := func() *t10Provider {
		return &t10Provider{
			version: "8.0.11-TiDB-v8.5.0",
			tables:  map[string]*spec.TableSnapshot{"t": existsTable},
			placement: map[string]*spec.PlacementPolicyFact{
				"p":       {BoundTargets: []string{}},
				"p_bound": {BoundTargets: []string{"table:golden.t"}},
			},
		}
	}
	audit := func(t *testing.T, provider *t10Provider, policy string, sql string) report.Result {
		t.Helper()
		result, err := AuditSQL(context.Background(), Request{
			SQL:              sql,
			Dialect:          spec.DialectTiDB,
			Schema:           "golden",
			ConfigPath:       policy,
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit error for %q: %v", sql, err)
		}
		return result
	}
	gapsOf := func(result report.Result, stmt int) []rule.EvidenceGap {
		return result.Statements[stmt].EvidenceGaps
	}
	findingsByRule := func(result report.Result, stmt int, id string) []rule.Finding {
		var out []rule.Finding
		for _, f := range result.Statements[stmt].Findings {
			if f.RuleID == id {
				out = append(out, f)
			}
		}
		return out
	}
	hasGap := func(result report.Result, stmt int, ruleID string, fact string) bool {
		for _, g := range gapsOf(result, stmt) {
			if g.RuleID != ruleID {
				continue
			}
			for _, f := range g.RequiredFacts {
				if f == fact {
					return true
				}
			}
		}
		return false
	}

	bothRules := t05PolicyPathConfigured(t, map[string]t05RuleConfig{
		"ddl.tidb.placement.warn":           {level: "warning"},
		"ddl.tidb.placement.exists.require": {level: "blocker"},
	})
	warnOff := t05PolicyPathConfigured(t, map[string]t05RuleConfig{
		"ddl.tidb.placement.exists.require": {level: "blocker"},
	})

	// A: proven bind + no topology facts → warn finding AND topology gap;
	// coverage drops to unverified instead of implying verified deployability.
	t.Run("proven bind warns and gaps topology", func(t *testing.T) {
		result := audit(t, boundProvider(), bothRules, "ALTER TABLE t PLACEMENT POLICY=p;")
		if len(findingsByRule(result, 0, "ddl.tidb.placement.warn")) != 1 {
			t.Fatalf("binding risk warning must survive, got %#v", result.Statements[0].Findings)
		}
		if !hasGap(result, 0, "ddl.tidb.placement.warn", "placement_policy.topology") {
			t.Fatalf("topology-unknown bind must report a placement_policy.topology gap, got %#v", gapsOf(result, 0))
		}
		if result.Statements[0].Coverage.Status != report.CoverageUnverified {
			t.Fatalf("unproven topology must lower coverage to unverified, got %s", result.Statements[0].Coverage.Status)
		}
		if len(findingsByRule(result, 0, "ddl.tidb.placement.exists.require")) != 0 {
			t.Fatalf("proven policy must not draw an existence finding, got %#v", result.Statements[0].Findings)
		}
	})

	// B: warn rule disabled → no warn finding and no topology gap; the
	// ordered-state binding still publishes (proved by the bound-drop
	// refusal the exists-require rule reports on the next statement).
	t.Run("disabled warn emits nothing and state still binds", func(t *testing.T) {
		result := audit(t, boundProvider(), warnOff,
			"ALTER TABLE t PLACEMENT POLICY=p; DROP PLACEMENT POLICY p;")
		if len(gapsOf(result, 0)) != 0 {
			t.Fatalf("disabled warn must not fabricate gaps, got %#v", gapsOf(result, 0))
		}
		found := findingsByRule(result, 1, "ddl.tidb.placement.exists.require")
		if len(found) != 1 {
			t.Fatalf("bound drop must refuse via exists.require (bind published despite disabled warn), got %#v", result.Statements[1].Findings)
		}
	})

	// C: unvouched catalog → per-rule catalog gaps with accurate ownership;
	// no topology gap stacks on an unproven premise.
	t.Run("unvouched catalog reports catalog gaps only", func(t *testing.T) {
		provider := boundProvider()
		provider.placement = nil // loader answers unvouched
		result := audit(t, provider, bothRules, "ALTER TABLE t PLACEMENT POLICY=p;")
		if !hasGap(result, 0, "ddl.tidb.placement.warn", "placement_policy.catalog") {
			t.Fatalf("warn must own its catalog gap, got %#v", gapsOf(result, 0))
		}
		if !hasGap(result, 0, "ddl.tidb.placement.exists.require", "placement_policy.catalog") {
			t.Fatalf("exists.require must own its catalog gap, got %#v", gapsOf(result, 0))
		}
		if hasGap(result, 0, "ddl.tidb.placement.warn", "placement_policy.topology") {
			t.Fatalf("unproven bind must not stack a topology gap, got %#v", gapsOf(result, 0))
		}
		if len(result.Statements[0].Findings) != 0 {
			t.Fatalf("unvouched premise must stay finding-free, got %#v", result.Statements[0].Findings)
		}
	})

	// D: proven-absent policy → exists.require blocker (the #99 对应错误);
	// no topology gap (nothing deploys); the refused rebind keeps the
	// prior binding — proven by the following bound-drop refusal.
	t.Run("missing policy blocks and keeps prior binding", func(t *testing.T) {
		result := audit(t, boundProvider(), bothRules,
			"ALTER TABLE t PLACEMENT POLICY=ghost; DROP PLACEMENT POLICY p_bound;")
		found := findingsByRule(result, 0, "ddl.tidb.placement.exists.require")
		if len(found) != 1 || found[0].Level != rule.LevelBlocker {
			t.Fatalf("proven-absent policy must block, got %#v", result.Statements[0].Findings)
		}
		if len(gapsOf(result, 0)) != 0 {
			t.Fatalf("a proven refusal must not also gap, got %#v", gapsOf(result, 0))
		}
		// The first bind refused (ghost absent) → t stays bound to p_bound
		// via the provider's proven bound-targets; dropping it must refuse.
		found = findingsByRule(result, 1, "ddl.tidb.placement.exists.require")
		if len(found) != 1 || found[0].Level != rule.LevelBlocker {
			t.Fatalf("bound-policy drop must block, got %#v", result.Statements[1].Findings)
		}
	})

	// E: lifecycle forms without a deployability premise carry no topology
	// gap — DEFAULT unbind, applied DROP, applied CREATE all stay gap-free;
	// applied ALTER POLICY (which rebinds live placement on bound objects)
	// does carry one.
	t.Run("only topology-relevant outcomes gap", func(t *testing.T) {
		result := audit(t, boundProvider(), bothRules,
			"ALTER TABLE t PLACEMENT POLICY=DEFAULT; "+
				"DROP PLACEMENT POLICY p; "+
				"CREATE PLACEMENT POLICY p_new PRIMARY_REGION='r1' REGIONS='r1' FOLLOWERS=1; "+
				"ALTER PLACEMENT POLICY p_bound FOLLOWERS=2;")
		for i, name := range []string{"default-unbind", "drop-policy", "create-policy"} {
			if hasGap(result, i, "ddl.tidb.placement.warn", "placement_policy.topology") {
				t.Fatalf("%s must not carry a topology gap, got %#v", name, gapsOf(result, i))
			}
		}
		if len(findingsByRule(result, 3, "ddl.tidb.placement.warn")) != 1 {
			t.Fatalf("applied alter-policy must still warn, got %#v", result.Statements[3].Findings)
		}
		if !hasGap(result, 3, "ddl.tidb.placement.warn", "placement_policy.topology") {
			t.Fatalf("applied alter-policy rebinds live placement — topology gap required, got %#v", gapsOf(result, 3))
		}
	})

	// F: provider catalog failure stays fail-loud — no error is converted
	// into a gap or a fabricated answer.
	t.Run("catalog error propagates fail-loud", func(t *testing.T) {
		provider := boundProvider()
		provider.placementErr = errors.New("catalog unreachable")
		_, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t PLACEMENT POLICY=p;",
			Dialect:          spec.DialectTiDB,
			Schema:           "golden",
			ConfigPath:       bothRules,
			MetadataProvider: provider,
		})
		if err == nil {
			t.Fatal("catalog failure must propagate, got nil error")
		}
	})

	// G: the bound-drop premise — a proven-bound policy refuses the drop
	// even when the policy exists (8241 parity), and an applied drop of an
	// unbound policy stays finding-free for exists.require.
	t.Run("drop premise distinguishes bound from unbound", func(t *testing.T) {
		result := audit(t, boundProvider(), bothRules, "DROP PLACEMENT POLICY p;")
		if len(findingsByRule(result, 0, "ddl.tidb.placement.exists.require")) != 0 {
			t.Fatalf("unbound existing policy drop must pass the premise check, got %#v", result.Statements[0].Findings)
		}
	})

	// H: if_exists disarms the drop's existence premise — a proven-absent
	// policy under IF EXISTS is a native no-op, not a violation.
	t.Run("if_exists disarms the missing-policy premise", func(t *testing.T) {
		result := audit(t, boundProvider(), bothRules, "DROP PLACEMENT POLICY IF EXISTS ghost;")
		if len(findingsByRule(result, 0, "ddl.tidb.placement.exists.require")) != 0 {
			t.Fatalf("IF EXISTS must not fire on a proven-absent policy, got %#v", result.Statements[0].Findings)
		}
	})

	gapFactsOf := func(result report.Result, stmt int, ruleID string) []string {
		var facts []string
		for _, g := range result.Statements[stmt].EvidenceGaps {
			if g.RuleID != ruleID {
				continue
			}
			facts = append(facts, g.RequiredFacts...)
		}
		return facts
	}

	// I: drop premise attribution — a proven-existing policy whose bound
	// set is unvouched owes bound_targets alone; an unvouched catalog owes
	// catalog+bound_targets (existence decides whether bound is even
	// consultable). Neither rule may claim a held fact as missing.
	t.Run("drop gaps attribute only the missing premise", func(t *testing.T) {
		provider := boundProvider()
		provider.placement["p_nobounds"] = &spec.PlacementPolicyFact{BoundTargets: nil}
		result := audit(t, provider, bothRules, "DROP PLACEMENT POLICY p_nobounds;")
		for _, id := range []string{"ddl.tidb.placement.warn", "ddl.tidb.placement.exists.require"} {
			facts := gapFactsOf(result, 0, id)
			if len(facts) != 1 || facts[0] != "placement_policy.bound_targets" {
				t.Fatalf("%s must owe bound_targets only on a proven-existing policy, got %v", id, facts)
			}
		}
		provider.placement = nil
		result = audit(t, provider, bothRules, "DROP PLACEMENT POLICY p;")
		for _, id := range []string{"ddl.tidb.placement.warn", "ddl.tidb.placement.exists.require"} {
			facts := gapFactsOf(result, 0, id)
			if len(facts) != 2 || facts[0] != "placement_policy.catalog" || facts[1] != "placement_policy.bound_targets" {
				t.Fatalf("%s must owe catalog+bound_targets on an unvouched catalog, got %v", id, facts)
			}
		}
	})

	// J: CREATE OR REPLACE on a proven-existing policy rewrites the
	// definition every bound object deploys — the same live reassignment
	// as ALTER POLICY — so it carries the topology gap; on a proven-absent
	// policy it is a plain create with no bound readers and no premise.
	t.Run("or_replace create carries topology only when replacing", func(t *testing.T) {
		result := audit(t, boundProvider(), bothRules,
			"CREATE OR REPLACE PLACEMENT POLICY p FOLLOWERS=2; "+
				"CREATE OR REPLACE PLACEMENT POLICY p_fresh FOLLOWERS=2;")
		if len(findingsByRule(result, 0, "ddl.tidb.placement.warn")) != 1 {
			t.Fatalf("applied or_replace must still warn, got %#v", result.Statements[0].Findings)
		}
		if !hasGap(result, 0, "ddl.tidb.placement.warn", "placement_policy.topology") {
			t.Fatalf("or_replace on an existing policy reassigns live placement — topology gap required, got %#v", gapsOf(result, 0))
		}
		if len(findingsByRule(result, 1, "ddl.tidb.placement.warn")) != 1 {
			t.Fatalf("or_replace on a new policy must still warn, got %#v", result.Statements[1].Findings)
		}
		if hasGap(result, 1, "ddl.tidb.placement.warn", "placement_policy.topology") {
			t.Fatalf("or_replace on a new policy reassigns nothing — no topology gap, got %#v", gapsOf(result, 1))
		}
		if len(findingsByRule(result, 1, "ddl.tidb.placement.exists.require")) != 0 {
			t.Fatalf("or_replace on a proven-absent policy is a legal create, got %#v", result.Statements[1].Findings)
		}
	})

	// K: IF EXISTS disarms only the existence premise — the bound set
	// stays live because native 8241 ignores the clause. A proven-existing
	// policy with an unvouched bound set owes bound_targets on BOTH rules,
	// and a proven-bound one still blocks under IF EXISTS.
	t.Run("if_exists keeps the bound premise alive", func(t *testing.T) {
		provider := boundProvider()
		provider.placement["p_nobounds"] = &spec.PlacementPolicyFact{BoundTargets: nil}
		result := audit(t, provider, bothRules, "DROP PLACEMENT POLICY IF EXISTS p_nobounds;")
		for _, id := range []string{"ddl.tidb.placement.warn", "ddl.tidb.placement.exists.require"} {
			facts := gapFactsOf(result, 0, id)
			if len(facts) != 1 || facts[0] != "placement_policy.bound_targets" {
				t.Fatalf("%s must owe bound_targets under IF EXISTS too, got %v", id, facts)
			}
		}
		found := findingsByRule(result, 0, "ddl.tidb.placement.exists.require")
		if len(found) != 0 {
			t.Fatalf("unproven drop must not fabricate a finding, got %#v", found)
		}
		result = audit(t, boundProvider(), bothRules, "DROP PLACEMENT POLICY IF EXISTS p_bound;")
		found = findingsByRule(result, 0, "ddl.tidb.placement.exists.require")
		if len(found) != 1 || found[0].Level != rule.LevelBlocker {
			t.Fatalf("proven-bound policy must still block under IF EXISTS (8241 ignores the clause), got %#v", result.Statements[0].Findings)
		}
	})

	// L: a table-level = DEFAULT is an unbind, not a binding — the shared
	// scan must keep looking so a named partition bind after it reaches
	// both rules identically (warn was previously silent here).
	t.Run("table-level DEFAULT does not hide a named partition bind", func(t *testing.T) {
		sqlText := "CREATE TABLE nt (id INT) PLACEMENT POLICY = DEFAULT PARTITION BY RANGE (id) " +
			"(PARTITION p0 VALUES LESS THAN (10) PLACEMENT POLICY = ghost);"
		result := audit(t, boundProvider(), bothRules, sqlText)
		found := findingsByRule(result, 0, "ddl.tidb.placement.exists.require")
		if len(found) != 1 || found[0].Level != rule.LevelBlocker {
			t.Fatalf("named partition bind to a proven-absent policy must block, got %#v", result.Statements[0].Findings)
		}
		sqlText = "CREATE TABLE nt (id INT) PLACEMENT POLICY = DEFAULT PARTITION BY RANGE (id) " +
			"(PARTITION p0 VALUES LESS THAN (10) PLACEMENT POLICY = p);"
		result = audit(t, boundProvider(), bothRules, sqlText)
		if len(findingsByRule(result, 0, "ddl.tidb.placement.warn")) != 1 {
			t.Fatalf("the partition bind must reach the warn rule too, got %#v", result.Statements[0].Findings)
		}
		if !hasGap(result, 0, "ddl.tidb.placement.warn", "placement_policy.topology") {
			t.Fatalf("applied partition bind owes the topology gap, got %#v", gapsOf(result, 0))
		}
	})

	// M: OR REPLACE over an unvouched catalog — the create lands and warns,
	// but whether bound objects were rewritten is undecidable: the warn
	// rule owes the catalog fact rather than staying silent on the premise.
	t.Run("or_replace over unvouched catalog owes the catalog gap", func(t *testing.T) {
		provider := boundProvider()
		provider.placement = nil
		result := audit(t, provider, bothRules, "CREATE OR REPLACE PLACEMENT POLICY p FOLLOWERS=2;")
		if len(findingsByRule(result, 0, "ddl.tidb.placement.warn")) != 1 {
			t.Fatalf("or_replace still lands a policy — warn must fire, got %#v", result.Statements[0].Findings)
		}
		if !hasGap(result, 0, "ddl.tidb.placement.warn", "placement_policy.catalog") {
			t.Fatalf("bound-rewrite relevance is undecidable — warn owes the catalog gap, got %#v", gapsOf(result, 0))
		}
	})

	// N: a fresh create over a tombstone starts with a proven-empty bound
	// set — a later drop applies cleanly instead of over-gapping.
	t.Run("fresh create resets the bound set to proven-empty", func(t *testing.T) {
		result := audit(t, boundProvider(), bothRules,
			"DROP PLACEMENT POLICY p; "+
				"CREATE PLACEMENT POLICY p PRIMARY_REGION='r1' REGIONS='r1' FOLLOWERS=1; "+
				"DROP PLACEMENT POLICY p;")
		if len(findingsByRule(result, 2, "ddl.tidb.placement.exists.require")) != 0 {
			t.Fatalf("drop of a freshly-created policy must not block, got %#v", result.Statements[2].Findings)
		}
		if facts := gapFactsOf(result, 2, "ddl.tidb.placement.warn"); len(facts) != 0 {
			t.Fatalf("fresh policy's bound set is proven-empty — no bound_targets gap owed, got %v", facts)
		}
		if len(findingsByRule(result, 2, "ddl.tidb.placement.warn")) != 1 {
			t.Fatalf("proven drop must still warn, got %#v", result.Statements[2].Findings)
		}
	})

	// O: OR REPLACE under an unvouched catalog lands the policy but its
	// bound set stays unvouched — the replace may have rewritten live
	// bindings. The following DROP must stay unproven (gaps, not a
	// silent apply), and a different policy reference must gap rather
	// than fabricate a proven-absent blocker (#99 invariant).
	t.Run("or_replace under unvouched catalog fabricates nothing", func(t *testing.T) {
		provider := boundProvider()
		provider.placement = nil
		result := audit(t, provider, bothRules,
			"CREATE OR REPLACE PLACEMENT POLICY p FOLLOWERS=2; "+
				"DROP PLACEMENT POLICY p; "+
				"ALTER TABLE t PLACEMENT POLICY=q;")
		// The drop cannot be proven unbound: the replaced policy may have
		// carried live bindings — both rules owe the premise facts.
		for _, id := range []string{"ddl.tidb.placement.warn", "ddl.tidb.placement.exists.require"} {
			facts := gapFactsOf(result, 1, id)
			if len(facts) != 2 || facts[0] != "placement_policy.catalog" || facts[1] != "placement_policy.bound_targets" {
				t.Fatalf("%s must owe catalog+bound_targets on the unproven drop, got %v", id, facts)
			}
		}
		if len(findingsByRule(result, 1, "ddl.tidb.placement.warn")) != 0 {
			t.Fatalf("unproven drop must not claim it removed a policy, got %#v", result.Statements[1].Findings)
		}
		// A different policy name under an unvouched catalog is unproven,
		// never proven-absent — gaps, not a fabricated blocker.
		if len(findingsByRule(result, 2, "ddl.tidb.placement.exists.require")) != 0 {
			t.Fatalf("unvouched catalog must not fabricate a missing-policy blocker, got %#v", result.Statements[2].Findings)
		}
		for _, id := range []string{"ddl.tidb.placement.warn", "ddl.tidb.placement.exists.require"} {
			if !hasGap(result, 2, id, "placement_policy.catalog") {
				t.Fatalf("%s must owe the catalog gap on the unproven bind, got %#v", id, gapsOf(result, 2))
			}
		}
	})

	// P: a tombstoned policy's bound set is proven-empty (a nonexistent
	// object binds nothing) — a bare re-drop adjudicates refused
	// (missing-policy blocker), never unproven.
	t.Run("re-drop of a tombstone refuses instead of gapping", func(t *testing.T) {
		result := audit(t, boundProvider(), bothRules,
			"DROP PLACEMENT POLICY p; DROP PLACEMENT POLICY p;")
		found := findingsByRule(result, 1, "ddl.tidb.placement.exists.require")
		if len(found) != 1 || found[0].Level != rule.LevelBlocker {
			t.Fatalf("bare re-drop of a proven-absent policy must block (8239 parity), got %#v", result.Statements[1].Findings)
		}
	})
}

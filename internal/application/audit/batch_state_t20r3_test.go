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
func TestT20R3TopologyGapAndMissingRefusal(t *testing.T) {
	existsTable := &spec.TableSnapshot{Exists: true, Table: &spec.Table{Schema: "golden", Name: "t"}}
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
}

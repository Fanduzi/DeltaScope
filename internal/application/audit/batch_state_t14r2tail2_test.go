// Package audit verifies the T14-R2-tail2 control-flow defect: inside
// declaredSchemeOutcome's RANGE branch the per-member comparability
// check and the pairwise adjudication were joined by else-if, so a
// member after a proven MAXVALUE edge skipped the pair check entirely —
// LessThanPairOutcome proves that refusal without the successor's own
// comparability. The ALTER and CREATE counterexamples run the public
// AuditSQL seam; snapshot assertions are marked internal.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// ALTER arm — MAXVALUE followed by an expression member is a proven
// refusal: the partition_bound_violation finding stays, the expression
// capability aspect stays, and the whole statement preserves the exact
// pre-state so the successor still binds the original directory.
func TestT14R2Tail2MaxvalueThenExprRefusesAlter(t *testing.T) {
	t.Parallel()

	sql := `ALTER TABLE t PARTITION BY RANGE(id)
(PARTITION qmax VALUES LESS THAN (MAXVALUE), PARTITION qexpr VALUES LESS THAN (1+0));
ALTER TABLE t TRUNCATE PARTITION p0;`
	provider := t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r2RangeTailShape("t")})
	rules := map[string]t05RuleConfig{
		t14r1RulePByApplies:   {level: "blocker"},
		t14RuleTruncateTarget: {level: "blocker"},
	}

	result := t14Audit(t, spec.DialectMySQL, sql, provider, rules)
	if len(result.Statements) != 2 {
		t.Fatalf("expected two statements, got %d", len(result.Statements))
	}
	findings := t05FindingsByRule(result, 0, t14r1RulePByApplies)
	if len(findings) != 1 || findings[0].Metadata["reason_code"] != "partition_bound_violation" {
		t.Fatalf("a member after MAXVALUE is a provable bound violation, got %#v", result.Statements[0].Findings)
	}
	if !t14r1HasFeature(result, "alter_table.partition_by.partition_bound") {
		t.Fatalf("the expression bound keeps its capability aspect, got %+v", result.Unsupported)
	}
	trunc := result.Statements[1]
	if got := t05FindingsByRule(result, 1, t14RuleTruncateTarget); len(got) != 0 {
		t.Fatalf("p0 must still bind on the intact pre-state, got %#v", got)
	}
	if len(trunc.EvidenceGaps) != 0 {
		t.Fatalf("a proven refusal must not degrade into successor gaps, got %+v", trunc.EvidenceGaps)
	}

	// Internal: the successor's pre-state is the exact pre-state —
	// original directory, partition method, and data statistics kept.
	enriched := t14r1Enrich(t, spec.DialectMySQL, sql, provider)
	snapshot := t14r1TargetSnapshot(enriched, 1)
	if snapshot == nil || !snapshot.Exists {
		t.Fatalf("the refused PARTITION BY keeps the table, got %+v", snapshot)
	}
	if snapshot.Partitioned == nil || !*snapshot.Partitioned || snapshot.PartitionType == nil || *snapshot.PartitionType != "range" {
		t.Fatalf("the proven scheme facts survive the refusal, got partitioned=%v type=%v", snapshot.Partitioned, snapshot.PartitionType)
	}
	if len(snapshot.PartitionNames) != 2 || snapshot.PartitionNames[0] != "p0" || snapshot.PartitionNames[1] != "p1" {
		t.Fatalf("the original directory survives verbatim, got %+v", snapshot.PartitionNames)
	}
	if snapshot.Options["table_rows"] != "9" {
		t.Fatalf("a proven refusal preserves data statistics, got %+v", snapshot.Options)
	}
}

// CREATE arm — the same pair on a provider-proven-absent target: a
// proven MAXVALUE refusal keeps the entry known-absent; it must not
// unvouch into an unknown successor premise.
func TestT14R2Tail2MaxvalueThenExprRefusesCreate(t *testing.T) {
	t.Parallel()

	sql := `CREATE TABLE c2 (id INT) PARTITION BY RANGE(id)
(PARTITION qmax VALUES LESS THAN (MAXVALUE), PARTITION qexpr VALUES LESS THAN (1+0));
ALTER TABLE c2 TRUNCATE PARTITION p0;`
	provider := t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{})

	result := t14Audit(t, spec.DialectMySQL, sql, provider,
		map[string]t05RuleConfig{t14RuleTruncateTarget: {level: "blocker"}})
	if len(result.Statements) != 2 {
		t.Fatalf("expected two statements, got %d", len(result.Statements))
	}
	if !t14r1HasFeature(result, "create_table.partition_bound") {
		t.Fatalf("the expression bound keeps its capability aspect, got %+v", result.Unsupported)
	}
	trunc := result.Statements[1]
	if len(trunc.EvidenceGaps) != 0 {
		t.Fatalf("a kept-absent entry owes no partition premise, got %+v", trunc.EvidenceGaps)
	}

	// Internal: proven-absent survives as Exists=false — distinct from
	// an unvouched (nil) snapshot.
	enriched := t14r1Enrich(t, spec.DialectMySQL, sql, provider)
	snapshot := t14r1TargetSnapshot(enriched, 1)
	if snapshot == nil {
		t.Fatal("a proven refusal on a known-absent target keeps the absence, not an unvouched entry")
	}
	if snapshot.Exists {
		t.Fatalf("the refused CREATE must not publish the table, got %+v", snapshot)
	}
}

// The refusal conclusion is a state fact, not a policy judgment:
// disabling every partition rule leaves the proven refusal intact.
func TestT14R2Tail2RuleOffKeepsRefusal(t *testing.T) {
	t.Parallel()

	sql := `ALTER TABLE t PARTITION BY RANGE(id)
(PARTITION qmax VALUES LESS THAN (MAXVALUE), PARTITION qexpr VALUES LESS THAN (1+0));
ALTER TABLE t TRUNCATE PARTITION p0;`
	provider := t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r2RangeTailShape("t")})
	enriched := t14r1Enrich(t, spec.DialectMySQL, sql, provider)
	snapshot := t14r1TargetSnapshot(enriched, 1)
	if snapshot == nil || len(snapshot.PartitionNames) != 2 {
		t.Fatalf("the refusal keeps the original directory regardless of rules, got %+v", snapshot)
	}
	result := t14Audit(t, spec.DialectMySQL, sql, provider, map[string]t05RuleConfig{})
	if len(result.Statements) != 2 {
		t.Fatalf("expected two statements, got %d", len(result.Statements))
	}
	if got := result.Statements[1].Findings; len(got) != 0 {
		t.Fatalf("no rule mints a finding on the preserved directory, got %#v", got)
	}
}

// Internal helper matrix — the pair check runs independently of the
// current member's own comparability: a proven MAXVALUE predecessor
// refuses whatever follows, a later provable violation survives an
// earlier undecidable member, and legal sequences stay combinable.
// Internal unit test — not the public seam.
func TestT14R2Tail2DeclaredSchemeOutcomePairs(t *testing.T) {
	t.Parallel()

	numBound := func(v string) *spec.PartitionBound {
		return &spec.PartitionBound{Kind: "less_than", Rows: [][]spec.BoundLiteral{{spec.BoundLiteral{Kind: spec.PartitionBoundNumeric, Value: v}}}}
	}
	maxBound := &spec.PartitionBound{Kind: "less_than", MaxValue: true}
	exprBound := &spec.PartitionBound{Kind: "less_than", Rows: [][]spec.BoundLiteral{{spec.BoundLiteral{Kind: spec.PartitionBoundExpression}}}}
	shellBound := &spec.PartitionBound{Kind: "less_than"}

	cases := []struct {
		name string
		defs []spec.PartitionDef
		want partitionOutcome
	}{
		{"maxvalue_then_expr", []spec.PartitionDef{{Name: "qmax", Bound: maxBound}, {Name: "qexpr", Bound: exprBound}}, partitionOutcomeRefused},
		{"maxvalue_then_nil", []spec.PartitionDef{{Name: "qmax", Bound: maxBound}, {Name: "qnil"}}, partitionOutcomeRefused},
		{"maxvalue_then_shell", []spec.PartitionDef{{Name: "qmax", Bound: maxBound}, {Name: "qsh", Bound: shellBound}}, partitionOutcomeRefused},
		{"expr_then_decreasing", []spec.PartitionDef{{Name: "q0", Bound: exprBound}, {Name: "q1", Bound: numBound("600")}, {Name: "q2", Bound: numBound("500")}}, partitionOutcomeRefused},
		{"single_expr", []spec.PartitionDef{{Name: "q0", Bound: exprBound}}, partitionOutcomeUnproven},
		{"legal_sequence", []spec.PartitionDef{{Name: "q0", Bound: numBound("200")}, {Name: "q1", Bound: numBound("400")}, {Name: "q2", Bound: maxBound}}, partitionOutcomeCombinable},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := declaredSchemeOutcome("range", tc.defs); got != tc.want {
				t.Fatalf("declaredSchemeOutcome(range) = %v, want %v", got, tc.want)
			}
		})
	}
}

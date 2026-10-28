// Package audit verifies the T14-R2-tail single-member defect: a
// declared partition scheme with exactly one member has no comparison
// pairs, so the per-member bound premise must still be adjudicated —
// an expression item, a nil bound, or an empty bound shell can never
// publish a fabricated member directory. The four minimal counterexamples
// run the public AuditSQL seam; snapshot assertions are marked internal.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// The lone-member RANGE scheme with an expression bound: the statement
// keeps its capability aspect while the ordered state must degrade the
// partition facts instead of publishing "fresh" as a proven member.
func TestT14R2TailSingleMemberAlterUndecidable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		method string
		sql    string
	}{
		{"range", "range", `ALTER TABLE t PARTITION BY RANGE(id) (PARTITION fresh VALUES LESS THAN (1+0));`},
		{"list", "list", `ALTER TABLE t PARTITION BY LIST(id) (PARTITION fresh VALUES IN (1+0));`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			shape := t14r2RangeTailShape("t")
			provider := t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": shape})

			// Public seam — current-statement capability aspect and the
			// successor's owed premises.
			result := t14Audit(t, spec.DialectMySQL, tc.sql+`
ALTER TABLE t TRUNCATE PARTITION fresh;`, provider,
				map[string]t05RuleConfig{t14RuleTruncateTarget: {level: "blocker"}})
			if len(result.Statements) != 2 {
				t.Fatalf("expected two statements, got %d", len(result.Statements))
			}
			if !t14r1HasFeature(result, "alter_table.partition_by.partition_bound") {
				t.Fatalf("the expression bound keeps its capability aspect, got %+v", result.Unsupported)
			}
			trunc := result.Statements[1]
			if got := t05FindingsByRule(result, 1, t14RuleTruncateTarget); len(got) != 0 {
				t.Fatalf("fresh must not bind on a fabricated directory, got %#v", got)
			}
			gaps := t14r1GapSet(trunc)
			if !gaps["target_table.partitions"] && !gaps["target_table.partitioned"] {
				t.Fatalf("the successor owes the actual unproven partition premise, got %+v", trunc.EvidenceGaps)
			}

			// Internal enrich seam — the successor's pre-state keeps the
			// unrelated structure while every partition fact is unvouched.
			enriched := t14r1Enrich(t, spec.DialectMySQL, tc.sql+`
ALTER TABLE t TRUNCATE PARTITION fresh;`, provider)
			snapshot := t14r1TargetSnapshot(enriched, 1)
			if snapshot == nil {
				t.Fatal("an undecidable scheme must not tombstone the table")
			}
			if !snapshot.Exists {
				t.Fatal("table existence survives the undecidable scheme")
			}
			if len(snapshot.Columns) != 1 || snapshot.Columns[0].Name != "id" {
				t.Fatalf("unrelated columns survive, got %+v", snapshot.Columns)
			}
			if snapshot.Partitioned != nil || snapshot.PartitionType != nil {
				t.Fatalf("the scheme facts are unvouched, got partitioned=%v type=%v", snapshot.Partitioned, snapshot.PartitionType)
			}
			if len(snapshot.PartitionNames) != 0 || len(snapshot.PartitionDefs) != 0 {
				t.Fatalf("fresh must not publish as a proven directory, got names=%v defs=%+v", snapshot.PartitionNames, snapshot.PartitionDefs)
			}
		})
	}
}

// The lone-member CREATE scheme with an expression bound on a
// provider-proven-absent target: the create premise is undecidable — the
// entry unvouches (no proven existence, no fabricated directory, no
// guessed absence) and the successor owes the real premise.
func TestT14R2TailSingleMemberCreateUndecidable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ddl  string
	}{
		{"range", `CREATE TABLE fresh (id INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (1+0));`},
		{"list", `CREATE TABLE fresh (id INT) PARTITION BY LIST(id) (PARTITION p0 VALUES IN (1+0));`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// "fresh" is absent from the provider map — a proven absence.
			provider := t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{})

			result := t14Audit(t, spec.DialectMySQL, tc.ddl+`
ALTER TABLE fresh TRUNCATE PARTITION p0;`, provider,
				map[string]t05RuleConfig{t14RuleTruncateTarget: {level: "blocker"}})
			if len(result.Statements) != 2 {
				t.Fatalf("expected two statements, got %d", len(result.Statements))
			}
			if !t14r1HasFeature(result, "create_table.partition_bound") {
				t.Fatalf("the expression bound keeps its capability aspect, got %+v", result.Unsupported)
			}
			trunc := result.Statements[1]
			if got := t05FindingsByRule(result, 1, t14RuleTruncateTarget); len(got) != 0 {
				t.Fatalf("p0 must not bind on a fabricated directory, got %#v", got)
			}
			gaps := t14r1GapSet(trunc)
			if len(gaps) == 0 {
				t.Fatalf("the successor owes the unvouched create premise, got %+v", trunc.EvidenceGaps)
			}

			// Internal: the invalidated entry yields no proven snapshot —
			// neither proven-present nor a guessed proven-absent.
			enriched := t14r1Enrich(t, spec.DialectMySQL, tc.ddl+`
ALTER TABLE fresh TRUNCATE PARTITION p0;`, provider)
			snapshot := t14r1TargetSnapshot(enriched, 1)
			if snapshot != nil && snapshot.Exists {
				t.Fatalf("an undecidable CREATE must not publish proven existence, got %+v", snapshot)
			}
			if snapshot != nil && len(snapshot.PartitionNames) != 0 {
				t.Fatalf("no fabricated member directory, got %+v", snapshot.PartitionNames)
			}
		})
	}
}

// Positive controls — a lone member still satisfies the method's
// decidability premise when its bound is a projected literal: the scheme
// publishes and the member binds for the successor.
func TestT14R2TailSingleMemberProvable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		sql  string
	}{
		{"range_numeric", `ALTER TABLE t PARTITION BY RANGE(id) (PARTITION fresh VALUES LESS THAN (500));`},
		{"range_maxvalue", `ALTER TABLE t PARTITION BY RANGE(id) (PARTITION fresh VALUES LESS THAN (MAXVALUE));`},
		{"range_columns_string", `ALTER TABLE t PARTITION BY RANGE COLUMNS(c) (PARTITION fresh VALUES LESS THAN ('a'));`},
		{"list_numeric", `ALTER TABLE t PARTITION BY LIST(id) (PARTITION fresh VALUES IN (3));`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			shape := t14r2RangeTailShape("t")
			if tc.name == "range_columns_string" {
				// The RANGE COLUMNS(c) declaration binds a real VARCHAR
				// member — the fixture must actually carry column c.
				shape = cloneTableSnapshot(shape)
				shape.Columns = append(shape.Columns, t10r2Col("c", "varchar", true, 20))
			}
			provider := t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": shape})
			enriched := t14r1Enrich(t, spec.DialectMySQL, tc.sql+`
ALTER TABLE t TRUNCATE PARTITION fresh;`, provider)
			snapshot := t14r1TargetSnapshot(enriched, 1)
			if snapshot == nil {
				t.Fatal("a provable single-member scheme must publish")
			}
			found := false
			for _, name := range snapshot.PartitionNames {
				if name == "fresh" {
					found = true
				}
			}
			if !found {
				t.Fatalf("fresh must publish as a proven member, got %+v", snapshot.PartitionNames)
			}
			if snapshot.Partitioned == nil || !*snapshot.Partitioned {
				t.Fatalf("the partitioned fact publishes, got %+v", snapshot.Partitioned)
			}
		})
	}
}

// Positive control — a provable refusal later in the def list survives an
// earlier undecidable member on the PARTITION BY path too: the whole
// statement refuses (pre-state intact) and the successor binds the
// original directory.
func TestT14R2TailExprThenProvableRefusal(t *testing.T) {
	t.Parallel()
	provider := t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r2RangeTailShape("t")})
	result := t14Audit(t, spec.DialectMySQL, `ALTER TABLE t PARTITION BY RANGE(id)
(PARTITION p0 VALUES LESS THAN (1+0), PARTITION p1 VALUES LESS THAN (600), PARTITION p2 VALUES LESS THAN (500));
ALTER TABLE t TRUNCATE PARTITION p0;`, provider,
		map[string]t05RuleConfig{t14r1RulePByApplies: {level: "blocker"}, t14RuleTruncateTarget: {level: "blocker"}})
	if len(result.Statements) != 2 {
		t.Fatalf("expected two statements, got %d", len(result.Statements))
	}
	findings := t05FindingsByRule(result, 0, t14r1RulePByApplies)
	if len(findings) != 1 || findings[0].Metadata["reason_code"] != "partition_bound_violation" {
		t.Fatalf("the later provable decrease must refuse the scheme, got %#v", result.Statements[0].Findings)
	}
	trunc := result.Statements[1]
	if got := t05FindingsByRule(result, 1, t14RuleTruncateTarget); len(got) != 0 {
		t.Fatalf("p0 must still bind after the refused PARTITION BY, got %#v", got)
	}
	if len(trunc.EvidenceGaps) != 0 {
		t.Fatalf("the successor reads the intact pre-state, got %+v", trunc.EvidenceGaps)
	}
}

// The undecidable conclusion is a state fact, not a policy judgment:
// disabling every partition rule leaves the degraded directory intact.
func TestT14R2TailRuleOffKeepsStateConclusion(t *testing.T) {
	t.Parallel()
	sql := `ALTER TABLE t PARTITION BY RANGE(id) (PARTITION fresh VALUES LESS THAN (1+0));
ALTER TABLE t TRUNCATE PARTITION fresh;`
	provider := t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r2RangeTailShape("t")})
	enriched := t14r1Enrich(t, spec.DialectMySQL, sql, provider)
	snapshot := t14r1TargetSnapshot(enriched, 1)
	if snapshot == nil || len(snapshot.PartitionNames) != 0 {
		t.Fatalf("the state conclusion is rule-independent, got %+v", snapshot)
	}
	result := t14Audit(t, spec.DialectMySQL, sql, provider, map[string]t05RuleConfig{})
	if len(result.Statements) != 2 {
		t.Fatalf("expected two statements, got %d", len(result.Statements))
	}
	if got := result.Statements[1].Findings; len(got) != 0 {
		t.Fatalf("no rule mints a finding on the unproven directory, got %#v", got)
	}
}

// Internal helper matrix — the per-member premise inside
// declaredSchemeOutcome itself: nil bounds, empty shells, and expression
// items are undecidable even with no comparison partner, while projected
// literal single members stay combinable. Internal unit test — not the
// public seam.
func TestT14R2TailDeclaredSchemeOutcomeSingleMember(t *testing.T) {
	t.Parallel()

	numBound := &spec.PartitionBound{Kind: "less_than", Rows: [][]spec.BoundLiteral{{spec.BoundLiteral{Kind: spec.PartitionBoundNumeric, Value: "500"}}}}
	maxBound := &spec.PartitionBound{Kind: "less_than", MaxValue: true}
	exprBound := &spec.PartitionBound{Kind: "less_than", Rows: [][]spec.BoundLiteral{{spec.BoundLiteral{Kind: spec.PartitionBoundExpression}}}}
	shellBound := &spec.PartitionBound{Kind: "less_than"}
	listNumBound := &spec.PartitionBound{Kind: "in", Rows: [][]spec.BoundLiteral{{spec.BoundLiteral{Kind: spec.PartitionBoundNumeric, Value: "3"}}}}
	listExprBound := &spec.PartitionBound{Kind: "in", Rows: [][]spec.BoundLiteral{{spec.BoundLiteral{Kind: spec.PartitionBoundExpression}}}}

	cases := []struct {
		name   string
		method string
		defs   []spec.PartitionDef
		want   partitionOutcome
	}{
		{"range_nil_bound", "range", []spec.PartitionDef{{Name: "p0"}}, partitionOutcomeUnproven},
		{"range_shell_bound", "range", []spec.PartitionDef{{Name: "p0", Bound: shellBound}}, partitionOutcomeUnproven},
		{"range_expr_bound", "range", []spec.PartitionDef{{Name: "p0", Bound: exprBound}}, partitionOutcomeUnproven},
		{"range_numeric", "range", []spec.PartitionDef{{Name: "p0", Bound: numBound}}, partitionOutcomeCombinable},
		{"range_maxvalue", "range", []spec.PartitionDef{{Name: "p0", Bound: maxBound}}, partitionOutcomeCombinable},
		{"list_nil_bound", "list", []spec.PartitionDef{{Name: "p0"}}, partitionOutcomeUnproven},
		{"list_expr_bound", "list", []spec.PartitionDef{{Name: "p0", Bound: listExprBound}}, partitionOutcomeUnproven},
		{"list_numeric", "list", []spec.PartitionDef{{Name: "p0", Bound: listNumBound}}, partitionOutcomeCombinable},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := declaredSchemeOutcome(tc.method, tc.defs); got != tc.want {
				t.Fatalf("declaredSchemeOutcome(%s) = %v, want %v", tc.method, got, tc.want)
			}
		})
	}
}

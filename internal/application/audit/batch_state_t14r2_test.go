// Package audit verifies the T14-R2 acceptance remediation on the public
// Audit seam: the frozen rule-ownership split for the ADD bound contract
// (target, not applicability), current-statement disclosure of actually
// undecidable bound comparisons, bounded partition state transitions that
// preserve unrelated facts under proven refusals and degradations, and
// the conditional OPTIMIZE scope wording for unobserved engines.
package audit

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t14r2RangeTailShape mirrors the reviewer-supplied MySQL 8.0.46 catalog:
// p0 VALUES LESS THAN (200), p1 VALUES LESS THAN (400) — plus
// data-derived statistics a proven refusal must preserve.
func t14r2RangeTailShape(name string) *spec.TableSnapshot {
	shape := t14r1RangeTailShape(name)
	shape.Options = map[string]string{"table_rows": "9", "comment": "kept"}
	return shape
}

// t14r2ShellShape reports partition defs whose bound kind is known but
// whose rows were never projected — a catalog-bound shell, not a
// projected bound.
func t14r2ShellShape(name string) *spec.TableSnapshot {
	partitioned := true
	method := "range"
	shell := func() *spec.PartitionBound { return &spec.PartitionBound{Kind: "less_than"} }
	return &spec.TableSnapshot{
		Exists:         true,
		Table:          &spec.Table{Name: name},
		Columns:        []spec.Column{t10r2Col("id", "int", true, 0)},
		Constraints:    []spec.Constraint{},
		Partitioned:    &partitioned,
		PartitionType:  &method,
		PartitionNames: []string{"p0", "p1"},
		PartitionDefs: []spec.PartitionDef{
			{Name: "p0", Bound: shell()},
			{Name: "p1", Bound: shell()},
		},
	}
}

// A — the ADD bound contract belongs to ddl.alter.add_partition.target.
// Under target-only the frozen refusals still fire; under
// applicability-only nothing fabricates them; under both the finding is
// emitted exactly once on the target rule; all-off emits nothing. The
// ordered state refuses identically under every policy — a successor
// TRUNCATE still binds the original members.
func TestT14R2AddTargetBoundContract(t *testing.T) {
	t.Parallel()
	rules := func() map[string]t05RuleConfig {
		return map[string]t05RuleConfig{
			t14r1RuleAddApplies:   {level: "blocker"},
			t14r1RuleAddTarget:    {level: "blocker"},
			t14RuleTruncateTarget: {level: "blocker"},
		}
	}
	cases := []struct {
		name     string
		dialect  spec.Dialect
		sql      string
		shape    *spec.TableSnapshot
		wantCode string
		wantPart string
	}{
		{name: "range_decreasing", dialect: spec.DialectMySQL,
			sql:   "ALTER TABLE t ADD PARTITION (PARTITION p2 VALUES LESS THAN (300));ALTER TABLE t TRUNCATE PARTITION p1;",
			shape: t14r2RangeTailShape("t"), wantCode: "1493", wantPart: "p2"},
		{name: "range_maxvalue_tail", dialect: spec.DialectMySQL,
			sql:   "ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES LESS THAN (300));ALTER TABLE t TRUNCATE PARTITION p0;",
			shape: t14RangeShape("t"), wantCode: "1481", wantPart: ""},
		{name: "list_overlap", dialect: spec.DialectMySQL,
			sql:   "ALTER TABLE t ADD PARTITION (PARTITION p2 VALUES IN (2));ALTER TABLE t TRUNCATE PARTITION p0;",
			shape: t14r1ListShape("t"), wantCode: "1495", wantPart: "p2"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, cfg := range []struct {
				name   string
				rules  map[string]t05RuleConfig
				target bool
			}{
				{name: "target_only", rules: map[string]t05RuleConfig{t14r1RuleAddTarget: {level: "blocker"}, t14RuleTruncateTarget: {level: "blocker"}}, target: true},
				{name: "applicability_only", rules: map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14RuleTruncateTarget: {level: "blocker"}}},
				{name: "both", rules: rules(), target: true},
				{name: "all_off", rules: map[string]t05RuleConfig{t14RuleTruncateTarget: {level: "blocker"}}},
			} {
				cfg := cfg
				t.Run(cfg.name, func(t *testing.T) {
					t.Parallel()
					result := t14Audit(t, tc.dialect, tc.sql, t14ProviderFor(tc.dialect, map[string]*spec.TableSnapshot{"t": tc.shape}), cfg.rules)
					if len(result.Statements) != 2 {
						t.Fatalf("expected two statements, got %d", len(result.Statements))
					}
					adds := t05FindingsByRule(result, 0, t14r1RuleAddTarget)
					if !cfg.target {
						if len(adds) != 0 {
							t.Fatalf("disabled or non-owning target must stay silent, got %#v", adds)
						}
					} else {
						if len(adds) != 1 || adds[0].Metadata["native_error"] != tc.wantCode {
							t.Fatalf("expected one %s finding on the target rule, got %#v", tc.wantCode, result.Statements[0].Findings)
						}
						if part, _ := adds[0].Metadata["partition"].(string); part != tc.wantPart {
							t.Fatalf("expected one %s finding on the target rule, got %#v", tc.wantCode, result.Statements[0].Findings)
						}
					}
					if dup := t05FindingsByRule(result, 0, t14r1RuleAddApplies); len(dup) != 0 {
						t.Fatalf("applicability must not duplicate the bound contract, got %#v", dup)
					}
					// The proven refusal preserves the exact pre-state —
					// a successor TRUNCATE still binds an original member
					// under every rule configuration.
					if got := t05FindingsByRule(result, 1, t14RuleTruncateTarget); len(got) != 0 {
						t.Fatalf("pre-state members must survive the refusal, got %#v", got)
					}
				})
			}
		})
	}
}

// A — the target ownership matrix: name collision and member binding stay
// on target; same-batch duplicate names stay on applicability.
func TestT14R2AddOwnershipSplit(t *testing.T) {
	t.Parallel()

	t.Run("existing_name_collision_stays_target", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES LESS THAN (600));",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r2RangeTailShape("t")}),
			map[string]t05RuleConfig{t14r1RuleAddTarget: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleAddTarget)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1517" || findings[0].Metadata["partition"] != "p1" {
			t.Fatalf("existing-name collision is the target contract, got %#v", result.Statements[0].Findings)
		}
	})

	t.Run("same_batch_duplicate_stays_applicability", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t ADD PARTITION (PARTITION p2 VALUES LESS THAN (600), PARTITION p2 VALUES LESS THAN (800));",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r2RangeTailShape("t")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleAddApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1517" {
			t.Fatalf("same-batch duplicate stays the applicability contract, got %#v", result.Statements[0].Findings)
		}
	})
}

// B1 — an undecidable literal bound comparison is disclosed on the current
// statement: RANGE/LIST COLUMNS strings differ bytewise with no collation
// comparison basis, so the owning target rule owes the defs fact as a gap
// and coverage drops to unverified without fabricating order or overlap.
func TestT14R2StringBoundCurrentGap(t *testing.T) {
	t.Parallel()

	t.Run("range_columns", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES LESS THAN ('a'));",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeColsShape("t", "'B'")}),
			map[string]t05RuleConfig{t14r1RuleAddTarget: {level: "blocker"}})
		stmt := result.Statements[0]
		if len(stmt.Findings) != 0 {
			t.Fatalf("undecidable string order must not fabricate findings, got %#v", stmt.Findings)
		}
		gaps := t14r1GapSet(stmt)
		if !gaps["target_table.partition_defs"] {
			t.Fatalf("the owning target rule must owe target_table.partition_defs, got %+v", stmt.EvidenceGaps)
		}
		for _, gap := range stmt.EvidenceGaps {
			if gap.RuleID != t14r1RuleAddTarget {
				t.Fatalf("the gap must be owned by %s, got %#v", t14r1RuleAddTarget, gap)
			}
		}
		if stmt.Coverage.Status != report.CoverageUnverified {
			t.Fatalf("undecidable comparison cannot stay complete, got %+v", stmt.Coverage)
		}
		if result.Verdict == report.VerdictPass {
			t.Fatalf("a statement-level gap holds the verdict at review, got %s", result.Verdict)
		}
	})

	t.Run("list_columns", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES IN ('a'));",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1ListColsShape("t", "'B'")}),
			map[string]t05RuleConfig{t14r1RuleAddTarget: {level: "blocker"}})
		stmt := result.Statements[0]
		if len(stmt.Findings) != 0 {
			t.Fatalf("undecidable string overlap must not fabricate findings, got %#v", stmt.Findings)
		}
		gaps := t14r1GapSet(stmt)
		if !gaps["target_table.partition_defs"] {
			t.Fatalf("the owning target rule must owe target_table.partition_defs, got %+v", stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status != report.CoverageUnverified {
			t.Fatalf("undecidable comparison cannot stay complete, got %+v", stmt.Coverage)
		}
	})

	t.Run("owner_disabled_leaves_no_gap", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES LESS THAN ('a'));",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeColsShape("t", "'B'")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}})
		stmt := result.Statements[0]
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("the disabled owner leaves no gap, got %+v", stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status != report.CoverageComplete {
			t.Fatalf("no enabled rule consumes the undecidable comparison, got %+v", stmt.Coverage)
		}
	})
}

// B2 — PARTITION BY whose declared string bounds cannot be ordered owes a
// declared-side gap on the applicability rule; an early action-level
// return must not suppress it. Equal byte strings stay provable.
func TestT14R2PartitionByBoundGap(t *testing.T) {
	t.Parallel()

	t.Run("undecidable_strings", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, `ALTER TABLE t PARTITION BY RANGE COLUMNS(c)
(PARTITION p0 VALUES LESS THAN ('a'), PARTITION p1 VALUES LESS THAN ('B'));`,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r2RangeTailShape("t")}),
			map[string]t05RuleConfig{t14r1RulePByApplies: {level: "blocker"}})
		stmt := result.Statements[0]
		if len(stmt.Findings) != 0 {
			t.Fatalf("undecidable declared bounds must not fabricate findings, got %#v", stmt.Findings)
		}
		found := false
		for _, gap := range stmt.EvidenceGaps {
			if gap.RuleID == t14r1RulePByApplies {
				for _, fact := range gap.RequiredFacts {
					if fact == "statement.partition_defs" {
						found = true
					}
				}
			}
		}
		if !found {
			t.Fatalf("partition_by.applicability must owe statement.partition_defs, got %+v", stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status != report.CoverageUnverified {
			t.Fatalf("the current statement must not stay complete, got %+v", stmt.Coverage)
		}
	})

	t.Run("equal_bytes_stay_provable", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, `ALTER TABLE t PARTITION BY RANGE COLUMNS(c)
(PARTITION p0 VALUES LESS THAN ('a'), PARTITION p1 VALUES LESS THAN ('a'));`,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r2RangeTailShape("t")}),
			map[string]t05RuleConfig{t14r1RulePByApplies: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RulePByApplies)
		if len(findings) != 1 || findings[0].Metadata["reason_code"] != "partition_bound_violation" || findings[0].Metadata["partition"] != "p1" {
			t.Fatalf("byte-equal adjacent bounds are a provable bound violation, got %#v", result.Statements[0].Findings)
		}
	})
}

// B3 — catalog bound shells (defs present, rows missing) are not complete
// bound evidence: REORGANIZE owes the defs fact instead of passing.
func TestT14R2CatalogBoundShellGap(t *testing.T) {
	t.Parallel()
	result := t14Audit(t, spec.DialectMySQL, `ALTER TABLE t REORGANIZE PARTITION p0 INTO (
  PARTITION p9 VALUES LESS THAN (50), PARTITION p8 VALUES LESS THAN (200));`,
		t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r2ShellShape("t")}),
		map[string]t05RuleConfig{t14RuleReorgTarget: {level: "blocker"}})
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("shell bounds must not fabricate refusals, got %#v", stmt.Findings)
	}
	gaps := t14r1GapSet(stmt)
	if !gaps["target_table.partition_defs"] {
		t.Fatalf("catalog bound shells owe target_table.partition_defs, got %+v", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("shell bounds cannot claim complete coverage, got %+v", stmt.Coverage)
	}
}

// B4 — a later provable violation is not swallowed by an earlier
// undecidable member: the expression bound keeps its capability aspect
// while the 1493 on the last pair still lands on the owning target rule.
func TestT14R2LaterProvableNotSwallowed(t *testing.T) {
	t.Parallel()
	result := t14Audit(t, spec.DialectMySQL, `ALTER TABLE t ADD PARTITION (
  PARTITION p2 VALUES LESS THAN (100+100),
  PARTITION p3 VALUES LESS THAN (600),
  PARTITION p4 VALUES LESS THAN (500));`,
		t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r2RangeTailShape("t")}),
		map[string]t05RuleConfig{t14r1RuleAddTarget: {level: "blocker"}, t14r1RuleAddApplies: {level: "blocker"}})
	stmt := result.Statements[0]
	findings := t05FindingsByRule(result, 0, t14r1RuleAddTarget)
	if len(findings) != 1 || findings[0].Metadata["native_error"] != "1493" || findings[0].Metadata["partition"] != "p4" {
		t.Fatalf("the later provable decrease must land on the target rule, got %#v", stmt.Findings)
	}
	if !t14r1HasFeature(result, "alter_table.add_partition.partition_bound") {
		t.Fatalf("the expression bound keeps its capability aspect, got %+v", result.Unsupported)
	}
	if stmt.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("the capability aspect keeps coverage incomplete, got %+v", stmt.Coverage)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("the provable blocker keeps its verdict, got %s", result.Verdict)
	}
}

// C1 — a proven same-batch duplicate refuses the whole statement: the
// unmodelled expression aspect stays, but the exact pre-state — members,
// statistics, keys, constraints — survives for the successor.
func TestT14R2ProvenRefusalKeepsPreState(t *testing.T) {
	t.Parallel()
	result := t14Audit(t, spec.DialectMySQL, `
ALTER TABLE t ADD PARTITION (
  PARTITION p2 VALUES LESS THAN (100+100),
  PARTITION p2 VALUES LESS THAN (800));
ALTER TABLE t TRUNCATE PARTITION p0;
`,
		t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r2RangeTailShape("t")}),
		map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14RuleTruncateTarget: {level: "blocker"}})
	if len(result.Statements) != 2 {
		t.Fatalf("expected two statements, got %d", len(result.Statements))
	}
	findings := t05FindingsByRule(result, 0, t14r1RuleAddApplies)
	if len(findings) != 1 || findings[0].Metadata["native_error"] != "1517" {
		t.Fatalf("the same-batch duplicate is a provable 1517, got %#v", result.Statements[0].Findings)
	}
	if !t14r1HasFeature(result, "alter_table.add_partition.partition_bound") {
		t.Fatalf("the expression aspect stays on the refused statement, got %+v", result.Unsupported)
	}
	trunc := result.Statements[1]
	if got := t05FindingsByRule(result, 1, t14RuleTruncateTarget); len(got) != 0 {
		t.Fatalf("p0 must still bind after the refused ADD, got %#v", got)
	}
	if len(trunc.EvidenceGaps) != 0 || trunc.Coverage.Status != report.CoverageComplete {
		t.Fatalf("the successor must read the intact pre-state, got gaps=%+v coverage=%+v", trunc.EvidenceGaps, trunc.Coverage)
	}
}

// C1b — the successor's observed target table carries the untouched
// structural facts and statistics of the refused pre-state.
func TestT14R2RefusedAddPreservesSnapshot(t *testing.T) {
	t.Parallel()
	provider := t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r2RangeTailShape("t")})
	enriched := t14r1Enrich(t, spec.DialectMySQL, `ALTER TABLE t ADD PARTITION (
  PARTITION p2 VALUES LESS THAN (100+100),
  PARTITION p2 VALUES LESS THAN (800));
ALTER TABLE t TRUNCATE PARTITION p0;`, provider)
	if len(enriched) != 2 {
		t.Fatalf("expected two statements, got %d", len(enriched))
	}
	snapshot := t14r1TargetSnapshot(enriched, 1)
	if snapshot == nil {
		t.Fatal("the refused statement must not tombstone the successor's target table")
	}
	if len(snapshot.PartitionNames) != 2 || snapshot.PartitionNames[0] != "p0" || snapshot.PartitionNames[1] != "p1" {
		t.Fatalf("the directory is preserved, got %+v", snapshot.PartitionNames)
	}
	if snapshot.Options["table_rows"] != "9" || snapshot.Options["comment"] != "kept" {
		t.Fatalf("statistics survive the proven refusal, got %+v", snapshot.Options)
	}
	if snapshot.Partitioned == nil || !*snapshot.Partitioned || snapshot.PartitionType == nil || *snapshot.PartitionType != "range" {
		t.Fatalf("the scheme facts are preserved, got partitioned=%v type=%v", snapshot.Partitioned, snapshot.PartitionType)
	}
}

// C2 — CREATE TABLE IF NOT EXISTS on a proven existing table keeps the
// existing facts even when the declared scheme carries expression bounds.
func TestT14R2CreateIfNotExistsKeepsFacts(t *testing.T) {
	t.Parallel()
	provider := t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r2RangeTailShape("t")})
	enriched := t14r1Enrich(t, spec.DialectMySQL, `CREATE TABLE IF NOT EXISTS t (id INT) PARTITION BY RANGE(id)
(PARTITION px VALUES LESS THAN (100+100), PARTITION py VALUES LESS THAN (200));
ALTER TABLE t TRUNCATE PARTITION p0;`, provider)
	if len(enriched) != 2 {
		t.Fatalf("expected two statements, got %d", len(enriched))
	}
	snapshot := t14r1TargetSnapshot(enriched, 1)
	if snapshot == nil {
		t.Fatal("a no-op CREATE must keep the proven existing table")
	}
	if len(snapshot.PartitionNames) != 2 || snapshot.PartitionNames[0] != "p0" {
		t.Fatalf("the observed directory is preserved, got %+v", snapshot.PartitionNames)
	}
	if snapshot.Options["table_rows"] != "9" {
		t.Fatalf("existing statistics are preserved, got %+v", snapshot.Options)
	}
}

// C3 — a bound-only-unproven ADD degrades just the partition directory:
// existence, scheme kind, columns, keys and constraints survive while the
// successor owes directory facts instead of table facts.
func TestT14R2UnprovenAddDegradesDirectoryOnly(t *testing.T) {
	t.Parallel()
	listShape := t14r1ListShape("t")
	listShape.Options = map[string]string{"table_rows": "9"}
	provider := t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": listShape})
	enriched := t14r1Enrich(t, spec.DialectMySQL, `ALTER TABLE t ADD PARTITION (PARTITION p2 VALUES IN (1+0));
ALTER TABLE t TRUNCATE PARTITION p2;`, provider)
	if len(enriched) != 2 {
		t.Fatalf("expected two statements, got %d", len(enriched))
	}
	snapshot := t14r1TargetSnapshot(enriched, 1)
	if snapshot == nil {
		t.Fatal("an unproven partition directory must not tombstone the table")
	}
	if !snapshot.Exists {
		t.Fatal("table existence is unaffected by the unproven bound")
	}
	if snapshot.Partitioned == nil || !*snapshot.Partitioned {
		t.Fatalf("the partitioned fact survives, got %+v", snapshot.Partitioned)
	}
	if snapshot.PartitionType == nil || *snapshot.PartitionType != "list" {
		t.Fatalf("the proven method survives, got %+v", snapshot.PartitionType)
	}
	if len(snapshot.Columns) != 1 || snapshot.Columns[0].Name != "id" {
		t.Fatalf("columns survive the unproven directory, got %+v", snapshot.Columns)
	}
	if len(snapshot.PartitionNames) != 0 || len(snapshot.PartitionDefs) != 0 {
		t.Fatalf("only the unprovable directory is unvouched, got names=%v defs=%+v", snapshot.PartitionNames, snapshot.PartitionDefs)
	}
}

// C3b — the successor TRUNCATE owes directory facts (not existence or
// partitioned) when the prior ADD left the directory unproven.
func TestT14R2UnprovenAddSuccessorGaps(t *testing.T) {
	t.Parallel()
	result := t14Audit(t, spec.DialectMySQL, `ALTER TABLE t ADD PARTITION (PARTITION p2 VALUES IN (1+0));
ALTER TABLE t TRUNCATE PARTITION p2;`,
		t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1ListShape("t")}),
		map[string]t05RuleConfig{t14RuleTruncateTarget: {level: "blocker"}})
	if len(result.Statements) != 2 {
		t.Fatalf("expected two statements, got %d", len(result.Statements))
	}
	stmt := result.Statements[1]
	gaps := t14r1GapSet(stmt)
	if !gaps["target_table.partitions"] {
		t.Fatalf("the successor owes the member directory, got %+v", stmt.EvidenceGaps)
	}
	if gaps["target_table.partitioned"] {
		t.Fatalf("the proven partitioned fact is not owed again, got %+v", stmt.EvidenceGaps)
	}
}

// D — an unobserved engine keeps OPTIMIZE's scope conditional; the InnoDB
// branch keeps its whole-table disclosure.
func TestT14R2OptimizeEngineScope(t *testing.T) {
	t.Parallel()

	t.Run("unknown_engine_conditional", func(t *testing.T) {
		t.Parallel()
		shape := t14RangeShape("t")
		shape.Options = map[string]string{"table_rows": "9"}
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t OPTIMIZE PARTITION p0;",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": shape}),
			map[string]t05RuleConfig{t14RuleOptimizeWarn: {level: "warning"}})
		findings := t05FindingsByRule(result, 0, t14RuleOptimizeWarn)
		if len(findings) != 1 {
			t.Fatalf("expected one optimize warning, got %#v", result.Statements[0].Findings)
		}
		text := findings[0].Message + " " + findings[0].Suggestion
		if strings.Contains(text, "rebuilds and re-analyzes the named members") {
			t.Fatalf("an unobserved engine cannot claim deterministic named-member scope, got message=%q suggestion=%q", findings[0].Message, findings[0].Suggestion)
		}
		if !strings.Contains(findings[0].Message, "observed engine decides") {
			t.Fatalf("the conditional scope wording must be disclosed, got %q", findings[0].Message)
		}
	})

	t.Run("innodb_keeps_whole_table", func(t *testing.T) {
		t.Parallel()
		shape := t14r1EngineShape("t", "innodb")
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t OPTIMIZE PARTITION p0;",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": shape}),
			map[string]t05RuleConfig{t14RuleOptimizeWarn: {level: "warning"}})
		findings := t05FindingsByRule(result, 0, t14RuleOptimizeWarn)
		if len(findings) != 1 || !strings.Contains(findings[0].Message, "named members") {
			t.Fatalf("the known InnoDB branch keeps the whole-table scope, got %#v", findings)
		}
	})
}

// Package audit verifies the T14 partition topology and maintenance
// semantics on the public Audit seam: the frozen REORGANIZE golden shape
// (proven member-directory transition a successor reads), the probed
// refusal classes bound to native errors, the soft-fail maintenance
// surface, REMOVE PARTITIONING's successor state, evidence gaps on
// unobserved partition facts, and the TiDB vendor-boundary split.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t14RuleReorgWarn          = "ddl.alter.reorganize_partition.warn"
	t14RuleReorgApplies       = "ddl.alter.reorganize_partition.applicability"
	t14RuleReorgTarget        = "ddl.alter.reorganize_partition.target"
	t14RuleCoalesceApplies    = "ddl.alter.coalesce_partition.applicability"
	t14RuleRemoveWarn         = "ddl.alter.remove_partitioning.warn"
	t14RuleCheckWarn          = "ddl.alter.check_partition.warn"
	t14RuleOptimizeApplies    = "ddl.alter.optimize_partition.applicability"
	t14RuleDiscardForbid      = "ddl.alter.discard_partition_tablespace.forbid"
	t14RuleDropTarget         = "ddl.alter.drop_partition.target"
	t14RuleTruncateTarget     = "ddl.alter.truncate_partition.target"
	t14RuleDropPartition      = "ddl.alter.drop_partition.forbid"
	t14RuleDropApplies        = "ddl.alter.drop_partition.applicability"
	t14RuleOptimizeWarn       = "ddl.alter.optimize_partition.warn"
	t14RuleMaintenanceApplies = "ddl.alter.rebuild_partition.applicability"
)

// t14RangeShape is the frozen golden fixture: a RANGE table whose catalog
// reports p0 VALUES LESS THAN (200) plus a MAXVALUE tail, carrying
// data-derived statistics a topology change can no longer vouch for.
func t14RangeShape(name string) *spec.TableSnapshot {
	partitioned := true
	method := "range"
	cardinality := int64(7)
	names := []string{"p0", "pmax"}
	descriptions := []string{"200", "MAXVALUE"}
	return &spec.TableSnapshot{
		Exists:                true,
		Table:                 &spec.Table{Name: name},
		Columns:               []spec.Column{t10r2Col("id", "int", true, 0), t10r2Col("c", "int", true, 0)},
		PrimaryKey:            &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}, Cardinality: &cardinality},
		Indexes:               []spec.Index{{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, Cardinality: &cardinality}},
		Constraints:           []spec.Constraint{},
		Partitioned:           &partitioned,
		PartitionType:         &method,
		PartitionNames:        names,
		PartitionDescriptions: descriptions,
		PartitionDefs:         spec.CatalogPartitionDefs(names, descriptions, method),
		Options: map[string]string{
			"table_rows": "9", "data_length": "100", "index_length": "40",
			"avg_row_length": "8", "auto_increment": "4", "comment": "kept",
		},
	}
}

// t14HashShape is a HASH-method partitioned table — TiDB refuses
// REORGANIZE on it natively (8200).
func t14HashShape(name string) *spec.TableSnapshot {
	partitioned := true
	method := "hash"
	names := []string{"p0", "p1"}
	return &spec.TableSnapshot{
		Exists:         true,
		Table:          &spec.Table{Name: name},
		Columns:        []spec.Column{t10r2Col("id", "int", true, 0)},
		Constraints:    []spec.Constraint{},
		Partitioned:    &partitioned,
		PartitionType:  &method,
		PartitionNames: names,
		PartitionDefs:  spec.CatalogPartitionDefs(names, nil, method),
	}
}

// t14PlainShape is a known non-partitioned table — every partition action
// on it is the probed 1505/soft-fail class, never a directory mutation.
func t14PlainShape(name string) *spec.TableSnapshot {
	partitioned := false
	return &spec.TableSnapshot{
		Exists:         true,
		Table:          &spec.Table{Name: name},
		Columns:        []spec.Column{t10r2Col("id", "int", true, 0)},
		Constraints:    []spec.Constraint{},
		Partitioned:    &partitioned,
		PartitionNames: []string{},
		PartitionDefs:  []spec.PartitionDef{},
	}
}

// t14UnobservedShape knows the table exists but never answered the
// partition probe — enabled rules owe evidence gaps rather than verdicts.
func t14UnobservedShape(name string) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists:      true,
		Table:       &spec.Table{Name: name},
		Columns:     []spec.Column{t10r2Col("id", "int", true, 0)},
		Constraints: []spec.Constraint{},
	}
}

func t14ProviderFor(dialect spec.Dialect, tables map[string]*spec.TableSnapshot) *t10Provider {
	version := "8.0.46"
	if dialect == spec.DialectTiDB {
		version = "8.0.11-TiDB-v8.5.0"
	}
	return &t10Provider{version: version, tables: tables}
}

func t14Audit(t *testing.T, dialect spec.Dialect, sql string, provider MetadataProvider, rules map[string]t05RuleConfig) report.Result {
	t.Helper()
	return t10Audit(t, dialect, sql, provider, t05PolicyPathConfigured(t, rules))
}

func t14GapFacts(stmt report.StatementResult) []string {
	facts := make([]string, 0, len(stmt.EvidenceGaps))
	for _, gap := range stmt.EvidenceGaps {
		facts = append(facts, gap.RequiredFacts...)
	}
	return facts
}

// The frozen first path: REORGANIZE p0→p1(120)/p2(200) on the RANGE
// fixture emits exactly one warning on the authored action with complete
// coverage, and the successor directory serves p1/p2 while p0 is proven
// absent — data-derived statistics are unvouched.
func TestT14ReorganizeGoldenSuccessorDirectory(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			result := t14Audit(t, dialect, `
ALTER TABLE t REORGANIZE PARTITION p0 INTO (
  PARTITION p1 VALUES LESS THAN (120),
  PARTITION p2 VALUES LESS THAN (200));
ALTER TABLE t TRUNCATE PARTITION p1;
ALTER TABLE t DROP PARTITION p0;
`,
				t14ProviderFor(dialect, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}),
				map[string]t05RuleConfig{
					t14RuleReorgWarn:      {level: "warning"},
					t14RuleTruncateTarget: {level: "blocker"},
					t14RuleDropTarget:     {level: "blocker"},
					t14RuleDropPartition:  {level: "blocker"},
				})
			if len(result.Statements) != 3 {
				t.Fatalf("expected three statements, got %d", len(result.Statements))
			}
			reorg := result.Statements[0]
			if reorg.Coverage.Status != report.CoverageComplete {
				t.Fatalf("reorg coverage = %+v, want complete", reorg.Coverage)
			}
			warns := t05FindingsByRule(result, 0, t14RuleReorgWarn)
			if len(warns) != 1 || warns[0].Metadata["table"] != "t" {
				t.Fatalf("expected exactly one reorg warning on t, got %#v", reorg.Findings)
			}
			// Successor 1: TRUNCATE p1 binds the published member — no
			// target finding, no fabricated absence.
			trunc := result.Statements[1]
			if got := t05FindingsByRule(result, 1, t14RuleTruncateTarget); len(got) != 0 {
				t.Fatalf("p1 must be present after reorg, got target findings %#v", got)
			}
			if trunc.Coverage.Status != report.CoverageComplete {
				t.Fatalf("truncate coverage = %+v, want complete", trunc.Coverage)
			}
			// Successor 2: DROP p0 proves the source name gone (1507-class
			// member-absence), while the action-level forbid still fires.
			absent := t05FindingsByRule(result, 2, t14RuleDropTarget)
			if len(absent) != 1 || absent[0].Metadata["native_error"] != "1507" || absent[0].Metadata["partition"] != "p0" {
				t.Fatalf("p0 must be proven absent after reorg, got %#v", result.Statements[2].Findings)
			}
		})
	}
}

// Proven refusal cells keep the exact pre-state: non-partitioned targets
// refuse 1505 on both dialects and a missing source member refuses 1507 on
// MySQL / 1567 on TiDB, with no directory mutation downstream.
func TestT14ReorganizeProvenRefusals(t *testing.T) {
	t.Parallel()

	t.Run("non_partitioned", func(t *testing.T) {
		t.Parallel()
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			dialect := dialect
			t.Run(string(dialect), func(t *testing.T) {
				t.Parallel()
				result := t14Audit(t, dialect, "ALTER TABLE t REORGANIZE PARTITION p0 INTO (PARTITION p9 VALUES LESS THAN (50));",
					t14ProviderFor(dialect, map[string]*spec.TableSnapshot{"t": t14PlainShape("t")}),
					map[string]t05RuleConfig{t14RuleReorgApplies: {level: "blocker"}})
				findings := t05FindingsByRule(result, 0, t14RuleReorgApplies)
				if len(findings) != 1 || findings[0].Metadata["native_error"] != "1505" || findings[0].Metadata["reason_code"] != "not_partitioned" {
					t.Fatalf("expected 1505 not_partitioned refusal, got %#v", result.Statements[0].Findings)
				}
			})
		}
	})

	t.Run("missing_source", func(t *testing.T) {
		t.Parallel()
		want := map[spec.Dialect]string{spec.DialectMySQL: "1507", spec.DialectTiDB: "1567"}
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			dialect := dialect
			t.Run(string(dialect), func(t *testing.T) {
				t.Parallel()
				result := t14Audit(t, dialect, "ALTER TABLE t REORGANIZE PARTITION p9 INTO (PARTITION p8 VALUES LESS THAN (50));",
					t14ProviderFor(dialect, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}),
					map[string]t05RuleConfig{t14RuleReorgTarget: {level: "blocker"}})
				findings := t05FindingsByRule(result, 0, t14RuleReorgTarget)
				if len(findings) != 1 || findings[0].Metadata["native_error"] != want[dialect] || findings[0].Metadata["partition"] != "p9" {
					t.Fatalf("expected %s missing-member refusal on p9, got %#v", want[dialect], result.Statements[0].Findings)
				}
			})
		}
	})

	t.Run("tidb_hash_method", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectTiDB, "ALTER TABLE t REORGANIZE PARTITION p0 INTO (PARTITION p8 VALUES LESS THAN (50));",
			t14ProviderFor(spec.DialectTiDB, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}),
			map[string]t05RuleConfig{t14RuleReorgApplies: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14RuleReorgApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "8200" {
			t.Fatalf("TiDB hash-method reorg must refuse 8200, got %#v", result.Statements[0].Findings)
		}
	})
}

// A proven refusal never mutates the directory: after the refused reorg a
// successor TRUNCATE still binds p0.
func TestT14RefusedReorgKeepsPreState(t *testing.T) {
	t.Parallel()
	result := t14Audit(t, spec.DialectMySQL, `
ALTER TABLE t REORGANIZE PARTITION p9 INTO (PARTITION p8 VALUES LESS THAN (50));
ALTER TABLE t TRUNCATE PARTITION p0;
`,
		t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}),
		map[string]t05RuleConfig{t14RuleReorgTarget: {level: "blocker"}, t14RuleTruncateTarget: {level: "blocker"}})
	if got := t05FindingsByRule(result, 1, t14RuleTruncateTarget); len(got) != 0 {
		t.Fatalf("p0 must survive the refused reorg, got %#v", got)
	}
}

// COALESCE on a RANGE scheme is the probed 1509 class.
func TestT14CoalesceRangeRefuses1509(t *testing.T) {
	t.Parallel()
	result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t COALESCE PARTITION 1;",
		t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}),
		map[string]t05RuleConfig{t14RuleCoalesceApplies: {level: "blocker"}})
	findings := t05FindingsByRule(result, 0, t14RuleCoalesceApplies)
	if len(findings) != 1 || findings[0].Metadata["native_error"] != "1509" {
		t.Fatalf("coalesce on range must refuse 1509, got %#v", result.Statements[0].Findings)
	}
}

// REMOVE PARTITIONING publishes the proven non-partitioned shape: the
// successor DROP PARTITION reads the 1505 class, not the stale directory.
func TestT14RemovePartitioningSuccessorReadsPlain(t *testing.T) {
	t.Parallel()
	result := t14Audit(t, spec.DialectMySQL, `
ALTER TABLE t REMOVE PARTITIONING;
ALTER TABLE t DROP PARTITION p0;
`,
		t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}),
		map[string]t05RuleConfig{t14RuleRemoveWarn: {level: "warning"}, t14RuleDropApplies: {level: "blocker"}, t14RuleDropTarget: {level: "blocker"}, t14RuleDropPartition: {level: "blocker"}})
	if got := t05FindingsByRule(result, 0, t14RuleRemoveWarn); len(got) != 1 {
		t.Fatalf("expected one remove_partitioning warning, got %#v", result.Statements[0].Findings)
	}
	applies := t05FindingsByRule(result, 1, t14RuleDropApplies)
	if len(applies) != 1 || applies[0].Metadata["native_error"] != "1505" {
		t.Fatalf("successor DROP must read the plain-table 1505 class, got %#v", result.Statements[1].Findings)
	}
	if got := t05FindingsByRule(result, 1, t14RuleDropTarget); len(got) != 0 {
		t.Fatalf("no member binding applies to a removed scheme, got %#v", got)
	}
}

// Maintenance ops keep their native shapes: CHECK emits the notice-level
// governance finding on a partitioned target, OPTIMIZE on a plain table
// is the probed soft-fail warning (never a fabricated hard refusal), and
// DISCARD owns its dedicated blocker.
func TestT14MaintenanceSemantics(t *testing.T) {
	t.Parallel()

	t.Run("check_notice", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t CHECK PARTITION p0;",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}),
			map[string]t05RuleConfig{t14RuleCheckWarn: {level: "notice"}})
		findings := t05FindingsByRule(result, 0, t14RuleCheckWarn)
		if len(findings) != 1 || findings[0].Level != "notice" {
			t.Fatalf("check partition emits one notice finding, got %#v", result.Statements[0].Findings)
		}
	})

	t.Run("optimize_soft_fail", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t OPTIMIZE PARTITION p0;",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14PlainShape("t")}),
			map[string]t05RuleConfig{t14RuleOptimizeApplies: {level: "warning"}, t14RuleOptimizeWarn: {level: "warning"}})
		findings := t05FindingsByRule(result, 0, t14RuleOptimizeApplies)
		if len(findings) != 1 || findings[0].Level != "warning" || findings[0].Metadata["soft_fail"] != "true" {
			t.Fatalf("optimize on a plain table is a soft-fail warning, got %#v", result.Statements[0].Findings)
		}
		if result.Verdict == report.VerdictReject {
			t.Fatalf("a soft-fail warning never rejects, got %s", result.Verdict)
		}
	})

	t.Run("discard_forbid", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t DISCARD PARTITION p0 TABLESPACE;",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}),
			map[string]t05RuleConfig{t14RuleDiscardForbid: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14RuleDiscardForbid)
		if len(findings) != 1 || findings[0].Level != "blocker" || findings[0].Metadata["partition"] != "p0" {
			t.Fatalf("discard keeps its dedicated blocker, got %#v", result.Statements[0].Findings)
		}
	})
}

// Unobserved partition state owes the consumed facts as evidence gaps —
// enabled rules never fabricate a partitioned verdict either way. The
// staging is honest: `partitioned` is consumed first, and only a proven
// partitioned table owes the method/member facts downstream.
func TestT14UnobservedPartitionStateOwesGaps(t *testing.T) {
	t.Parallel()

	t.Run("partitioned_unknown", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t REORGANIZE PARTITION p0 INTO (PARTITION p9 VALUES LESS THAN (50));",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14UnobservedShape("t")}),
			map[string]t05RuleConfig{t14RuleReorgApplies: {level: "blocker"}, t14RuleReorgTarget: {level: "blocker"}})
		stmt := result.Statements[0]
		if len(stmt.Findings) != 0 {
			t.Fatalf("unobserved state must not fabricate refusals, got %#v", stmt.Findings)
		}
		facts := t14GapFacts(stmt)
		seen := false
		for _, f := range facts {
			if f == "target_table.partitioned" {
				seen = true
			}
			if f == "target_table.partition_type" || f == "target_table.partitions" {
				t.Fatalf("a maybe-unpartitioned table cannot owe %s — the fact is only consumed once partitioning is proven, got %+v", f, facts)
			}
		}
		if !seen {
			t.Fatalf("expected the target_table.partitioned gap, got %+v", facts)
		}
		if stmt.Coverage.Status == report.CoverageComplete {
			t.Fatalf("unobserved state must lower coverage, got %+v", stmt.Coverage)
		}
	})

	t.Run("method_and_members_unknown", func(t *testing.T) {
		t.Parallel()
		partitioned := true
		shape := &spec.TableSnapshot{
			Exists:      true,
			Table:       &spec.Table{Name: "t"},
			Columns:     []spec.Column{t10r2Col("id", "int", true, 0)},
			Constraints: []spec.Constraint{},
			Partitioned: &partitioned,
		}
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t REORGANIZE PARTITION p0 INTO (PARTITION p9 VALUES LESS THAN (50));",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": shape}),
			map[string]t05RuleConfig{t14RuleReorgApplies: {level: "blocker"}, t14RuleReorgTarget: {level: "blocker"}})
		stmt := result.Statements[0]
		if len(stmt.Findings) != 0 {
			t.Fatalf("unvouched method/member facts must not fabricate refusals, got %#v", stmt.Findings)
		}
		facts := t14GapFacts(stmt)
		need := map[string]bool{"target_table.partition_type": false, "target_table.partitions": false}
		for _, f := range facts {
			if _, ok := need[f]; ok {
				need[f] = true
			}
		}
		for f, seen := range need {
			if !seen {
				t.Fatalf("expected gap fact %s, got %+v", f, facts)
			}
		}
	})
}

// TiDB refuses the maintenance set natively — the actions stay
// vendor-boundary aspects with incomplete coverage and zero findings,
// never MySQL verdicts copied onto TiDB.
func TestT14TiDBMaintenanceVendorBoundary(t *testing.T) {
	t.Parallel()
	result := t14Audit(t, spec.DialectTiDB, "ALTER TABLE t REBUILD PARTITION p0;",
		t14ProviderFor(spec.DialectTiDB, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}),
		map[string]t05RuleConfig{t14RuleMaintenanceApplies: {level: "blocker"}})
	stmt := result.Statements[0]
	if stmt.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("TiDB maintenance must stay vendor boundary, got %+v", stmt.Coverage)
	}
	if len(stmt.Findings) != 0 {
		t.Fatalf("vendor boundary emits no findings, got %#v", stmt.Findings)
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Feature != "alter_table.rebuild_partition" {
		t.Fatalf("expected the rebuild vendor aspect, got %+v", result.Unsupported)
	}
}

// A disabled policy removes the findings, never the audited aspect or the
// gap accounting — the statement still reports complete coverage.
func TestT14RulesOffKeepAuditedCoverage(t *testing.T) {
	t.Parallel()
	result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t REORGANIZE PARTITION p0 INTO (PARTITION p9 VALUES LESS THAN (50));",
		t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}),
		map[string]t05RuleConfig{})
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("all-off policy emits no findings, got %#v", stmt.Findings)
	}
	if stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("the audited action stays complete under an empty policy, got %+v", stmt.Coverage)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("no unsupported evidence on an audited action, got %+v", result.Unsupported)
	}
}

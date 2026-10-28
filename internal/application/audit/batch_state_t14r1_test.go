// Package audit verifies the T14-R1 remediation cells on the public Audit
// seam: overflow-safe partition counts, same-batch duplicate member names,
// CREATE consuming the declared-scheme validity judgment, RANGE COLUMNS
// tuple bounds, bounded string/overlap comparison domains, expression-bound
// coverage attribution, and the vendor/engine maintenance matrix. Every
// assertion rides the ordered-state projection — rules, evidence gaps,
// coverage, and successor metadata must answer from one judgment.
package audit

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t14r1RuleAddApplies      = "ddl.alter.add_partition.applicability"
	t14r1RuleAddTarget       = "ddl.alter.add_partition.target"
	t14r1RuleReorgApplies    = "ddl.alter.reorganize_partition.applicability"
	t14r1RuleReorgTarget     = "ddl.alter.reorganize_partition.target"
	t14r1RuleCoalesceApplies = "ddl.alter.coalesce_partition.applicability"
	t14r1RulePByApplies      = "ddl.alter.partition_by.applicability"
	t14r1RuleOptimizeWarn    = "ddl.alter.optimize_partition.warn"
	t14r1RuleCheckWarn       = "ddl.alter.check_partition.warn"
	t14r1RuleRebuildTarget   = "ddl.alter.rebuild_partition.target"
	t14r1RuleRebuildApplies  = "ddl.alter.rebuild_partition.applicability"
	t14r1RuleDiscardApplies  = "ddl.alter.discard_partition_tablespace.applicability"
	t14r1RuleDiscardForbid   = "ddl.alter.discard_partition_tablespace.forbid"
	t14r1RuleImportApplies   = "ddl.alter.import_partition_tablespace.applicability"
	t14r1RuleTruncateTarget  = "ddl.alter.truncate_partition.target"
)

// t14r1RangeTailShape is a RANGE table whose tail bound is numeric — the
// ADD-below-tail refusal (native 1493) and REORGANIZE span-shrink refusal
// (native 1520 MySQL / 1493 TiDB) fixtures.
func t14r1RangeTailShape(name string) *spec.TableSnapshot {
	partitioned := true
	method := "range"
	names := []string{"p0", "p1"}
	descriptions := []string{"200", "400"}
	return &spec.TableSnapshot{
		Exists:                true,
		Table:                 &spec.Table{Name: name},
		Columns:               []spec.Column{t10r2Col("id", "int", true, 0)},
		Constraints:           []spec.Constraint{},
		Partitioned:           &partitioned,
		PartitionType:         &method,
		PartitionNames:        names,
		PartitionDescriptions: descriptions,
		PartitionDefs:         spec.CatalogPartitionDefs(names, descriptions, method),
	}
}

// t14r1ListShape is a LIST table — overlap refusals (native 1495) and the
// expression-bound unproven path ride on it.
func t14r1ListShape(name string) *spec.TableSnapshot {
	partitioned := true
	method := "list"
	names := []string{"p0", "p1"}
	descriptions := []string{"1", "2"}
	return &spec.TableSnapshot{
		Exists:                true,
		Table:                 &spec.Table{Name: name},
		Columns:               []spec.Column{t10r2Col("id", "int", true, 0)},
		Constraints:           []spec.Constraint{},
		Partitioned:           &partitioned,
		PartitionType:         &method,
		PartitionNames:        names,
		PartitionDescriptions: descriptions,
		PartitionDefs:         spec.CatalogPartitionDefs(names, descriptions, method),
	}
}

// t14r1RangeColsShape carries RANGE COLUMNS catalog descriptions — the
// per-item MAXVALUE projection fixture (`10,MAXVALUE` etc.).
func t14r1RangeColsShape(name string, descriptions ...string) *spec.TableSnapshot {
	partitioned := true
	method := "range columns"
	names := make([]string, len(descriptions))
	for i := range names {
		names[i] = "p" + string(rune('0'+i))
	}
	return &spec.TableSnapshot{
		Exists:                true,
		Table:                 &spec.Table{Name: name},
		Columns:               []spec.Column{t10r2Col("a", "int", true, 0), t10r2Col("b", "int", true, 0)},
		Constraints:           []spec.Constraint{},
		Partitioned:           &partitioned,
		PartitionType:         &method,
		PartitionNames:        names,
		PartitionDescriptions: descriptions,
		PartitionDefs:         spec.CatalogPartitionDefs(names, descriptions, method),
	}
}

// t14r1ListColsShape wraps t14r1RangeColsShape with the list-columns
// method — the string-overlap fixture.
func t14r1ListColsShape(name string, descriptions ...string) *spec.TableSnapshot {
	shape := t14r1RangeColsShape(name, descriptions...)
	method := "list columns"
	shape.PartitionType = &method
	shape.PartitionDefs = spec.CatalogPartitionDefs(shape.PartitionNames, descriptions, method)
	return shape
}

// t14r1EngineShape is a partitioned table with a proven storage engine —
// the DISCARD/IMPORT applicability fixture.
func t14r1EngineShape(name, engine string) *spec.TableSnapshot {
	shape := t14RangeShape(name)
	shape.Options = map[string]string{"engine": engine, "table_rows": "9"}
	return shape
}

// t14r1Enrich runs the same ordered-state seam the public audit uses and
// returns the enriched statements — statement[i].Metadata.TargetTable is
// the ordered pre-state at that point (the published post-state of every
// earlier statement).
func t14r1Enrich(t *testing.T, dialect spec.Dialect, sql string, provider MetadataProvider) []spec.Statement {
	t.Helper()
	parsed, parseErr := parseSQL(context.Background(), sql, dialect)
	if parseErr != nil || len(parsed.Statements) == 0 {
		t.Fatalf("parse: err=%v statements=%d", parseErr, len(parsed.Statements))
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	enriched, err := enrichStatementsWithMetadata(context.Background(), dialect,
		&MetadataRequest{Schema: "golden", Provider: provider}, statements, parsed.failures)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	return enriched
}

// t14r1TargetSnapshot returns the ordered pre-state enriched[i] observed —
// i.e. the post-state published by the statements before it.
func t14r1TargetSnapshot(enriched []spec.Statement, i int) *spec.TableSnapshot {
	if i >= len(enriched) || enriched[i].Metadata == nil {
		return nil
	}
	return enriched[i].Metadata.TargetTable
}

func t14r1GapSet(stmt report.StatementResult) map[string]bool {
	set := map[string]bool{}
	for _, gap := range stmt.EvidenceGaps {
		for _, fact := range gap.RequiredFacts {
			set[fact] = true
		}
	}
	return set
}

func t14r1HasFeature(result report.Result, feature string) bool {
	for _, detail := range result.Unsupported {
		if detail.Feature == feature {
			return true
		}
	}
	return false
}

// t14r1AuditParserBoundary runs the public seam on shapes the shared
// parser itself refuses natively (PARTITIONS 0 → 1504, count-without-defs
// on range|list → 1492): the audit returns errParserUnsupported with the
// parser diagnostic on the result — that diagnostic IS the refusal
// record, so the helper tolerates the sentinel instead of fataling.
func t14r1AuditParserBoundary(t *testing.T, dialect spec.Dialect, sql string, provider MetadataProvider, rules map[string]t05RuleConfig) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          dialect,
		Schema:           "golden",
		ConfigPath:       t05PolicyPathConfigured(t, rules),
		MetadataProvider: provider,
	})
	if err != nil && !errors.Is(err, ErrUnsupportedStatement) && !errors.Is(err, errParserUnsupported) {
		t.Fatalf("audit: %v", err)
	}
	return result
}

// ---------------------------------------------------------------------------
// A group — count safety, same-batch duplicate names, CREATE scheme gate.
// ---------------------------------------------------------------------------

// Overflow-safe count handling: over-limit counts must refuse like
// ordinary overflow (native 1499, both anchors) — never enter an append
// loop and never fabricate a successful unchanged directory. Counts that
// can overflow the signed domain run only inside the isolated helper
// subprocess (see TestT14R1AddPartitionCountHugeIsolated): on the unfixed
// product this defect is a literal unbounded append loop.
func TestT14R1AddPartitionCountOverflow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		count string
	}{
		{"plain_overflow", "8193"},
		{"base_plus_count_overflow", "8191"}, // p0,p1 base + 8191 = 8193 > 8192
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sql := "ALTER TABLE t ADD PARTITION PARTITIONS " + tc.count + ";\nALTER TABLE t REBUILD PARTITION p0;"
			provider := t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")})
			result := t14Audit(t, spec.DialectMySQL, sql, provider,
				map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleRebuildTarget: {level: "blocker"}})
			findings := t05FindingsByRule(result, 0, t14r1RuleAddApplies)
			if len(findings) != 1 || findings[0].Metadata["native_error"] != "1499" {
				t.Fatalf("count %s must refuse natively 1499, got %#v", tc.count, result.Statements[0].Findings)
			}
			// Proven refusal preserves the exact pre-state — the successor
			// still binds p0 with no partition fact gaps.
			enriched := t14r1Enrich(t, spec.DialectMySQL, sql, t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}))
			snap := t14r1TargetSnapshot(enriched, 1)
			if snap == nil || len(snap.PartitionNames) != 2 || snap.PartitionNames[0] != "p0" || snap.PartitionNames[1] != "p1" {
				t.Fatalf("refused ADD must keep [p0,p1], got %+v", snap)
			}
			if got := t05FindingsByRule(result, 1, t14r1RuleRebuildTarget); len(got) != 0 {
				t.Fatalf("p0 must survive the refused count add, got %#v", got)
			}
		})
	}
}

// MaxInt64/MaxUint64 counts run in a forked helper under a hard deadline:
// the pre-fix defect appends count members without a guard, so the unfixed
// product burns CPU+memory until killed — the deadline IS the failure
// record. On the fixed product the helper refuses 1499 immediately.
func TestT14R1AddPartitionCountHugeIsolated(t *testing.T) {
	if os.Getenv("T14R1_COUNT_HELPER") == "1" {
		count := os.Getenv("T14R1_COUNT")
		sql := "ALTER TABLE t ADD PARTITION PARTITIONS " + count + ";\nALTER TABLE t REBUILD PARTITION p0;"
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleRebuildTarget: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleAddApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1499" {
			t.Fatalf("count %s must refuse natively 1499, got %#v", count, result.Statements[0].Findings)
		}
		enriched := t14r1Enrich(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}))
		snap := t14r1TargetSnapshot(enriched, 1)
		if snap == nil || len(snap.PartitionNames) != 2 {
			t.Fatalf("refused ADD must keep [p0,p1], got %+v", snap)
		}
		return
	}
	t.Parallel()
	for _, count := range []string{"18446744073709551615", "9223372036854775807"} {
		count := count
		t.Run("count_"+count, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestT14R1AddPartitionCountHugeIsolated$", "-test.count=1")
			cmd.Env = append(os.Environ(), "T14R1_COUNT_HELPER=1", "T14R1_COUNT="+count)
			out, err := cmd.CombinedOutput()
			if ctx.Err() == context.DeadlineExceeded {
				t.Fatalf("count %s hung the audit — the unbounded append loop must be guarded before iterating: %s", count, out)
			}
			if err != nil {
				t.Fatalf("helper subprocess failed for count %s: %v\n%s", count, err, out)
			}
		})
	}
}

// In-range counts still execute and publish the extended directory.
func TestT14R1AddPartitionCountInRange(t *testing.T) {
	t.Parallel()
	sql := "ALTER TABLE t ADD PARTITION PARTITIONS 4;\nALTER TABLE t REBUILD PARTITION p5;"
	result := t14Audit(t, spec.DialectMySQL, sql,
		t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}),
		map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleRebuildTarget: {level: "blocker"}})
	if got := t05FindingsByRule(result, 0, t14r1RuleAddApplies); len(got) != 0 {
		t.Fatalf("in-range count add must not refuse, got %#v", got)
	}
	enriched := t14r1Enrich(t, spec.DialectMySQL, sql, t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}))
	snap := t14r1TargetSnapshot(enriched, 1)
	if snap == nil || len(snap.PartitionNames) != 6 {
		t.Fatalf("expected 6 members after +4 on p0,p1, got %+v", snap)
	}
	if got := t05FindingsByRule(result, 1, t14r1RuleRebuildTarget); len(got) != 0 {
		t.Fatalf("p5 must bind after the count add, got %#v", got)
	}
}

// COALESCE PARTITION 0 is grammar-authored — the extractor must keep the
// authored zero so the probed native refusal (1515) preserves the pre-state
// instead of reading the statement as count-less and clearing the
// directory.
func TestT14R1CoalesceZeroAuthored(t *testing.T) {
	t.Parallel()
	sql := "ALTER TABLE t COALESCE PARTITION 0;\nALTER TABLE t REBUILD PARTITION p1;"
	result := t14Audit(t, spec.DialectMySQL, sql,
		t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}),
		map[string]t05RuleConfig{t14r1RuleCoalesceApplies: {level: "blocker"}, t14r1RuleRebuildTarget: {level: "blocker"}})
	findings := t05FindingsByRule(result, 0, t14r1RuleCoalesceApplies)
	if len(findings) != 1 || findings[0].Metadata["native_error"] != "1515" {
		t.Fatalf("authored COALESCE PARTITION 0 must refuse 1515, got %#v", result.Statements[0].Findings)
	}
	enriched := t14r1Enrich(t, spec.DialectMySQL, sql, t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}))
	snap := t14r1TargetSnapshot(enriched, 1)
	if snap == nil || len(snap.PartitionNames) != 2 {
		t.Fatalf("refused coalesce must keep both members, got %+v", snap)
	}
}

// A coalesce count at-or-above the member count refuses 1508 — MaxUint64
// included, judged in the unsigned domain before any int conversion.
func TestT14R1CoalesceHugeCount(t *testing.T) {
	t.Parallel()
	sql := "ALTER TABLE t COALESCE PARTITION 18446744073709551615;\nALTER TABLE t REBUILD PARTITION p1;"
	result := t14Audit(t, spec.DialectMySQL, sql,
		t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}),
		map[string]t05RuleConfig{t14r1RuleCoalesceApplies: {level: "blocker"}, t14r1RuleRebuildTarget: {level: "blocker"}})
	findings := t05FindingsByRule(result, 0, t14r1RuleCoalesceApplies)
	if len(findings) != 1 || findings[0].Metadata["native_error"] != "1508" {
		t.Fatalf("huge coalesce must refuse 1508, got %#v", result.Statements[0].Findings)
	}
	enriched := t14r1Enrich(t, spec.DialectMySQL, sql, t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}))
	snap := t14r1TargetSnapshot(enriched, 1)
	if snap == nil || len(snap.PartitionNames) != 2 {
		t.Fatalf("refused coalesce must keep both members, got %+v", snap)
	}
}

// PARTITION BY count forms: an over-limit count refuses 1499 instead of
// falling into the omitted-count default; a RANGE/LIST count without
// definitions refuses 1492 instead of fabricating implicit members.
func TestT14R1PartitionByCountGuards(t *testing.T) {
	t.Parallel()

	t.Run("hash_count_overflow", func(t *testing.T) {
		t.Parallel()
		sql := "ALTER TABLE t PARTITION BY HASH(id) PARTITIONS 8193;\nALTER TABLE t REBUILD PARTITION p1;"
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}),
			map[string]t05RuleConfig{t14r1RulePByApplies: {level: "blocker"}, t14r1RuleRebuildTarget: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RulePByApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1499" {
			t.Fatalf("PARTITIONS 8193 must refuse 1499, got %#v", result.Statements[0].Findings)
		}
		enriched := t14r1Enrich(t, spec.DialectMySQL, sql, t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}))
		snap := t14r1TargetSnapshot(enriched, 1)
		if snap == nil || len(snap.PartitionNames) != 2 || snap.PartitionNames[0] != "p0" {
			t.Fatalf("refused PARTITION BY must keep [p0,p1], got %+v", snap)
		}
	})

	// `PARTITION BY RANGE(id) PARTITIONS 4` never reaches the state machine:
	// the shared parser itself refuses the count-without-defs form natively
	// with the same 1492 the anchors answer — the parser diagnostic IS the
	// refusal record, no product path may claim a semantic for the shape.
	t.Run("range_count_without_defs_parser_boundary", func(t *testing.T) {
		t.Parallel()
		result := t14r1AuditParserBoundary(t, spec.DialectMySQL,
			"ALTER TABLE t PARTITION BY RANGE(id) PARTITIONS 4;",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeTailShape("t")}),
			map[string]t05RuleConfig{t14r1RulePByApplies: {level: "blocker"}})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Classification != spec.DiagnosticParserError {
			t.Fatalf("RANGE PARTITIONS without defs is a parser boundary (1492) — expected one parser diagnostic, got %+v", result.Diagnostics)
		}
		// The public diagnostic deliberately stays generic; the underlying
		// parse error proves WHICH native refusal this is — [ddl:1492], the
		// same code every anchor answers.
		if _, err := parseSQL(context.Background(), "ALTER TABLE t PARTITION BY RANGE(id) PARTITIONS 4;", spec.DialectMySQL); err == nil || !strings.Contains(err.Error(), "1492") {
			t.Fatalf("the parser refusal must be the native 1492 class, got %v", err)
		}
	})

	// `PARTITIONS 0` never reaches the state machine — the parser refuses
	// it natively (1504) on every anchor. This cell records the boundary
	// so no product path may claim the semantic instead.
	t.Run("parser_rejected_count_zero", func(t *testing.T) {
		t.Parallel()
		result := t14r1AuditParserBoundary(t, spec.DialectMySQL,
			"ALTER TABLE t PARTITION BY HASH(id) PARTITIONS 0;",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}),
			map[string]t05RuleConfig{t14r1RulePByApplies: {level: "blocker"}})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Classification != spec.DiagnosticParserError {
			t.Fatalf("PARTITIONS 0 is a parser boundary (1504) — expected one parser diagnostic, got %+v", result.Diagnostics)
		}
	})
}

// A2 — same-batch duplicate member names refuse natively (1517, case-
// insensitive, both anchors). The refusal preserves the pre-state: no
// duplicate directory is published, no surviving member retires, and the
// successor still reads the old directory.
func TestT14R1SameBatchDuplicateNames(t *testing.T) {
	t.Parallel()

	t.Run("add_defs_duplicate", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t ADD PARTITION (PARTITION p2 VALUES LESS THAN (600), PARTITION p2 VALUES LESS THAN (800));
ALTER TABLE t TRUNCATE PARTITION p2;
ALTER TABLE t TRUNCATE PARTITION p0;`
		provider := func() MetadataProvider {
			return t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeTailShape("t")})
		}
		result := t14Audit(t, spec.DialectMySQL, sql, provider(),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleAddApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1517" {
			t.Fatalf("same-batch duplicate ADD name must refuse 1517, got %#v", result.Statements[0].Findings)
		}
		// p2 was never published; the refused statement keeps p0,p1.
		absent := t05FindingsByRule(result, 1, t14r1RuleTruncateTarget)
		if len(absent) != 1 || absent[0].Metadata["native_error"] != "1735" {
			t.Fatalf("p2 must stay absent after the refused ADD, got %#v", result.Statements[1].Findings)
		}
		if got := t05FindingsByRule(result, 2, t14r1RuleTruncateTarget); len(got) != 0 {
			t.Fatalf("p0 must survive the refused ADD, got %#v", got)
		}
	})

	t.Run("reorg_defs_duplicate", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t REORGANIZE PARTITION p0 INTO (PARTITION p1 VALUES LESS THAN (120), PARTITION p1 VALUES LESS THAN (200));
ALTER TABLE t TRUNCATE PARTITION p0;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}),
			map[string]t05RuleConfig{t14r1RuleReorgApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleReorgApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1517" {
			t.Fatalf("same-batch duplicate REORGANIZE name must refuse 1517, got %#v", result.Statements[0].Findings)
		}
		if got := t05FindingsByRule(result, 1, t14r1RuleTruncateTarget); len(got) != 0 {
			t.Fatalf("p0 must survive the refused REORGANIZE, got %#v", got)
		}
	})

	t.Run("partition_by_duplicate", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t PARTITION BY RANGE(id) (PARTITION q VALUES LESS THAN (200), PARTITION q VALUES LESS THAN MAXVALUE);
ALTER TABLE t TRUNCATE PARTITION p0;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}),
			map[string]t05RuleConfig{t14r1RulePByApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RulePByApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1517" {
			t.Fatalf("PARTITION BY duplicate member name must refuse 1517, got %#v", result.Statements[0].Findings)
		}
		if got := t05FindingsByRule(result, 1, t14r1RuleTruncateTarget); len(got) != 0 {
			t.Fatalf("p0 must survive the refused PARTITION BY, got %#v", got)
		}
	})

	t.Run("case_insensitive_duplicate", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t ADD PARTITION (PARTITION P2 VALUES LESS THAN (600), PARTITION p2 VALUES LESS THAN (800));
ALTER TABLE t TRUNCATE PARTITION p0;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeTailShape("t")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleAddApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1517" {
			t.Fatalf("case-insensitive duplicate name must refuse 1517, got %#v", result.Statements[0].Findings)
		}
	})
}

// A3 — CREATE consumes the declared-scheme validity judgment: a provably
// invalid partition scheme refuses the whole statement natively, so the
// ordered state may not publish the table or its member directory.
func TestT14R1CreateConsumesSchemeValidity(t *testing.T) {
	t.Parallel()

	assertStaysAbsent := func(t *testing.T, createSQL string) {
		t.Helper()
		sql := createSQL + "\nALTER TABLE t TRUNCATE PARTITION p0;"
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{}),
			map[string]t05RuleConfig{t14r1RuleTruncateTarget: {level: "blocker"}})
		enriched := t14r1Enrich(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{}))
		snap := t14r1TargetSnapshot(enriched, 1)
		if snap == nil || snap.Exists {
			t.Fatalf("refused CREATE must not publish t — successor pre-state = %+v", snap)
		}
		_ = result
	}

	t.Run("decreasing_range_bounds", func(t *testing.T) {
		t.Parallel()
		assertStaysAbsent(t, `CREATE TABLE t(id INT) PARTITION BY RANGE(id)
  (PARTITION p0 VALUES LESS THAN (200), PARTITION p1 VALUES LESS THAN (100));`)
	})

	t.Run("duplicate_member_names", func(t *testing.T) {
		t.Parallel()
		assertStaysAbsent(t, `CREATE TABLE t(id INT) PARTITION BY RANGE(id)
  (PARTITION p0 VALUES LESS THAN (200), PARTITION p0 VALUES LESS THAN (400));`)
	})

	t.Run("duplicate_list_values", func(t *testing.T) {
		t.Parallel()
		assertStaysAbsent(t, `CREATE TABLE t(id INT) PARTITION BY LIST(id)
  (PARTITION p0 VALUES IN (1), PARTITION p1 VALUES IN (1));`)
	})

	t.Run("maxvalue_mid", func(t *testing.T) {
		t.Parallel()
		assertStaysAbsent(t, `CREATE TABLE t(id INT) PARTITION BY RANGE(id)
  (PARTITION p0 VALUES LESS THAN MAXVALUE, PARTITION p1 VALUES LESS THAN (200));`)
	})

	t.Run("hash_count_overflow", func(t *testing.T) {
		t.Parallel()
		assertStaysAbsent(t, `CREATE TABLE t(id INT) PARTITION BY HASH(id) PARTITIONS 8193;`)
	})

	t.Run("valid_create_publishes", func(t *testing.T) {
		t.Parallel()
		sql := `CREATE TABLE t(id INT) PARTITION BY RANGE(id)
  (PARTITION p0 VALUES LESS THAN (200), PARTITION p1 VALUES LESS THAN (400));
ALTER TABLE t TRUNCATE PARTITION p1;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{}),
			map[string]t05RuleConfig{t14r1RuleTruncateTarget: {level: "blocker"}})
		enriched := t14r1Enrich(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{}))
		snap := t14r1TargetSnapshot(enriched, 1)
		if snap == nil || !snap.Exists || len(snap.PartitionNames) != 2 {
			t.Fatalf("valid CREATE publishes the partitioned table, got %+v", snap)
		}
		_ = result
	})

	// IF NOT EXISTS on an existing table is the native no-op — the
	// partition clause is never adjudicated and the pre-state stands.
	t.Run("if_not_exists_noop_preserved", func(t *testing.T) {
		t.Parallel()
		sql := `CREATE TABLE IF NOT EXISTS t(id INT) PARTITION BY RANGE(id)
  (PARTITION p0 VALUES LESS THAN (200), PARTITION p1 VALUES LESS THAN (100));
ALTER TABLE t TRUNCATE PARTITION p0;`
		enriched := t14r1Enrich(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}))
		snap := t14r1TargetSnapshot(enriched, 1)
		if snap == nil || !snap.Exists || len(snap.PartitionNames) != 2 || snap.PartitionNames[0] != "p0" {
			t.Fatalf("IF NOT EXISTS no-op must keep the provider shape, got %+v", snap)
		}
	})
}

// ---------------------------------------------------------------------------
// B group — bound fidelity, comparison domains, tri-state propagation.
// ---------------------------------------------------------------------------

// B1 — RANGE COLUMNS bounds keep per-item position and tuple arity: the
// catalog `10,MAXVALUE` projects as [numeric,maxvalue], declared
// `(20,MAXVALUE)` compares at position 0, and an all-MAXVALUE row is the
// +∞ tail. Refused adds keep the exact directory.
func TestT14R1RangeColumnsMaxValue(t *testing.T) {
	t.Parallel()

	t.Run("declared_tuple_bound_publishes_and_compares", func(t *testing.T) {
		t.Parallel()
		// CREATE stores p0=(10,MAXVALUE); the successor ADD (20,MAXVALUE)
		// compares position 0 (10<20) and lands — proof the stored bound
		// kept its tuple shape rather than collapsing to a scalar tail.
		sql := `CREATE TABLE t(a INT, b INT) PARTITION BY RANGE COLUMNS(a,b)
  (PARTITION p0 VALUES LESS THAN (10,MAXVALUE));
ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES LESS THAN (20,MAXVALUE));
ALTER TABLE t TRUNCATE PARTITION p1;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		if got := t05FindingsByRule(result, 1, t14r1RuleAddApplies); len(got) != 0 {
			t.Fatalf("ADD (20,MAXVALUE) after (10,MAXVALUE) must succeed, got %#v", result.Statements[1].Findings)
		}
		enriched := t14r1Enrich(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{}))
		snap := t14r1TargetSnapshot(enriched, 2)
		if snap == nil || len(snap.PartitionNames) != 2 {
			t.Fatalf("expected [p0,p1] after the tuple add, got %+v", snap)
		}
	})

	t.Run("catalog_tuple_bound_compares_by_position", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES LESS THAN (20,MAXVALUE));
ALTER TABLE t TRUNCATE PARTITION p1;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeColsShape("t", "10,MAXVALUE")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		if got := t05FindingsByRule(result, 0, t14r1RuleAddApplies); len(got) != 0 {
			t.Fatalf("catalog `10,MAXVALUE` + ADD (20,MAXVALUE) must compare 10<20 and succeed, got %#v", result.Statements[0].Findings)
		}
		if got := t05FindingsByRule(result, 1, t14r1RuleTruncateTarget); len(got) != 0 {
			t.Fatalf("p1 must bind after the tuple add, got %#v", got)
		}
	})

	t.Run("tuple_bound_decreasing_refuses", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES LESS THAN (5,MAXVALUE));
ALTER TABLE t TRUNCATE PARTITION p1;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeColsShape("t", "10,MAXVALUE")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleAddApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1493" {
			t.Fatalf("ADD (5,MAXVALUE) under (10,MAXVALUE) must refuse 1493, got %#v", result.Statements[0].Findings)
		}
		absent := t05FindingsByRule(result, 1, t14r1RuleTruncateTarget)
		if len(absent) != 1 || absent[0].Metadata["native_error"] != "1735" {
			t.Fatalf("p1 must stay absent after the refused add, got %#v", result.Statements[1].Findings)
		}
	})

	t.Run("all_maxvalue_row_is_tail", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL,
			"ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES LESS THAN (20,MAXVALUE));",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeColsShape("t", "10,MAXVALUE", "MAXVALUE,MAXVALUE")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleAddApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1481" {
			t.Fatalf("ADD under an all-MAXVALUE tail must refuse 1481, got %#v", result.Statements[0].Findings)
		}
	})

	t.Run("alter_partition_by_tuple_bounds", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t PARTITION BY RANGE COLUMNS(a,b)
  (PARTITION q0 VALUES LESS THAN (15,MAXVALUE), PARTITION q1 VALUES LESS THAN (30,MAXVALUE));
ALTER TABLE t TRUNCATE PARTITION q1;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeColsShape("t", "10,MAXVALUE")}),
			map[string]t05RuleConfig{t14r1RulePByApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		if got := t05FindingsByRule(result, 0, t14r1RulePByApplies); len(got) != 0 {
			t.Fatalf("PARTITION BY with increasing tuple bounds must not refuse, got %#v", result.Statements[0].Findings)
		}
		if got := t05FindingsByRule(result, 1, t14r1RuleTruncateTarget); len(got) != 0 {
			t.Fatalf("q1 must bind after the PARTITION BY redefinition, got %#v", got)
		}
	})
}

// B2 — byte-order Go comparisons never fabricate a string verdict: 'a'
// versus 'B' under an unobserved collation stays unproven, the directory
// unvouches instead of publishing or refusing, and equal bytes still prove.
func TestT14R1StringBoundDomain(t *testing.T) {
	t.Parallel()

	t.Run("lower_after_upper_unproven", func(t *testing.T) {
		t.Parallel()
		// catalog bound 'B'; ADD 'a' — byte order would say a>B (success),
		// case-insensitive collation would refuse. Neither may be claimed.
		sql := `ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES LESS THAN ('a'));
ALTER TABLE t TRUNCATE PARTITION p0;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeColsShape("t", "'B'")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		if got := result.Statements[0].Findings; len(got) != 0 {
			t.Fatalf("unproven string ordering must emit no verdict finding, got %#v", got)
		}
		enriched := t14r1Enrich(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeColsShape("t", "'B'")}))
		snap := t14r1TargetSnapshot(enriched, 1)
		if snap == nil || snap.PartitionNames != nil {
			t.Fatalf("unproven ordering must unvouch the member directory, got %+v", snap)
		}
		if !t14r1GapSet(result.Statements[1])["target_table.partitions"] {
			t.Fatalf("successor must owe the unvouched partitions fact, got %+v", result.Statements[1].EvidenceGaps)
		}
	})

	t.Run("upper_after_lower_unproven", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES LESS THAN ('B'));
ALTER TABLE t TRUNCATE PARTITION p0;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeColsShape("t", "'a'")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		if got := result.Statements[0].Findings; len(got) != 0 {
			t.Fatalf("byte-order-decreasing 'B'<'a' must not fabricate a 1493 refusal, got %#v", got)
		}
		enriched := t14r1Enrich(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeColsShape("t", "'a'")}))
		snap := t14r1TargetSnapshot(enriched, 1)
		if snap != nil && snap.PartitionNames != nil {
			t.Fatalf("unproven ordering must not keep the vouched directory, got %+v", snap)
		}
	})

	t.Run("byte_equal_strings_prove", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES LESS THAN ('a'));
ALTER TABLE t TRUNCATE PARTITION p0;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeColsShape("t", "'a'")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleAddApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1493" {
			t.Fatalf("equal string bound provably fails increasing order — expect 1493, got %#v", result.Statements[0].Findings)
		}
		enriched := t14r1Enrich(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeColsShape("t", "'a'")}))
		snap := t14r1TargetSnapshot(enriched, 1)
		if snap == nil || len(snap.PartitionNames) != 1 {
			t.Fatalf("refused add keeps the pre-state directory, got %+v", snap)
		}
	})

	t.Run("list_columns_string_overlap", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES IN ('B'));
ALTER TABLE t TRUNCATE PARTITION p0;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1ListColsShape("t", "'a'")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		if got := result.Statements[0].Findings; len(got) != 0 {
			t.Fatalf("unproven LIST string overlap must emit no verdict, got %#v", got)
		}
		enriched := t14r1Enrich(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1ListColsShape("t", "'a'")}))
		snap := t14r1TargetSnapshot(enriched, 1)
		if snap != nil && snap.PartitionNames != nil {
			t.Fatalf("unproven overlap must unvouch the directory, got %+v", snap)
		}
	})

	t.Run("list_columns_byte_equal_overlap_refuses", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES IN ('a'));
ALTER TABLE t TRUNCATE PARTITION p0;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1ListColsShape("t", "'a'")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleAddApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1495" {
			t.Fatalf("byte-equal LIST value provably overlaps — expect 1495, got %#v", result.Statements[0].Findings)
		}
	})
}

// B3 — LIST overlap is tri-state: proven overlap refuses, proven disjoint
// executes, and an uncomparable bound (expression item, unprojectable
// catalog shell) publishes nothing — it unvouches the directory and leaves
// the capability boundary to coverage.
func TestT14R1ListOverlapTriState(t *testing.T) {
	t.Parallel()

	t.Run("expression_bound_unproven", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t ADD PARTITION (PARTITION p2 VALUES IN (1+0));
ALTER TABLE t TRUNCATE PARTITION p2;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1ListShape("t")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		enriched := t14r1Enrich(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1ListShape("t")}))
		snap := t14r1TargetSnapshot(enriched, 1)
		if snap != nil {
			for _, n := range snap.PartitionNames {
				if n == "p2" {
					t.Fatalf("unproven bound must not publish p2, got %v", snap.PartitionNames)
				}
			}
		}
		if result.Statements[0].Coverage.Status != report.CoverageIncomplete {
			t.Fatalf("expression bound is a capability boundary — coverage must be incomplete, got %+v", result.Statements[0].Coverage)
		}
		if !t14r1HasFeature(result, "alter_table.add_partition.partition_bound") {
			t.Fatalf("expected the partition_bound aspect, got %+v", result.Unsupported)
		}
	})

	t.Run("proven_overlap_refuses", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t ADD PARTITION (PARTITION p2 VALUES IN (1));
ALTER TABLE t TRUNCATE PARTITION p0;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1ListShape("t")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleAddApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1495" {
			t.Fatalf("proven LIST overlap must refuse 1495, got %#v", result.Statements[0].Findings)
		}
		if got := t05FindingsByRule(result, 1, t14r1RuleTruncateTarget); len(got) != 0 {
			t.Fatalf("p0 must survive the refused add, got %#v", got)
		}
	})

	t.Run("proven_disjoint_executes", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t ADD PARTITION (PARTITION p2 VALUES IN (5));
ALTER TABLE t TRUNCATE PARTITION p2;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1ListShape("t")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		if got := t05FindingsByRule(result, 0, t14r1RuleAddApplies); len(got) != 0 {
			t.Fatalf("disjoint LIST add must not refuse, got %#v", got)
		}
		if got := t05FindingsByRule(result, 1, t14r1RuleTruncateTarget); len(got) != 0 {
			t.Fatalf("p2 must bind after the disjoint add, got %#v", got)
		}
	})

	t.Run("catalog_bound_shell_unproven", func(t *testing.T) {
		t.Parallel()
		shape := t14r1ListShape("t")
		// A member whose bound the catalog could not project: the clause
		// exists (kind set) but carries no comparable rows.
		shape.PartitionDefs[0].Bound = &spec.PartitionBound{Kind: "in"}
		sql := `ALTER TABLE t ADD PARTITION (PARTITION p2 VALUES IN (5));
ALTER TABLE t TRUNCATE PARTITION p0;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": shape}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		if got := result.Statements[0].Findings; len(got) != 0 {
			t.Fatalf("unproven catalog shell must emit no verdict, got %#v", got)
		}
		if !t14r1GapSet(result.Statements[0])["target_table.partition_defs"] {
			t.Fatalf("the rule consuming an unproven bound owes target_table.partition_defs, got %+v", result.Statements[0].EvidenceGaps)
		}
		enriched := t14r1Enrich(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": shape}))
		snap := t14r1TargetSnapshot(enriched, 1)
		if snap != nil && snap.PartitionNames != nil {
			t.Fatalf("unproven overlap must unvouch the directory, got %+v", snap)
		}
	})
}

// B4 — ADD below the tail is the probed 1493 (both anchors); REORGANIZE
// span-shrink keeps its own code (1520 MySQL / 1493 TiDB).
func TestT14R1AddBoundNativeCode(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect)+"_add_1493", func(t *testing.T) {
			t.Parallel()
			result := t14Audit(t, dialect,
				"ALTER TABLE t ADD PARTITION (PARTITION p2 VALUES LESS THAN (300));",
				t14ProviderFor(dialect, map[string]*spec.TableSnapshot{"t": t14r1RangeTailShape("t")}),
				map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}})
			findings := t05FindingsByRule(result, 0, t14r1RuleAddApplies)
			if len(findings) != 1 || findings[0].Metadata["native_error"] != "1493" {
				t.Fatalf("ADD below tail must refuse 1493 on %s, got %#v", dialect, result.Statements[0].Findings)
			}
		})
	}

	t.Run("reorg_keeps_1520_mysql", func(t *testing.T) {
		t.Parallel()
		// Directory [p0<200, p1<400]: the reorg narrows p0 to 150 — a proven
		// span shrink (1520). The replacement name must not collide with the
		// surviving member, or the name check legitimately fires first.
		result := t14Audit(t, spec.DialectMySQL,
			"ALTER TABLE t REORGANIZE PARTITION p0 INTO (PARTITION px VALUES LESS THAN (150));",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeTailShape("t")}),
			map[string]t05RuleConfig{t14r1RuleReorgTarget: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleReorgTarget)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1520" {
			t.Fatalf("REORGANIZE span-shrink keeps 1520 on MySQL, got %#v", result.Statements[0].Findings)
		}
	})
}

// ---------------------------------------------------------------------------
// C group — statement gaps/coverage mirror the incomplete judgment.
// ---------------------------------------------------------------------------

// C1 — the ADD target rule owes the catalog facts it consumes: a defs-form
// ADD on an unvouched member space owes target_table.partitions; a range
// ADD without projected bounds owes target_table.partition_defs. Known
// directories stay gap-free.
func TestT14R1AddTargetOwesDirectoryFacts(t *testing.T) {
	t.Parallel()

	t.Run("defs_add_hash_names_nil", func(t *testing.T) {
		t.Parallel()
		partitioned := true
		method := "hash"
		shape := &spec.TableSnapshot{
			Exists: true, Table: &spec.Table{Name: "t"},
			Columns: []spec.Column{t10r2Col("id", "int", true, 0)}, Constraints: []spec.Constraint{},
			Partitioned: &partitioned, PartitionType: &method,
		}
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t ADD PARTITION (PARTITION p9);",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": shape}),
			map[string]t05RuleConfig{t14r1RuleAddTarget: {level: "blocker"}})
		stmt := result.Statements[0]
		if !t14r1GapSet(stmt)["target_table.partitions"] {
			t.Fatalf("defs-form ADD on an unvouched member space owes target_table.partitions, got %+v", stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status == report.CoverageComplete {
			t.Fatalf("an owed fact cannot read as complete coverage, got %+v", stmt.Coverage)
		}
	})

	t.Run("range_add_defs_nil", func(t *testing.T) {
		t.Parallel()
		partitioned := true
		method := "range"
		shape := &spec.TableSnapshot{
			Exists: true, Table: &spec.Table{Name: "t"},
			Columns: []spec.Column{t10r2Col("id", "int", true, 0)}, Constraints: []spec.Constraint{},
			Partitioned: &partitioned, PartitionType: &method,
			PartitionNames: []string{"p0", "p1"},
		}
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t ADD PARTITION (PARTITION p9 VALUES LESS THAN (600));",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": shape}),
			map[string]t05RuleConfig{t14r1RuleAddTarget: {level: "blocker"}})
		stmt := result.Statements[0]
		if !t14r1GapSet(stmt)["target_table.partition_defs"] {
			t.Fatalf("range ADD with unvouched bounds owes target_table.partition_defs, got %+v", stmt.EvidenceGaps)
		}
	})

	t.Run("known_directory_no_gap", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t ADD PARTITION (PARTITION p9);",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}),
			map[string]t05RuleConfig{t14r1RuleAddTarget: {level: "blocker"}})
		stmt := result.Statements[0]
		if gaps := t14r1GapSet(stmt); gaps["target_table.partitions"] || gaps["target_table.partition_defs"] {
			t.Fatalf("a vouched directory owes no partition facts, got %+v", stmt.EvidenceGaps)
		}
	})
}

// C2 — a structurally-known bound expression is still an unimplemented
// capability: every declaration channel (CREATE defs, ADD/REORG defs,
// PARTITION BY scheme defs) reports the partition_bound aspect with
// incomplete coverage, while provider-side unproven bound shells stay
// metadata evidence gaps. Provable sibling findings still stand.
func TestT14R1ExpressionBoundCoverage(t *testing.T) {
	t.Parallel()

	t.Run("partition_by_scheme_defs", func(t *testing.T) {
		t.Parallel()
		sql := `ALTER TABLE t PARTITION BY RANGE(id)
  (PARTITION p0 VALUES LESS THAN (100+100), PARTITION p1 VALUES LESS THAN (150));
ALTER TABLE t TRUNCATE PARTITION p0;`
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}),
			map[string]t05RuleConfig{t14r1RulePByApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		stmt := result.Statements[0]
		if stmt.Coverage.Status != report.CoverageIncomplete {
			t.Fatalf("scheme-def expression bounds are unaudited — expect incomplete, got %+v", stmt.Coverage)
		}
		if !t14r1HasFeature(result, "alter_table.partition_by.partition_bound") {
			t.Fatalf("expected the partition_by bound aspect, got %+v", result.Unsupported)
		}
		// The unproven redefinition unvouches partitioning facts; the
		// successor owes the partitioned gap, not a fabricated directory.
		if !t14r1GapSet(result.Statements[1])["target_table.partitioned"] {
			t.Fatalf("unvouched partition state owes target_table.partitioned, got %+v", result.Statements[1].EvidenceGaps)
		}
	})

	t.Run("create_defs", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, `
CREATE TABLE t(id INT) PARTITION BY RANGE(id)
  (PARTITION p0 VALUES LESS THAN (100+100), PARTITION p1 VALUES LESS THAN (150));
`,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{}),
			map[string]t05RuleConfig{})
		stmt := result.Statements[0]
		if stmt.Coverage.Status != report.CoverageIncomplete {
			t.Fatalf("CREATE-def expression bounds are unaudited — expect incomplete, got %+v", stmt.Coverage)
		}
		if !t14r1HasFeature(result, "create_table.partition_bound") {
			t.Fatalf("expected the create bound aspect, got %+v", result.Unsupported)
		}
	})

	t.Run("add_defs", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL,
			"ALTER TABLE t ADD PARTITION (PARTITION p2 VALUES LESS THAN (100+100));",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeTailShape("t")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}})
		stmt := result.Statements[0]
		if stmt.Coverage.Status != report.CoverageIncomplete {
			t.Fatalf("ADD-def expression bound is unaudited — expect incomplete, got %+v", stmt.Coverage)
		}
		if !t14r1HasFeature(result, "alter_table.add_partition.partition_bound") {
			t.Fatalf("expected the add bound aspect, got %+v", result.Unsupported)
		}
	})

	// A provably-invalid sibling name set still reports its refusal even
	// when another bound stays uncomparable — the unproven dimension never
	// swallows proven findings.
	t.Run("proven_dup_survives_unproven_bound", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL,
			"ALTER TABLE t ADD PARTITION (PARTITION p2 VALUES LESS THAN (100+100), PARTITION p2 VALUES LESS THAN (800));",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1RangeTailShape("t")}),
			map[string]t05RuleConfig{t14r1RuleAddApplies: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleAddApplies)
		found := false
		for _, f := range findings {
			if f.Metadata["native_error"] == "1517" {
				found = true
			}
		}
		if !found {
			t.Fatalf("the provable duplicate-name refusal must survive the uncomparable bound, got %#v", findings)
		}
	})
}

// ---------------------------------------------------------------------------
// D group — vendor, engine, and maintenance state stay consistent.
// ---------------------------------------------------------------------------

// D1 — all six maintenance actions share the TiDB vendor boundary: the
// statement keeps its vendor aspect and emits zero governance findings.
// DISCARD previously re-used the T13 forbid without the gate.
func TestT14R1TiDBMaintenanceVendorUnified(t *testing.T) {
	t.Parallel()
	actions := []struct {
		sql     string
		feature string
	}{
		{"ALTER TABLE t CHECK PARTITION p0;", "alter_table.check_partition"},
		{"ALTER TABLE t REBUILD PARTITION p0;", "alter_table.rebuild_partition"},
		{"ALTER TABLE t REPAIR PARTITION p0;", "alter_table.repair_partition"},
		{"ALTER TABLE t OPTIMIZE PARTITION p0;", "alter_table.optimize_partition"},
		{"ALTER TABLE t IMPORT PARTITION p0 TABLESPACE;", "alter_table.import_partition_tablespace"},
		{"ALTER TABLE t DISCARD PARTITION p0 TABLESPACE;", "alter_table.discard_partition_tablespace"},
	}
	for _, tc := range actions {
		tc := tc
		t.Run(tc.feature, func(t *testing.T) {
			t.Parallel()
			result := t14Audit(t, spec.DialectTiDB, tc.sql,
				t14ProviderFor(spec.DialectTiDB, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}),
				map[string]t05RuleConfig{
					t14r1RuleDiscardForbid:  {level: "blocker"},
					t14r1RuleCheckWarn:      {level: "blocker"},
					t14r1RuleOptimizeWarn:   {level: "blocker"},
					t14r1RuleRebuildApplies: {level: "blocker"},
					t14r1RuleDiscardApplies: {level: "blocker"},
					t14r1RuleImportApplies:  {level: "blocker"},
				})
			stmt := result.Statements[0]
			if len(stmt.Findings) != 0 {
				t.Fatalf("%s on TiDB must emit no governance finding, got %#v", tc.feature, stmt.Findings)
			}
			if stmt.Coverage.Status != report.CoverageIncomplete {
				t.Fatalf("%s keeps its vendor boundary, got %+v", tc.feature, stmt.Coverage)
			}
			if !t14r1HasFeature(result, tc.feature) {
				t.Fatalf("expected the %s vendor aspect, got %+v", tc.feature, result.Unsupported)
			}
			if result.Verdict == report.VerdictReject {
				t.Fatalf("%s vendor boundary never rejects, got %s", tc.feature, result.Verdict)
			}
		})
	}

	t.Run("mysql_discard_forbid_control", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t DISCARD PARTITION p0 TABLESPACE;",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1EngineShape("t", "innodb")}),
			map[string]t05RuleConfig{t14r1RuleDiscardForbid: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleDiscardForbid)
		if len(findings) != 1 || findings[0].Level != "blocker" {
			t.Fatalf("MySQL discard keeps its forbid blocker, got %#v", result.Statements[0].Findings)
		}
	})
}

// D2 — a proven vendor refusal preserves the exact pre-state: the TiDB
// statement keeps its aspect, and the successor still binds p0 from the
// untouched directory — columns, PK, and partition facts all stand.
func TestT14R1VendorRefusalPreservesPreState(t *testing.T) {
	t.Parallel()
	actions := []string{
		"ALTER TABLE t CHECK PARTITION p0;",
		"ALTER TABLE t REBUILD PARTITION p0;",
		"ALTER TABLE t REPAIR PARTITION p0;",
		"ALTER TABLE t OPTIMIZE PARTITION p0;",
		"ALTER TABLE t IMPORT PARTITION p0 TABLESPACE;",
		"ALTER TABLE t DISCARD PARTITION p0 TABLESPACE;",
	}
	for _, sql := range actions {
		sql := sql
		t.Run(sql[13:len(sql)-1], func(t *testing.T) {
			t.Parallel()
			full := sql + "\nALTER TABLE t TRUNCATE PARTITION p0;"
			result := t14Audit(t, spec.DialectTiDB, full,
				t14ProviderFor(spec.DialectTiDB, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}),
				map[string]t05RuleConfig{t14r1RuleTruncateTarget: {level: "blocker"}})
			enriched := t14r1Enrich(t, spec.DialectTiDB, full,
				t14ProviderFor(spec.DialectTiDB, map[string]*spec.TableSnapshot{"t": t14RangeShape("t")}))
			snap := t14r1TargetSnapshot(enriched, 1)
			if snap == nil || snap.Partitioned == nil || !*snap.Partitioned {
				t.Fatalf("vendor refusal must keep the partitioned pre-state, got %+v", snap)
			}
			if len(snap.PartitionNames) != 2 || snap.PartitionNames[0] != "p0" {
				t.Fatalf("vendor refusal must keep [p0,pmax], got %v", snap.PartitionNames)
			}
			if snap.PrimaryKey == nil || len(snap.Indexes) != 1 {
				t.Fatalf("vendor refusal must keep the index/key facts, got %+v", snap)
			}
			if got := t05FindingsByRule(result, 1, t14r1RuleTruncateTarget); len(got) != 0 {
				t.Fatalf("p0 must bind after the vendor refusal, got %#v", got)
			}
			if result.Statements[1].Coverage.Status != report.CoverageComplete {
				t.Fatalf("successor reads the preserved state — expect complete, got %+v", result.Statements[1].Coverage)
			}
		})
	}
}

// D3 — DISCARD/IMPORT consume the proven engine: MyISAM is the probed 1031
// refusal on MySQL, InnoDB executes, and an unobserved engine owes the
// fact instead of guessing either way.
func TestT14R1EngineApplicability(t *testing.T) {
	t.Parallel()

	t.Run("myisam_refuses_1031", func(t *testing.T) {
		t.Parallel()
		sql := "ALTER TABLE t DISCARD PARTITION p0 TABLESPACE;\nALTER TABLE t TRUNCATE PARTITION p0;"
		result := t14Audit(t, spec.DialectMySQL, sql,
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1EngineShape("t", "myisam")}),
			map[string]t05RuleConfig{t14r1RuleDiscardApplies: {level: "blocker"}, t14r1RuleTruncateTarget: {level: "blocker"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleDiscardApplies)
		if len(findings) != 1 || findings[0].Metadata["native_error"] != "1031" {
			t.Fatalf("MyISAM discard must refuse 1031, got %#v", result.Statements[0].Findings)
		}
		if got := t05FindingsByRule(result, 1, t14r1RuleTruncateTarget); len(got) != 0 {
			t.Fatalf("p0 must survive the refused discard, got %#v", got)
		}
	})

	t.Run("innodb_executes", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t DISCARD PARTITION p0 TABLESPACE;",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1EngineShape("t", "innodb")}),
			map[string]t05RuleConfig{t14r1RuleDiscardApplies: {level: "blocker"}})
		if got := t05FindingsByRule(result, 0, t14r1RuleDiscardApplies); len(got) != 0 {
			t.Fatalf("InnoDB discard is applicable — no refusal finding, got %#v", got)
		}
	})

	t.Run("engine_unobserved_owes_fact", func(t *testing.T) {
		t.Parallel()
		shape := t14RangeShape("t")
		shape.Options = nil
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t DISCARD PARTITION p0 TABLESPACE;",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": shape}),
			map[string]t05RuleConfig{t14r1RuleDiscardApplies: {level: "blocker"}})
		stmt := result.Statements[0]
		if !t14r1GapSet(stmt)["target_table.engine"] {
			t.Fatalf("unobserved engine owes target_table.engine, got %+v", stmt.EvidenceGaps)
		}
		if len(stmt.Findings) != 0 {
			t.Fatalf("unobserved engine must not fabricate a verdict, got %#v", stmt.Findings)
		}
	})
}

// D4 — OPTIMIZE on a proven InnoDB engine reports the native scope: all
// partitions rebuild and analyze, never the authored member list.
func TestT14R1OptimizeScope(t *testing.T) {
	t.Parallel()

	t.Run("innodb_reports_all_partitions", func(t *testing.T) {
		t.Parallel()
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t OPTIMIZE PARTITION p0;",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14r1EngineShape("t", "innodb")}),
			map[string]t05RuleConfig{t14r1RuleOptimizeWarn: {level: "warning"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleOptimizeWarn)
		if len(findings) != 1 {
			t.Fatalf("expected the optimize warning, got %#v", result.Statements[0].Findings)
		}
		if findings[0].Metadata["engine"] != "innodb" {
			t.Fatalf("the scope finding must carry the proven engine, got %+v", findings[0].Metadata)
		}
	})

	t.Run("engine_unknown_stays_qualified", func(t *testing.T) {
		t.Parallel()
		shape := t14RangeShape("t")
		shape.Options = nil
		result := t14Audit(t, spec.DialectMySQL, "ALTER TABLE t OPTIMIZE PARTITION p0;",
			t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": shape}),
			map[string]t05RuleConfig{t14r1RuleOptimizeWarn: {level: "warning"}})
		findings := t05FindingsByRule(result, 0, t14r1RuleOptimizeWarn)
		if len(findings) != 1 {
			t.Fatalf("expected the optimize warning, got %#v", result.Statements[0].Findings)
		}
		if _, has := findings[0].Metadata["engine"]; has {
			t.Fatalf("an unobserved engine must not be fabricated into metadata, got %+v", findings[0].Metadata)
		}
	})
}

// State transitions never depend on governance rules being enabled: the
// proven refusals above preserve the pre-state even under an all-off
// policy, and the successor still reads the old directory.
func TestT14R1RefusalStateIndependentOfRules(t *testing.T) {
	t.Parallel()
	sql := "ALTER TABLE t ADD PARTITION PARTITIONS 8193;\nALTER TABLE t REBUILD PARTITION p0;"
	result := t14Audit(t, spec.DialectMySQL, sql,
		t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}),
		map[string]t05RuleConfig{})
	enriched := t14r1Enrich(t, spec.DialectMySQL, sql,
		t14ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": t14HashShape("t")}))
	snap := t14r1TargetSnapshot(enriched, 1)
	if snap == nil || len(snap.PartitionNames) != 2 || snap.PartitionNames[0] != "p0" {
		t.Fatalf("the proven refusal preserves the directory with rules off, got %+v", snap)
	}
	_ = result
}

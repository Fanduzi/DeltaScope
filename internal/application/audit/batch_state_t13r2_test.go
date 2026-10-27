// Package audit verifies the T13-R2 ordered-state and mounting
// remediations on the public Audit seam: a DROP post-state no longer
// proves the old member directory, incomplete declared/provider catalogs
// stay unknown instead of masquerading as complete, the exchange-only
// metadata shape still mounts, and unproven modifier combinations keep
// their coverage boundary on the current statement.
package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t13r2RuleDropTarget     = "ddl.alter.drop_partition.target"
	t13r2RuleTruncateTarget = "ddl.alter.truncate_partition.target"
	t13r2RuleExchangeTarget = "ddl.alter.exchange_partition.target"
)

func t13r2Audit(t *testing.T, sql string, provider MetadataProvider, ids ...string) report.Result {
	t.Helper()
	enabled := map[string]string{}
	for _, id := range ids {
		enabled[id] = ""
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          spec.DialectMySQL,
		Schema:           "golden",
		ConfigPath:       t05PolicyPath(t, enabled),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	return result
}

// A1: a proven DROP post-state no longer vouches the removed member — the
// successor targeting the dropped name owes the real absent-member
// finding, while a surviving member still proves present. Successor DROP,
// TRUNCATE, and EXCHANGE all read the same post-state.
func TestT13R2DropPostStateSuccessorTargets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		successor string
		ruleID    string
		nativeErr string
	}{
		{"successor drop", "ALTER TABLE t DROP PARTITION p0", t13r2RuleDropTarget, "1507"},
		{"successor truncate", "ALTER TABLE t TRUNCATE PARTITION p0", t13r2RuleTruncateTarget, "1735"},
		{"successor exchange", "ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE x", t13r2RuleExchangeTarget, "1735"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := t13r1ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{
				"t": t13r1RangeShape("t"),
				"x": {Exists: true, Table: &spec.Table{Name: "x"}, Partitioned: t13r1BoolP(false)},
			})
			result := t13r2Audit(t,
				"ALTER TABLE t DROP PARTITION p0;\n"+tc.successor+";",
				provider, tc.ruleID)
			findings := t05FindingsByRule(result, 1, tc.ruleID)
			if len(findings) != 1 || findings[0].Metadata["reason_code"] != "partition_absent" ||
				findings[0].Metadata["native_error"] != tc.nativeErr {
				t.Fatalf("dropped member must not stay proven — %s owes partition_absent/%s, got %#v",
					tc.successor, tc.nativeErr, result.Statements[1].Findings)
			}
		})
	}
}

// A1 control: a surviving member still proves, and the modifier+DROP form
// (proven-executed on the versioned anchor) leaves the same precise
// post-state — never the stale directory.
func TestT13R2DropPostStateSurvivorAndModifier(t *testing.T) {
	t.Parallel()
	provider := t13r1ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{
		"t": t13r1RangeShape("t"),
	})
	result := t13r2Audit(t,
		"ALTER TABLE t ALGORITHM=INPLACE, DROP PARTITION p0;\nALTER TABLE t TRUNCATE PARTITION p0;\nALTER TABLE t TRUNCATE PARTITION p1;",
		provider, t13r2RuleTruncateTarget)
	// The surviving member is still provable after the executed drop.
	if got := t05FindingsByRule(result, 2, t13r2RuleTruncateTarget); len(got) != 0 {
		t.Fatalf("surviving member p1 must still prove, got %#v", result.Statements[2].Findings)
	}
	// The dropped member owes the absent finding under the modifier form.
	if got := t05FindingsByRule(result, 1, t13r2RuleTruncateTarget); len(got) != 1 ||
		got[0].Metadata["reason_code"] != "partition_absent" {
		t.Fatalf("modifier+drop must leave the same precise post-state, got %#v", result.Statements[1].Findings)
	}
}

// A2: a declared subpartitioned directory is not a complete member proof —
// a leaf name exchange must owe the partitions gap, never fabricate
// partition_absent (MySQL accepts the leaf name natively).
func TestT13R2SubpartitionedDirectoryStaysUnknown(t *testing.T) {
	t.Parallel()
	result := t13r2Audit(t,
		"CREATE TABLE t(id INT) PARTITION BY RANGE(id) SUBPARTITION BY HASH(id) SUBPARTITIONS 2 "+
			"(PARTITION p0 VALUES LESS THAN(10), PARTITION p1 VALUES LESS THAN(20));\n"+
			"CREATE TABLE x(id INT);\n"+
			"ALTER TABLE t EXCHANGE PARTITION p0sp0 WITH TABLE x;",
		nil, t13r2RuleExchangeTarget)
	exchange := result.Statements[2]
	if got := t05FindingsByRule(result, 2, t13r2RuleExchangeTarget); len(got) != 0 {
		t.Fatalf("unprojected leaf must not be judged proven-absent, got %#v", exchange.Findings)
	}
	gaps := t05GapsByRule(result, 2, t13r2RuleExchangeTarget)
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.partitions" {
		t.Fatalf("incomplete declared directory owes target_table.partitions, got %+v", exchange.EvidenceGaps)
	}
	if exchange.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
		t.Fatalf("incomplete directory holds unverified/review, got %s/%s",
			exchange.Coverage.Status, result.Verdict)
	}
}

// A2: a declared TiDB RANGE INTERVAL directory is never expanded into a
// member proof — the named TRUNCATE owes the partitions gap.
func TestT13R2IntervalDirectoryStaysUnknown(t *testing.T) {
	t.Parallel()
	enabled := map[string]string{t13r2RuleTruncateTarget: ""}
	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE TABLE t(id INT) PARTITION BY RANGE(id) INTERVAL(10) FIRST PARTITION LESS THAN(10) LAST PARTITION LESS THAN(30);\n" +
			"ALTER TABLE t TRUNCATE PARTITION p_lt_10;",
		Dialect:    spec.DialectTiDB,
		Schema:     "golden",
		ConfigPath: t05PolicyPath(t, enabled),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	truncate := result.Statements[1]
	if got := t05FindingsByRule(result, 1, t13r2RuleTruncateTarget); len(got) != 0 {
		t.Fatalf("unexpanded interval directory must not judge member absence, got %#v", truncate.Findings)
	}
	gaps := t05GapsByRule(result, 1, t13r2RuleTruncateTarget)
	if len(gaps) != 1 || gaps[0].RequiredFacts[0] != "target_table.partitions" {
		t.Fatalf("interval directory owes target_table.partitions, got %+v", truncate.EvidenceGaps)
	}
}

// A2 control: a provider-loaded subpartitioned table likewise owns no
// projected member proof — nil names already gap, asserted end-to-end.
func TestT13R2ProviderSubpartitionedDirectoryStaysUnknown(t *testing.T) {
	t.Parallel()
	provider := t13r1ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{
		// The catalog answer for a subpartitioned table: top-level names
		// exist but the member directory is incomplete — the provider
		// projects it as unobserved, never as the complete top-level list.
		"t": {Exists: true, Table: &spec.Table{Name: "t"},
			Partitioned: t13r1BoolP(true), PartitionType: t13r1StrP("range")},
		"x": {Exists: true, Table: &spec.Table{Name: "x"}, Partitioned: t13r1BoolP(false)},
	})
	result := t13r2Audit(t,
		"ALTER TABLE t EXCHANGE PARTITION p0sp0 WITH TABLE x;",
		provider, t13r2RuleExchangeTarget)
	exchange := result.Statements[0]
	if got := t05FindingsByRule(result, 0, t13r2RuleExchangeTarget); len(got) != 0 {
		t.Fatalf("incomplete provider directory must not fabricate absence, got %#v", exchange.Findings)
	}
	gaps := t05GapsByRule(result, 0, t13r2RuleExchangeTarget)
	if len(gaps) != 1 || gaps[0].RequiredFacts[0] != "target_table.partitions" {
		t.Fatalf("incomplete provider directory owes target_table.partitions, got %+v", exchange.EvidenceGaps)
	}
}

// C1: a statement carrying only the exchange-table fact still mounts
// Metadata — the declared counterpart's proven partitioning reaches the
// rule as the 1732 finding, never degraded to an existence gap.
func TestT13R2ExchangeOnlyMetadataMounts(t *testing.T) {
	t.Parallel()
	// No schema, no version, no provider: the statement carries ONLY the
	// declared exchange-table fact — the mount condition must honor it.
	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE TABLE t2(id INT) PARTITION BY HASH(id) PARTITIONS 2;\n" +
			"ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2;",
		Dialect:    spec.DialectMySQL,
		ConfigPath: t05PolicyPath(t, map[string]string{"ddl.alter.exchange_partition.counterpart": ""}),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[1]
	findings := t05FindingsByRule(result, 1, "ddl.alter.exchange_partition.counterpart")
	if len(findings) != 1 || findings[0].Metadata["reason_code"] != "counterpart_partitioned" ||
		findings[0].Metadata["native_error"] != "1732" {
		t.Fatalf("mounted exchange fact must prove 1732, got findings=%#v gaps=%+v",
			stmt.Findings, stmt.EvidenceGaps)
	}
}

// C1 control: a non-EXCHANGE statement never observes a counterpart, so a
// plain partitioned successor touching t2 sees no phantom exchange fact.
func TestT13R2NonExchangeNeverSeesCounterpart(t *testing.T) {
	t.Parallel()
	result := t13r2Audit(t,
		"CREATE TABLE t2(id INT) PARTITION BY HASH(id) PARTITIONS 2;\n"+
			"ALTER TABLE t2 DROP PARTITION p0;",
		nil, t13r2RuleDropTarget, "ddl.alter.exchange_partition.counterpart")
	// t2 is partitioned — a normal DROP sees its own target's directory,
	// not a counterpart fact; p0/p1 are the implicit members.
	if got := t05FindingsByRule(result, 1, t13r2RuleDropTarget); len(got) != 0 {
		t.Fatalf("declared hash members p0..p1 must prove for a plain drop, got %#v", result.Statements[1].Findings)
	}
	if got := t05FindingsByRule(result, 1, "ddl.alter.exchange_partition.counterpart"); len(got) != 0 ||
		len(t05GapsByRule(result, 1, "ddl.alter.exchange_partition.counterpart")) != 0 {
		t.Fatalf("non-exchange statement carries no counterpart binding, got findings=%#v gaps=%+v",
			result.Statements[1].Findings, result.Statements[1].EvidenceGaps)
	}
}

// C2: an unproven modifier combination keeps its coverage boundary on the
// current statement — ALGORITHM=COPY never reached a probed verdict, so the
// statement must not be laundered into complete/pass by the ordered-state
// invalidation alone.
func TestT13R2UnprovenModifierKeepsCoverageBoundary(t *testing.T) {
	t.Parallel()
	provider := t13r1ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{
		"t": t13r1RangeShape("t"),
	})
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "ALTER TABLE t ALGORITHM=COPY, DROP PARTITION p0;",
		Dialect:          spec.DialectMySQL,
		Schema:           "golden",
		ConfigPath:       t05PolicyPath(t, map[string]string{t13r2RuleDropTarget: ""}),
		MetadataProvider: provider,
	})
	// The current statement carries the unproven modifier boundary — the
	// service surfaces it as ErrUnsupportedStatement with the result intact.
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("unproven ALGORITHM=COPY combination must surface as unsupported, got err=%v", err)
	}
	stmt := result.Statements[0]
	if stmt.Coverage.Status == report.CoverageComplete {
		t.Fatalf("unproven modifier combination must not report complete, got %s", stmt.Coverage.Status)
	}
	found := false
	for _, detail := range result.Unsupported {
		if detail.Index == 0 && detail.Feature == "alter_table.algorithm" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unproven modifier keeps its unsupported aspect on the current statement, got %+v",
			result.Unsupported)
	}
}

// C2 control: a proven modifier combination (ALGORITHM=DEFAULT) still
// removes the aspect — the statement stays complete under the same gate.
func TestT13R2ProvenModifierKeepsCompleteCoverage(t *testing.T) {
	t.Parallel()
	provider := t13r1ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{
		"t": t13r1RangeShape("t"),
	})
	result := t13r2Audit(t,
		"ALTER TABLE t ALGORITHM=DEFAULT, DROP PARTITION p0;",
		provider, t13r2RuleDropTarget)
	for _, detail := range result.Unsupported {
		if detail.Index == 0 && detail.Feature == "alter_table.algorithm" {
			t.Fatalf("proven ALGORITHM=DEFAULT+DROP consumes the aspect, got %+v", result.Unsupported)
		}
	}
}

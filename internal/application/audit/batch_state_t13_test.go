// Package audit verifies the T13 destructive/data-moving partition path
// end to end: the three actions are audited coverage-complete, the frozen
// rules emit their governed findings on the action alone, the exchanged
// table enters the protected-object target set in both directions, and the
// ordered state keeps member facts while dropping data-derived statistics —
// with a native multi-spec refusal preserving the exact pre-state.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t13RuleDropPartition     = "ddl.alter.drop_partition.forbid"
	t13RuleTruncatePartition = "ddl.alter.truncate_partition.forbid"
	t13RuleExchangePartition = "ddl.alter.exchange_partition.warn"
	t13RuleDenylist          = "ddl.table.denylist.forbid"
)

// t13Shape is a partitioned table carrying provable member facts plus
// data-derived statistics that a landed partition action can never vouch
// for anymore.
func t13Shape(name string) *spec.TableSnapshot {
	partitioned := true
	cardinality := int64(7)
	return &spec.TableSnapshot{
		Exists:      true,
		Table:       &spec.Table{Name: name},
		Columns:     []spec.Column{t10r2Col("id", "int", true, 0), t10r2Col("c", "int", true, 0)},
		PrimaryKey:  &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}, Cardinality: &cardinality},
		Indexes:     []spec.Index{{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, Cardinality: &cardinality}},
		Constraints: []spec.Constraint{},
		Partitioned: &partitioned,
		Options: map[string]string{
			"table_rows": "9", "data_length": "100", "index_length": "40",
			"avg_row_length": "8", "auto_increment": "4", "comment": "kept",
		},
	}
}

// t13StatKeys are the data-derived facts a partition DROP/TRUNCATE/EXCHANGE
// destroys or moves — they can never survive the transition.
var t13StatKeys = []string{"table_rows", "data_length", "index_length", "avg_row_length", "auto_increment"}

func t13AssertStatsCleared(t *testing.T, shape *spec.TableSnapshot, what string) {
	t.Helper()
	for _, key := range t13StatKeys {
		if _, ok := shape.Options[key]; ok {
			t.Fatalf("%s must drop Options[%s], got %+v", what, key, shape.Options)
		}
	}
	if shape.PrimaryKey != nil && shape.PrimaryKey.Cardinality != nil {
		t.Fatalf("%s must drop PK cardinality, got %+v", what, shape.PrimaryKey.Cardinality)
	}
	for i, index := range shape.Indexes {
		if index.Cardinality != nil {
			t.Fatalf("%s must drop index %d cardinality", what, i)
		}
	}
}

func t13AssertShapeKept(t *testing.T, shape *spec.TableSnapshot, what string) {
	t.Helper()
	if shape == nil || !shape.Exists {
		t.Fatalf("%s must keep the present shape, got %+v", what, shape)
	}
	if len(shape.Columns) != 2 || shape.FindColumn("id") == nil || shape.FindColumn("c") == nil {
		t.Fatalf("%s must keep member columns, got %+v", what, shape.Columns)
	}
	if shape.PrimaryKey == nil || len(shape.Indexes) != 1 {
		t.Fatalf("%s must keep PK and secondary index, got %+v", what, shape)
	}
	if shape.Options["comment"] != "kept" {
		t.Fatalf("%s must keep non-statistic options, got %+v", what, shape.Options)
	}
	if shape.Partitioned == nil || !*shape.Partitioned {
		// The engine refuses the last partition drop, and every refusal
		// keeps pre-state — partitioned can never flip under T13 actions.
		t.Fatalf("%s must keep Partitioned=true, got %+v", what, shape.Partitioned)
	}
}

// A landed DROP/TRUNCATE partition keeps every provable member fact and
// drops every data-derived statistic on the one named table.
func TestT13PartitionSingleActionPostState(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, tc := range []struct{ name, sql string }{
			{"drop", "ALTER TABLE t DROP PARTITION p0"},
			{"drop_multi_name", "ALTER TABLE t DROP PARTITION p0, p1"},
			{"truncate_named", "ALTER TABLE t TRUNCATE PARTITION p0"},
			{"truncate_all", "ALTER TABLE t TRUNCATE PARTITION ALL"},
		} {
			tc := tc
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				entry := t10r4Apply(t, dialect, t13Shape("t"), tc.sql)
				if entry == nil || entry.state != tablePresent || entry.shape == nil {
					t.Fatalf("partition action must keep the present entry, got %+v", entry)
				}
				t13AssertShapeKept(t, entry.shape, tc.sql)
				t13AssertStatsCleared(t, entry.shape, tc.sql)
			})
		}
	}
}

// EXCHANGE PARTITION moves data on BOTH object identities — the counterpart
// table's data-derived statistics invalidate alongside the partitioned
// table's; member facts survive on both.
func TestT13ExchangePostStateClearsBothSides(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			version := "8.0.46"
			if dialect == spec.DialectTiDB {
				version = "8.0.11-TiDB-v8.5.0"
			}
			provider := &t10Provider{version: version}
			state := newBatchState(dialect, "golden", provider)
			state.resolvedVersion = t10r4Version(dialect)
			state.entries[state.keyFor("golden", spec.Table{Name: "t"})] =
				&batchTableEntry{state: tablePresent, shape: t13Shape("t")}
			state.entries[state.keyFor("golden", spec.Table{Name: "t2"})] =
				&batchTableEntry{state: tablePresent, shape: t13Shape("t2")}
			parsed, err := parseSQL(context.Background(),
				"ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2 WITHOUT VALIDATION", dialect)
			if err != nil || len(parsed.Statements) != 1 {
				t.Fatalf("parse: %v", err)
			}
			statements, err := Extract(context.Background(), parsed)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			statements[0].Metadata = &spec.Metadata{Version: t10r4Version(dialect)}
			if err := state.apply(context.Background(), statements[0]); err != nil {
				t.Fatalf("apply: %v", err)
			}
			for _, name := range []string{"t", "t2"} {
				entry := state.entries[state.keyFor("golden", spec.Table{Name: name})]
				if entry == nil || entry.state != tablePresent || entry.shape == nil {
					t.Fatalf("exchange side %s must keep the present entry, got %+v", name, entry)
				}
				t13AssertShapeKept(t, entry.shape, "exchange "+name)
				t13AssertStatsCleared(t, entry.shape, "exchange "+name)
			}
		})
	}
}

// A partition action sharing a multi-spec ALTER is a proven native refusal
// on every anchor (MySQL 1064, TiDB 8200): the exact pre-state survives —
// no column added, no statistic dropped.
func TestT13PartitionMultiSpecRefusalKeepsPreState(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, tc := range []struct{ name, sql string }{
			{"drop_after_add", "ALTER TABLE t ADD COLUMN x INT, DROP PARTITION p0"},
			{"truncate_sibling", "ALTER TABLE t DROP COLUMN c, TRUNCATE PARTITION p0"},
			{"exchange_sibling", "ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2, ADD COLUMN x INT"},
		} {
			tc := tc
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				entry := t10r4Apply(t, dialect, t13Shape("t"), tc.sql)
				if entry == nil || entry.state != tablePresent || entry.shape == nil {
					t.Fatalf("refused statement must keep the present entry, got %+v", entry)
				}
				shape := entry.shape
				if len(shape.Columns) != 2 || shape.FindColumn("x") != nil {
					t.Fatalf("refused %q must not add the sibling column, got %+v", tc.sql, shape.Columns)
				}
				for _, key := range t13StatKeys {
					if shape.Options[key] == "" {
						t.Fatalf("refused %q must keep Options[%s], got %+v", tc.sql, key, shape.Options)
					}
				}
				if shape.PrimaryKey.Cardinality == nil || shape.Indexes[0].Cardinality == nil {
					t.Fatalf("refused %q must keep cardinalities", tc.sql)
				}
			})
		}
	}
}

// --- Public Audit-path governance ---

// The first golden path: an isolated DROP PARTITION policy emits exactly
// one blocker bound to {table, partition}, coverage is complete, and the
// verdict rejects — on both dialects.
func TestT13DropPartitionGovernedPublicPath(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:        "ALTER TABLE t DROP PARTITION p0;",
				Dialect:    dialect,
				Schema:     "golden",
				ConfigPath: t05PolicyPath(t, map[string]string{t13RuleDropPartition: "      forbid: true\n"}),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected one statement, got %d", len(result.Statements))
			}
			stmt := result.Statements[0]
			if stmt.Coverage.Status != report.CoverageComplete {
				t.Fatalf("coverage = %+v, want complete — drop_partition must be audited", stmt.Coverage)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("unsupported = %+v, want none", result.Unsupported)
			}
			if result.Verdict != report.VerdictReject {
				t.Fatalf("verdict = %s, want reject", result.Verdict)
			}
			findings := t05FindingsByRule(result, 0, t13RuleDropPartition)
			if len(findings) != 1 {
				t.Fatalf("expected exactly one drop-partition finding, got %#v", stmt.Findings)
			}
			f := findings[0]
			if f.Level != "blocker" || f.Metadata["table"] != "t" || f.Metadata["partition"] != "p0" {
				t.Fatalf("finding = %#v, want blocker {table=t partition=p0}", f)
			}
			if len(stmt.EvidenceGaps) != 0 {
				t.Fatalf("the action is fully proven — no gaps expected, got %+v", stmt.EvidenceGaps)
			}
		})
	}
}

// Multiple named partitions produce one finding per member in clause order.
func TestT13DropPartitionMultipleNames(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:        "ALTER TABLE t DROP PARTITION p0, p1;",
		Dialect:    spec.DialectMySQL,
		Schema:     "golden",
		ConfigPath: t05PolicyPath(t, map[string]string{t13RuleDropPartition: "      forbid: true\n"}),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	findings := t05FindingsByRule(result, 0, t13RuleDropPartition)
	if len(findings) != 2 || findings[0].Metadata["partition"] != "p0" || findings[1].Metadata["partition"] != "p1" {
		t.Fatalf("want one finding per named partition in order, got %#v", findings)
	}
}

// TRUNCATE PARTITION is a distinct governed action: named and ALL forms
// each fire the truncate rule — never the drop rule, never fabricated names.
func TestT13TruncatePartitionGoverned(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		sql       string
		wantMeta  map[string]string
		wantCount int
	}{
		{name: "named", sql: "ALTER TABLE t TRUNCATE PARTITION p0", wantMeta: map[string]string{"partition": "p0"}, wantCount: 1},
		{name: "all", sql: "ALTER TABLE t TRUNCATE PARTITION ALL", wantMeta: map[string]string{"all_partitions": "true"}, wantCount: 1},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:        tc.sql + ";",
				Dialect:    spec.DialectMySQL,
				Schema:     "golden",
				ConfigPath: t05PolicyPath(t, map[string]string{t13RuleTruncatePartition: "      forbid: true\n"}),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			stmt := result.Statements[0]
			if stmt.Coverage.Status != report.CoverageComplete {
				t.Fatalf("coverage = %+v, want complete", stmt.Coverage)
			}
			findings := t05FindingsByRule(result, 0, t13RuleTruncatePartition)
			if len(findings) != tc.wantCount {
				t.Fatalf("want %d truncate findings, got %#v", tc.wantCount, findings)
			}
			for key, want := range tc.wantMeta {
				if findings[0].Metadata[key] != want {
					t.Fatalf("truncate metadata[%s] = %q, want %q in %#v", key, findings[0].Metadata[key], want, findings[0].Metadata)
				}
			}
			if got := t05FindingsByRule(result, 0, t13RuleDropPartition); len(got) != 0 {
				t.Fatalf("truncate must never fire the drop rule, got %#v", got)
			}
		})
	}
}

// EXCHANGE PARTITION warns with both object identities and the effective
// validation clause.
func TestT13ExchangePartitionGoverned(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		sql          string
		wantValidate string
	}{
		{name: "absent clause", sql: "ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2", wantValidate: "with"},
		{name: "without", sql: "ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE s2.t2 WITHOUT VALIDATION", wantValidate: "without"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:        tc.sql + ";",
				Dialect:    spec.DialectMySQL,
				Schema:     "golden",
				ConfigPath: t05PolicyPathConfigured(t, map[string]t05RuleConfig{t13RuleExchangePartition: {level: "warning"}}),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			stmt := result.Statements[0]
			if stmt.Coverage.Status != report.CoverageComplete {
				t.Fatalf("coverage = %+v, want complete", stmt.Coverage)
			}
			findings := t05FindingsByRule(result, 0, t13RuleExchangePartition)
			if len(findings) != 1 {
				t.Fatalf("want exactly one exchange warning, got %#v", stmt.Findings)
			}
			f := findings[0]
			if f.Level != "warning" {
				t.Fatalf("exchange default level = %s, want warning", f.Level)
			}
			if f.Metadata["table"] != "t" || f.Metadata["partition"] != "p0" ||
				f.Metadata["exchange_table"] != "t2" || f.Metadata["validation"] != tc.wantValidate {
				t.Fatalf("exchange metadata = %#v", f.Metadata)
			}
			if result.Verdict == report.VerdictReject {
				t.Fatalf("a lone warning must not reject, got %s", result.Verdict)
			}
		})
	}
}

// The exchanged table is a named object identity on the statement — the
// denylist protects it in both directions.
func TestT13ExchangeDenylistBothDirections(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		sql     string
		params  string
		hitName string
	}{
		{
			name:   "counterpart side",
			sql:    "ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2",
			params: "      tables:\n        - t2\n", hitName: "t2",
		},
		{
			name:   "partitioned side",
			sql:    "ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2",
			params: "      tables:\n        - t\n", hitName: "t",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:        tc.sql + ";",
				Dialect:    spec.DialectMySQL,
				Schema:     "golden",
				ConfigPath: t05PolicyPath(t, map[string]string{t13RuleDenylist: tc.params}),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			findings := t05FindingsByRule(result, 0, t13RuleDenylist)
			if len(findings) == 0 {
				t.Fatalf("denylist must hit %q through the exchange target set, got %#v", tc.hitName, result.Statements[0].Findings)
			}
			found := false
			for _, f := range findings {
				if f.Metadata["table"] == tc.hitName {
					found = true
				}
			}
			if !found {
				t.Fatalf("denylist findings must name %q, got %#v", tc.hitName, findings)
			}
		})
	}
}

// Disabling the rule removes the finding but never the audited-capability
// answer: coverage stays complete either way.
func TestT13DisabledRuleKeepsCompleteCoverage(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:        "ALTER TABLE t DROP PARTITION p0;",
		Dialect:    spec.DialectMySQL,
		Schema:     "golden",
		ConfigPath: t05PolicyPath(t, map[string]string{}),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("coverage = %+v, want complete — audited capability is not policy", stmt.Coverage)
	}
	if len(stmt.Findings) != 0 || result.Verdict != report.VerdictPass {
		t.Fatalf("all-disabled policy: want no findings + pass, got findings=%+v verdict=%s", stmt.Findings, result.Verdict)
	}
}

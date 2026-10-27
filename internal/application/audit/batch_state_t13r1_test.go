// Package audit verifies the T13-R1 remediation on the public Audit seam:
// ordered-state EXCHANGE preloads the counterpart so a never-touched table
// enters with cleared data-derived statistics, and the multi-spec refusal
// answer narrows to the natively proven shapes instead of every two-spec
// ALTER carrying a partition action.
package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t13r1StatsPolicy pins the observable seams the successor statements read:
// the alter existence rule keeps a table-state gap signal, the truncate-rows
// rule turns surviving data-derived statistics into a finding, and the
// add-column existence rule turns surviving member facts into a finding.
func t13r1StatsPolicy(t *testing.T) string {
	t.Helper()
	return t05PolicyPathConfigured(t, map[string]t05RuleConfig{
		t05RuleAlterRequire:                 {level: "blocker"},
		t05RuleAddColumnForbid:              {level: "blocker"},
		"ddl.table.truncate.rows.max_count": {level: "blocker", params: "      limit: 1\n"},
	})
}

// t13r1CounterpartShape is the provider's batch-prior answer for a table the
// batch never touched — members plus every data-derived statistic a landed
// EXCHANGE destroys.
func t13r1CounterpartShape(name string) *spec.TableSnapshot {
	partitioned := false
	cardinality := int64(11)
	return &spec.TableSnapshot{
		Exists:      true,
		Table:       &spec.Table{Name: name},
		Columns:     []spec.Column{t10r2Col("id", "int", true, 0), t10r2Col("c", "int", true, 0)},
		PrimaryKey:  &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}, Cardinality: &cardinality},
		Indexes:     []spec.Index{{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}, Cardinality: &cardinality}},
		Constraints: []spec.Constraint{},
		Partitioned: &partitioned,
		Options: map[string]string{
			"table_rows": "42", "data_length": "2048", "index_length": "1024",
			"avg_row_length": "48", "auto_increment": "50", "comment": "kept",
		},
	}
}

func t13r1ProviderFor(dialect spec.Dialect, tables map[string]*spec.TableSnapshot) *t10Provider {
	version := "8.0.46"
	if dialect == spec.DialectTiDB {
		version = "8.0.11-TiDB-v8.5.0"
	}
	return &t10Provider{version: version, tables: tables}
}

func t13r1T2Reads(provider *t10Provider) int {
	reads := 0
	for _, call := range provider.calls {
		if call == "golden.t2" {
			reads++
		}
	}
	return reads
}

// B: EXCHANGE PARTITION touches a counterpart the batch never saw. Without
// a preload the first-read path serves the provider's batch-prior snapshot
// — stale data-derived statistics a landed exchange already destroyed —
// while the provider's own object stays untouched.
func TestT13R1ExchangeFirstTouchCounterpartClearsStats(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			counterpart := t13r1CounterpartShape("t2")
			provider := t13r1ProviderFor(dialect, map[string]*spec.TableSnapshot{
				"t":  t13Shape("t"),
				"t2": counterpart,
			})
			result, err := AuditSQL(context.Background(), Request{
				SQL: "ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2 WITHOUT VALIDATION;\n" +
					"TRUNCATE TABLE t2;\n" +
					"ALTER TABLE t2 ADD COLUMN c INT;",
				Dialect:          dialect,
				Schema:           "golden",
				ConfigPath:       t13r1StatsPolicy(t),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 3 {
				t.Fatalf("expected three statements, got %d", len(result.Statements))
			}
			// The counterpart entered the ordered state through the exchange's
			// preload — exactly once, never a second batch-prior read.
			if reads := t13r1T2Reads(provider); reads != 1 {
				t.Fatalf("counterpart must resolve through one ordered-state read, got calls=%#v", provider.calls)
			}
			// Data-derived statistics did NOT survive the exchange: the
			// post-exchange shape can no longer vouch 42 rows for TRUNCATE —
			// this is the read that exposes a missing preload (the provider's
			// batch-prior snapshot would answer instead).
			if got := t05FindingsByRule(result, 1, "ddl.table.truncate.rows.max_count"); len(got) != 0 {
				t.Fatalf("stale batch-prior statistics must not answer the truncate, got %#v",
					result.Statements[1].Findings)
			}
			// Member facts survived the exchange: adding the existing column c
			// still collides against the post-exchange shape.
			if got := t05FindingsByRule(result, 2, t05RuleAddColumnForbid); len(got) != 1 {
				t.Fatalf("member columns must survive the exchange — duplicate add must fire, got %#v",
					result.Statements[2].Findings)
			}
			// The provider's own object was never mutated into service.
			if _, ok := counterpart.Options["table_rows"]; !ok {
				t.Fatalf("provider snapshot must stay untouched, got %+v", counterpart.Options)
			}
			if counterpart.PrimaryKey.Cardinality == nil || counterpart.Indexes[0].Cardinality == nil {
				t.Fatalf("provider cardinality facts must stay untouched, got %+v", counterpart)
			}
		})
	}
}

// B control: an explicitly schema-qualified counterpart resolves the same
// keyed identity — the preload must not leak into a second schema's table.
func TestT13R1ExchangeQualifiedCounterpart(t *testing.T) {
	t.Parallel()
	counterpart := t13r1CounterpartShape("t2")
	provider := t13r1ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{
		"t":  t13Shape("t"),
		"t2": counterpart,
	})
	result, err := AuditSQL(context.Background(), Request{
		SQL: "ALTER TABLE golden.t EXCHANGE PARTITION p0 WITH TABLE golden.t2 WITHOUT VALIDATION;\n" +
			"TRUNCATE TABLE t2;",
		Dialect:          spec.DialectMySQL,
		Schema:           "golden",
		ConfigPath:       t13r1StatsPolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if reads := t13r1T2Reads(provider); reads != 1 {
		t.Fatalf("qualified counterpart resolves once through golden.t2, got calls=%#v", provider.calls)
	}
	if got := t05FindingsByRule(result, 1, "ddl.table.truncate.rows.max_count"); len(got) != 0 {
		t.Fatalf("stale statistics must not answer the qualified truncate, got %#v",
			result.Statements[1].Findings)
	}
}

// C: `ALGORITHM=INPLACE, DROP PARTITION` executes natively on MySQL 8.0 and
// TiDB (probed 2026-10-28) — the post-state clears statistics like any
// landed partition mutation, never preserving the stale pre-state as if the
// engine had refused the statement.
func TestT13R1AlgorithmDropPartitionExecutes(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := t13r1ProviderFor(dialect, map[string]*spec.TableSnapshot{
				"t": t13Shape("t"),
			})
			result, err := AuditSQL(context.Background(), Request{
				SQL: "ALTER TABLE t ALGORITHM=INPLACE, DROP PARTITION p0;\n" +
					"TRUNCATE TABLE t;",
				Dialect:          dialect,
				Schema:           "golden",
				ConfigPath:       t13r1StatsPolicy(t),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			// The executed drop cleared statistics — TRUNCATE has no surviving
			// table_rows fact to read.
			if got := t05FindingsByRule(result, 1, "ddl.table.truncate.rows.max_count"); len(got) != 0 {
				t.Fatalf("combinable ALGORITHM+drop must not keep serving pre-state statistics, got %#v",
					result.Statements[1].Findings)
			}
			if gaps := result.Statements[1].EvidenceGaps; len(gaps) != 0 {
				t.Fatalf("executed drop leaves a vouched post-state — no gap expected, got %+v", gaps)
			}
		})
	}
}

// C control: MySQL 5.7 natively refuses `ALGORITHM=INPLACE, DROP PARTITION`
// (error 1846) — the exact pre-state survives there; without a version fact
// the outcome is unprovable and the entry must not keep serving stale
// statistics as if the drop had landed.
func TestT13R1AlgorithmDropPartitionVersionGate(t *testing.T) {
	t.Parallel()
	t.Run("mysql57 refused keeps pre-state", func(t *testing.T) {
		t.Parallel()
		provider := &t10Provider{
			version: "5.7.44",
			tables:  map[string]*spec.TableSnapshot{"t": t13Shape("t")},
		}
		result, err := AuditSQL(context.Background(), Request{
			SQL: "ALTER TABLE t ALGORITHM=INPLACE, DROP PARTITION p0;\n" +
				"TRUNCATE TABLE t;",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t13r1StatsPolicy(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		// The refused statement kept every data-derived statistic — TRUNCATE
		// still reads the surviving table_rows fact.
		if got := t05FindingsByRule(result, 1, "ddl.table.truncate.rows.max_count"); len(got) != 1 {
			t.Fatalf("MySQL 5.7 refuses the combo — surviving statistics must answer, got %#v",
				result.Statements[1].Findings)
		}
	})
	t.Run("unobserved version stays unproven", func(t *testing.T) {
		t.Parallel()
		provider := &t10Provider{
			version: "",
			tables:  map[string]*spec.TableSnapshot{"t": t13Shape("t")},
		}
		result, err := AuditSQL(context.Background(), Request{
			SQL: "ALTER TABLE t ALGORITHM=INPLACE, DROP PARTITION p0;\n" +
				"ALTER TABLE t ADD COLUMN x INT;",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			ConfigPath:       t13r1StatsPolicy(t),
			MetadataProvider: provider,
		})
		// Without a version fact the INPLACE+DROP combination is unproven —
		// the current statement keeps its unsupported aspect (T13-R2 C2).
		if !errors.Is(err, ErrUnsupportedStatement) {
			t.Fatalf("unproven modifier combination must surface as unsupported, got err=%v", err)
		}
		successor := result.Statements[1]
		if len(successor.EvidenceGaps) == 0 {
			t.Fatalf("unprovable transition must hold a review gap on the successor, got none")
		}
		if got := t05FindingsByRule(result, 1, "ddl.table.truncate.rows.max_count"); len(got) != 0 {
			t.Fatalf("unprovable outcome must not keep serving stale statistics, got %#v", got)
		}
	})
}

// Structural siblings remain a proven refusal — a structural spec leading a
// partition tail is refused on every anchor (MySQL 1064, TiDB 8200) and the
// exact pre-state stands. A partition action followed by any sibling is
// unreachable here: the locked grammar only admits partition_options as the
// alter-list tail, so the parser refuses the form upstream — the audit
// reports it unparseable rather than inventing a state.
func TestT13R1StructuralSiblingStillRefused(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, tc := range []struct{ name, sql string }{
			{"structural_leads", "ALTER TABLE t ADD COLUMN x INT, DROP PARTITION p0"},
			{"partition_leads_unparseable", "ALTER TABLE t DROP PARTITION p0, ADD COLUMN x INT"},
		} {
			tc := tc
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				provider := t13r1ProviderFor(dialect, map[string]*spec.TableSnapshot{
					"t": t13Shape("t"),
				})
				result, err := AuditSQL(context.Background(), Request{
					SQL:              tc.sql + ";\nTRUNCATE TABLE t;",
					Dialect:          dialect,
					Schema:           "golden",
					ConfigPath:       t13r1StatsPolicy(t),
					MetadataProvider: provider,
				})
				if tc.name == "partition_leads_unparseable" {
					// The locked grammar refuses the form at parse: it never
					// reaches the classifier, and the surviving TRUNCATE
					// inherits a contaminated batch — no facts answer.
					if len(result.Statements) != 1 {
						t.Fatalf("only the TRUNCATE survives an unparseable first statement, got %d statements (err=%v)",
							len(result.Statements), err)
					}
					if got := t05FindingsByRule(result, 0, "ddl.table.truncate.rows.max_count"); len(got) != 0 {
						t.Fatalf("contamination must not keep serving stale statistics, got %#v", got)
					}
					if len(result.Statements[0].Findings) != 0 {
						t.Fatalf("contaminated successor must stay silent, got %#v",
							result.Statements[0].Findings)
					}
					return
				}
				if err != nil {
					t.Fatalf("audit: %v", err)
				}
				// The refused mix kept every data-derived statistic — TRUNCATE
				// still reads the surviving table_rows fact.
				if got := t05FindingsByRule(result, 1, "ddl.table.truncate.rows.max_count"); len(got) != 1 {
					t.Fatalf("refused %q keeps surviving statistics, got %#v", tc.sql,
						result.Statements[1].Findings)
				}
			})
		}
	}
}

// A-public coverage/verdict pinning lives in the companion file — it
// consumes the new spec fields whose compile red is recorded separately.

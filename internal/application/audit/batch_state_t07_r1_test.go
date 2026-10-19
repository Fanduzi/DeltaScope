// Package audit verifies the T07-R1 semantic closure: declaration-time
// generated-column reference premises and the complete TiDB expression-
// default structure boundary.
// input: real-parser AuditSQL runs over the frozen R1 inputs with isolated
// policies and controlled provider snapshots
// output: exact reason_code findings, bounded evidence gaps, ordered-state
// publication and tombstone behavior, plus MySQL opposite controls
// pos: T07-R1 regression seam beside the frozen T07 matrix suite
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// r1CompatOnly enables only the advanced-compatibility rule at blocker.
func r1CompatOnly(t *testing.T) string {
	t.Helper()
	return t07PolicyPath(t, map[string]string{t07RuleCompat: "      required: true\n"})
}

// r1TiDBProvider returns a provider answering the v VARCHAR(36), x INT
// fixture table at the pinned TiDB 8.5.0 identity.
func r1TiDBProvider(columns ...spec.Column) *t05PresentProvider {
	provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
		"golden.t": {
			Exists:      true,
			Schema:      "golden",
			Table:       &spec.Table{Schema: "golden", Name: "t"},
			Columns:     columns,
			Indexes:     []spec.Index{},
			Constraints: []spec.Constraint{},
		},
	}}
	provider.t07InstanceVersion = "8.0.11-TiDB-v8.5.0"
	return provider
}

// TestT07R1GeneratedReferencePremises runs the frozen R1-1 inputs through the
// real parser and AuditSQL on both dialects: ordinary references at any
// ordinal are legal; self-reference, absent names, and references to
// generated columns declared later are determined rejections; a same-named
// string literal is not a reference.
func TestT07R1GeneratedReferencePremises(t *testing.T) {
	t.Parallel()
	for _, variant := range []struct {
		dialect spec.Dialect
		version string
	}{
		{spec.DialectMySQL, "8.0.46"},
		{spec.DialectTiDB, "8.5.0"},
	} {
		variant := variant
		t.Run(string(variant.dialect), func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name       string
				sql        string
				wantReason string
				wantGap    bool
			}{
				{"g_valid", "CREATE TABLE t (a INT, b INT AS (a + 1) STORED)", "", false},
				{"g_base_forward", "CREATE TABLE t (b INT AS (a + 1) STORED, a INT)", "", false},
				{"g_literal", "CREATE TABLE t (a INT, b VARCHAR(32) AS ('missing_c') VIRTUAL)", "", false},
				{"g_self", "CREATE TABLE t (a INT, b INT AS (b + 1) STORED)", "generated_self_reference", false},
				{"g_missing", "CREATE TABLE t (a INT, b INT AS (missing_c + 1) STORED)", "generated_missing_column", false},
				{"g_generated_forward", "CREATE TABLE t (a INT, b INT AS (c + 1) STORED, c INT AS (a + 1) STORED)", "generated_column_order", false},
				{"g_cycle", "CREATE TABLE t (a INT, b INT AS (c + 1) STORED, c INT AS (b + 1) STORED)", "generated_column_order", false},
				{"qualified_self_table", "CREATE TABLE t (a INT, b INT AS (t.a + 1) STORED)", "", false},
				{"qualified_other_table", "CREATE TABLE t (a INT, b INT AS (other.a + 1) STORED)", "", true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					result, err := AuditSQL(context.Background(), Request{
						SQL:           tc.sql,
						Dialect:       variant.dialect,
						Schema:        "golden",
						TargetVersion: variant.version,
						ConfigPath:    r1CompatOnly(t),
					})
					if err != nil {
						t.Fatalf("audit: %v", err)
					}
					if len(result.Statements) != 1 {
						t.Fatalf("statements = %d, want 1", len(result.Statements))
					}
					findings := result.Statements[0].Findings
					gaps := result.Statements[0].EvidenceGaps
					switch {
					case tc.wantReason != "":
						if len(findings) != 1 || findings[0].RuleID != t07RuleCompat ||
							findings[0].Metadata["reason_code"] != tc.wantReason ||
							findings[0].Metadata["feature"] != "generated" {
							t.Fatalf("findings = %+v, want one %s finding", findings, tc.wantReason)
						}
					case tc.wantGap:
						if len(findings) != 0 {
							t.Fatalf("findings = %+v, want none", findings)
						}
						if len(gaps) == 0 {
							t.Fatalf("gaps = none, want a bounded evidence gap")
						}
						for _, gap := range gaps {
							if gap.ReasonCode != "incomplete_source_column" {
								t.Fatalf("gap reason = %q, want incomplete_source_column", gap.ReasonCode)
							}
						}
					default:
						if len(findings) != 0 || len(gaps) != 0 {
							t.Fatalf("findings/gaps = %+v/%+v, want none", findings, gaps)
						}
					}
				})
			}
		})
	}
}

// TestT07R1GeneratedPremisePostState proves a determined premise rejection
// tombstones the created identity: the follow-up index cannot reuse a
// fabricated column set, and disabling the compat rule does not resurrect
// the invalid shape.
func TestT07R1GeneratedPremisePostState(t *testing.T) {
	t.Parallel()
	sql := "CREATE TABLE t (a INT, b INT AS (missing_c + 1) STORED);\nCREATE INDEX idx_a ON t (a);"
	t.Run("compat_enabled", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:           sql,
			Dialect:       spec.DialectMySQL,
			Schema:        "golden",
			TargetVersion: "8.0.46",
			ConfigPath: t07PolicyPath(t, map[string]string{
				t07RuleCompat: "      required: true\n",
				"ddl.create_index.columns.exists.require": "      required: true\n",
			}),
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements) != 2 {
			t.Fatalf("statements = %d, want 2", len(result.Statements))
		}
		findings := result.Statements[0].Findings
		if len(findings) != 1 || findings[0].Metadata["reason_code"] != "generated_missing_column" {
			t.Fatalf("statement 0 findings = %+v, want generated_missing_column", findings)
		}
		// The follow-up index sees an unresolved table state — a bounded
		// unknown_table_state gap, never a proven-present or proven-absent
		// answer derived from the invalid declaration.
		gaps := result.Statements[1].EvidenceGaps
		if len(result.Statements[1].Findings) != 0 {
			t.Fatalf("statement 1 findings = %+v, want none", result.Statements[1].Findings)
		}
		if len(gaps) == 0 {
			t.Fatalf("statement 1 gaps = none, want unknown_table_state")
		}
		for _, gap := range gaps {
			if gap.ReasonCode != "unknown_table_state" {
				t.Fatalf("statement 1 gap reason = %q, want unknown_table_state", gap.ReasonCode)
			}
		}
	})
	t.Run("compat_disabled", func(t *testing.T) {
		// With the compat rule switched off the premise issue produces no
		// finding, but the invalid transition still blocks publication.
		result, err := AuditSQL(context.Background(), Request{
			SQL:           sql,
			Dialect:       spec.DialectMySQL,
			Schema:        "golden",
			TargetVersion: "8.0.46",
			ConfigPath: t07PolicyPath(t, map[string]string{
				"ddl.create_index.columns.exists.require": "      required: true\n",
			}),
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].Findings) != 0 {
			t.Fatalf("statement 0 findings = %+v, want none", result.Statements[0].Findings)
		}
		gaps := result.Statements[1].EvidenceGaps
		if len(gaps) == 0 {
			t.Fatalf("statement 1 gaps = none, want unknown_table_state")
		}
		for _, gap := range gaps {
			if gap.ReasonCode != "unknown_table_state" {
				t.Fatalf("statement 1 gap reason = %q, want unknown_table_state", gap.ReasonCode)
			}
		}
	})
}

// TestT07R1GeneratedPremiseAlter covers the same root cause on known
// single-action ALTER sources and the bounded gap cells around snapshots,
// positions, and same-statement binds.
func TestT07R1GeneratedPremiseAlter(t *testing.T) {
	t.Parallel()
	for _, variant := range []struct {
		dialect spec.Dialect
		version string
	}{
		{spec.DialectMySQL, "8.0.46"},
		{spec.DialectTiDB, "8.5.0"},
	} {
		variant := variant
		t.Run(string(variant.dialect), func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name            string
				columns         []spec.Column
				sql             string
				wantReason      string
				wantGap         string
				wantUnsupported bool
			}{
				{
					name:    "add_generated_references_prior",
					columns: []spec.Column{{Name: "a", Type: "int"}},
					sql:     "ALTER TABLE t ADD COLUMN b INT AS (a + 1) VIRTUAL",
				},
				{
					name:    "add_generated_missing",
					columns: []spec.Column{{Name: "a", Type: "int"}},
					sql:     "ALTER TABLE t ADD COLUMN b INT AS (missing_c + 1) VIRTUAL",
					// The reference is absent from the provable candidate
					// set — a determined missing-column rejection.
					wantReason: "generated_missing_column",
				},
				{
					name: "modify_to_generated_self",
					columns: []spec.Column{
						{Name: "a", Type: "int"},
						{Name: "b", Type: "int"},
					},
					sql:        "ALTER TABLE t MODIFY COLUMN b INT AS (b + 1) VIRTUAL",
					wantReason: "generated_self_reference",
				},
				{
					name: "add_generated_after_later_prior_generated",
					columns: []spec.Column{
						{Name: "a", Type: "int"},
						{Name: "g", Type: "int", Generated: &spec.GeneratedColumn{
							Storage: spec.GeneratedStorageStored,
							Expression: spec.ExpressionFacts{
								Known:      true,
								References: []spec.ColumnReference{{Name: "a"}},
							},
						}},
					},
					// FIRST|AFTER positioning is outside audited semantics
					// — the coverage layer returns the statement to the
					// unsupported channel before the premise check can
					// produce a guessed verdict.
					sql:             "ALTER TABLE t ADD COLUMN b INT AS (g + 1) VIRTUAL FIRST",
					wantUnsupported: true,
				},
				{
					name: "add_generated_references_prior_generated",
					columns: []spec.Column{
						{Name: "a", Type: "int"},
						{Name: "g", Type: "int", Generated: &spec.GeneratedColumn{
							Storage: spec.GeneratedStorageStored,
							Expression: spec.ExpressionFacts{
								Known:      true,
								References: []spec.ColumnReference{{Name: "a"}},
							},
						}},
					},
					// g precedes the appended column — legal on both
					// products (an earlier generated column may be
					// referenced).
					sql: "ALTER TABLE t ADD COLUMN b INT AS (g + 1) VIRTUAL",
				},
				{
					name:    "add_generated_missing_snapshot",
					columns: nil, // provider answers unknown: absent map entry
					sql:     "ALTER TABLE t ADD COLUMN b INT AS (a + 1) VIRTUAL",
					wantGap: "",
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					snapshots := map[string]*spec.TableSnapshot{}
					if tc.columns != nil {
						snapshots["golden.t"] = &spec.TableSnapshot{
							Exists:      true,
							Schema:      "golden",
							Table:       &spec.Table{Schema: "golden", Name: "t"},
							Columns:     tc.columns,
							Indexes:     []spec.Index{},
							Constraints: []spec.Constraint{},
						}
					}
					p := &t05PresentProvider{snapshots: snapshots}
					p.t07InstanceVersion = variant.version
					if variant.dialect == spec.DialectTiDB {
						p.t07InstanceVersion = "8.0.11-TiDB-v8.5.0"
					}
					result, err := AuditSQL(context.Background(), Request{
						SQL:              tc.sql,
						Dialect:          variant.dialect,
						Schema:           "golden",
						ConfigPath:       r1CompatOnly(t),
						MetadataProvider: p,
					})
					if tc.wantUnsupported {
						if err == nil {
							t.Fatalf("audit succeeded for %q, want the unsupported channel", tc.sql)
						}
						return
					}
					if err != nil {
						t.Fatalf("audit: %v", err)
					}
					findings := result.Statements[0].Findings
					gaps := result.Statements[0].EvidenceGaps
					switch {
					case tc.wantReason != "":
						if len(findings) != 1 || findings[0].Metadata["reason_code"] != tc.wantReason {
							t.Fatalf("findings = %+v, want one %s finding", findings, tc.wantReason)
						}
					case tc.wantGap != "":
						if len(findings) != 0 {
							t.Fatalf("findings = %+v, want none", findings)
						}
						found := false
						for _, gap := range gaps {
							if gap.ReasonCode == tc.wantGap {
								found = true
							}
						}
						if !found {
							t.Fatalf("gaps = %+v, want one %s gap", gaps, tc.wantGap)
						}
					case tc.name == "add_generated_missing_snapshot":
						if len(findings) != 0 {
							t.Fatalf("findings = %+v, want none", findings)
						}
						if len(gaps) == 0 {
							t.Fatalf("gaps = none, want a bounded gap")
						}
					default:
						if len(findings) != 0 {
							t.Fatalf("findings = %+v, want none", findings)
						}
					}
				})
			}
		})
	}
}

// TestT07R1TiDBAlterExpressionShapes runs the frozen R1-2 inputs through the
// real parser and AuditSQL on TiDB 8.5.0: the observed positive family, the
// determined rejections, and the bounded gaps — never a function-name-only
// verdict on a wrapped expression.
func TestT07R1TiDBAlterExpressionShapes(t *testing.T) {
	t.Parallel()
	varcharX := []spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "int"}}
	for _, tc := range []struct {
		name       string
		sql        string
		columns    []spec.Column
		wantReason string
		wantGap    string
	}{
		{"d_uuid", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid())", varcharX, "", ""},
		{"d_arith", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid() + 1)", varcharX, "tidb_default_expression", ""},
		{"d_case", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END)", varcharX, "", ""},
		{"d_parens", "ALTER TABLE t ALTER COLUMN v SET DEFAULT ((uuid()))", varcharX, "", ""},
		{"d_constant", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (1 + 2)", varcharX, "", ""},
		{"column_ref", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (x + 1)", varcharX, "tidb_default_expression", ""},
		{"disallowed_function", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (lower('A'))", varcharX, "tidb_default_expression", ""},
		{"foldable_ts_on_int", "ALTER TABLE t ALTER COLUMN x SET DEFAULT (current_timestamp)", varcharX, "tidb_default_expression", ""},
		// A single bare allowed call stores verbatim on every non-1101
		// target — uuid() is observed verbatim on date too.
		{"uuid_on_date", "ALTER TABLE t ALTER COLUMN x SET DEFAULT (uuid())",
			[]spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "date"}}, "", ""},
		{"json_object_on_int", "ALTER TABLE t ALTER COLUMN x SET DEFAULT (json_object('k',1))", varcharX, "", ""},
		// R1 root-wrap cells: `((f(...)))` takes the evaluation path — the
		// disallowed-callee gate does not apply (observed ((lower('x')))
		// stores 'x'), observed folds keep their target coercion.
		{"dp_uuid_int", "ALTER TABLE t ALTER COLUMN x SET DEFAULT ((uuid()))", varcharX, "tidb_default_expression", ""},
		{"dp_uuid_enum", "ALTER TABLE t ALTER COLUMN x SET DEFAULT ((uuid()))",
			[]spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "enum('a','b')"}}, "tidb_default_expression", ""},
		{"dp_uuid_time", "ALTER TABLE t ALTER COLUMN x SET DEFAULT ((uuid()))",
			[]spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "time"}}, "tidb_default_expression", ""},
		{"dp_uuid_varbinary", "ALTER TABLE t ALTER COLUMN x SET DEFAULT ((uuid()))",
			[]spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "varbinary(36)"}}, "", ""},
		{"dp_uuid_date_null", "ALTER TABLE t ALTER COLUMN x SET DEFAULT ((uuid()))",
			[]spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "date"}}, "", "incomplete_source_column"},
		{"dp_json_vc", "ALTER TABLE t ALTER COLUMN v SET DEFAULT ((json_object('k',1)))", varcharX, "", "incomplete_source_column"},
		{"dp_lower_vc", "ALTER TABLE t ALTER COLUMN v SET DEFAULT ((lower('x')))", varcharX, "", ""},
		{"dp_lower_int", "ALTER TABLE t ALTER COLUMN x SET DEFAULT ((lower('x')))", varcharX, "tidb_default_expression", ""},
		{"dp_cts_ts", "ALTER TABLE t ALTER COLUMN v SET DEFAULT ((current_timestamp))",
			[]spec.Column{{Name: "v", Type: "timestamp"}}, "", "incomplete_source_column"},
		{"c_arith_int", "ALTER TABLE t ALTER COLUMN x SET DEFAULT (1 + 2)", varcharX, "", ""},
		{"c_arith_char", "ALTER TABLE t ALTER COLUMN x SET DEFAULT (1 + 2)",
			[]spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "char(8)"}}, "", ""},
		{"c_arith_set", "ALTER TABLE t ALTER COLUMN x SET DEFAULT (1 + 2)",
			[]spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "set('1','3','5')"}}, "", ""},
		{"c_arith_date", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (1 + 2)",
			[]spec.Column{{Name: "v", Type: "date"}}, "", "incomplete_source_column"},
		{"case_uuid_int", "ALTER TABLE t ALTER COLUMN x SET DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END)", varcharX, "tidb_default_expression", ""},
		{"case_uuid_char", "ALTER TABLE t ALTER COLUMN x SET DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END)",
			[]spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "char(40)"}}, "", ""},
		// Literal-class (current_timestamp) cells: verbatim temporal on
		// temporal/DATE targets, folded literal on the observed positive
		// bases, determined 1067 on int/decimal/year.
		{"fts_vc", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (current_timestamp)", varcharX, "", ""},
		{"fts_ts", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (current_timestamp)",
			[]spec.Column{{Name: "v", Type: "timestamp"}}, "", ""},
		{"fts_date", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (current_timestamp)",
			[]spec.Column{{Name: "v", Type: "date"}}, "", ""},
		{"fts_bigint", "ALTER TABLE t ALTER COLUMN x SET DEFAULT (current_timestamp)",
			[]spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "bigint"}}, "", ""},
		{"fts_time", "ALTER TABLE t ALTER COLUMN x SET DEFAULT (current_timestamp)",
			[]spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "time"}}, "", ""},
		{"fts_decimal", "ALTER TABLE t ALTER COLUMN x SET DEFAULT (current_timestamp)",
			[]spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "decimal(20,2)"}}, "tidb_default_expression", ""},
		{"fts_tinyint_gap", "ALTER TABLE t ALTER COLUMN x SET DEFAULT (current_timestamp)",
			[]spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "tinyint"}}, "", "incomplete_source_column"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:              tc.sql,
				Dialect:          spec.DialectTiDB,
				Schema:           "golden",
				ConfigPath:       r1CompatOnly(t),
				MetadataProvider: r1TiDBProvider(tc.columns...),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			findings := result.Statements[0].Findings
			gaps := result.Statements[0].EvidenceGaps
			switch {
			case tc.wantReason != "":
				if len(findings) != 1 || findings[0].RuleID != t07RuleCompat ||
					findings[0].Metadata["reason_code"] != tc.wantReason {
					t.Fatalf("findings = %+v, want one %s finding", findings, tc.wantReason)
				}
			case tc.wantGap != "":
				if len(findings) != 0 {
					t.Fatalf("findings = %+v, want none", findings)
				}
				found := false
				for _, gap := range gaps {
					if gap.ReasonCode == tc.wantGap {
						found = true
					}
				}
				if !found {
					t.Fatalf("gaps = %+v, want one %s gap", gaps, tc.wantGap)
				}
			default:
				if len(findings) != 0 || len(gaps) != 0 {
					t.Fatalf("findings/gaps = %+v/%+v, want none", findings, gaps)
				}
			}
		})
	}
}

// TestT07R1ExpressionShapeOppositeControls proves the TiDB setter
// restrictions never leak into MySQL: every frozen D_* form stays an
// expression default on MySQL 8.0.46.
func TestT07R1ExpressionShapeOppositeControls(t *testing.T) {
	t.Parallel()
	columns := []spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "int"}}
	provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
		"golden.t": {
			Exists:      true,
			Schema:      "golden",
			Table:       &spec.Table{Schema: "golden", Name: "t"},
			Columns:     columns,
			Indexes:     []spec.Index{},
			Constraints: []spec.Constraint{},
		},
	}}
	provider.t07InstanceVersion = "8.0.46"
	for _, tc := range []struct {
		name string
		sql  string
	}{
		{"d_uuid", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid())"},
		{"d_arith", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid() + 1)"},
		{"d_case", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END)"},
		{"d_parens", "ALTER TABLE t ALTER COLUMN v SET DEFAULT ((uuid()))"},
		{"d_constant", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (1 + 2)"},
		{"column_ref", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (x + 1)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:              tc.sql,
				Dialect:          spec.DialectMySQL,
				Schema:           "golden",
				TargetVersion:    "8.0.46",
				ConfigPath:       r1CompatOnly(t),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
				t.Fatalf("findings/gaps = %+v/%+v, want none", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
			}
		})
	}
}

// TestT07R1TiDBFoldedLiteralPostState pins the publication seam: foldable
// composites publish literal default presence — never expression facts or a
// computed value — while a verbatim bare call keeps its expression facts.
func TestT07R1TiDBFoldedLiteralPostState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	version := &spec.VersionIdentity{Product: spec.VersionProductTiDB, Version: "8.5.0", Major: 8, Minor: 5, Patch: 0, ValidatedRange: true}
	for _, tc := range []struct {
		name           string
		sql            string
		priorType      string
		wantExpr       bool
		wantCts        bool
		wantHasDefault bool
	}{
		{"folded_arith", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (1 + 2)", "varchar(36)", false, false, true},
		{"folded_arith_int", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (1 + 2)", "int", false, false, true},
		{"folded_arith_time", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (1 + 2)", "time", false, false, true},
		{"folded_case", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END)", "varchar(36)", false, false, true},
		{"folded_wrapped_call", "ALTER TABLE t ALTER COLUMN v SET DEFAULT ((uuid()))", "varchar(36)", false, false, true},
		{"folded_wrapped_call_char", "ALTER TABLE t ALTER COLUMN v SET DEFAULT ((uuid()))", "char(36)", false, false, true},
		{"folded_wrapped_lower", "ALTER TABLE t ALTER COLUMN v SET DEFAULT ((lower('x')))", "varchar(36)", false, false, true},
		{"folded_cts_varchar", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (current_timestamp)", "varchar(36)", false, false, true},
		{"folded_cts_bigint", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (current_timestamp)", "bigint", false, false, true},
		{"folded_cts_time", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (current_timestamp)", "time", false, false, true},
		{"verbatim_cts_ts", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (current_timestamp)", "timestamp", false, true, true},
		{"verbatim_uuid", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid())", "varchar(36)", true, false, true},
		{"verbatim_uuid_date", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid())", "date", true, false, true},
		{"verbatim_json_int", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (json_object('k',1))", "int", true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := Parse(ctx, tc.sql, spec.DialectTiDB)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			statements, err := Extract(ctx, parsed)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			statements[0].Metadata = &spec.Metadata{Version: version}
			snapshot := &spec.TableSnapshot{
				Exists:  true,
				Schema:  "golden",
				Table:   &spec.Table{Schema: "golden", Name: "t"},
				Columns: []spec.Column{{Name: "v", Type: tc.priorType}, {Name: "x", Type: "int"}},
			}
			statements[0].Metadata.TargetTable = snapshot
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": snapshot}}
			state := newBatchState(spec.DialectTiDB, "golden", provider)
			if _, err := state.preState(ctx, "golden", spec.Table{Schema: "golden", Name: "t"}); err != nil {
				t.Fatalf("pre-state resolve: %v", err)
			}
			if err := state.apply(ctx, statements[0]); err != nil {
				t.Fatalf("apply: %v", err)
			}
			post, err := state.preState(ctx, "golden", spec.Table{Schema: "golden", Name: "t"})
			if err != nil {
				t.Fatalf("preState: %v", err)
			}
			if post == nil || !post.Exists {
				t.Fatalf("post-state = %+v, want present", post)
			}
			var got *spec.Column
			for i := range post.Columns {
				if post.Columns[i].Name == "v" {
					got = &post.Columns[i]
				}
			}
			if got == nil {
				t.Fatalf("column v missing from post-state: %+v", post.Columns)
			}
			if got.HasDefault != tc.wantHasDefault {
				t.Fatalf("HasDefault = %v, want %v", got.HasDefault, tc.wantHasDefault)
			}
			if tc.wantExpr {
				if got.DefaultExpression == nil || len(got.DefaultExpression.Functions) != 1 {
					t.Fatalf("DefaultExpression = %+v, want single-callee call facts", got.DefaultExpression)
				}
			} else {
				if got.DefaultExpression != nil {
					t.Fatalf("DefaultExpression = %+v, want nil for a folded literal", got.DefaultExpression)
				}
				if !tc.wantCts && got.DefaultValue != "" {
					t.Fatalf("DefaultValue = %q, want empty — no fabricated value", got.DefaultValue)
				}
			}
			if got.DefaultIsCurrentTimestamp != tc.wantCts {
				t.Fatalf("DefaultIsCurrentTimestamp = %v, want %v", got.DefaultIsCurrentTimestamp, tc.wantCts)
			}
			// A verbatim expression default keeps the authored parens fact
			// (catalog expression defaults are always parenthesized); the
			// folded shapes publish the catalog-normalized false.
			if got.DefaultParenthesized != tc.wantExpr {
				t.Fatalf("DefaultParenthesized = %v, want %v", got.DefaultParenthesized, tc.wantExpr)
			}
		})
	}
}

// TestT07R1DefaultParenthesizedExtraction pins the authored-parens syntax
// fact: CREATE and ALTER SET sites mark DefaultParenthesized independently
// of the expression facts, and bare forms stay unmarked.
func TestT07R1DefaultParenthesizedExtraction(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		sql        string
		action     string
		wantParens bool
	}{
		{"create_parens_call", "CREATE TABLE t (v VARCHAR(36) DEFAULT (uuid()))", "create_table", true},
		{"create_bare_call", "CREATE TABLE t (v VARCHAR(36) DEFAULT uuid())", "create_table", false},
		{"create_parens_literal", "CREATE TABLE t (v VARCHAR(36) DEFAULT ('abc'))", "create_table", true},
		{"set_parens_call", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid())", "set_column_default", true},
		{"set_bare_literal", "ALTER TABLE t ALTER COLUMN v SET DEFAULT 2", "set_column_default", false},
		{"add_parens_arith", "ALTER TABLE t ADD COLUMN c INT DEFAULT (1 + 2)", "add_columns", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
				statements := t07Columns(t, tc.sql, dialect)
				if len(statements) != 1 {
					t.Fatalf("statements = %d, want 1", len(statements))
				}
				var col *spec.Column
				switch tc.action {
				case "create_table":
					col = &statements[0].DDL.Columns[0]
				default:
					col = statements[0].DDL.Alter[0].Column.Definition
				}
				if col == nil {
					t.Fatalf("%s: no column for %q", dialect, tc.sql)
				}
				if col.DefaultParenthesized != tc.wantParens {
					t.Fatalf("%s %s DefaultParenthesized = %v, want %v", dialect, tc.name, col.DefaultParenthesized, tc.wantParens)
				}
			}
		})
	}
}

// TestT07R1TiDBCreateShapeGate pins the create-family parens grammar
// boundary on TiDB: only a bare call survives inside DEFAULT (...);
// MySQL keeps the composite expression support.
func TestT07R1TiDBCreateShapeGate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		sql        string
		dialect    spec.Dialect
		version    string
		wantReason string
	}{
		{"tidb_parens_call", "CREATE TABLE t (v VARCHAR(36) DEFAULT (uuid()))", spec.DialectTiDB, "8.5.0", ""},
		{"tidb_bare_call", "CREATE TABLE t (v VARCHAR(36) DEFAULT uuid())", spec.DialectTiDB, "8.5.0", ""},
		{"tidb_parens_arith", "CREATE TABLE t (v INT DEFAULT (1 + 2))", spec.DialectTiDB, "8.5.0", "tidb_default_expression"},
		{"tidb_parens_case", "CREATE TABLE t (v VARCHAR(36) DEFAULT (CASE WHEN 1 THEN 'a' ELSE 'b' END))", spec.DialectTiDB, "8.5.0", "tidb_default_expression"},
		{"tidb_parens_literal", "CREATE TABLE t (v VARCHAR(36) DEFAULT ('abc'))", spec.DialectTiDB, "8.5.0", "tidb_default_expression"},
		{"tidb_bare_call_disallowed", "CREATE TABLE t (v VARCHAR(36) DEFAULT lower('a'))", spec.DialectTiDB, "8.5.0", "tidb_default_expression"},
		{"mysql_parens_arith", "CREATE TABLE t (v INT DEFAULT (1 + 2))", spec.DialectMySQL, "8.0.46", ""},
		{"mysql_parens_literal", "CREATE TABLE t (v VARCHAR(36) DEFAULT ('abc'))", spec.DialectMySQL, "8.0.46", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:           tc.sql,
				Dialect:       tc.dialect,
				Schema:        "golden",
				TargetVersion: tc.version,
				ConfigPath:    r1CompatOnly(t),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			findings := result.Statements[0].Findings
			if tc.wantReason == "" {
				if len(findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
					t.Fatalf("findings/gaps = %+v/%+v, want none", findings, result.Statements[0].EvidenceGaps)
				}
				return
			}
			if len(findings) != 1 || findings[0].Metadata["reason_code"] != tc.wantReason {
				t.Fatalf("findings = %+v, want one %s finding", findings, tc.wantReason)
			}
		})
	}
}

// TestT07R1MixedDefaultRegression pins the supplement-binding fix: bare
// literal defaults no longer poison the paren-default site binding in the
// same statement.
func TestT07R1MixedDefaultRegression(t *testing.T) {
	t.Parallel()
	statements := t07Columns(t, "CREATE TABLE t (c INT DEFAULT (1 + 2), d INT DEFAULT 2)", spec.DialectTiDB)
	if len(statements) != 1 || len(statements[0].DDL.Columns) != 2 {
		t.Fatalf("statements/columns = %+v", statements)
	}
	if statements[0].DDL.Columns[0].DefaultExpression == nil {
		t.Fatalf("column c lost its expression default")
	}
	if !statements[0].DDL.Columns[1].HasDefault || statements[0].DDL.Columns[1].DefaultValue != "2" {
		t.Fatalf("column d default = %+v, want literal 2", statements[0].DDL.Columns[1])
	}
}

// TestT07R1ToolBoundaryChannels pins the real error channels the R1 cells
// must keep: a bare SET DEFAULT call is a grammar rejection (never a
// fabricated compat finding), and unmodeled surface — unknown functions,
// FIRST|AFTER positioning — stays inside the unsupported channel.
func TestT07R1ToolBoundaryChannels(t *testing.T) {
	t.Parallel()
	varcharX := []spec.Column{{Name: "v", Type: "varchar(36)"}, {Name: "x", Type: "int"}}
	t.Run("bare_set_default_call_is_a_parse_rejection", func(t *testing.T) {
		// `SET DEFAULT uuid()` without parens is a grammar rejection on
		// every product (observed 1064): the parser error channel owns it.
		_, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t ALTER COLUMN v SET DEFAULT uuid()",
			Dialect:          spec.DialectTiDB,
			Schema:           "golden",
			ConfigPath:       r1CompatOnly(t),
			MetadataProvider: r1TiDBProvider(varcharX...),
		})
		if err == nil {
			t.Fatalf("audit succeeded, want the parser rejection channel")
		}
	})
	t.Run("unknown_function_is_unsupported", func(t *testing.T) {
		// md5 is outside the parser's function registry — the coverage
		// layer's unknown_function channel owns it, never an invented
		// function-family verdict.
		_, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t ALTER COLUMN v SET DEFAULT (md5('x'))",
			Dialect:          spec.DialectTiDB,
			Schema:           "golden",
			ConfigPath:       r1CompatOnly(t),
			MetadataProvider: r1TiDBProvider(varcharX...),
		})
		if err == nil {
			t.Fatalf("audit succeeded, want the unsupported channel")
		}
	})
}

// TestT07R1NoExpressionLeak pins the privacy boundary for the new issue
// families: findings and gaps carry only safe identifiers and bounded
// reason codes — never expression text, literals, fingerprints, or
// function arguments.
func TestT07R1NoExpressionLeak(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		sql  string
	}{
		{"missing_column", "CREATE TABLE t (a INT, b INT AS (secret_col + 1) STORED)"},
		{"self_reference", "CREATE TABLE t (a INT, b INT AS (b + 1) STORED)"},
		{"tidb_arith_default", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid() + 1)"},
		{"tidb_literal_name", "CREATE TABLE t (a INT, b VARCHAR(32) AS ('secret_col') VIRTUAL)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := Request{
				SQL:           tc.sql,
				Dialect:       spec.DialectMySQL,
				Schema:        "golden",
				TargetVersion: "8.0.46",
				ConfigPath:    r1CompatOnly(t),
			}
			if tc.name == "tidb_arith_default" {
				req.Dialect = spec.DialectTiDB
				req.TargetVersion = ""
				req.MetadataProvider = r1TiDBProvider(spec.Column{Name: "v", Type: "varchar(36)"})
			}
			result, err := AuditSQL(context.Background(), req)
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			leaks := []string{"secret_col", "uuid()", "fingerprint", "0x", "c1f300"}
			check := func(label, s string) {
				for _, leak := range leaks {
					if strings.Contains(s, leak) {
						t.Fatalf("%s leaks %q: %q", label, leak, s)
					}
				}
			}
			for _, st := range result.Statements {
				for _, f := range st.Findings {
					check("finding.message", f.Message)
					for key, v := range f.Metadata {
						check("finding.metadata."+key, fmt.Sprintf("%v", v))
					}
				}
				for _, g := range st.EvidenceGaps {
					check("gap.reason", g.ReasonCode)
					for _, fact := range g.RequiredFacts {
						check("gap.fact", fact)
					}
				}
			}
		})
	}
}

// TestT07R1RequestIsolation proves concurrent audits over the new fact
// fields stay request-local: parallel runs produce identical verdicts and
// never alias each other's expression facts.
func TestT07R1RequestIsolation(t *testing.T) {
	t.Parallel()
	configPath := r1CompatOnly(t)
	sql := "ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid() + 1)"
	const runs = 8
	results := make(chan error, runs)
	for i := 0; i < runs; i++ {
		go func() {
			result, err := AuditSQL(context.Background(), Request{
				SQL:              sql,
				Dialect:          spec.DialectTiDB,
				Schema:           "golden",
				ConfigPath:       configPath,
				MetadataProvider: r1TiDBProvider(spec.Column{Name: "v", Type: "varchar(36)"}),
			})
			if err != nil {
				results <- err
				return
			}
			findings := result.Statements[0].Findings
			if len(findings) != 1 || findings[0].Metadata["reason_code"] != "tidb_default_expression" {
				results <- fmt.Errorf("findings = %+v, want one tidb_default_expression", findings)
				return
			}
			results <- nil
		}()
	}
	for i := 0; i < runs; i++ {
		if err := <-results; err != nil {
			t.Fatalf("concurrent run %d: %v", i, err)
		}
	}
}

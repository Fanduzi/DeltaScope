// Package audit verifies the T06-A7 create-table collation upgrade: the
// declared table COLLATE clause moves from an always-unsupported aspect to a
// governed policy fact, the column charset/collation rules stay isolated per
// rule, and the CREATE-derived state carries the declared option through.
// input: CREATE TABLE batches with declared column charset/collation and table COLLATE through AuditSQL plus the enrichment seam
// output: exact per-rule findings, complete/pass under all-off and allowed policies, preserved Options facts in derived state
// pos: application-layer contract tests for the T06-A7 collation-policy oracle (issue #85)
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t06A7ColumnCharsetRule   = "ddl.column.charset.allowlist"
	t06A7ColumnCollationRule = "ddl.column.collation.allowlist"
	t06A7ColumnMatchRule     = "ddl.column.charset_collation.match.require"
	t06A7TableCollationRule  = "ddl.table.collation.allowlist"
)

// t06A7TableCollateSQL is the fixed S4 baseline input: one ordinary column and
// a declared table-level COLLATE clause.
const t06A7TableCollateSQL = "CREATE TABLE t (c INT) COLLATE=utf8mb4_bin;"

// t06A7Audit runs AuditSQL with an isolated policy and fails the test on
// unexpected errors (findings stay data, they are not errors).
func t06A7Audit(t *testing.T, dialect spec.Dialect, sql, policy string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL: sql, Dialect: dialect, Schema: "golden", ConfigPath: policy,
	})
	if err != nil {
		t.Fatalf("%s: unexpected error %v", dialect, err)
	}
	return result
}

// t06A7CollationPolicy enables exactly the named rules at blocker with the
// given params blocks; everything else stays disabled.
func t06A7CollationPolicy(t *testing.T, enabled map[string]string) string {
	t.Helper()
	return t05PolicyPath(t, enabled)
}

// t06A7WantFinding pins exactly one blocker finding on statement 0 with the
// expected rule ID, message, and metadata.
func t06A7WantFinding(t *testing.T, result report.Result, ruleID, message string, metadata map[string]any) {
	t.Helper()
	if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements = %d, want 1", len(result.Statements))
	}
	statement := result.Statements[0]
	if statement.Index != 0 || statement.Kind != "ddl" {
		t.Fatalf("statement identity = index %d kind %s, want 0/ddl", statement.Index, statement.Kind)
	}
	if len(statement.Findings) != 1 {
		t.Fatalf("findings = %#v, want exactly 1", statement.Findings)
	}
	finding := statement.Findings[0]
	if finding.RuleID != ruleID || finding.Level != "blocker" {
		t.Fatalf("finding = %s/%s, want %s blocker", finding.RuleID, finding.Level, ruleID)
	}
	if finding.Message != message {
		t.Fatalf("message = %q, want %q", finding.Message, message)
	}
	if !reflect.DeepEqual(finding.Metadata, metadata) {
		t.Fatalf("metadata = %#v, want %#v", finding.Metadata, metadata)
	}
	if finding.Location == nil || finding.Location.Line != 1 || finding.Location.Column != 1 {
		t.Fatalf("location = %+v, want 1:1", finding.Location)
	}
}

// t06A7WantPass pins a complete, clean pass on a single statement.
func t06A7WantPass(t *testing.T, result report.Result) {
	t.Helper()
	if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("unsupported = %#v, want none", result.Unsupported)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements = %d, want 1", len(result.Statements))
	}
	statement := result.Statements[0]
	if statement.Coverage.Status != report.CoverageComplete {
		t.Fatalf("statement coverage = %s, want complete", statement.Coverage.Status)
	}
	if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
		t.Fatalf("statement = findings %#v gaps %#v, want none", statement.Findings, statement.EvidenceGaps)
	}
}

// TestT06A7ColumnAllowlistIsolation is group 1a: each column allowlist fires
// on its own declared field only — allowed, denied, and off shapes on both
// dialects, with per-rule profiles so attribution cannot hide behind totals.
func TestT06A7ColumnAllowlistIsolation(t *testing.T) {
	csPolicy := t06A7CollationPolicy(t, map[string]string{
		t06A7ColumnCharsetRule: "      values: [utf8mb4]\n",
	})
	ccPolicy := t06A7CollationPolicy(t, map[string]string{
		t06A7ColumnCollationRule: "      values: [utf8mb4_bin]\n",
	})
	offPolicy := t05PolicyPath(t, map[string]string{})
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/charset_allowed", func(t *testing.T) {
			t06A7WantPass(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin);", csPolicy))
		})
		t.Run(string(dialect)+"/charset_denied", func(t *testing.T) {
			t06A7WantFinding(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) CHARACTER SET latin1 COLLATE latin1_bin);", csPolicy),
				t06A7ColumnCharsetRule,
				`column "c" uses unsupported charset "latin1"`,
				map[string]any{"table": "t", "column": "c", "field": "charset", "value": "latin1", "allowed": []string{"utf8mb4"}})
		})
		t.Run(string(dialect)+"/charset_off", func(t *testing.T) {
			t06A7WantPass(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) CHARACTER SET latin1 COLLATE latin1_bin);", offPolicy))
		})
		t.Run(string(dialect)+"/collation_allowed", func(t *testing.T) {
			t06A7WantPass(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin);", ccPolicy))
		})
		t.Run(string(dialect)+"/collation_denied", func(t *testing.T) {
			t06A7WantFinding(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci);", ccPolicy),
				t06A7ColumnCollationRule,
				`column "c" uses unsupported collation "utf8mb4_general_ci"`,
				map[string]any{"table": "t", "column": "c", "field": "collation", "value": "utf8mb4_general_ci", "allowed": []string{"utf8mb4_bin"}})
		})
		t.Run(string(dialect)+"/collation_off", func(t *testing.T) {
			t06A7WantPass(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci);", offPolicy))
		})
	}
}

// TestT06A7ColumnMatchShapes is group 1b: the match rule's four declaration
// shapes — paired, single-side (either direction), both-empty, mismatched —
// plus required=false and off, with exact message/metadata on the column.
func TestT06A7ColumnMatchShapes(t *testing.T) {
	matchPolicy := t06A7CollationPolicy(t, map[string]string{
		t06A7ColumnMatchRule: "      required: true\n",
	})
	matchOptional := t06A7CollationPolicy(t, map[string]string{
		t06A7ColumnMatchRule: "      required: false\n",
	})
	offPolicy := t05PolicyPath(t, map[string]string{})
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/pair", func(t *testing.T) {
			t06A7WantPass(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin);", matchPolicy))
		})
		t.Run(string(dialect)+"/collate_only", func(t *testing.T) {
			t06A7WantFinding(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) COLLATE utf8mb4_bin);", matchPolicy),
				t06A7ColumnMatchRule,
				`column "c" must specify charset and collation together`,
				map[string]any{"table": "t", "column": "c", "charset": "", "collation": "utf8mb4_bin"})
		})
		t.Run(string(dialect)+"/charset_only", func(t *testing.T) {
			t06A7WantFinding(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4);", matchPolicy),
				t06A7ColumnMatchRule,
				`column "c" must specify charset and collation together`,
				map[string]any{"table": "t", "column": "c", "charset": "utf8mb4", "collation": ""})
		})
		t.Run(string(dialect)+"/both_empty", func(t *testing.T) {
			t06A7WantPass(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16));", matchPolicy))
		})
		t.Run(string(dialect)+"/mismatch", func(t *testing.T) {
			t06A7WantFinding(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE latin1_swedish_ci);", matchPolicy),
				t06A7ColumnMatchRule,
				`column "c" collation "latin1_swedish_ci" must match charset "utf8mb4"`,
				map[string]any{"table": "t", "column": "c", "charset": "utf8mb4", "collation": "latin1_swedish_ci"})
		})
		t.Run(string(dialect)+"/required_false_silent", func(t *testing.T) {
			t06A7WantPass(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) COLLATE utf8mb4_bin);", matchOptional))
		})
		t.Run(string(dialect)+"/off", func(t *testing.T) {
			t06A7WantPass(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE latin1_swedish_ci);", offPolicy))
		})
	}
}

// TestT06A7ColumnAttribution is group 1c: findings bind to the violating
// column and statement — a second clean column/statement cannot absorb or
// mask the single expected finding.
func TestT06A7ColumnAttribution(t *testing.T) {
	matchPolicy := t06A7CollationPolicy(t, map[string]string{
		t06A7ColumnMatchRule: "      required: true\n",
	})
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/mixed_columns", func(t *testing.T) {
			result := t06A7Audit(t, dialect,
				"CREATE TABLE t (a VARCHAR(16) COLLATE utf8mb4_bin, b VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin, d INT);",
				matchPolicy)
			statement := result.Statements[0]
			if len(statement.Findings) != 1 || statement.Findings[0].Metadata["column"] != "a" {
				t.Fatalf("mixed findings = %#v, want one finding on a", statement.Findings)
			}
		})
		t.Run(string(dialect)+"/statement_owner", func(t *testing.T) {
			result := t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) COLLATE utf8mb4_bin);\nCREATE TABLE u (c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin);",
				matchPolicy)
			if len(result.Statements) != 2 {
				t.Fatalf("statements = %d, want 2", len(result.Statements))
			}
			if len(result.Statements[0].Findings) != 1 || result.Statements[0].Findings[0].Metadata["table"] != "t" {
				t.Fatalf("statement 0 findings = %#v, want one t-bound finding", result.Statements[0].Findings)
			}
			if len(result.Statements[1].Findings) != 0 || len(result.Statements[1].EvidenceGaps) != 0 {
				t.Fatalf("statement 1 = findings %#v gaps %#v, want clean", result.Statements[1].Findings, result.Statements[1].EvidenceGaps)
			}
		})
	}
}

// TestT06A7TableCollateAllOffComplete is the frozen S4 red path: with every
// rule disabled the declared table COLLATE is governed input, not an
// unsupported aspect — the statement must report complete/pass with no
// findings, gaps, or unsupported entries.
func TestT06A7TableCollateAllOffComplete(t *testing.T) {
	policy := t05PolicyPath(t, map[string]string{})
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL: t06A7TableCollateSQL, Dialect: dialect, ConfigPath: policy,
			})
			if err != nil {
				t.Fatalf("%s: unexpected error %v", dialect, err)
			}
			t06A7WantPass(t, result)
		})
	}
}

// TestT06A7TableCollationRule is group 2 at the shared seam: allowed, denied,
// case-insensitive, missing-with-optional, missing-with-required, and off.
func TestT06A7TableCollationRule(t *testing.T) {
	optionalPolicy := t06A7CollationPolicy(t, map[string]string{
		t06A7TableCollationRule: "      values: [utf8mb4_bin]\n      require_explicit: false\n",
	})
	requiredPolicy := t06A7CollationPolicy(t, map[string]string{
		t06A7TableCollationRule: "      values: [utf8mb4_bin]\n      require_explicit: true\n",
	})
	offPolicy := t05PolicyPath(t, map[string]string{})
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/allowed", func(t *testing.T) {
			t06A7WantPass(t, t06A7Audit(t, dialect, t06A7TableCollateSQL, optionalPolicy))
		})
		t.Run(string(dialect)+"/allowed_case_insensitive", func(t *testing.T) {
			t06A7WantPass(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c INT) COLLATE=UTF8MB4_BIN;", optionalPolicy))
		})
		t.Run(string(dialect)+"/denied", func(t *testing.T) {
			t06A7WantFinding(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c INT) COLLATE=utf8mb4_general_ci;", optionalPolicy),
				t06A7TableCollationRule,
				`table collation must be one of [utf8mb4_bin]`,
				map[string]any{"table": "t", "option": "collate", "actual": "utf8mb4_general_ci", "allowed": []string{"utf8mb4_bin"}})
		})
		t.Run(string(dialect)+"/undeclared_optional_silent", func(t *testing.T) {
			t06A7WantPass(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c INT);", optionalPolicy))
		})
		t.Run(string(dialect)+"/undeclared_required_finding", func(t *testing.T) {
			t06A7WantFinding(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c INT);", requiredPolicy),
				t06A7TableCollationRule,
				`table collation must be one of [utf8mb4_bin]`,
				map[string]any{"table": "t", "option": "collate", "actual": "", "allowed": []string{"utf8mb4_bin"}})
		})
		t.Run(string(dialect)+"/off", func(t *testing.T) {
			t06A7WantPass(t, t06A7Audit(t, dialect,
				"CREATE TABLE t (c INT) COLLATE=utf8mb4_general_ci;", offPolicy))
		})
	}
}

// TestT06A7TableCollationVsColumnRules pins the two directions of the
// separation: a column-level COLLATE cannot satisfy the table rule's
// require_explicit, and the table COLLATE is not a column declaration the
// column rules may consume.
func TestT06A7TableCollationVsColumnRules(t *testing.T) {
	requiredPolicy := t06A7CollationPolicy(t, map[string]string{
		t06A7TableCollationRule: "      values: [utf8mb4_bin]\n      require_explicit: true\n",
	})
	matchPolicy := t06A7CollationPolicy(t, map[string]string{
		t06A7ColumnMatchRule: "      required: true\n",
	})
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/column_collate_not_table", func(t *testing.T) {
			result := t06A7Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(16) COLLATE utf8mb4_bin);", requiredPolicy)
			t06A7WantFinding(t, result, t06A7TableCollationRule,
				`table collation must be one of [utf8mb4_bin]`,
				map[string]any{"table": "t", "option": "collate", "actual": "", "allowed": []string{"utf8mb4_bin"}})
		})
		t.Run(string(dialect)+"/table_collate_not_column", func(t *testing.T) {
			// The column declares neither charset nor collation: the match
			// rule must see both-empty, not the table option.
			t06A7WantPass(t, t06A7Audit(t, dialect, t06A7TableCollateSQL, matchPolicy))
		})
	}
}

// TestT06A7SchemaBoundariesPreserved is group 3a: the CREATE/ALTER DATABASE
// collation boundary is unchanged — only create-table left the gap branch.
func TestT06A7SchemaBoundariesPreserved(t *testing.T) {
	offPolicy := t05PolicyPath(t, map[string]string{})
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range []struct {
			name string
			sql  string
		}{
			{"create_database", "CREATE DATABASE d1 COLLATE utf8mb4_bin;"},
			{"alter_database", "ALTER DATABASE d1 COLLATE utf8mb4_bin;"},
		} {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				result, err := AuditSQL(context.Background(), Request{
					SQL: tc.sql, Dialect: dialect, ConfigPath: offPolicy,
				})
				if err == nil {
					t.Fatalf("expected unsupported error, got result %#v", result)
				}
				found := false
				for _, u := range result.Unsupported {
					if u.Feature == "create_schema.option.collate" || u.Feature == "alter_schema.option.collate" {
						found = true
					}
				}
				if !found {
					t.Fatalf("unsupported = %#v, want schema collate boundary", result.Unsupported)
				}
			})
		}
	}
}

// TestT06A7CoexistingUnauditedAspects is group 3b: when a table COLLATE rides
// alongside another unaudited aspect, only the collate entry leaves — the
// other unsupported details keep feature/reason/metadata verbatim.
func TestT06A7CoexistingUnauditedAspects(t *testing.T) {
	offPolicy := t05PolicyPath(t, map[string]string{})
	result, err := AuditSQL(context.Background(), Request{
		SQL:        "CREATE TABLE t (c INT) COLLATE=utf8mb4_bin COMPRESSION='zlib';",
		Dialect:    spec.DialectMySQL,
		Schema:     "golden",
		ConfigPath: offPolicy,
	})
	if err == nil {
		t.Fatalf("expected unsupported error for the remaining aspect, got %#v", result)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("unsupported = %#v, want exactly one remaining aspect", result.Unsupported)
	}
	u := result.Unsupported[0]
	if u.Feature != "create_table.option.compression" {
		t.Fatalf("feature = %q, want create_table.option.compression", u.Feature)
	}
	if u.Reason != spec.UnsupportedUnauditedReason {
		t.Fatalf("reason = %q, want unaudited", u.Reason)
	}
}

// TestT06A7SourceFacts is group 4a: pin the extracted spec fields for every
// declared combination, straight through the real parser/extractor seam.
func TestT06A7SourceFacts(t *testing.T) {
	parsed, err := parseSQL(context.Background(), strings.Join([]string{
		"CREATE TABLE s1 (c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin);",
		"CREATE TABLE s2 (c VARCHAR(16) COLLATE utf8mb4_bin);",
		"CREATE TABLE s3 (c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE latin1_swedish_ci);",
		"CREATE TABLE s4 (c INT) COLLATE=utf8mb4_bin;",
		"CREATE TABLE s5 (c VARCHAR(16));",
	}, "\n"), spec.DialectMySQL)
	if err != nil || len(parsed.Statements) != 5 {
		t.Fatalf("parse: err=%v statements=%d", err, len(parsed.Statements))
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	want := []struct {
		table    string
		charset  string
		collate  string
		optsColl string
	}{
		{"s1", "utf8mb4", "utf8mb4_bin", ""},
		{"s2", "", "utf8mb4_bin", ""},
		{"s3", "utf8mb4", "latin1_swedish_ci", ""},
		{"s4", "", "", "utf8mb4_bin"},
		{"s5", "", "", ""},
	}
	for i, w := range want {
		ddl := statements[i].DDL
		if ddl == nil || len(ddl.Columns) != 1 {
			t.Fatalf("statement %d ddl = %+v, want one column", i, ddl)
		}
		column := ddl.Columns[0]
		if column.Charset != w.charset || column.Collation != w.collate {
			t.Fatalf("%s column = charset %q collation %q, want %q/%q",
				w.table, column.Charset, column.Collation, w.charset, w.collate)
		}
		if ddl.Options["collate"] != w.optsColl {
			t.Fatalf("%s options.collate = %q, want %q", w.table, ddl.Options["collate"], w.optsColl)
		}
	}
}

// TestT06A7DerivedStateCarriesCollate is group 4b: the CREATE-derived
// prospective state keeps the declared table option for the following
// statement — the statement no longer degrades to unknown just because an
// extracted collate key is present.
func TestT06A7DerivedStateCarriesCollate(t *testing.T) {
	sql := "CREATE TABLE t (c VARCHAR(16)) COLLATE=utf8mb4_bin;\nALTER TABLE t ADD COLUMN x INT;"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
			preAlter := enriched[1].Metadata.TargetTable
			if preAlter == nil || !preAlter.Exists {
				t.Fatalf("pre-alter state = %+v, want present derived table", preAlter)
			}
			if got := preAlter.Options["collate"]; got != "utf8mb4_bin" {
				t.Fatalf("derived Options[collate] = %q, want utf8mb4_bin", got)
			}
			column := preAlter.FindColumn("c")
			if column == nil || column.Charset != "" || column.Collation != "" {
				t.Fatalf("derived column = %+v, want no fabricated column declarations", column)
			}
		})
	}
}

// TestT06A7ScopeAndErrors is group 5: ALTER exclusion and the config-error
// channel for the new rule. The PostgreSQL gate is pinned at the rule-method
// layer (AppliesTo) where dialect handling lives.
func TestT06A7ScopeAndErrors(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/alter_unaffected", func(t *testing.T) {
			policy := t06A7CollationPolicy(t, map[string]string{
				t06A7TableCollationRule: "      values: [utf8mb4_bin]\n      require_explicit: true\n",
			})
			result := t06A7Audit(t, dialect,
				"ALTER TABLE t ADD COLUMN x INT;", policy)
			for _, f := range result.Statements[0].Findings {
				if f.RuleID == t06A7TableCollationRule {
					t.Fatalf("collation rule fired on ALTER: %#v", f)
				}
			}
		})
		t.Run(string(dialect)+"/bad_params_channel", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bad.yaml")
			var text strings.Builder
			text.WriteString("rules:\n")
			for _, entry := range catalog.All() {
				if entry.RuleID == t06A7TableCollationRule {
					text.WriteString("  \"" + t06A7TableCollationRule + "\":\n    enabled: true\n    params:\n      values: \"not-a-list\"\n")
				} else {
					text.WriteString("  \"" + entry.RuleID + "\":\n    enabled: false\n")
				}
			}
			if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := AuditSQL(context.Background(), Request{
				SQL: t06A7TableCollateSQL, Dialect: dialect, ConfigPath: path,
			})
			if err == nil {
				t.Fatalf("expected config error for non-list values, got nil")
			}
		})
	}
}

// Package audit verifies the T03/#82 coverage contract for recognized-but-uncovered DDL.
// input: audit service requests over parsed-but-unsupported, audited, and parse-failing statement mixes
// output: end-to-end coverage.status assertions proving incomplete audits never silently pass
// pos: application audit coverage regression tests for issue #82
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// writeAllRulesDisabledPolicy renders a temporary policy file that disables every
// cataloged rule, proving coverage status is a capability fact that survives
// policy shutdown.
func writeAllRulesDisabledPolicy(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "all-rules-disabled.yaml")
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var builder strings.Builder
	builder.WriteString("rules:\n")
	for _, id := range ids {
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write all-rules-disabled policy: %v", err)
	}
	return path
}

// TestAuditSQLT03GoldenPathMySQLSequenceIncomplete is the frozen T03 oracle:
// a MySQL CREATE SEQUENCE (parsed, vendor boundary) plus an audited ALTER must
// return a partial result with per-statement and aggregate coverage statuses,
// bounded unsupported evidence, a review-floor verdict, and the unsupported
// sentinel error — even with every finding rule disabled.
func TestAuditSQLT03GoldenPathMySQLSequenceIncomplete(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "CREATE SEQUENCE golden_seq START WITH 1;\nALTER TABLE t ADD COLUMN c INT;",
		Dialect:    spec.DialectMySQL,
		ConfigPath: configPath,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
	}

	if len(result.Statements) != 2 {
		t.Fatalf("expected 2 statement results preserving source order, got %#v", result.Statements)
	}
	first := result.Statements[0]
	if first.Index != 0 {
		t.Fatalf("expected first statement index 0, got %d", first.Index)
	}
	if first.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("expected first statement coverage incomplete, got %q", first.Coverage.Status)
	}
	if len(first.Findings) != 0 {
		t.Fatalf("expected no fabricated findings on unsupported statement, got %#v", first.Findings)
	}
	second := result.Statements[1]
	if second.Index != 1 || second.Coverage.Status != report.CoverageComplete {
		t.Fatalf("expected second statement complete at index 1, got %#v", second)
	}

	if result.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("expected aggregate coverage incomplete, got %q", result.Coverage.Status)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("expected review verdict floor for incomplete coverage, got %q", result.Verdict)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("expected exactly one bounded unsupported detail, got %#v", result.Unsupported)
	}
	if result.Unsupported[0].Feature != "create_sequence" {
		t.Fatalf("expected unsupported feature create_sequence, got %#v", result.Unsupported[0])
	}
	if result.Unsupported[0].Index != 0 {
		t.Fatalf("expected unsupported detail bound to statement index 0, got %d", result.Unsupported[0].Index)
	}
	if result.Unsupported[0].Reason == "" {
		t.Fatal("expected bounded unsupported reason")
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("expected unsupported-statement diagnostic")
	}
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Reason, "golden_seq") || strings.Contains(d.Reason, "START WITH") {
			t.Fatalf("diagnostic leaks raw SQL: %#v", d)
		}
	}
}

// TestAuditSQLT03SupportedAlterComplete proves the contrast path: a fully
// audited ALTER TABLE ... ADD COLUMN stays complete and passes with all rules
// disabled.
func TestAuditSQLT03SupportedAlterComplete(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "ALTER TABLE t ADD COLUMN c INT;",
		Dialect:    spec.DialectMySQL,
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("expected clean audit, got %v", err)
	}
	if result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("expected aggregate coverage complete, got %q", result.Coverage.Status)
	}
	if len(result.Statements) != 1 || result.Statements[0].Coverage.Status != report.CoverageComplete {
		t.Fatalf("expected one complete statement, got %#v", result.Statements)
	}
	if result.Verdict != report.VerdictPass {
		t.Fatalf("expected pass verdict, got %q", result.Verdict)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("expected no unsupported details, got %#v", result.Unsupported)
	}
}

// TestAuditSQLT03IncompletePreservesReject proves verdict precedence: an
// existing reject verdict survives an incomplete-coverage statement in the same
// audit.
func TestAuditSQLT03IncompletePreservesReject(t *testing.T) {
	t.Parallel()
	configPath := writeDenylistPolicy(t, "      tables: [sensitive]\n")

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "DROP TABLE sensitive;\nCREATE SEQUENCE seq_boundary START WITH 1;",
		Dialect:    spec.DialectMySQL,
		ConfigPath: configPath,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("expected reject verdict preserved over review floor, got %q", result.Verdict)
	}
	if result.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("expected aggregate coverage incomplete, got %q", result.Coverage.Status)
	}
}

// TestAuditSQLT03TiDBUnauditedBoundaries covers the named TiDB contrast cases:
// valid resource-group statements and unaudited partition sub-actions must be
// marked incomplete (not silently passed) ahead of #92/#101.
func TestAuditSQLT03TiDBUnauditedBoundaries(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)

	cases := []struct {
		name        string
		sql         string
		wantFeature string
	}{
		{name: "resource group create", sql: "CREATE RESOURCE GROUP rg1 RU_PER_SEC=100;", wantFeature: "create_resource_group"},
		{name: "resource group alter", sql: "ALTER RESOURCE GROUP rg1 RU_PER_SEC=200;", wantFeature: "alter_resource_group"},
		{name: "partition drop", sql: "ALTER TABLE t DROP PARTITION p0;", wantFeature: "alter_table.drop_partition"},
		{name: "partition reorganize", sql: "ALTER TABLE t REORGANIZE PARTITION p0 INTO (PARTITION p0 VALUES LESS THAN (10));", wantFeature: "alter_table.reorganize_partition"},
		{name: "tidb ttl table option", sql: "ALTER TABLE t TTL_ENABLE = 'OFF';", wantFeature: "alter_table.option.ttl_enable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:        tc.sql,
				Dialect:    spec.DialectTiDB,
				ConfigPath: configPath,
			})
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
			}
			if result.Coverage.Status != report.CoverageIncomplete {
				t.Fatalf("expected incomplete aggregate coverage, got %q", result.Coverage.Status)
			}
			if len(result.Statements) != 1 || result.Statements[0].Coverage.Status != report.CoverageIncomplete {
				t.Fatalf("expected single incomplete statement, got %#v", result.Statements)
			}
			found := false
			for _, u := range result.Unsupported {
				if u.Feature == tc.wantFeature {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected unsupported feature %q, got %#v", tc.wantFeature, result.Unsupported)
			}
		})
	}
}

// TestAuditSQLT03TiDBAuditedBoundariesStayComplete guards the audited side of
// the TiDB boundary: bare sequences and DROP RESOURCE GROUP keep complete
// coverage because generic-notice rules model them. Option-bearing sequences
// carry create_sequence.options evidence instead — the option list itself is
// extracted but unaudited (see the incomplete-aspects table).
func TestAuditSQLT03TiDBAuditedBoundariesStayComplete(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "CREATE SEQUENCE seq1; DROP RESOURCE GROUP rg1;",
		Dialect:    spec.DialectTiDB,
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("expected clean audit, got %v", err)
	}
	if result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("expected complete aggregate coverage, got %q", result.Coverage.Status)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("expected no unsupported details, got %#v", result.Unsupported)
	}
}

// TestAuditSQLT03ParserFailureKeepsParserContract proves genuine parse failures
// stay on the parser-error path: coverage is incomplete but the failure is not
// reclassified as a recognized-unsupported statement.
func TestAuditSQLT03ParserFailureKeepsParserContract(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "CREATE TABLE broken (id INT;",
		Dialect:    spec.DialectMySQL,
		ConfigPath: configPath,
	})
	if err == nil || errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected parser error (not unsupported), got %v", err)
	}
	if result.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("expected incomplete aggregate coverage on parse failure, got %q", result.Coverage.Status)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("parser failure must not fabricate unsupported details, got %#v", result.Unsupported)
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0].Classification != DiagnosticParserError {
		t.Fatalf("expected parser_error diagnostic, got %#v", result.Diagnostics)
	}
}

// TestAuditSQLT03ProcedureBodyParseFailureKeepsParserContract pins the
// compound-body form that currently fails to parse: it must stay on the
// parser-error path (CLI exit 2) and must never be converted into a
// recognized has_body unsupported aspect inferred from the failed text.
func TestAuditSQLT03ProcedureBodyParseFailureKeepsParserContract(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "CREATE PROCEDURE p() BEGIN SELECT 1; END;",
		Dialect:    spec.DialectMySQL,
		ConfigPath: configPath,
	})
	if err == nil || errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected parser error (not unsupported), got %v", err)
	}
	if result.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("expected incomplete aggregate coverage on parse failure, got %q", result.Coverage.Status)
	}
	for _, detail := range result.Unsupported {
		if strings.Contains(detail.Feature, "procedure") {
			t.Fatalf("procedure effects must not be inferred from failed SQL text, got %#v", detail)
		}
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0].Classification != DiagnosticParserError {
		t.Fatalf("expected parser_error diagnostic, got %#v", result.Diagnostics)
	}
}

// TestAuditSQLT03MixedValidInvalidKeepsValidResults proves a parse failure
// alongside valid statements keeps valid statement results and stays
// distinguishable from recognized-unsupported outcomes.
func TestAuditSQLT03MixedValidInvalidKeepsValidResults(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "ALTER TABLE t ADD COLUMN c INT;\nTHIS IS NOT SQL;",
		Dialect:    spec.DialectMySQL,
		ConfigPath: configPath,
	})
	if err == nil {
		t.Fatal("expected parse-failure error for invalid tail")
	}
	if errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("mixed parse failure must not masquerade as unsupported, got %v", err)
	}
	if len(result.Statements) != 1 || result.Statements[0].Coverage.Status != report.CoverageComplete {
		t.Fatalf("expected the valid statement result preserved, got %#v", result.Statements)
	}
	if result.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("expected incomplete aggregate coverage, got %q", result.Coverage.Status)
	}
}

// TestAuditSQLT03RecognizedUnauditedAspectsIncomplete locks the reworked defect
// class: parser-recognized ALTER sub-actions and nested options that no rule
// audits must surface incomplete coverage with stable bounded unsupported
// evidence instead of silently passing. Feature names are derived from
// normalized facts (operation + action/option), never from raw SQL.
func TestAuditSQLT03RecognizedUnauditedAspectsIncomplete(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)

	cases := []struct {
		name        string
		sql         string
		dialect     spec.Dialect
		wantFeature string
		wantReason  string
		wantKind    string
	}{
		{name: "mysql alter index invisible", sql: "ALTER TABLE t ALTER INDEX idx INVISIBLE;", dialect: spec.DialectMySQL, wantFeature: "alter_table.alter_index", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql alter algorithm", sql: "ALTER TABLE t ALGORITHM=INPLACE;", dialect: spec.DialectMySQL, wantFeature: "alter_table.algorithm", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql alter lock", sql: "ALTER TABLE t LOCK=NONE;", dialect: spec.DialectMySQL, wantFeature: "alter_table.lock", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb drop check", sql: "ALTER TABLE t DROP CHECK chk;", dialect: spec.DialectTiDB, wantFeature: "alter_table.drop_check", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql nested placement policy", sql: "CREATE TABLE t (id INT) PLACEMENT POLICY=p;", dialect: spec.DialectMySQL, wantFeature: "create_table.option.placement_policy", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "tidb nested placement policy", sql: "CREATE TABLE t (id INT) PLACEMENT POLICY=p;", dialect: spec.DialectTiDB, wantFeature: "create_table.option.placement_policy", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb drop procedure", sql: "DROP PROCEDURE IF EXISTS p;", dialect: spec.DialectTiDB, wantFeature: "drop_procedure", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "tidb explain analyze", sql: "EXPLAIN ANALYZE DELETE FROM t;", dialect: spec.DialectTiDB, wantFeature: "explain_analyze", wantReason: spec.UnsupportedUnauditedReason, wantKind: "unknown"},
		{name: "mysql explain analyze", sql: "EXPLAIN ANALYZE DELETE FROM t;", dialect: spec.DialectMySQL, wantFeature: "explain_analyze", wantReason: spec.UnsupportedUnauditedReason, wantKind: "unknown"},
		{name: "tidb execute prepared", sql: "EXECUTE stmt;", dialect: spec.DialectTiDB, wantFeature: "execute_prepared", wantReason: spec.UnsupportedUnauditedReason, wantKind: "unknown"},
		{name: "mysql execute prepared", sql: "EXECUTE stmt;", dialect: spec.DialectMySQL, wantFeature: "execute_prepared", wantReason: spec.UnsupportedUnauditedReason, wantKind: "unknown"},
		{name: "tidb trace", sql: "TRACE DELETE FROM t;", dialect: spec.DialectTiDB, wantFeature: "trace", wantReason: spec.UnsupportedUnauditedReason, wantKind: "unknown"},
		{name: "tidb explain explore", sql: "EXPLAIN EXPLORE SELECT * FROM t;", dialect: spec.DialectTiDB, wantFeature: "explain_explore", wantReason: spec.UnsupportedUnauditedReason, wantKind: "unknown"},
		{name: "tidb sequence options", sql: "CREATE SEQUENCE seq1 START WITH 1;", dialect: spec.DialectTiDB, wantFeature: "create_sequence.options", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb alter sequence options", sql: "ALTER SEQUENCE seq1 START WITH 100;", dialect: spec.DialectTiDB, wantFeature: "alter_sequence.options", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb placement policy options", sql: "CREATE PLACEMENT POLICY p1 PRIMARY_REGION='us-east-1';", dialect: spec.DialectTiDB, wantFeature: "create_placement_policy.options", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb alter placement options", sql: "ALTER PLACEMENT POLICY p1 REGIONS='us-west-1';", dialect: spec.DialectTiDB, wantFeature: "alter_placement_policy.options", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql create schema charset", sql: "CREATE DATABASE d1 CHARACTER SET utf8mb4;", dialect: spec.DialectMySQL, wantFeature: "create_schema.option.charset", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb create schema collate", sql: "CREATE DATABASE d1 COLLATE utf8mb4_bin;", dialect: spec.DialectTiDB, wantFeature: "create_schema.option.collate", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql alter schema charset", sql: "ALTER DATABASE d1 CHARACTER SET utf8mb4;", dialect: spec.DialectMySQL, wantFeature: "alter_schema.option.charset", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb alter schema placement", sql: "ALTER DATABASE d1 PLACEMENT POLICY=p1;", dialect: spec.DialectTiDB, wantFeature: "alter_schema.option.placement_policy", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql alter schema placement vendor", sql: "ALTER DATABASE d1 PLACEMENT POLICY=p1;", dialect: spec.DialectMySQL, wantFeature: "alter_schema.option.placement_policy", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "tidb create table collate", sql: "CREATE TABLE t (id INT) COLLATE utf8mb4_bin;", dialect: spec.DialectTiDB, wantFeature: "create_table.option.collate", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb create temporary table", sql: "CREATE TEMPORARY TABLE t (id INT PRIMARY KEY);", dialect: spec.DialectTiDB, wantFeature: "create_table.temporary", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql create temporary table", sql: "CREATE TEMPORARY TABLE t (id INT PRIMARY KEY);", dialect: spec.DialectMySQL, wantFeature: "create_table.temporary", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb create global temporary table", sql: "CREATE GLOBAL TEMPORARY TABLE t (id INT PRIMARY KEY) ON COMMIT DELETE ROWS;", dialect: spec.DialectTiDB, wantFeature: "create_table.temporary.global", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql create global temporary table", sql: "CREATE GLOBAL TEMPORARY TABLE t (id INT PRIMARY KEY) ON COMMIT DELETE ROWS;", dialect: spec.DialectMySQL, wantFeature: "create_table.temporary.global", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "tidb drop temporary table", sql: "DROP TEMPORARY TABLE t;", dialect: spec.DialectTiDB, wantFeature: "drop_table.temporary", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql drop temporary table", sql: "DROP TEMPORARY TABLE t;", dialect: spec.DialectMySQL, wantFeature: "drop_table.temporary", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb create table auto_random", sql: "CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);", dialect: spec.DialectTiDB, wantFeature: "create_table.column.auto_random", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql create table auto_random", sql: "CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);", dialect: spec.DialectMySQL, wantFeature: "create_table.column.auto_random", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "tidb alter add column auto_random", sql: "ALTER TABLE t ADD COLUMN id BIGINT AUTO_RANDOM;", dialect: spec.DialectTiDB, wantFeature: "alter_table.add_columns.column.auto_random", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql alter add column auto_random", sql: "ALTER TABLE t ADD COLUMN id BIGINT AUTO_RANDOM;", dialect: spec.DialectMySQL, wantFeature: "alter_table.add_columns.column.auto_random", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "mysql alter add column first", sql: "ALTER TABLE t ADD COLUMN c INT FIRST;", dialect: spec.DialectMySQL, wantFeature: "alter_table.add_columns.column_position", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb alter add column after", sql: "ALTER TABLE t ADD COLUMN c INT AFTER id;", dialect: spec.DialectTiDB, wantFeature: "alter_table.add_columns.column_position", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql create table tidb-only options", sql: "CREATE TABLE t (id INT) SHARD_ROW_ID_BITS=4 PRE_SPLIT_REGIONS=2;", dialect: spec.DialectMySQL, wantFeature: "create_table.option.shard_row_id_bits", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "mysql create table auto_random_base", sql: "CREATE TABLE t (id INT) AUTO_RANDOM_BASE=10;", dialect: spec.DialectMySQL, wantFeature: "create_table.option.auto_random_base", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "tidb create table auto_random_base", sql: "CREATE TABLE t (id INT) AUTO_RANDOM_BASE=10;", dialect: spec.DialectTiDB, wantFeature: "create_table.option.auto_random_base", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql create table unique global", sql: "CREATE TABLE t (id INT UNIQUE GLOBAL);", dialect: spec.DialectMySQL, wantFeature: "create_table.column.unique_global", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "tidb create table unique global", sql: "CREATE TABLE t (id INT UNIQUE GLOBAL);", dialect: spec.DialectTiDB, wantFeature: "create_table.column.unique_global", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql create table primary key global", sql: "CREATE TABLE t (id INT PRIMARY KEY GLOBAL);", dialect: spec.DialectMySQL, wantFeature: "create_table.column.primary_key_global", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "mysql create table index global", sql: "CREATE TABLE t (id INT, UNIQUE KEY uk (id) GLOBAL);", dialect: spec.DialectMySQL, wantFeature: "create_table.index.global", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "tidb create table index global", sql: "CREATE TABLE t (id INT, UNIQUE KEY uk (id) GLOBAL);", dialect: spec.DialectTiDB, wantFeature: "create_table.index.global", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql alter add index global", sql: "ALTER TABLE t ADD UNIQUE KEY uk (c) GLOBAL;", dialect: spec.DialectMySQL, wantFeature: "alter_table.add_index.index.global", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "mysql create index global", sql: "CREATE UNIQUE INDEX uk ON t (c) GLOBAL;", dialect: spec.DialectMySQL, wantFeature: "create_index.create_index.index.global", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "mysql alter add list index global", sql: "ALTER TABLE t ADD (c INT, UNIQUE KEY uk (c) GLOBAL);", dialect: spec.DialectMySQL, wantFeature: "alter_table.add_index.index.global", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "tidb alter add list index global", sql: "ALTER TABLE t ADD (c INT, UNIQUE KEY uk (c) GLOBAL);", dialect: spec.DialectTiDB, wantFeature: "alter_table.add_index.index.global", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql alter add list primary global", sql: "ALTER TABLE t ADD (c INT, PRIMARY KEY (c) GLOBAL);", dialect: spec.DialectMySQL, wantFeature: "alter_table.add_constraint.index.global", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "mysql alter add list check", sql: "ALTER TABLE t ADD (c INT, CHECK (c > 0));", dialect: spec.DialectMySQL, wantFeature: "alter_table.add_constraint.check", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb alter add list primary global", sql: "ALTER TABLE t ADD (c INT, PRIMARY KEY (c) GLOBAL);", dialect: spec.DialectTiDB, wantFeature: "alter_table.add_constraint.index.global", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql alter add list comment interference", sql: "ALTER TABLE t ADD (c INT COMMENT 'ADD CONSTRAINT', UNIQUE KEY uk (c) GLOBAL);", dialect: spec.DialectMySQL, wantFeature: "alter_table.add_index.index.global", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "mysql create table index expr", sql: "CREATE TABLE t (a VARCHAR(32), KEY ix (a, (LOWER(a))));", dialect: spec.DialectMySQL, wantFeature: "create_table.index.expr", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb create table index expr", sql: "CREATE TABLE t (a VARCHAR(32), KEY ix (a, (LOWER(a))));", dialect: spec.DialectTiDB, wantFeature: "create_table.index.expr", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "mysql create index expr prefix desc", sql: "CREATE INDEX ix ON t (c(10), (LOWER(b)) DESC);", dialect: spec.DialectMySQL, wantFeature: "create_index.create_index.index.expr", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb create index expr prefix desc", sql: "CREATE INDEX ix ON t (c(10), (LOWER(b)) DESC);", dialect: spec.DialectTiDB, wantFeature: "create_index.create_index.index.expr", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "mysql create table index prefix desc", sql: "CREATE TABLE t (a VARCHAR(32), KEY ix (a(8) DESC));", dialect: spec.DialectMySQL, wantFeature: "create_table.index.prefix", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql alter add index prefix", sql: "ALTER TABLE t ADD INDEX ix (c(8));", dialect: spec.DialectMySQL, wantFeature: "alter_table.add_index.index.prefix", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql drop user multi", sql: "DROP USER u1, u2;", dialect: spec.DialectMySQL, wantFeature: "drop_user.unaudited_targets", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb drop sequence multi", sql: "DROP SEQUENCE s1, s2;", dialect: spec.DialectTiDB, wantFeature: "drop_sequence.unaudited_targets", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb drop role multi", sql: "DROP ROLE r1, r2;", dialect: spec.DialectTiDB, wantFeature: "drop_role.unaudited_targets", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql grant multi", sql: "GRANT SELECT ON db.* TO 'a'@'%', 'b'@'%';", dialect: spec.DialectMySQL, wantFeature: "grant.unaudited_targets", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb revoke single", sql: "REVOKE SELECT ON db.* FROM 'a'@'%';", dialect: spec.DialectTiDB, wantFeature: "revoke.unaudited_targets", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql create user multi", sql: "CREATE USER u1 IDENTIFIED BY 'x', u2 IDENTIFIED BY 'y';", dialect: spec.DialectMySQL, wantFeature: "create_user.unaudited_targets", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql alter user multi", sql: "ALTER USER u1 IDENTIFIED BY 'x', u2 IDENTIFIED BY 'y';", dialect: spec.DialectMySQL, wantFeature: "alter_user.unaudited_targets", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql alter tidb-only option", sql: "ALTER TABLE t AUTO_RANDOM_BASE=10;", dialect: spec.DialectMySQL, wantFeature: "alter_table.option.auto_random_base", wantReason: spec.UnsupportedVendorBoundaryReason, wantKind: "ddl"},
		{name: "mysql alter modify column position", sql: "ALTER TABLE t MODIFY COLUMN c INT AFTER id;", dialect: spec.DialectMySQL, wantFeature: "alter_table.modify_column.column_position", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql alter change column position", sql: "ALTER TABLE t CHANGE COLUMN c c2 INT FIRST;", dialect: spec.DialectMySQL, wantFeature: "alter_table.change_column.column_position", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql create procedure body", sql: "CREATE PROCEDURE p() SELECT 1;", dialect: spec.DialectMySQL, wantFeature: "create_procedure.body", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql create table generated column", sql: "CREATE TABLE t (a INT, b INT GENERATED ALWAYS AS (a+1) STORED);", dialect: spec.DialectMySQL, wantFeature: "create_table.column.generated", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql create table inline reference", sql: "CREATE TABLE t (id INT, pid INT REFERENCES parent(id));", dialect: spec.DialectMySQL, wantFeature: "create_table.column.reference", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "tidb create table inline check", sql: "CREATE TABLE t (id INT CHECK (id > 0));", dialect: spec.DialectTiDB, wantFeature: "create_table.column.check", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
		{name: "mysql create table inline unique", sql: "CREATE TABLE t (id INT UNIQUE);", dialect: spec.DialectMySQL, wantFeature: "create_table.column.unique", wantReason: spec.UnsupportedUnauditedReason, wantKind: "ddl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:        tc.sql,
				Dialect:    tc.dialect,
				ConfigPath: configPath,
			})
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected the recognized statement retained, got %#v", result.Statements)
			}
			stmt := result.Statements[0]
			if stmt.Index != 0 || stmt.Kind != tc.wantKind {
				t.Fatalf("expected retained statement kind=%s index=0, got %#v", tc.wantKind, stmt)
			}
			if stmt.RawSQL == "" || stmt.NormalizedSQL == "" {
				t.Fatalf("expected statement identity preserved, got %#v", stmt)
			}
			if stmt.Coverage.Status != report.CoverageIncomplete {
				t.Fatalf("expected statement coverage incomplete, got %#v", stmt.Coverage)
			}
			if len(stmt.Findings) != 0 {
				t.Fatalf("expected no fabricated findings, got %#v", stmt.Findings)
			}
			if result.Coverage.Status != report.CoverageIncomplete {
				t.Fatalf("expected aggregate coverage incomplete, got %q", result.Coverage.Status)
			}
			if result.Verdict != report.VerdictReview {
				t.Fatalf("expected review verdict floor, got %q", result.Verdict)
			}
			var found *spec.UnsupportedDetail
			for i := range result.Unsupported {
				if result.Unsupported[i].Feature == tc.wantFeature {
					found = &result.Unsupported[i]
				}
			}
			if found == nil {
				t.Fatalf("expected unsupported feature %q, got %#v", tc.wantFeature, result.Unsupported)
			}
			if found.Reason != tc.wantReason {
				t.Fatalf("expected reason %q, got %q", tc.wantReason, found.Reason)
			}
			for _, d := range result.Diagnostics {
				if strings.Contains(d.Reason, "INVISIBLE") || strings.Contains(d.Reason, "chk") ||
					strings.Contains(d.Reason, "POLICY") || strings.Contains(d.Reason, "stmt") {
					t.Fatalf("diagnostic leaks statement text: %#v", d)
				}
			}
		})
	}
}

// TestAuditSQLT03UnsupportedFeatureReasonPairs locks the exact
// (index, feature, reason) association on mixed-gap statements: a vendor
// boundary and an unaudited gap on the same statement must keep their own
// reasons instead of being interchangeable.
func TestAuditSQLT03UnsupportedFeatureReasonPairs(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)

	cases := []struct {
		name      string
		sql       string
		dialect   spec.Dialect
		wantIndex int
		want      [][2]string
	}{
		{name: "mysql mixed column uniqueness", sql: "CREATE TABLE t (id INT UNIQUE GLOBAL, c INT UNIQUE);", dialect: spec.DialectMySQL, want: [][2]string{
			{"create_table.column.unique", spec.UnsupportedUnauditedReason},
			{"create_table.column.unique_global", spec.UnsupportedVendorBoundaryReason},
		}},
		{name: "mysql alter add list index global and check", sql: "ALTER TABLE t ADD (c INT, UNIQUE KEY uk (c) GLOBAL, CHECK (c > 0));", dialect: spec.DialectMySQL, want: [][2]string{
			{"alter_table.add_index.index.global", spec.UnsupportedVendorBoundaryReason},
			{"alter_table.add_constraint.check", spec.UnsupportedUnauditedReason},
		}},
		{name: "mysql unsupported second statement", sql: "ALTER TABLE t ADD COLUMN c INT; CREATE TEMPORARY TABLE x (id INT);", dialect: spec.DialectMySQL, wantIndex: 1, want: [][2]string{
			{"create_table.temporary", spec.UnsupportedUnauditedReason},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:        tc.sql,
				Dialect:    tc.dialect,
				ConfigPath: configPath,
			})
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
			}
			got := make([][2]string, 0, len(result.Unsupported))
			for _, u := range result.Unsupported {
				if u.Index != tc.wantIndex {
					t.Fatalf("expected unsupported entry bound to statement %d, got %#v", tc.wantIndex, u)
				}
				got = append(got, [2]string{u.Feature, u.Reason})
			}
			sort.Slice(got, func(i, j int) bool { return got[i][0] < got[j][0] })
			want := append([][2]string{}, tc.want...)
			sort.Slice(want, func(i, j int) bool { return want[i][0] < want[j][0] })
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("expected exact (feature,reason) pairs %#v, got %#v", want, got)
			}
		})
	}
}

// TestAuditSQLT03RecognizedOutOfSurfaceStaysComplete locks the counter-examples:
// ordinary read-only EXPLAIN, session-scope PREPARE/DEALLOCATE, and the
// TiDB-audited placement-policy alter stay complete — only execution-capable or
// unaudited forms flip coverage.
func TestAuditSQLT03RecognizedOutOfSurfaceStaysComplete(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)

	cases := []struct {
		name    string
		sql     string
		dialect spec.Dialect
	}{
		{name: "tidb plain explain", sql: "EXPLAIN DELETE FROM t;", dialect: spec.DialectTiDB},
		{name: "mysql plain explain", sql: "EXPLAIN SELECT id FROM t;", dialect: spec.DialectMySQL},
		{name: "tidb prepare deallocate", sql: "PREPARE s FROM 'SELECT 1'; DEALLOCATE PREPARE s;", dialect: spec.DialectTiDB},
		{name: "tidb alter placement policy", sql: "ALTER TABLE t PLACEMENT POLICY=p;", dialect: spec.DialectTiDB},
		{name: "mysql plain create table", sql: "CREATE TABLE t (id INT PRIMARY KEY);", dialect: spec.DialectMySQL},
		{name: "tidb plain create table", sql: "CREATE TABLE t (id INT PRIMARY KEY);", dialect: spec.DialectTiDB},
		{name: "mysql drop table", sql: "DROP TABLE t;", dialect: spec.DialectMySQL},
		{name: "mysql drop procedure", sql: "DROP PROCEDURE p;", dialect: spec.DialectMySQL},
		{name: "mysql alter add list column only", sql: "ALTER TABLE t ADD (c INT);", dialect: spec.DialectMySQL},
		{name: "mysql alter add list index", sql: "ALTER TABLE t ADD (c INT, UNIQUE KEY uk (c));", dialect: spec.DialectMySQL},
		{name: "mysql alter add list index only", sql: "ALTER TABLE t ADD (UNIQUE KEY uk (id));", dialect: spec.DialectMySQL},
		{name: "mysql alter add list foreign key", sql: "ALTER TABLE t ADD (c INT, FOREIGN KEY (c) REFERENCES p(id));", dialect: spec.DialectMySQL},
		{name: "tidb alter add list check", sql: "ALTER TABLE t ADD (c INT, CHECK (c > 0));", dialect: spec.DialectTiDB},
		{name: "mysql alter add list primary key only", sql: "ALTER TABLE t ADD (PRIMARY KEY (c));", dialect: spec.DialectMySQL},
		{name: "mysql drop user single", sql: "DROP USER u1;", dialect: spec.DialectMySQL},
		{name: "tidb drop sequence single", sql: "DROP SEQUENCE s1;", dialect: spec.DialectTiDB},
		{name: "mysql create index plain", sql: "CREATE INDEX ix ON t (c);", dialect: spec.DialectMySQL},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:        tc.sql,
				Dialect:    tc.dialect,
				ConfigPath: configPath,
			})
			if err != nil {
				t.Fatalf("expected clean audit, got %v", err)
			}
			if result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("expected complete aggregate coverage, got %q", result.Coverage.Status)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", result.Unsupported)
			}
		})
	}
}

// TestAuditSQLT03MySQLPlacementAlterIsVendorBoundary proves the TiDB-only
// placement-policy alter action is a vendor boundary under MySQL even though
// TiDB audits the same action at notice level.
func TestAuditSQLT03MySQLPlacementAlterIsVendorBoundary(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "ALTER TABLE t PLACEMENT POLICY=p;",
		Dialect:    spec.DialectMySQL,
		ConfigPath: configPath,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Feature != "alter_table.placement_policy" {
		t.Fatalf("expected alter_table.placement_policy unsupported detail, got %#v", result.Unsupported)
	}
	if result.Unsupported[0].Reason != spec.UnsupportedVendorBoundaryReason {
		t.Fatalf("expected vendor-boundary reason, got %q", result.Unsupported[0].Reason)
	}
}

// TestAuditSQLT03UnsupportedAndParserErrorCoexist proves one batch can carry
// both diagnostic classes: a recognized-unsupported statement, a genuine parse
// failure, and an audited statement coexist without either being reclassified.
func TestAuditSQLT03UnsupportedAndParserErrorCoexist(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "ALTER TABLE t ADD COLUMN c INT;\nCREATE SEQUENCE seq_boundary START WITH 1;\nTHIS IS NOT SQL;",
		Dialect:    spec.DialectMySQL,
		ConfigPath: configPath,
	})
	if err == nil {
		t.Fatal("expected parse-failure error for invalid tail")
	}
	if len(result.Statements) != 2 {
		t.Fatalf("expected audited and unsupported statements retained, got %#v", result.Statements)
	}
	if result.Statements[0].Coverage.Status != report.CoverageComplete {
		t.Fatalf("expected first statement complete, got %#v", result.Statements[0])
	}
	if result.Statements[1].Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("expected unsupported statement incomplete, got %#v", result.Statements[1])
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Feature != "create_sequence" {
		t.Fatalf("expected one create_sequence unsupported detail, got %#v", result.Unsupported)
	}
	var sawParserError, sawUnsupported bool
	for _, d := range result.Diagnostics {
		switch d.Classification {
		case DiagnosticParserError:
			sawParserError = true
		case DiagnosticUnsupportedStatement:
			sawUnsupported = true
		}
	}
	if !sawParserError || !sawUnsupported {
		t.Fatalf("expected parser_error and unsupported_statement diagnostics to coexist, got %#v", result.Diagnostics)
	}
	if result.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("expected incomplete aggregate coverage, got %q", result.Coverage.Status)
	}
}

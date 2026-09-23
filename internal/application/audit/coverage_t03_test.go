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
// the TiDB boundary: sequences, placement policies, and DROP RESOURCE GROUP keep
// complete coverage because generic-notice rules model them.
func TestAuditSQLT03TiDBAuditedBoundariesStayComplete(t *testing.T) {
	t.Parallel()
	configPath := writeAllRulesDisabledPolicy(t)

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "CREATE SEQUENCE seq1 START WITH 1; DROP RESOURCE GROUP rg1;",
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

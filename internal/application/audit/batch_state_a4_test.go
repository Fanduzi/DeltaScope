// Package audit verifies the T05-A4 ordinary single-column MODIFY post-state.
// input: ordered MySQL/TiDB statements whose later MODIFY must read the
// conditional column definition published by the previous ordinary MODIFY
// output: public AuditSQL results that pin source and target lengths, plus
// direct pre-state length assertions on the enriched statements
// pos: T05-A4 regression for conditional MODIFY state supply; compatibility
// rule meaning stays on the accepted #83 contract
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t05RuleModifyExists = "ddl.alter.modify_column.exists.require"
	t05RuleModifyCompat = "ddl.alter.modify_column.compatibility.require"

	t05A4NarrowSQL = "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL);\n" +
		"ALTER TABLE t MODIFY COLUMN c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
		"ALTER TABLE t MODIFY COLUMN c VARCHAR(15) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;"
)

func t05A4FourRulePolicy(t *testing.T) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{
		t05RuleCreateForbid: "",
		t05RuleAlterRequire: "",
		t05RuleModifyExists: "",
		t05RuleModifyCompat: "      required: true\n",
	})
}

func t05A4Audit(t *testing.T, sql string, dialect spec.Dialect, provider MetadataProvider, policy string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          dialect,
		Schema:           "golden",
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	return result
}

func t05A4Length(t *testing.T, statement spec.Statement, column string) int {
	t.Helper()
	if statement.Metadata == nil || statement.Metadata.TargetTable == nil {
		t.Fatalf("missing pre-state for column %s", column)
	}
	found := statement.Metadata.TargetTable.FindColumn(column)
	if found == nil {
		t.Fatalf("pre-state column %s missing from %+v", column, statement.Metadata.TargetTable.Columns)
	}
	return found.Length
}

func t05A4Shrink(t *testing.T, result report.Result, index, source, target int) {
	t.Helper()
	statement := result.Statements[index]
	if statement.Coverage.Status != report.CoverageComplete || len(statement.EvidenceGaps) != 0 || len(statement.Findings) != 1 {
		t.Fatalf("statement %d = coverage %s findings %#v gaps %#v, want one complete blocker", index, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
	}
	finding := statement.Findings[0]
	if finding.RuleID != t05RuleModifyCompat || finding.Level != rule.LevelBlocker || finding.StatementIndex != index {
		t.Fatalf("finding = %#v, want %s blocker on statement %d", finding, t05RuleModifyCompat, index)
	}
	if finding.Location == nil || finding.Location.Line != index+1 {
		t.Fatalf("finding location = %#v, want line %d", finding.Location, index+1)
	}
	if finding.Metadata["action"] != "modify_column" || finding.Metadata["table"] != "t" || finding.Metadata["name"] != "c" || finding.Metadata["column_name"] != "c" {
		t.Fatalf("finding metadata identity = %#v, want modify_column t.c", finding.Metadata)
	}
	if finding.Metadata["source_length"] != source || finding.Metadata["target_length"] != target {
		t.Fatalf("finding lengths = %#v, want source_length=%d target_length=%d", finding.Metadata, source, target)
	}
}

// TestAuditSQLT05A4ModifyNarrowThirdStatement is the first-path regression:
// after VARCHAR(10) then VARCHAR(20), the third statement must see source
// length 20 and emit exactly one shrink blocker to 15.
func TestAuditSQLT05A4ModifyNarrowThirdStatement(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t05AbsentProvider{}
			result := t05A4Audit(t, t05A4NarrowSQL, dialect, provider, t05A4FourRulePolicy(t))
			if len(result.Statements) != 3 {
				t.Fatalf("expected 3 statements, got %d", len(result.Statements))
			}
			for _, index := range []int{0, 1} {
				statement := result.Statements[index]
				if statement.Index != index || statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = index %d coverage %s findings %#v gaps %#v, want complete with none", index, statement.Index, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
				}
			}
			t05A4Shrink(t, result, 2, 20, 15)
			if result.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictReject {
				t.Fatalf("aggregate = %s/%s, want complete/reject", result.Coverage.Status, result.Verdict)
			}
			if len(result.Unsupported) != 0 || len(result.Diagnostics) != 0 {
				t.Fatalf("unexpected unsupported/diagnostics: %+v / %+v", result.Unsupported, result.Diagnostics)
			}
			if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
				t.Fatalf("provider read ledger = %#v, want exactly [golden.t]", provider.calls)
			}
			enriched := enrichA3(t, t05A4NarrowSQL, dialect, &t05AbsentProvider{})
			if got := t05A4Length(t, enriched[1], "c"); got != 10 {
				t.Fatalf("statement 2 pre-state c.Length = %d, want 10", got)
			}
			if got := t05A4Length(t, enriched[2], "c"); got != 20 {
				t.Fatalf("statement 3 pre-state c.Length = %d, want 20", got)
			}
		})
	}
}

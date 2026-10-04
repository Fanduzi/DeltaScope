// Package audit verifies the T05-A5 column-identity migration.
// input: real parser statements and shared AuditSQL requests whose later
// MODIFY and CREATE INDEX must read the column published by CHANGE or RENAME
// output: public results plus direct pre-state assertions for c versus c2
// pos: T05-A5 regression for single-column CHANGE and RENAME COLUMN state
// note: if this file changes, update this header and module README.md.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t05A5ChangeFirstSQL = "CREATE TABLE t (\n" +
	"  id INT PRIMARY KEY,\n" +
	"  c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL\n" +
	");\n" +
	"ALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"ALTER TABLE t MODIFY COLUMN c2 VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"CREATE INDEX idx_c2 ON t(c2);"

func t05A5IdentityPolicy(t *testing.T) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{
		t05RuleCreateForbid:                             "",
		t05RuleAlterRequire:                             "",
		"ddl.alter.change_column.exists.require":        "",
		"ddl.alter.change_column.compatibility.require": "      required: true\n",
		"ddl.alter.rename_column.exists.require":        "",
		"ddl.alter.change_column.target.exists.forbid":  "",
		"ddl.alter.rename_column.target.exists.forbid":  "",
		"ddl.alter.rename_column.version.require":       "      required: true\n",
		t05RuleModifyExists:                             "",
		t05RuleModifyCompat:                             "      required: true\n",
		t05RuleCreateIndexColumns:                       "      required: true\n",
	})
}

// TestAuditSQLT05A5ChangeFirstPath is the first-path contract: CHANGE c to c2
// keeps length 10, the following MODIFY sees that length and publishes 20,
// and CREATE INDEX on c2 has nothing left to complain about.
func TestAuditSQLT05A5ChangeFirstPath(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t05AbsentProvider{}
			result := t05A4Audit(t, t05A5ChangeFirstSQL, dialect, provider, t05A5IdentityPolicy(t))
			if len(result.Statements) != 4 {
				t.Fatalf("expected 4 statements, got %d", len(result.Statements))
			}
			for index, statement := range result.Statements {
				if statement.Index != index || statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = index %d coverage %s findings %#v gaps %#v, want complete with none", index, statement.Index, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
				}
			}
			if result.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
				t.Fatalf("aggregate = %s/%s, want complete/pass", result.Coverage.Status, result.Verdict)
			}
			if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
				t.Fatalf("provider read ledger = %#v, want exactly [golden.t]", provider.calls)
			}
			enriched := enrichA3(t, t05A5ChangeFirstSQL, dialect, &t05AbsentProvider{})
			if enriched[1].Metadata == nil || enriched[1].Metadata.TargetTable == nil || enriched[1].Metadata.TargetTable.FindColumn("c") == nil || enriched[1].Metadata.TargetTable.FindColumn("c2") != nil {
				t.Fatalf("CHANGE pre-state = %+v, want c present and c2 absent", enriched[1].Metadata)
			}
			if got := t05A4Length(t, enriched[1], "c"); got != 10 {
				t.Fatalf("CHANGE pre-state c.Length = %d, want 10", got)
			}
			if enriched[2].Metadata == nil || enriched[2].Metadata.TargetTable == nil || enriched[2].Metadata.TargetTable.FindColumn("c") != nil {
				t.Fatalf("MODIFY pre-state still contains c: %+v", enriched[2].Metadata)
			}
			if got := t05A4Length(t, enriched[2], "c2"); got != 10 {
				t.Fatalf("MODIFY pre-state c2.Length = %d, want 10", got)
			}
			if enriched[3].Metadata == nil || enriched[3].Metadata.TargetTable == nil || enriched[3].Metadata.TargetTable.FindColumn("c") != nil {
				t.Fatalf("CREATE INDEX pre-state still contains c: %+v", enriched[3].Metadata)
			}
			if got := t05A4Length(t, enriched[3], "c2"); got != 20 {
				t.Fatalf("CREATE INDEX pre-state c2.Length = %d, want 20", got)
			}
		})
	}
}

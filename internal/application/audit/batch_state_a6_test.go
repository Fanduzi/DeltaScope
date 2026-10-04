// Package audit verifies the T05-A6 ordinary single-column DROP post-state.
// input: real parser statements and shared AuditSQL requests whose later
// MODIFY and CREATE INDEX must read the columns left by DROP COLUMN
// output: public results plus direct pre-state assertions for the remaining
// column order, length, and primary key
// pos: T05-A6 regression for a dependency-free DROP COLUMN state transition
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t05RuleDropColumnExists = "ddl.alter.drop_column.exists.require"

const t05A6FirstPathSQL = "CREATE TABLE t (\n" +
	"  id INT PRIMARY KEY,\n" +
	"  obsolete INT,\n" +
	"  keep_c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL\n" +
	");\n" +
	"ALTER TABLE t DROP COLUMN obsolete;\n" +
	"ALTER TABLE t MODIFY COLUMN keep_c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"CREATE INDEX idx_keep ON t(keep_c);"

func t05A6SixRulePolicy(t *testing.T) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{
		t05RuleCreateForbid:       "",
		t05RuleAlterRequire:       "",
		t05RuleDropColumnExists:   "",
		t05RuleModifyExists:       "",
		t05RuleModifyCompat:       "      required: true\n",
		t05RuleCreateIndexColumns: "      required: true\n",
	})
}

func t05A6ColumnNames(snapshot *spec.TableSnapshot) []string {
	if snapshot == nil {
		return nil
	}
	names := make([]string, len(snapshot.Columns))
	for i, column := range snapshot.Columns {
		names[i] = column.Name
	}
	return names
}

// TestAuditSQLT05A6DropColumnFirstPath is the first-path contract. DROP
// obsolete must leave [id, keep_c] with keep_c still VARCHAR(10), and the
// following MODIFY must see that length before publishing 20.
func TestAuditSQLT05A6DropColumnFirstPath(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t05AbsentProvider{}
			result := t05A4Audit(t, t05A6FirstPathSQL, dialect, provider, t05A6SixRulePolicy(t))
			if len(result.Statements) != 4 {
				t.Fatalf("expected 4 statements, got %d", len(result.Statements))
			}
			if result.RuleSummary == nil || result.RuleSummary.Loaded != 6 {
				t.Fatalf("rule_summary = %+v, want loaded 6", result.RuleSummary)
			}
			for index, statement := range result.Statements {
				if statement.Index != index || statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = index %d coverage %s findings %#v gaps %#v, want complete with none", index, statement.Index, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
				}
			}
			if result.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
				t.Fatalf("aggregate = %s/%s, want complete/pass", result.Coverage.Status, result.Verdict)
			}
			if len(result.Unsupported) != 0 || len(result.Diagnostics) != 0 {
				t.Fatalf("unexpected unsupported/diagnostics: %+v / %+v", result.Unsupported, result.Diagnostics)
			}
			if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
				t.Fatalf("provider read ledger = %#v, want exactly [golden.t]", provider.calls)
			}

			enriched := enrichA3(t, t05A6FirstPathSQL, dialect, &t05AbsentProvider{})
			dropPre := enriched[1].Metadata.TargetTable
			if got := t05A6ColumnNames(dropPre); len(got) != 3 || got[0] != "id" || got[1] != "obsolete" || got[2] != "keep_c" {
				t.Fatalf("DROP pre-state columns = %#v, want [id obsolete keep_c]", got)
			}
			modifyPre := enriched[2].Metadata.TargetTable
			if got := t05A6ColumnNames(modifyPre); len(got) != 2 || got[0] != "id" || got[1] != "keep_c" || modifyPre.FindColumn("obsolete") != nil {
				t.Fatalf("MODIFY pre-state columns = %#v, want [id keep_c] without obsolete", got)
			}
			keep := modifyPre.FindColumn("keep_c")
			if keep == nil || keep.Length != 10 || keep.Charset != "utf8mb4" || keep.Collation != "utf8mb4_bin" || !keep.NotNull {
				t.Fatalf("MODIFY pre-state keep_c = %+v, want length 10 utf8mb4/utf8mb4_bin NOT NULL", keep)
			}
			if modifyPre.PrimaryKey == nil || modifyPre.PrimaryKey.Name != "primary" || len(modifyPre.PrimaryKey.Columns) != 1 || modifyPre.PrimaryKey.Columns[0] != "id" {
				t.Fatalf("MODIFY pre-state primary key = %+v, want PRIMARY(id)", modifyPre.PrimaryKey)
			}
			indexPre := enriched[3].Metadata.TargetTable
			if indexPre.FindColumn("obsolete") != nil {
				t.Fatalf("CREATE INDEX pre-state still contains obsolete: %#v", t05A6ColumnNames(indexPre))
			}
			if got := t05A4Length(t, enriched[3], "keep_c"); got != 20 {
				t.Fatalf("CREATE INDEX pre-state keep_c.Length = %d, want 20", got)
			}

			probed := enrichA3(t, t05A6FirstPathSQL+"\nALTER TABLE t ADD COLUMN probe_z INT;", dialect, &t05AbsentProvider{})
			derived := probed[4].Metadata.TargetTable
			idx := derived.FindIndex("idx_keep")
			if idx == nil || len(idx.Columns) != 1 || idx.Columns[0] != "keep_c" {
				t.Fatalf("post-CREATE INDEX shape = %#v, want idx_keep(keep_c)", derived.Indexes)
			}
		})
	}
}

// TestAuditSQLT05A6DropColumnRemovedIndexNegative binds the old column name
// to the fourth statement. The earlier DROP still publishes [id, keep_c], so
// the index finding is a definite missing column.
func TestAuditSQLT05A6DropColumnRemovedIndexNegative(t *testing.T) {
	t.Parallel()
	const sql = "CREATE TABLE t (\n" +
		"  id INT PRIMARY KEY,\n" +
		"  obsolete INT,\n" +
		"  keep_c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL\n" +
		");\n" +
		"ALTER TABLE t DROP COLUMN obsolete;\n" +
		"ALTER TABLE t MODIFY COLUMN keep_c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
		"CREATE INDEX ix_removed ON t(obsolete);"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t05AbsentProvider{}
			result := t05A4Audit(t, sql, dialect, provider, t05A6SixRulePolicy(t))
			if len(result.Statements) != 4 {
				t.Fatalf("expected 4 statements, got %d", len(result.Statements))
			}
			for index := 0; index < 3; index++ {
				statement := result.Statements[index]
				if statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = coverage %s findings %#v gaps %#v, want complete with none", index, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
				}
			}
			last := result.Statements[3]
			if last.Coverage.Status != report.CoverageComplete || len(last.EvidenceGaps) != 0 || !strings.Contains(last.RawSQL, "ix_removed") || !strings.Contains(last.RawSQL, "obsolete") {
				t.Fatalf("last statement = coverage %s sql %q gaps %#v, want a complete ix_removed(obsolete) with no gap", last.Coverage.Status, last.RawSQL, last.EvidenceGaps)
			}
			findings := t05FindingsByRule(result, 3, t05RuleCreateIndexColumns)
			if len(findings) != 1 || len(last.Findings) != 1 {
				t.Fatalf("last findings = %#v, want exactly one index-column blocker", last.Findings)
			}
			finding := findings[0]
			if finding.Level != rule.LevelBlocker || finding.StatementIndex != 3 {
				t.Fatalf("finding = %#v, want a blocker on statement 3", finding)
			}
			if finding.Location == nil || finding.Location.Line != lastLineOf(sql, "CREATE INDEX ix_removed") {
				t.Fatalf("finding location = %+v, want the ix_removed line", finding.Location)
			}
			if finding.Metadata["schema"] != "golden" || finding.Metadata["table"] != "t" || finding.Metadata["index"] != "ix_removed" || finding.Metadata["column"] != "obsolete" || finding.Metadata["exists"] != false {
				t.Fatalf("finding metadata = %#v, want golden.t ix_removed.obsolete exists=false", finding.Metadata)
			}
			if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("aggregate = %s/%s, want complete/reject", result.Coverage.Status, result.Verdict)
			}
			if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
				t.Fatalf("provider read ledger = %#v, want exactly [golden.t]", provider.calls)
			}
			enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
			indexPre := enriched[3].Metadata.TargetTable
			if got := t05A6ColumnNames(indexPre); len(got) != 2 || got[0] != "id" || got[1] != "keep_c" || indexPre.FindColumn("obsolete") != nil {
				t.Fatalf("CREATE INDEX pre-state = %#v, want [id keep_c]", got)
			}
			if got := t05A4Length(t, enriched[3], "keep_c"); got != 20 {
				t.Fatalf("CREATE INDEX pre-state keep_c.Length = %d, want 20", got)
			}
		})
	}
}

func lastLineOf(sql, fragment string) int {
	index := strings.Index(sql, fragment)
	if index < 0 {
		return 0
	}
	return strings.Count(sql[:index], "\n") + 1
}

// TestAuditSQLT05A6OfflineNoProvider locks the no-provider path the CLI golden
// cases audit. CREATE stays unverified. The derived shape still lets the
// later three statements complete. An empty request schema records the index
// finding's real schema field as empty; schema golden records golden.
func TestAuditSQLT05A6OfflineNoProvider(t *testing.T) {
	t.Parallel()
	const negative = "CREATE TABLE t (\n" +
		"  id INT PRIMARY KEY,\n" +
		"  obsolete INT,\n" +
		"  keep_c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL\n" +
		");\n" +
		"ALTER TABLE t DROP COLUMN obsolete;\n" +
		"ALTER TABLE t MODIFY COLUMN keep_c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
		"CREATE INDEX ix_removed ON t(obsolete);"
	policy := t05A6SixRulePolicy(t)
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, schema := range []string{"", "golden"} {
			schema := schema
			t.Run(string(dialect)+"/"+schema, func(t *testing.T) {
				t.Parallel()
				positive := auditT05A6Offline(t, t05A6FirstPathSQL, dialect, schema, policy)
				if positive.Verdict != report.VerdictReview || positive.Coverage.Status != report.CoverageUnverified {
					t.Fatalf("positive aggregate = %s/%s, want unverified/review", positive.Coverage.Status, positive.Verdict)
				}
				assertT05A6OfflineCreateGap(t, positive)
				for index := 1; index < 4; index++ {
					statement := positive.Statements[index]
					if statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
						t.Fatalf("positive statement %d = %s findings %#v gaps %#v", index, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
					}
				}
				result := auditT05A6Offline(t, negative, dialect, schema, policy)
				if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageUnverified {
					t.Fatalf("negative aggregate = %s/%s, want unverified/reject", result.Coverage.Status, result.Verdict)
				}
				assertT05A6OfflineCreateGap(t, result)
				last := result.Statements[3]
				if last.Coverage.Status != report.CoverageComplete || len(last.EvidenceGaps) != 0 || len(last.Findings) != 1 {
					t.Fatalf("negative last = coverage %s findings %#v gaps %#v", last.Coverage.Status, last.Findings, last.EvidenceGaps)
				}
				finding := last.Findings[0]
				if finding.RuleID != t05RuleCreateIndexColumns || finding.Metadata["schema"] != schema || finding.Metadata["table"] != "t" || finding.Metadata["index"] != "ix_removed" || finding.Metadata["column"] != "obsolete" || finding.Metadata["exists"] != false {
					t.Fatalf("finding = %#v, want schema %q", finding, schema)
				}
			})
		}
	}
}

func auditT05A6Offline(t *testing.T, sql string, dialect spec.Dialect, schema, policy string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL:        sql,
		Dialect:    dialect,
		Schema:     schema,
		ConfigPath: policy,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.RuleSummary == nil || result.RuleSummary.Loaded != 6 {
		t.Fatalf("rule_summary = %+v, want loaded 6", result.RuleSummary)
	}
	return result
}

func assertT05A6OfflineCreateGap(t *testing.T, result report.Result) {
	t.Helper()
	if result.Statements[0].Coverage.Status != report.CoverageUnverified {
		t.Fatalf("create coverage = %s, want unverified", result.Statements[0].Coverage.Status)
	}
	gaps := t05GapsByRule(result, 0, t05RuleCreateForbid)
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.existence" {
		t.Fatalf("create gaps = %#v, want one unknown_table_state existence gap", gaps)
	}
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("create findings = %#v, want none", result.Statements[0].Findings)
	}
}

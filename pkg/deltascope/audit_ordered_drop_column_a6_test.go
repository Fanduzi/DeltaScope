// Package deltascope verifies the public dependency-free DROP COLUMN contract (T05-A6).
// input: public audit requests under the isolated six-rule policy for the
// first path and the removed-column index
// output: nil-error pass and reject results, with schema golden on the finding
// pos: public SDK contract tests for ordinary single-column DROP COLUMN post-state
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func writeT05A6DropPolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":                "",
		"ddl.table.exists.alter.require":                "",
		"ddl.alter.drop_column.exists.require":          "",
		"ddl.alter.modify_column.exists.require":        "",
		"ddl.alter.modify_column.compatibility.require": "      required: true\n",
		"ddl.create_index.columns.exists.require":       "      required: true\n",
	}
	ruleIDs := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	var builder strings.Builder
	builder.WriteString("rules:\n")
	for _, id := range ruleIDs {
		if _, keep := enabled[id]; keep {
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	for id, params := range enabled {
		fmt.Fprintf(&builder, "  %s:\n    enabled: true\n    level: blocker\n", strconv.Quote(id))
		if params != "" {
			builder.WriteString("    params:\n")
			builder.WriteString(params)
		}
	}
	path := filepath.Join(t.TempDir(), "t05-a6-isolated.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

const t05A6DropSQL = "CREATE TABLE t (id INT PRIMARY KEY, obsolete INT, keep_c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL);\n" +
	"ALTER TABLE t DROP COLUMN obsolete;\n" +
	"ALTER TABLE t MODIFY COLUMN keep_c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"CREATE INDEX idx_keep ON t(keep_c);"

func TestAuditT05A6DropColumnPass(t *testing.T) {
	t.Parallel()
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &fakeMetadataProvider{snapshot: &TableSnapshot{Exists: false, Schema: "golden"}}
			result, err := Audit(context.Background(), Request{
				SQL:              t05A6DropSQL,
				Dialect:          dialect,
				Schema:           "golden",
				ConfigPath:       writeT05A6DropPolicy(t),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 4 || result.Verdict != VerdictPass || result.Coverage.Status != CoverageComplete {
				t.Fatalf("aggregate = %s/%s statements %d", result.Coverage.Status, result.Verdict, len(result.Statements))
			}
			for i, statement := range result.Statements {
				if statement.Coverage.Status != CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = %+v", i, statement)
				}
			}
			if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "t" {
				t.Fatalf("provider calls = %#v", provider.tableCalls)
			}
		})
	}
}

func TestAuditT05A6RemovedIndexReject(t *testing.T) {
	t.Parallel()
	sql := strings.Replace(t05A6DropSQL, "CREATE INDEX idx_keep ON t(keep_c);", "CREATE INDEX ix_removed ON t(obsolete);", 1)
	provider := &fakeMetadataProvider{snapshot: &TableSnapshot{Exists: false, Schema: "golden"}}
	result, err := Audit(context.Background(), Request{
		SQL:              sql,
		Dialect:          DialectMySQL,
		Schema:           "golden",
		ConfigPath:       writeT05A6DropPolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != VerdictReject || result.Coverage.Status != CoverageComplete {
		t.Fatalf("aggregate = %s/%s", result.Coverage.Status, result.Verdict)
	}
	findings := result.Statements[3].Findings
	if len(findings) != 1 || findings[0].RuleID != "ddl.create_index.columns.exists.require" {
		t.Fatalf("finding = %#v", findings)
	}
	metadata := findings[0].Metadata
	if metadata["schema"] != "golden" || metadata["table"] != "t" || metadata["index"] != "ix_removed" || metadata["column"] != "obsolete" || metadata["exists"] != false {
		t.Fatalf("metadata = %#v", metadata)
	}
}

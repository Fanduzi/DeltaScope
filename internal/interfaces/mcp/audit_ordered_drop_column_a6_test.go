// Package mcpapi verifies the T05-A6 ordered DROP COLUMN contract at the MCP seam.
// input: offline six-rule audit_sql calls for the first path and the
// removed-column index
// output: isError=false review and reject results
// pos: MCP transport contract tests for ordinary single-column DROP COLUMN post-state
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func t05A6MCPSixRulePolicy(t *testing.T) string {
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

func TestAuditSQLT05A6DropColumnOffline(t *testing.T) {
	t.Parallel()
	result := callAuditSQL(t, map[string]any{
		"sql":         t05A6DropSQL,
		"dialect":     "mysql",
		"config_path": t05A6MCPSixRulePolicy(t),
	})
	if result.IsError {
		t.Fatalf("expected isError=false, got %+v", result)
	}
	body := requireAuditStructuredMap(t, result)
	if body["verdict"] != "review" {
		t.Fatalf("expected review, got %#v", body["verdict"])
	}
	statements, _ := body["statements"].([]any)
	if len(statements) != 4 {
		t.Fatalf("expected 4 statements, got %#v", body["statements"])
	}
	first, _ := statements[0].(map[string]any)
	gaps, _ := first["evidence_gaps"].([]any)
	if len(gaps) != 1 {
		t.Fatalf("expected the create gap, got %#v", first["evidence_gaps"])
	}
	for index := 1; index < 4; index++ {
		statement, _ := statements[index].(map[string]any)
		coverage, _ := statement["coverage"].(map[string]any)
		if coverage["status"] != "complete" {
			t.Fatalf("statement %d coverage = %#v", index, coverage)
		}
	}
}

func TestAuditSQLT05A6RemovedIndexOffline(t *testing.T) {
	t.Parallel()
	sql := strings.Replace(t05A6DropSQL, "CREATE INDEX idx_keep ON t(keep_c);", "CREATE INDEX ix_removed ON t(obsolete);", 1)
	result := callAuditSQL(t, map[string]any{
		"sql":         sql,
		"dialect":     "tidb",
		"config_path": t05A6MCPSixRulePolicy(t),
	})
	if result.IsError {
		t.Fatalf("expected isError=false, got %+v", result)
	}
	body := requireAuditStructuredMap(t, result)
	if body["verdict"] != "reject" {
		t.Fatalf("expected reject, got %#v", body["verdict"])
	}
	statements, _ := body["statements"].([]any)
	last, _ := statements[3].(map[string]any)
	findings, _ := last["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %#v", last["findings"])
	}
	finding, _ := findings[0].(map[string]any)
	metadata, _ := finding["metadata"].(map[string]any)
	if finding["rule_id"] != "ddl.create_index.columns.exists.require" || metadata["table"] != "t" || metadata["index"] != "ix_removed" || metadata["column"] != "obsolete" || metadata["exists"] != false {
		t.Fatalf("finding = %#v", finding)
	}
}

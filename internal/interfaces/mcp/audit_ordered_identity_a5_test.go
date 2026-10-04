// Package mcpapi verifies the T05-A5 column-identity contract at the MCP seam.
// input: offline eleven-rule audit_sql calls for a published CHANGE, a MySQL
// 5.7 RENAME rejection, a missing-version RENAME, and an unknown dialect
// output: isError=false review, reject, and gap results, plus isError=true
// pos: MCP transport contract tests for CHANGE and RENAME COLUMN post-state
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func t05A5MCPIdentityPolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":                "",
		"ddl.table.exists.alter.require":                "",
		"ddl.alter.change_column.exists.require":        "",
		"ddl.alter.change_column.compatibility.require": "      required: true\n",
		"ddl.alter.rename_column.exists.require":        "",
		"ddl.alter.modify_column.exists.require":        "",
		"ddl.alter.modify_column.compatibility.require": "      required: true\n",
		"ddl.create_index.columns.exists.require":       "      required: true\n",
		"ddl.alter.change_column.target.exists.forbid":  "",
		"ddl.alter.rename_column.target.exists.forbid":  "",
		"ddl.alter.rename_column.version.require":       "      required: true\n",
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
	path := filepath.Join(t.TempDir(), "t05-a5-isolated.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

const t05A5ChangeSQL = "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL);\n" +
	"ALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"ALTER TABLE t MODIFY COLUMN c2 VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"CREATE INDEX idx_c2 ON t(c2);"

func TestAuditSQLT05A5ChangePublishes(t *testing.T) {
	t.Parallel()
	result := callAuditSQL(t, map[string]any{
		"sql":         t05A5ChangeSQL,
		"dialect":     "mysql",
		"config_path": t05A5MCPIdentityPolicy(t),
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
		t.Fatalf("expected 4 statements, got %#v", statements)
	}
	for _, index := range []int{1, 2, 3} {
		statement, _ := statements[index].(map[string]any)
		if coverage, _ := statement["coverage"].(map[string]any); coverage["status"] != "complete" {
			t.Fatalf("statement %d = %#v", index, statement)
		}
	}
}

func TestAuditSQLT05A5RenameVersionReject(t *testing.T) {
	t.Parallel()
	sql := strings.Replace(t05A5ChangeSQL, "CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL", "RENAME COLUMN c TO c2", 1)
	result := callAuditSQL(t, map[string]any{
		"sql":            sql,
		"dialect":        "mysql",
		"config_path":    t05A5MCPIdentityPolicy(t),
		"target_version": "5.7.44",
	})
	if result.IsError {
		t.Fatalf("expected isError=false, got %+v", result)
	}
	body := requireAuditStructuredMap(t, result)
	if body["verdict"] != "reject" {
		t.Fatalf("expected reject, got %#v", body["verdict"])
	}
	statements, _ := body["statements"].([]any)
	rename, _ := statements[1].(map[string]any)
	findings, _ := rename["findings"].([]any)
	finding, _ := findings[0].(map[string]any)
	metadata, _ := finding["metadata"].(map[string]any)
	if finding["rule_id"] != "ddl.alter.rename_column.version.require" || metadata["target_version"] != "5.7.44" {
		t.Fatalf("finding = %#v", finding)
	}
}

func TestAuditSQLT05A5RenameVersionGap(t *testing.T) {
	t.Parallel()
	result := callAuditSQL(t, map[string]any{
		"sql":         "ALTER TABLE t RENAME COLUMN c TO c2;",
		"dialect":     "tidb",
		"config_path": t05A5MCPIdentityPolicy(t),
	})
	if result.IsError {
		t.Fatalf("expected isError=false, got %+v", result)
	}
	body := requireAuditStructuredMap(t, result)
	if body["verdict"] != "review" {
		t.Fatalf("expected review, got %#v", body["verdict"])
	}
	statements, _ := body["statements"].([]any)
	statement, _ := statements[0].(map[string]any)
	gaps, _ := statement["evidence_gaps"].([]any)
	found := false
	for _, item := range gaps {
		gap, _ := item.(map[string]any)
		if gap["rule_id"] == "ddl.alter.rename_column.version.require" && gap["reason_code"] == "missing_target_version" {
			found = true
		}
	}
	if !found {
		t.Fatalf("gaps = %#v", gaps)
	}
}

func TestAuditSQLT05A5UnknownDialectIsError(t *testing.T) {
	t.Parallel()
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test-version"}))
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     t05A5ChangeSQL,
			"dialect": "oracle",
		},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected isError=true for an unknown dialect")
	}
	body := requireAuditStructuredMap(t, result)
	if body["code"] != "bad_request" {
		t.Fatalf("expected bad_request, got %#v", body["code"])
	}
}

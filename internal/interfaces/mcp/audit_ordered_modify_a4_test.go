// Package mcpapi verifies the T05-A4 ordered MODIFY contract at the MCP seam.
// input: offline four-rule audit_sql calls for the VARCHAR narrow sequence,
// an isolated unknown MODIFY, and an unknown dialect
// output: isError=false reject and gap results, plus isError=true for the
// dialect error
// pos: MCP transport contract tests for ordinary single-column MODIFY post-state
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

func t05A4MCPFourRulePolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":                "",
		"ddl.table.exists.alter.require":                "",
		"ddl.alter.modify_column.exists.require":        "",
		"ddl.alter.modify_column.compatibility.require": "      required: true\n",
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
	path := filepath.Join(t.TempDir(), "t05-a4-isolated.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

const t05A4NarrowSQL = "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL);\n" +
	"ALTER TABLE t MODIFY COLUMN c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"ALTER TABLE t MODIFY COLUMN c VARCHAR(15) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;"

func TestAuditSQLT05A4ModifyNarrowReject(t *testing.T) {
	t.Parallel()
	result := callAuditSQL(t, map[string]any{
		"sql":         t05A4NarrowSQL,
		"dialect":     "mysql",
		"config_path": t05A4MCPFourRulePolicy(t),
	})
	if result.IsError {
		t.Fatalf("expected isError=false, got %+v", result)
	}
	body := requireAuditStructuredMap(t, result)
	if body["verdict"] != "reject" {
		t.Fatalf("expected reject, got %#v", body["verdict"])
	}
	statements, _ := body["statements"].([]any)
	if len(statements) != 3 {
		t.Fatalf("expected 3 statements, got %#v", body["statements"])
	}
	third, _ := statements[2].(map[string]any)
	findings, _ := third["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("expected one shrink finding, got %#v", third["findings"])
	}
	finding, _ := findings[0].(map[string]any)
	metadata, _ := finding["metadata"].(map[string]any)
	if finding["rule_id"] != "ddl.alter.modify_column.compatibility.require" || metadata["source_length"] != float64(20) || metadata["target_length"] != float64(15) {
		t.Fatalf("finding = %#v, want 20 to 15", finding)
	}
}

func TestAuditSQLT05A4IsolatedModifyGap(t *testing.T) {
	t.Parallel()
	result := callAuditSQL(t, map[string]any{
		"sql":         "ALTER TABLE t MODIFY COLUMN c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;",
		"dialect":     "tidb",
		"config_path": t05A4MCPFourRulePolicy(t),
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
	if findings, ok := statement["findings"].([]any); ok && len(findings) != 0 {
		t.Fatalf("isolated modify must have no findings, got %#v", findings)
	}
	gaps, _ := statement["evidence_gaps"].([]any)
	got := map[string]string{}
	for _, item := range gaps {
		gap, _ := item.(map[string]any)
		ruleID, _ := gap["rule_id"].(string)
		reason, _ := gap["reason_code"].(string)
		got[ruleID] = reason
	}
	if got["ddl.alter.modify_column.compatibility.require"] != "missing_source_column" || got["ddl.table.exists.alter.require"] != "unknown_table_state" {
		t.Fatalf("gaps = %#v", got)
	}
}

func TestAuditSQLT05A4UnknownDialectIsError(t *testing.T) {
	t.Parallel()
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test-version"}))
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     t05A4NarrowSQL,
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

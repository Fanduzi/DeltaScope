// Package mcpapi verifies the T07 advanced-column MCP contract.
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

// t07MCPPolicy writes the frozen t07-all profile so the tool exercises only
// the advanced-column surface.
func t07MCPPolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.alter.column.dependency.require":       "      required: true\n",
		"ddl.column.advanced.compatibility.require": "      required: true\n",
		"ddl.create_index.columns.exists.require":   "",
		"ddl.alter.rename_column.version.require":   "      required: true\n",
	}
	var builder strings.Builder
	builder.WriteString("rules:\n")
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if params, keep := enabled[id]; keep {
			fmt.Fprintf(&builder, "  %s:\n    enabled: true\n    level: blocker\n", strconv.Quote(id))
			if params != "" {
				builder.WriteString("    params:\n")
				builder.WriteString(params)
			}
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	path := filepath.Join(t.TempDir(), "t07-mcp-policy.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return path
}

func callAuditT07(t *testing.T, args map[string]any) (*sdkmcp.CallToolResult, map[string]any) {
	t.Helper()
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test-version"}))
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	return result, requireAuditStructuredMap(t, result)
}

// TestAuditSQLT07VisibilityTransport keeps the visibility family on the MCP
// seam: MySQL 8.0.46 passes, MySQL 5.7 rejects, TiDB returns the vendor
// boundary as a tool error result with the partial body retained.
func TestAuditSQLT07VisibilityTransport(t *testing.T) {
	t.Parallel()
	sql := "CREATE TABLE t (id INT PRIMARY KEY, hidden_c INT INVISIBLE, visible_c INT)"

	t.Run("mysql80_pass", func(t *testing.T) {
		t.Parallel()
		result, body := callAuditT07(t, map[string]any{
			"sql": sql, "dialect": "mysql", "target_version": "8.0.46",
			"config_path": t07MCPPolicy(t),
		})
		if result.IsError {
			t.Fatalf("expected isError=false, got %#v", body)
		}
		if body["verdict"] != "pass" {
			t.Fatalf("verdict = %#v, want pass", body["verdict"])
		}
		if c, _ := body["coverage"].(map[string]any); c["status"] != "complete" {
			t.Fatalf("coverage = %#v, want complete", body["coverage"])
		}
	})

	t.Run("mysql57_reject", func(t *testing.T) {
		t.Parallel()
		result, body := callAuditT07(t, map[string]any{
			"sql": sql, "dialect": "mysql", "target_version": "5.7.44",
			"config_path": t07MCPPolicy(t),
		})
		if result.IsError {
			t.Fatalf("expected isError=false for a policy reject, got %#v", body)
		}
		if body["verdict"] != "reject" {
			t.Fatalf("verdict = %#v, want reject", body["verdict"])
		}
	})

	t.Run("tidb_vendor_boundary", func(t *testing.T) {
		t.Parallel()
		result, body := callAuditT07(t, map[string]any{
			"sql": sql, "dialect": "tidb", "target_version": "8.5.0",
			"config_path": t07MCPPolicy(t),
		})
		if !result.IsError {
			t.Fatalf("expected isError=true for the TiDB vendor boundary, got %#v", body)
		}
		unsupported, _ := body["unsupported"].([]any)
		if len(unsupported) != 1 {
			t.Fatalf("unsupported = %#v, want one entry", unsupported)
		}
		entry, _ := unsupported[0].(map[string]any)
		if entry["feature"] != "create_table.column.visibility" {
			t.Fatalf("unsupported feature = %#v", entry["feature"])
		}
	})
}

// TestAuditSQLT07DependencyTransport keeps the dependency finding on the MCP
// seam — the shared result carries the blocker without adapter analysis.
func TestAuditSQLT07DependencyTransport(t *testing.T) {
	t.Parallel()
	_, body := callAuditT07(t, map[string]any{
		"sql":            "CREATE TABLE t (a INT, b INT GENERATED ALWAYS AS (a + 1) STORED); ALTER TABLE t DROP COLUMN a;",
		"dialect":        "mysql",
		"target_version": "8.0.46",
		"config_path":    t07MCPPolicy(t),
	})
	if body["verdict"] != "reject" {
		t.Fatalf("verdict = %#v, want reject", body["verdict"])
	}
	stmts, _ := body["statements"].([]any)
	second, _ := stmts[1].(map[string]any)
	findings, _ := second["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want one dependency blocker", findings)
	}
	finding, _ := findings[0].(map[string]any)
	if finding["rule_id"] != "ddl.alter.column.dependency.require" {
		t.Fatalf("rule_id = %#v", finding["rule_id"])
	}
}

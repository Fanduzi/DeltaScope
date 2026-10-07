// Package mcpapi verifies the T06-A7 table-collation upgrade at the MCP seam
// — the declared table COLLATE produces a normal audit_sql result body, not
// the unsupported-statement diagnostic it carried before.
// input: audit_sql calls carrying a declared table COLLATE under isolated policies
// output: complete/pass verdict body without unsupported diagnostics; policy rejection stays result data
// pos: MCP transport contract test for issue #85 T06-A7
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
)

const t06a7MCPRuleID = "ddl.table.collation.allowlist"

// t06a7MCPPolicy writes an isolated policy file for the MCP reps.
func t06a7MCPPolicy(t *testing.T, enabled map[string]string) string {
	t.Helper()
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		if params, keep := enabled[entry.RuleID]; keep {
			fmt.Fprintf(&text, "  %q:\n    enabled: true\n    level: blocker\n", entry.RuleID)
			if params != "" {
				text.WriteString("    params:\n" + params)
			}
		} else {
			fmt.Fprintf(&text, "  %q:\n    enabled: false\n", entry.RuleID)
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func callT06A7Audit(t *testing.T, policy, sql string) map[string]any {
	t.Helper()
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test-version"}))
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":         sql,
			"dialect":     "mysql",
			"config_path": policy,
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected isError result: %#v", result)
	}
	return requireAuditStructuredMap(t, result)
}

// TestAuditSQLT06A7TableCollateAllOff is the S4 representative: the declared
// table collation is governed input, not an unsupported diagnostic.
func TestAuditSQLT06A7TableCollateAllOff(t *testing.T) {
	body := callT06A7Audit(t, t06a7MCPPolicy(t, map[string]string{}),
		"CREATE TABLE t (c INT) COLLATE=utf8mb4_bin;")
	if body["verdict"] != "pass" {
		t.Fatalf("verdict = %#v, want pass", body["verdict"])
	}
	if coverage, _ := body["coverage"].(map[string]any); coverage["status"] != "complete" {
		t.Fatalf("coverage = %#v, want complete", body["coverage"])
	}
	diagnostics, _ := body["diagnostics"].([]any)
	for _, raw := range diagnostics {
		if diagnostic, _ := raw.(map[string]any); diagnostic["classification"] == "unsupported_statement" {
			t.Fatalf("unsupported_statement diagnostic resurfaced: %#v", body["diagnostics"])
		}
	}
}

// TestAuditSQLT06A7TableCollateDenied is the policy representative: the new
// rule's rejection stays a structured result, never a transport error.
func TestAuditSQLT06A7TableCollateDenied(t *testing.T) {
	body := callT06A7Audit(t, t06a7MCPPolicy(t, map[string]string{
		t06a7MCPRuleID: "      values: [utf8mb4_bin]\n      require_explicit: false\n",
	}), "CREATE TABLE t (c INT) COLLATE=utf8mb4_general_ci;")
	if body["verdict"] != "reject" {
		t.Fatalf("verdict = %#v, want reject", body["verdict"])
	}
	statements, _ := body["statements"].([]any)
	if len(statements) != 1 {
		t.Fatalf("statements = %#v, want 1", statements)
	}
	findings, _ := statements[0].(map[string]any)["findings"].([]any)
	if len(findings) != 1 || findings[0].(map[string]any)["rule_id"] != t06a7MCPRuleID {
		t.Fatalf("findings = %#v, want one %s blocker", findings, t06a7MCPRuleID)
	}
}

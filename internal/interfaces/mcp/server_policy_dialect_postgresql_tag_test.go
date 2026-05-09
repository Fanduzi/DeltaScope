//go:build postgresql

package mcpapi

import (
	"context"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLToolDefaultPolicyDialectHygienePostgreSQLExcludesMySQLFamilyRules(t *testing.T) {
	t.Parallel()

	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "CREATE TABLE pg_smoke (id bigint primary key);", "dialect": "postgresql"},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}

	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured content, got %T", result.StructuredContent)
	}
	mysqlOnly := []string{
		"ddl.table.engine.allowlist",
		"ddl.table.charset.allowlist",
		"ddl.table.row_format.allowlist",
		"ddl.table.auto_increment_init.value",
		"ddl.primary_key.unsigned.require",
		"ddl.primary_key.auto_increment.require",
		"ddl.primary_key.not_null.require",
	}
	statements, ok := body["statements"].([]any)
	if !ok {
		t.Fatalf("expected statements array, got %#v", body["statements"])
	}
	for _, rawStmt := range statements {
		stmt, ok := rawStmt.(map[string]any)
		if !ok {
			continue
		}
		findings, ok := stmt["findings"].([]any)
		if !ok {
			continue
		}
		for _, rawFinding := range findings {
			finding, ok := rawFinding.(map[string]any)
			if !ok {
				continue
			}
			ruleID, _ := finding["rule_id"].(string)
			for _, forbidden := range mysqlOnly {
				if ruleID == forbidden {
					t.Errorf("PG default MCP audit should not emit MySQL-only rule %q", forbidden)
				}
			}
			message, _ := finding["message"].(string)
			suggestion, _ := finding["suggestion"].(string)
			combined := strings.ToUpper(message + " " + suggestion)
			for _, pattern := range []string{"UNSIGNED", "AUTO_INCREMENT", "ON UPDATE CURRENT_TIMESTAMP"} {
				if strings.Contains(combined, pattern) {
					t.Errorf("PG default MCP audit should not contain MySQL-specific text %q", pattern)
				}
			}
		}
	}
	globalFindings, ok := body["global_findings"].([]any)
	if !ok {
		return
	}
	for _, rawFinding := range globalFindings {
		finding, ok := rawFinding.(map[string]any)
		if !ok {
			continue
		}
		ruleID, _ := finding["rule_id"].(string)
		for _, forbidden := range mysqlOnly {
			if ruleID == forbidden {
				t.Errorf("PG default MCP audit should not emit MySQL-only rule %q in global findings", forbidden)
			}
		}
	}
}

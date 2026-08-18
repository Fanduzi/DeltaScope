// Package mcpapi verifies MCP audit_sql tool behavior.
// input: in-process MCP audit_sql CallTool sessions and shipped default policy
// output: coverage for the compact audit_sql text surface and structured result
// pos: interface-layer tests for MCP audit_sql content vs structuredContent
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLCallTextIncludesFindingSummary(t *testing.T) {
	t.Parallel()

	result := callAuditSQL(t, map[string]any{"sql": "delete from users"})
	text := requireAuditToolText(t, result)

	if !strings.Contains(text, "Audit verdict: reject") {
		t.Fatalf("text missing verdict: %q", text)
	}
	if !strings.Contains(text, "Statements: 1") {
		t.Fatalf("text missing statement count: %q", text)
	}
	if !strings.Contains(text, "Blockers: 1") {
		t.Fatalf("text missing blocker count: %q", text)
	}
	if !strings.Contains(text, "Warnings: 0") {
		t.Fatalf("text missing warning count: %q", text)
	}
	if !strings.Contains(text, "Notices: 0") {
		t.Fatalf("text missing notice count: %q", text)
	}
	if !strings.Contains(text, "[blocker] dml.where.require: UPDATE and DELETE statements must include a WHERE clause") {
		t.Fatalf("text missing finding line: %q", text)
	}
	if !strings.Contains(text, "Suggestion: add a WHERE clause that narrows the affected rows") {
		t.Fatalf("text missing suggestion: %q", text)
	}

	body := requireAuditStructuredMap(t, result)
	if body["verdict"] != "reject" {
		t.Fatalf("structured verdict = %#v, want reject", body["verdict"])
	}
	summary, ok := body["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured summary, got %#v", body["summary"])
	}
	if summary["statements"] != float64(1) || summary["blockers"] != float64(1) {
		t.Fatalf("unexpected structured summary: %#v", summary)
	}
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one structured statement, got %#v", body["statements"])
	}
}

func TestAuditSQLCallTextIsNotStructuredJSON(t *testing.T) {
	t.Parallel()

	result := callAuditSQL(t, map[string]any{"sql": "delete from users"})
	text := requireAuditToolText(t, result)
	structuredJSON := marshalAuditStructuredJSON(t, result)

	if text == structuredJSON {
		t.Fatal("content[0].text is a second copy of structuredContent")
	}
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		t.Fatalf("content[0].text looks like a JSON dump: %q", truncateAuditText(trimmed, 120))
	}
	if strings.Contains(text, `"rule_id"`) || strings.Contains(text, `"structuredContent"`) {
		t.Fatalf("content[0].text looks like serialized JSON fields: %q", truncateAuditText(text, 120))
	}
	if len(text) > 2048 {
		t.Fatalf("delete-from-users text is %d bytes; want on the order of 1-2 KB", len(text))
	}
}

func TestAuditSQLCallTextOmitsSQLAndSkippedRules(t *testing.T) {
	t.Parallel()

	sql := "delete from users"
	result := callAuditSQL(t, map[string]any{"sql": sql})
	text := requireAuditToolText(t, result)

	if strings.Contains(text, sql) {
		t.Fatalf("content[0].text echoed raw SQL: %q", text)
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "skipped") || strings.Contains(text, "rule_summary") {
		t.Fatalf("content[0].text includes a skipped-rule dump: %q", text)
	}
}

func TestAuditSQLCallTextIncludesCreateTableFindings(t *testing.T) {
	t.Parallel()

	result := callAuditSQL(t, map[string]any{
		"sql":     "create table t (id int)",
		"dialect": "mysql",
	})
	text := requireAuditToolText(t, result)
	body := requireAuditStructuredMap(t, result)

	if body["verdict"] != "reject" {
		t.Fatalf("structured verdict = %#v, want reject", body["verdict"])
	}
	if !strings.Contains(text, "Audit verdict: reject") {
		t.Fatalf("text missing verdict: %q", text)
	}
	if !strings.Contains(text, "Blockers: 3") {
		t.Fatalf("text missing blocker count: %q", text)
	}
	if !strings.Contains(text, "Warnings: 6") {
		t.Fatalf("text missing warning count: %q", text)
	}
	if strings.Count(text, "[blocker] ") != 3 {
		t.Fatalf("expected 3 blocker finding lines, got %q", text)
	}
	if strings.Count(text, "[warning] ") != 6 {
		t.Fatalf("expected 6 warning finding lines, got %q", text)
	}
	if len(text) > 4096 {
		t.Fatalf("create-table text is %d bytes; want a compact summary", len(text))
	}
}

func TestAuditSQLEmptySQLRemainsBadRequest(t *testing.T) {
	t.Parallel()

	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test-version"}))
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": ""},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	body := requireAuditStructuredMap(t, result)
	if body["code"] != "bad_request" {
		t.Fatalf("unexpected error code: %#v", body["code"])
	}
	message, _ := body["message"].(string)
	if !strings.Contains(message, "audit SQL must not be empty") {
		t.Fatalf("unexpected empty-SQL message: %#v", body["message"])
	}
	text := requireAuditToolText(t, result)
	if !strings.Contains(text, "audit SQL must not be empty") {
		t.Fatalf("empty-SQL text missing message: %q", text)
	}
}

func callAuditSQL(t *testing.T, arguments map[string]any) *sdkmcp.CallToolResult {
	t.Helper()

	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test-version"}))
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: arguments,
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success from audit_sql, got tool error: %#v", result)
	}
	return result
}

func requireAuditToolText(t *testing.T, result *sdkmcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("expected content[0].text")
	}
	text, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	if text.Text == "" {
		t.Fatal("expected non-empty content[0].text")
	}
	return text.Text
}

func requireAuditStructuredMap(t *testing.T, result *sdkmcp.CallToolResult) map[string]any {
	t.Helper()
	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map, got %T", result.StructuredContent)
	}
	return body
}

func marshalAuditStructuredJSON(t *testing.T, result *sdkmcp.CallToolResult) string {
	t.Helper()
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	return string(raw)
}

func truncateAuditText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

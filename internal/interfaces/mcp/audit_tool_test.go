// Package mcpapi verifies MCP audit_sql tool behavior.
// input: in-process MCP audit_sql CallTool sessions and shipped default policy
// output: coverage for compact audit_sql text, structured result without CLI-only fail_on_triggered, and offline existence caveats
// pos: interface-layer tests for MCP audit_sql content vs structuredContent
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
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
	if _, ok := body["fail_on_triggered"]; ok {
		t.Fatalf("MCP Result JSON must not include fail_on_triggered, got %#v", body["fail_on_triggered"])
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

func TestAuditSQLOfflineDropColumnStatesExistenceNotChecked(t *testing.T) {
	t.Parallel()

	result := callAuditSQL(t, map[string]any{"sql": "alter table users drop column not_a_col"})
	text := requireAuditToolText(t, result)
	body := requireAuditStructuredMap(t, result)

	if body["verdict"] != "review" {
		t.Fatalf("expected review verdict, got %#v", body["verdict"])
	}
	assertJSONContextExistenceCaveat(t, body)
	if !strings.Contains(text, "existence not checked (no database connection)") {
		t.Fatalf("content[0].text must state existence was not checked, got %q", text)
	}
	if strings.Contains(text, "{") || strings.Contains(text, `"unproven"`) {
		t.Fatalf("content[0].text must not dump structured context JSON, got %q", text)
	}
	if strings.Contains(text, "existing column") {
		t.Fatalf("MCP notice must not claim the column exists, got %q", text)
	}
}

func TestAuditSQLDenylistChecksEveryMultiTargetDropAndRename(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "denylist-only.yaml")
	var builder strings.Builder
	builder.WriteString("rules:\n")
	ruleIDs := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	for _, id := range ruleIDs {
		if id == "ddl.table.denylist.forbid" {
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	builder.WriteString("  \"ddl.table.denylist.forbid\":\n    enabled: true\n    level: blocker\n    params:\n      tables: [sensitive]\n")
	if err := os.WriteFile(configPath, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write denylist policy: %v", err)
	}

	cases := []struct {
		name    string
		sql     string
		blocked bool
	}{
		{name: "single", sql: "DROP TABLE sensitive", blocked: true},
		{name: "second", sql: "DROP TABLE harmless, sensitive", blocked: true},
		{name: "all_allowed", sql: "DROP TABLE harmless, other", blocked: false},
		{name: "rename_source", sql: "RENAME TABLE harmless TO harmless_old, sensitive TO sensitive_old", blocked: true},
		{name: "rename_destination", sql: "RENAME TABLE harmless TO harmless_old, other TO sensitive", blocked: true},
	}
	for _, dialect := range []string{"mysql", "tidb"} {
		for _, tc := range cases {
			tc := tc
			t.Run(dialect+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				result := callAuditSQL(t, map[string]any{
					"sql":         tc.sql,
					"dialect":     dialect,
					"config_path": configPath,
				})
				if result.IsError {
					t.Fatalf("expected success from audit_sql, got tool error: %#v", result)
				}
				body := requireAuditStructuredMap(t, result)
				statements, ok := body["statements"].([]any)
				if !ok || len(statements) != 1 {
					t.Fatalf("expected exactly 1 structured statement, got %#v", body["statements"])
				}
				var denylist []any
				statement := statements[0].(map[string]any)
				rawFindings, _ := statement["findings"].([]any)
				for _, raw := range rawFindings {
					finding := raw.(map[string]any)
					if finding["rule_id"] == "ddl.table.denylist.forbid" {
						denylist = append(denylist, finding)
					}
				}
				wantCount := 0
				wantVerdict := "pass"
				if tc.blocked {
					wantCount = 1
					wantVerdict = "reject"
				}
				if len(denylist) != wantCount {
					t.Fatalf("expected %d denylist findings, got %#v", wantCount, statement["findings"])
				}
				if body["verdict"] != wantVerdict {
					t.Fatalf("expected structured verdict %q, got %#v", wantVerdict, body["verdict"])
				}
				text := requireAuditToolText(t, result)
				if !strings.Contains(text, "Audit verdict: "+wantVerdict) {
					t.Fatalf("text missing verdict %q: %q", wantVerdict, text)
				}
				if !tc.blocked {
					return
				}
				finding := denylist[0].(map[string]any)
				metadata, ok := finding["metadata"].(map[string]any)
				if !ok || metadata["table"] != "sensitive" {
					t.Fatalf("expected metadata.table sensitive, got %#v", finding["metadata"])
				}
			})
		}
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

func assertJSONContextExistenceCaveat(t *testing.T, payload map[string]any) {
	t.Helper()
	contextValue, ok := payload["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context object, got %#v", payload["context"])
	}
	if contextValue["mode"] != "offline" {
		t.Fatalf("expected offline mode, got %#v", contextValue)
	}
	if contextValue["note"] != "existence not checked (no database connection)" {
		t.Fatalf("expected context.note existence caveat, got %#v", contextValue["note"])
	}
	unproven, ok := contextValue["unproven"].([]any)
	if !ok {
		t.Fatalf("expected context.unproven array, got %#v", contextValue["unproven"])
	}
	got := make([]string, 0, len(unproven))
	for _, item := range unproven {
		text, _ := item.(string)
		got = append(got, text)
	}
	if strings.Join(got, ",") != "column_exists,table_exists" {
		t.Fatalf("expected unproven [column_exists table_exists], got %#v", unproven)
	}
}

func truncateAuditText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// T05-A1 ordered-state first path at the MCP seam under the frozen four-rule
// profile: isError=false, review verdict — the first statement's offline
// existence gap rides a real tool call while the derived statements stay
// complete.
func TestAuditSQLOfflineOrderedFirstPathGapThenDerivedComplete(t *testing.T) {
	t.Parallel()

	result := callAuditSQL(t, map[string]any{
		"sql":         "CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT; CREATE INDEX idx_c ON t (c);",
		"dialect":     "mysql",
		"config_path": t05MCPFourRulePolicy(t),
	})
	if result.IsError {
		t.Fatalf("expected isError=false, got %+v", result)
	}
	body := requireAuditStructuredMap(t, result)
	if body["verdict"] != "review" {
		t.Fatalf("expected review verdict, got %#v", body["verdict"])
	}
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 3 {
		t.Fatalf("expected 3 statements, got %#v", body["statements"])
	}
	stmt0 := statements[0].(map[string]any)
	if c, _ := stmt0["coverage"].(map[string]any); c["status"] != "unverified" {
		t.Fatalf("statement 0 coverage must be unverified, got %#v", stmt0["coverage"])
	}
	gaps, ok := stmt0["evidence_gaps"].([]any)
	if !ok || len(gaps) != 1 || gaps[0].(map[string]any)["reason_code"] != "unknown_table_state" {
		t.Fatalf("statement 0 must carry one unknown_table_state gap, got %#v", stmt0["evidence_gaps"])
	}
	for i := 1; i <= 2; i++ {
		stmt := statements[i].(map[string]any)
		if c, _ := stmt["coverage"].(map[string]any); c["status"] != "complete" {
			t.Fatalf("statement %d coverage must be complete via derived state, got %#v", i, stmt["coverage"])
		}
	}
}

// t05MCPFourRulePolicy writes the frozen T05-A1 first-path profile: only the
// four existence rules enabled at blocker, everything else disabled.
func t05MCPFourRulePolicy(t *testing.T) string {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("rules:\n")
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":          "",
		"ddl.table.exists.alter.require":          "",
		"ddl.alter.add_column.exists.forbid":      "",
		"ddl.create_index.columns.exists.require": "      required: true\n",
	}
	ruleIDs := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
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
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return path
}

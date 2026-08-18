//go:build postgresql

// Package mcpapi verifies MCP audit_sql text on the PostgreSQL-capable build.
// input: in-process MCP audit_sql CallTool sessions with dialect=postgresql
// output: coverage for compact review-verdict finding text
// pos: PG-tagged interface-layer tests for MCP audit_sql text
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"strings"
	"testing"
)

func TestAuditSQLCallTextIncludesPostgreSQLDropColumnFinding(t *testing.T) {
	t.Parallel()

	result := callAuditSQL(t, map[string]any{
		"sql":     "alter table users drop column email",
		"dialect": "postgresql",
	})
	text := requireAuditToolText(t, result)

	if !strings.Contains(text, "Audit verdict: review") {
		t.Fatalf("text missing review verdict: %q", text)
	}
	if !strings.Contains(text, `[warning] ddl.pg.alter.drop_column.advisory: DROP COLUMN "email" removes data permanently on PostgreSQL`) {
		t.Fatalf("text missing drop-column finding: %q", text)
	}
	if !strings.Contains(text, "Suggestion: Verify no application code, views, or stored procedures reference this column before dropping.") {
		t.Fatalf("text missing drop-column suggestion: %q", text)
	}

	body := requireAuditStructuredMap(t, result)
	if body["verdict"] != "review" {
		t.Fatalf("structured verdict = %#v, want review", body["verdict"])
	}
}

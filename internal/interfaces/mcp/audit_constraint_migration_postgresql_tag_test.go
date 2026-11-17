//go:build postgresql

package mcpapi

import (
	"context"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLPGConstraintMigration(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	for _, tc := range []struct{ sql, verdict string }{
		{"ALTER TABLE t ADD PRIMARY KEY(id)", "review"},
		{"ALTER TABLE t ADD PRIMARY KEY USING INDEX idx", "pass"},
	} {
		r, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{"sql": tc.sql, "dialect": "postgresql"}})
		if err != nil || r.IsError {
			t.Fatalf("result=%+v err=%v", r, err)
		}
		body := requireAuditStructuredMap(t, r)
		if body["verdict"] != tc.verdict {
			t.Fatalf("verdict=%v", body["verdict"])
		}
		found := false
		for _, s := range body["statements"].([]any) {
			findings, _ := s.(map[string]any)["findings"].([]any)
			for _, v := range findings {
				f := v.(map[string]any)
				if f["rule_id"] == "ddl.pg.alter.add_primary_key.concurrent_index.warn" && f["level"] == "warning" {
					found = true
				}
			}
		}
		if found != (tc.verdict == "review") {
			t.Fatal("lost shared declaration-mode semantics")
		}
	}
}

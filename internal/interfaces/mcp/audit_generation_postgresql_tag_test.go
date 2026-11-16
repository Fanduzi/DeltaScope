//go:build postgresql

package mcpapi

import (
	"context"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"testing"
)

func TestAuditSQLPGGenerationWarningParity(t *testing.T) {
	s, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	r, err := s.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{"sql": "ALTER TABLE t ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY(CYCLE)", "dialect": "postgresql"}})
	if err != nil || r.IsError {
		t.Fatalf("result=%+v err=%v", r, err)
	}
	body := requireAuditStructuredMap(t, r)
	if body["verdict"] != "review" {
		t.Fatalf("verdict=%v", body["verdict"])
	}
	for _, s := range body["statements"].([]any) {
		for _, v := range s.(map[string]any)["findings"].([]any) {
			f := v.(map[string]any)
			if f["rule_id"] == "ddl.pg.column.identity.cycle.warn" && f["level"] == "warning" {
				return
			}
		}
	}
	t.Fatal("missing shared cycling warning")
}

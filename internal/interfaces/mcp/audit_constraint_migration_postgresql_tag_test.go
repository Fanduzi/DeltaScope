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
	for _, tc := range []struct {
		sql          string
		buildWarning bool
	}{
		{"ALTER TABLE t ADD PRIMARY KEY(id)", true},
		{"ALTER TABLE t ADD PRIMARY KEY USING INDEX idx", false},
	} {
		r, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{"sql": tc.sql, "dialect": "postgresql"}})
		if err != nil || r.IsError {
			t.Fatalf("result=%+v err=%v", r, err)
		}
		body := requireAuditStructuredMap(t, r)
		if body["verdict"] != "review" {
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
		if found != tc.buildWarning {
			t.Fatal("lost shared declaration-mode semantics")
		}
		statements := body["statements"].([]any)
		if len(statements) != 1 {
			t.Fatalf("unexpected statements: %v", statements)
		}
		gaps, _ := statements[0].(map[string]any)["evidence_gaps"].([]any)
		wantRule, wantReason := "ddl.pg.alter.constraint_using_index.valid", "unproven_constraint_using_index"
		if tc.buildWarning {
			wantRule, wantReason = "ddl.pg.alter.constraint_columns.valid", "unproven_pg_constraint_columns"
		}
		if len(gaps) != 1 {
			t.Fatalf("unexpected constraint gaps: %v", gaps)
		}
		gap := gaps[0].(map[string]any)
		facts, _ := gap["required_facts"].([]any)
		if gap["rule_id"] != wantRule || gap["reason_code"] != wantReason || len(facts) != 1 || facts[0] != "instance.version" {
			t.Fatalf("unexpected constraint gap: %v", gap)
		}
	}
}

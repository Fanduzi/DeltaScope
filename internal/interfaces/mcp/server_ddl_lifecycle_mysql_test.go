package mcpapi

import (
	"context"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLToolMySQLDDLLifecycleFindings(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{name: "rename_table", sql: "RENAME TABLE users TO users_old", wantRuleID: "ddl.rename_table.notice"},
		{name: "create_index", sql: "CREATE INDEX idx_email ON users (email)", wantRuleID: "ddl.create_index.notice"},
		{name: "create_user", sql: "CREATE USER 'admin'@'%' IDENTIFIED BY 's3cret'", wantRuleID: "ddl.create_user.notice"},
		{name: "drop_resource_group", sql: "DROP RESOURCE GROUP rg1", wantRuleID: "ddl.drop_resource_group.notice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(Config{Version: "test-version"})
			session, err := connectClientSession(context.Background(), server)
			if err != nil {
				t.Fatalf("connect session: %v", err)
			}
			t.Cleanup(func() { _ = session.Close() })

			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name: "audit_sql",
				Arguments: map[string]any{
					"sql":     tt.sql,
					"dialect": "mysql",
				},
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
			stmts, ok := body["statements"].([]any)
			if !ok || len(stmts) == 0 {
				t.Fatalf("expected statements, got %#v", body["statements"])
			}
			stmt, ok := stmts[0].(map[string]any)
			if !ok {
				t.Fatalf("expected map statement, got %#v", stmts[0])
			}
			findings, ok := stmt["findings"].([]any)
			if !ok {
				findings = []any{}
			}
			found := false
			for _, f := range findings {
				fm, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if fm["rule_id"] == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding %q, got %#v", tt.wantRuleID, findings)
			}
		})
	}
}

func TestAuditSQLToolTiDBDDLLifecycleFindings(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{name: "create_placement_policy", sql: "CREATE PLACEMENT POLICY p1 PRIMARY_REGION='us-east-1' REGIONS='us-east-1'", wantRuleID: "ddl.create_placement_policy.notice"},
		{name: "create_sequence", sql: "CREATE SEQUENCE seq1 START WITH 1 INCREMENT BY 1", wantRuleID: "ddl.create_sequence.notice"},
		{name: "alter_table_placement_policy", sql: "ALTER TABLE users PLACEMENT POLICY p1", wantRuleID: "ddl.tidb.alter_table.placement_policy.notice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(Config{Version: "test-version"})
			session, err := connectClientSession(context.Background(), server)
			if err != nil {
				t.Fatalf("connect session: %v", err)
			}
			t.Cleanup(func() { _ = session.Close() })

			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name: "audit_sql",
				Arguments: map[string]any{
					"sql":     tt.sql,
					"dialect": "tidb",
				},
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
			stmts, ok := body["statements"].([]any)
			if !ok || len(stmts) == 0 {
				t.Fatalf("expected statements, got %#v", body["statements"])
			}
			stmt, ok := stmts[0].(map[string]any)
			if !ok {
				t.Fatalf("expected map statement, got %#v", stmts[0])
			}
			findings, ok := stmt["findings"].([]any)
			if !ok {
				findings = []any{}
			}
			found := false
			for _, f := range findings {
				fm, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if fm["rule_id"] == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding %q, got %#v", tt.wantRuleID, findings)
			}
		})
	}
}

func TestAuditSQLToolMySQLDDLNoLeakPasswords(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "CREATE USER 'admin'@'%' IDENTIFIED BY 's3cretP@ss'",
			"dialect": "mysql",
		},
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
	stmts, ok := body["statements"].([]any)
	if !ok || len(stmts) == 0 {
		t.Fatalf("expected statements, got %#v", body["statements"])
	}
	stmt, ok := stmts[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map statement, got %#v", stmts[0])
	}
	findings, ok := stmt["findings"].([]any)
	if !ok {
		findings = []any{}
	}
	forbidden := []string{"s3cretp@ss", "identified by"}
	for _, f := range findings {
		fm, ok := f.(map[string]any)
		if !ok {
			continue
		}
		for _, field := range []string{"message", "suggestion"} {
			val, _ := fm[field].(string)
			lower := strings.ToLower(val)
			for _, substr := range forbidden {
				if strings.Contains(lower, substr) {
					t.Fatalf("finding %s leaks forbidden payload %q: %s", field, substr, val)
				}
			}
		}
		meta, ok := fm["metadata"].(map[string]any)
		if !ok {
			continue
		}
		for k, v := range meta {
			s, _ := v.(string)
			lower := strings.ToLower(s)
			for _, substr := range forbidden {
				if strings.Contains(lower, substr) {
					t.Fatalf("finding metadata[%q] leaks forbidden payload %q: %s", k, substr, s)
				}
			}
		}
	}
}

//go:build postgresql

package mcpapi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLToolPostgreSQLCreateTableConstraintsReturnNormalResult(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	cases := map[string]string{
		"named table-level CHECK":        "create table orders (id bigint primary key, amount numeric, constraint chk_orders_amount check (amount > 0));",
		"column-level inline CHECK":      "create table orders (id bigint primary key, amount numeric check (amount > 0));",
		"named table-level UNIQUE":       "create table users (id bigint primary key, email text, constraint uq_users_email unique (email));",
		"column-level inline UNIQUE":     "create table users (id bigint primary key, email text unique);",
		"named table-level FOREIGN KEY":  "create table orders (id bigint primary key, user_id bigint, constraint fk_orders_user foreign key (user_id) references users(id));",
		"column-level inline REFERENCES": "create table orders (id bigint primary key, user_id bigint references users(id));",
	}

	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name: "audit_sql",
				Arguments: map[string]any{
					"sql":     sql,
					"dialect": "postgresql",
				},
			})
			if err != nil {
				t.Fatalf("call audit_sql: %v", err)
			}
			if result.IsError {
				t.Fatalf("expected success result, got tool error: %#v", result)
			}
			body := result.StructuredContent.(map[string]any)
			contextValue, ok := body["context"].(map[string]any)
			if !ok || contextValue["mode"] != "offline" {
				t.Fatalf("expected offline context, got %#v", body["context"])
			}
			statements, ok := body["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", body["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			if statement["kind"] != "ddl" {
				t.Fatalf("expected ddl kind, got %#v", statement["kind"])
			}
			unsupported, ok := body["unsupported"].([]any)
			if ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported entries, got %#v", unsupported)
			}
		})
	}
}

func TestAuditSQLToolPostgreSQLCreateTableForeignKeyRendersForbidFinding(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	cases := map[string]string{
		"named FOREIGN KEY": "create table orders (id bigint primary key, user_id bigint, constraint bad_fk foreign key (user_id) references users(id));",
		"inline REFERENCES": "create table orders (id bigint primary key, user_id bigint references users(id));",
	}

	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name: "audit_sql",
				Arguments: map[string]any{
					"sql":     sql,
					"dialect": "postgresql",
				},
			})
			if err != nil {
				t.Fatalf("call audit_sql: %v", err)
			}
			if result.IsError {
				t.Fatalf("expected success result, got tool error: %#v", result)
			}
			body := result.StructuredContent.(map[string]any)
			statements, ok := body["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", body["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) < 1 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}
			found := false
			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if ok && finding["rule_id"] == "ddl.table.foreign_key.forbid" {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected foreign_key forbid finding, got %#v", findings)
			}
		})
	}
}

func TestAuditSQLToolPostgreSQLSchemaQualifiedReferencesRenderFKFindings(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	cases := map[string]string{
		"inline REFERENCES public.users":   "create table orders (id bigint primary key, user_id bigint references public.users(id));",
		"named FK REFERENCES public.users": "create table orders (id bigint primary key, approver_id bigint, constraint fk_orders_approver foreign key (approver_id) references public.users(id));",
	}

	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name: "audit_sql",
				Arguments: map[string]any{
					"sql":     sql,
					"dialect": "postgresql",
				},
			})
			if err != nil {
				t.Fatalf("call audit_sql: %v", err)
			}
			if result.IsError {
				t.Fatalf("expected success result, got tool error: %#v", result)
			}
			body := result.StructuredContent.(map[string]any)
			statements, ok := body["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", body["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) < 1 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}
			found := false
			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if ok && finding["rule_id"] == "ddl.table.foreign_key.forbid" {
					found = true
					meta, _ := finding["metadata"].(map[string]any)
					if meta == nil {
						t.Fatalf("expected finding metadata, got nil")
					}
					// Schema-qualified reference must not produce "public.users" as referenced_table.
					if refTable, _ := meta["referenced_table"].(string); refTable == "public.users" {
						t.Fatalf("referenced_table must not be schema-qualified concat 'public.users', got %q", refTable)
					}
				}
			}
			if !found {
				t.Fatalf("expected foreign_key forbid finding, got %#v", findings)
			}
		})
	}
}

func TestAuditSQLToolPostgreSQLCreateTableBoundaryReturnsUnsupportedError(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	cases := map[string]struct {
		sql     string
		feature string
	}{
		"exclusion": {
			sql:     "CREATE TABLE bookings (room_id int, during tsrange, EXCLUDE USING gist (room_id WITH =, during WITH &&));",
			feature: "exclusion_constraint",
		},
		"partitioned": {
			sql:     "CREATE TABLE events (id bigint, created_at timestamptz NOT NULL) PARTITION BY RANGE (created_at);",
			feature: "partitioning",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name: "audit_sql",
				Arguments: map[string]any{
					"sql":     tc.sql,
					"dialect": "postgresql",
				},
			})
			if err != nil {
				t.Fatalf("call audit_sql: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected tool error for unsupported %s (%s), got success result", name, tc.feature)
			}
			body, ok := result.StructuredContent.(map[string]any)
			if !ok {
				t.Fatalf("expected structured content, got %#v", result.StructuredContent)
			}
			code, _ := body["code"].(string)
			if code == "" {
				t.Fatalf("expected error code in tool error, got %#v", body)
			}
			message, _ := body["message"].(string)
			if !strings.Contains(message, "unsupported") {
				t.Fatalf("expected unsupported in error message for feature %q, got %q", tc.feature, message)
			}
		})
	}
}

func TestAuditSQLToolPostgreSQLSchemaQualifiedForeignKeyExposesReferencedObjectMetadata(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "CREATE TABLE orders (id bigint PRIMARY KEY, approver_id bigint, CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES public.users(id));",
			"dialect": "postgresql",
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}
	body := result.StructuredContent.(map[string]any)
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) < 1 {
		t.Fatalf("expected at least one finding, got %#v", statement["findings"])
	}

	// 1. FK forbid finding must trigger.
	found := false
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.table.foreign_key.forbid" {
			found = true
			metadata, _ := finding["metadata"].(map[string]any)
			if metadata == nil {
				t.Fatalf("expected metadata map in finding, got nil")
			}

			// 2. Current metadata contains: table, constraint, columns.
			if metadata["table"] == nil {
				t.Errorf("expected metadata key 'table', got nil")
			}
			if metadata["constraint"] == nil {
				t.Errorf("expected metadata key 'constraint', got nil")
			}
			if metadata["columns"] == nil {
				t.Errorf("expected metadata key 'columns', got nil")
			}

			// v0.28.0: referenced-object metadata is now exposed.
			if metadata["referenced_schema"] == nil {
				t.Errorf("expected metadata key 'referenced_schema', got nil")
			}
			if metadata["referenced_table"] == nil {
				t.Errorf("expected metadata key 'referenced_table', got nil")
			}
			if metadata["referenced_columns"] == nil {
				t.Errorf("expected metadata key 'referenced_columns', got nil")
			}
			// referenced_table must NOT be schema-qualified.
			if refTable, _ := metadata["referenced_table"].(string); refTable == "public.users" {
				t.Fatalf("referenced_table must not be schema-qualified 'public.users'")
			}
		}
	}
	if !found {
		t.Fatalf("expected foreign_key forbid finding, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLCrossSchemaFKRendersAdvisoryNotice(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "CREATE TABLE public.orders (id bigint PRIMARY KEY, approver_id bigint, CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES auth.users(id));",
			"dialect": "postgresql",
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}
	body := result.StructuredContent.(map[string]any)
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) < 2 {
		t.Fatalf("expected at least two findings, got %#v", statement["findings"])
	}

	advisoryFound := false
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.pg.table.foreign_key.cross_schema.advisory" {
			advisoryFound = true
			if finding["level"] != "notice" {
				t.Errorf("expected advisory level notice, got %v", finding["level"])
			}
			meta, _ := finding["metadata"].(map[string]any)
			if meta == nil {
				t.Fatalf("expected metadata in advisory finding, got nil")
			}
			if meta["table_schema"] != "public" {
				t.Errorf("expected table_schema public, got %v", meta["table_schema"])
			}
			if meta["referenced_schema"] != "auth" {
				t.Errorf("expected referenced_schema auth, got %v", meta["referenced_schema"])
			}
			if meta["referenced_table"] != "users" {
				t.Errorf("expected referenced_table users, got %v", meta["referenced_table"])
			}
			refCols, _ := meta["referenced_columns"].([]any)
			if len(refCols) < 1 || refCols[0] != "id" {
				t.Errorf("expected referenced_columns [id], got %v", refCols)
			}
			if refTable, _ := meta["referenced_table"].(string); refTable == "auth.users" {
				t.Fatalf("referenced_table must not be schema-qualified 'auth.users'")
			}
		}
	}
	if !advisoryFound {
		t.Fatalf("expected cross-schema advisory finding, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLSameSchemaFKDoesNotRenderAdvisory(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "CREATE TABLE public.orders (id bigint PRIMARY KEY, user_id bigint, CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES public.users(id));",
			"dialect": "postgresql",
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}
	body := result.StructuredContent.(map[string]any)
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}

	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if ok && finding["rule_id"] == "ddl.pg.table.foreign_key.cross_schema.advisory" {
			t.Fatalf("expected no cross-schema advisory for same-schema FK, got %#v", finding)
		}
	}
}

func TestAuditSQLToolPostgreSQLBareFKDoesNotRenderAdvisory(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "CREATE TABLE public.orders (id bigint PRIMARY KEY, approver_id bigint REFERENCES users(id));",
			"dialect": "postgresql",
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}
	body := result.StructuredContent.(map[string]any)
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}

	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if ok && finding["rule_id"] == "ddl.pg.table.foreign_key.cross_schema.advisory" {
			t.Fatalf("expected no cross-schema advisory for bare FK reference, got %#v", finding)
		}
	}
}

// TestAuditSQLToolPostgreSQLGeneratedIdentityNarrowNowSupported proves that narrow
// generated/identity forms are now processed through the normal supported MCP path.
// Each case asserts: no tool error, structured content with one statement.

func TestAuditSQLToolPostgreSQLPrimaryKeyRuleCoverage(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "CREATE TABLE bad_pk_type (id integer PRIMARY KEY, name text);",
			"dialect": "postgresql",
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
		t.Fatalf("expected structured content, got %#v", result.StructuredContent)
	}
	contextValue, ok := body["context"].(map[string]any)
	if !ok || contextValue["mode"] != "offline" || contextValue["dialect"] != "postgresql" {
		t.Fatalf("expected offline postgresql context, got %#v", body["context"])
	}
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}
	found := false
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.table.primary_key.bigint.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding with rule_id ddl.table.primary_key.bigint.require, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLUniqueIndexRuleCoverage(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "CREATE UNIQUE INDEX bad_email_unique ON users (email);",
			"dialect": "postgresql",
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
		t.Fatalf("expected structured content, got %#v", result.StructuredContent)
	}
	contextValue, ok := body["context"].(map[string]any)
	if !ok || contextValue["mode"] != "offline" || contextValue["dialect"] != "postgresql" {
		t.Fatalf("expected offline postgresql context, got %#v", body["context"])
	}
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}
	found := false
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.index.unique.prefix.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding with rule_id ddl.index.unique.prefix.require, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLAlterTableAddConstraintRuleCoverage(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "ALTER TABLE users ADD CONSTRAINT bad_email_key UNIQUE (email);",
			"dialect": "postgresql",
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
		t.Fatalf("expected structured content, got %#v", result.StructuredContent)
	}
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}
	found := false
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.alter.add_index.unique.prefix.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding with rule_id ddl.alter.add_index.unique.prefix.require, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLAlterTableForeignKeyRuleCoverage(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "forbid only",
			sql:        "ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id);",
			wantRuleID: "ddl.table.foreign_key.forbid",
		},
		{
			name:       "cross_schema advisory",
			sql:        "ALTER TABLE public.orders ADD CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES auth.users(id);",
			wantRuleID: "ddl.pg.table.foreign_key.cross_schema.advisory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name: "audit_sql",
				Arguments: map[string]any{
					"sql":     tt.sql,
					"dialect": "postgresql",
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
				t.Fatalf("expected structured content, got %#v", result.StructuredContent)
			}
			statements, ok := body["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", body["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok {
				t.Fatalf("expected findings array, got %#v", statement["findings"])
			}
			found := false
			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if finding["rule_id"] == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule_id %s, got %#v", tt.wantRuleID, findings)
			}
		})
	}
}

func TestAuditSQLToolPostgreSQLAlterTableAddConstraintCheckRuleCoverage(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(configPath, []byte("rules:\n  ddl.constraint.check.name.prefix.require:\n    enabled: true\n    params:\n      prefix: ck_\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":         "ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0);",
			"dialect":     "postgresql",
			"config_path": configPath,
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
		t.Fatalf("expected structured content, got %#v", result.StructuredContent)
	}
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}

	wantRuleIDs := map[string]bool{
		"ddl.constraint.check.name.prefix.require": false,
		"ddl.pg.alter.add_check.not_valid.require": false,
	}
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ruleID, _ := finding["rule_id"].(string)
		if _, expected := wantRuleIDs[ruleID]; expected {
			wantRuleIDs[ruleID] = true
		}
	}
	for ruleID, found := range wantRuleIDs {
		if !found {
			t.Fatalf("expected finding with rule_id %s, got %#v", ruleID, findings)
		}
	}
}

func TestAuditSQLToolPostgreSQLNotValidConstraintValidationRuleCoverage(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;",
			"dialect": "postgresql",
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
		t.Fatalf("expected structured content, got %#v", result.StructuredContent)
	}
	globalFindings, ok := body["global_findings"].([]any)
	if !ok || len(globalFindings) == 0 {
		t.Fatalf("expected at least one global finding, got %#v", body["global_findings"])
	}
	found := false
	for _, item := range globalFindings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.pg.alter.not_valid_constraint.validate.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected global finding with rule_id ddl.pg.alter.not_valid_constraint.validate.require, got %#v", globalFindings)
	}
}

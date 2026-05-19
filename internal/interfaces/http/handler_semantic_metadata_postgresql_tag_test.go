//go:build postgresql

package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerAuditPostgreSQLSemanticMetadataParity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		sql          string
		ruleID       string
		wantMetadata map[string]any
		forbidden    []string
	}{
		{
			name:   "create_index_bounded_metadata",
			sql:    "CREATE INDEX idx_users_active_email ON users USING gin (email) INCLUDE (status) WHERE active = true",
			ruleID: "ddl.pg.create_index.concurrently.require",
			wantMetadata: map[string]any{
				"operation": "create_index", "index": "idx_users_active_email", "table": "users",
				"concurrently": false, "index_kind": "secondary", "access_method": "gin",
				"column_count": float64(1), "included_column_count": float64(1),
				"has_predicate": true, "has_expression_keys": false, "expression_count": float64(0),
			},
			forbidden: []string{"active = true", "WHERE active", "predicate_sql", "expression_sql"},
		},
		{
			name:   "add_column_non_null_default_metadata",
			sql:    "ALTER TABLE users ADD COLUMN created_at timestamptz NOT NULL DEFAULT now()",
			ruleID: "ddl.pg.alter.add_column.non_null_default.rewrite.warn",
			wantMetadata: map[string]any{
				"action": "add_column", "column": "created_at", "table": "users",
				"not_null": true, "has_default": true, "default_kind": "function_call",
			},
			forbidden: []string{"now()", "now", "default_sql", "default_expr"},
		},
		{
			name:   "add_column_non_null_no_default_metadata",
			sql:    "ALTER TABLE users ADD COLUMN status text NOT NULL",
			ruleID: "ddl.pg.alter.add_column.non_null_no_default.warn",
			wantMetadata: map[string]any{
				"action": "add_column", "column": "status", "table": "users",
				"not_null": true, "has_default": false,
			},
		},
		{
			name:   "set_data_type_using_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN score TYPE bigint USING score::bigint",
			ruleID: "ddl.pg.alter.set_data_type.rewrite.warn",
			wantMetadata: map[string]any{
				"action": "set_data_type", "column": "score", "table": "users", "has_using": true,
			},
			forbidden: []string{"score::bigint", "USING score", "using_sql", "using_expr"},
		},
		{
			name:   "set_tablespace_metadata",
			sql:    "ALTER TABLE users SET TABLESPACE fastspace",
			ruleID: "ddl.pg.alter.set_tablespace.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "set_tablespace", "table": "users",
				"tablespace": "fastspace",
			},
			forbidden: []string{"fillfactor", "reloptions", "cluster", "tablespace_sql", "access_method_sql"},
		},
		{
			name:   "set_access_method_heap_metadata",
			sql:    "ALTER TABLE users SET ACCESS METHOD heap",
			ruleID: "ddl.pg.alter.set_access_method.warn",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "set_access_method", "table": "users",
				"access_method": "heap", "is_default": "false",
			},
			forbidden: []string{"fillfactor", "reloptions", "cluster", "tablespace_sql", "access_method_sql"},
		},
		{
			name:   "set_access_method_default_metadata",
			sql:    "ALTER TABLE users SET ACCESS METHOD DEFAULT",
			ruleID: "ddl.pg.alter.set_access_method.warn",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "set_access_method", "table": "users",
				"access_method": "default", "is_default": "true",
			},
			forbidden: []string{"fillfactor", "reloptions", "cluster", "tablespace_sql", "access_method_sql"},
		},
		{
			name:   "enable_replica_trigger_metadata",
			sql:    "ALTER TABLE users ENABLE REPLICA TRIGGER sync_trigger",
			ruleID: "ddl.pg.alter.enable_replica_trigger.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "enable_replica_trigger", "table": "users",
				"trigger": "sync_trigger", "trigger_mode": "replica",
			},
		},
		{
			name:   "enable_always_trigger_metadata",
			sql:    "ALTER TABLE users ENABLE ALWAYS TRIGGER audit_trigger",
			ruleID: "ddl.pg.alter.enable_always_trigger.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "enable_always_trigger", "table": "users",
				"trigger": "audit_trigger", "trigger_mode": "always",
			},
		},
		{
			name:   "enable_rule_metadata",
			sql:    "ALTER TABLE users ENABLE RULE route_rule",
			ruleID: "ddl.pg.alter.enable_rule.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "enable_rule", "table": "users",
				"rule": "route_rule",
			},
		},
		{
			name:   "disable_rule_metadata",
			sql:    "ALTER TABLE users DISABLE RULE route_rule",
			ruleID: "ddl.pg.alter.disable_rule.warn",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "disable_rule", "table": "users",
				"rule": "route_rule",
			},
		},
		{
			name:   "enable_replica_rule_metadata",
			sql:    "ALTER TABLE users ENABLE REPLICA RULE route_rule",
			ruleID: "ddl.pg.alter.enable_replica_rule.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "enable_replica_rule", "table": "users",
				"rule": "route_rule", "rule_mode": "replica",
			},
		},
		{
			name:   "enable_always_rule_metadata",
			sql:    "ALTER TABLE users ENABLE ALWAYS RULE route_rule",
			ruleID: "ddl.pg.alter.enable_always_rule.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "enable_always_rule", "table": "users",
				"rule": "route_rule", "rule_mode": "always",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler, err := NewHandler("", "test-build")
			if err != nil {
				t.Fatalf("new handler: %v", err)
			}
			body := fmt.Sprintf(`{"sql":%q,"dialect":"postgresql"}`, tt.sql)
			req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode: %v", err)
			}
			finding, _ := firstFindingByRuleID(t, payload, tt.ruleID)
			metadata, ok := finding["metadata"].(map[string]any)
			if !ok {
				t.Fatalf("expected metadata map, got %#v", finding["metadata"])
			}
			for key, want := range tt.wantMetadata {
				got, ok := metadata[key]
				if !ok {
					t.Errorf("metadata[%q] missing, want %v", key, want)
				} else if got != want {
					t.Errorf("metadata[%q] = %v (%T), want %v (%T)", key, got, got, want, want)
				}
			}
			for _, word := range tt.forbidden {
				for key, val := range metadata {
					s, ok := val.(string)
					if !ok {
						continue
					}
					if strings.Contains(strings.ToLower(s), strings.ToLower(word)) {
						t.Errorf("metadata[%q] = %q contains forbidden string %q", key, s, word)
					}
				}
			}
		})
	}
}

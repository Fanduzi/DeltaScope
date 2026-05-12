//go:build postgresql

package mcpapi

import (
	"context"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLToolPostgreSQLAdvancedIndexFormsSupportedAndCovered(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "CREATE INDEX idx_users_active_email ON users (email) WHERE active = true",
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

	if unsupported, ok := body["unsupported"].([]any); ok && len(unsupported) != 0 {
		t.Fatalf("expected no unsupported details, got %#v", unsupported)
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
		if finding["rule_id"] == "ddl.pg.create_index.concurrently.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding with rule_id ddl.pg.create_index.concurrently.require, got %#v", findings)
	}
}

func TestAuditSQLToolPostgreSQLObjectLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "drop_schema_advisory",
			sql:        "DROP SCHEMA IF EXISTS staging;",
			wantRuleID: "ddl.pg.drop_schema.advisory",
		},
		{
			name:       "drop_schema_cascade_warn",
			sql:        "DROP SCHEMA IF EXISTS staging CASCADE;",
			wantRuleID: "ddl.pg.drop_schema.cascade.warn",
		},
		{
			name:       "alter_sequence_restart_warn",
			sql:        "ALTER SEQUENCE seq_order_id RESTART WITH 100;",
			wantRuleID: "ddl.pg.alter_sequence.restart.warn",
		},
		{
			name:       "drop_sequence_advisory",
			sql:        "DROP SEQUENCE IF EXISTS seq_order_id;",
			wantRuleID: "ddl.pg.drop_sequence.advisory",
		},
		{
			name:       "drop_materialized_view_advisory",
			sql:        "DROP MATERIALIZED VIEW IF EXISTS mv_stats;",
			wantRuleID: "ddl.pg.drop_materialized_view.advisory",
		},
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
				Name:      "audit_sql",
				Arguments: map[string]any{"sql": tt.sql, "dialect": "postgresql"},
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

			if unsupported, ok := body["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
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

func TestAuditSQLToolPostgreSQLAlterTableUnsupportedActionRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "set_schema_advisory",
			sql:        "ALTER TABLE users SET SCHEMA archive;",
			wantRuleID: "ddl.pg.alter.set_schema.advisory",
		},
		{
			name:       "disable_trigger_warn",
			sql:        "ALTER TABLE users DISABLE TRIGGER trg_users_audit;",
			wantRuleID: "ddl.pg.alter.disable_trigger.warn",
		},
		{
			name:       "disable_trigger_all_warn",
			sql:        "ALTER TABLE users DISABLE TRIGGER ALL;",
			wantRuleID: "ddl.pg.alter.disable_trigger.warn",
		},
		{
			name:       "replica_identity_full_warn",
			sql:        "ALTER TABLE users REPLICA IDENTITY FULL;",
			wantRuleID: "ddl.pg.alter.replica_identity_full.warn",
		},
		{
			name:       "replica_identity_using_index_notice",
			sql:        "ALTER TABLE users REPLICA IDENTITY USING INDEX users_pkey;",
			wantRuleID: "ddl.pg.alter.replica_identity_using_index.notice",
		},
		{
			name:       "detach_partition_warn",
			sql:        "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04;",
			wantRuleID: "ddl.pg.alter.detach_partition.warn",
		},
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
				Name:      "audit_sql",
				Arguments: map[string]any{"sql": tt.sql, "dialect": "postgresql"},
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

			if unsupported, ok := body["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
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

func TestAuditSQLToolPostgreSQLRefreshMaterializedViewRuleCoverage(t *testing.T) {
	t.Run("basic_refresh_concurrently_warn", func(t *testing.T) {
		server := NewServer(Config{Version: "test-version"})
		session, err := connectClientSession(context.Background(), server)
		if err != nil {
			t.Fatalf("connect session: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })

		result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
			Name:      "audit_sql",
			Arguments: map[string]any{"sql": "REFRESH MATERIALIZED VIEW mv_stats;", "dialect": "postgresql"},
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
		if unsupported, ok := body["unsupported"].([]any); ok && len(unsupported) != 0 {
			t.Fatalf("expected no unsupported, got %#v", unsupported)
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
		if !ok || len(findings) == 0 {
			t.Fatalf("expected findings, got %#v", statement["findings"])
		}
		found := false
		for _, item := range findings {
			finding, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if finding["rule_id"] == "ddl.pg.refresh_materialized_view.concurrently.warn" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected concurrently.warn, got %#v", findings)
		}
	})

	t.Run("with_no_data_both_rules", func(t *testing.T) {
		server := NewServer(Config{Version: "test-version"})
		session, err := connectClientSession(context.Background(), server)
		if err != nil {
			t.Fatalf("connect session: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })

		result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
			Name:      "audit_sql",
			Arguments: map[string]any{"sql": "REFRESH MATERIALIZED VIEW mv_stats WITH NO DATA;", "dialect": "postgresql"},
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
			t.Fatalf("expected at least 2 findings, got %#v", statement["findings"])
		}
		var foundConcurrent, foundNoData bool
		for _, item := range findings {
			finding, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if finding["rule_id"] == "ddl.pg.refresh_materialized_view.concurrently.warn" {
				foundConcurrent = true
			}
			if finding["rule_id"] == "ddl.pg.refresh_materialized_view.no_data.notice" {
				foundNoData = true
			}
		}
		if !foundConcurrent {
			t.Fatalf("expected concurrently.warn, got %#v", findings)
		}
		if !foundNoData {
			t.Fatalf("expected no_data.notice, got %#v", findings)
		}
	})
}

func TestAuditSQLToolPostgreSQLAlterTableGapRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "drop_column_advisory",
			sql:        "ALTER TABLE users DROP COLUMN email;",
			wantRuleID: "ddl.pg.alter.drop_column.advisory",
		},
		{
			name:       "validate_constraint_advisory",
			sql:        "ALTER TABLE users VALIDATE CONSTRAINT chk_price;",
			wantRuleID: "ddl.pg.alter.validate_constraint.advisory",
		},
		{
			name:       "add_column_nullable_notice",
			sql:        "ALTER TABLE users ADD COLUMN bio text;",
			wantRuleID: "ddl.pg.alter.add_column.nullable.notice",
		},
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
				Name:      "audit_sql",
				Arguments: map[string]any{"sql": tt.sql, "dialect": "postgresql"},
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

			if unsupported, ok := body["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
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

func TestAuditSQLToolPostgreSQLAlterTableLoggedStateRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "set_logged_notice",
			sql:        "ALTER TABLE users SET LOGGED;",
			wantRuleID: "ddl.pg.alter.set_logged.notice",
		},
		{
			name:       "set_unlogged_notice",
			sql:        "ALTER TABLE users SET UNLOGGED;",
			wantRuleID: "ddl.pg.alter.set_unlogged.notice",
		},
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
				Name:      "audit_sql",
				Arguments: map[string]any{"sql": tt.sql, "dialect": "postgresql"},
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

			if unsupported, ok := body["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
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
					if finding["level"] != "notice" {
						t.Errorf("expected level notice, got %v", finding["level"])
					}
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule_id %s, got %#v", tt.wantRuleID, findings)
			}
		})
	}
}

func TestAuditSQLToolPostgreSQLTypeLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "create_type_enum_notice",
			sql:         "CREATE TYPE color AS ENUM ('red', 'green', 'blue');",
			wantRuleIDs: []string{"ddl.pg.create_type.enum.notice"},
		},
		{
			name:        "alter_type_add_value_with_position",
			sql:         "ALTER TYPE color ADD VALUE 'yellow' AFTER 'green';",
			wantRuleIDs: []string{"ddl.pg.alter_type.add_value.advisory", "ddl.pg.alter_type.add_value.position.notice"},
		},
		{
			name:        "drop_type_cascade",
			sql:         "DROP TYPE IF EXISTS color CASCADE;",
			wantRuleIDs: []string{"ddl.pg.drop_type.advisory", "ddl.pg.drop_type.cascade.warn"},
		},
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
				Name:      "audit_sql",
				Arguments: map[string]any{"sql": tt.sql, "dialect": "postgresql"},
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

			if unsupported, ok := body["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
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

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
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
		})
	}
}

func TestAuditSQLToolPostgreSQLCompositeTypeLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "create_type_composite_notice",
			sql:         "CREATE TYPE address AS (street text, city text);",
			wantRuleIDs: []string{"ddl.pg.create_type.composite.notice"},
		},
		{
			name:        "alter_type_composite_rename_notice",
			sql:         "ALTER TYPE address RENAME TO mailing_address;",
			wantRuleIDs: []string{"ddl.pg.alter_type.composite_rename.notice"},
		},
		{
			name:        "alter_type_composite_set_schema_notice",
			sql:         "ALTER TYPE address SET SCHEMA archive;",
			wantRuleIDs: []string{"ddl.pg.alter_type.composite_set_schema.notice"},
		},
		{
			name:        "alter_type_add_attribute_notice",
			sql:         "ALTER TYPE address ADD ATTRIBUTE country text;",
			wantRuleIDs: []string{"ddl.pg.alter_type.add_attribute.notice"},
		},
		{
			name:        "alter_type_drop_attribute_warn",
			sql:         "ALTER TYPE address DROP ATTRIBUTE city;",
			wantRuleIDs: []string{"ddl.pg.alter_type.drop_attribute.warn"},
		},
		{
			name:        "alter_type_alter_attribute_type_warn",
			sql:         "ALTER TYPE address ALTER ATTRIBUTE street TYPE varchar(255);",
			wantRuleIDs: []string{"ddl.pg.alter_type.alter_attribute_type.warn"},
		},
		{
			name:        "alter_type_rename_attribute_notice",
			sql:         "ALTER TYPE address RENAME ATTRIBUTE street TO line1;",
			wantRuleIDs: []string{"ddl.pg.alter_type.rename_attribute.notice"},
		},
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
				Name:      "audit_sql",
				Arguments: map[string]any{"sql": tt.sql, "dialect": "postgresql"},
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

			if unsupported, ok := body["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
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

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
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
		})
	}
}

func TestAuditSQLToolPostgreSQLDomainLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "create_domain_notice",
			sql:         "CREATE DOMAIN email AS text CHECK (VALUE <> '');",
			wantRuleIDs: []string{"ddl.pg.create_domain.notice"},
		},
		{
			name:        "alter_domain_add_constraint",
			sql:         "ALTER DOMAIN email ADD CONSTRAINT email_not_empty CHECK (VALUE <> '');",
			wantRuleIDs: []string{"ddl.pg.alter_domain.constraint.notice"},
		},
		{
			name:        "alter_domain_rename",
			sql:         "ALTER DOMAIN email RENAME TO contact_email;",
			wantRuleIDs: []string{"ddl.pg.alter_domain.rename.notice"},
		},
		{
			name:        "drop_domain_cascade",
			sql:         "DROP DOMAIN IF EXISTS email CASCADE;",
			wantRuleIDs: []string{"ddl.pg.drop_domain.advisory", "ddl.pg.drop_domain.cascade.warn"},
		},
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
				Name:      "audit_sql",
				Arguments: map[string]any{"sql": tt.sql, "dialect": "postgresql"},
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

			if unsupported, ok := body["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
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

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
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
		})
	}
}

func TestAuditSQLToolPostgreSQLExtensionLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "create_extension_notice",
			sql:         "CREATE EXTENSION pg_trgm;",
			wantRuleIDs: []string{"ddl.pg.create_extension.notice"},
		},
		{
			name:        "create_extension_cascade",
			sql:         "CREATE EXTENSION pg_trgm CASCADE;",
			wantRuleIDs: []string{"ddl.pg.create_extension.notice", "ddl.pg.create_extension.cascade.warn"},
		},
		{
			name:        "alter_extension_update_notice",
			sql:         "ALTER EXTENSION pg_trgm UPDATE;",
			wantRuleIDs: []string{"ddl.pg.alter_extension.update.notice"},
		},
		{
			name:        "alter_extension_set_schema_notice",
			sql:         "ALTER EXTENSION pg_trgm SET SCHEMA extensions;",
			wantRuleIDs: []string{"ddl.pg.alter_extension.set_schema.notice"},
		},
		{
			name:        "drop_extension_cascade",
			sql:         "DROP EXTENSION IF EXISTS pg_trgm CASCADE;",
			wantRuleIDs: []string{"ddl.pg.drop_extension.advisory", "ddl.pg.drop_extension.cascade.warn"},
		},
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
				Name:      "audit_sql",
				Arguments: map[string]any{"sql": tt.sql, "dialect": "postgresql"},
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

			if unsupported, ok := body["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
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

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
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
		})
	}
}

func TestAuditSQLToolPostgreSQLPolicyLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "create_policy_notice",
			sql:        "CREATE POLICY users_select ON users FOR SELECT USING (true);",
			wantRuleID: "ddl.pg.create_policy.notice",
		},
		{
			name:       "alter_policy_notice",
			sql:        "ALTER POLICY users_select ON users USING (id > 0);",
			wantRuleID: "ddl.pg.alter_policy.notice",
		},
		{
			name:       "drop_policy_warn",
			sql:        "DROP POLICY users_select ON users;",
			wantRuleID: "ddl.pg.drop_policy.warn",
		},
		{
			name:       "enable_rls_notice",
			sql:        "ALTER TABLE users ENABLE ROW LEVEL SECURITY;",
			wantRuleID: "ddl.pg.alter.enable_rls.notice",
		},
		{
			name:       "disable_rls_warn",
			sql:        "ALTER TABLE users DISABLE ROW LEVEL SECURITY;",
			wantRuleID: "ddl.pg.alter.disable_rls.warn",
		},
		{
			name:       "force_rls_notice",
			sql:        "ALTER TABLE users FORCE ROW LEVEL SECURITY;",
			wantRuleID: "ddl.pg.alter.force_rls.notice",
		},
		{
			name:       "no_force_rls_notice",
			sql:        "ALTER TABLE users NO FORCE ROW LEVEL SECURITY;",
			wantRuleID: "ddl.pg.alter.no_force_rls.notice",
		},
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
				Name:      "audit_sql",
				Arguments: map[string]any{"sql": tt.sql, "dialect": "postgresql"},
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

			if unsupported, ok := body["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
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

func TestAuditSQLToolPostgreSQLTriggerLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "create_trigger_notice",
			sql:        "CREATE TRIGGER trg_audit AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION log_change()",
			wantRuleID: "ddl.pg.create_trigger.notice",
		},
		{
			name:       "create_constraint_trigger_warn",
			sql:        "CREATE CONSTRAINT TRIGGER trg_fk AFTER INSERT ON orders FROM items DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_fk()",
			wantRuleID: "ddl.pg.create_constraint_trigger.warn",
		},
		{
			name:       "drop_trigger_advisory",
			sql:        "DROP TRIGGER trg_audit ON users",
			wantRuleID: "ddl.pg.drop_trigger.advisory",
		},
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
				Name:      "audit_sql",
				Arguments: map[string]any{"sql": tt.sql, "dialect": "postgresql"},
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

func TestAuditSQLToolPostgreSQLAlterObjectLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "alter_schema_rename_notice",
			sql:        "ALTER SCHEMA app RENAME TO app_new;",
			wantRuleID: "ddl.pg.alter_schema.rename.notice",
		},
		{
			name:       "alter_schema_owner_notice",
			sql:        "ALTER SCHEMA app OWNER TO app_owner;",
			wantRuleID: "ddl.pg.alter_schema.owner.notice",
		},
		{
			name:       "alter_index_rename_notice",
			sql:        "ALTER INDEX idx_users_email RENAME TO idx_users_email_v2;",
			wantRuleID: "ddl.pg.alter_index.rename.notice",
		},
		{
			name:       "alter_index_set_tablespace_notice",
			sql:        "ALTER INDEX idx_users_email SET TABLESPACE pg_default;",
			wantRuleID: "ddl.pg.alter_index.set_tablespace.notice",
		},
		{
			name:       "alter_materialized_view_rename_notice",
			sql:        "ALTER MATERIALIZED VIEW mv_stats RENAME TO mv_stats_v2;",
			wantRuleID: "ddl.pg.alter_materialized_view.rename.notice",
		},
		{
			name:       "alter_materialized_view_set_schema_notice",
			sql:        "ALTER MATERIALIZED VIEW mv_stats SET SCHEMA archive;",
			wantRuleID: "ddl.pg.alter_materialized_view.set_schema.notice",
		},
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
			if unsupported, ok := body["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
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
			if !ok || len(findings) == 0 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}

			found := false
			for _, f := range findings {
				finding, ok := f.(map[string]any)
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

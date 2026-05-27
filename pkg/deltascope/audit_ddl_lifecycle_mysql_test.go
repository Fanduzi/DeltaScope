package deltascope

import (
	"context"
	"strings"
	"testing"
)

func TestAuditMySQLDDLLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{name: "rename_table_notice", sql: "RENAME TABLE users TO users_old", wantRuleID: "ddl.rename_table.notice"},
		{name: "create_index_notice", sql: "CREATE INDEX idx_email ON users (email)", wantRuleID: "ddl.create_index.notice"},
		{name: "alter_add_index_notice", sql: "ALTER TABLE users ADD INDEX idx_email (email)", wantRuleID: "ddl.alter.add_constraint.notice"},
		{name: "create_user_notice", sql: "CREATE USER 'admin'@'%' IDENTIFIED BY 'secret'", wantRuleID: "ddl.create_user.notice"},
		{name: "grant_notice", sql: "GRANT SELECT ON app.users TO 'reader'@'%'", wantRuleID: "ddl.grant.notice"},
		{name: "drop_resource_group_notice", sql: "DROP RESOURCE GROUP rg1", wantRuleID: "ddl.drop_resource_group.notice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectMySQL,
			})
			if err != nil {
				t.Fatalf("expected supported path, got error: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}
			if result.Statements[0].Kind != "ddl" {
				t.Fatalf("expected ddl kind, got %q", result.Statements[0].Kind)
			}

			found := false
			for _, f := range result.Statements[0].Findings {
				if f.RuleID == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule %q, got %#v", tt.wantRuleID, result.Statements[0].Findings)
			}
		})
	}
}

func TestAuditTiDBDDLLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{name: "rename_table_notice", sql: "RENAME TABLE users TO users_old", wantRuleID: "ddl.rename_table.notice"},
		{name: "create_index_notice", sql: "CREATE INDEX idx_email ON users (email)", wantRuleID: "ddl.create_index.notice"},
		{name: "alter_add_index_notice", sql: "ALTER TABLE users ADD INDEX idx_email (email)", wantRuleID: "ddl.alter.add_constraint.notice"},
		{name: "create_user_notice", sql: "CREATE USER 'admin'@'%' IDENTIFIED BY 'secret'", wantRuleID: "ddl.create_user.notice"},
		{name: "grant_notice", sql: "GRANT SELECT ON app.users TO 'reader'@'%'", wantRuleID: "ddl.grant.notice"},
		{name: "create_placement_policy_notice", sql: "CREATE PLACEMENT POLICY p1 PRIMARY_REGION='us-east-1' REGIONS='us-east-1'", wantRuleID: "ddl.create_placement_policy.notice"},
		{name: "create_sequence_notice", sql: "CREATE SEQUENCE seq1 START WITH 1 INCREMENT BY 1", wantRuleID: "ddl.create_sequence.notice"},
		{name: "alter_table_placement_policy_notice", sql: "ALTER TABLE users PLACEMENT POLICY p1", wantRuleID: "ddl.tidb.alter_table.placement_policy.notice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectTiDB,
			})
			if err != nil {
				t.Fatalf("expected supported path, got error: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}
			if result.Statements[0].Kind != "ddl" {
				t.Fatalf("expected ddl kind, got %q", result.Statements[0].Kind)
			}

			found := false
			for _, f := range result.Statements[0].Findings {
				if f.RuleID == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule %q, got %#v", tt.wantRuleID, result.Statements[0].Findings)
			}
		})
	}
}

func TestAuditMySQLDDLNoLeakSensitivePayloads(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		forbidden []string
	}{
		{name: "create_user_password", sql: "CREATE USER 'admin'@'%' IDENTIFIED BY 'secret'",
			forbidden: []string{"secret", "IDENTIFIED BY"}},
		{name: "grant_privilege_path", sql: "GRANT SELECT ON app.users TO 'reader'@'%'",
			forbidden: []string{"app.users", "reader"}},
		{name: "create_procedure_body", sql: "CREATE PROCEDURE p_cleanup() SELECT 1",
			forbidden: []string{"SELECT 1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectMySQL,
			})
			if err != nil {
				t.Fatalf("audit error: %v", err)
			}
			for _, stmt := range result.Statements {
				for _, f := range stmt.Findings {
					checkFindingNoForbidden(t, f, tt.forbidden)
				}
			}
		})
	}
}

func TestAuditTiDBDDLNoLeakSensitivePayloads(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		forbidden []string
	}{
		{name: "create_user_password", sql: "CREATE USER 'admin'@'%' IDENTIFIED BY 'secret'",
			forbidden: []string{"secret", "IDENTIFIED BY"}},
		{name: "grant_privilege_path", sql: "GRANT SELECT ON app.users TO 'reader'@'%'",
			forbidden: []string{"app.users", "reader"}},
		{name: "placement_policy_regions", sql: "CREATE PLACEMENT POLICY p1 PRIMARY_REGION='us-east-1' REGIONS='us-east-1'",
			forbidden: []string{"us-east-1"}},
		{name: "sequence_options", sql: "CREATE SEQUENCE seq1 START WITH 1 INCREMENT BY 1",
			forbidden: []string{"START WITH 1", "INCREMENT BY 1"}},
		{name: "alter_placement_regions", sql: "ALTER PLACEMENT POLICY p1 PRIMARY_REGION='us-west-1' REGIONS='us-west-1'",
			forbidden: []string{"us-west-1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectTiDB,
			})
			if err != nil {
				t.Fatalf("audit error: %v", err)
			}
			for _, stmt := range result.Statements {
				for _, f := range stmt.Findings {
					checkFindingNoForbidden(t, f, tt.forbidden)
				}
			}
		})
	}
}

func checkFindingNoForbidden(t *testing.T, f Finding, forbidden []string) {
	t.Helper()
	for _, substr := range forbidden {
		if strings.Contains(strings.ToLower(f.Message), strings.ToLower(substr)) {
			t.Errorf("finding message leaks forbidden payload %q: %s", substr, f.Message)
		}
		if strings.Contains(strings.ToLower(f.Suggestion), strings.ToLower(substr)) {
			t.Errorf("finding suggestion leaks forbidden payload %q: %s", substr, f.Suggestion)
		}
		for k, v := range f.Metadata {
			if k == "raw_sql" || k == "validation_result" || k == "dependency_graph" ||
				k == "lock_duration" || k == "rewrite_duration" {
				t.Errorf("finding metadata contains forbidden key %q", k)
			}
			s, ok := v.(string)
			if ok && strings.Contains(strings.ToLower(s), strings.ToLower(substr)) {
				t.Errorf("finding metadata[%q] leaks forbidden payload %q: %s", k, substr, s)
			}
		}
	}
}

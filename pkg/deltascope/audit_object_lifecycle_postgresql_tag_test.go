//go:build postgresql

package deltascope

import (
	"context"
	"testing"
)

func TestAuditPostgreSQLObjectLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "create_schema_notice",
			sql:        "CREATE SCHEMA app;",
			wantRuleID: "ddl.pg.create_schema.notice",
		},
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
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectPostgreSQL,
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

func TestAuditPostgreSQLAlterTableGapRuleCoverage(t *testing.T) {
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
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectPostgreSQL,
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

func TestAuditPostgreSQLRefreshMaterializedViewRuleCoverage(t *testing.T) {
	t.Run("basic_refresh_concurrently_warn", func(t *testing.T) {
		result, err := Audit(context.Background(), Request{
			SQL:     "REFRESH MATERIALIZED VIEW mv_stats;",
			Dialect: DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Unsupported) != 0 {
			t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
		}
		if len(result.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result.Statements))
		}
		found := false
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.refresh_materialized_view.concurrently.warn" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected concurrently.warn, got %#v", result.Statements[0].Findings)
		}
	})

	t.Run("with_no_data_both_rules", func(t *testing.T) {
		result, err := Audit(context.Background(), Request{
			SQL:     "REFRESH MATERIALIZED VIEW mv_stats WITH NO DATA;",
			Dialect: DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Unsupported) != 0 {
			t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
		}
		if len(result.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result.Statements))
		}
		var foundConcurrent, foundNoData bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.refresh_materialized_view.concurrently.warn" {
				foundConcurrent = true
			}
			if f.RuleID == "ddl.pg.refresh_materialized_view.no_data.notice" {
				foundNoData = true
			}
		}
		if !foundConcurrent {
			t.Fatalf("expected concurrently.warn, got %#v", result.Statements[0].Findings)
		}
		if !foundNoData {
			t.Fatalf("expected no_data.notice, got %#v", result.Statements[0].Findings)
		}
	})
}

func TestAuditPostgreSQLAlterTableUnsupportedActionRuleCoverage(t *testing.T) {
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
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectPostgreSQL,
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

func TestAuditPostgreSQLAlterTableLoggedStateRuleCoverage(t *testing.T) {
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
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectPostgreSQL,
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
					if f.Level != "notice" {
						t.Errorf("expected level notice, got %q", f.Level)
					}
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule %q, got %#v", tt.wantRuleID, result.Statements[0].Findings)
			}
		})
	}
}

func TestAuditPostgreSQLTypeLifecycleRuleCoverage(t *testing.T) {
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
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectPostgreSQL,
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

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
			}
			for _, f := range result.Statements[0].Findings {
				if _, expected := wantRuleIDs[f.RuleID]; expected {
					wantRuleIDs[f.RuleID] = true
				}
			}
			for ruleID, found := range wantRuleIDs {
				if !found {
					t.Fatalf("expected finding with rule %q, got %#v", ruleID, result.Statements[0].Findings)
				}
			}
		})
	}
}

func TestAuditPostgreSQLCompositeTypeLifecycleRuleCoverage(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectPostgreSQL,
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

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
			}
			for _, f := range result.Statements[0].Findings {
				if _, expected := wantRuleIDs[f.RuleID]; expected {
					wantRuleIDs[f.RuleID] = true
				}
			}
			for ruleID, found := range wantRuleIDs {
				if !found {
					t.Fatalf("expected finding with rule %q, got %#v", ruleID, result.Statements[0].Findings)
				}
			}
		})
	}
}

func TestAuditPostgreSQLDomainLifecycleRuleCoverage(t *testing.T) {
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
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectPostgreSQL,
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

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
			}
			for _, f := range result.Statements[0].Findings {
				if _, expected := wantRuleIDs[f.RuleID]; expected {
					wantRuleIDs[f.RuleID] = true
				}
			}
			for ruleID, found := range wantRuleIDs {
				if !found {
					t.Fatalf("expected finding with rule %q, got %#v", ruleID, result.Statements[0].Findings)
				}
			}
		})
	}
}

func TestAuditPostgreSQLTablePrivilegeDCLRuleCoverage(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "grant_select_notice",
			sql:         "GRANT SELECT ON TABLE users TO analyst;",
			wantRuleIDs: []string{"ddl.pg.grant.table_privilege.notice"},
		},
		{
			name:        "grant_all_duplicate",
			sql:         "GRANT ALL PRIVILEGES ON TABLE users TO analyst;",
			wantRuleIDs: []string{"ddl.pg.grant.table_privilege.notice", "ddl.pg.grant.table_privilege.all.warn"},
		},
		{
			name:        "revoke_all_cascade_duplicate",
			sql:         "REVOKE ALL PRIVILEGES ON TABLE users FROM analyst CASCADE;",
			wantRuleIDs: []string{"ddl.pg.revoke.table_privilege.notice", "ddl.pg.revoke.table_privilege.cascade.warn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectPostgreSQL,
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

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
			}
			for _, f := range result.Statements[0].Findings {
				if _, expected := wantRuleIDs[f.RuleID]; expected {
					wantRuleIDs[f.RuleID] = true
				}
			}
			for ruleID, found := range wantRuleIDs {
				if !found {
					t.Fatalf("expected finding with rule %q, got %#v", ruleID, result.Statements[0].Findings)
				}
			}
		})
	}
}

func TestAuditPostgreSQLPolicyLifecycleRuleCoverage(t *testing.T) {
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
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectPostgreSQL,
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

func TestAuditPostgreSQLTriggerLifecycleRuleCoverage(t *testing.T) {
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
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectPostgreSQL,
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

func TestAuditPostgreSQLExtensionLifecycleRuleCoverage(t *testing.T) {
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
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectPostgreSQL,
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

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
			}
			for _, f := range result.Statements[0].Findings {
				if _, expected := wantRuleIDs[f.RuleID]; expected {
					wantRuleIDs[f.RuleID] = true
				}
			}
			for ruleID, found := range wantRuleIDs {
				if !found {
					t.Fatalf("expected finding with rule %q, got %#v", ruleID, result.Statements[0].Findings)
				}
			}
		})
	}
}

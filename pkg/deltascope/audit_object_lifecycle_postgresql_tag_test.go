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
		{name: "create_text_search_configuration_notice", sql: "CREATE TEXT SEARCH CONFIGURATION english_copy ( COPY = pg_catalog.english )", wantRuleID: "ddl.pg.create_text_search_configuration.notice"},
		{name: "alter_text_search_configuration_notice", sql: "ALTER TEXT SEARCH CONFIGURATION english_copy RENAME TO english_copy_v2", wantRuleID: "ddl.pg.alter_text_search_configuration.notice"},
		{name: "drop_text_search_configuration_warn", sql: "DROP TEXT SEARCH CONFIGURATION english_copy", wantRuleID: "ddl.pg.drop_text_search_configuration.warn"},
		{name: "create_text_search_dictionary_notice", sql: "CREATE TEXT SEARCH DICTIONARY simple_dict (TEMPLATE = simple)", wantRuleID: "ddl.pg.create_text_search_dictionary.notice"},
		{name: "alter_text_search_dictionary_notice", sql: "ALTER TEXT SEARCH DICTIONARY simple_dict RENAME TO simple_dict_v2", wantRuleID: "ddl.pg.alter_text_search_dictionary.notice"},
		{name: "drop_text_search_dictionary_warn", sql: "DROP TEXT SEARCH DICTIONARY simple_dict", wantRuleID: "ddl.pg.drop_text_search_dictionary.warn"},
		{name: "create_text_search_parser_notice", sql: "CREATE TEXT SEARCH PARSER parser_name (START = start_func, GETTOKEN = token_func, END = end_func, LEXTYPES = lextype_func)", wantRuleID: "ddl.pg.create_text_search_parser.notice"},
		{name: "alter_text_search_parser_notice", sql: "ALTER TEXT SEARCH PARSER parser_name RENAME TO parser_name_v2", wantRuleID: "ddl.pg.alter_text_search_parser.notice"},
		{name: "drop_text_search_parser_warn", sql: "DROP TEXT SEARCH PARSER parser_name", wantRuleID: "ddl.pg.drop_text_search_parser.warn"},
		{name: "create_text_search_template_notice", sql: "CREATE TEXT SEARCH TEMPLATE template_name (LEXIZE = lexize_func)", wantRuleID: "ddl.pg.create_text_search_template.notice"},
		{name: "alter_text_search_template_notice", sql: "ALTER TEXT SEARCH TEMPLATE template_name RENAME TO template_name_v2", wantRuleID: "ddl.pg.alter_text_search_template.notice"},
		{name: "drop_text_search_template_warn", sql: "DROP TEXT SEARCH TEMPLATE template_name", wantRuleID: "ddl.pg.drop_text_search_template.warn"},
		{name: "create_transform_notice", sql: "CREATE TRANSFORM FOR jsonb LANGUAGE plpython3u (FROM SQL WITH FUNCTION jsonb_to_plpython(jsonb), TO SQL WITH FUNCTION plpython_to_jsonb(internal))", wantRuleID: "ddl.pg.create_transform.notice"},
		{name: "create_access_method_notice", sql: "CREATE ACCESS METHOD heap2 TYPE TABLE HANDLER heap_tableam_handler", wantRuleID: "ddl.pg.create_access_method.notice"},
		{name: "drop_transform_warn", sql: "DROP TRANSFORM FOR jsonb LANGUAGE plpython3u", wantRuleID: "ddl.pg.drop_transform.warn"},
		{name: "drop_access_method_warn", sql: "DROP ACCESS METHOD heap2", wantRuleID: "ddl.pg.drop_access_method.warn"},
		{name: "alter_large_object_owner_notice", sql: "ALTER LARGE OBJECT 12345 OWNER TO app_owner", wantRuleID: "ddl.pg.alter_large_object.owner.notice"},
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

func TestAuditPostgreSQLAlterTableStorageLayoutRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
		wantLevel  string
	}{
		{
			name:       "set_tablespace_notice",
			sql:        "ALTER TABLE users SET TABLESPACE fastspace;",
			wantRuleID: "ddl.pg.alter.set_tablespace.notice",
			wantLevel:  "notice",
		},
		{
			name:       "set_access_method_heap_warn",
			sql:        "ALTER TABLE users SET ACCESS METHOD heap;",
			wantRuleID: "ddl.pg.alter.set_access_method.warn",
			wantLevel:  "warning",
		},
		{
			name:       "set_access_method_default_warn",
			sql:        "ALTER TABLE users SET ACCESS METHOD DEFAULT;",
			wantRuleID: "ddl.pg.alter.set_access_method.warn",
			wantLevel:  "warning",
		},
		{
			name:       "enable_replica_trigger_notice",
			sql:        "ALTER TABLE users ENABLE REPLICA TRIGGER sync_trigger",
			wantRuleID: "ddl.pg.alter.enable_replica_trigger.notice",
			wantLevel:  "notice",
		},
		{
			name:       "enable_always_trigger_notice",
			sql:        "ALTER TABLE users ENABLE ALWAYS TRIGGER audit_trigger",
			wantRuleID: "ddl.pg.alter.enable_always_trigger.notice",
			wantLevel:  "notice",
		},
		{
			name:       "enable_rule_notice",
			sql:        "ALTER TABLE users ENABLE RULE route_rule",
			wantRuleID: "ddl.pg.alter.enable_rule.notice",
			wantLevel:  "notice",
		},
		{
			name:       "disable_rule_warn",
			sql:        "ALTER TABLE users DISABLE RULE route_rule",
			wantRuleID: "ddl.pg.alter.disable_rule.warn",
			wantLevel:  "warning",
		},
		{
			name:       "enable_replica_rule_notice",
			sql:        "ALTER TABLE users ENABLE REPLICA RULE route_rule",
			wantRuleID: "ddl.pg.alter.enable_replica_rule.notice",
			wantLevel:  "notice",
		},
		{
			name:       "enable_always_rule_notice",
			sql:        "ALTER TABLE users ENABLE ALWAYS RULE route_rule",
			wantRuleID: "ddl.pg.alter.enable_always_rule.notice",
			wantLevel:  "notice",
		},
		{
			name:       "set_reloptions_warn",
			sql:        "ALTER TABLE users SET (fillfactor = 70)",
			wantRuleID: "ddl.pg.alter.set_reloptions.warn",
			wantLevel:  "warning",
		},
		{
			name:       "reset_reloptions_notice",
			sql:        "ALTER TABLE users RESET (fillfactor)",
			wantRuleID: "ddl.pg.alter.reset_reloptions.notice",
			wantLevel:  "notice",
		},
		// Bounded residual: column attribute rules
		{
			name:       "set_column_statistics_notice",
			sql:        "ALTER TABLE users ALTER COLUMN email SET STATISTICS 100",
			wantRuleID: "ddl.pg.alter.set_column_statistics.notice",
			wantLevel:  "notice",
		},
		{
			name:       "set_column_options_notice",
			sql:        "ALTER TABLE users ALTER COLUMN email SET (n_distinct = -1)",
			wantRuleID: "ddl.pg.alter.set_column_options.notice",
			wantLevel:  "notice",
		},
		{
			name:       "reset_column_options_notice",
			sql:        "ALTER TABLE users ALTER COLUMN email RESET (n_distinct)",
			wantRuleID: "ddl.pg.alter.reset_column_options.notice",
			wantLevel:  "notice",
		},
		{
			name:       "set_column_storage_notice",
			sql:        "ALTER TABLE users ALTER COLUMN bio SET STORAGE EXTERNAL",
			wantRuleID: "ddl.pg.alter.set_column_storage.notice",
			wantLevel:  "notice",
		},
		{
			name:       "set_column_compression_notice",
			sql:        "ALTER TABLE users ALTER COLUMN bio SET COMPRESSION lz4",
			wantRuleID: "ddl.pg.alter.set_column_compression.notice",
			wantLevel:  "notice",
		},
		// Bounded residual: cluster/finalize rules
		{
			name:       "cluster_on_notice",
			sql:        "ALTER TABLE users CLUSTER ON users_email_idx",
			wantRuleID: "ddl.pg.alter.cluster_on.notice",
			wantLevel:  "notice",
		},
		{
			name:       "set_without_cluster_notice",
			sql:        "ALTER TABLE users SET WITHOUT CLUSTER",
			wantRuleID: "ddl.pg.alter.set_without_cluster.notice",
			wantLevel:  "notice",
		},
		{
			name:       "detach_partition_finalize_notice",
			sql:        "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04 FINALIZE",
			wantRuleID: "ddl.pg.alter.detach_partition_finalize.notice",
			wantLevel:  "notice",
		},
		// Table relationship rules
		{
			name:       "add_inherit_notice",
			sql:        "ALTER TABLE child_users INHERIT users",
			wantRuleID: "ddl.pg.alter.add_inherit.notice",
			wantLevel:  "notice",
		},
		{
			name:       "drop_inherit_notice",
			sql:        "ALTER TABLE child_users NO INHERIT users",
			wantRuleID: "ddl.pg.alter.drop_inherit.notice",
			wantLevel:  "notice",
		},
		{
			name:       "add_of_type_notice",
			sql:        "ALTER TABLE users OF user_type",
			wantRuleID: "ddl.pg.alter.add_of_type.notice",
			wantLevel:  "notice",
		},
		{
			name:       "drop_of_type_notice",
			sql:        "ALTER TABLE users NOT OF",
			wantRuleID: "ddl.pg.alter.drop_of_type.notice",
			wantLevel:  "notice",
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
					if f.Level != Level(tt.wantLevel) {
						t.Errorf("expected level %s, got %q", tt.wantLevel, f.Level)
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
		{
			name:        "alter_extension_add_member_notice",
			sql:         "ALTER EXTENSION pg_trgm ADD TABLE users;",
			wantRuleIDs: []string{"ddl.pg.alter_extension.add_member.notice"},
		},
		{
			name:        "alter_extension_drop_member_warn",
			sql:         "ALTER EXTENSION pg_trgm DROP TABLE users;",
			wantRuleIDs: []string{"ddl.pg.alter_extension.drop_member.warn"},
		},
		{
			name:        "create_publication_notice",
			sql:         "CREATE PUBLICATION pub_all FOR ALL TABLES",
			wantRuleIDs: []string{"ddl.pg.create_publication.notice"},
		},
		{
			name:        "alter_publication_notice",
			sql:         "ALTER PUBLICATION pub_all ADD TABLE users",
			wantRuleIDs: []string{"ddl.pg.alter_publication.notice"},
		},
		{
			name:        "drop_publication_warn",
			sql:         "DROP PUBLICATION pub_all",
			wantRuleIDs: []string{"ddl.pg.drop_publication.warn"},
		},
		{
			name:        "create_subscription_notice",
			sql:         "CREATE SUBSCRIPTION sub CONNECTION 'host=localhost' PUBLICATION pub_all",
			wantRuleIDs: []string{"ddl.pg.create_subscription.notice"},
		},
		{
			name:        "alter_subscription_notice",
			sql:         "ALTER SUBSCRIPTION sub ENABLE",
			wantRuleIDs: []string{"ddl.pg.alter_subscription.notice"},
		},
		{
			name:        "alter_subscription_disable_warn",
			sql:         "ALTER SUBSCRIPTION sub DISABLE",
			wantRuleIDs: []string{"ddl.pg.alter_subscription.disable.warn"},
		},
		{
			name:        "drop_subscription_warn",
			sql:         "DROP SUBSCRIPTION sub",
			wantRuleIDs: []string{"ddl.pg.drop_subscription.warn"},
		},
		// Foreign table lifecycle.
		{
			name:        "create_foreign_table_notice",
			sql:         "CREATE FOREIGN TABLE ft_users (id bigint) SERVER srv OPTIONS (table_name 'users')",
			wantRuleIDs: []string{"ddl.pg.create_foreign_table.notice"},
		},
		{
			name:        "alter_foreign_table_notice",
			sql:         "ALTER FOREIGN TABLE ft_users OPTIONS (SET table_name 'users_v2')",
			wantRuleIDs: []string{"ddl.pg.alter_foreign_table.notice"},
		},
		{
			name:        "drop_foreign_table_warn",
			sql:         "DROP FOREIGN TABLE ft_users",
			wantRuleIDs: []string{"ddl.pg.drop_foreign_table.warn"},
		},
		// Foreign server lifecycle.
		{
			name:        "create_foreign_server_notice",
			sql:         "CREATE SERVER srv FOREIGN DATA WRAPPER postgres_fdw",
			wantRuleIDs: []string{"ddl.pg.create_foreign_server.notice"},
		},
		{
			name:        "alter_foreign_server_notice",
			sql:         "ALTER SERVER srv OPTIONS (SET host 'db')",
			wantRuleIDs: []string{"ddl.pg.alter_foreign_server.notice"},
		},
		{
			name:        "drop_foreign_server_warn",
			sql:         "DROP SERVER srv",
			wantRuleIDs: []string{"ddl.pg.drop_foreign_server.warn"},
		},
		// User mapping lifecycle.
		{
			name:        "create_user_mapping_notice",
			sql:         "CREATE USER MAPPING FOR app SERVER srv OPTIONS (user 'app')",
			wantRuleIDs: []string{"ddl.pg.create_user_mapping.notice"},
		},
		{
			name:        "alter_user_mapping_notice",
			sql:         "ALTER USER MAPPING FOR app SERVER srv OPTIONS (SET user 'app2')",
			wantRuleIDs: []string{"ddl.pg.alter_user_mapping.notice"},
		},
		{
			name:        "drop_user_mapping_warn",
			sql:         "DROP USER MAPPING FOR app SERVER srv",
			wantRuleIDs: []string{"ddl.pg.drop_user_mapping.warn"},
		},
		// Foreign data wrapper lifecycle.
		{
			name:        "create_foreign_data_wrapper_notice",
			sql:         "CREATE FOREIGN DATA WRAPPER fdw HANDLER fdw_handler",
			wantRuleIDs: []string{"ddl.pg.create_foreign_data_wrapper.notice"},
		},
		{
			name:        "alter_foreign_data_wrapper_notice",
			sql:         "ALTER FOREIGN DATA WRAPPER fdw OPTIONS (SET key 'value')",
			wantRuleIDs: []string{"ddl.pg.alter_foreign_data_wrapper.notice"},
		},
		{
			name:        "drop_foreign_data_wrapper_warn",
			sql:         "DROP FOREIGN DATA WRAPPER fdw",
			wantRuleIDs: []string{"ddl.pg.drop_foreign_data_wrapper.warn"},
		},
		// PostgreSQL annotation lifecycle (PG-only).
		{
			name:        "comment_on_notice",
			sql:         "COMMENT ON TABLE users IS 'user accounts'",
			wantRuleIDs: []string{"ddl.pg.comment_on.notice"},
		},
		{
			name:        "comment_on_remove_notice",
			sql:         "COMMENT ON TABLE users IS NULL",
			wantRuleIDs: []string{"ddl.pg.comment_on.remove.notice"},
		},
		{
			name:        "security_label_notice",
			sql:         "SECURITY LABEL FOR selinux ON TABLE users IS 'system_u:object_r:sepgsql_table_t:s0'",
			wantRuleIDs: []string{"ddl.pg.security_label.notice"},
		},
		{
			name:        "security_label_remove_notice",
			sql:         "SECURITY LABEL FOR selinux ON TABLE users IS NULL",
			wantRuleIDs: []string{"ddl.pg.security_label.remove.notice"},
		},
		// PostgreSQL event trigger lifecycle (PG-only).
		{
			name:        "create_event_trigger_notice",
			sql:         "CREATE EVENT TRIGGER trg_ddl ON ddl_command_end EXECUTE FUNCTION log_ddl()",
			wantRuleIDs: []string{"ddl.pg.create_event_trigger.notice"},
		},
		{
			name:        "alter_event_trigger_notice_enable",
			sql:         "ALTER EVENT TRIGGER trg_ddl ENABLE",
			wantRuleIDs: []string{"ddl.pg.alter_event_trigger.notice"},
		},
		{
			name:        "alter_event_trigger_notice_rename",
			sql:         "ALTER EVENT TRIGGER trg_ddl RENAME TO trg_ddl_v2",
			wantRuleIDs: []string{"ddl.pg.alter_event_trigger.notice"},
		},
		{
			name:        "alter_event_trigger_disable_warn",
			sql:         "ALTER EVENT TRIGGER trg_ddl DISABLE",
			wantRuleIDs: []string{"ddl.pg.alter_event_trigger.disable.warn"},
		},
		{
			name:        "drop_event_trigger_warn",
			sql:         "DROP EVENT TRIGGER trg_ddl",
			wantRuleIDs: []string{"ddl.pg.drop_event_trigger.warn"},
		},
		// PostgreSQL rewrite rule lifecycle (PG-only).
		{
			name:        "create_rule_notice",
			sql:         "CREATE RULE users_insert AS ON INSERT TO users DO NOTHING",
			wantRuleIDs: []string{"ddl.pg.create_rule.notice"},
		},
		{
			name:        "alter_rule_notice_rename",
			sql:         "ALTER RULE users_insert ON users RENAME TO users_insert_ignore",
			wantRuleIDs: []string{"ddl.pg.alter_rule.notice"},
		},
		{
			name:        "drop_rule_warn",
			sql:         "DROP RULE users_insert_ignore ON users",
			wantRuleIDs: []string{"ddl.pg.drop_rule.warn"},
		},
		// Collation lifecycle
		{
			name:        "create_collation_notice",
			sql:         "CREATE COLLATION app_collation (provider = libc, locale = 'C')",
			wantRuleIDs: []string{"ddl.pg.create_collation.notice"},
		},
		{
			name:        "alter_collation_notice_rename",
			sql:         "ALTER COLLATION app_collation RENAME TO app_collation_v2",
			wantRuleIDs: []string{"ddl.pg.alter_collation.notice"},
		},
		{
			name:        "drop_collation_warn",
			sql:         "DROP COLLATION app_collation",
			wantRuleIDs: []string{"ddl.pg.drop_collation.warn"},
		},
		// Statistics lifecycle
		{
			name:        "create_statistics_notice",
			sql:         "CREATE STATISTICS users_stats ON email, status FROM users",
			wantRuleIDs: []string{"ddl.pg.create_statistics.notice"},
		},
		{
			name:        "alter_statistics_notice_rename",
			sql:         "ALTER STATISTICS users_stats RENAME TO users_stats_v2",
			wantRuleIDs: []string{"ddl.pg.alter_statistics.notice"},
		},
		{
			name:        "drop_statistics_warn",
			sql:         "DROP STATISTICS users_stats",
			wantRuleIDs: []string{"ddl.pg.drop_statistics.warn"},
		},
		// Aggregate lifecycle
		{
			name:        "create_aggregate_notice",
			sql:         "CREATE AGGREGATE sum2(integer) (SFUNC = int4pl, STYPE = integer)",
			wantRuleIDs: []string{"ddl.pg.create_aggregate.notice"},
		},
		{
			name:        "alter_aggregate_notice",
			sql:         "ALTER AGGREGATE sum2(integer) OWNER TO app_owner",
			wantRuleIDs: []string{"ddl.pg.alter_aggregate.notice"},
		},
		{
			name:        "drop_aggregate_warn",
			sql:         "DROP AGGREGATE sum2(integer)",
			wantRuleIDs: []string{"ddl.pg.drop_aggregate.warn"},
		},
		// Operator lifecycle
		{
			name:        "create_operator_notice",
			sql:         "CREATE OPERATOR === (LEFTARG = integer, RIGHTARG = integer, PROCEDURE = int4eq)",
			wantRuleIDs: []string{"ddl.pg.create_operator.notice"},
		},
		{
			name:        "alter_operator_notice",
			sql:         "ALTER OPERATOR === (integer, integer) OWNER TO app_owner",
			wantRuleIDs: []string{"ddl.pg.alter_operator.notice"},
		},
		{
			name:        "drop_operator_warn",
			sql:         "DROP OPERATOR === (integer, integer)",
			wantRuleIDs: []string{"ddl.pg.drop_operator.warn"},
		},
		// Conversion lifecycle
		{
			name:        "create_conversion_notice",
			sql:         "CREATE CONVERSION conv FOR 'UTF8' TO 'LATIN1' FROM utf8_to_latin1",
			wantRuleIDs: []string{"ddl.pg.create_conversion.notice"},
		},
		{
			name:        "alter_conversion_notice",
			sql:         "ALTER CONVERSION conv OWNER TO app_owner",
			wantRuleIDs: []string{"ddl.pg.alter_conversion.notice"},
		},
		{
			name:        "drop_conversion_warn",
			sql:         "DROP CONVERSION conv",
			wantRuleIDs: []string{"ddl.pg.drop_conversion.warn"},
		},
		// Operator family lifecycle
		{
			name:        "create_operator_family_notice",
			sql:         "CREATE OPERATOR FAMILY int4_ops_family USING btree",
			wantRuleIDs: []string{"ddl.pg.create_operator_family.notice"},
		},
		{
			name:        "alter_operator_family_notice",
			sql:         "ALTER OPERATOR FAMILY int4_ops_family USING btree RENAME TO int4_ops_family_v2",
			wantRuleIDs: []string{"ddl.pg.alter_operator_family.notice"},
		},
		{
			name:        "drop_operator_family_warn",
			sql:         "DROP OPERATOR FAMILY int4_ops_family USING btree",
			wantRuleIDs: []string{"ddl.pg.drop_operator_family.warn"},
		},
		// Operator class lifecycle
		{
			name:        "create_operator_class_notice",
			sql:         "CREATE OPERATOR CLASS int4_ops_class DEFAULT FOR TYPE int4 USING btree FAMILY int4_ops_family AS OPERATOR 1 < (int4, int4)",
			wantRuleIDs: []string{"ddl.pg.create_operator_class.notice"},
		},
		{
			name:        "alter_operator_class_notice",
			sql:         "ALTER OPERATOR CLASS int4_ops_class USING btree RENAME TO int4_ops_class_v2",
			wantRuleIDs: []string{"ddl.pg.alter_operator_class.notice"},
		},
		{
			name:        "drop_operator_class_warn",
			sql:         "DROP OPERATOR CLASS int4_ops_class USING btree",
			wantRuleIDs: []string{"ddl.pg.drop_operator_class.warn"},
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

func TestAuditPostgreSQLAlterObjectLifecycleRuleCoverage(t *testing.T) {
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

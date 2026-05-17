//go:build postgresql

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerAuditPostgreSQLAlterObjectLifecycleRuleCoverage(t *testing.T) {
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
		{
			name:       "alter_type_add_attribute_notice",
			sql:        "ALTER TYPE address ADD ATTRIBUTE country text;",
			wantRuleID: "ddl.pg.alter_type.add_attribute.notice",
		},
		{
			name:       "alter_type_drop_attribute_warn",
			sql:        "ALTER TYPE address DROP ATTRIBUTE city;",
			wantRuleID: "ddl.pg.alter_type.drop_attribute.warn",
		},
		{
			name:       "alter_type_alter_attribute_type_warn",
			sql:        "ALTER TYPE address ALTER ATTRIBUTE street TYPE varchar(255);",
			wantRuleID: "ddl.pg.alter_type.alter_attribute_type.warn",
		},
		{
			name:       "alter_type_rename_attribute_notice",
			sql:        "ALTER TYPE address RENAME ATTRIBUTE street TO line1;",
			wantRuleID: "ddl.pg.alter_type.rename_attribute.notice",
		},
		{
			name:       "alter_extension_add_member_notice",
			sql:        "ALTER EXTENSION pg_trgm ADD TABLE users;",
			wantRuleID: "ddl.pg.alter_extension.add_member.notice",
		},
		{
			name:       "alter_extension_drop_member_warn",
			sql:        "ALTER EXTENSION pg_trgm DROP TABLE users;",
			wantRuleID: "ddl.pg.alter_extension.drop_member.warn",
		},
		{
			name:       "create_publication_notice",
			sql:        "CREATE PUBLICATION pub_all FOR ALL TABLES",
			wantRuleID: "ddl.pg.create_publication.notice",
		},
		{
			name:       "alter_publication_notice",
			sql:        "ALTER PUBLICATION pub_all ADD TABLE users",
			wantRuleID: "ddl.pg.alter_publication.notice",
		},
		{
			name:       "drop_publication_warn",
			sql:        "DROP PUBLICATION pub_all",
			wantRuleID: "ddl.pg.drop_publication.warn",
		},
		{
			name:       "create_subscription_notice",
			sql:        "CREATE SUBSCRIPTION sub CONNECTION 'host=localhost' PUBLICATION pub_all",
			wantRuleID: "ddl.pg.create_subscription.notice",
		},
		{
			name:       "alter_subscription_notice",
			sql:        "ALTER SUBSCRIPTION sub ENABLE",
			wantRuleID: "ddl.pg.alter_subscription.notice",
		},
		{
			name:       "alter_subscription_disable_warn",
			sql:        "ALTER SUBSCRIPTION sub DISABLE",
			wantRuleID: "ddl.pg.alter_subscription.disable.warn",
		},
		{
			name:       "drop_subscription_warn",
			sql:        "DROP SUBSCRIPTION sub",
			wantRuleID: "ddl.pg.drop_subscription.warn",
		},
		// Foreign table lifecycle.
		{
			name:       "create_foreign_table_notice",
			sql:        "CREATE FOREIGN TABLE ft_users (id bigint) SERVER srv OPTIONS (table_name 'users')",
			wantRuleID: "ddl.pg.create_foreign_table.notice",
		},
		{
			name:       "alter_foreign_table_notice",
			sql:        "ALTER FOREIGN TABLE ft_users OPTIONS (SET table_name 'users_v2')",
			wantRuleID: "ddl.pg.alter_foreign_table.notice",
		},
		{
			name:       "drop_foreign_table_warn",
			sql:        "DROP FOREIGN TABLE ft_users",
			wantRuleID: "ddl.pg.drop_foreign_table.warn",
		},
		// Foreign server lifecycle.
		{
			name:       "create_foreign_server_notice",
			sql:        "CREATE SERVER srv FOREIGN DATA WRAPPER postgres_fdw",
			wantRuleID: "ddl.pg.create_foreign_server.notice",
		},
		{
			name:       "alter_foreign_server_notice",
			sql:        "ALTER SERVER srv OPTIONS (SET host 'db')",
			wantRuleID: "ddl.pg.alter_foreign_server.notice",
		},
		{
			name:       "drop_foreign_server_warn",
			sql:        "DROP SERVER srv",
			wantRuleID: "ddl.pg.drop_foreign_server.warn",
		},
		// User mapping lifecycle.
		{
			name:       "create_user_mapping_notice",
			sql:        "CREATE USER MAPPING FOR app SERVER srv OPTIONS (user 'app')",
			wantRuleID: "ddl.pg.create_user_mapping.notice",
		},
		{
			name:       "alter_user_mapping_notice",
			sql:        "ALTER USER MAPPING FOR app SERVER srv OPTIONS (SET user 'app2')",
			wantRuleID: "ddl.pg.alter_user_mapping.notice",
		},
		{
			name:       "drop_user_mapping_warn",
			sql:        "DROP USER MAPPING FOR app SERVER srv",
			wantRuleID: "ddl.pg.drop_user_mapping.warn",
		},
		// Foreign data wrapper lifecycle.
		{
			name:       "create_foreign_data_wrapper_notice",
			sql:        "CREATE FOREIGN DATA WRAPPER fdw HANDLER fdw_handler",
			wantRuleID: "ddl.pg.create_foreign_data_wrapper.notice",
		},
		{
			name:       "alter_foreign_data_wrapper_notice",
			sql:        "ALTER FOREIGN DATA WRAPPER fdw OPTIONS (SET key 'value')",
			wantRuleID: "ddl.pg.alter_foreign_data_wrapper.notice",
		},
		{
			name:       "drop_foreign_data_wrapper_warn",
			sql:        "DROP FOREIGN DATA WRAPPER fdw",
			wantRuleID: "ddl.pg.drop_foreign_data_wrapper.warn",
		},
		// PostgreSQL annotation lifecycle (PG-only).
		{
			name:       "comment_on_notice",
			sql:        "COMMENT ON TABLE users IS 'user accounts'",
			wantRuleID: "ddl.pg.comment_on.notice",
		},
		{
			name:       "comment_on_remove_notice",
			sql:        "COMMENT ON TABLE users IS NULL",
			wantRuleID: "ddl.pg.comment_on.remove.notice",
		},
		{
			name:       "security_label_notice",
			sql:        "SECURITY LABEL FOR selinux ON TABLE users IS 'system_u:object_r:sepgsql_table_t:s0'",
			wantRuleID: "ddl.pg.security_label.notice",
		},
		{
			name:       "security_label_remove_notice",
			sql:        "SECURITY LABEL FOR selinux ON TABLE users IS NULL",
			wantRuleID: "ddl.pg.security_label.remove.notice",
		},
		// PostgreSQL event trigger lifecycle (PG-only).
		{
			name:       "create_event_trigger_notice",
			sql:        "CREATE EVENT TRIGGER trg_ddl ON ddl_command_end EXECUTE FUNCTION log_ddl()",
			wantRuleID: "ddl.pg.create_event_trigger.notice",
		},
		{
			name:       "alter_event_trigger_notice_enable",
			sql:        "ALTER EVENT TRIGGER trg_ddl ENABLE",
			wantRuleID: "ddl.pg.alter_event_trigger.notice",
		},
		{
			name:       "alter_event_trigger_notice_rename",
			sql:        "ALTER EVENT TRIGGER trg_ddl RENAME TO trg_ddl_v2",
			wantRuleID: "ddl.pg.alter_event_trigger.notice",
		},
		{
			name:       "alter_event_trigger_disable_warn",
			sql:        "ALTER EVENT TRIGGER trg_ddl DISABLE",
			wantRuleID: "ddl.pg.alter_event_trigger.disable.warn",
		},
		{
			name:       "drop_event_trigger_warn",
			sql:        "DROP EVENT TRIGGER trg_ddl",
			wantRuleID: "ddl.pg.drop_event_trigger.warn",
		},
		// PostgreSQL rewrite rule lifecycle (PG-only).
		{
			name:       "create_rule_notice",
			sql:        "CREATE RULE users_insert AS ON INSERT TO users DO NOTHING",
			wantRuleID: "ddl.pg.create_rule.notice",
		},
		{
			name:       "alter_rule_notice_rename",
			sql:        "ALTER RULE users_insert ON users RENAME TO users_insert_ignore",
			wantRuleID: "ddl.pg.alter_rule.notice",
		},
		{
			name:       "drop_rule_warn",
			sql:        "DROP RULE users_insert_ignore ON users",
			wantRuleID: "ddl.pg.drop_rule.warn",
		},
		// Collation lifecycle
		{
			name:       "create_collation_notice",
			sql:        "CREATE COLLATION app_collation (provider = libc, locale = 'C')",
			wantRuleID: "ddl.pg.create_collation.notice",
		},
		{
			name:       "alter_collation_notice_rename",
			sql:        "ALTER COLLATION app_collation RENAME TO app_collation_v2",
			wantRuleID: "ddl.pg.alter_collation.notice",
		},
		{
			name:       "drop_collation_warn",
			sql:        "DROP COLLATION app_collation",
			wantRuleID: "ddl.pg.drop_collation.warn",
		},
		// Statistics lifecycle
		{name: "create_statistics_notice", sql: "CREATE STATISTICS users_stats ON email, status FROM users", wantRuleID: "ddl.pg.create_statistics.notice"},
		{name: "alter_statistics_notice_rename", sql: "ALTER STATISTICS users_stats RENAME TO users_stats_v2", wantRuleID: "ddl.pg.alter_statistics.notice"},
		{name: "drop_statistics_warn", sql: "DROP STATISTICS users_stats", wantRuleID: "ddl.pg.drop_statistics.warn"},
		// Aggregate lifecycle
		{name: "create_aggregate_notice", sql: "CREATE AGGREGATE sum2(integer) (SFUNC = int4pl, STYPE = integer)", wantRuleID: "ddl.pg.create_aggregate.notice"},
		{name: "alter_aggregate_notice", sql: "ALTER AGGREGATE sum2(integer) OWNER TO app_owner", wantRuleID: "ddl.pg.alter_aggregate.notice"},
		{name: "drop_aggregate_warn", sql: "DROP AGGREGATE sum2(integer)", wantRuleID: "ddl.pg.drop_aggregate.warn"},
		// Operator lifecycle
		{name: "create_operator_notice", sql: "CREATE OPERATOR === (LEFTARG = integer, RIGHTARG = integer, PROCEDURE = int4eq)", wantRuleID: "ddl.pg.create_operator.notice"},
		{name: "alter_operator_notice", sql: "ALTER OPERATOR === (integer, integer) OWNER TO app_owner", wantRuleID: "ddl.pg.alter_operator.notice"},
		{name: "drop_operator_warn", sql: "DROP OPERATOR === (integer, integer)", wantRuleID: "ddl.pg.drop_operator.warn"},
		// Conversion lifecycle
		{name: "create_conversion_notice", sql: "CREATE CONVERSION conv FOR 'UTF8' TO 'LATIN1' FROM utf8_to_latin1", wantRuleID: "ddl.pg.create_conversion.notice"},
		{name: "alter_conversion_notice", sql: "ALTER CONVERSION conv OWNER TO app_owner", wantRuleID: "ddl.pg.alter_conversion.notice"},
		{name: "drop_conversion_warn", sql: "DROP CONVERSION conv", wantRuleID: "ddl.pg.drop_conversion.warn"},
		// Operator family lifecycle
		{name: "create_operator_family_notice", sql: "CREATE OPERATOR FAMILY int4_ops_family USING btree", wantRuleID: "ddl.pg.create_operator_family.notice"},
		{name: "alter_operator_family_notice", sql: "ALTER OPERATOR FAMILY int4_ops_family USING btree RENAME TO int4_ops_family_v2", wantRuleID: "ddl.pg.alter_operator_family.notice"},
		{name: "drop_operator_family_warn", sql: "DROP OPERATOR FAMILY int4_ops_family USING btree", wantRuleID: "ddl.pg.drop_operator_family.warn"},
		// Operator class lifecycle
		{name: "create_operator_class_notice", sql: "CREATE OPERATOR CLASS int4_ops_class DEFAULT FOR TYPE int4 USING btree FAMILY int4_ops_family AS OPERATOR 1 < (int4, int4)", wantRuleID: "ddl.pg.create_operator_class.notice"},
		{name: "alter_operator_class_notice", sql: "ALTER OPERATOR CLASS int4_ops_class USING btree RENAME TO int4_ops_class_v2", wantRuleID: "ddl.pg.alter_operator_class.notice"},
		{name: "drop_operator_class_warn", sql: "DROP OPERATOR CLASS int4_ops_class USING btree", wantRuleID: "ddl.pg.drop_operator_class.warn"},
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
			handler, err := NewHandler("", "test-build")
			if err != nil {
				t.Fatalf("new handler: %v", err)
			}

			body := `{"sql":"` + tt.sql + `","dialect":"postgresql"}`
			req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			statements, ok := payload["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", payload["statements"])
			}
			statement, _ := statements[0].(map[string]any)
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) < 1 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}

			found := false
			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if !ok {
					continue
				}
				ruleID, _ := finding["rule_id"].(string)
				if ruleID == tt.wantRuleID {
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

//go:build postgresql

package deltascope

import (
	"context"
	"strings"
	"testing"
)

func TestAuditDefaultPolicyDialectHygienePostgreSQLExcludesMySQLFamilyRules(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE pg_smoke (id bigint primary key);",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	mysqlOnly := []string{
		"ddl.table.engine.allowlist",
		"ddl.table.charset.allowlist",
		"ddl.table.row_format.allowlist",
		"ddl.table.auto_increment_init.value",
		"ddl.primary_key.unsigned.require",
		"ddl.primary_key.auto_increment.require",
		"ddl.primary_key.not_null.require",
		"ddl.database.create.notice",
		"ddl.database.drop.warn",
	}
	for _, stmt := range result.Statements {
		for _, finding := range stmt.Findings {
			for _, id := range mysqlOnly {
				if finding.RuleID == id {
					t.Errorf("PG default audit should not emit MySQL-only rule %q", id)
				}
			}
			combined := strings.ToUpper(finding.Message + " " + finding.Suggestion)
			for _, pattern := range []string{"UNSIGNED", "AUTO_INCREMENT", "ON UPDATE CURRENT_TIMESTAMP"} {
				if strings.Contains(combined, pattern) {
					t.Errorf("PG default audit should not contain MySQL-specific text %q", pattern)
				}
			}
		}
	}
	for _, finding := range result.GlobalFindings {
		for _, id := range mysqlOnly {
			if finding.RuleID == id {
				t.Errorf("PG default audit should not emit MySQL-only rule %q in global findings", id)
			}
		}
	}
}

func TestAuditPostgreSQLReturnsSourceLocationsForMultiStatementSQL(t *testing.T) {
	sql := `create table ok_users (
  id bigint primary key
);

delete from users;`

	result, err := Audit(context.Background(), Request{
		SQL:     sql,
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) < 2 {
		t.Fatalf("expected at least 2 statements, got %d", len(result.Statements))
	}

	deleteStmt := result.Statements[1]
	var whereFinding *Finding
	for i := range deleteStmt.Findings {
		if deleteStmt.Findings[i].RuleID == "dml.where.require" {
			whereFinding = &deleteStmt.Findings[i]
			break
		}
	}
	if whereFinding == nil {
		t.Fatal("expected dml.where.require finding for 'delete from users'")
	}
	if whereFinding.Location == nil {
		t.Fatal("dml.where.require finding Location is nil, expected {Line:5,Column:1}")
	}
	if whereFinding.Location.Line != 5 {
		t.Errorf("finding Location.Line=%d, want 5", whereFinding.Location.Line)
	}
	if whereFinding.Location.Column != 1 {
		t.Errorf("finding Location.Column=%d, want 1", whereFinding.Location.Column)
	}
}

func TestAuditPostgreSQLAdvancedIndexFormsAreSupportedAndCovered(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE INDEX idx_users_active_email ON users (email) WHERE active = true",
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
	if result.Statements[0].Kind != "ddl" {
		t.Fatalf("expected ddl kind, got %q", result.Statements[0].Kind)
	}

	found := false
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_index.concurrently.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected concurrently.require finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLSemanticMetadataParity(t *testing.T) {
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
				"operation":             "create_index",
				"index":                 "idx_users_active_email",
				"table":                 "users",
				"concurrently":          false,
				"index_kind":            "secondary",
				"access_method":         "gin",
				"column_count":          1,
				"included_column_count": 1,
				"has_predicate":         true,
				"has_expression_keys":   false,
				"expression_count":      0,
			},
			forbidden: []string{"active = true", "WHERE active", "predicate_sql", "expression_sql"},
		},
		{
			name:   "add_column_non_null_default_metadata",
			sql:    "ALTER TABLE users ADD COLUMN created_at timestamptz NOT NULL DEFAULT now()",
			ruleID: "ddl.pg.alter.add_column.non_null_default.rewrite.warn",
			wantMetadata: map[string]any{
				"action":       "add_column",
				"column":       "created_at",
				"table":        "users",
				"not_null":     true,
				"has_default":  true,
				"default_kind": "function_call",
			},
			forbidden: []string{"now()", "now", "default_sql", "default_expr"},
		},
		{
			name:   "add_column_non_null_no_default_metadata",
			sql:    "ALTER TABLE users ADD COLUMN status text NOT NULL",
			ruleID: "ddl.pg.alter.add_column.non_null_no_default.warn",
			wantMetadata: map[string]any{
				"action":      "add_column",
				"column":      "status",
				"table":       "users",
				"not_null":    true,
				"has_default": false,
			},
		},
		{
			name:   "set_data_type_using_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN score TYPE bigint USING score::bigint",
			ruleID: "ddl.pg.alter.set_data_type.rewrite.warn",
			wantMetadata: map[string]any{
				"action":    "set_data_type",
				"column":    "score",
				"table":     "users",
				"has_using": true,
			},
			forbidden: []string{"score::bigint", "USING score", "using_sql", "using_expr"},
		},
		{
			name:   "set_tablespace_metadata",
			sql:    "ALTER TABLE users SET TABLESPACE fastspace",
			ruleID: "ddl.pg.alter.set_tablespace.notice",
			wantMetadata: map[string]any{
				"operation":  "alter_table",
				"action":     "set_tablespace",
				"table":      "users",
				"tablespace": "fastspace",
			},
			forbidden: []string{"fillfactor", "reloptions", "cluster", "tablespace_sql", "access_method_sql"},
		},
		{
			name:   "set_access_method_heap_metadata",
			sql:    "ALTER TABLE users SET ACCESS METHOD heap",
			ruleID: "ddl.pg.alter.set_access_method.warn",
			wantMetadata: map[string]any{
				"operation":     "alter_table",
				"action":        "set_access_method",
				"table":         "users",
				"access_method": "heap",
				"is_default":    "false",
			},
			forbidden: []string{"fillfactor", "reloptions", "cluster", "tablespace_sql", "access_method_sql"},
		},
		{
			name:   "set_access_method_default_metadata",
			sql:    "ALTER TABLE users SET ACCESS METHOD DEFAULT",
			ruleID: "ddl.pg.alter.set_access_method.warn",
			wantMetadata: map[string]any{
				"operation":     "alter_table",
				"action":        "set_access_method",
				"table":         "users",
				"access_method": "default",
				"is_default":    "true",
			},
			forbidden: []string{"fillfactor", "reloptions", "cluster", "tablespace_sql", "access_method_sql"},
		},
		{
			name:   "enable_replica_trigger_metadata",
			sql:    "ALTER TABLE users ENABLE REPLICA TRIGGER sync_trigger",
			ruleID: "ddl.pg.alter.enable_replica_trigger.notice",
			wantMetadata: map[string]any{
				"operation":    "alter_table",
				"action":       "enable_replica_trigger",
				"table":        "users",
				"trigger":      "sync_trigger",
				"trigger_mode": "replica",
			},
		},
		{
			name:   "enable_always_trigger_metadata",
			sql:    "ALTER TABLE users ENABLE ALWAYS TRIGGER audit_trigger",
			ruleID: "ddl.pg.alter.enable_always_trigger.notice",
			wantMetadata: map[string]any{
				"operation":    "alter_table",
				"action":       "enable_always_trigger",
				"table":        "users",
				"trigger":      "audit_trigger",
				"trigger_mode": "always",
			},
		},
		{
			name:   "enable_rule_metadata",
			sql:    "ALTER TABLE users ENABLE RULE route_rule",
			ruleID: "ddl.pg.alter.enable_rule.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table",
				"action":    "enable_rule",
				"table":     "users",
				"rule":      "route_rule",
			},
		},
		{
			name:   "disable_rule_metadata",
			sql:    "ALTER TABLE users DISABLE RULE route_rule",
			ruleID: "ddl.pg.alter.disable_rule.warn",
			wantMetadata: map[string]any{
				"operation": "alter_table",
				"action":    "disable_rule",
				"table":     "users",
				"rule":      "route_rule",
			},
		},
		{
			name:   "enable_replica_rule_metadata",
			sql:    "ALTER TABLE users ENABLE REPLICA RULE route_rule",
			ruleID: "ddl.pg.alter.enable_replica_rule.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table",
				"action":    "enable_replica_rule",
				"table":     "users",
				"rule":      "route_rule",
				"rule_mode": "replica",
			},
		},
		{
			name:   "enable_always_rule_metadata",
			sql:    "ALTER TABLE users ENABLE ALWAYS RULE route_rule",
			ruleID: "ddl.pg.alter.enable_always_rule.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table",
				"action":    "enable_always_rule",
				"table":     "users",
				"rule":      "route_rule",
				"rule_mode": "always",
			},
		},
		// Bounded residual: column attribute metadata
		{
			name:   "set_column_statistics_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN email SET STATISTICS 100",
			ruleID: "ddl.pg.alter.set_column_statistics.notice",
			wantMetadata: map[string]any{
				"operation":              "alter_table",
				"action":                 "set_statistics",
				"table":                  "users",
				"column":                 "email",
				"statistics_target_kind": "value",
				"has_statistics_target":  "true",
			},
			forbidden: []string{"100", "raw_sql", "n_distinct", "-1"},
		},
		{
			name:   "set_column_statistics_default_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN email SET STATISTICS DEFAULT",
			ruleID: "ddl.pg.alter.set_column_statistics.notice",
			wantMetadata: map[string]any{
				"operation":              "alter_table",
				"action":                 "set_statistics",
				"table":                  "users",
				"column":                 "email",
				"statistics_target_kind": "default",
				"has_statistics_target":  "true",
			},
			forbidden: []string{"raw_sql"},
		},
		{
			name:   "set_column_options_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN email SET (n_distinct = -1)",
			ruleID: "ddl.pg.alter.set_column_options.notice",
			wantMetadata: map[string]any{
				"operation":          "alter_table",
				"action":             "set_column_options",
				"table":              "users",
				"column":             "email",
				"option_count":       "1",
				"has_column_options": "true",
			},
			forbidden: []string{"n_distinct", "-1", "raw_sql", "option_names", "option_values"},
		},
		{
			name:   "reset_column_options_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN email RESET (n_distinct)",
			ruleID: "ddl.pg.alter.reset_column_options.notice",
			wantMetadata: map[string]any{
				"operation":          "alter_table",
				"action":             "reset_column_options",
				"table":              "users",
				"column":             "email",
				"reset_count":        "1",
				"has_column_options": "true",
			},
			forbidden: []string{"n_distinct", "-1", "raw_sql", "option_names", "option_values"},
		},
		{
			name:   "set_column_storage_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN bio SET STORAGE EXTERNAL",
			ruleID: "ddl.pg.alter.set_column_storage.notice",
			wantMetadata: map[string]any{
				"operation":           "alter_table",
				"action":              "set_storage",
				"table":               "users",
				"column":              "bio",
				"storage_kind":        "external",
				"has_storage_setting": "true",
			},
			forbidden: []string{"raw_sql", "lz4", "pglz"},
		},
		{
			name:   "set_column_storage_default_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN bio SET STORAGE DEFAULT",
			ruleID: "ddl.pg.alter.set_column_storage.notice",
			wantMetadata: map[string]any{
				"operation":           "alter_table",
				"action":              "set_storage",
				"table":               "users",
				"column":              "bio",
				"storage_kind":        "default",
				"has_storage_setting": "true",
			},
			forbidden: []string{"raw_sql"},
		},
		{
			name:   "set_column_compression_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN bio SET COMPRESSION lz4",
			ruleID: "ddl.pg.alter.set_column_compression.notice",
			wantMetadata: map[string]any{
				"operation":       "alter_table",
				"action":          "set_compression",
				"table":           "users",
				"column":          "bio",
				"has_compression": "true",
			},
			forbidden: []string{"lz4", "pglz", "compression_kind", "raw_sql"},
		},
		// Bounded residual: cluster/finalize metadata
		{
			name:   "cluster_on_metadata",
			sql:    "ALTER TABLE users CLUSTER ON users_email_idx",
			ruleID: "ddl.pg.alter.cluster_on.notice",
			wantMetadata: map[string]any{
				"operation":         "alter_table",
				"action":            "cluster_on",
				"table":             "users",
				"index":             "users_email_idx",
				"has_cluster_index": "true",
			},
			forbidden: []string{"cluster_sql", "raw_sql"},
		},
		{
			name:   "set_without_cluster_metadata",
			sql:    "ALTER TABLE users SET WITHOUT CLUSTER",
			ruleID: "ddl.pg.alter.set_without_cluster.notice",
			wantMetadata: map[string]any{
				"operation":         "alter_table",
				"action":            "set_without_cluster",
				"table":             "users",
				"has_cluster_index": "false",
			},
			forbidden: []string{"cluster_sql", "raw_sql"},
		},
		{
			name:   "detach_partition_finalize_metadata",
			sql:    "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04 FINALIZE",
			ruleID: "ddl.pg.alter.detach_partition_finalize.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table",
				"action":    "detach_partition_finalize",
				"table":     "measurement",
				"partition": "measurement_y2026m04",
				"finalize":  "true",
			},
			forbidden: []string{"partition_bound", "raw_sql", "concurrently"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) == 0 {
				t.Fatal("expected at least one statement")
			}
			var found *Finding
			for i := range result.Statements[0].Findings {
				if result.Statements[0].Findings[i].RuleID == tt.ruleID {
					f := result.Statements[0].Findings[i]
					found = &f
					break
				}
			}
			if found == nil {
				t.Fatalf("expected finding for rule %q, got %#v", tt.ruleID, result.Statements[0].Findings)
			}
			if found.Metadata == nil {
				t.Fatalf("expected non-nil metadata for rule %q", tt.ruleID)
			}
			for key, want := range tt.wantMetadata {
				got, ok := found.Metadata[key]
				if !ok {
					t.Errorf("metadata[%q] missing, want %v", key, want)
				} else if got != want {
					t.Errorf("metadata[%q] = %v (%T), want %v (%T)", key, got, got, want, want)
				}
			}
			for _, word := range tt.forbidden {
				for key, val := range found.Metadata {
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

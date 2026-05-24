//go:build postgresql

package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditCommandPostgreSQLAdvancedIndexFormsSupportedAndCovered(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE INDEX idx_users_active_email ON users (email) WHERE active = true", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}

	if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
		t.Fatalf("expected no unsupported details, got %#v", unsupported)
	}

	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
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
		if finding["rule_id"] == "ddl.pg.create_index.concurrently.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding with rule_id ddl.pg.create_index.concurrently.require, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLObjectLifecycleRuleCoverage(t *testing.T) {
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
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
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

func TestAuditCommandPostgreSQLAlterTableGapRuleCoverage(t *testing.T) {
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
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
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

func TestAuditCommandPostgreSQLRefreshMaterializedViewRuleCoverage(t *testing.T) {
	t.Run("basic_refresh_concurrently_warn", func(t *testing.T) {
		stdout := &strings.Builder{}
		stderr := &strings.Builder{}

		code := Execute(
			context.Background(),
			[]string{"audit", "--sql", "REFRESH MATERIALIZED VIEW mv_stats;", "--dialect", "postgresql", "--format", "json"},
			strings.NewReader(""),
			stdout,
			stderr,
		)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
		}

		var decoded map[string]any
		if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
			t.Fatalf("unmarshal json output: %v", err)
		}
		if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
			t.Fatalf("expected no unsupported, got %#v", unsupported)
		}
		statements, ok := decoded["statements"].([]any)
		if !ok || len(statements) != 1 {
			t.Fatalf("expected one statement, got %#v", decoded["statements"])
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
		for _, f := range findings {
			finding, ok := f.(map[string]any)
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
		stdout := &strings.Builder{}
		stderr := &strings.Builder{}

		code := Execute(
			context.Background(),
			[]string{"audit", "--sql", "REFRESH MATERIALIZED VIEW mv_stats WITH NO DATA;", "--dialect", "postgresql", "--format", "json"},
			strings.NewReader(""),
			stdout,
			stderr,
		)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
		}

		var decoded map[string]any
		if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
			t.Fatalf("unmarshal json output: %v", err)
		}
		statements, ok := decoded["statements"].([]any)
		if !ok || len(statements) != 1 {
			t.Fatalf("expected one statement, got %#v", decoded["statements"])
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
		for _, f := range findings {
			finding, ok := f.(map[string]any)
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

func TestAuditCommandPostgreSQLAlterTableUnsupportedActionRuleCoverage(t *testing.T) {
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
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
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

func TestAuditCommandPostgreSQLAlterTableLoggedStateRuleCoverage(t *testing.T) {
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
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
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

func TestAuditCommandPostgreSQLAlterTableStorageLayoutRuleCoverage(t *testing.T) {
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
		{
			name:       "constraint_deferrable_notice",
			sql:        "ALTER TABLE orders ALTER CONSTRAINT orders_user_id_fkey DEFERRABLE",
			wantRuleID: "ddl.pg.alter.constraint_deferrable.notice",
			wantLevel:  "notice",
		},
		{
			name:       "constraint_initially_deferred_notice",
			sql:        "ALTER TABLE orders ALTER CONSTRAINT orders_user_id_fkey INITIALLY DEFERRED",
			wantRuleID: "ddl.pg.alter.constraint_initially_deferred.notice",
			wantLevel:  "notice",
		},
		{
			name:       "set_expression_notice",
			sql:        "ALTER TABLE users ALTER COLUMN full_name SET EXPRESSION AS (first_name || ' ' || last_name)",
			wantRuleID: "ddl.pg.alter.set_expression.notice",
			wantLevel:  "notice",
		},
		{
			name:       "add_identity_notice",
			sql:        "ALTER TABLE users ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY",
			wantRuleID: "ddl.pg.alter.add_identity.notice",
			wantLevel:  "notice",
		},
		{
			name:       "add_exclusion_constraint_notice",
			sql:        "ALTER TABLE bookings ADD CONSTRAINT bookings_no_overlap EXCLUDE USING gist (room_id WITH =, during WITH &&)",
			wantRuleID: "ddl.pg.alter.add_exclusion_constraint.notice",
			wantLevel:  "notice",
		},
		{
			name:       "move_all_tablespace_notice",
			sql:        "ALTER TABLE ALL IN TABLESPACE pg_default SET TABLESPACE fastspace",
			wantRuleID: "ddl.pg.alter.move_all_tablespace.notice",
			wantLevel:  "notice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
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
					if finding["level"] != tt.wantLevel {
						t.Errorf("expected level %s, got %v", tt.wantLevel, finding["level"])
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

func TestAuditCommandPostgreSQLTypeLifecycleRuleCoverage(t *testing.T) {
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
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) == 0 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
			}
			for _, f := range findings {
				finding, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if _, expected := wantRuleIDs[finding["rule_id"].(string)]; expected {
					wantRuleIDs[finding["rule_id"].(string)] = true
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

func TestAuditCommandPostgreSQLCompositeTypeLifecycleRuleCoverage(t *testing.T) {
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
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) == 0 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
			}
			for _, f := range findings {
				finding, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if _, expected := wantRuleIDs[finding["rule_id"].(string)]; expected {
					wantRuleIDs[finding["rule_id"].(string)] = true
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

func TestAuditCommandPostgreSQLDomainLifecycleRuleCoverage(t *testing.T) {
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
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) == 0 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
			}
			for _, f := range findings {
				finding, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if _, expected := wantRuleIDs[finding["rule_id"].(string)]; expected {
					wantRuleIDs[finding["rule_id"].(string)] = true
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

func TestAuditCommandPostgreSQLExtensionLifecycleRuleCoverage(t *testing.T) {
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
		{name: "create_text_search_configuration_notice", sql: "CREATE TEXT SEARCH CONFIGURATION english_copy ( COPY = pg_catalog.english )", wantRuleIDs: []string{"ddl.pg.create_text_search_configuration.notice"}},
		{name: "alter_text_search_configuration_notice", sql: "ALTER TEXT SEARCH CONFIGURATION english_copy RENAME TO english_copy_v2", wantRuleIDs: []string{"ddl.pg.alter_text_search_configuration.notice"}},
		{name: "drop_text_search_configuration_warn", sql: "DROP TEXT SEARCH CONFIGURATION english_copy", wantRuleIDs: []string{"ddl.pg.drop_text_search_configuration.warn"}},
		{name: "create_text_search_dictionary_notice", sql: "CREATE TEXT SEARCH DICTIONARY simple_dict (TEMPLATE = simple)", wantRuleIDs: []string{"ddl.pg.create_text_search_dictionary.notice"}},
		{name: "alter_text_search_dictionary_notice", sql: "ALTER TEXT SEARCH DICTIONARY simple_dict RENAME TO simple_dict_v2", wantRuleIDs: []string{"ddl.pg.alter_text_search_dictionary.notice"}},
		{name: "drop_text_search_dictionary_warn", sql: "DROP TEXT SEARCH DICTIONARY simple_dict", wantRuleIDs: []string{"ddl.pg.drop_text_search_dictionary.warn"}},
		{name: "create_text_search_parser_notice", sql: "CREATE TEXT SEARCH PARSER parser_name (START = start_func, GETTOKEN = token_func, END = end_func, LEXTYPES = lextype_func)", wantRuleIDs: []string{"ddl.pg.create_text_search_parser.notice"}},
		{name: "alter_text_search_parser_notice", sql: "ALTER TEXT SEARCH PARSER parser_name RENAME TO parser_name_v2", wantRuleIDs: []string{"ddl.pg.alter_text_search_parser.notice"}},
		{name: "drop_text_search_parser_warn", sql: "DROP TEXT SEARCH PARSER parser_name", wantRuleIDs: []string{"ddl.pg.drop_text_search_parser.warn"}},
		{name: "create_text_search_template_notice", sql: "CREATE TEXT SEARCH TEMPLATE template_name (LEXIZE = lexize_func)", wantRuleIDs: []string{"ddl.pg.create_text_search_template.notice"}},
		{name: "alter_text_search_template_notice", sql: "ALTER TEXT SEARCH TEMPLATE template_name RENAME TO template_name_v2", wantRuleIDs: []string{"ddl.pg.alter_text_search_template.notice"}},
		{name: "drop_text_search_template_warn", sql: "DROP TEXT SEARCH TEMPLATE template_name", wantRuleIDs: []string{"ddl.pg.drop_text_search_template.warn"}},
		{name: "create_transform_notice", sql: "CREATE TRANSFORM FOR jsonb LANGUAGE plpython3u (FROM SQL WITH FUNCTION jsonb_to_plpython(jsonb), TO SQL WITH FUNCTION plpython_to_jsonb(internal))", wantRuleIDs: []string{"ddl.pg.create_transform.notice"}},
		{name: "create_access_method_notice", sql: "CREATE ACCESS METHOD heap2 TYPE TABLE HANDLER heap_tableam_handler", wantRuleIDs: []string{"ddl.pg.create_access_method.notice"}},
		{name: "drop_transform_warn", sql: "DROP TRANSFORM FOR jsonb LANGUAGE plpython3u", wantRuleIDs: []string{"ddl.pg.drop_transform.warn"}},
		{name: "drop_access_method_warn", sql: "DROP ACCESS METHOD heap2", wantRuleIDs: []string{"ddl.pg.drop_access_method.warn"}},
		{name: "alter_large_object_owner_notice", sql: "ALTER LARGE OBJECT 12345 OWNER TO app_owner", wantRuleIDs: []string{"ddl.pg.alter_large_object.owner.notice"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) == 0 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
			}
			for _, f := range findings {
				finding, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if _, expected := wantRuleIDs[finding["rule_id"].(string)]; expected {
					wantRuleIDs[finding["rule_id"].(string)] = true
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

func TestAuditCommandPostgreSQLPolicyLifecycleRuleCoverage(t *testing.T) {
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
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
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

func TestAuditCommandPostgreSQLTriggerLifecycleRuleCoverage(t *testing.T) {
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
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
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

func TestAuditCommandPostgreSQLAlterObjectLifecycleRuleCoverage(t *testing.T) {
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
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
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

func TestAuditCommandPostgreSQLSemanticMetadataParity(t *testing.T) {
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
		// Bounded residual: column attribute metadata
		{
			name:   "set_column_statistics_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN email SET STATISTICS 100",
			ruleID: "ddl.pg.alter.set_column_statistics.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "set_statistics", "table": "users",
				"column": "email", "statistics_target_kind": "value", "has_statistics_target": "true",
			},
			forbidden: []string{"100", "raw_sql", "n_distinct", "-1"},
		},
		{
			name:   "set_column_statistics_default_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN email SET STATISTICS DEFAULT",
			ruleID: "ddl.pg.alter.set_column_statistics.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "set_statistics", "table": "users",
				"column": "email", "statistics_target_kind": "default", "has_statistics_target": "true",
			},
			forbidden: []string{"raw_sql"},
		},
		{
			name:   "set_column_options_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN email SET (n_distinct = -1)",
			ruleID: "ddl.pg.alter.set_column_options.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "set_column_options", "table": "users",
				"column": "email", "option_count": "1", "has_column_options": "true",
			},
			forbidden: []string{"n_distinct", "-1", "raw_sql", "option_names", "option_values"},
		},
		{
			name:   "reset_column_options_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN email RESET (n_distinct)",
			ruleID: "ddl.pg.alter.reset_column_options.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "reset_column_options", "table": "users",
				"column": "email", "reset_count": "1", "has_column_options": "true",
			},
			forbidden: []string{"n_distinct", "-1", "raw_sql", "option_names", "option_values"},
		},
		{
			name:   "set_column_storage_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN bio SET STORAGE EXTERNAL",
			ruleID: "ddl.pg.alter.set_column_storage.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "set_storage", "table": "users",
				"column": "bio", "storage_kind": "external", "has_storage_setting": "true",
			},
			forbidden: []string{"raw_sql", "lz4", "pglz"},
		},
		{
			name:   "set_column_storage_default_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN bio SET STORAGE DEFAULT",
			ruleID: "ddl.pg.alter.set_column_storage.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "set_storage", "table": "users",
				"column": "bio", "storage_kind": "default", "has_storage_setting": "true",
			},
			forbidden: []string{"raw_sql"},
		},
		{
			name:   "set_column_compression_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN bio SET COMPRESSION lz4",
			ruleID: "ddl.pg.alter.set_column_compression.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "set_compression", "table": "users",
				"column": "bio", "has_compression": "true",
			},
			forbidden: []string{"lz4", "pglz", "compression_kind", "raw_sql"},
		},
		// Bounded residual: cluster/finalize metadata
		{
			name:   "cluster_on_metadata",
			sql:    "ALTER TABLE users CLUSTER ON users_email_idx",
			ruleID: "ddl.pg.alter.cluster_on.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "cluster_on", "table": "users",
				"index": "users_email_idx", "has_cluster_index": "true",
			},
			forbidden: []string{"cluster_sql", "raw_sql"},
		},
		{
			name:   "set_without_cluster_metadata",
			sql:    "ALTER TABLE users SET WITHOUT CLUSTER",
			ruleID: "ddl.pg.alter.set_without_cluster.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "set_without_cluster", "table": "users",
				"has_cluster_index": "false",
			},
			forbidden: []string{"cluster_sql", "raw_sql"},
		},
		{
			name:   "detach_partition_finalize_metadata",
			sql:    "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04 FINALIZE",
			ruleID: "ddl.pg.alter.detach_partition_finalize.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "detach_partition_finalize", "table": "measurement",
				"partition": "measurement_y2026m04", "finalize": "true",
			},
			forbidden: []string{"partition_bound", "raw_sql", "concurrently"},
		},
		// Table relationship metadata
		{
			name:   "add_inherit_metadata",
			sql:    "ALTER TABLE child_users INHERIT users",
			ruleID: "ddl.pg.alter.add_inherit.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "add_inherit", "table": "child_users",
				"parent_table": "users", "relationship": "inheritance",
			},
			forbidden: []string{"raw_sql", "column_definition", "type_attributes", "catalog_state", "validation_result", "dependency_graph"},
		},
		{
			name:   "drop_inherit_metadata",
			sql:    "ALTER TABLE child_users NO INHERIT users",
			ruleID: "ddl.pg.alter.drop_inherit.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "drop_inherit", "table": "child_users",
				"parent_table": "users", "relationship": "inheritance",
			},
			forbidden: []string{"raw_sql", "column_definition", "type_attributes", "catalog_state", "validation_result", "dependency_graph"},
		},
		{
			name:   "add_of_type_metadata",
			sql:    "ALTER TABLE users OF user_type",
			ruleID: "ddl.pg.alter.add_of_type.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "add_of_type", "table": "users",
				"type": "user_type", "relationship": "typed_table",
			},
			forbidden: []string{"raw_sql", "column_definition", "type_attributes", "catalog_state", "validation_result", "dependency_graph", "column_shape", "type_shape", "compatibility_result"},
		},
		{
			name:   "drop_of_type_metadata",
			sql:    "ALTER TABLE users NOT OF",
			ruleID: "ddl.pg.alter.drop_of_type.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "drop_of_type", "table": "users",
				"relationship": "typed_table",
			},
			forbidden: []string{"raw_sql", "column_definition", "type_attributes", "catalog_state", "validation_result", "dependency_graph", "column_shape", "type_shape", "compatibility_result"},
		},
		{
			name:   "constraint_deferrable_metadata",
			sql:    "ALTER TABLE orders ALTER CONSTRAINT orders_user_id_fkey DEFERRABLE",
			ruleID: "ddl.pg.alter.constraint_deferrable.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "alter_constraint_deferrable", "table": "orders",
				"constraint": "orders_user_id_fkey", "constraint_type": "foreign_key",
				"deferrable": "true", "initially_deferred": "false",
			},
			forbidden: []string{"raw_sql", "expression", "predicate", "operator_class", "exclusions", "sequence_options", "catalog_state", "validation_result", "dependency_graph"},
		},
		{
			name:   "constraint_initially_deferred_metadata",
			sql:    "ALTER TABLE orders ALTER CONSTRAINT orders_user_id_fkey INITIALLY DEFERRED",
			ruleID: "ddl.pg.alter.constraint_initially_deferred.notice",
			wantMetadata: map[string]any{
				"operation": "alter_table", "action": "alter_constraint_initially_deferred", "table": "orders",
				"constraint": "orders_user_id_fkey", "constraint_type": "foreign_key",
				"deferrable": "true", "initially_deferred": "true",
			},
			forbidden: []string{"raw_sql", "expression", "predicate", "operator_class", "exclusions", "sequence_options", "catalog_state", "validation_result", "dependency_graph"},
		},
		{
			name:   "set_expression_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN full_name SET EXPRESSION AS (first_name || ' ' || last_name)",
			ruleID: "ddl.pg.alter.set_expression.notice",
			wantMetadata: map[string]any{
				"operation":      "alter_table",
				"action":         "set_expression",
				"table":          "users",
				"column":         "full_name",
				"has_expression": "true",
			},
			forbidden: []string{"first_name", "last_name", "||", "raw_sql"},
		},
		{
			name:   "add_identity_metadata",
			sql:    "ALTER TABLE users ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY",
			ruleID: "ddl.pg.alter.add_identity.notice",
			wantMetadata: map[string]any{
				"operation":      "alter_table",
				"action":         "add_identity",
				"table":          "users",
				"column":         "id",
				"identity":       "true",
				"generated_when": "by_default",
			},
			forbidden: []string{"sequence_options", "start", "increment", "cache", "cycle", "raw_sql"},
		},
		{
			name:   "add_exclusion_constraint_metadata",
			sql:    "ALTER TABLE bookings ADD CONSTRAINT bookings_no_overlap EXCLUDE USING gist (room_id WITH =, during WITH &&)",
			ruleID: "ddl.pg.alter.add_exclusion_constraint.notice",
			wantMetadata: map[string]any{
				"operation":       "alter_table",
				"action":          "add_exclusion_constraint",
				"table":           "bookings",
				"constraint":      "bookings_no_overlap",
				"constraint_type": "exclusion",
				"access_method":   "gist",
			},
			forbidden: []string{"exclusions", "operator", "operator_class", "room_id", "during", "&&", "predicate", "where_clause", "raw_sql"},
		},
		{
			name:   "move_all_tablespace_metadata",
			sql:    "ALTER TABLE ALL IN TABLESPACE pg_default SET TABLESPACE fastspace",
			ruleID: "ddl.pg.alter.move_all_tablespace.notice",
			wantMetadata: map[string]any{
				"operation":          "alter_table",
				"action":             "move_all_tablespace",
				"object_type":        "table",
				"source_tablespace":  "pg_default",
				"target_tablespace":  "fastspace",
				"has_table_identity": "false",
			},
			forbidden: []string{"raw_sql"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}
			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""), stdout, stderr,
			)
			if code != 0 {
				t.Fatalf("exit code %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal: %v\noutput=%s", err, stdout.String())
			}
			statements, _ := decoded["statements"].([]any)
			if len(statements) == 0 {
				t.Fatal("expected at least one statement")
			}
			statement, _ := statements[0].(map[string]any)
			findings, _ := statement["findings"].([]any)
			var finding map[string]any
			for _, f := range findings {
				fm, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if fm["rule_id"] == tt.ruleID {
					finding = fm
					break
				}
			}
			if finding == nil {
				t.Fatalf("expected finding for rule %q, got %#v", tt.ruleID, findings)
			}
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

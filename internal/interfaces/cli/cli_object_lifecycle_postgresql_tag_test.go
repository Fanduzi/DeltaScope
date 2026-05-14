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

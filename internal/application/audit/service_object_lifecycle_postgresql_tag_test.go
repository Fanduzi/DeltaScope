//go:build postgresql

package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLPostgreSQLObjectLifecycleRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
		wantLevel  rule.Level
	}{
		{
			name:       "drop_schema_advisory",
			sql:        "DROP SCHEMA staging;",
			wantRuleID: "ddl.pg.drop_schema.advisory",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "drop_schema_cascade_warn",
			sql:        "DROP SCHEMA staging CASCADE;",
			wantRuleID: "ddl.pg.drop_schema.cascade.warn",
			wantLevel:  rule.LevelWarning,
		},
		{
			name:       "create_sequence_cycle_warn",
			sql:        "CREATE SEQUENCE order_seq START WITH 1 INCREMENT BY 1 CYCLE;",
			wantRuleID: "ddl.pg.create_sequence.cycle.warn",
			wantLevel:  rule.LevelWarning,
		},
		{
			name:       "alter_sequence_restart_warn",
			sql:        "ALTER SEQUENCE order_seq RESTART WITH 100;",
			wantRuleID: "ddl.pg.alter_sequence.restart.warn",
			wantLevel:  rule.LevelWarning,
		},
		{
			name:       "alter_sequence_cycle_warn",
			sql:        "ALTER SEQUENCE order_seq CYCLE;",
			wantRuleID: "ddl.pg.alter_sequence.cycle.warn",
			wantLevel:  rule.LevelWarning,
		},
		{
			name:       "drop_sequence_advisory",
			sql:        "DROP SEQUENCE order_seq;",
			wantRuleID: "ddl.pg.drop_sequence.advisory",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "drop_sequence_cascade_warn",
			sql:        "DROP SEQUENCE order_seq CASCADE;",
			wantRuleID: "ddl.pg.drop_sequence.cascade.warn",
			wantLevel:  rule.LevelWarning,
		},
		{
			name:       "drop_materialized_view_advisory",
			sql:        "DROP MATERIALIZED VIEW mv_daily_sales;",
			wantRuleID: "ddl.pg.drop_materialized_view.advisory",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "drop_materialized_view_cascade_warn",
			sql:        "DROP MATERIALIZED VIEW mv_daily_sales CASCADE;",
			wantRuleID: "ddl.pg.drop_materialized_view.cascade.warn",
			wantLevel:  rule.LevelWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: spec.DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			found := false
			for _, f := range result.Statements[0].Findings {
				if f.RuleID == tt.wantRuleID {
					found = true
					if f.Level != tt.wantLevel {
						t.Errorf("expected level %q, got %q", tt.wantLevel, f.Level)
					}
					if f.Metadata["object_type"] == nil {
						t.Errorf("expected object_type metadata, got nil")
					}
					break
				}
			}
			if !found {
				t.Fatalf("expected finding %q, got %v", tt.wantRuleID, collectAuditResultRuleIDs(result))
			}
		})
	}
}

// TestAuditSQLPostgreSQLObjectLifecycleNegativeCases proves that lifecycle
// rules do NOT fire for: non-PG dialects, operations without the required
// option, and operations that are not lifecycle-related.
func TestAuditSQLPostgreSQLObjectLifecycleNegativeCases(t *testing.T) {
	t.Parallel()
	pgLifecycleRuleIDs := []string{
		"ddl.pg.drop_schema.advisory",
		"ddl.pg.drop_schema.cascade.warn",
		"ddl.pg.create_sequence.cycle.warn",
		"ddl.pg.alter_sequence.restart.warn",
		"ddl.pg.alter_sequence.cycle.warn",
		"ddl.pg.drop_sequence.advisory",
		"ddl.pg.drop_sequence.cascade.warn",
		"ddl.pg.drop_materialized_view.advisory",
		"ddl.pg.drop_materialized_view.cascade.warn",
	}

	assertNoPGLifecycleFindings := func(t *testing.T, result *report.Result) {
		t.Helper()
		for _, stmt := range result.Statements {
			for _, f := range stmt.Findings {
				for _, pgID := range pgLifecycleRuleIDs {
					if f.RuleID == pgID {
						t.Fatalf("PG lifecycle rule %q should not fire, but did", pgID)
					}
				}
			}
		}
	}

	t.Run("mysql_dialect_does_not_fire_lifecycle_rules", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name           string
			sql            string
			expectParseErr bool
		}{
			{name: "drop_schema", sql: "DROP SCHEMA staging;"},
			{name: "drop_sequence", sql: "DROP SEQUENCE order_seq;"},
			{name: "drop_materialized_view", sql: "DROP MATERIALIZED VIEW mv_daily_sales;", expectParseErr: true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt := tt
				t.Parallel()
				result, err := AuditSQL(context.Background(), Request{
					SQL:     tt.sql,
					Dialect: spec.DialectMySQL,
				})
				if tt.expectParseErr {
					if err == nil {
						t.Fatalf("expected parse error for MySQL-incompatible SQL, got nil")
					}
					return
				}
				if err != nil {
					t.Fatalf("audit sql: %v", err)
				}
				assertNoPGLifecycleFindings(t, &result)
			})
		}
	})

	t.Run("create_schema_default_no_finding", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{
			SQL:     "CREATE SCHEMA staging;",
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit sql: %v", err)
		}
		assertNoPGLifecycleFindings(t, &result)
	})

	t.Run("create_sequence_without_cycle_no_lifecycle_finding", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{
			SQL:     "CREATE SEQUENCE order_seq START WITH 1 INCREMENT BY 1;",
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit sql: %v", err)
		}
		for _, stmt := range result.Statements {
			for _, f := range stmt.Findings {
				for _, pgID := range pgLifecycleRuleIDs {
					if f.RuleID == pgID {
						t.Fatalf("PG lifecycle rule %q should not fire for CREATE SEQUENCE without CYCLE", pgID)
					}
				}
			}
		}
	})

	t.Run("drop_schema_without_cascade_no_cascade_warn", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{
			SQL:     "DROP SCHEMA staging;",
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit sql: %v", err)
		}
		for _, stmt := range result.Statements {
			for _, f := range stmt.Findings {
				if f.RuleID == "ddl.pg.drop_schema.cascade.warn" {
					t.Fatalf("cascade warn should not fire without CASCADE option")
				}
			}
		}
	})

	t.Run("drop_materialized_view_without_cascade_no_cascade_warn", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{
			SQL:     "DROP MATERIALIZED VIEW mv_daily_sales;",
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit sql: %v", err)
		}
		for _, stmt := range result.Statements {
			for _, f := range stmt.Findings {
				if f.RuleID == "ddl.pg.drop_materialized_view.cascade.warn" {
					t.Fatalf("cascade warn should not fire without CASCADE option")
				}
			}
		}
	})
}

// TestAuditSQLPostgreSQLRefreshMaterializedViewRules proves that the two
// PG-only refresh materialized view rules fire through the full AuditSQL
// pipeline with correct rule IDs, levels, and metadata.
func TestAuditSQLPostgreSQLRefreshMaterializedViewRules(t *testing.T) {
	t.Parallel()
	t.Run("basic_refresh_fires_concurrently_warn_only", func(t *testing.T) {
		t.Parallel()
		const sql = "REFRESH MATERIALIZED VIEW mv_stats;"
		result, err := AuditSQL(context.Background(), Request{
			SQL:     sql,
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit sql: %v", err)
		}
		if len(result.Unsupported) != 0 {
			t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
		}
		if len(result.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result.Statements))
		}

		// Verify normalization.
		stmt, ok := corpusExtractStatement(t, sql, spec.DialectPostgreSQL)
		if !ok {
			t.Fatal("expected supported statement")
		}
		if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationRefreshMaterializedView {
			t.Fatalf("expected refresh_materialized_view operation")
		}

		var foundConcurrent bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.refresh_materialized_view.concurrently.warn" {
				foundConcurrent = true
				if f.Level != rule.LevelWarning {
					t.Errorf("expected warning, got %s", f.Level)
				}
				if f.Metadata["operation"] != "refresh_materialized_view" {
					t.Errorf("expected operation metadata, got %v", f.Metadata["operation"])
				}
				if f.Metadata["object"] != "mv_stats" {
					t.Errorf("expected object=mv_stats, got %v", f.Metadata["object"])
				}
			}
			if f.RuleID == "ddl.pg.refresh_materialized_view.no_data.notice" {
				t.Error("no-data notice should not fire for basic refresh")
			}
		}
		if !foundConcurrent {
			t.Fatalf("expected concurrently.warn finding, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("concurrent_refresh_no_findings", func(t *testing.T) {
		t.Parallel()
		const sql = "REFRESH MATERIALIZED VIEW CONCURRENTLY mv_stats;"
		result, err := AuditSQL(context.Background(), Request{
			SQL:     sql,
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit sql: %v", err)
		}
		if len(result.Unsupported) != 0 {
			t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
		}
		if len(result.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result.Statements))
		}

		stmt, ok := corpusExtractStatement(t, sql, spec.DialectPostgreSQL)
		if !ok {
			t.Fatal("expected supported statement")
		}
		if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationRefreshMaterializedView {
			t.Fatalf("expected refresh_materialized_view operation")
		}
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.refresh_materialized_view.concurrently.warn" ||
				f.RuleID == "ddl.pg.refresh_materialized_view.no_data.notice" {
				t.Errorf("concurrent refresh should produce no findings, got %s", f.RuleID)
			}
		}
	})

	t.Run("with_data_fires_concurrently_warn_only", func(t *testing.T) {
		t.Parallel()
		const sql = "REFRESH MATERIALIZED VIEW mv_stats WITH DATA;"
		result, err := AuditSQL(context.Background(), Request{
			SQL:     sql,
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit sql: %v", err)
		}
		if len(result.Unsupported) != 0 {
			t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
		}
		if len(result.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result.Statements))
		}

		stmt, ok := corpusExtractStatement(t, sql, spec.DialectPostgreSQL)
		if !ok {
			t.Fatal("expected supported statement")
		}
		if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationRefreshMaterializedView {
			t.Fatalf("expected refresh_materialized_view operation")
		}

		var foundConcurrent bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.refresh_materialized_view.concurrently.warn" {
				foundConcurrent = true
				if f.Level != rule.LevelWarning {
					t.Errorf("expected warning, got %s", f.Level)
				}
			}
			if f.RuleID == "ddl.pg.refresh_materialized_view.no_data.notice" {
				t.Error("no-data notice should not fire for WITH DATA")
			}
		}
		if !foundConcurrent {
			t.Fatalf("expected concurrently.warn finding, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("with_no_data_fires_both", func(t *testing.T) {
		t.Parallel()
		const sql = "REFRESH MATERIALIZED VIEW mv_stats WITH NO DATA;"
		result, err := AuditSQL(context.Background(), Request{
			SQL:     sql,
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit sql: %v", err)
		}
		if len(result.Unsupported) != 0 {
			t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
		}
		if len(result.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result.Statements))
		}

		stmt, ok := corpusExtractStatement(t, sql, spec.DialectPostgreSQL)
		if !ok {
			t.Fatal("expected supported statement")
		}
		if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationRefreshMaterializedView {
			t.Fatalf("expected refresh_materialized_view operation")
		}

		var foundConcurrent, foundNoData bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.refresh_materialized_view.concurrently.warn" {
				foundConcurrent = true
				if f.Level != rule.LevelWarning {
					t.Errorf("expected warning, got %s", f.Level)
				}
			}
			if f.RuleID == "ddl.pg.refresh_materialized_view.no_data.notice" {
				foundNoData = true
				if f.Level != rule.LevelNotice {
					t.Errorf("expected notice, got %s", f.Level)
				}
			}
		}
		if !foundConcurrent {
			t.Fatalf("expected concurrently.warn finding, got %v", collectAuditResultRuleIDs(result))
		}
		if !foundNoData {
			t.Fatalf("expected no_data.notice finding, got %v", collectAuditResultRuleIDs(result))
		}
	})
}

func TestAuditSQLPostgreSQLReplicaIdentityRules(t *testing.T) {
	t.Parallel()
	t.Run("replica_identity_full_warn", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TABLE users REPLICA IDENTITY FULL;"
		result, err := AuditSQL(context.Background(), Request{
			SQL:     sql,
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit sql: %v", err)
		}
		if len(result.Unsupported) != 0 {
			t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
		}
		if len(result.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result.Statements))
		}

		var found bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.alter.replica_identity_full.warn" {
				found = true
				if f.Level != rule.LevelWarning {
					t.Errorf("expected warning, got %s", f.Level)
				}
				if f.Metadata["action"] != "replica_identity" {
					t.Errorf("expected action=replica_identity, got %v", f.Metadata["action"])
				}
				if f.Metadata["identity"] != "full" {
					t.Errorf("expected identity=full, got %v", f.Metadata["identity"])
				}
				if f.Metadata["table"] != "users" {
					t.Errorf("expected table=users, got %v", f.Metadata["table"])
				}
			}
		}
		if !found {
			t.Fatalf("expected replica_identity_full.warn, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("replica_identity_nothing_warn", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TABLE users REPLICA IDENTITY NOTHING;"
		result, err := AuditSQL(context.Background(), Request{
			SQL:     sql,
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit sql: %v", err)
		}
		if len(result.Unsupported) != 0 {
			t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
		}
		if len(result.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result.Statements))
		}

		var found bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.alter.replica_identity_nothing.warn" {
				found = true
				if f.Level != rule.LevelWarning {
					t.Errorf("expected warning, got %s", f.Level)
				}
				if f.Metadata["action"] != "replica_identity" {
					t.Errorf("expected action=replica_identity, got %v", f.Metadata["action"])
				}
				if f.Metadata["identity"] != "nothing" {
					t.Errorf("expected identity=nothing, got %v", f.Metadata["identity"])
				}
				if f.Metadata["table"] != "users" {
					t.Errorf("expected table=users, got %v", f.Metadata["table"])
				}
			}
		}
		if !found {
			t.Fatalf("expected replica_identity_nothing.warn, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("replica_identity_using_index_notice", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TABLE users REPLICA IDENTITY USING INDEX users_replica_identity_idx;"
		result, err := AuditSQL(context.Background(), Request{
			SQL:     sql,
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit sql: %v", err)
		}
		if len(result.Unsupported) != 0 {
			t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
		}
		if len(result.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result.Statements))
		}

		var found bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.alter.replica_identity_using_index.notice" {
				found = true
				if f.Level != rule.LevelNotice {
					t.Errorf("expected notice, got %s", f.Level)
				}
				if f.Metadata["action"] != "replica_identity" {
					t.Errorf("expected action=replica_identity, got %v", f.Metadata["action"])
				}
				if f.Metadata["identity"] != "using_index" {
					t.Errorf("expected identity=using_index, got %v", f.Metadata["identity"])
				}
				if f.Metadata["index"] != "users_replica_identity_idx" {
					t.Errorf("expected index=users_replica_identity_idx, got %v", f.Metadata["index"])
				}
				if f.Metadata["table"] != "users" {
					t.Errorf("expected table=users, got %v", f.Metadata["table"])
				}
			}
		}
		if !found {
			t.Fatalf("expected replica_identity_using_index.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("replica_identity_default_silent", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TABLE users REPLICA IDENTITY DEFAULT;"
		result, err := AuditSQL(context.Background(), Request{
			SQL:     sql,
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("audit sql: %v", err)
		}
		if len(result.Unsupported) != 0 {
			t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
		}
		if len(result.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result.Statements))
		}

		stmt, ok := corpusExtractStatement(t, sql, spec.DialectPostgreSQL)
		if !ok {
			t.Fatal("expected supported statement")
		}
		if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
			t.Fatalf("expected alter_table operation")
		}
		for _, f := range result.Statements[0].Findings {
			if strings.HasPrefix(f.RuleID, "ddl.pg.alter.replica_identity") {
				t.Errorf("DEFAULT should be silent, got finding %s", f.RuleID)
			}
		}
	})
}

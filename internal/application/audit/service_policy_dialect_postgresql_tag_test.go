//go:build postgresql

package audit

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLPostgreSQLDefaultPolicyDialectHygiene(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE TABLE pg_smoke (id bigint primary key);",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}

	ruleIDs := collectAuditResultRuleIDs(result)
	seen := make(map[string]struct{}, len(ruleIDs))
	for _, ruleID := range ruleIDs {
		seen[ruleID] = struct{}{}
	}
	leakedRuleIDs := make([]string, 0)
	for _, ruleID := range defaultPolicyDialectHygienePostgreSQLForbiddenRuleIDs {
		if _, ok := seen[ruleID]; ok {
			leakedRuleIDs = append(leakedRuleIDs, ruleID)
		}
	}
	sort.Strings(leakedRuleIDs)

	text := collectAuditResultText(result)
	normalizedText := strings.ToLower(text)
	matchedTokens := make([]string, 0)
	for _, token := range defaultPolicyDialectHygienePostgreSQLForbiddenTokens {
		if strings.Contains(normalizedText, strings.ToLower(token)) {
			matchedTokens = append(matchedTokens, token)
		}
	}

	if len(leakedRuleIDs) > 0 || len(matchedTokens) > 0 {
		t.Fatalf("expected PostgreSQL default-policy audit hygiene; leaked_rule_ids=%v matched_tokens=%v all_rule_ids=%v text=%s", leakedRuleIDs, matchedTokens, ruleIDs, fmt.Sprintf("%q", text))
	}
}

// ---------------------------------------------------------------------------
// v0.48.0 Task 3: Service tests — PG migration-safety gap closure rules
// ---------------------------------------------------------------------------

// TestAuditSQLPostgreSQLDropIndexAdvisory proves that DROP INDEX triggers the
// PG-only advisory alongside the existing cross-dialect drop_index.exists.require
// (when metadata is provided). Both findings coexist as expected.
func TestAuditSQLPostgreSQLDropIndexAdvisory(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "DROP INDEX idx_users_email;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	found := false
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.drop_index.advisory" {
			found = true
			if f.Level != "notice" {
				t.Errorf("expected notice level, got %q", f.Level)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected ddl.pg.drop_index.advisory finding, got %#v", result.Statements[0].Findings)
	}
}

// TestAuditSQLPostgreSQLAddColumnNonNullNoDefaultWarn covers the three
// behavioral branches for ALTER TABLE ADD COLUMN:
//   - NOT NULL without DEFAULT → fires the PG warning
//   - NOT NULL with DEFAULT → does NOT fire the new rule (may fire existing rewrite warning)
//   - nullable → does NOT fire the new rule
func TestAuditSQLPostgreSQLAddColumnNonNullNoDefaultWarn(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		sql             string
		wantRulePresent bool
		wantRuleAbsent  bool
	}{
		{
			name:            "not_null_no_default_fires",
			sql:             "ALTER TABLE users ADD COLUMN bio text NOT NULL;",
			wantRulePresent: true,
		},
		{
			name:           "not_null_with_default_does_not_fire",
			sql:            "ALTER TABLE users ADD COLUMN email text NOT NULL DEFAULT '';",
			wantRuleAbsent: true,
		},
		{
			name:           "nullable_does_not_fire",
			sql:            "ALTER TABLE users ADD COLUMN bio text;",
			wantRuleAbsent: true,
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
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			found := false
			for _, f := range result.Statements[0].Findings {
				if f.RuleID == "ddl.pg.alter.add_column.non_null_no_default.warn" {
					found = true
					break
				}
			}

			if tt.wantRulePresent && !found {
				t.Fatalf("expected ddl.pg.alter.add_column.non_null_no_default.warn, got %#v", result.Statements[0].Findings)
			}
			if tt.wantRuleAbsent && found {
				t.Fatalf("expected rule to be absent for %q, but it fired", tt.name)
			}
		})
	}
}

// TestAuditSQLPostgreSQLAddUniqueConstraintAdvisory proves that ADD CONSTRAINT
// UNIQUE triggers the PG-only concurrent-index advisory.
func TestAuditSQLPostgreSQLAddUniqueConstraintAdvisory(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "ALTER TABLE users ADD CONSTRAINT uniq_users_email UNIQUE (email);",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	found := false
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.add_unique_constraint.concurrent_index.advisory" {
			found = true
			if f.Level != "notice" {
				t.Errorf("expected notice level, got %q", f.Level)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected ddl.pg.alter.add_unique_constraint.concurrent_index.advisory finding, got %#v", result.Statements[0].Findings)
	}
}

// TestAuditSQLPostgreSQLDropConstraintAdvisory proves that DROP CONSTRAINT
// triggers the PG-only advisory.
func TestAuditSQLPostgreSQLDropConstraintAdvisory(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "ALTER TABLE users DROP CONSTRAINT uniq_email;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	found := false
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.drop_constraint.advisory" {
			found = true
			if f.Level != "notice" {
				t.Errorf("expected notice level, got %q", f.Level)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected ddl.pg.alter.drop_constraint.advisory finding, got %#v", result.Statements[0].Findings)
	}
}

// TestAuditSQLPostgreSQLAlterTableGapRules proves that the three PG-only
// alter-table gap rules from Task 2 fire through the full AuditSQL pipeline
// with correct rule IDs and levels.
func TestAuditSQLPostgreSQLAlterTableGapRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
		wantLevel  rule.Level
	}{
		{
			name:       "drop_column_advisory",
			sql:        "ALTER TABLE users DROP COLUMN email;",
			wantRuleID: "ddl.pg.alter.drop_column.advisory",
			wantLevel:  rule.LevelWarning,
		},
		{
			name:       "validate_constraint_advisory",
			sql:        "ALTER TABLE users VALIDATE CONSTRAINT chk_price;",
			wantRuleID: "ddl.pg.alter.validate_constraint.advisory",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "add_column_nullable_notice",
			sql:        "ALTER TABLE users ADD COLUMN bio text;",
			wantRuleID: "ddl.pg.alter.add_column.nullable.notice",
			wantLevel:  rule.LevelNotice,
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
					break
				}
			}
			if !found {
				t.Fatalf("expected %s finding, got %#v", tt.wantRuleID, result.Statements[0].Findings)
			}
		})
	}
}

// TestAuditSQLPostgreSQLAlterTableUnsupportedActionRules proves that the six
// PG-only alter-table unsupported-action rules fire through the full AuditSQL
// pipeline with correct rule IDs and levels.
func TestAuditSQLPostgreSQLAlterTableUnsupportedActionRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
		wantLevel  rule.Level
	}{
		{
			name:       "set_schema_advisory",
			sql:        "ALTER TABLE users SET SCHEMA archive;",
			wantRuleID: "ddl.pg.alter.set_schema.advisory",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "owner_advisory",
			sql:        "ALTER TABLE users OWNER TO app_owner;",
			wantRuleID: "ddl.pg.alter.owner.advisory",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "enable_trigger_notice",
			sql:        "ALTER TABLE users ENABLE TRIGGER trg_users_audit;",
			wantRuleID: "ddl.pg.alter.enable_trigger.notice",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "disable_trigger_warn",
			sql:        "ALTER TABLE users DISABLE TRIGGER trg_users_audit;",
			wantRuleID: "ddl.pg.alter.disable_trigger.warn",
			wantLevel:  rule.LevelWarning,
		},
		{
			name:       "attach_partition_advisory",
			sql:        "ALTER TABLE measurement ATTACH PARTITION measurement_y2026m04 FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');",
			wantRuleID: "ddl.pg.alter.attach_partition.advisory",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "detach_partition_warn",
			sql:        "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04;",
			wantRuleID: "ddl.pg.alter.detach_partition.warn",
			wantLevel:  rule.LevelWarning,
		},
		{
			name:       "set_logged_notice",
			sql:        "ALTER TABLE users SET LOGGED;",
			wantRuleID: "ddl.pg.alter.set_logged.notice",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "set_unlogged_notice",
			sql:        "ALTER TABLE users SET UNLOGGED;",
			wantRuleID: "ddl.pg.alter.set_unlogged.notice",
			wantLevel:  rule.LevelNotice,
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
					break
				}
			}
			if !found {
				t.Fatalf("expected %s finding, got %#v", tt.wantRuleID, result.Statements[0].Findings)
			}
		})
	}
}

// TestAuditSQLPostgreSQLAddColumnNullableSkipsNotNull proves that the nullable
// notice rule does NOT fire when the added column has NOT NULL.
func TestAuditSQLPostgreSQLAddColumnNullableSkipsNotNull(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "ALTER TABLE users ADD COLUMN bio text NOT NULL;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.add_column.nullable.notice" {
			t.Fatal("nullable notice should not fire for NOT NULL column")
		}
	}
}

// TestAuditSQLPostgreSQLPGOnlyRulesDoNotFireOnMySQL proves that none of the
// four new ddl.pg.* rules fire when dialect is MySQL. This complements the
// domain-level PG-only tests in postgresql_migration_rules_test.go.
func TestAuditSQLPostgreSQLPGOnlyRulesDoNotFireOnMySQL(t *testing.T) {
	t.Parallel()
	pgOnlyRuleIDs := []string{
		"ddl.pg.drop_index.advisory",
		"ddl.pg.alter.add_column.non_null_no_default.warn",
		"ddl.pg.alter.add_unique_constraint.concurrent_index.advisory",
		"ddl.pg.alter.drop_constraint.advisory",
		"ddl.pg.alter.drop_column.advisory",
		"ddl.pg.alter.validate_constraint.advisory",
		"ddl.pg.alter.add_column.nullable.notice",
	}

	tests := []struct {
		name string
		sql  string
	}{
		{name: "drop_index_mysql", sql: "DROP INDEX idx_users_email ON users;"},
		{name: "add_column_nn_mysql", sql: "ALTER TABLE users ADD COLUMN bio text NOT NULL;"},
		{name: "add_unique_mysql", sql: "ALTER TABLE users ADD CONSTRAINT uniq_email UNIQUE (email);"},
		{name: "drop_constraint_mysql", sql: "ALTER TABLE users DROP CONSTRAINT uniq_email;"},
		{name: "drop_column_mysql", sql: "ALTER TABLE users DROP COLUMN email;"},
		{name: "add_column_nullable_mysql", sql: "ALTER TABLE users ADD COLUMN bio text;"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: spec.DialectMySQL,
			})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}

			for _, stmt := range result.Statements {
				for _, f := range stmt.Findings {
					for _, pgID := range pgOnlyRuleIDs {
						if f.RuleID == pgID {
							t.Fatalf("PG-only rule %q fired on MySQL for SQL: %s", pgID, tt.sql)
						}
					}
				}
			}
			for _, f := range result.GlobalFindings {
				for _, pgID := range pgOnlyRuleIDs {
					if f.RuleID == pgID {
						t.Fatalf("PG-only rule %q fired as global finding on MySQL for SQL: %s", pgID, tt.sql)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// v0.64.0 Task 4: Service tests — PG create schema rule
// ---------------------------------------------------------------------------

// TestAuditSQLPostgreSQLCreateSchemaNotice proves that CREATE SCHEMA triggers
// the PG-only notice through the full AuditSQL pipeline.
func TestAuditSQLPostgreSQLCreateSchemaNotice(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE SCHEMA staging",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	found := false
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_schema.notice" {
			found = true
			if f.Level != "notice" {
				t.Errorf("expected notice level, got %q", f.Level)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected ddl.pg.create_schema.notice finding, got %#v", result.Statements[0].Findings)
	}
}

// TestAuditSQLPostgreSQLCreateSchemaDoesNotFireOnMySQL proves that the PG
// create schema rule never fires on MySQL/TiDB.
func TestAuditSQLPostgreSQLCreateSchemaDoesNotFireOnMySQL(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:     "CREATE DATABASE app",
				Dialect: dialect,
			})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}
			for _, stmt := range result.Statements {
				for _, f := range stmt.Findings {
					if f.RuleID == "ddl.pg.create_schema.notice" {
						t.Fatalf("PG create_schema rule fired on %s", dialect)
					}
				}
			}
		})
	}
}

// TestAuditSQLPostgreSQLCreateSchemaIFNotExistsFiresNotice proves that
// CREATE SCHEMA IF NOT EXISTS still triggers the notice rule.
func TestAuditSQLPostgreSQLCreateSchemaIFNotExistsFiresNotice(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE SCHEMA IF NOT EXISTS app",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	found := false
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_schema.notice" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ddl.pg.create_schema.notice finding for CREATE SCHEMA IF NOT EXISTS, got %#v", result.Statements[0].Findings)
	}
}

// TestAuditSQLDatabaseRulesDoNotFireOnPostgreSQL proves that MySQL/TiDB
// database lifecycle rules never fire on PostgreSQL.
func TestAuditSQLDatabaseRulesDoNotFireOnPostgreSQL(t *testing.T) {
	t.Parallel()
	databaseRuleIDs := []string{
		"ddl.database.create.notice",
		"ddl.database.drop.warn",
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE SCHEMA app",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	for _, stmt := range result.Statements {
		for _, f := range stmt.Findings {
			for _, dbID := range databaseRuleIDs {
				if f.RuleID == dbID {
					t.Fatalf("database rule %q fired on PostgreSQL", dbID)
				}
			}
		}
	}
}

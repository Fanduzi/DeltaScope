//go:build postgresql

package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLPostgreSQLExtensionLifecycleRules(t *testing.T) {
	t.Parallel()
	t.Run("create_extension_notice", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE EXTENSION pg_trgm;"
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
			if f.RuleID == "ddl.pg.create_extension.notice" {
				found = true
				if f.Level != rule.LevelNotice {
					t.Errorf("expected notice, got %s", f.Level)
				}
				if f.Metadata["operation"] != "create_extension" {
					t.Errorf("expected operation=create_extension, got %v", f.Metadata["operation"])
				}
				if f.Metadata["object_name"] != "pg_trgm" {
					t.Errorf("expected object_name=pg_trgm, got %v", f.Metadata["object_name"])
				}
				if f.Metadata["object_type"] != "extension" {
					t.Errorf("expected object_type=extension, got %v", f.Metadata["object_type"])
				}
			}
		}
		if !found {
			t.Fatalf("expected create_extension.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("create_extension_if_not_exists", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE EXTENSION IF NOT EXISTS pg_trgm;"
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

		var found bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.create_extension.notice" {
				found = true
				if f.Metadata["if_not_exists"] != "true" {
					t.Errorf("expected if_not_exists=true, got %v", f.Metadata["if_not_exists"])
				}
			}
		}
		if !found {
			t.Fatalf("expected create_extension.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("create_extension_with_schema", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE EXTENSION pg_trgm WITH SCHEMA public;"
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

		var found bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.create_extension.notice" {
				found = true
				if f.Metadata["schema"] != "public" {
					t.Errorf("expected schema=public, got %v", f.Metadata["schema"])
				}
			}
		}
		if !found {
			t.Fatalf("expected create_extension.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("create_extension_with_version", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE EXTENSION pg_trgm WITH VERSION '1.6';"
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

		var found bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.create_extension.notice" {
				found = true
				if f.Metadata["version"] != "1.6" {
					t.Errorf("expected version=1.6, got %v", f.Metadata["version"])
				}
			}
		}
		if !found {
			t.Fatalf("expected create_extension.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("create_extension_cascade", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE EXTENSION pg_trgm WITH CASCADE;"
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

		var foundNotice, foundCascade bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.create_extension.notice" {
				foundNotice = true
			}
			if f.RuleID == "ddl.pg.create_extension.cascade.warn" {
				foundCascade = true
				if f.Level != rule.LevelWarning {
					t.Errorf("cascade: expected warning, got %s", f.Level)
				}
				if f.Metadata["cascade"] != "true" {
					t.Errorf("cascade: expected cascade=true, got %v", f.Metadata["cascade"])
				}
			}
		}
		if !foundNotice {
			t.Fatalf("expected create_extension.notice, got %v", collectAuditResultRuleIDs(result))
		}
		if !foundCascade {
			t.Fatalf("expected create_extension.cascade.warn, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_extension_update_no_version", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER EXTENSION pg_trgm UPDATE;"
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
			if f.RuleID == "ddl.pg.alter_extension.update.notice" {
				found = true
				if f.Level != rule.LevelNotice {
					t.Errorf("expected notice, got %s", f.Level)
				}
				if f.Metadata["operation"] != "alter_extension" {
					t.Errorf("expected operation=alter_extension, got %v", f.Metadata["operation"])
				}
				if f.Metadata["action"] != "update" {
					t.Errorf("expected action=update, got %v", f.Metadata["action"])
				}
				if f.Metadata["object_name"] != "pg_trgm" {
					t.Errorf("expected object_name=pg_trgm, got %v", f.Metadata["object_name"])
				}
			}
		}
		if !found {
			t.Fatalf("expected alter_extension.update.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_extension_update_to_version", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER EXTENSION pg_trgm UPDATE TO '1.6';"
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

		var found bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.alter_extension.update.notice" {
				found = true
				if f.Metadata["action"] != "update" {
					t.Errorf("expected action=update, got %v", f.Metadata["action"])
				}
				if f.Metadata["version"] != "1.6" {
					t.Errorf("expected version=1.6, got %v", f.Metadata["version"])
				}
			}
		}
		if !found {
			t.Fatalf("expected alter_extension.update.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_extension_set_schema", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER EXTENSION pg_trgm SET SCHEMA extensions;"
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
			if f.RuleID == "ddl.pg.alter_extension.set_schema.notice" {
				found = true
				if f.Level != rule.LevelNotice {
					t.Errorf("expected notice, got %s", f.Level)
				}
				if f.Metadata["operation"] != "alter_extension" {
					t.Errorf("expected operation=alter_extension, got %v", f.Metadata["operation"])
				}
				if f.Metadata["action"] != "set_schema" {
					t.Errorf("expected action=set_schema, got %v", f.Metadata["action"])
				}
				if f.Metadata["new_schema"] != "extensions" {
					t.Errorf("expected new_schema=extensions, got %v", f.Metadata["new_schema"])
				}
				if f.Metadata["object_name"] != "pg_trgm" {
					t.Errorf("expected object_name=pg_trgm, got %v", f.Metadata["object_name"])
				}
			}
		}
		if !found {
			t.Fatalf("expected alter_extension.set_schema.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("drop_extension_advisory", func(t *testing.T) {
		t.Parallel()
		const sql = "DROP EXTENSION pg_trgm;"
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
			if f.RuleID == "ddl.pg.drop_extension.advisory" {
				found = true
				if f.Level != rule.LevelWarning {
					t.Errorf("expected warning, got %s", f.Level)
				}
				if f.Metadata["operation"] != "drop_extension" {
					t.Errorf("expected operation=drop_extension, got %v", f.Metadata["operation"])
				}
				if f.Metadata["object_name"] != "pg_trgm" {
					t.Errorf("expected object_name=pg_trgm, got %v", f.Metadata["object_name"])
				}
			}
		}
		if !found {
			t.Fatalf("expected drop_extension.advisory, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("drop_extension_if_exists_cascade", func(t *testing.T) {
		t.Parallel()
		const sql = "DROP EXTENSION IF EXISTS pg_trgm CASCADE;"
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

		var foundAdvisory, foundCascade bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.drop_extension.advisory" {
				foundAdvisory = true
				if f.Metadata["if_exists"] != "true" {
					t.Errorf("advisory: expected if_exists=true, got %v", f.Metadata["if_exists"])
				}
			}
			if f.RuleID == "ddl.pg.drop_extension.cascade.warn" {
				foundCascade = true
				if f.Level != rule.LevelWarning {
					t.Errorf("cascade: expected warning, got %s", f.Level)
				}
				if f.Metadata["cascade"] != "true" {
					t.Errorf("cascade: expected cascade=true, got %v", f.Metadata["cascade"])
				}
				if f.Metadata["if_exists"] != "true" {
					t.Errorf("cascade: expected if_exists=true, got %v", f.Metadata["if_exists"])
				}
			}
		}
		if !foundAdvisory {
			t.Fatalf("expected drop_extension.advisory, got %v", collectAuditResultRuleIDs(result))
		}
		if !foundCascade {
			t.Fatalf("expected drop_extension.cascade.warn, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("deferred_member_mutation_unsupported", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name string
			sql  string
		}{
			{"add_member", "ALTER EXTENSION pg_trgm ADD TABLE users"},
			{"drop_member", "ALTER EXTENSION pg_trgm DROP TABLE users"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				result, err := AuditSQL(context.Background(), Request{
					SQL:     tc.sql,
					Dialect: spec.DialectPostgreSQL,
				})
				if !errors.Is(err, ErrUnsupportedStatement) {
					t.Fatalf("expected unsupported statement sentinel, got %v", err)
				}
				if len(result.Unsupported) == 0 {
					t.Fatalf("expected unsupported for deferred member mutation")
				}
			})
		}
	})
}

func TestAuditSQLPostgreSQLTablePrivilegeDCLRules(t *testing.T) {
	t.Parallel()
	t.Run("grant_select_notice", func(t *testing.T) {
		t.Parallel()
		const sql = "GRANT SELECT ON TABLE users TO analyst;"
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
			if f.RuleID == "ddl.pg.grant.table_privilege.notice" {
				found = true
				if f.Level != rule.LevelNotice {
					t.Errorf("expected notice, got %s", f.Level)
				}
				if f.Metadata["operation"] != "grant_table" {
					t.Errorf("expected operation=grant_table, got %v", f.Metadata["operation"])
				}
				if f.Metadata["object_name"] != "users" {
					t.Errorf("expected object_name=users, got %v", f.Metadata["object_name"])
				}
			}
		}
		if !found {
			t.Fatalf("expected grant.table_privilege.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("grant_multi_privilege_grantee", func(t *testing.T) {
		t.Parallel()
		const sql = "GRANT SELECT, INSERT ON TABLE orders TO analyst, reporter;"
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

		var found bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.grant.table_privilege.notice" {
				found = true
				if f.Metadata["privileges"] != "select,insert" {
					t.Errorf("expected privileges=select,insert, got %v", f.Metadata["privileges"])
				}
				if f.Metadata["grantees"] != "analyst,reporter" {
					t.Errorf("expected grantees=analyst,reporter, got %v", f.Metadata["grantees"])
				}
			}
		}
		if !found {
			t.Fatalf("expected grant.table_privilege.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("grant_schema_qualified", func(t *testing.T) {
		t.Parallel()
		const sql = "GRANT SELECT ON TABLE public.users TO analyst;"
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

		var found bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.grant.table_privilege.notice" {
				found = true
				if f.Metadata["schema"] != "public" {
					t.Errorf("expected schema=public, got %v", f.Metadata["schema"])
				}
				if f.Metadata["object_name"] != "users" {
					t.Errorf("expected object_name=users, got %v", f.Metadata["object_name"])
				}
			}
		}
		if !found {
			t.Fatalf("expected grant.table_privilege.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("grant_all_privileges", func(t *testing.T) {
		t.Parallel()
		const sql = "GRANT ALL PRIVILEGES ON TABLE users TO analyst;"
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

		var foundNotice, foundAllWarn bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.grant.table_privilege.notice" {
				foundNotice = true
			}
			if f.RuleID == "ddl.pg.grant.table_privilege.all.warn" {
				foundAllWarn = true
				if f.Level != rule.LevelWarning {
					t.Errorf("all.warn: expected warning, got %s", f.Level)
				}
				if f.Metadata["all_privileges"] != "true" {
					t.Errorf("all.warn: expected all_privileges=true, got %v", f.Metadata["all_privileges"])
				}
			}
		}
		if !foundNotice {
			t.Fatalf("expected grant.table_privilege.notice, got %v", collectAuditResultRuleIDs(result))
		}
		if !foundAllWarn {
			t.Fatalf("expected grant.table_privilege.all.warn, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("revoke_select_notice", func(t *testing.T) {
		t.Parallel()
		const sql = "REVOKE SELECT ON TABLE users FROM analyst;"
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
			if f.RuleID == "ddl.pg.revoke.table_privilege.notice" {
				found = true
				if f.Level != rule.LevelNotice {
					t.Errorf("expected notice, got %s", f.Level)
				}
				if f.Metadata["operation"] != "revoke_table" {
					t.Errorf("expected operation=revoke_table, got %v", f.Metadata["operation"])
				}
				if f.Metadata["object_name"] != "users" {
					t.Errorf("expected object_name=users, got %v", f.Metadata["object_name"])
				}
			}
		}
		if !found {
			t.Fatalf("expected revoke.table_privilege.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("revoke_multi_privilege_grantee", func(t *testing.T) {
		t.Parallel()
		const sql = "REVOKE SELECT, DELETE ON TABLE orders FROM analyst, reporter;"
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

		var found bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.revoke.table_privilege.notice" {
				found = true
				if f.Metadata["privileges"] != "select,delete" {
					t.Errorf("expected privileges=select,delete, got %v", f.Metadata["privileges"])
				}
				if f.Metadata["grantees"] != "analyst,reporter" {
					t.Errorf("expected grantees=analyst,reporter, got %v", f.Metadata["grantees"])
				}
			}
		}
		if !found {
			t.Fatalf("expected revoke.table_privilege.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("revoke_all_cascade", func(t *testing.T) {
		t.Parallel()
		const sql = "REVOKE ALL PRIVILEGES ON TABLE users FROM analyst CASCADE;"
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

		var foundNotice, foundCascadeWarn bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.revoke.table_privilege.notice" {
				foundNotice = true
			}
			if f.RuleID == "ddl.pg.revoke.table_privilege.cascade.warn" {
				foundCascadeWarn = true
				if f.Level != rule.LevelWarning {
					t.Errorf("cascade.warn: expected warning, got %s", f.Level)
				}
				if f.Metadata["cascade"] != "true" {
					t.Errorf("cascade.warn: expected cascade=true, got %v", f.Metadata["cascade"])
				}
			}
		}
		if !foundNotice {
			t.Fatalf("expected revoke.table_privilege.notice, got %v", collectAuditResultRuleIDs(result))
		}
		if !foundCascadeWarn {
			t.Fatalf("expected revoke.table_privilege.cascade.warn, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("deferred_governance_dcl_unsupported", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name string
			sql  string
		}{
			{"grant_role", "GRANT analyst TO reporter"},
			{"revoke_role", "REVOKE analyst FROM reporter"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				result, err := AuditSQL(context.Background(), Request{
					SQL:     tc.sql,
					Dialect: spec.DialectPostgreSQL,
				})
				if !errors.Is(err, ErrUnsupportedStatement) {
					t.Fatalf("expected unsupported statement sentinel, got %v", err)
				}
				if len(result.Unsupported) == 0 {
					t.Fatalf("expected unsupported for deferred governance DCL")
				}
			})
		}
	})
}

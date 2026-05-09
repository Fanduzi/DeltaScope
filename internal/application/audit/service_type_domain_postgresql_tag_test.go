//go:build postgresql

package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLPostgreSQLTypeLifecycleRules(t *testing.T) {
	t.Parallel()
	t.Run("create_type_enum_notice", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE TYPE color AS ENUM ('red', 'green', 'blue');"
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
			if f.RuleID == "ddl.pg.create_type.enum.notice" {
				found = true
				if f.Level != rule.LevelNotice {
					t.Errorf("expected notice, got %s", f.Level)
				}
				if f.Metadata["operation"] != "create_type" {
					t.Errorf("expected operation=create_type, got %v", f.Metadata["operation"])
				}
				if f.Metadata["object_type"] != "type" {
					t.Errorf("expected object_type=type, got %v", f.Metadata["object_type"])
				}
				if f.Metadata["object_name"] != "color" {
					t.Errorf("expected object_name=color, got %v", f.Metadata["object_name"])
				}
				if f.Metadata["type_kind"] != "enum" {
					t.Errorf("expected type_kind=enum, got %v", f.Metadata["type_kind"])
				}
			}
		}
		if !found {
			t.Fatalf("expected create_type.enum.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_type_add_value_advisory", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TYPE color ADD VALUE 'yellow';"
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
			if f.RuleID == "ddl.pg.alter_type.add_value.advisory" {
				found = true
				if f.Level != rule.LevelWarning {
					t.Errorf("expected warning, got %s", f.Level)
				}
				if f.Metadata["operation"] != "alter_type" {
					t.Errorf("expected operation=alter_type, got %v", f.Metadata["operation"])
				}
				if f.Metadata["action"] != "add_value" {
					t.Errorf("expected action=add_value, got %v", f.Metadata["action"])
				}
				if f.Metadata["value"] != "yellow" {
					t.Errorf("expected value=yellow, got %v", f.Metadata["value"])
				}
			}
		}
		if !found {
			t.Fatalf("expected add_value.advisory, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_type_add_value_position_notice", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TYPE color ADD VALUE 'yellow' AFTER 'green';"
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

		var foundAdvisory, foundPosition bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.alter_type.add_value.advisory" {
				foundAdvisory = true
			}
			if f.RuleID == "ddl.pg.alter_type.add_value.position.notice" {
				foundPosition = true
				if f.Level != rule.LevelNotice {
					t.Errorf("expected notice, got %s", f.Level)
				}
				if f.Metadata["placement"] != "after" {
					t.Errorf("expected placement=after, got %v", f.Metadata["placement"])
				}
				if f.Metadata["neighbor"] != "green" {
					t.Errorf("expected neighbor=green, got %v", f.Metadata["neighbor"])
				}
			}
		}
		if !foundAdvisory {
			t.Fatalf("expected add_value.advisory, got %v", collectAuditResultRuleIDs(result))
		}
		if !foundPosition {
			t.Fatalf("expected add_value.position.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("drop_type_advisory", func(t *testing.T) {
		t.Parallel()
		const sql = "DROP TYPE color;"
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
			if f.RuleID == "ddl.pg.drop_type.advisory" {
				found = true
				if f.Level != rule.LevelWarning {
					t.Errorf("expected warning, got %s", f.Level)
				}
				if f.Metadata["operation"] != "drop_type" {
					t.Errorf("expected operation=drop_type, got %v", f.Metadata["operation"])
				}
				if f.Metadata["object_name"] != "color" {
					t.Errorf("expected object_name=color, got %v", f.Metadata["object_name"])
				}
			}
		}
		if !found {
			t.Fatalf("expected drop_type.advisory, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("drop_type_cascade_warn", func(t *testing.T) {
		t.Parallel()
		const sql = "DROP TYPE IF EXISTS color CASCADE;"
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

		var foundAdvisory, foundCascade bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.drop_type.advisory" {
				foundAdvisory = true
			}
			if f.RuleID == "ddl.pg.drop_type.cascade.warn" {
				foundCascade = true
				if f.Level != rule.LevelWarning {
					t.Errorf("expected warning, got %s", f.Level)
				}
				if f.Metadata["cascade"] != "true" {
					t.Errorf("expected cascade=true, got %v", f.Metadata["cascade"])
				}
				if f.Metadata["if_exists"] != "true" {
					t.Errorf("expected if_exists=true, got %v", f.Metadata["if_exists"])
				}
			}
		}
		if !foundAdvisory {
			t.Fatalf("expected drop_type.advisory, got %v", collectAuditResultRuleIDs(result))
		}
		if !foundCascade {
			t.Fatalf("expected drop_type.cascade.warn, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("create_type_composite_normalized", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE TYPE address AS (street text, city text);"

		// Verify service-level audit succeeds without unsupported details.
		result, err := AuditSQL(context.Background(), Request{
			SQL:     sql,
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Unsupported) != 0 {
			t.Fatalf("expected 0 unsupported, got %d", len(result.Unsupported))
		}

		// Extract the underlying spec to prove extractor-level facts.
		stmt, ok := corpusExtractStatement(t, sql, spec.DialectPostgreSQL)
		if !ok {
			t.Fatal("expected supported statement")
		}
		if stmt.DDL == nil {
			t.Fatalf("expected normalized DDL, got nil")
		}
		if stmt.DDL.Operation != spec.DDLOperationCreateType {
			t.Fatalf("expected create_type, got %q", stmt.DDL.Operation)
		}
		if stmt.DDL.ObjectName != "address" {
			t.Fatalf("expected object name address, got %q", stmt.DDL.ObjectName)
		}
		if stmt.DDL.Options["type_kind"] != "composite" {
			t.Fatalf("expected type_kind=composite, got %q", stmt.DDL.Options["type_kind"])
		}
		if stmt.DDL.Options["attributes"] != "2" {
			t.Fatalf("expected attributes=2, got %q", stmt.DDL.Options["attributes"])
		}
		if stmt.DDL.Options["attribute_names"] != "street,city" {
			t.Fatalf("expected attribute_names=street,city, got %q", stmt.DDL.Options["attribute_names"])
		}
	})

	t.Run("create_domain_normalized", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE DOMAIN email AS text CHECK (VALUE <> '');"

		// Verify service-level audit succeeds without unsupported details.
		result, err := AuditSQL(context.Background(), Request{
			SQL:     sql,
			Dialect: spec.DialectPostgreSQL,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Unsupported) != 0 {
			t.Fatalf("expected 0 unsupported, got %d", len(result.Unsupported))
		}

		// Extract the underlying spec to prove extractor-level facts.
		stmt, ok := corpusExtractStatement(t, sql, spec.DialectPostgreSQL)
		if !ok {
			t.Fatal("expected supported statement")
		}
		if stmt.DDL == nil {
			t.Fatalf("expected normalized DDL, got nil")
		}
		if stmt.DDL.Operation != spec.DDLOperationCreateDomain {
			t.Fatalf("expected create_domain, got %q", stmt.DDL.Operation)
		}
		if stmt.DDL.ObjectName != "email" {
			t.Fatalf("expected object name email, got %q", stmt.DDL.ObjectName)
		}
		if stmt.DDL.ObjectType != "domain" {
			t.Fatalf("expected object type domain, got %q", stmt.DDL.ObjectType)
		}
	})
}

func TestAuditSQLPostgreSQLCompositeTypeLifecycleRules(t *testing.T) {
	t.Parallel()
	t.Run("create_type_composite_notice", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE TYPE address AS (street text, city text);"
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
			if f.RuleID == "ddl.pg.create_type.composite.notice" {
				found = true
				if f.Level != rule.LevelNotice {
					t.Errorf("expected notice, got %s", f.Level)
				}
				if f.Metadata["operation"] != "create_type" {
					t.Errorf("expected operation=create_type, got %v", f.Metadata["operation"])
				}
				if f.Metadata["object_name"] != "address" {
					t.Errorf("expected object_name=address, got %v", f.Metadata["object_name"])
				}
				if f.Metadata["type_kind"] != "composite" {
					t.Errorf("expected type_kind=composite, got %v", f.Metadata["type_kind"])
				}
				if f.Metadata["attributes"] != "2" {
					t.Errorf("expected attributes=2, got %v", f.Metadata["attributes"])
				}
				if f.Metadata["attribute_names"] != "street,city" {
					t.Errorf("expected attribute_names=street,city, got %v", f.Metadata["attribute_names"])
				}
			}
		}
		if !found {
			t.Fatalf("expected create_type.composite.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("create_type_composite_qualified_schema", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE TYPE qualified.address AS (street text, city text);"
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
			if f.RuleID == "ddl.pg.create_type.composite.notice" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected create_type.composite.notice, got %v", collectAuditResultRuleIDs(result))
		}

		stmt, ok := corpusExtractStatement(t, sql, spec.DialectPostgreSQL)
		if !ok {
			t.Fatal("expected supported statement")
		}
		if stmt.DDL.Options["schema"] != "qualified" {
			t.Fatalf("expected schema=qualified, got %q", stmt.DDL.Options["schema"])
		}
	})

	t.Run("alter_type_composite_rename_notice", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TYPE address RENAME TO mailing_address;"
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
			if f.RuleID == "ddl.pg.alter_type.composite_rename.notice" {
				found = true
				if f.Level != rule.LevelNotice {
					t.Errorf("expected notice, got %s", f.Level)
				}
				if f.Metadata["operation"] != "alter_type" {
					t.Errorf("expected operation=alter_type, got %v", f.Metadata["operation"])
				}
				if f.Metadata["object_name"] != "address" {
					t.Errorf("expected object_name=address, got %v", f.Metadata["object_name"])
				}
				if f.Metadata["type_kind"] != "composite" {
					t.Errorf("expected type_kind=composite, got %v", f.Metadata["type_kind"])
				}
				if f.Metadata["action"] != "rename" {
					t.Errorf("expected action=rename, got %v", f.Metadata["action"])
				}
				if f.Metadata["new_name"] != "mailing_address" {
					t.Errorf("expected new_name=mailing_address, got %v", f.Metadata["new_name"])
				}
			}
		}
		if !found {
			t.Fatalf("expected alter_type.composite_rename.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_type_composite_set_schema_notice", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TYPE address SET SCHEMA archive;"
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
			if f.RuleID == "ddl.pg.alter_type.composite_set_schema.notice" {
				found = true
				if f.Level != rule.LevelNotice {
					t.Errorf("expected notice, got %s", f.Level)
				}
				if f.Metadata["operation"] != "alter_type" {
					t.Errorf("expected operation=alter_type, got %v", f.Metadata["operation"])
				}
				if f.Metadata["object_name"] != "address" {
					t.Errorf("expected object_name=address, got %v", f.Metadata["object_name"])
				}
				if f.Metadata["type_kind"] != "composite" {
					t.Errorf("expected type_kind=composite, got %v", f.Metadata["type_kind"])
				}
				if f.Metadata["action"] != "set_schema" {
					t.Errorf("expected action=set_schema, got %v", f.Metadata["action"])
				}
				if f.Metadata["new_schema"] != "archive" {
					t.Errorf("expected new_schema=archive, got %v", f.Metadata["new_schema"])
				}
			}
		}
		if !found {
			t.Fatalf("expected alter_type.composite_set_schema.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("drop_type_no_composite_rule_duplicate", func(t *testing.T) {
		t.Parallel()
		const sql = "DROP TYPE address;"
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

		var foundDropAdvisory bool
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == "ddl.pg.drop_type.advisory" {
				foundDropAdvisory = true
			}
			if f.RuleID == "ddl.pg.create_type.composite.notice" ||
				f.RuleID == "ddl.pg.alter_type.composite_rename.notice" ||
				f.RuleID == "ddl.pg.alter_type.composite_set_schema.notice" {
				t.Fatalf("composite rule %q must not fire for DROP TYPE", f.RuleID)
			}
		}
		if !foundDropAdvisory {
			t.Fatalf("expected drop_type.advisory, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("deferred_attribute_actions_unsupported", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name string
			sql  string
		}{
			{"add_attribute", "ALTER TYPE address ADD ATTRIBUTE country text"},
			{"drop_attribute", "ALTER TYPE address DROP ATTRIBUTE city"},
			{"alter_attribute_type", "ALTER TYPE address ALTER ATTRIBUTE street TYPE varchar(255)"},
			{"rename_attribute", "ALTER TYPE address RENAME ATTRIBUTE street TO line1"},
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
					t.Fatalf("expected unsupported for deferred attribute action")
				}
			})
		}
	})
}

func TestAuditSQLPostgreSQLDomainLifecycleRules(t *testing.T) {
	t.Parallel()
	t.Run("create_domain_with_check", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE DOMAIN email AS text CHECK (VALUE <> '');"
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
			if f.RuleID == "ddl.pg.create_domain.notice" {
				found = true
				if f.Level != rule.LevelNotice {
					t.Errorf("expected notice, got %s", f.Level)
				}
				if f.Metadata["operation"] != "create_domain" {
					t.Errorf("expected operation=create_domain, got %v", f.Metadata["operation"])
				}
				if f.Metadata["object_name"] != "email" {
					t.Errorf("expected object_name=email, got %v", f.Metadata["object_name"])
				}
				if f.Metadata["base_type"] != "text" {
					t.Errorf("expected base_type=text, got %v", f.Metadata["base_type"])
				}
				if f.Metadata["has_check"] != "true" {
					t.Errorf("expected has_check=true, got %v", f.Metadata["has_check"])
				}
			}
		}
		if !found {
			t.Fatalf("expected create_domain.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("create_domain_not_null_default_named_check", func(t *testing.T) {
		t.Parallel()
		const sql = "CREATE DOMAIN email AS text NOT NULL DEFAULT 'n/a' CONSTRAINT email_not_empty CHECK (VALUE <> '');"
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
			if f.RuleID == "ddl.pg.create_domain.notice" {
				found = true
				if f.Metadata["not_null"] != "true" {
					t.Errorf("expected not_null=true, got %v", f.Metadata["not_null"])
				}
				if f.Metadata["has_default"] != "true" {
					t.Errorf("expected has_default=true, got %v", f.Metadata["has_default"])
				}
				if f.Metadata["has_check"] != "true" {
					t.Errorf("expected has_check=true, got %v", f.Metadata["has_check"])
				}
				if f.Metadata["constraint"] != "email_not_empty" {
					t.Errorf("expected constraint=email_not_empty, got %v", f.Metadata["constraint"])
				}
				if f.Metadata["base_type"] != "text" {
					t.Errorf("expected base_type=text, got %v", f.Metadata["base_type"])
				}
			}
		}
		if !found {
			t.Fatalf("expected create_domain.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_domain_set_default", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER DOMAIN email SET DEFAULT 'unknown@example.com';"
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
			if f.RuleID == "ddl.pg.alter_domain.default.notice" {
				found = true
				if f.Metadata["action"] != "set_default" {
					t.Errorf("expected action=set_default, got %v", f.Metadata["action"])
				}
				if f.Metadata["has_default"] != "true" {
					t.Errorf("expected has_default=true, got %v", f.Metadata["has_default"])
				}
				if f.Metadata["object_name"] != "email" {
					t.Errorf("expected object_name=email, got %v", f.Metadata["object_name"])
				}
			}
		}
		if !found {
			t.Fatalf("expected alter_domain.default.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_domain_drop_default", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER DOMAIN email DROP DEFAULT;"
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
			if f.RuleID == "ddl.pg.alter_domain.default.notice" {
				found = true
				if f.Metadata["action"] != "drop_default" {
					t.Errorf("expected action=drop_default, got %v", f.Metadata["action"])
				}
			}
		}
		if !found {
			t.Fatalf("expected alter_domain.default.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_domain_set_not_null", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER DOMAIN email SET NOT NULL;"
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
			if f.RuleID == "ddl.pg.alter_domain.not_null.notice" {
				found = true
				if f.Metadata["action"] != "set_not_null" {
					t.Errorf("expected action=set_not_null, got %v", f.Metadata["action"])
				}
				if f.Metadata["not_null"] != "true" {
					t.Errorf("expected not_null=true, got %v", f.Metadata["not_null"])
				}
				if f.Metadata["object_name"] != "email" {
					t.Errorf("expected object_name=email, got %v", f.Metadata["object_name"])
				}
			}
		}
		if !found {
			t.Fatalf("expected alter_domain.not_null.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_domain_drop_not_null", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER DOMAIN email DROP NOT NULL;"
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
			if f.RuleID == "ddl.pg.alter_domain.not_null.notice" {
				found = true
				if f.Metadata["action"] != "drop_not_null" {
					t.Errorf("expected action=drop_not_null, got %v", f.Metadata["action"])
				}
			}
		}
		if !found {
			t.Fatalf("expected alter_domain.not_null.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_domain_add_constraint", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER DOMAIN email ADD CONSTRAINT email_not_empty CHECK (VALUE <> '');"
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
			if f.RuleID == "ddl.pg.alter_domain.constraint.notice" {
				found = true
				if f.Metadata["action"] != "add_constraint" {
					t.Errorf("expected action=add_constraint, got %v", f.Metadata["action"])
				}
				if f.Metadata["has_check"] != "true" {
					t.Errorf("expected has_check=true, got %v", f.Metadata["has_check"])
				}
				if f.Metadata["object_name"] != "email" {
					t.Errorf("expected object_name=email, got %v", f.Metadata["object_name"])
				}
			}
		}
		if !found {
			t.Fatalf("expected alter_domain.constraint.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_domain_drop_constraint", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER DOMAIN email DROP CONSTRAINT email_not_empty;"
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
			if f.RuleID == "ddl.pg.alter_domain.constraint.notice" {
				found = true
				if f.Metadata["action"] != "drop_constraint" {
					t.Errorf("expected action=drop_constraint, got %v", f.Metadata["action"])
				}
				if f.Metadata["constraint"] != "email_not_empty" {
					t.Errorf("expected constraint=email_not_empty, got %v", f.Metadata["constraint"])
				}
			}
		}
		if !found {
			t.Fatalf("expected alter_domain.constraint.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_domain_validate_constraint", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER DOMAIN email VALIDATE CONSTRAINT email_not_empty;"
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
			if f.RuleID == "ddl.pg.alter_domain.constraint.notice" {
				found = true
				if f.Metadata["action"] != "validate_constraint" {
					t.Errorf("expected action=validate_constraint, got %v", f.Metadata["action"])
				}
				if f.Metadata["constraint"] != "email_not_empty" {
					t.Errorf("expected constraint=email_not_empty, got %v", f.Metadata["constraint"])
				}
			}
		}
		if !found {
			t.Fatalf("expected alter_domain.constraint.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("alter_domain_rename", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER DOMAIN email RENAME TO contact_email;"
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
			if f.RuleID == "ddl.pg.alter_domain.rename.notice" {
				found = true
				if f.Metadata["action"] != "rename" {
					t.Errorf("expected action=rename, got %v", f.Metadata["action"])
				}
				if f.Metadata["new_name"] != "contact_email" {
					t.Errorf("expected new_name=contact_email, got %v", f.Metadata["new_name"])
				}
				if f.Metadata["object_name"] != "email" {
					t.Errorf("expected object_name=email, got %v", f.Metadata["object_name"])
				}
			}
		}
		if !found {
			t.Fatalf("expected alter_domain.rename.notice, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("drop_domain", func(t *testing.T) {
		t.Parallel()
		const sql = "DROP DOMAIN email;"
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
			if f.RuleID == "ddl.pg.drop_domain.advisory" {
				found = true
				if f.Level != rule.LevelWarning {
					t.Errorf("expected warning, got %s", f.Level)
				}
				if f.Metadata["operation"] != "drop_domain" {
					t.Errorf("expected operation=drop_domain, got %v", f.Metadata["operation"])
				}
				if f.Metadata["object_name"] != "email" {
					t.Errorf("expected object_name=email, got %v", f.Metadata["object_name"])
				}
			}
		}
		if !found {
			t.Fatalf("expected drop_domain.advisory, got %v", collectAuditResultRuleIDs(result))
		}
	})

	t.Run("drop_domain_cascade_duplicate_findings", func(t *testing.T) {
		t.Parallel()
		const sql = "DROP DOMAIN IF EXISTS email CASCADE;"
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
			if f.RuleID == "ddl.pg.drop_domain.advisory" {
				foundAdvisory = true
				if f.Level != rule.LevelWarning {
					t.Errorf("advisory: expected warning, got %s", f.Level)
				}
				if f.Metadata["if_exists"] != "true" {
					t.Errorf("advisory: expected if_exists=true, got %v", f.Metadata["if_exists"])
				}
			}
			if f.RuleID == "ddl.pg.drop_domain.cascade.warn" {
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
			t.Fatalf("expected drop_domain.advisory, got %v", collectAuditResultRuleIDs(result))
		}
		if !foundCascade {
			t.Fatalf("expected drop_domain.cascade.warn, got %v", collectAuditResultRuleIDs(result))
		}
	})
}

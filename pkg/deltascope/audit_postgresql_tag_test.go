//go:build postgresql

package deltascope

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditReturnsUnsupportedSentinelAndPartialResultForPostgreSQL(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "alter table users rename column old_name to new_name; select 1;",
		Dialect: DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 supported statement result, got %#v", result.Statements)
	}
	if result.Statements[0].Kind != "ddl" {
		t.Fatalf("expected supported statement kind ddl, got %#v", result.Statements[0])
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", result.Unsupported)
	}
	if result.Unsupported[0].Feature != "select" {
		t.Fatalf("expected unsupported feature select, got %#v", result.Unsupported[0])
	}
	if result.Unsupported[0].Reason == "" {
		t.Fatalf("expected unsupported reason, got %#v", result.Unsupported[0])
	}
	if result.Unsupported[0].Index != 1 {
		t.Fatalf("expected unsupported statement index 1, got %#v", result.Unsupported[0])
	}
}

func TestAuditPostgreSQLOnConflictDoesNotReturnMySQLSpecificFinding(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "insert into users(id, name) values (1, 'a') on conflict (id) do update set name = excluded.name;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "dml.insert.on_duplicate.forbid" {
			t.Fatalf("expected no on-duplicate finding for postgresql on conflict, got %#v", finding)
		}
		if finding.RuleID == "dml.insert.select.forbid" {
			t.Fatalf("expected no insert-select finding for values-based postgresql insert, got %#v", finding)
		}
		if strings.Contains(finding.Message, "ON DUPLICATE KEY") {
			t.Fatalf("expected no MySQL-specific finding message, got %#v", finding)
		}
	}
}

func TestAuditPostgreSQLInsertSelectOnConflictKeepsInsertSelectFindingOnly(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "insert into users(id, name) select id, name from staging_users on conflict (id) do update set name = excluded.name;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	foundInsertSelect := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "dml.insert.on_duplicate.forbid" {
			t.Fatalf("expected no on-duplicate finding for postgresql on conflict, got %#v", finding)
		}
		if finding.RuleID == "dml.insert.select.forbid" {
			foundInsertSelect = true
		}
		if strings.Contains(finding.Message, "ON DUPLICATE KEY") {
			t.Fatalf("expected no MySQL-specific finding message, got %#v", finding)
		}
	}
	if !foundInsertSelect {
		t.Fatal("expected insert-select finding for postgresql insert-select on conflict")
	}
}

func TestAuditPostgreSQLCreateViewEmitsDefaultForbidFinding(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "create view public.active_users as select id from public.users;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.view.create.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected create-view forbid finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLDropViewEmitsDefaultForbidFinding(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "drop view if exists public.active_users;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.view.drop.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop-view forbid finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLMetadataDoesNotLoadTableSnapshotForCreateView(t *testing.T) {
	provider := &fakeMetadataProvider{}

	result, err := Audit(context.Background(), Request{
		SQL:              "create view public.active_users as select id from public.users;",
		Dialect:          DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.tableCalls) != 0 {
		t.Fatalf("expected no table-snapshot calls for create view, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.view.create.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected create-view forbid finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLMetadataDoesNotLoadTableSnapshotForDropView(t *testing.T) {
	provider := &fakeMetadataProvider{}

	result, err := Audit(context.Background(), Request{
		SQL:              "drop view if exists public.active_users;",
		Dialect:          DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.tableCalls) != 0 {
		t.Fatalf("expected no table-snapshot calls for drop view, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.view.drop.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop-view forbid finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLMetadataMapsDropConstraintToPrimaryKeyRule(t *testing.T) {
	provider := &fakeMetadataProvider{
		snapshot: &TableSnapshot{
			Exists:      true,
			Table:       &Table{Name: "users"},
			PrimaryKey:  &Index{Name: "users_primary_idx", Kind: "primary", Columns: []string{"id"}},
			Constraints: []Constraint{{Type: "primary_key", Name: "users_pkey", Columns: []string{"id"}}},
		},
	}

	result, err := Audit(context.Background(), Request{
		SQL:              "alter table users drop constraint users_pkey;",
		Dialect:          DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one table-snapshot call for users, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.drop_primary_key.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop primary key finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLMetadataRequiresExistingColumnForRenameColumn(t *testing.T) {
	provider := &fakeMetadataProvider{
		snapshot: &TableSnapshot{
			Exists: true,
			Table:  &Table{Name: "users"},
			Columns: []Column{
				{Name: "email"},
			},
		},
	}

	result, err := Audit(context.Background(), Request{
		SQL:              "alter table users rename column missing_email to email;",
		Dialect:          DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one table-snapshot call for users, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.rename_column.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected rename-column existence finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLMetadataRequiresExistingColumnForDropColumn(t *testing.T) {
	provider := &fakeMetadataProvider{
		snapshot: &TableSnapshot{
			Exists: true,
			Table:  &Table{Name: "users"},
			Columns: []Column{
				{Name: "email"},
			},
		},
	}

	result, err := Audit(context.Background(), Request{
		SQL:              "alter table users drop column missing_email;",
		Dialect:          DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one table-snapshot call for users, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.drop_column.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop-column existence finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLMetadataRequiresExistingTableForRenameTable(t *testing.T) {
	provider := &fakeMetadataProvider{
		snapshot: &TableSnapshot{
			Exists: false,
			Table:  &Table{Name: "users"},
		},
	}

	result, err := Audit(context.Background(), Request{
		SQL:              "alter table users rename to users_archive;",
		Dialect:          DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one table-snapshot call for users, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.table.exists.alter.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected alter-table existence finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLAlterColumnActionsMapToSemanticRules(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "alter table users alter column created_at set default now(), alter column updated_at drop default, alter column email set not null, alter column phone drop not null;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	counts := map[string]int{}
	for _, finding := range result.Statements[0].Findings {
		counts[finding.RuleID]++
	}
	if len(result.Statements[0].Findings) != 8 {
		t.Fatalf("expected exactly 8 alter-column findings, got %#v", result.Statements[0].Findings)
	}
	if counts["ddl.alter.set_default.explicit_default_change.forbid"] != 1 {
		t.Fatalf("expected set_default semantic finding, got %#v", result.Statements[0].Findings)
	}
	if counts["ddl.alter.drop_default.explicit_default_change.forbid"] != 1 {
		t.Fatalf("expected drop_default semantic finding, got %#v", result.Statements[0].Findings)
	}
	if counts["ddl.alter.set_not_null.explicit_nullability_change.forbid"] != 1 {
		t.Fatalf("expected set_not_null semantic finding, got %#v", result.Statements[0].Findings)
	}
	if counts["ddl.alter.drop_not_null.explicit_nullability_change.forbid"] != 1 {
		t.Fatalf("expected drop_not_null semantic finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLRenameIndexMapsToForbidRule(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "alter index idx_old rename to idx_new;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	if len(result.Statements[0].Findings) != 1 {
		t.Fatalf("expected exactly 1 rename_index finding, got %#v", result.Statements[0].Findings)
	}
	if result.Statements[0].Findings[0].RuleID != "ddl.alter.rename_index.forbid" {
		t.Fatalf("expected rename_index forbid finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLMetadataResolvesOwningTableForRenameIndex(t *testing.T) {
	provider := &fakeMetadataProvider{
		indexTable: "users",
		snapshot: &TableSnapshot{
			Exists:  true,
			Table:   &Table{Name: "users"},
			Indexes: []Index{{Name: "idx_users_email", Kind: "secondary"}},
		},
	}

	result, err := Audit(context.Background(), Request{
		SQL:              "alter index missing_idx rename to idx_new;",
		Dialect:          DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.indexCalls) != 1 || provider.indexCalls[0] != "missing_idx" {
		t.Fatalf("expected one index-owner resolution for missing_idx, got %#v", provider.indexCalls)
	}
	if len(provider.indexSchemas) != 1 || provider.indexSchemas[0] != "public" {
		t.Fatalf("expected public schema for index-owner resolution, got %#v", provider.indexSchemas)
	}
	if len(provider.indexDialects) != 1 || provider.indexDialects[0] != DialectPostgreSQL {
		t.Fatalf("expected postgresql dialect for index-owner resolution, got %#v", provider.indexDialects)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one table-snapshot call for users, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.rename_index.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected rename_index existence finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLMetadataResolvesQualifiedRenameIndexWithoutRequestSchema(t *testing.T) {
	provider := &fakeMetadataProvider{
		indexTable: "users",
		snapshot: &TableSnapshot{
			Exists:  true,
			Table:   &Table{Name: "users"},
			Indexes: []Index{{Name: "idx_users_email", Kind: "secondary"}},
		},
	}

	result, err := Audit(context.Background(), Request{
		SQL:              "alter index accounting.missing_idx rename to idx_new;",
		Dialect:          DialectPostgreSQL,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.indexCalls) != 1 || provider.indexCalls[0] != "missing_idx" {
		t.Fatalf("expected one index-owner resolution for missing_idx, got %#v", provider.indexCalls)
	}
	if len(provider.indexSchemas) != 1 || provider.indexSchemas[0] != "accounting" {
		t.Fatalf("expected accounting schema for index-owner resolution, got %#v", provider.indexSchemas)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one table-snapshot call for users, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
}
func TestAuditPostgreSQLMetadataResolvesOwningTableForDropIndex(t *testing.T) {
	provider := &fakeMetadataProvider{
		indexTable: "users",
		snapshot: &TableSnapshot{
			Exists:  true,
			Table:   &Table{Name: "users"},
			Indexes: []Index{{Name: "idx_users_email", Kind: "secondary"}},
		},
	}

	result, err := Audit(context.Background(), Request{
		SQL:              "drop index missing_idx;",
		Dialect:          DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.indexCalls) != 1 || provider.indexCalls[0] != "missing_idx" {
		t.Fatalf("expected one index-owner resolution for missing_idx, got %#v", provider.indexCalls)
	}
	if len(provider.indexSchemas) != 1 || provider.indexSchemas[0] != "public" {
		t.Fatalf("expected public schema for index-owner resolution, got %#v", provider.indexSchemas)
	}
	if len(provider.indexDialects) != 1 || provider.indexDialects[0] != DialectPostgreSQL {
		t.Fatalf("expected postgresql dialect for index-owner resolution, got %#v", provider.indexDialects)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one table-snapshot call for users, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.drop_index.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop_index existence finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLValidateConstraintWithoutPrimaryKeyFinding(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "alter table users validate constraint chk_amount;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	if result.Statements[0].Kind != "ddl" {
		t.Fatalf("expected ddl kind, got %q", result.Statements[0].Kind)
	}
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.drop_primary_key.forbid" {
			t.Fatalf("validate_constraint should not trigger drop_primary_key finding, got %#v", finding)
		}
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("expected no unsupported entries, got %#v", result.Unsupported)
	}
}

func TestAuditPostgreSQLDropNonPrimaryKeyConstraintDoesNotTriggerPrimaryKeyFinding(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "alter table users drop constraint chk_amount;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	if result.Statements[0].Kind != "ddl" {
		t.Fatalf("expected ddl kind, got %q", result.Statements[0].Kind)
	}
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.drop_primary_key.forbid" {
			t.Fatalf("expected no drop_primary_key finding for non-PK constraint, got %#v", finding)
		}
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("expected no unsupported entries, got %#v", result.Unsupported)
	}
}

func TestAuditPostgreSQLAlterColumnSetDefaultRendersForbidFinding(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "alter table users alter column status set default 'active';",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.set_default.explicit_default_change.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected set_default semantic finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLAlterColumnDropNotNullRendersForbidFinding(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "alter table users alter column status drop not null;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.drop_not_null.explicit_nullability_change.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop_not_null semantic finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLSetDataTypeMapsToForbidRule(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "alter table users alter column status type bigint;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	counts := make(map[string]int)
	for _, finding := range result.Statements[0].Findings {
		counts[finding.RuleID]++
	}
	if counts["ddl.alter.set_data_type.forbid"] != 1 {
		t.Fatalf("expected set_data_type forbid finding, got %#v", result.Statements[0].Findings)
	}
	if counts["ddl.pg.alter.set_data_type.rewrite.warn"] != 1 {
		t.Fatalf("expected pg set_data_type rewrite warning, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLCreateTableConstraintsReturnNormalResult(t *testing.T) {
	cases := map[string]string{
		"named table-level CHECK":        "create table orders (id bigint primary key, amount numeric, constraint chk_orders_amount check (amount > 0));",
		"column-level inline CHECK":      "create table orders (id bigint primary key, amount numeric check (amount > 0));",
		"named table-level UNIQUE":       "create table users (id bigint primary key, email text, constraint uq_users_email unique (email));",
		"column-level inline UNIQUE":     "create table users (id bigint primary key, email text unique);",
		"named table-level FOREIGN KEY":  "create table orders (id bigint primary key, user_id bigint, constraint fk_orders_user foreign key (user_id) references users(id));",
		"column-level inline REFERENCES": "create table orders (id bigint primary key, user_id bigint references users(id));",
	}

	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     sql,
				Dialect: DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement result, got %#v", result.Statements)
			}
			if result.Statements[0].Kind != "ddl" {
				t.Fatalf("expected ddl kind, got %q", result.Statements[0].Kind)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected no unsupported entries, got %#v", result.Unsupported)
			}
		})
	}
}

func TestAuditPostgreSQLCreateTableForeignKeyRendersForbidFinding(t *testing.T) {
	cases := map[string]string{
		"named FOREIGN KEY": "create table orders (id bigint primary key, user_id bigint, constraint bad_fk foreign key (user_id) references users(id));",
		"inline REFERENCES": "create table orders (id bigint primary key, user_id bigint references users(id));",
	}

	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     sql,
				Dialect: DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement result, got %#v", result.Statements)
			}
			if result.Statements[0].Kind != "ddl" {
				t.Fatalf("expected ddl kind, got %q", result.Statements[0].Kind)
			}
			found := false
			for _, finding := range result.Statements[0].Findings {
				if finding.RuleID == "ddl.table.foreign_key.forbid" {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected foreign_key forbid finding, got %#v", result.Statements[0].Findings)
			}
		})
	}
}

func TestAuditPostgreSQLSchemaQualifiedReferencesReturnSupportedFKFindings(t *testing.T) {
	cases := map[string]struct {
		sql            string
		wantConstraint string
	}{
		"inline REFERENCES public.users": {
			sql: "create table orders (id bigint primary key, user_id bigint references public.users(id));",
		},
		"named FK REFERENCES public.users": {
			sql:            "create table orders (id bigint primary key, approver_id bigint, constraint fk_orders_approver foreign key (approver_id) references public.users(id));",
			wantConstraint: "fk_orders_approver",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     tc.sql,
				Dialect: DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement result, got %#v", result.Statements)
			}
			if result.Statements[0].Kind != "ddl" {
				t.Fatalf("expected ddl kind, got %q", result.Statements[0].Kind)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected no unsupported entries, got %#v", result.Unsupported)
			}

			found := false
			for _, finding := range result.Statements[0].Findings {
				if finding.RuleID == "ddl.table.foreign_key.forbid" {
					found = true
					if tc.wantConstraint != "" {
						if finding.Metadata["constraint"] != tc.wantConstraint {
							t.Errorf("expected constraint %q, got %q", tc.wantConstraint, finding.Metadata["constraint"])
						}
					}
					// Schema-qualified reference must not concatenate into "public.users".
					if refTable, _ := finding.Metadata["referenced_table"].(string); refTable == "public.users" {
						t.Fatalf("referenced_table must not be schema-qualified 'public.users', got %q", refTable)
					}
				}
			}
			if !found {
				t.Fatalf("expected foreign_key forbid finding, got %#v", result.Statements[0].Findings)
			}
		})
	}
}

func TestAuditPostgreSQLCreateOrReplaceViewReturnsUnsupported(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "create or replace view active_users as select id from users;",
		Dialect: DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", result.Unsupported)
	}
	if result.Unsupported[0].Feature != "create_view" {
		t.Fatalf("expected unsupported feature create_view, got %#v", result.Unsupported[0])
	}
	if result.Unsupported[0].Reason == "" {
		t.Fatalf("expected unsupported reason, got %#v", result.Unsupported[0])
	}
}

func TestAuditPostgreSQLCreateTablePartitioningReturnsUnsupported(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "create table orders (id bigint, created_at date) partition by range (created_at);",
		Dialect: DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", result.Unsupported)
	}
	if result.Unsupported[0].Feature != "partitioning" {
		t.Fatalf("expected unsupported feature partitioning, got %#v", result.Unsupported[0])
	}
	if result.Unsupported[0].Reason == "" {
		t.Fatalf("expected unsupported reason, got %#v", result.Unsupported[0])
	}
}

func TestAuditPostgreSQLCreateTableIdentityNarrowNowSupported(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE users (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, email text);",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("expected supported, got error: %v", err)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
}

func TestAuditPostgreSQLCreateTableGeneratedStoredNarrowNowSupported(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE users (first_name text, last_name text, full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED);",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("expected supported, got error: %v", err)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
}

func TestAuditPostgreSQLAlterTableGeneratedIdentityStateTransitionsNowSupported(t *testing.T) {
	cases := map[string]struct {
		sql string
	}{
		"drop generated expression": {
			sql: `ALTER TABLE users
  ALTER COLUMN full_name DROP EXPRESSION;`,
		},
		"set identity generated always": {
			sql: `ALTER TABLE users
  ALTER COLUMN id SET GENERATED ALWAYS;`,
		},
		"set identity generated by default": {
			sql: `ALTER TABLE users
  ALTER COLUMN id SET GENERATED BY DEFAULT;`,
		},
		"drop identity": {
			sql: `ALTER TABLE users
  ALTER COLUMN id DROP IDENTITY;`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     tc.sql,
				Dialect: DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("expected supported %s, got error: %v", name, err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected no unsupported entries, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected one statement result, got %#v", result.Statements)
			}
			if result.Statements[0].Kind != "ddl" {
				t.Fatalf("expected ddl kind, got %q", result.Statements[0].Kind)
			}
		})
	}
}

func TestAuditPostgreSQLAlterTableAddGeneratedIdentityNarrowNowSupported(t *testing.T) {
	cases := map[string]struct {
		sql string
	}{
		"generated stored add-column": {
			sql: "ALTER TABLE users ADD COLUMN full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED;",
		},
		"identity add-column": {
			sql: "ALTER TABLE users ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     tc.sql,
				Dialect: DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("expected supported, got error: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}
		})
	}
}

func TestAuditPostgreSQLCreateTableExclusionReturnsUnsupported(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE bookings (room_id int, during tsrange, EXCLUDE USING gist (room_id WITH =, during WITH &&));",
		Dialect: DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", result.Unsupported)
	}
	if result.Unsupported[0].Feature != "exclusion_constraint" {
		t.Fatalf("expected unsupported feature exclusion_constraint, got %#v", result.Unsupported[0])
	}
	if result.Unsupported[0].Reason == "" {
		t.Fatalf("expected unsupported reason, got %#v", result.Unsupported[0])
	}
}

func TestAuditPostgreSQLSchemaQualifiedForeignKeyExposesReferencedObjectMetadata(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE orders (id bigint PRIMARY KEY, approver_id bigint, CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES public.users(id));",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.table.foreign_key.forbid" {
			found = true
			if finding.Metadata["table"] == nil {
				t.Errorf("expected metadata key 'table', got nil")
			}
			if finding.Metadata["constraint"] == nil {
				t.Errorf("expected metadata key 'constraint', got nil")
			}
			if finding.Metadata["columns"] == nil {
				t.Errorf("expected metadata key 'columns', got nil")
			}
			// v0.28.0: referenced-object metadata is now exposed.
			if finding.Metadata["referenced_schema"] != "public" {
				t.Errorf("expected referenced_schema %q, got %v", "public", finding.Metadata["referenced_schema"])
			}
			if finding.Metadata["referenced_table"] != "users" {
				t.Errorf("expected referenced_table %q, got %v", "users", finding.Metadata["referenced_table"])
			}
			if finding.Metadata["referenced_columns"] == nil {
				t.Errorf("expected metadata key 'referenced_columns', got nil")
			}
		}
	}
	if !found {
		t.Fatalf("expected foreign_key forbid finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLSchemaQualifiedFKExposesReferencedObjectMetadata(t *testing.T) {
	cases := map[string]struct {
		sql            string
		wantConstraint string
		wantColumns    []string
		wantRefSchema  string
		wantRefTable   string
		wantRefColumns []string
	}{
		"inline REFERENCES public.users": {
			sql:            "create table orders (id bigint primary key, user_id bigint references public.users(id));",
			wantRefSchema:  "public",
			wantRefTable:   "users",
			wantRefColumns: []string{"id"},
		},
		"named FK REFERENCES public.users": {
			sql:            "create table orders (id bigint primary key, approver_id bigint, constraint fk_orders_approver foreign key (approver_id) references public.users(id));",
			wantConstraint: "fk_orders_approver",
			wantColumns:    []string{"approver_id"},
			wantRefSchema:  "public",
			wantRefTable:   "users",
			wantRefColumns: []string{"id"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     tc.sql,
				Dialect: DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement result, got %#v", result.Statements)
			}

			found := false
			for _, finding := range result.Statements[0].Findings {
				if finding.RuleID == "ddl.table.foreign_key.forbid" {
					found = true

					// Existing metadata.
					if tc.wantConstraint != "" && finding.Metadata["constraint"] != tc.wantConstraint {
						t.Errorf("expected constraint %q, got %v", tc.wantConstraint, finding.Metadata["constraint"])
					}
					if tc.wantColumns != nil {
						cols, ok := finding.Metadata["columns"].([]string)
						if !ok || len(cols) != len(tc.wantColumns) || cols[0] != tc.wantColumns[0] {
							t.Errorf("expected columns %v, got %v", tc.wantColumns, finding.Metadata["columns"])
						}
					}

					// Referenced-object metadata.
					if finding.Metadata["referenced_schema"] != tc.wantRefSchema {
						t.Errorf("expected referenced_schema %q, got %v", tc.wantRefSchema, finding.Metadata["referenced_schema"])
					}
					if finding.Metadata["referenced_table"] != tc.wantRefTable {
						t.Errorf("expected referenced_table %q, got %v", tc.wantRefTable, finding.Metadata["referenced_table"])
					}
					refCols, ok := finding.Metadata["referenced_columns"].([]string)
					if !ok || len(refCols) != len(tc.wantRefColumns) || refCols[0] != tc.wantRefColumns[0] {
						t.Errorf("expected referenced_columns %v, got %v", tc.wantRefColumns, finding.Metadata["referenced_columns"])
					}

					// referenced_table must NOT be schema-qualified.
					if finding.Metadata["referenced_table"] == "public.users" {
						t.Fatalf("referenced_table must not be schema-qualified 'public.users'")
					}
				}
			}
			if !found {
				t.Fatalf("expected foreign_key forbid finding, got %#v", result.Statements[0].Findings)
			}
		})
	}
}

func TestAuditPostgreSQLCrossSchemaFKRendersAdvisoryNotice(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE public.orders (id bigint PRIMARY KEY, approver_id bigint, CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES auth.users(id));",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	advisoryFound := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.pg.table.foreign_key.cross_schema.advisory" {
			advisoryFound = true
			if finding.Level != "notice" {
				t.Errorf("expected advisory level notice, got %q", finding.Level)
			}
			if finding.Metadata["table_schema"] != "public" {
				t.Errorf("expected table_schema public, got %v", finding.Metadata["table_schema"])
			}
			if finding.Metadata["referenced_schema"] != "auth" {
				t.Errorf("expected referenced_schema auth, got %v", finding.Metadata["referenced_schema"])
			}
			if finding.Metadata["referenced_table"] != "users" {
				t.Errorf("expected referenced_table users, got %v", finding.Metadata["referenced_table"])
			}
			refCols, _ := finding.Metadata["referenced_columns"].([]string)
			if len(refCols) < 1 || refCols[0] != "id" {
				t.Errorf("expected referenced_columns [id], got %v", refCols)
			}
			if finding.Metadata["referenced_table"] == "auth.users" {
				t.Fatalf("referenced_table must not be schema-qualified 'auth.users'")
			}
		}
	}
	if !advisoryFound {
		t.Fatalf("expected cross-schema advisory finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLSameSchemaFKDoesNotRenderAdvisory(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE public.orders (id bigint PRIMARY KEY, user_id bigint, CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES public.users(id));",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.pg.table.foreign_key.cross_schema.advisory" {
			t.Fatalf("expected no cross-schema advisory for same-schema FK, got %#v", finding)
		}
	}
}

func TestAuditPostgreSQLBareFKDoesNotRenderAdvisory(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE public.orders (id bigint PRIMARY KEY, approver_id bigint REFERENCES users(id));",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.pg.table.foreign_key.cross_schema.advisory" {
			t.Fatalf("expected no cross-schema advisory for bare FK reference, got %#v", finding)
		}
	}
}

// TestAuditPostgreSQLGeneratedIdentityNarrowNowSupported proves that narrow
// generated/identity forms are now processed through the normal supported path.
// Each case asserts: no error, no unsupported details, and exactly one statement result.
// Fact preservation (generated_when, is_identity, identity_options) is verified at the
// service/corpus level; the public SDK surface confirms the supported-path contract.
func TestAuditPostgreSQLGeneratedIdentityNarrowNowSupported(t *testing.T) {
	cases := map[string]struct {
		sql string
	}{
		"generated_stored_column": {
			sql: `CREATE TABLE t (first_name text, full_name text GENERATED ALWAYS AS (first_name) STORED);`,
		},
		"generated_always_as_identity": {
			sql: `CREATE TABLE t (id bigint GENERATED ALWAYS AS IDENTITY);`,
		},
		"generated_by_default_identity_with_options": {
			sql: `CREATE TABLE t (id bigint GENERATED BY DEFAULT AS IDENTITY (START WITH 10 INCREMENT BY 5 CACHE 20 CYCLE));`,
		},
		"alter_table_add_generated_column": {
			sql: `ALTER TABLE t ADD COLUMN full_name text GENERATED ALWAYS AS (first_name) STORED;`,
		},
		"alter_table_add_identity_column": {
			sql: `ALTER TABLE t ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     tc.sql,
				Dialect: DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("expected supported, got error: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}
		})
	}
}

// TestAuditPostgreSQLGeneratedIdentityRuleCoverage locks the three PG-only
// generated/identity state-transition forbid rules at the public SDK surface.
// For each case it asserts: supported path, no unsupported, correct rule ID.
func TestAuditPostgreSQLGeneratedIdentityRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "drop_expression",
			sql:        "ALTER TABLE users ALTER COLUMN full_name DROP EXPRESSION;",
			wantRuleID: "ddl.alter.drop_expression.forbid",
		},
		{
			name:       "set_generated_by_default",
			sql:        "ALTER TABLE users ALTER COLUMN id SET GENERATED BY DEFAULT;",
			wantRuleID: "ddl.alter.set_generated.forbid",
		},
		{
			name:       "set_generated_always",
			sql:        "ALTER TABLE users ALTER COLUMN id SET GENERATED ALWAYS;",
			wantRuleID: "ddl.alter.set_generated.forbid",
		},
		{
			name:       "drop_identity",
			sql:        "ALTER TABLE users ALTER COLUMN id DROP IDENTITY;",
			wantRuleID: "ddl.alter.drop_identity.forbid",
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

// pkgMetadataValueEqual compares values with numeric type coercion.
func TestAuditPostgreSQLPrimaryKeyRuleCoverage(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE bad_pk_type (id integer PRIMARY KEY, name text);",
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
		if f.RuleID == "ddl.table.primary_key.bigint.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding with rule_id ddl.table.primary_key.bigint.require, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLUniqueIndexRuleCoverage(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE UNIQUE INDEX bad_email_unique ON users (email);",
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
		if f.RuleID == "ddl.index.unique.prefix.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding with rule_id ddl.index.unique.prefix.require, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLAlterTableAddConstraintRuleCoverage(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "ALTER TABLE users ADD CONSTRAINT bad_email_key UNIQUE (email);",
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
		if f.RuleID == "ddl.alter.add_index.unique.prefix.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding with rule_id ddl.alter.add_index.unique.prefix.require, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLAlterTableForeignKeyRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "forbid only",
			sql:        "ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id);",
			wantRuleID: "ddl.table.foreign_key.forbid",
		},
		{
			name:       "cross_schema advisory",
			sql:        "ALTER TABLE public.orders ADD CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES auth.users(id);",
			wantRuleID: "ddl.pg.table.foreign_key.cross_schema.advisory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
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
				if f.RuleID == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule_id %s, got %#v", tt.wantRuleID, result.Statements[0].Findings)
			}
		})
	}
}

func TestAuditPostgreSQLAlterTableAddConstraintCheckRuleCoverage(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(configPath, []byte("rules:\n  ddl.constraint.check.name.prefix.require:\n    enabled: true\n    params:\n      prefix: ck_\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := Audit(context.Background(), Request{
		SQL:        "ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0);",
		Dialect:    DialectPostgreSQL,
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("expected postgresql request to succeed, got %v", err)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("expected no unsupported entries, got %#v", result.Unsupported)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", result.Statements)
	}
	if result.Statements[0].Kind != "ddl" {
		t.Fatalf("expected ddl kind, got %q", result.Statements[0].Kind)
	}

	wantRuleIDs := map[string]bool{
		"ddl.constraint.check.name.prefix.require": false,
		"ddl.pg.alter.add_check.not_valid.require": false,
	}
	for _, f := range result.Statements[0].Findings {
		if _, expected := wantRuleIDs[f.RuleID]; expected {
			wantRuleIDs[f.RuleID] = true
		}
	}
	for ruleID, found := range wantRuleIDs {
		if !found {
			t.Fatalf("expected finding with rule_id %s, got %#v", ruleID, result.Statements[0].Findings)
		}
	}
}

func TestAuditPostgreSQLNotValidConstraintValidationRuleCoverage(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("expected postgresql request to succeed, got %v", err)
	}
	if len(result.GlobalFindings) == 0 {
		t.Fatalf("expected at least one global finding, got none")
	}
	found := false
	for _, f := range result.GlobalFindings {
		if f.RuleID == "ddl.pg.alter.not_valid_constraint.validate.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected global finding with rule_id ddl.pg.alter.not_valid_constraint.validate.require, got %#v", result.GlobalFindings)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("expected no unsupported entries, got %#v", result.Unsupported)
	}
}

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

func pkgMetadataValueEqual(a, b any) bool {
	aFloat, aIsNum := pkgToFloat64(a)
	bFloat, bIsNum := pkgToFloat64(b)
	if aIsNum && bIsNum {
		return aFloat == bFloat
	}
	return a == b
}

func pkgToFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
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

func TestAuditPostgreSQLObjectLifecycleRuleCoverage(t *testing.T) {
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

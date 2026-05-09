//go:build postgresql

package deltascope

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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

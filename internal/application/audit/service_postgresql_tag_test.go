//go:build postgresql

package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// --- v0.29.0 Task 1: Schema-Aware FK Policy Characterization Tests ---
//
// These three tests characterize the current state of FK facts and rule
// behavior for the schema-aware FK policy decision gate (v0.29.0 Task 1).
//
// They are NOT TDD tests driving new production behavior. They lock in
// observable facts about what the current extractor/rule path provides,
// so that Task 2 has stable decision inputs.

// TestAuditSQLPostgreSQLSchemaQualifiedForeignKeyFactsAvailableForPolicy
// proves that the current rule/service path can observe all four
// schema-aware FK facts: owning table schema, referenced_schema,
// referenced_table, and referenced_columns.
//
// This establishes that the schema-aware policy limitation is NOT a
// data-availability problem — the extractor already preserves these facts.
func TestAuditSQLPostgreSQLSchemaQualifiedForeignKeyFactsAvailableForPolicy(t *testing.T) {
	const sql = "CREATE TABLE public.orders (id bigint PRIMARY KEY, approver_id bigint, CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES auth.users(id));"

	result, err := AuditSQL(context.Background(), Request{
		SQL:     sql,
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	// Extract the underlying spec to prove extractor-level facts.
	stmt, ok := corpusExtractStatement(t, sql, spec.DialectPostgreSQL)
	if !ok {
		t.Fatal("expected supported statement")
	}
	if stmt.DDL == nil {
		t.Fatal("expected DDL payload")
	}

	// Owning table schema must be "public" (from CREATE TABLE public.orders).
	if stmt.DDL.Table == nil {
		t.Fatal("expected table payload")
	}
	if stmt.DDL.Table.Schema != "public" {
		t.Errorf("expected owning table schema %q, got %q", "public", stmt.DDL.Table.Schema)
	}
	if stmt.DDL.Table.Name != "orders" {
		t.Errorf("expected owning table name %q, got %q", "orders", stmt.DDL.Table.Name)
	}

	// FK constraint must carry explicit cross-schema referenced-object facts.
	found := false
	for _, c := range stmt.DDL.Constraints {
		if c.Type == "foreign_key" && c.Name == "fk_orders_approver" {
			found = true

			// referenced_schema must be "auth" (from REFERENCES auth.users).
			if c.ReferencedSchema != "auth" {
				t.Errorf("expected ReferencedSchema %q, got %q", "auth", c.ReferencedSchema)
			}
			// referenced_table must be "users" — never "auth.users".
			if c.ReferencedTable != "users" {
				t.Errorf("expected ReferencedTable %q, got %q", "users", c.ReferencedTable)
			}
			// referenced_columns must be ["id"].
			if len(c.ReferencedColumns) != 1 || c.ReferencedColumns[0] != "id" {
				t.Errorf("expected ReferencedColumns [id], got %v", c.ReferencedColumns)
			}
			// owning columns must be ["approver_id"].
			if len(c.Columns) != 1 || c.Columns[0] != "approver_id" {
				t.Errorf("expected Columns [approver_id], got %v", c.Columns)
			}
		}
	}
	if !found {
		t.Fatalf("expected named FK constraint fk_orders_approver in %+v", stmt.DDL.Constraints)
	}

	// The FK forbid finding metadata must also expose referenced_schema = "auth".
	findingFound := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.table.foreign_key.forbid" {
			findingFound = true
			if finding.Metadata["referenced_schema"] != "auth" {
				t.Errorf("expected finding referenced_schema %q, got %v", "auth", finding.Metadata["referenced_schema"])
			}
			if finding.Metadata["referenced_table"] != "users" {
				t.Errorf("expected finding referenced_table %q, got %v", "users", finding.Metadata["referenced_table"])
			}
		}
	}
	if !findingFound {
		t.Fatalf("expected foreign_key forbid finding with cross-schema metadata, got %#v", result.Statements[0].Findings)
	}
}

// TestAuditSQLPostgreSQLBareForeignKeyReferenceLeavesSchemaUnknown
// proves that bare REFERENCES users(id) — without a schema qualifier —
// leaves referenced_schema empty at both extractor and rule level.
//
// This is evidence for the "no search_path inference" policy: bare
// references cannot be treated as "public.users" because DeltaScope
// does not model PostgreSQL search_path semantics.
func TestAuditSQLPostgreSQLBareForeignKeyReferenceLeavesSchemaUnknown(t *testing.T) {
	const sql = "CREATE TABLE orders (id bigint PRIMARY KEY, approver_id bigint REFERENCES users(id));"

	result, err := AuditSQL(context.Background(), Request{
		SQL:     sql,
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	// Extract the underlying spec to prove extractor-level facts.
	stmt, ok := corpusExtractStatement(t, sql, spec.DialectPostgreSQL)
	if !ok {
		t.Fatal("expected supported statement")
	}
	if stmt.DDL == nil {
		t.Fatal("expected DDL payload")
	}

	// Owning table schema must be empty — bare CREATE TABLE has no schema.
	if stmt.DDL.Table == nil {
		t.Fatal("expected table payload")
	}
	if stmt.DDL.Table.Schema != "" {
		t.Errorf("expected owning table schema to be empty for bare CREATE TABLE, got %q", stmt.DDL.Table.Schema)
	}

	// FK constraint must NOT have referenced_schema populated.
	found := false
	for _, c := range stmt.DDL.Constraints {
		if c.Type == "foreign_key" {
			found = true

			// referenced_schema must be empty — bare REFERENCES has no schema qualifier.
			if c.ReferencedSchema != "" {
				t.Errorf("expected ReferencedSchema to be empty for bare REFERENCES, got %q", c.ReferencedSchema)
			}
			if c.ReferencedTable != "users" {
				t.Errorf("expected ReferencedTable %q, got %q", "users", c.ReferencedTable)
			}
			if len(c.ReferencedColumns) != 1 || c.ReferencedColumns[0] != "id" {
				t.Errorf("expected ReferencedColumns [id], got %v", c.ReferencedColumns)
			}
		}
	}
	if !found {
		t.Fatalf("expected FK constraint in %+v", stmt.DDL.Constraints)
	}

	// The FK forbid finding must NOT include referenced_schema.
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.table.foreign_key.forbid" {
			if v, exists := finding.Metadata["referenced_schema"]; exists && v != "" {
				t.Errorf("expected no referenced_schema in finding metadata for bare REFERENCES, got %v", v)
			}
		}
	}
}

// TestAuditSQLPostgreSQLForeignKeyRuleIsSchemaAgnosticToday
// proves that the current FK forbid rule fires identically regardless of
// whether the FK is schema-qualified or bare. All three cases produce
// exactly one ddl.table.foreign_key.forbid finding with the same level.
//
// This establishes that v0.29's challenge is a policy decision — whether
// to distinguish cross-schema from same-schema — not a metadata visibility
// issue.
func TestAuditSQLPostgreSQLForeignKeyRuleIsSchemaAgnosticToday(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		wantFindings  int
		wantRefSchema string // expected referenced_schema in finding metadata (empty string = absent)
		wantRuleID    string
	}{
		{
			name:          "cross_schema_fk_triggers_forbid_identically",
			sql:           "CREATE TABLE public.orders (id bigint PRIMARY KEY, approver_id bigint, CONSTRAINT fk_approver FOREIGN KEY (approver_id) REFERENCES auth.users(id));",
			wantFindings:  1,
			wantRefSchema: "auth",
			wantRuleID:    "ddl.table.foreign_key.forbid",
		},
		{
			name:          "same_schema_fk_triggers_forbid_identically",
			sql:           "CREATE TABLE public.orders (id bigint PRIMARY KEY, user_id bigint, CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES public.users(id));",
			wantFindings:  1,
			wantRefSchema: "public",
			wantRuleID:    "ddl.table.foreign_key.forbid",
		},
		{
			name:          "bare_fk_triggers_forbid_identically",
			sql:           "CREATE TABLE orders (id bigint PRIMARY KEY, user_id bigint REFERENCES users(id));",
			wantFindings:  1,
			wantRefSchema: "", // absent
			wantRuleID:    "ddl.table.foreign_key.forbid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: spec.DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement result, got %#v", result.Statements)
			}

			// Count FK forbid findings.
			count := 0
			for _, finding := range result.Statements[0].Findings {
				if finding.RuleID == tt.wantRuleID {
					count++

					// All three cases must produce the same level.
					if finding.Level != "blocker" {
						t.Errorf("expected level blocker, got %q", finding.Level)
					}

					// Verify referenced_schema matches expectation.
					if tt.wantRefSchema == "" {
						if v, exists := finding.Metadata["referenced_schema"]; exists && v != "" {
							t.Errorf("expected no referenced_schema, got %v", v)
						}
					} else {
						if finding.Metadata["referenced_schema"] != tt.wantRefSchema {
							t.Errorf("expected referenced_schema %q, got %v", tt.wantRefSchema, finding.Metadata["referenced_schema"])
						}
					}
				}
			}
			if count != tt.wantFindings {
				t.Fatalf("expected %d FK forbid findings, got %d", tt.wantFindings, count)
			}
		})
	}
}

func TestAuditSQLReturnsMixedSupportedAndUnsupportedPostgreSQLResults(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "alter table users rename column old_name to new_name; select 1;",
		Dialect: spec.DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 supported statement result, got %#v", result.Statements)
	}
	if result.Statements[0].Kind != spec.KindDDL.String() {
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

func TestAuditSQLPostgreSQLMetadataMapsDropConstraintToPrimaryKeyRule(t *testing.T) {
	provider := &fakeMetadataProvider{
		snapshot: &spec.TableSnapshot{
			Exists:      true,
			Table:       &spec.Table{Name: "users"},
			PrimaryKey:  &spec.Index{Name: "users_primary_idx", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
			Constraints: []spec.Constraint{{Type: "primary_key", Name: "users_pkey", Columns: []string{"id"}}},
		},
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:              "alter table users drop constraint users_pkey;",
		Dialect:          spec.DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one target-table metadata call for users, got %#v", provider.tableCalls)
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

func TestAuditSQLPostgreSQLMetadataRequiresExistingColumnForRenameColumn(t *testing.T) {
	provider := &fakeMetadataProvider{
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
			Columns: []spec.Column{
				{Name: "email"},
			},
		},
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:              "alter table users rename column missing_email to email;",
		Dialect:          spec.DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one target-table metadata call for users, got %#v", provider.tableCalls)
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

func TestAuditSQLPostgreSQLMetadataRequiresExistingColumnForDropColumn(t *testing.T) {
	provider := &fakeMetadataProvider{
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
			Columns: []spec.Column{
				{Name: "email"},
			},
		},
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:              "alter table users drop column missing_email;",
		Dialect:          spec.DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one target-table metadata call for users, got %#v", provider.tableCalls)
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

func TestAuditSQLPostgreSQLMetadataRequiresExistingTableForRenameTable(t *testing.T) {
	provider := &fakeMetadataProvider{
		snapshot: &spec.TableSnapshot{
			Exists: false,
			Table:  &spec.Table{Name: "users"},
		},
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:              "alter table users rename to users_archive;",
		Dialect:          spec.DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one target-table metadata call for users, got %#v", provider.tableCalls)
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

func TestAuditSQLPostgreSQLAlterColumnActionsMapToSemanticRules(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "alter table users alter column created_at set default now(), alter column updated_at drop default, alter column email set not null, alter column phone drop not null;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
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

func TestAuditSQLPostgreSQLSetDataTypeMapsToForbidRule(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "alter table users alter column status type bigint;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
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

func TestAuditSQLPostgreSQLRenameIndexMapsToForbidRule(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "alter index idx_old rename to idx_new;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
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

func TestAuditSQLPostgreSQLMetadataResolvesOwningTableForRenameIndex(t *testing.T) {
	provider := &fakeMetadataProvider{
		indexTable: "users",
		snapshot: &spec.TableSnapshot{
			Exists:  true,
			Table:   &spec.Table{Name: "users"},
			Indexes: []spec.Index{{Name: "idx_users_email", Kind: spec.IndexKindSecondary}},
		},
	}

	_, err := AuditSQL(context.Background(), Request{
		SQL:              "alter index missing_idx rename to idx_new;",
		Dialect:          spec.DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(provider.indexCalls) != 1 || provider.indexCalls[0] != "missing_idx" {
		t.Fatalf("expected one index-owner resolution for missing_idx, got %#v", provider.indexCalls)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one target-table metadata call for users, got %#v", provider.tableCalls)
	}
}

func TestAuditSQLPostgreSQLMetadataResolvesOwningTableForDropIndex(t *testing.T) {
	provider := &fakeMetadataProvider{
		indexTable: "users",
		snapshot: &spec.TableSnapshot{
			Exists:  true,
			Table:   &spec.Table{Name: "users"},
			Indexes: []spec.Index{{Name: "idx_users_email", Kind: spec.IndexKindSecondary}},
		},
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:              "drop index missing_idx;",
		Dialect:          spec.DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(provider.indexCalls) != 1 || provider.indexCalls[0] != "missing_idx" {
		t.Fatalf("expected one index-owner resolution for missing_idx, got %#v", provider.indexCalls)
	}
	if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "users" {
		t.Fatalf("expected one target-table metadata call for users, got %#v", provider.tableCalls)
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

func TestAuditSQLPostgreSQLDropNonPrimaryKeyConstraintDoesNotTriggerPrimaryKeyForbid(t *testing.T) {
	provider := &fakeMetadataProvider{
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
			Constraints: []spec.Constraint{
				{Type: "check", Name: "chk_amount", Columns: []string{"amount"}},
			},
		},
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:              "alter table users drop constraint chk_amount;",
		Dialect:          spec.DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.drop_primary_key.forbid" {
			t.Fatalf("expected no drop_primary_key finding for non-PK constraint, got %#v", finding)
		}
	}
}

func TestAuditSQLPostgreSQLValidateConstraintFlowsThroughPipeline(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "alter table users validate constraint chk_amount;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	if result.Statements[0].Kind != spec.KindDDL.String() {
		t.Fatalf("expected DDL kind, got %q", result.Statements[0].Kind)
	}

	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.drop_primary_key.forbid" {
			t.Fatalf("validate_constraint should not trigger drop_primary_key finding, got %#v", finding)
		}
	}
}

func TestAuditSQLPostgreSQLCreateTableForeignKeyRetainsSupportedResultWithReferencedObjectFacts(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "create table orders (user_id bigint, constraint bad_fk foreign key (user_id) references users(id));",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	if result.Statements[0].Kind != spec.KindDDL.String() {
		t.Fatalf("expected DDL kind, got %q", result.Statements[0].Kind)
	}

	// The default policy forbids foreign keys. The richer FK semantics
	// (ReferencedTable, ReferencedColumns) should not interfere with the
	// shared FK-forbid rule firing correctly.
	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.table.foreign_key.forbid" {
			found = true
			if finding.Metadata["constraint"] != "bad_fk" {
				t.Fatalf("expected constraint name bad_fk, got %#v", finding.Metadata)
			}
		}
		if finding.RuleID == "ddl.alter.drop_primary_key.forbid" {
			t.Fatalf("create-table FK should not trigger drop_primary_key finding, got %#v", finding)
		}
	}
	if !found {
		t.Fatalf("expected foreign_key forbid finding, got %#v", result.Statements[0].Findings)
	}

	// FK naming rules should be suppressed when FK-forbid is active.
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.constraint.foreign_key.name.prefix.require" {
			t.Fatalf("FK naming rules should be suppressed by FK-forbid, got %#v", finding)
		}
	}
}

func TestAuditSQLPostgreSQLCreateTableInlineReferencesRetainsSupportedResult(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "create table orders (user_id bigint references users(id));",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	if result.Statements[0].Kind != spec.KindDDL.String() {
		t.Fatalf("expected DDL kind, got %q", result.Statements[0].Kind)
	}

	// Inline REFERENCES produces an unnamed FK constraint. The default policy
	// forbids foreign keys, so ddl.table.foreign_key.forbid must fire. Naming
	// rules are suppressed by the forbid rule so they should not appear.
	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.table.foreign_key.forbid" {
			found = true
		}
		if finding.RuleID == "ddl.alter.drop_primary_key.forbid" {
			t.Fatalf("inline references should not trigger drop_primary_key finding, got %#v", finding)
		}
	}
	if !found {
		t.Fatalf("expected foreign_key forbid finding for inline REFERENCES, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditSQLPostgreSQLCreateOrReplaceViewReturnsUnsupported(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "create or replace view active_users as select id from users;",
		Dialect: spec.DialectPostgreSQL,
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

func TestAuditSQLPostgreSQLCreateTablePartitioningReturnsUnsupported(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "create table orders (id bigint, created_at date) partition by range (created_at);",
		Dialect: spec.DialectPostgreSQL,
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

func TestAuditSQLPostgreSQLInlineSchemaQualifiedReferencesPreservesReferencedSchemaFacts(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "create table orders (id bigint primary key, user_id bigint references public.users(id));",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	if result.Statements[0].Kind != spec.KindDDL.String() {
		t.Fatalf("expected DDL kind, got %q", result.Statements[0].Kind)
	}

	stmt, ok := corpusExtractStatement(t, "create table orders (id bigint primary key, user_id bigint references public.users(id));", spec.DialectPostgreSQL)
	if !ok {
		t.Fatal("expected supported statement")
	}
	if stmt.DDL == nil {
		t.Fatal("expected DDL payload")
	}

	found := false
	for _, c := range stmt.DDL.Constraints {
		if c.Type == "foreign_key" {
			found = true
			if c.ReferencedSchema != "public" {
				t.Errorf("expected ReferencedSchema %q, got %q", "public", c.ReferencedSchema)
			}
			if c.ReferencedTable != "users" {
				t.Errorf("expected ReferencedTable %q, got %q", "users", c.ReferencedTable)
			}
			if len(c.ReferencedColumns) != 1 || c.ReferencedColumns[0] != "id" {
				t.Errorf("expected ReferencedColumns [id], got %v", c.ReferencedColumns)
			}
		}
	}
	if !found {
		t.Fatalf("expected foreign_key constraint in %+v", stmt.DDL.Constraints)
	}
}

func TestAuditSQLPostgreSQLNamedSchemaQualifiedFKPreservesReferencedSchemaFacts(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "create table orders (id bigint primary key, approver_id bigint, constraint fk_orders_approver foreign key (approver_id) references public.users(id));",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	if result.Statements[0].Kind != spec.KindDDL.String() {
		t.Fatalf("expected DDL kind, got %q", result.Statements[0].Kind)
	}

	stmt, ok := corpusExtractStatement(t, "create table orders (id bigint primary key, approver_id bigint, constraint fk_orders_approver foreign key (approver_id) references public.users(id));", spec.DialectPostgreSQL)
	if !ok {
		t.Fatal("expected supported statement")
	}
	if stmt.DDL == nil {
		t.Fatal("expected DDL payload")
	}

	found := false
	for _, c := range stmt.DDL.Constraints {
		if c.Type == "foreign_key" && c.Name == "fk_orders_approver" {
			found = true
			if c.ReferencedSchema != "public" {
				t.Errorf("expected ReferencedSchema %q, got %q", "public", c.ReferencedSchema)
			}
			if c.ReferencedTable != "users" {
				t.Errorf("expected ReferencedTable %q, got %q", "users", c.ReferencedTable)
			}
			if len(c.ReferencedColumns) != 1 || c.ReferencedColumns[0] != "id" {
				t.Errorf("expected ReferencedColumns [id], got %v", c.ReferencedColumns)
			}
			if len(c.Columns) != 1 || c.Columns[0] != "approver_id" {
				t.Errorf("expected Columns [approver_id], got %v", c.Columns)
			}
		}
	}
	if !found {
		t.Fatalf("expected named foreign_key constraint fk_orders_approver in %+v", stmt.DDL.Constraints)
	}
}

func TestAuditSQLPostgreSQLSchemaQualifiedForeignKeyExposesReferencedObjectMetadata(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE TABLE orders (id bigint PRIMARY KEY, approver_id bigint, CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES public.users(id));",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
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

func TestAuditSQLPostgreSQLSchemaQualifiedInlineFKExposesReferencedObjectMetadata(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE TABLE orders (id bigint PRIMARY KEY, user_id bigint REFERENCES public.users(id));",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.table.foreign_key.forbid" {
			found = true

			// Existing metadata must remain.
			if finding.Metadata["table"] == nil {
				t.Errorf("expected metadata key 'table', got nil")
			}
			if finding.Metadata["columns"] == nil {
				t.Errorf("expected metadata key 'columns', got nil")
			}

			// Referenced-object metadata must now be exposed.
			if finding.Metadata["referenced_schema"] != "public" {
				t.Errorf("expected referenced_schema %q, got %v", "public", finding.Metadata["referenced_schema"])
			}
			if finding.Metadata["referenced_table"] != "users" {
				t.Errorf("expected referenced_table %q, got %v", "users", finding.Metadata["referenced_table"])
			}
			referencedColumns, ok := finding.Metadata["referenced_columns"].([]string)
			if !ok || len(referencedColumns) != 1 || referencedColumns[0] != "id" {
				t.Errorf("expected referenced_columns [id], got %v", finding.Metadata["referenced_columns"])
			}
		}
	}
	if !found {
		t.Fatalf("expected foreign_key forbid finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditSQLPostgreSQLSchemaQualifiedNamedFKExposesReferencedObjectMetadata(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE TABLE orders (id bigint PRIMARY KEY, approver_id bigint, CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES public.users(id));",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.table.foreign_key.forbid" {
			found = true

			// Existing metadata must remain.
			if finding.Metadata["table"] == nil {
				t.Errorf("expected metadata key 'table', got nil")
			}
			if finding.Metadata["constraint"] != "fk_orders_approver" {
				t.Errorf("expected constraint %q, got %v", "fk_orders_approver", finding.Metadata["constraint"])
			}
			columns, ok := finding.Metadata["columns"].([]string)
			if !ok || len(columns) != 1 || columns[0] != "approver_id" {
				t.Errorf("expected columns [approver_id], got %v", finding.Metadata["columns"])
			}

			// Referenced-object metadata must now be exposed.
			if finding.Metadata["referenced_schema"] != "public" {
				t.Errorf("expected referenced_schema %q, got %v", "public", finding.Metadata["referenced_schema"])
			}
			if finding.Metadata["referenced_table"] != "users" {
				t.Errorf("expected referenced_table %q, got %v", "users", finding.Metadata["referenced_table"])
			}
			referencedColumns, ok := finding.Metadata["referenced_columns"].([]string)
			if !ok || len(referencedColumns) != 1 || referencedColumns[0] != "id" {
				t.Errorf("expected referenced_columns [id], got %v", finding.Metadata["referenced_columns"])
			}

			// referenced_table must NOT be schema-qualified concat.
			if finding.Metadata["referenced_table"] == "public.users" {
				t.Fatalf("referenced_table must not be schema-qualified 'public.users'")
			}
		}
	}
	if !found {
		t.Fatalf("expected foreign_key forbid finding, got %#v", result.Statements[0].Findings)
	}
}

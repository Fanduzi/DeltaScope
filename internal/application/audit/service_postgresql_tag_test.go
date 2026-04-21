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

func TestAuditSQLPostgreSQLAlterTableAddGeneratedIdentityNarrowNowSupported(t *testing.T) {
	tests := []struct {
		name              string
		sql               string
		wantColumnName    string
		wantGeneratedWhen string
		wantIsIdentity    bool
	}{
		{
			name:              "add generated stored column",
			sql:               "ALTER TABLE users\n  ADD COLUMN full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED;",
			wantColumnName:    "full_name",
			wantGeneratedWhen: "a",
		},
		{
			name:              "add identity column",
			sql:               "ALTER TABLE users\n  ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;",
			wantColumnName:    "id",
			wantGeneratedWhen: "a",
			wantIsIdentity:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: spec.DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("expected supported, got error: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement result, got %d", len(result.Statements))
			}

			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}

			var col *spec.Column
			for i := range stmt.DDL.Alter {
				alt := &stmt.DDL.Alter[i]
				if alt.Column != nil && alt.Column.Definition != nil && alt.Column.Definition.Name == tt.wantColumnName {
					col = alt.Column.Definition
					break
				}
			}
			if col == nil {
				t.Fatalf("expected column %q not found in alter actions", tt.wantColumnName)
			}
			if col.GeneratedWhen != tt.wantGeneratedWhen {
				t.Fatalf("expected GeneratedWhen %q, got %q", tt.wantGeneratedWhen, col.GeneratedWhen)
			}
			if col.IsIdentity != tt.wantIsIdentity {
				t.Fatalf("expected IsIdentity %v, got %v", tt.wantIsIdentity, col.IsIdentity)
			}
		})
	}
}

func TestAuditSQLPostgreSQLGeneratedIdentityStateTransitionsNowSupported(t *testing.T) {
	tests := []struct {
		name              string
		sql               string
		wantAction        string
		wantColumnOldName string
		wantGeneratedWhen string
	}{
		{
			name: "drop generated expression",
			sql: `ALTER TABLE users
  ALTER COLUMN full_name DROP EXPRESSION;`,
			wantAction:        "drop_expression",
			wantColumnOldName: "full_name",
		},
		{
			name: "set identity generated by default",
			sql: `ALTER TABLE users
  ALTER COLUMN id SET GENERATED BY DEFAULT;`,
			wantAction:        "set_generated",
			wantColumnOldName: "id",
			wantGeneratedWhen: "d",
		},
		{
			name: "set identity generated always",
			sql: `ALTER TABLE users
  ALTER COLUMN id SET GENERATED ALWAYS;`,
			wantAction:        "set_generated",
			wantColumnOldName: "id",
			wantGeneratedWhen: "a",
		},
		{
			name: "drop identity",
			sql: `ALTER TABLE users
  ALTER COLUMN id DROP IDENTITY;`,
			wantAction:        "drop_identity",
			wantColumnOldName: "id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: spec.DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("expected supported statement, got %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported details, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 supported statement result, got %#v", result.Statements)
			}
			if result.Statements[0].Kind != spec.KindDDL.String() {
				t.Fatalf("expected supported statement kind ddl, got %#v", result.Statements[0])
			}
			if result.Statements[0].RawSQL == "" && result.Statements[0].NormalizedSQL == "" {
				t.Fatalf("expected supported statement payload in result, got %#v", result.Statements[0])
			}

			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}
			if stmt.DDL.Operation != spec.DDLOperationAlterTable {
				t.Fatalf("expected alter_table operation, got %q", stmt.DDL.Operation)
			}

			var matched *spec.Alter
			for i := range stmt.DDL.Alter {
				alter := &stmt.DDL.Alter[i]
				if alter.Action != tt.wantAction {
					continue
				}
				if alter.Column == nil {
					continue
				}
				if alter.Column.OldName != tt.wantColumnOldName {
					continue
				}
				matched = alter
				break
			}
			if matched == nil {
				t.Fatalf("expected alter action %q for column %q in %+v", tt.wantAction, tt.wantColumnOldName, stmt.DDL.Alter)
			}
			if matched.Column == nil {
				t.Fatal("expected alter column payload")
			}
			if matched.Column.OldName != tt.wantColumnOldName {
				t.Fatalf("expected column old_name %q, got %q", tt.wantColumnOldName, matched.Column.OldName)
			}
			if tt.wantGeneratedWhen != "" {
				if matched.Options["generated_when"] != tt.wantGeneratedWhen {
					t.Fatalf("expected options.generated_when %q, got %#v", tt.wantGeneratedWhen, matched.Options)
				}
			}
		})
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

// --- v0.29.0 Task 2: Cross-schema FK advisory service-level tests ---

// TestAuditSQLPostgreSQLCrossSchemaFKTriggersAdvisory proves that an explicit
// cross-schema FK on a PostgreSQL CREATE TABLE triggers the new advisory rule
// alongside the existing FK forbid rule.
func TestAuditSQLPostgreSQLCrossSchemaFKTriggersAdvisory(t *testing.T) {
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

	advisoryFound := false
	forbidFound := false
	for _, finding := range result.Statements[0].Findings {
		switch finding.RuleID {
		case "ddl.pg.table.foreign_key.cross_schema.advisory":
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
		case "ddl.table.foreign_key.forbid":
			forbidFound = true
		}
	}
	if !advisoryFound {
		t.Fatalf("expected cross-schema advisory finding, got %#v", result.Statements[0].Findings)
	}
	if !forbidFound {
		t.Fatalf("expected FK forbid finding to still be present, got %#v", result.Statements[0].Findings)
	}
}

// TestAuditSQLPostgreSQLSameSchemaFKDoesNotTriggerAdvisory proves that an
// explicit same-schema FK does not trigger the cross-schema advisory.
func TestAuditSQLPostgreSQLSameSchemaFKDoesNotTriggerAdvisory(t *testing.T) {
	const sql = "CREATE TABLE public.orders (id bigint PRIMARY KEY, user_id bigint, CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES public.users(id));"

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

	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.pg.table.foreign_key.cross_schema.advisory" {
			t.Fatalf("expected no cross-schema advisory for same-schema FK, got %#v", finding)
		}
	}
}

// TestAuditSQLPostgreSQLBareFKDoesNotTriggerAdvisory proves that a bare
// REFERENCES without schema qualifier does not trigger the cross-schema advisory.
func TestAuditSQLPostgreSQLBareFKDoesNotTriggerAdvisory(t *testing.T) {
	const sql = "CREATE TABLE public.orders (id bigint PRIMARY KEY, approver_id bigint REFERENCES users(id));"

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

	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.pg.table.foreign_key.cross_schema.advisory" {
			t.Fatalf("expected no cross-schema advisory for bare FK reference, got %#v", finding)
		}
	}
}

// ---------------------------------------------------------------------------
// v0.36.0 Task 3: Service tests — generated/identity rule coverage
// ---------------------------------------------------------------------------

// TestAuditSQLPostgreSQLGeneratedIdentityRuleCoverage proves that the three
// PG-only generated/identity state-transition forbid rules fire through the
// full AuditSQL pipeline and produce the correct rule IDs.
func TestAuditSQLPostgreSQLGeneratedIdentityRuleCoverage(t *testing.T) {
	tests := []struct {
		name              string
		sql               string
		wantRuleID        string
		wantAction        string
		wantGeneratedWhen string
	}{
		{
			name:              "drop_expression",
			sql:               "ALTER TABLE users ALTER COLUMN full_name DROP EXPRESSION;",
			wantRuleID:        "ddl.alter.drop_expression.forbid",
			wantAction:        "drop_expression",
		},
		{
			name:              "set_generated_by_default",
			sql:               "ALTER TABLE users ALTER COLUMN id SET GENERATED BY DEFAULT;",
			wantRuleID:        "ddl.alter.set_generated.forbid",
			wantAction:        "set_generated",
			wantGeneratedWhen: "d",
		},
		{
			name:              "set_generated_always",
			sql:               "ALTER TABLE users ALTER COLUMN id SET GENERATED ALWAYS;",
			wantRuleID:        "ddl.alter.set_generated.forbid",
			wantAction:        "set_generated",
			wantGeneratedWhen: "a",
		},
		{
			name:              "drop_identity",
			sql:               "ALTER TABLE users ALTER COLUMN id DROP IDENTITY;",
			wantRuleID:        "ddl.alter.drop_identity.forbid",
			wantAction:        "drop_identity",
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
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			// Must have the expected forbid rule finding.
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

			// Verify spec-level facts are still correct.
			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}
			var matched *spec.Alter
			for i := range stmt.DDL.Alter {
				if stmt.DDL.Alter[i].Action == tt.wantAction {
					matched = &stmt.DDL.Alter[i]
					break
				}
			}
			if matched == nil {
				t.Fatalf("expected alter action %q, got %+v", tt.wantAction, stmt.DDL.Alter)
			}
			if tt.wantGeneratedWhen != "" {
				if matched.Options["generated_when"] != tt.wantGeneratedWhen {
					t.Fatalf("expected generated_when %q, got %#v", tt.wantGeneratedWhen, matched.Options)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// v0.33.0 Task 3: Service tests — unsupported metadata surfacing
// ---------------------------------------------------------------------------

func TestAuditSQLPostgreSQLGeneratedIdentityNarrowNowSupported(t *testing.T) {
	tests := []struct {
		name              string
		sql               string
		wantColumnName    string
		wantGeneratedWhen string
		wantIsIdentity    bool
		wantIdentityOpts  map[string]any
		isAlter           bool
	}{
		{
			name:              "generated_stored_column",
			sql:               `CREATE TABLE t (first_name text, full_name text GENERATED ALWAYS AS (first_name) STORED);`,
			wantColumnName:    "full_name",
			wantGeneratedWhen: "a",
		},
		{
			name:              "generated_always_as_identity",
			sql:               `CREATE TABLE t (id bigint GENERATED ALWAYS AS IDENTITY);`,
			wantColumnName:    "id",
			wantGeneratedWhen: "a",
			wantIsIdentity:    true,
		},
		{
			name:              "generated_by_default_as_identity_with_options",
			sql:               `CREATE TABLE t (id bigint GENERATED BY DEFAULT AS IDENTITY (START WITH 10 INCREMENT BY 5 CACHE 20 CYCLE));`,
			wantColumnName:    "id",
			wantGeneratedWhen: "d",
			wantIsIdentity:    true,
			wantIdentityOpts: map[string]any{
				"start":     int32(10),
				"increment": int32(5),
				"cache":     int32(20),
				"cycle":     true,
			},
		},
		{
			name:              "alter_table_add_generated_column",
			sql:               `ALTER TABLE t ADD COLUMN full_name text GENERATED ALWAYS AS (first_name) STORED;`,
			wantColumnName:    "full_name",
			wantGeneratedWhen: "a",
			isAlter:           true,
		},
		{
			name:              "alter_table_add_identity_column",
			sql:               `ALTER TABLE t ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;`,
			wantColumnName:    "id",
			wantGeneratedWhen: "a",
			wantIsIdentity:    true,
			isAlter:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: spec.DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("expected supported, got error: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}

			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}

			var col *spec.Column
			if tt.isAlter {
				for i := range stmt.DDL.Alter {
					alt := &stmt.DDL.Alter[i]
					if alt.Column != nil && alt.Column.Definition != nil && alt.Column.Definition.Name == tt.wantColumnName {
						col = alt.Column.Definition
						break
					}
				}
			} else {
				for i := range stmt.DDL.Columns {
					if stmt.DDL.Columns[i].Name == tt.wantColumnName {
						col = &stmt.DDL.Columns[i]
						break
					}
				}
			}
			if col == nil {
				t.Fatalf("expected column %q not found", tt.wantColumnName)
			}

			if col.GeneratedWhen != tt.wantGeneratedWhen {
				t.Fatalf("expected GeneratedWhen %q, got %q", tt.wantGeneratedWhen, col.GeneratedWhen)
			}
			if col.IsIdentity != tt.wantIsIdentity {
				t.Fatalf("expected IsIdentity %v, got %v", tt.wantIsIdentity, col.IsIdentity)
			}

			if tt.wantIdentityOpts != nil {
				if col.IdentityOptions == nil {
					t.Fatal("expected IdentityOptions to be populated")
				}
				for key, wantVal := range tt.wantIdentityOpts {
					actVal, ok := col.IdentityOptions[key]
					if !ok {
						t.Fatalf("expected identity_options key %q, got %+v", key, col.IdentityOptions)
					}
					if !serviceMetadataValueEqual(wantVal, actVal) {
						t.Fatalf("identity_options[%q]: expected %v (%T), got %v (%T)", key, wantVal, wantVal, actVal, actVal)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// v0.37.0 Task 3: Service tests — primary-key rule coverage
// ---------------------------------------------------------------------------

// TestAuditSQLPostgreSQLPrimaryKeyRuleCoverage proves that the PostgreSQL
// extractor now populates DDL.PrimaryKey facts so existing primary-key rules
// can fire through the full AuditSQL pipeline.
func TestAuditSQLPostgreSQLPrimaryKeyRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "bigint_required_non_bigint_pk",
			sql:        "CREATE TABLE bad_pk_type (id integer PRIMARY KEY, name text);",
			wantRuleID: "ddl.table.primary_key.bigint.require",
		},
		{
			name:       "composite_pk_exceeds_max_columns",
			sql:        "CREATE TABLE composite_pk (tenant_id bigint, user_id bigint, PRIMARY KEY (tenant_id, user_id));",
			wantRuleID: "ddl.table.primary_key.columns.max_count",
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
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			// Must have the expected rule finding.
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

			// Verify spec-level primary-key facts are populated.
			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}
			if stmt.DDL.PrimaryKey == nil {
				t.Fatal("expected DDL.PrimaryKey to be populated")
			}
			if stmt.DDL.PrimaryKey.Kind != spec.IndexKindPrimary {
				t.Fatalf("expected primary key kind, got %q", stmt.DDL.PrimaryKey.Kind)
			}
			if len(stmt.DDL.PrimaryKey.Columns) == 0 {
				t.Fatal("expected at least one primary key column")
			}

			// Verify PK columns are marked NotNull.
			for _, pkCol := range stmt.DDL.PrimaryKey.Columns {
				foundCol := false
				for _, col := range stmt.DDL.Columns {
					if col.Name == pkCol {
						foundCol = true
						if !col.NotNull {
							t.Errorf("PK column %q should be NotNull=true", pkCol)
						}
					}
				}
				if !foundCol {
					t.Errorf("PK column %q not found in DDL.Columns", pkCol)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// v0.38.0 Task 3: Service tests — standalone CREATE INDEX rule coverage
// ---------------------------------------------------------------------------

// TestAuditSQLPostgreSQLUniqueIndexRuleCoverage proves that standalone
// CREATE INDEX and CREATE UNIQUE INDEX on PostgreSQL trigger generic index
// rules through the full AuditSQL pipeline.
func TestAuditSQLPostgreSQLUniqueIndexRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "secondary_prefix_on_create_index",
			sql:        "CREATE INDEX bad_users_email ON users (email);",
			wantRuleID: "ddl.index.secondary.prefix.require",
		},
		{
			name:       "unique_prefix_on_create_unique_index",
			sql:        "CREATE UNIQUE INDEX bad_users_email_unique ON users (email);",
			wantRuleID: "ddl.index.unique.prefix.require",
		},
		{
			name:       "columns_max_count_on_create_index",
			sql:        "CREATE INDEX idx_wide ON users (c1, c2, c3, c4, c5, c6, c7, c8, c9);",
			wantRuleID: "ddl.index.columns.max_count",
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
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule %q, got %#v", tt.wantRuleID, result.Statements[0].Findings)
			}

			// Verify spec-level index facts are populated.
			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}
			if stmt.DDL.Operation != spec.DDLOperationCreateIndex {
				t.Fatalf("expected create_index operation, got %q", stmt.DDL.Operation)
			}
			if len(stmt.DDL.Indexes) == 0 {
				t.Fatal("expected at least one index")
			}
		})
	}
}

func TestAuditSQLPostgreSQLAlterTableAddConstraintRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "primary_key_bigint_required",
			sql:        "ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (id);",
			wantRuleID: "ddl.table.primary_key.bigint.require",
		},
		{
			name:       "unique_prefix_required",
			sql:        "ALTER TABLE users ADD CONSTRAINT bad_email_key UNIQUE (email);",
			wantRuleID: "ddl.alter.add_index.unique.prefix.require",
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
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule %q, got %#v", tt.wantRuleID, result.Statements[0].Findings)
			}

			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}
			if stmt.DDL.Operation != spec.DDLOperationAlterTable {
				t.Fatalf("expected alter_table operation, got %q", stmt.DDL.Operation)
			}
			if len(stmt.DDL.Alter) != 1 {
				t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
			}
			alter := stmt.DDL.Alter[0]
			if alter.Action != "add_constraint" {
				t.Fatalf("expected add_constraint action, got %q", alter.Action)
			}
			if alter.Options["columns"] == "" {
				t.Fatal("expected columns option to be populated")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// v0.40.0 Task 3: Service tests — ALTER TABLE ADD CONSTRAINT FOREIGN KEY rule coverage
// ---------------------------------------------------------------------------

// TestAuditSQLPostgreSQLAlterTableForeignKeyRuleCoverage proves that ALTER
// TABLE ADD CONSTRAINT FOREIGN KEY triggers the FK forbid and cross-schema
// advisory rules through the full AuditSQL pipeline, with correct DDL/Alter
// facts preserved.
func TestAuditSQLPostgreSQLAlterTableForeignKeyRuleCoverage(t *testing.T) {
	tests := []struct {
		name                string
		sql                 string
		wantRuleID          string
		wantColumns         []string
		wantRefTable        string
		wantRefColumns      []string
		wantRefSchema       string
		wantConstraintType  string
		wantCrossSchemaRule bool
	}{
		{
			name:                "fk_forbid_bare_reference",
			sql:                 "ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id);",
			wantRuleID:          "ddl.table.foreign_key.forbid",
			wantColumns:         []string{"user_id"},
			wantRefTable:        "users",
			wantRefColumns:      []string{"id"},
			wantConstraintType:  "foreign_key",
			wantCrossSchemaRule: false,
		},
		{
			name:                "fk_cross_schema_advisory",
			sql:                 "ALTER TABLE public.orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES auth.users(id);",
			wantRuleID:          "ddl.table.foreign_key.forbid",
			wantColumns:         []string{"user_id"},
			wantRefTable:        "users",
			wantRefColumns:      []string{"id"},
			wantRefSchema:       "auth",
			wantConstraintType:  "foreign_key",
			wantCrossSchemaRule: true,
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
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			// Must have the FK forbid finding.
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

			// Cross-schema advisory check.
			if tt.wantCrossSchemaRule {
				advisoryFound := false
				for _, f := range result.Statements[0].Findings {
					if f.RuleID == "ddl.pg.table.foreign_key.cross_schema.advisory" {
						advisoryFound = true
						break
					}
				}
				if !advisoryFound {
					t.Fatalf("expected cross-schema advisory finding, got %#v", result.Statements[0].Findings)
				}
			}

			// Verify spec-level DDL/Alter facts.
			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}
			if stmt.DDL.Operation != spec.DDLOperationAlterTable {
				t.Fatalf("expected alter_table operation, got %q", stmt.DDL.Operation)
			}
			if len(stmt.DDL.Alter) != 1 {
				t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
			}
			alter := stmt.DDL.Alter[0]
			if alter.Action != "add_constraint" {
				t.Fatalf("expected add_constraint action, got %q", alter.Action)
			}
			if alter.Options["constraint_type"] != tt.wantConstraintType {
				t.Fatalf("expected constraint_type %q, got %q", tt.wantConstraintType, alter.Options["constraint_type"])
			}
			if alter.Options["columns"] == "" {
				t.Fatal("expected columns option to be populated")
			}

			// Verify DDL.Constraints contains the FK constraint with correct facts.
			fkFound := false
			for _, c := range stmt.DDL.Constraints {
				if c.Type == "foreign_key" && c.Name == "fk_orders_user" {
					fkFound = true
					if len(c.Columns) != len(tt.wantColumns) || c.Columns[0] != tt.wantColumns[0] {
						t.Errorf("expected Columns %v, got %v", tt.wantColumns, c.Columns)
					}
					if c.ReferencedTable != tt.wantRefTable {
						t.Errorf("expected ReferencedTable %q, got %q", tt.wantRefTable, c.ReferencedTable)
					}
					if len(c.ReferencedColumns) != len(tt.wantRefColumns) || c.ReferencedColumns[0] != tt.wantRefColumns[0] {
						t.Errorf("expected ReferencedColumns %v, got %v", tt.wantRefColumns, c.ReferencedColumns)
					}
					if tt.wantRefSchema != "" && c.ReferencedSchema != tt.wantRefSchema {
						t.Errorf("expected ReferencedSchema %q, got %q", tt.wantRefSchema, c.ReferencedSchema)
					}
				}
			}
			if !fkFound {
				t.Fatalf("expected foreign_key constraint fk_orders_user in %+v", stmt.DDL.Constraints)
			}
		})
	}
}

func serviceMetadataValueEqual(a, b any) bool {
	aFloat, aIsNum := toFloat64(a)
	bFloat, bIsNum := toFloat64(b)
	if aIsNum && bIsNum {
		return aFloat == bFloat
	}
	return a == b
}

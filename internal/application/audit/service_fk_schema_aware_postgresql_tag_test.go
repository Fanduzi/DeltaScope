//go:build postgresql

package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLPostgreSQLInlineSchemaQualifiedReferencesPreservesReferencedSchemaFacts(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

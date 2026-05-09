//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractCreateTableConstraintNormalization(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		sql            string
		tableName      string
		wantColumns    int
		wantConstraint *spec.Constraint
		wantIndex      *spec.Index
	}{
		{
			name: "named_table_check",
			sql: `create table orders (
				id bigint primary key,
				amount bigint,
				constraint chk_orders_amount check (amount > 0)
			);`,
			tableName:   "orders",
			wantColumns: 2,
			wantConstraint: &spec.Constraint{
				Type:    "check",
				Name:    "chk_orders_amount",
				Columns: []string{"amount"},
			},
		},
		{
			name: "inline_column_check",
			sql: `create table users (
				age int check (age >= 0)
			);`,
			tableName:   "users",
			wantColumns: 1,
			wantConstraint: &spec.Constraint{
				Type:    "check",
				Columns: []string{"age"},
			},
		},
		{
			name: "named_table_unique",
			sql: `create table users (
				id bigint,
				email text,
				constraint uq_users_email unique (email)
			);`,
			tableName:   "users",
			wantColumns: 2,
			wantIndex: &spec.Index{
				Name:    "uq_users_email",
				Kind:    spec.IndexKindUnique,
				Columns: []string{"email"},
			},
		},
		{
			name: "inline_column_unique",
			sql: `create table users (
				email text unique
			);`,
			tableName:   "users",
			wantColumns: 1,
			wantIndex: &spec.Index{
				Kind:    spec.IndexKindUnique,
				Columns: []string{"email"},
			},
		},
		{
			name: "named_table_foreign_key",
			sql: `create table orders (
				user_id bigint,
				constraint fk_orders_user foreign key (user_id) references users(id)
			);`,
			tableName:   "orders",
			wantColumns: 1,
			wantConstraint: &spec.Constraint{
				Type:    "foreign_key",
				Name:    "fk_orders_user",
				Columns: []string{"user_id"},
			},
		},
		{
			name: "inline_column_references",
			sql: `create table orders (
				user_id bigint references users(id)
			);`,
			tableName:   "orders",
			wantColumns: 1,
			wantConstraint: &spec.Constraint{
				Type:    "foreign_key",
				Columns: []string{"user_id"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			statement := extractPostgreSQLStatement(t, tt.sql)

			if statement.Kind != spec.KindDDL {
				t.Fatalf("expected kind %q, got %q", spec.KindDDL, statement.Kind)
			}
			if statement.Unsupported != nil {
				t.Fatalf("expected supported create table, got unsupported %#v", statement.Unsupported)
			}
			if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCreateTable {
				t.Fatalf("expected create_table ddl payload, got %#v", statement.DDL)
			}
			if statement.DDL.Table == nil || statement.DDL.Table.Name != tt.tableName {
				t.Fatalf("expected table name %s, got %#v", tt.tableName, statement.DDL.Table)
			}
			if len(statement.DDL.Columns) != tt.wantColumns {
				t.Fatalf("expected %d columns, got %#v", tt.wantColumns, statement.DDL.Columns)
			}

			if tt.wantConstraint != nil {
				if len(statement.DDL.Constraints) != 1 {
					t.Fatalf("expected 1 constraint, got %#v", statement.DDL.Constraints)
				}
				constraint := statement.DDL.Constraints[0]
				if constraint.Type != tt.wantConstraint.Type || constraint.Name != tt.wantConstraint.Name {
					t.Fatalf("expected constraint %+v, got %+v", *tt.wantConstraint, constraint)
				}
				if len(constraint.Columns) != len(tt.wantConstraint.Columns) {
					t.Fatalf("expected constraint columns %#v, got %#v", tt.wantConstraint.Columns, constraint.Columns)
				}
				for i, wantColumn := range tt.wantConstraint.Columns {
					if constraint.Columns[i] != wantColumn {
						t.Fatalf("expected constraint columns %#v, got %#v", tt.wantConstraint.Columns, constraint.Columns)
					}
				}
			}

			if tt.wantIndex != nil {
				if len(statement.DDL.Indexes) != 1 {
					t.Fatalf("expected 1 index, got %#v", statement.DDL.Indexes)
				}
				index := statement.DDL.Indexes[0]
				if index.Name != tt.wantIndex.Name || index.Kind != tt.wantIndex.Kind {
					t.Fatalf("expected index %+v, got %+v", *tt.wantIndex, index)
				}
				if len(index.Columns) != len(tt.wantIndex.Columns) {
					t.Fatalf("expected index columns %#v, got %#v", tt.wantIndex.Columns, index.Columns)
				}
				for i, wantColumn := range tt.wantIndex.Columns {
					if index.Columns[i] != wantColumn {
						t.Fatalf("expected index columns %#v, got %#v", tt.wantIndex.Columns, index.Columns)
					}
				}
			}
		})
	}
}

func TestExtractCreateTableForeignKeyPreservesReferencedTableAndColumns(t *testing.T) {
	t.Parallel()
	sql := `create table orders (
		user_id bigint,
		constraint fk_orders_user foreign key (user_id) references users(id)
	);`
	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind %q, got %q", spec.KindDDL, statement.Kind)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported create table, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCreateTable {
		t.Fatalf("expected create_table ddl payload, got %#v", statement.DDL)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "orders" {
		t.Fatalf("expected table name orders, got %#v", statement.DDL.Table)
	}
	if len(statement.DDL.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(statement.DDL.Constraints))
	}
	constraint := statement.DDL.Constraints[0]
	if constraint.Type != "foreign_key" || constraint.Name != "fk_orders_user" {
		t.Fatalf("expected named foreign_key constraint, got %+v", constraint)
	}
	if len(constraint.Columns) != 1 || constraint.Columns[0] != "user_id" {
		t.Fatalf("expected local columns [user_id], got %#v", constraint.Columns)
	}
	if constraint.ReferencedTable != "users" {
		t.Fatalf("expected referenced table users, got %q", constraint.ReferencedTable)
	}
	if len(constraint.ReferencedColumns) != 1 || constraint.ReferencedColumns[0] != "id" {
		t.Fatalf("expected referenced columns [id], got %#v", constraint.ReferencedColumns)
	}
}

func TestExtractCreateTableInlineReferencesPreservesReferencedTableAndColumns(t *testing.T) {
	t.Parallel()
	sql := `create table orders (
		user_id bigint references users(id)
	);`
	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind %q, got %q", spec.KindDDL, statement.Kind)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported create table, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCreateTable {
		t.Fatalf("expected create_table ddl payload, got %#v", statement.DDL)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "orders" {
		t.Fatalf("expected table name orders, got %#v", statement.DDL.Table)
	}
	if len(statement.DDL.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(statement.DDL.Constraints))
	}
	constraint := statement.DDL.Constraints[0]
	if constraint.Type != "foreign_key" {
		t.Fatalf("expected foreign_key constraint, got %+v", constraint)
	}
	if len(constraint.Columns) != 1 || constraint.Columns[0] != "user_id" {
		t.Fatalf("expected local columns [user_id], got %#v", constraint.Columns)
	}
	if constraint.ReferencedTable != "users" {
		t.Fatalf("expected referenced table users, got %q", constraint.ReferencedTable)
	}
	if len(constraint.ReferencedColumns) != 1 || constraint.ReferencedColumns[0] != "id" {
		t.Fatalf("expected referenced columns [id], got %#v", constraint.ReferencedColumns)
	}
}

func TestExtractValidateConstraintSchemaQualifiedFact(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "ALTER TABLE public.orders VALIDATE CONSTRAINT chk_orders_amount;")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table DDL, got %#v", statement.DDL)
	}
	if statement.DDL.Table == nil {
		t.Fatal("expected table fact")
	}
	if statement.DDL.Table.Schema != "public" || statement.DDL.Table.Name != "orders" {
		t.Fatalf("expected public.orders, got %#v", statement.DDL.Table)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "validate_constraint" || alter.Name != "chk_orders_amount" {
		t.Fatalf("expected validate_constraint chk_orders_amount, got %#v", alter)
	}
}

func TestExtractAlterAddCheckNotValidConstraintFact(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t,
		"ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table, got %#v", statement.DDL)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "orders" {
		t.Fatalf("expected table orders, got %#v", statement.DDL.Table)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "add_constraint" {
		t.Fatalf("expected add_constraint, got %q", alter.Action)
	}
	if alter.Name != "chk_orders_amount" {
		t.Fatalf("expected constraint name chk_orders_amount, got %q", alter.Name)
	}
	if alter.Options["constraint_type"] != "check" {
		t.Fatalf("expected constraint_type=check, got %q", alter.Options["constraint_type"])
	}
	if alter.Options["not_valid"] != "true" {
		t.Fatalf("expected not_valid=true, got %q", alter.Options["not_valid"])
	}
}

func TestExtractAlterAddForeignKeyNotValidConstraintFact(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t,
		"ALTER TABLE public.orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id) NOT VALID;")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table, got %#v", statement.DDL)
	}
	if statement.DDL.Table == nil {
		t.Fatal("expected table fact")
	}
	if statement.DDL.Table.Schema != "public" || statement.DDL.Table.Name != "orders" {
		t.Fatalf("expected public.orders, got %#v", statement.DDL.Table)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "add_constraint" {
		t.Fatalf("expected add_constraint, got %q", alter.Action)
	}
	if alter.Name != "fk_orders_user" {
		t.Fatalf("expected constraint name fk_orders_user, got %q", alter.Name)
	}
	if alter.Options["constraint_type"] != "foreign_key" {
		t.Fatalf("expected constraint_type=foreign_key, got %q", alter.Options["constraint_type"])
	}
	if alter.Options["not_valid"] != "true" {
		t.Fatalf("expected not_valid=true, got %q", alter.Options["not_valid"])
	}
	if alter.Options["columns"] != "user_id" {
		t.Fatalf("expected columns=user_id, got %q", alter.Options["columns"])
	}
	if alter.Options["referenced_table"] != "users" {
		t.Fatalf("expected referenced_table=users, got %q", alter.Options["referenced_table"])
	}
	if alter.Options["referenced_columns"] != "id" {
		t.Fatalf("expected referenced_columns=id, got %q", alter.Options["referenced_columns"])
	}
}

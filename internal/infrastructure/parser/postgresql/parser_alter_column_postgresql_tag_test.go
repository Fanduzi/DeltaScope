//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractAlterAddCheckNotValidFlag(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "alter table public.orders add constraint chk_amount check (amount > 0) not valid;")
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
	if statement.DDL == nil || len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %#v", statement)
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "add_constraint" {
		t.Fatalf("expected add_constraint, got %#v", alter)
	}
	if alter.Options["constraint_type"] != "check" {
		t.Fatalf("expected constraint_type=check, got %#v", alter.Options)
	}
	if alter.Options["not_valid"] != "true" {
		t.Fatalf("expected not_valid=true, got %#v", alter.Options)
	}
}

func TestExtractAlterAddCheckWithoutNotValid(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "alter table public.orders add constraint chk_amount check (amount > 0);")
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
	if statement.DDL == nil || len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %#v", statement)
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "add_constraint" {
		t.Fatalf("expected add_constraint, got %#v", alter)
	}
	if alter.Options["constraint_type"] != "check" {
		t.Fatalf("expected constraint_type=check, got %#v", alter.Options)
	}
	if alter.Options["not_valid"] != "false" {
		t.Fatalf("expected not_valid=false, got %#v", alter.Options)
	}
}

func TestExtractAlterColumnSetDefault(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "alter table users alter column status set default 'active';")
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
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "set_default" {
		t.Fatalf("expected set_default action, got %q", alter.Action)
	}
	if alter.Name != "status" {
		t.Fatalf("expected column name status, got %q", alter.Name)
	}
	if alter.Column == nil || alter.Column.Change == nil || !alter.Column.Change.TouchesDefault {
		t.Fatalf("expected TouchesDefault=true, got %#v", alter.Column)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

func TestExtractAlterColumnDropDefault(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "alter table users alter column status drop default;")
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
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "drop_default" {
		t.Fatalf("expected drop_default action, got %q", alter.Action)
	}
	if alter.Name != "status" {
		t.Fatalf("expected column name status, got %q", alter.Name)
	}
	if alter.Column == nil || alter.Column.Change == nil || !alter.Column.Change.TouchesDefault {
		t.Fatalf("expected TouchesDefault=true, got %#v", alter.Column)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

func TestExtractAlterColumnSetNotNull(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "alter table users alter column status set not null;")
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
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "set_not_null" {
		t.Fatalf("expected set_not_null action, got %q", alter.Action)
	}
	if alter.Name != "status" {
		t.Fatalf("expected column name status, got %q", alter.Name)
	}
	if alter.Column == nil || alter.Column.Change == nil || !alter.Column.Change.TouchesNullability {
		t.Fatalf("expected TouchesNullability=true, got %#v", alter.Column)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

func TestExtractAlterColumnDropNotNull(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "alter table users alter column status drop not null;")
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
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "drop_not_null" {
		t.Fatalf("expected drop_not_null action, got %q", alter.Action)
	}
	if alter.Name != "status" {
		t.Fatalf("expected column name status, got %q", alter.Name)
	}
	if alter.Column == nil || alter.Column.Change == nil || !alter.Column.Change.TouchesNullability {
		t.Fatalf("expected TouchesNullability=true, got %#v", alter.Column)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

func TestParserSupportsPostgreSQLRenameIndex(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "alter index idx_old rename to idx_new;")
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
		t.Fatalf("expected supported rename index, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil {
		t.Fatalf("expected DDL, got nil")
	}
	if statement.DDL.Operation != spec.DDLOperationAlterIndex {
		t.Fatalf("expected operation alter_index, got %q", statement.DDL.Operation)
	}
	if statement.DDL.ObjectName != "idx_old" {
		t.Fatalf("expected object_name idx_old, got %q", statement.DDL.ObjectName)
	}
	if statement.DDL.ObjectType != "index" {
		t.Fatalf("expected object_type index, got %q", statement.DDL.ObjectType)
	}
	if statement.DDL.Options["action"] != "rename" {
		t.Fatalf("expected action=rename, got %q", statement.DDL.Options["action"])
	}
	if statement.DDL.Options["new_name"] != "idx_new" {
		t.Fatalf("expected new_name=idx_new, got %q", statement.DDL.Options["new_name"])
	}
}

func TestExtractValidateConstraint(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "alter table users validate constraint chk_amount;")
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
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "validate_constraint" {
		t.Fatalf("expected validate_constraint action, got %q", alter.Action)
	}
	if alter.Name != "chk_amount" {
		t.Fatalf("expected constraint name chk_amount, got %q", alter.Name)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

func TestExtractDropPrimaryKeyConstraint(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "alter table users drop constraint users_pkey;")
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
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "drop_constraint" {
		t.Fatalf("expected drop_constraint action, got %q", alter.Action)
	}
	if alter.Name != "users_pkey" {
		t.Fatalf("expected constraint name users_pkey, got %q", alter.Name)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

func TestExtractDropCheckConstraint(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "alter table users drop constraint chk_amount;")
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
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "drop_constraint" {
		t.Fatalf("expected drop_constraint action, got %q", alter.Action)
	}
	if alter.Name != "chk_amount" {
		t.Fatalf("expected constraint name chk_amount, got %q", alter.Name)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

// Package audit verifies AST-to-StatementSpec extraction behavior.
// input: parsed SQL statements produced by the application parsing flow
// output: test coverage for first-pass StatementSpec extraction
// pos: application audit extraction test coverage
// note: if this file changes, update this header and module README.md.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractMapsCreateTable(t *testing.T) {
	parsed, err := Parse("create table users (id bigint comment 'pk', name varchar(32) comment 'name', primary key (id), key idx_name (name)) comment='user table';", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if len(statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(statements))
	}

	stmt := statements[0]
	if stmt.Kind != spec.KindDDL {
		t.Fatalf("expected kind %q, got %q", spec.KindDDL, stmt.Kind)
	}
	if stmt.RawSQL == "" || stmt.NormalizedSQL == "" {
		t.Fatalf("expected raw and normalized sql to be populated")
	}
	if stmt.DDL == nil || stmt.DDL.Table == nil {
		t.Fatalf("expected ddl table metadata to be populated")
	}
	if stmt.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %q", stmt.DDL.Table.Name)
	}
	if len(stmt.DDL.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(stmt.DDL.Columns))
	}
	if stmt.DDL.PrimaryKey == nil {
		t.Fatalf("expected primary key metadata to be populated")
	}
	if stmt.DDL.PrimaryKey.Name != "primary" {
		t.Fatalf("expected primary key name primary, got %q", stmt.DDL.PrimaryKey.Name)
	}
	if len(stmt.DDL.Indexes) != 1 {
		t.Fatalf("expected 1 secondary index, got %d", len(stmt.DDL.Indexes))
	}
}

func TestExtractMapsAlterTable(t *testing.T) {
	parsed, err := Parse("alter table users add column age int, drop column old_age;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	stmt := statements[0]
	if stmt.DDL == nil || stmt.DDL.Table == nil {
		t.Fatalf("expected ddl metadata to be populated")
	}
	if stmt.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %q", stmt.DDL.Table.Name)
	}
	if len(stmt.DDL.Alter) != 2 {
		t.Fatalf("expected 2 alter actions, got %d", len(stmt.DDL.Alter))
	}
}

func TestExtractMapsInsert(t *testing.T) {
	parsed, err := Parse("insert into users(id, name) values (1, 'a'), (2, 'b') on duplicate key update name = values(name);", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	stmt := statements[0]
	if stmt.Kind != spec.KindDML || stmt.DML == nil {
		t.Fatalf("expected dml statement to be populated")
	}
	if stmt.DML.Operation != spec.DMLOperationInsert {
		t.Fatalf("expected insert operation, got %q", stmt.DML.Operation)
	}
	if stmt.DML.InsertRows != 2 {
		t.Fatalf("expected 2 insert rows, got %d", stmt.DML.InsertRows)
	}
	if !stmt.DML.HasOnDuplicate {
		t.Fatalf("expected insert to report has_on_duplicate=true")
	}
}

func TestExtractMapsUpdate(t *testing.T) {
	parsed, err := Parse("update users set name = 'c' where id = 1;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	stmt := statements[0]
	if stmt.DML == nil {
		t.Fatalf("expected dml metadata to be populated")
	}
	if stmt.DML.Operation != spec.DMLOperationUpdate {
		t.Fatalf("expected update operation, got %q", stmt.DML.Operation)
	}
	if !stmt.DML.HasWhere {
		t.Fatalf("expected update to have where")
	}
	if stmt.DML.HasJoin {
		t.Fatalf("expected single-table update to report no join")
	}
}

func TestExtractMapsDelete(t *testing.T) {
	parsed, err := Parse("delete from users where id = 1 limit 1;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	stmt := statements[0]
	if stmt.DML == nil {
		t.Fatalf("expected dml metadata to be populated")
	}
	if stmt.DML.Operation != spec.DMLOperationDelete {
		t.Fatalf("expected delete operation, got %q", stmt.DML.Operation)
	}
	if !stmt.DML.HasWhere {
		t.Fatalf("expected delete to have where")
	}
	if !stmt.DML.HasLimit {
		t.Fatalf("expected delete to have limit")
	}
	if stmt.DML.HasJoin {
		t.Fatalf("expected single-table delete to report no join")
	}
}

func TestExtractDistinguishesJoinWithoutOn(t *testing.T) {
	parsed, err := Parse("update users u join accounts a set u.name = 'c' where u.id = a.user_id;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	stmt := statements[0]
	if stmt.DML == nil {
		t.Fatalf("expected dml metadata to be populated")
	}
	if !stmt.DML.HasJoin {
		t.Fatalf("expected update join to report has_join=true")
	}
	if stmt.DML.HasJoinOn {
		t.Fatalf("expected join without ON to report has_join_on=false")
	}
}

func TestExtractMapsInsertSelect(t *testing.T) {
	parsed, err := Parse("insert into users(id, name) select id, name from staging_users;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	stmt := statements[0]
	if stmt.DML == nil {
		t.Fatalf("expected dml metadata to be populated")
	}
	if !stmt.DML.IsInsertSelect {
		t.Fatalf("expected insert-select metadata to be populated")
	}
}

func TestExtractLeavesUnknownStatementsAvailableForLaterLayers(t *testing.T) {
	parsed, err := Parse("select 1;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("expected unknown-but-parseable statement to survive extraction, got %v", err)
	}

	if len(statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(statements))
	}
	if statements[0].Kind != spec.KindUnknown {
		t.Fatalf("expected unknown kind, got %q", statements[0].Kind)
	}
	if statements[0].DDL != nil || statements[0].DML != nil {
		t.Fatalf("expected unknown statement to keep empty DDL/DML substructures")
	}
}

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
	if len(stmt.DDL.Indexes) != 2 {
		t.Fatalf("expected 2 indexes, got %d", len(stmt.DDL.Indexes))
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
	parsed, err := Parse("insert into users(id, name) values (1, 'a'), (2, 'b');", spec.DialectMySQL)
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
	if stmt.DML.InsertRows != 2 {
		t.Fatalf("expected 2 insert rows, got %d", stmt.DML.InsertRows)
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
	if !stmt.DML.HasWhere {
		t.Fatalf("expected update to have where")
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
	if !stmt.DML.HasWhere {
		t.Fatalf("expected delete to have where")
	}
	if !stmt.DML.HasLimit {
		t.Fatalf("expected delete to have limit")
	}
}

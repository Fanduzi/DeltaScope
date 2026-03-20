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
	parsed, err := Parse("create table users (id bigint not null default 1 comment 'pk', name varchar(32) default 'guest' comment 'name', body text comment 'body', created_at datetime not null default current_timestamp comment 'created', updated_at datetime not null default current_timestamp on update current_timestamp comment 'updated', primary key (id), key idx_name (name), unique key uniq_name (name), fulltext key full_body (body)) comment='user table';", spec.DialectMySQL)
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
	if len(stmt.DDL.Columns) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(stmt.DDL.Columns))
	}

	nameCol := stmt.DDL.Columns[1]
	if nameCol.Type != "varchar(32)" {
		t.Fatalf("expected normalized varchar type, got %q", nameCol.Type)
	}
	if nameCol.Length != 32 {
		t.Fatalf("expected varchar length 32, got %d", nameCol.Length)
	}
	if !nameCol.HasDefault || nameCol.DefaultValue != "'guest'" {
		t.Fatalf("expected default value 'guest', got has_default=%t default=%q", nameCol.HasDefault, nameCol.DefaultValue)
	}

	createdAt := stmt.DDL.Columns[3]
	if !createdAt.NotNull {
		t.Fatalf("expected created_at to be not null")
	}
	if !createdAt.DefaultIsCurrentTimestamp {
		t.Fatalf("expected created_at to use current_timestamp default")
	}
	if createdAt.OnUpdateCurrentTimestamp {
		t.Fatalf("expected created_at not to carry on update current_timestamp")
	}

	updatedAt := stmt.DDL.Columns[4]
	if !updatedAt.DefaultIsCurrentTimestamp || !updatedAt.OnUpdateCurrentTimestamp {
		t.Fatalf("expected updated_at audit timestamp metadata, got %+v", updatedAt)
	}
	if stmt.DDL.PrimaryKey == nil {
		t.Fatalf("expected primary key metadata to be populated")
	}
	if stmt.DDL.PrimaryKey.Name != "primary" {
		t.Fatalf("expected primary key name primary, got %q", stmt.DDL.PrimaryKey.Name)
	}
	if stmt.DDL.PrimaryKey.Kind != spec.IndexKindPrimary {
		t.Fatalf("expected primary key kind %q, got %q", spec.IndexKindPrimary, stmt.DDL.PrimaryKey.Kind)
	}
	if len(stmt.DDL.Indexes) != 3 {
		t.Fatalf("expected 3 secondary indexes, got %d", len(stmt.DDL.Indexes))
	}
	if stmt.DDL.Indexes[0].Kind != spec.IndexKindSecondary {
		t.Fatalf("expected first index kind %q, got %q", spec.IndexKindSecondary, stmt.DDL.Indexes[0].Kind)
	}
	if stmt.DDL.Indexes[1].Kind != spec.IndexKindUnique {
		t.Fatalf("expected second index kind %q, got %q", spec.IndexKindUnique, stmt.DDL.Indexes[1].Kind)
	}
	if stmt.DDL.Indexes[2].Kind != spec.IndexKindFulltext {
		t.Fatalf("expected third index kind %q, got %q", spec.IndexKindFulltext, stmt.DDL.Indexes[2].Kind)
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

func TestExtractMapsCreateTableLike(t *testing.T) {
	parsed, err := Parse("create table users_copy like users;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !statements[0].DDL.HasReferTable {
		t.Fatalf("expected create table like to set has_refer_table=true")
	}
}

func TestExtractMapsCreateTableAsSelect(t *testing.T) {
	parsed, err := Parse("create table users_copy as select * from users;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !statements[0].DDL.HasSelect {
		t.Fatalf("expected create table as select to set has_select=true")
	}
}

func TestExtractMapsCreateTablePartition(t *testing.T) {
	parsed, err := Parse("create table users (id bigint primary key) partition by hash(id) partitions 4;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !statements[0].DDL.HasPartition {
		t.Fatalf("expected partitioned create table to set has_partition=true")
	}
}

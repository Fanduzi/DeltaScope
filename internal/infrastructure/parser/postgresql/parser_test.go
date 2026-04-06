//go:build postgresql

package postgresql

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestParserParsesMultiStatementSQL(t *testing.T) {
	parser := New()

	result, err := parser.Parse("create table t1 (id bigint); update t1 set id = 2 where id = 1;")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(result.Statements))
	}

	if result.Statements[0].Kind != spec.KindDDL {
		t.Fatalf("expected first statement to be DDL, got %q", result.Statements[0].Kind)
	}
	if result.Statements[1].Kind != spec.KindDML {
		t.Fatalf("expected second statement to be DML, got %q", result.Statements[1].Kind)
	}
	if result.Statements[0].RawSQL != "create table t1 (id bigint);" {
		t.Fatalf("expected first statement SQL to be sliced, got %q", result.Statements[0].RawSQL)
	}
	if result.Statements[1].RawSQL != "update t1 set id = 2 where id = 1;" {
		t.Fatalf("expected second statement SQL to be sliced, got %q", result.Statements[1].RawSQL)
	}
}

func TestPGExtractorExtractNormalizesStatementSQL(t *testing.T) {
	parser := New()

	result, err := parser.Parse(" create table t1 (id bigint); ")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, " create table t1 (id bigint); ")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind %q, got %q", spec.KindDDL, statement.Kind)
	}
	if statement.Dialect != spec.DialectPostgreSQL {
		t.Fatalf("expected dialect %q, got %q", spec.DialectPostgreSQL, statement.Dialect)
	}
	if statement.RawSQL != " create table t1 (id bigint); " {
		t.Fatalf("expected raw SQL preserved, got %q", statement.RawSQL)
	}
	if statement.NormalizedSQL != "create table t1 (id bigint)" {
		t.Fatalf("expected normalized SQL without outer whitespace or trailing semicolon, got %q", statement.NormalizedSQL)
	}
}

func TestParserMarksUnsupportedPostgreSQLStatementsStructurally(t *testing.T) {
	parser := New()

	result, err := parser.Parse("select 1;")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract unsupported statement: %v", err)
	}
	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected unsupported statement kind unknown, got %q", statement.Kind)
	}
	if statement.Unsupported == nil {
		t.Fatal("expected unsupported detail to be populated")
	}
	if statement.Unsupported.Feature != "select" {
		t.Fatalf("expected unsupported feature select, got %#v", statement.Unsupported)
	}
	if statement.Unsupported.Reason == "" {
		t.Fatalf("expected unsupported reason, got %#v", statement.Unsupported)
	}
}

func TestParserSupportsApprovedPostgreSQLAlterTableWhitelist(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter table users rename column old_name to new_name;")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract supported alter table statement: %v", err)
	}
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected ddl statement kind, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %#v", statement.DDL)
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "rename_column" {
		t.Fatalf("expected rename_column action, got %#v", alter)
	}
	if alter.Column == nil || alter.Column.OldName != "old_name" || alter.Column.Definition == nil || alter.Column.Definition.Name != "new_name" {
		t.Fatalf("expected old/new column names, got %#v", alter.Column)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported alter table to omit unsupported detail, got %#v", statement.Unsupported)
	}
}

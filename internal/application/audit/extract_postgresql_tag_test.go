//go:build postgresql

// Package audit verifies PostgreSQL-tagged AST-to-StatementSpec extraction behavior.
// input: PostgreSQL parsed SQL statements produced by the application parsing flow
// output: test coverage for PostgreSQL-only extraction boundaries
// pos: application audit PostgreSQL extraction test coverage
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractMapsPostgreSQLDropIndex(t *testing.T) {
	parsed, err := Parse(context.Background(), "drop index idx_name;", spec.DialectPostgreSQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(statements))
	}

	stmt := statements[0]
	if stmt.DDL == nil {
		t.Fatalf("expected ddl drop-index metadata")
	}
	if stmt.DDL.Operation != spec.DDLOperationDropIndex {
		t.Fatalf("expected operation %q, got %q", spec.DDLOperationDropIndex, stmt.DDL.Operation)
	}
	if stmt.DDL.Table != nil {
		t.Fatalf("expected standalone drop index to keep nil table, got %+v", stmt.DDL.Table)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter payload, got %d", len(stmt.DDL.Alter))
	}
	if stmt.DDL.Alter[0].Action != "drop_index" {
		t.Fatalf("expected alter action drop_index, got %q", stmt.DDL.Alter[0].Action)
	}
	if stmt.DDL.Alter[0].Name != "idx_name" {
		t.Fatalf("expected alter name idx_name, got %q", stmt.DDL.Alter[0].Name)
	}
}

func TestExtractMapsPostgreSQLRenameIndex(t *testing.T) {
	parsed, err := Parse(context.Background(), "alter index idx_old rename to idx_new;", spec.DialectPostgreSQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(statements))
	}

	stmt := statements[0]
	if stmt.DDL == nil {
		t.Fatalf("expected ddl rename-index metadata")
	}
	if stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected operation %q, got %q", spec.DDLOperationAlterTable, stmt.DDL.Operation)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter payload, got %d", len(stmt.DDL.Alter))
	}
	if stmt.DDL.Alter[0].Action != "rename_index" {
		t.Fatalf("expected alter action rename_index, got %q", stmt.DDL.Alter[0].Action)
	}
	if stmt.DDL.Alter[0].Name != "idx_old" {
		t.Fatalf("expected old index name idx_old, got %q", stmt.DDL.Alter[0].Name)
	}
	if stmt.DDL.Alter[0].Options["new_name"] != "idx_new" {
		t.Fatalf("expected new index name idx_new, got %#v", stmt.DDL.Alter[0].Options)
	}
}

func TestExtractPreservesPostgreSQLRenameIndexSchema(t *testing.T) {
	parsed, err := Parse(context.Background(), "alter index accounting.idx_old rename to idx_new;", spec.DialectPostgreSQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(statements))
	}

	stmt := statements[0]
	if stmt.DDL == nil || len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected one alter payload, got %#v", stmt)
	}
	if stmt.DDL.Alter[0].Options["schema"] != "accounting" {
		t.Fatalf("expected preserved index schema, got %#v", stmt.DDL.Alter[0].Options)
	}
}

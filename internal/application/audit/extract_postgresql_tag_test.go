//go:build postgresql

// Package audit verifies PostgreSQL-tagged AST-to-StatementSpec extraction behavior.
// input: PostgreSQL parsed SQL statements produced by the application parsing flow
// output: test coverage for PostgreSQL-only extraction boundaries
// pos: application audit PostgreSQL extraction test coverage
// note: if this file changes, update this header and module README.md.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractMapsPostgreSQLDropIndex(t *testing.T) {
	parsed, err := Parse("drop index idx_name;", spec.DialectPostgreSQL)
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

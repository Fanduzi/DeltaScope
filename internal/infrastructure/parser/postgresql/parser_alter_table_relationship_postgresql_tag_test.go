//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractAlterAddInherit(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE child_users INHERIT users")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported: %v", stmt.Unsupported)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table, got %#v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "add_inherit" {
		t.Fatalf("expected action add_inherit, got %q", alter.Action)
	}
	if alter.Options["parent_table"] != "users" {
		t.Fatalf("expected option parent_table=users, got %q", alter.Options["parent_table"])
	}
	if alter.Options["relationship"] != "inheritance" {
		t.Fatalf("expected option relationship=inheritance, got %q", alter.Options["relationship"])
	}
	for _, forbidden := range []string{"raw_sql", "column_definition", "catalog_state", "validation_result", "dependency_graph"} {
		if _, ok := alter.Options[forbidden]; ok {
			t.Fatalf("forbidden key %q present in options", forbidden)
		}
	}
}

func TestExtractAlterDropInherit(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE child_users NO INHERIT users")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported: %v", stmt.Unsupported)
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "drop_inherit" {
		t.Fatalf("expected action drop_inherit, got %q", alter.Action)
	}
	if alter.Options["parent_table"] != "users" {
		t.Fatalf("expected option parent_table=users, got %q", alter.Options["parent_table"])
	}
	if alter.Options["relationship"] != "inheritance" {
		t.Fatalf("expected option relationship=inheritance, got %q", alter.Options["relationship"])
	}
	for _, forbidden := range []string{"raw_sql", "column_definition", "catalog_state", "validation_result", "dependency_graph"} {
		if _, ok := alter.Options[forbidden]; ok {
			t.Fatalf("forbidden key %q present in options", forbidden)
		}
	}
}

func TestExtractAlterAddOfType(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE users OF user_type")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported: %v", stmt.Unsupported)
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "add_of_type" {
		t.Fatalf("expected action add_of_type, got %q", alter.Action)
	}
	if alter.Options["type"] != "user_type" {
		t.Fatalf("expected option type=user_type, got %q", alter.Options["type"])
	}
	if alter.Options["relationship"] != "typed_table" {
		t.Fatalf("expected option relationship=typed_table, got %q", alter.Options["relationship"])
	}
	for _, forbidden := range []string{"raw_sql", "column_definition", "type_attributes", "catalog_state", "validation_result", "dependency_graph"} {
		if _, ok := alter.Options[forbidden]; ok {
			t.Fatalf("forbidden key %q present in options", forbidden)
		}
	}
}

func TestExtractAlterDropOfType(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE users NOT OF")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported: %v", stmt.Unsupported)
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "drop_of_type" {
		t.Fatalf("expected action drop_of_type, got %q", alter.Action)
	}
	if alter.Options["relationship"] != "typed_table" {
		t.Fatalf("expected option relationship=typed_table, got %q", alter.Options["relationship"])
	}
	// NOT OF must NOT emit type key
	if _, hasType := alter.Options["type"]; hasType {
		t.Fatalf("forbidden key \"type\" present in NOT OF options")
	}
	for _, forbidden := range []string{"raw_sql", "column_definition", "type_attributes", "catalog_state", "validation_result", "dependency_graph"} {
		if _, ok := alter.Options[forbidden]; ok {
			t.Fatalf("forbidden key %q present in options", forbidden)
		}
	}
}

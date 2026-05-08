//go:build postgresql

package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestParseParsesPostgreSQLWithBuildTag(t *testing.T) {
	result, err := Parse(context.Background(), "select 1;", spec.DialectPostgreSQL)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got, want := result.Dialect, spec.DialectPostgreSQL; got != want {
		t.Fatalf("expected dialect %q, got %q", want, got)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 parsed statement, got %d", len(result.Statements))
	}
	if result.Statements[0].RawSQL == "" {
		t.Fatal("expected RawSQL to be populated")
	}
	if result.Statements[0].Extractor == nil {
		t.Fatal("expected parsed statement to expose a StatementExtractor")
	}
}

func TestExtractMapsPostgreSQLInsertOnConflictWithoutMySQLFlags(t *testing.T) {
	parsed, err := Parse(context.Background(), "insert into users(id, name) values (1, 'a') on conflict (id) do update set name = excluded.name;", spec.DialectPostgreSQL)
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
	if stmt.DML == nil {
		t.Fatalf("expected dml metadata to be populated")
	}
	if stmt.DML.HasOnDuplicate {
		t.Fatalf("expected postgresql on conflict not to map to has_on_duplicate")
	}
	if stmt.DML.IsInsertSelect {
		t.Fatalf("expected values-based postgresql insert not to map to insert-select")
	}
}

func TestExtractMapsPostgreSQLInsertSelectOnConflictWithoutMySQLDuplicateFlag(t *testing.T) {
	parsed, err := Parse(context.Background(), "insert into users(id, name) select id, name from staging_users on conflict (id) do update set name = excluded.name;", spec.DialectPostgreSQL)
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
	if stmt.DML == nil {
		t.Fatalf("expected dml metadata to be populated")
	}
	if stmt.DML.HasOnDuplicate {
		t.Fatalf("expected postgresql on conflict not to map to has_on_duplicate")
	}
	if !stmt.DML.IsInsertSelect {
		t.Fatalf("expected select-based postgresql insert to remain insert-select")
	}
}

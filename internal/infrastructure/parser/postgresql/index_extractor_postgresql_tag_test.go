//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractIndexStmtPartial(t *testing.T) {
	t.Parallel()
	stmt := extractIndex(t, "CREATE INDEX idx_active ON users (email) WHERE active = true")

	if stmt.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported: %s", stmt.Unsupported.Reason)
	}
	idx := stmt.DDL.Indexes[0]
	assertIndex(t, idx, "email", "btree")
	if !idx.HasPredicate {
		t.Error("expected HasPredicate true")
	}
	if idx.HasExpressionKeys {
		t.Error("expected HasExpressionKeys false")
	}
	if stmt.DDL.Options["concurrently"] != "false" {
		t.Errorf("expected concurrently=false, got %q", stmt.DDL.Options["concurrently"])
	}
}

func TestExtractIndexStmtExpression(t *testing.T) {
	t.Parallel()
	stmt := extractIndex(t, "CREATE INDEX idx_lower ON users (LOWER(email))")

	if stmt.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported: %s", stmt.Unsupported.Reason)
	}
	idx := stmt.DDL.Indexes[0]
	if len(idx.Columns) != 0 {
		t.Errorf("expected no key columns, got %v", idx.Columns)
	}
	if !idx.HasExpressionKeys {
		t.Error("expected HasExpressionKeys true")
	}
	if idx.ExpressionCount != 1 {
		t.Errorf("expected ExpressionCount 1, got %d", idx.ExpressionCount)
	}
}

func TestExtractIndexStmtInclude(t *testing.T) {
	t.Parallel()
	stmt := extractIndex(t, "CREATE INDEX idx_cover ON users (email) INCLUDE (name, active)")

	if stmt.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported: %s", stmt.Unsupported.Reason)
	}
	idx := stmt.DDL.Indexes[0]
	assertIndex(t, idx, "email", "btree")
	if len(idx.IncludedColumns) != 2 {
		t.Fatalf("expected 2 included columns, got %v", idx.IncludedColumns)
	}
	if idx.IncludedColumns[0] != "name" || idx.IncludedColumns[1] != "active" {
		t.Errorf("expected included [name active], got %v", idx.IncludedColumns)
	}
}

func TestExtractIndexStmtGin(t *testing.T) {
	t.Parallel()
	stmt := extractIndex(t, "CREATE INDEX idx_body ON docs USING gin (body)")

	if stmt.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported: %s", stmt.Unsupported.Reason)
	}
	idx := stmt.DDL.Indexes[0]
	assertIndex(t, idx, "body", "gin")
}

func TestExtractIndexStmtConcurrentPartial(t *testing.T) {
	t.Parallel()
	stmt := extractIndex(t, "CREATE INDEX CONCURRENTLY idx_active ON users (email) WHERE active = true")

	if stmt.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported: %s", stmt.Unsupported.Reason)
	}
	idx := stmt.DDL.Indexes[0]
	if !idx.HasPredicate {
		t.Error("expected HasPredicate true")
	}
	if stmt.DDL.Options["concurrently"] != "true" {
		t.Errorf("expected concurrently=true, got %q", stmt.DDL.Options["concurrently"])
	}
}

func TestExtractIndexStmtNullsNotDistinctUnsupported(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "CREATE INDEX idx_nulls ON t (col) NULLS NOT DISTINCT")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) == 0 {
		t.Fatal("expected at least one statement")
	}
	stmt, extractErr := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if extractErr != nil {
		t.Fatalf("extract: %v", extractErr)
	}
	if stmt.Unsupported == nil {
		t.Fatal("expected NULLS NOT DISTINCT to remain unsupported")
	}
	if stmt.Unsupported.Feature != "create_index" {
		t.Errorf("expected feature create_index, got %q", stmt.Unsupported.Feature)
	}
}

func extractIndex(t *testing.T, sql string) spec.Statement {
	t.Helper()
	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) == 0 {
		t.Fatal("no statements returned")
	}
	s, extractErr := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if extractErr != nil {
		t.Fatalf("extract: %v", extractErr)
	}
	if s.Unsupported != nil {
		return s
	}
	if s.DDL == nil || len(s.DDL.Indexes) == 0 {
		t.Fatal("expected DDL with at least one index")
	}
	return s
}

func assertIndex(t *testing.T, idx spec.Index, firstCol string, accessMethod string) {
	t.Helper()
	if len(idx.Columns) == 0 || idx.Columns[0] != firstCol {
		t.Errorf("expected first column %q, got %v", firstCol, idx.Columns)
	}
	if idx.AccessMethod != accessMethod {
		t.Errorf("expected access method %q, got %q", accessMethod, idx.AccessMethod)
	}
}

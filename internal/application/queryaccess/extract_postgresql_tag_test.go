//go:build postgresql

// Package queryaccess tests the PostgreSQL query access application adapter.
// input: QueryAccessRequest with SQL exercising PostgreSQL-specific features
// output: QueryAccessResult assertions for read classification, relations, columns, and admission
// pos: application-level tests for the PostgreSQL query access adapter
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestAnalyzePostgreSQL_SimpleSelect(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "SELECT id, name FROM users",
		Dialect: "postgresql",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	dr := result.DomainResult
	if dr.ReadClassification != domain.ReadOnly {
		t.Errorf("classification: got %q, want %q", dr.ReadClassification, domain.ReadOnly)
	}
	if dr.Dialect != "postgresql" {
		t.Errorf("dialect: got %q, want %q", dr.Dialect, "postgresql")
	}
	if len(dr.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(dr.Relations))
	}
	if dr.Relations[0].Name != "users" {
		t.Errorf("relation name: got %q, want %q", dr.Relations[0].Name, "users")
	}
}

func TestAnalyzePostgreSQL_SelectWithWhere(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "SELECT id FROM users WHERE id = 1",
		Dialect: "postgresql",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	// PostgreSQL V1: WHERE operator → indeterminate
	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification: got %q, want %q", result.DomainResult.ReadClassification, domain.Indeterminate)
	}
}

func TestAnalyzePostgreSQL_SelectForUpdate(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "SELECT * FROM users FOR UPDATE",
		Dialect: "postgresql",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.DomainResult.ReadClassification != domain.NotReadOnly {
		t.Errorf("classification: got %q, want %q", result.DomainResult.ReadClassification, domain.NotReadOnly)
	}
}

func TestAnalyzePostgreSQL_ExplainAnalyze(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "EXPLAIN ANALYZE SELECT * FROM users",
		Dialect: "postgresql",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.DomainResult.ReadClassification != domain.NotReadOnly {
		t.Errorf("classification: got %q, want %q", result.DomainResult.ReadClassification, domain.NotReadOnly)
	}
}

func TestAnalyzePostgreSQL_DDLNotReadOnly(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "CREATE TABLE t1 (id INT)",
		Dialect: "postgresql",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.DomainResult.ReadClassification != domain.NotReadOnly {
		t.Errorf("classification: got %q, want %q", result.DomainResult.ReadClassification, domain.NotReadOnly)
	}
}

func TestAnalyzePostgreSQL_InsertNotReadOnly(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "INSERT INTO users (name) VALUES ('alice')",
		Dialect: "postgresql",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.DomainResult.ReadClassification != domain.NotReadOnly {
		t.Errorf("classification: got %q, want %q", result.DomainResult.ReadClassification, domain.NotReadOnly)
	}
}

func TestAnalyzePostgreSQL_EmptyInput(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "",
		Dialect: "postgresql",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification: got %q, want %q", result.DomainResult.ReadClassification, domain.Indeterminate)
	}
}

func TestAnalyzePostgreSQL_InvalidMode(t *testing.T) {
	t.Parallel()
	_, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "SELECT id FROM users",
		Dialect: "postgresql",
		Mode:    "invalid_mode",
	})
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestAnalyzePostgreSQL_DefaultSchema(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:           "SELECT id FROM users",
		Dialect:       "postgresql",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(result.DomainResult.Relations) != 1 {
		t.Fatalf("relations: got %d, want 1", len(result.DomainResult.Relations))
	}
	if result.DomainResult.Relations[0].Schema != "app" {
		t.Errorf("relation schema: got %q, want %q", result.DomainResult.Relations[0].Schema, "app")
	}
}

func TestAnalyzePostgreSQL_AuditRegression(t *testing.T) {
	t.Parallel()
	// SELECT reaches audit as KindUnknown (unsupported boundary)
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "SELECT id FROM users",
		Dialect: "postgresql",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.DomainResult.ReadClassification == "" {
		t.Error("classification should not be empty")
	}
}

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
	if result.DomainResult.Relations[0].Schema != "" {
		t.Errorf("relation schema: got %q, want empty (unqualified uses search_path)", result.DomainResult.Relations[0].Schema)
	}
}

func TestAnalyzePostgreSQL_AuditRegression(t *testing.T) {
	t.Parallel()
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

func TestAnalyzePostgreSQL_OperatorExprStaysIndeterminate(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "SELECT id FROM users WHERE id = 1",
		Dialect: "postgresql",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification: got %q, want %q (operator expression)", result.DomainResult.ReadClassification, domain.Indeterminate)
	}
	if result.DomainResult.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission: got %q, want %q", result.DomainResult.Admission, domain.IndeterminateAdmission)
	}
}

func TestAnalyzePostgreSQL_FunctionCallStaysIndeterminate(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "SELECT now() FROM users",
		Dialect: "postgresql",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification: got %q, want %q (function call)", result.DomainResult.ReadClassification, domain.Indeterminate)
	}
}

func TestAnalyzePostgreSQL_CastExprStaysIndeterminate(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "SELECT id::text FROM users",
		Dialect: "postgresql",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification: got %q, want %q (cast expression)", result.DomainResult.ReadClassification, domain.Indeterminate)
	}
}

func TestAnalyzePostgreSQL_MalformedSQLStaysIndeterminate(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "SELECT * FROM WHERE",
		Dialect: "postgresql",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.DomainResult.ReadClassification != domain.Indeterminate {
		t.Errorf("classification: got %q, want %q (malformed SQL)", result.DomainResult.ReadClassification, domain.Indeterminate)
	}
	if result.DomainResult.Admission != domain.IndeterminateAdmission {
		t.Errorf("admission: got %q, want %q", result.DomainResult.Admission, domain.IndeterminateAdmission)
	}
}

func TestAnalyzePostgreSQL_OperatorColumnRefsMapped(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:           "SELECT u.id FROM users u JOIN orders o ON u.id = o.user_id",
		Dialect:       "postgresql",
		DefaultSchema: "public",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(result.EffectCandidates) == 0 {
		t.Fatal("expected operator candidate for u.id = o.user_id")
	}
	var opCand *EffectCandidate
	for i := range result.EffectCandidates {
		if result.EffectCandidates[i].Kind == EffectCandidateOperator {
			opCand = &result.EffectCandidates[i]
			break
		}
	}
	if opCand == nil {
		t.Fatal("expected operator candidate")
	}
	if len(opCand.OperandColumnRefs) != 2 {
		t.Fatalf("expected 2 operand column refs, got %d: %+v", len(opCand.OperandColumnRefs), opCand.OperandColumnRefs)
	}
	ref0 := opCand.OperandColumnRefs[0]
	if ref0.Schema != "" || ref0.Table != "users" || ref0.Column != "id" {
		t.Errorf("left operand: got schema=%q table=%q column=%q, want users.id", ref0.Schema, ref0.Table, ref0.Column)
	}
	ref1 := opCand.OperandColumnRefs[1]
	if ref1.Schema != "" || ref1.Table != "orders" || ref1.Column != "user_id" {
		t.Errorf("right operand: got schema=%q table=%q column=%q, want orders.user_id", ref1.Schema, ref1.Table, ref1.Column)
	}
}

func TestAnalyzePostgreSQL_LiteralOperandNoColumnRefs(t *testing.T) {
	t.Parallel()
	result, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
		SQL:     "SELECT id FROM users WHERE id = 1",
		Dialect: "postgresql",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(result.EffectCandidates) == 0 {
		t.Fatal("expected operator candidate for id = 1")
	}
	var opCand *EffectCandidate
	for i := range result.EffectCandidates {
		if result.EffectCandidates[i].Kind == EffectCandidateOperator {
			opCand = &result.EffectCandidates[i]
			break
		}
	}
	if opCand == nil {
		t.Fatal("expected operator candidate")
	}
	if len(opCand.OperandColumnRefs) != 1 {
		t.Fatalf("expected 1 operand column ref (left column, right literal), got %d: %+v", len(opCand.OperandColumnRefs), opCand.OperandColumnRefs)
	}
	if opCand.OperandColumnRefs[0].Table != "users" || opCand.OperandColumnRefs[0].Column != "id" {
		t.Errorf("left operand: got table=%q column=%q, want users.id", opCand.OperandColumnRefs[0].Table, opCand.OperandColumnRefs[0].Column)
	}
}
